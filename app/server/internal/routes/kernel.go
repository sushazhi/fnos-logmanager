package routes

import (
	"bytes"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/sushazhi/fnos-logmanager/internal/middleware"
)

// kernelVersion represents an installed Linux kernel version.
type kernelVersion struct {
	Version              string `json:"version"`
	IsCurrent            bool   `json:"isCurrent"`
	BootSize             int64  `json:"bootSize"`
	BootSizeFormatted    string `json:"bootSizeFormatted"`
	ModulesSize          int64  `json:"modulesSize"`
	ModulesSizeFormatted string `json:"modulesSizeFormatted"`
	SrcSize              int64  `json:"srcSize"`
	SrcSizeFormatted     string `json:"srcSizeFormatted"`
	// ImageSize is the /usr/lib/linux-image-<version> device-tree tree.
	ImageSize          int64  `json:"imageSize"`
	ImageSizeFormatted string `json:"imageSizeFormatted"`
	TotalSize          int64  `json:"totalSize"`
	TotalSizeFormatted string `json:"totalSizeFormatted"`
	HasModules         bool   `json:"hasModules"`
	HasSrc             bool   `json:"hasSrc"`
	HasImage           bool   `json:"hasImage"`
	// DpkgPackages lists the dpkg packages that still carry this kernel, so the
	// user can finish the job with an explicit apt purge. Removal here deletes
	// FILES only and never touches the package database, so a kernel removed by
	// this feature leaves a "half-installed" dpkg record behind; surfacing it is
	// the point, since the residue is otherwise silent.
	//
	// Absent means dpkg could not be queried — the frontend must not render
	// "no records" in that case. An empty (non-nil) array means dpkg answered
	// and there genuinely are none.
	DpkgPackages []string `json:"dpkgPackages"`
}

// RegisterKernelRoutes registers kernel-related routes under the given router group.
func RegisterKernelRoutes(rg *gin.RouterGroup) {
	rg.GET("/kernel/versions", middleware.ValidateToken, kernelVersionsHandler)
	rg.POST("/kernel/versions/cleanup", middleware.ValidateToken, middleware.RequireAdmin, middleware.ValidateCSRF, middleware.SensitiveActionRateLimit(5, 300000), kernelVersionCleanupHandler)
	rg.POST("/kernel/versions/:version/remove", middleware.ValidateToken, middleware.RequireAdmin, middleware.ValidateCSRF, middleware.SensitiveActionRateLimit(10, 300000), kernelVersionRemoveHandler)
}

// kernelVersionsHandler lists all installed kernel versions.
func kernelVersionsHandler(c *gin.Context) {
	currentKernel, err := getCurrentKernel()
	if err != nil {
		slog.Warn("failed to get current kernel", "error", err)
	}

	installed, err := getInstalledKernels(currentKernel)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"versions": []kernelVersion{},
			"error":    "无法读取内核版本信息: " + err.Error(),
		})
		return
	}

	totalSize := int64(0)
	unusedSize := int64(0)
	for _, v := range installed {
		totalSize += v.TotalSize
		if !v.IsCurrent {
			unusedSize += v.TotalSize
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"versions":            installed,
		"total":               len(installed),
		"current":             currentKernel,
		"unusedCount":         countNonCurrent(installed),
		"unusedSize":          unusedSize,
		"unusedSizeFormatted": formatBytes(unusedSize),
		"totalSize":           totalSize,
		"totalSizeFormatted":  formatBytes(totalSize),
	})
}

// kernelVersionCleanupHandler removes all non-current kernel versions.
func kernelVersionCleanupHandler(c *gin.Context) {
	currentKernel, err := getCurrentKernel()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "无法获取当前内核版本: " + err.Error()})
		return
	}

	installed, err := getInstalledKernels(currentKernel)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "无法读取内核版本信息: " + err.Error()})
		return
	}

	removed := 0
	freedSize := int64(0)
	var errors []string
	var dpkgLeftovers []string

	for _, v := range installed {
		if v.IsCurrent {
			continue
		}
		if err := removeKernelVersion(v.Version); err != nil {
			errors = append(errors, fmt.Sprintf("%s: %s", v.Version, err.Error()))
			continue
		}
		freedSize += v.TotalSize
		removed++
		// Files are gone but dpkg records are deliberately untouched; tell the
		// caller exactly how to finish, instead of leaving a silent residue.
		if hint := aptPurgeHint(v.Version, v.DpkgPackages); hint != "" {
			dpkgLeftovers = append(dpkgLeftovers, hint)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"removed":            removed,
		"total":              countNonCurrent(installed),
		"freedSize":          freedSize,
		"freedSizeFormatted": formatBytes(freedSize),
		"errors":             errors,
		"dpkgLeftovers":      dpkgLeftovers,
		"dpkgHint":           dpkgResidueHint(dpkgLeftovers),
	})
}

// kernelVersionRemoveHandler removes a specific kernel version.
func kernelVersionRemoveHandler(c *gin.Context) {
	version := c.Param("version")
	if version == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少内核版本"})
		return
	}

	currentKernel, err := getCurrentKernel()
	if err == nil && version == currentKernel {
		c.JSON(http.StatusBadRequest, gin.H{"error": "不能卸载当前正在使用的内核"})
		return
	}

	// Validate version string strictly: no path components, no "."/"..",
	// and the version must actually be installed before we touch the filesystem.
	if !isValidKernelVersion(version) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的内核版本号"})
		return
	}

	installed, err := getInstalledKernels(currentKernel)
	found := false
	if err == nil {
		for _, v := range installed {
			if v.Version == version {
				found = true
				break
			}
		}
	}
	if !found {
		c.JSON(http.StatusBadRequest, gin.H{"error": "内核版本不存在或未安装"})
		return
	}

	if err := removeKernelVersion(version); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("删除内核 %s 失败: %s", version, err.Error())})
		return
	}

	// Re-read to return updated state
	installed, _ = getInstalledKernels(currentKernel)

	// Report any dpkg records still carrying this kernel (files are gone, the
	// package database is untouched by design).
	var pkgs []string
	for _, v := range installed {
		if v.Version == version {
			pkgs = v.DpkgPackages
			break
		}
	}
	if pkgs == nil {
		pkgs = dpkgKernelPackagesFn(version)
	}
	var hintParts []string
	if h := aptPurgeHint(version, pkgs); h != "" {
		hintParts = append(hintParts, h)
	}

	c.JSON(http.StatusOK, gin.H{
		"success":       true,
		"message":       fmt.Sprintf("内核 %s 已删除", version),
		"versions":      installed,
		"dpkgLeftovers": pkgs,
		"dpkgHint":      dpkgResidueHint(hintParts),
	})
}

// getCurrentKernel returns the currently running kernel version via uname.
func getCurrentKernel() (string, error) {
	cmd := exec.Command("uname", "-r")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		// Fallback: try reading /proc/sys/kernel/osrelease
		data, err := os.ReadFile("/proc/sys/kernel/osrelease")
		if err != nil {
			return "", fmt.Errorf("无法获取当前内核版本: %w", err)
		}
		return strings.TrimSpace(string(data)), nil
	}
	return strings.TrimSpace(out.String()), nil
}

// getInstalledKernels discovers all installed kernel versions on the system.
// It scans /boot/ for vmlinuz-* files and /lib/modules/ for kernel module directories.
func getInstalledKernels(currentKernel string) ([]kernelVersion, error) {
	versionSet := make(map[string]bool)

	// Scan /boot/ for vmlinuz-* files
	bootEntries, err := os.ReadDir("/boot")
	if err == nil {
		for _, entry := range bootEntries {
			name := entry.Name()
			// Match vmlinuz-*, initrd.img-*, config-*, System.map-*
			if strings.HasPrefix(name, "vmlinuz-") {
				ver := strings.TrimPrefix(name, "vmlinuz-")
				versionSet[ver] = true
			} else if strings.HasPrefix(name, "initrd.img-") {
				ver := strings.TrimPrefix(name, "initrd.img-")
				versionSet[ver] = true
			}
		}
	}

	// Scan /lib/modules/ for kernel module directories
	modEntries, err := os.ReadDir("/lib/modules")
	if err == nil {
		for _, entry := range modEntries {
			if entry.IsDir() {
				versionSet[entry.Name()] = true
			}
		}
	}

	// Scan /usr/src/ for kernel header directories (e.g. linux-headers-6.12.41)
	srcEntries, err := os.ReadDir("/usr/src")
	if err == nil {
		for _, entry := range srcEntries {
			if !entry.IsDir() {
				continue
			}
			name := entry.Name()
			// Match common kernel header patterns:
			//   linux-headers-6.12.41, linux-headers-6.12.41-common
			//   c790, c821 (fnOS custom naming)
			for _, prefix := range []string{"linux-headers-", "linux-kbuild-", "linux-hwe-"} {
				if rest, ok := strings.CutPrefix(name, prefix); ok {
					versionSet[rest] = true
					break
				}
			}
			// Also add the directory name itself (handles c790, c821 etc.)
			if name != "." && name != ".." && !strings.HasPrefix(name, "linux-headers-") {
				versionSet[name] = true
			}
		}
	}

	// Scan /usr/lib/ for per-kernel device-tree (DTB) directories:
	//   /usr/lib/linux-image-6.18.18.c1090-trim
	//
	// This location must be scanned or an orphaned DTB directory left behind by
	// an earlier removal stays invisible forever: once the kernel's /boot files
	// and /lib/modules entry are gone, nothing else references the version, so
	// it would never appear in this list and never be offered for cleanup.
	libEntries, err := os.ReadDir(kernelImageDirRoot)
	if err == nil {
		for _, entry := range libEntries {
			if !entry.IsDir() {
				continue
			}
			if ver, ok := kernelImageVersion(entry.Name()); ok {
				versionSet[ver] = true
			}
		}
	}

	if len(versionSet) == 0 {
		return nil, fmt.Errorf("未发现已安装的内核")
	}

	var versions []kernelVersion
	for ver := range versionSet {
		isCurrent := (ver == currentKernel)

		// Calculate boot file size
		bootSize := calcBootSize(ver)

		// Calculate /lib/modules/ size
		modulesSize := calcDirSize(filepath.Join("/lib/modules", ver))
		hasModules := true
		if modulesSize == 0 {
			// Check if the directory exists at all
			if _, err := os.Stat(filepath.Join("/lib/modules", ver)); os.IsNotExist(err) {
				hasModules = false
			}
		}

		// Calculate /usr/src/ header size
		srcSize := calcSrcSize(ver)
		hasSrc := srcSize > 0

		// Calculate /usr/lib/linux-image-<ver>/ device-tree size
		imageSize := calcImageSize(ver)
		hasImage := imageSize > 0

		totalSize := bootSize + modulesSize + srcSize + imageSize

		versions = append(versions, kernelVersion{
			Version:              ver,
			IsCurrent:            isCurrent,
			BootSize:             bootSize,
			BootSizeFormatted:    formatBytes(bootSize),
			ModulesSize:          modulesSize,
			ModulesSizeFormatted: formatBytes(modulesSize),
			SrcSize:              srcSize,
			SrcSizeFormatted:     formatBytes(srcSize),
			ImageSize:            imageSize,
			ImageSizeFormatted:   formatBytes(imageSize),
			TotalSize:            totalSize,
			TotalSizeFormatted:   formatBytes(totalSize),
			HasModules:           hasModules,
			HasSrc:               hasSrc,
			HasImage:             hasImage,
			DpkgPackages:         dpkgKernelPackagesFn(ver),
		})
	}

	// Sort by version (newest first)
	sort.Slice(versions, func(i, j int) bool {
		return kernelCompareVersions(versions[i].Version, versions[j].Version) > 0
	})

	return versions, nil
}

// calcBootSize calculates total size of boot files for a kernel version.
func calcBootSize(version string) int64 {
	var total int64
	prefixes := []string{"vmlinuz-", "initrd.img-", "config-", "System.map-"}
	bootEntries, err := os.ReadDir("/boot")
	if err != nil {
		return 0
	}
	for _, entry := range bootEntries {
		name := entry.Name()
		for _, prefix := range prefixes {
			if strings.HasPrefix(name, prefix+version) || name == prefix+version {
				fi, err := entry.Info()
				if err == nil {
					total += fi.Size()
				}
				break
			}
		}
	}
	return total
}

// calcSrcSize calculates the total size of /usr/src headers for a kernel version.
// Looks for directories named after the version, e.g.:
//
//	/usr/src/linux-headers-6.12.41
//	/usr/src/c790
func calcSrcSize(version string) int64 {
	srcEntries, err := os.ReadDir("/usr/src")
	if err != nil {
		return 0
	}
	var total int64
	for _, entry := range srcEntries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		// Match linux-headers-<version> or exact version directory
		if name == version ||
			strings.HasSuffix(name, "-"+version) ||
			strings.HasSuffix(name, "-"+version+"-common") ||
			strings.HasPrefix(name, "linux-headers-"+version) ||
			strings.HasPrefix(name, "linux-kbuild-"+version) {
			total += calcDirSize(filepath.Join("/usr/src", name))
		}
	}
	return total
}

// calcImageSize calculates the size of the per-kernel device-tree directory
// under /usr/lib, e.g. /usr/lib/linux-image-6.18.18.c1090-trim.
//
// This path is the Debian arm64 kernel packaging convention and was reported by
// an on-device audit of this fnOS machine; it could not be re-verified from the
// development machine, because trim-cli only accepts /vol{v}/... paths and
// rejects /usr entirely. The code therefore treats it as "optional extra": if
// the directory is absent, the size is 0 and behaviour is exactly what it was
// before this location was considered. A wrong path degrades to the old
// behaviour; it cannot delete or misreport anything.
func calcImageSize(version string) int64 {
	path := filepath.Join(kernelImageDirRoot, kernelImageDirPrefix+version)
	if fi, err := os.Lstat(path); err != nil || !fi.IsDir() {
		return 0
	}
	return calcDirSize(path)
}

// calcDirSize recursively calculates the total size of a directory.
func calcDirSize(path string) int64 {
	var total int64
	filepath.Walk(path, func(_ string, fi os.FileInfo, err error) error {
		if err != nil {
			return nil // skip inaccessible files
		}
		if !fi.IsDir() {
			total += fi.Size()
		}
		return nil
	})
	return total
}

// kernelImageDirPrefix is the prefix of the per-kernel device-tree (DTB)
// directory shipped by the Debian kernel packaging on arm64:
//
//	/usr/lib/linux-image-6.18.18.c1090-trim/
//
// It is NOT under /boot, /lib/modules or /usr/src, which are the only three
// locations getInstalledKernels historically scanned. On a real fnOS device
// that let two orphaned DTB directories survive a manual kernel removal
// (36 MB total) while being invisible to this feature's own listing — so the
// cleanup reported success and the residue became a permanent blind spot.
const kernelImageDirPrefix = "linux-image-"

// kernelImageDirRoot is the directory holding per-kernel DTB trees.
const kernelImageDirRoot = "/usr/lib"

// kernelImageVersion extracts the kernel version from a
// "/usr/lib/linux-image-<version>" directory name. ok is false when name is
// not such a directory or the version part is not a valid kernel version.
//
// Kept pure (no filesystem access) so the naming rule is unit-testable on any
// platform; kernel handling itself is Linux-only and would otherwise go
// untested on the development machine.
func kernelImageVersion(name string) (version string, ok bool) {
	if !strings.HasPrefix(name, kernelImageDirPrefix) {
		return "", false
	}
	version = strings.TrimPrefix(name, kernelImageDirPrefix)
	if !isValidKernelVersion(version) {
		return "", false
	}
	return version, true
}

// dpkgResidueHint renders the human-facing explanation for leftover dpkg
// records. Returns "" when there is nothing to say.
func dpkgResidueHint(hints []string) string {
	if len(hints) == 0 {
		return ""
	}
	return "文件已删除，但 dpkg 包记录仍在（此功能只删文件、不经包管理器）。" +
		"如需彻底移除，请在设备上执行：" + strings.Join(hints, " ; ")
}

// kernelVersionPattern allows alphanumeric, dots, hyphens, underscores only.
var kernelVersionPattern = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

// dpkgKernelPackagesImpl asks dpkg which kernel packages are still recorded as
// installed for the given kernel version. It returns nil when dpkg cannot be
// queried (not installed, no permission, non-zero exit) so callers can
// distinguish "no records" from "unknown" — the same nil-vs-empty contract
// used by listOpenPaths in the space-cleanup scanner.
func dpkgKernelPackagesImpl(version string) []string {
	// -W = show, -f = format. Query the two package families the Debian kernel
	// packaging uses for a specific version.
	out, err := runDpkgQuery(
		"linux-image-"+version,
		"linux-headers-"+version,
		"linux-modules-trim-"+version,
		"linux-image-"+version+"-trim",
		"linux-headers-"+version+"-trim",
	)
	if err != nil {
		return nil
	}
	var pkgs []string
	for _, name := range out {
		if name != "" {
			pkgs = append(pkgs, name)
		}
	}
	if len(pkgs) == 0 {
		// Empty (not nil): dpkg answered and there are genuinely no records.
		return []string{}
	}
	return pkgs
}

// runDpkgQuery returns the subset of candidate package names that dpkg reports
// as installed. Returns an error when dpkg itself could not be run.
//
// Candidates are passed as argv (never through a shell) so a version string can
// never turn into shell syntax.
func runDpkgQuery(candidates ...string) ([]string, error) {
	if _, err := exec.LookPath("dpkg-query"); err != nil {
		return nil, err
	}
	var found []string
	for _, name := range candidates {
		cmd := exec.Command("dpkg-query", "-W", "-f", "${Status}", name)
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			// Not installed at all is a normal, expected outcome.
			continue
		}
		// Status looks like "install ok installed" or "install ok half-installed".
		if strings.Contains(stdout.String(), "installed") {
			found = append(found, name)
		}
	}
	return found, nil
}

// dpkgKernelPackagesFn is an injection seam around dpkgKernelPackagesImpl:
// kernel handling is Linux-only and dpkg is absent on the development machine,
// so tests substitute it to exercise the reporting logic on any platform.
var dpkgKernelPackagesFn = dpkgKernelPackagesImpl

// aptPurgeHint builds the exact command a user can run to finish the job this
// feature deliberately does not do: removing the dpkg records and any package
// files left behind. Returns "" when there are no records to purge.
func aptPurgeHint(version string, pkgs []string) string {
	if len(pkgs) == 0 {
		return ""
	}
	return "apt purge -y " + strings.Join(pkgs, " ")
}

// isValidKernelVersion reports whether version is safe to use in filesystem
// operations: it must be a single path component (no separators, not "."/"..")
// and must contain at least one digit, as real kernel versions always do.
func isValidKernelVersion(version string) bool {
	if version == "" || version == "." || version == ".." {
		return false
	}
	if !kernelVersionPattern.MatchString(version) {
		return false
	}
	if filepath.Base(version) != version || strings.ContainsAny(version, `/\\`) {
		return false
	}
	return strings.ContainsAny(version, "0123456789")
}

// removeKernelVersion removes all files associated with a kernel version.
func removeKernelVersion(version string) error {
	if !isValidKernelVersion(version) {
		return fmt.Errorf("无效的内核版本号")
	}

	var errs []string

	// Remove boot files
	bootEntries, err := os.ReadDir("/boot")
	if err == nil {
		for _, entry := range bootEntries {
			name := entry.Name()
			// Match any kernel-related boot file for this version
			if strings.HasSuffix(name, "-"+version) || strings.Contains(name, "-"+version+"-") {
				path := filepath.Join("/boot", name)
				if err := os.Remove(path); err != nil {
					errs = append(errs, fmt.Sprintf("删除 %s 失败: %s", path, err.Error()))
				}
			}
		}
	}

	// Remove /lib/modules/<version>/ (Lstat: never follow symlinks)
	modPath := filepath.Join("/lib/modules", version)
	if fi, err := os.Lstat(modPath); err == nil && fi.IsDir() {
		if err := os.RemoveAll(modPath); err != nil {
			errs = append(errs, fmt.Sprintf("删除 %s 失败: %s", modPath, err.Error()))
		}
	}

	// Remove /usr/src/ headers for this version
	srcEntries, err := os.ReadDir("/usr/src")
	if err == nil {
		for _, entry := range srcEntries {
			if !entry.IsDir() {
				continue
			}
			name := entry.Name()
			match := name == version ||
				strings.HasSuffix(name, "-"+version) ||
				strings.HasSuffix(name, "-"+version+"-common") ||
				strings.HasPrefix(name, "linux-headers-"+version) ||
				strings.HasPrefix(name, "linux-kbuild-"+version)
			if match {
				srcPath := filepath.Join("/usr/src", name)
				if err := os.RemoveAll(srcPath); err != nil {
					errs = append(errs, fmt.Sprintf("删除 %s 失败: %s", srcPath, err.Error()))
				}
			}
		}
	}

	// Remove /usr/lib/linux-image-<version>/ device-tree directory.
	//
	// Omitting this is what left 36 MB orphaned on a real fnOS device: the
	// kernel's boot files and modules were gone but this tree survived, and it
	// was invisible to getInstalledKernels so nothing ever offered it again.
	//
	// Guarded by Lstat + IsDir, so a non-existent path is a silent no-op.
	imagePath := filepath.Join(kernelImageDirRoot, kernelImageDirPrefix+version)
	if fi, err := os.Lstat(imagePath); err == nil && fi.IsDir() {
		if err := os.RemoveAll(imagePath); err != nil {
			errs = append(errs, fmt.Sprintf("删除 %s 失败: %s", imagePath, err.Error()))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	return nil
}

// countNonCurrent counts non-current kernel versions.
func countNonCurrent(versions []kernelVersion) int {
	count := 0
	for _, v := range versions {
		if !v.IsCurrent {
			count++
		}
	}
	return count
}

// compareVersions compares two kernel version strings, returning >0 if a > b.
// Handles common kernel version formats like "6.18.0", "6.12.0-amd64", etc.
func kernelCompareVersions(a, b string) int {
	// Strip architecture suffix for comparison
	cleanA := cleanupVersion(a)
	cleanB := cleanupVersion(b)

	// Split into parts by dots
	partsA := strings.Split(cleanA, ".")
	partsB := strings.Split(cleanB, ".")

	maxLen := len(partsA)
	if len(partsB) > maxLen {
		maxLen = len(partsB)
	}

	for i := 0; i < maxLen; i++ {
		var numA, numB int
		if i < len(partsA) {
			fmt.Sscanf(partsA[i], "%d", &numA)
		}
		if i < len(partsB) {
			fmt.Sscanf(partsB[i], "%d", &numB)
		}
		if numA != numB {
			return numA - numB
		}
	}
	return 0
}

// cleanupVersion strips architecture/distro suffixes for version comparison.
// e.g. "6.18.0-amd64" → "6.18.0", "6.12.0-2-pve" → "6.12.0"
func cleanupVersion(v string) string {
	// Remove trailing distro-specific suffixes
	parts := strings.SplitN(v, "-", 3)
	if len(parts) >= 2 {
		// Keep only the numeric version and maybe first suffix
		baseParts := strings.Split(v, "-")
		result := baseParts[0]
		for i := 1; i < len(baseParts); i++ {
			part := baseParts[i]
			// If this part starts with a digit, it's part of the version (e.g. "6.12.0-2-pve")
			if len(part) > 0 && part[0] >= '0' && part[0] <= '9' {
				result += "." + part
			} else {
				break
			}
		}
		return result
	}
	return v
}

// formatBytes converts bytes to human-readable string.
func formatBytes(bytes int64) string {
	if bytes == 0 {
		return "0 B"
	}
	units := []string{"B", "KB", "MB", "GB"}
	i := 0
	size := float64(bytes)
	for size >= 1024 && i < len(units)-1 {
		size /= 1024
		i++
	}
	if i == 0 {
		return fmt.Sprintf("%.0f %s", size, units[i])
	}
	return fmt.Sprintf("%.1f %s", size, units[i])
}
