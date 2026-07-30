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
	Version    string `json:"version"`
	IsCurrent  bool   `json:"isCurrent"`
	BootSize   int64  `json:"bootSize"`
	BootSizeFormatted string `json:"bootSizeFormatted"`
	ModulesSize int64  `json:"modulesSize"`
	ModulesSizeFormatted string `json:"modulesSizeFormatted"`
	TotalSize  int64  `json:"totalSize"`
	TotalSizeFormatted string `json:"totalSizeFormatted"`
	HasModules bool   `json:"hasModules"`
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
		"versions":           installed,
		"total":              len(installed),
		"current":            currentKernel,
		"unusedCount":        countNonCurrent(installed),
		"unusedSize":         unusedSize,
		"unusedSizeFormatted": formatBytes(unusedSize),
		"totalSize":          totalSize,
		"totalSizeFormatted": formatBytes(totalSize),
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
	}

	c.JSON(http.StatusOK, gin.H{
		"removed":            removed,
		"total":              countNonCurrent(installed),
		"freedSize":          freedSize,
		"freedSizeFormatted": formatBytes(freedSize),
		"errors":             errors,
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
	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"message":  fmt.Sprintf("内核 %s 已删除", version),
		"versions": installed,
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

		versions = append(versions, kernelVersion{
			Version:    ver,
			IsCurrent:  isCurrent,
			BootSize:   bootSize,
			BootSizeFormatted: formatBytes(bootSize),
			ModulesSize: modulesSize,
			ModulesSizeFormatted: formatBytes(modulesSize),
			TotalSize:  bootSize + modulesSize,
			TotalSizeFormatted: formatBytes(bootSize + modulesSize),
			HasModules: hasModules,
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

// kernelVersionPattern allows alphanumeric, dots, hyphens, underscores only.
var kernelVersionPattern = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

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
