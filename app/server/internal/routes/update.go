package routes

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sushazhi/fnos-logmanager/internal/config"
	"github.com/sushazhi/fnos-logmanager/internal/middleware"
	"github.com/sushazhi/fnos-logmanager/internal/services"
	"github.com/sushazhi/fnos-logmanager/internal/utils"
)

const (
	githubRepo  = "sushazhi/fnos-logmanager"
	githubAPI   = "https://api.github.com"
	binaryProxy = "https://ghfast.top/"
	rawProxyURL = "https://ghfast.top/https://raw.githubusercontent.com/sushazhi/fnos-logmanager/main/version.json"
)

var allowedGitHubHosts = []string{
	"github.com",
	"api.github.com",
	"raw.githubusercontent.com",
	"objects.githubusercontent.com",
	"ghfast.top",
}

type updateState struct {
	mu             sync.RWMutex
	ready          string
	updating       bool
	updateProgress int
	updateMessage  string
}

var globalUpdateState = &updateState{
	ready:          "true",
	updating:       false,
	updateProgress: 0,
	updateMessage:  "",
}

type cachedCheck struct {
	expiresAt time.Time
	payload   map[string]interface{}
}

var (
	checkCache   *cachedCheck
	checkCacheMu sync.Mutex
)

// RegisterUpdateRoutes registers update routes under the given router group.
func RegisterUpdateRoutes(rg *gin.RouterGroup) {
	rg.GET("/version", middleware.ValidateToken, updateVersionHandler)
	rg.GET("/check", middleware.ValidateToken, updateCheckHandler)
	rg.POST("/install", middleware.ValidateToken, middleware.RequireAdmin, middleware.ValidateCSRF, middleware.SensitiveActionRateLimit(1, 600000), updateInstallHandler)
	rg.GET("/status", middleware.ValidateToken, updateStatusHandler)
}

// ---------- Version Handler ----------

func updateVersionHandler(c *gin.Context) {
	currentVersion := os.Getenv("TRIM_APPVER")
	if currentVersion == "" {
		currentVersion = "0.0.0"
	}

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"version":  currentVersion,
		"appName":  "飞牛日志管理",
		"platform": os.Getenv("TRIM_ARCH"),
	})
}

// ---------- Check Handler ----------

type gitHubReleaseAPI struct {
	TagName     string           `json:"tag_name"`
	Body        string           `json:"body"`
	PublishedAt string           `json:"published_at"`
	Assets      []gitHubAssetAPI `json:"assets"`
}

type gitHubAssetAPI struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               *int64 `json:"size,omitempty"`
}

type versionJSON struct {
	Version string `json:"version"`
}

func updateCheckHandler(c *gin.Context) {
	checkCacheMu.Lock()
	if checkCache != nil && time.Now().Before(checkCache.expiresAt) {
		payload := checkCache.payload
		checkCacheMu.Unlock()
		c.JSON(http.StatusOK, payload)
		return
	}
	checkCacheMu.Unlock()

	currentVersion := os.Getenv("TRIM_APPVER")
	if currentVersion == "" {
		currentVersion = "0.0.0"
	}

	latestRelease, usedFallback, err := fetchLatestRelease()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "检查更新失败: " + err.Error(),
		})
		return
	}

	hasUpdate := compareVersions(latestRelease.Version, currentVersion) > 0

	// Proxy mode uses shorter cache
	cacheMs := config.Get().Update.CheckCacheMs
	if usedFallback {
		if cacheMs > 120000 {
			cacheMs = 120000
		}
	}

	payload := map[string]interface{}{
		"success":        true,
		"currentVersion": currentVersion,
		"latestVersion":  latestRelease.Version,
		"hasUpdate":      hasUpdate,
		"changelog":      latestRelease.Changelog,
		"publishedAt":    latestRelease.PublishedAt,
		"message":        "已是最新版本",
	}
	if hasUpdate {
		payload["message"] = "发现新版本"
	}

	checkCacheMu.Lock()
	checkCache = &cachedCheck{
		expiresAt: time.Now().Add(time.Duration(cacheMs) * time.Millisecond),
		payload:   payload,
	}
	checkCacheMu.Unlock()

	c.JSON(http.StatusOK, payload)
}

type releaseInfo struct {
	Version     string
	Changelog   string
	PublishedAt string
	Assets      []configAsset
}

type configAsset struct {
	Name               string
	BrowserDownloadURL string
	Size               *int64
}

func fetchLatestRelease() (*releaseInfo, bool, error) {
	// Try GitHub API first
	release, err := fetchFromGitHubAPI()
	if err == nil {
		return release, false, nil
	}

	slog.Warn("GitHub API check failed, falling back to proxy", "error", err)

	// Fallback to proxy version.json
	proxyRelease, proxyErr := fetchFromProxy()
	if proxyErr != nil {
		return nil, false, fmt.Errorf("GitHub API 和代理均不可用: %v / %v", err, proxyErr)
	}

	return proxyRelease, true, nil
}

func fetchFromGitHubAPI() (*releaseInfo, error) {
	url := fmt.Sprintf("%s/repos/%s/releases/latest", githubAPI, githubRepo)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "fnos-logmanager-updater")
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API 返回 %d", resp.StatusCode)
	}

	var ghRelease gitHubReleaseAPI
	if err := json.NewDecoder(resp.Body).Decode(&ghRelease); err != nil {
		return nil, err
	}

	version := strings.TrimPrefix(ghRelease.TagName, "v")

	assets := make([]configAsset, 0, len(ghRelease.Assets))
	for _, a := range ghRelease.Assets {
		assets = append(assets, configAsset{
			Name:               a.Name,
			BrowserDownloadURL: a.BrowserDownloadURL,
			Size:               a.Size,
		})
	}

	return &releaseInfo{
		Version:     version,
		Changelog:   ghRelease.Body,
		PublishedAt: ghRelease.PublishedAt,
		Assets:      assets,
	}, nil
}

func fetchFromProxy() (*releaseInfo, error) {
	if !isHTTPSURL(rawProxyURL) || !isAllowedHost(rawProxyURL) {
		return nil, fmt.Errorf("版本检查代理链接不安全")
	}

	client := &http.Client{
		Timeout: 15 * time.Second,
	}

	resp, err := client.Get(rawProxyURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("代理返回 %d", resp.StatusCode)
	}

	var vj versionJSON
	if err := json.NewDecoder(resp.Body).Decode(&vj); err != nil {
		return nil, err
	}

	if vj.Version == "" {
		return nil, fmt.Errorf("version.json 格式无效")
	}

	return &releaseInfo{
		Version: vj.Version,
	}, nil
}

// ---------- Install Handler ----------

func updateInstallHandler(c *gin.Context) {
	globalUpdateState.mu.Lock()
	if globalUpdateState.updating {
		globalUpdateState.mu.Unlock()
		c.JSON(http.StatusConflict, gin.H{
			"success": false,
			"error":   "正在更新中，请稍候",
		})
		return
	}
	globalUpdateState.updating = true
	globalUpdateState.updateProgress = 0
	globalUpdateState.updateMessage = "准备更新..."
	globalUpdateState.mu.Unlock()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "开始下载更新，请稍候...",
	})

	// Run update in background (recovered: a panic must not kill the server)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("panic in performUpdate", "recover", r)
				setUpdateError(fmt.Sprintf("内部错误: %v", r))
			}
		}()
		performUpdate()
	}()
}

func performUpdate() {
	globalUpdateState.mu.Lock()
	globalUpdateState.updateMessage = "正在获取更新信息..."
	globalUpdateState.updateProgress = 5
	globalUpdateState.mu.Unlock()

	releaseInfo, err := fetchFromGitHubAPI()
	if err != nil {
		setUpdateError("获取更新信息失败: " + err.Error())
		return
	}

	globalUpdateState.mu.Lock()
	globalUpdateState.updateMessage = "正在查找更新包..."
	globalUpdateState.updateProgress = 10
	globalUpdateState.mu.Unlock()

	// Find .fpk asset
	var asset *configAsset
	for _, a := range releaseInfo.Assets {
		if strings.HasSuffix(a.Name, ".fpk") && strings.Contains(a.Name, "logmanager") && isValidGitHubAsset(a) {
			asset = &a
			break
		}
	}

	if asset == nil {
		setUpdateError("未找到有效的更新包")
		return
	}

	if !utils.IsValidGitHubURL(asset.BrowserDownloadURL) {
		setUpdateError("更新包来源不安全")
		return
	}

	if !isHTTPSURL(asset.BrowserDownloadURL) || !isAllowedHost(asset.BrowserDownloadURL) {
		setUpdateError("更新包链接不安全")
		return
	}

	maxAssetBytes := config.Get().Update.MaxAssetBytes
	if asset.Size != nil && *asset.Size > maxAssetBytes {
		setUpdateError("更新包大小超过限制")
		return
	}

	globalUpdateState.mu.Lock()
	globalUpdateState.updateMessage = "正在准备更新目录..."
	globalUpdateState.updateProgress = 15
	globalUpdateState.mu.Unlock()

	vol := os.Getenv("TRIM_APPDEST_VOL")
	if vol == "" {
		vol = "/vol1"
	}
	appName := os.Getenv("TRIM_APPNAME")
	if appName == "" {
		appName = "logmanager"
	}
	updateDir := filepath.Join(vol, "@appshare", appName, "update")

	if !ensureSafeUpdateDir(updateDir) {
		setUpdateError("更新目录不安全")
		return
	}

	if err := os.MkdirAll(updateDir, 0755); err != nil {
		setUpdateError("创建更新目录失败: " + err.Error())
		return
	}

	fpkFile := filepath.Join(updateDir, "logmanager.tar")
	configFile := filepath.Join(updateDir, "config.env")

	globalUpdateState.mu.Lock()
	globalUpdateState.updateMessage = "正在下载更新包..."
	globalUpdateState.updateProgress = 20
	globalUpdateState.mu.Unlock()

	downloadURL := binaryProxy + asset.BrowserDownloadURL
	if !isHTTPSURL(downloadURL) || !isAllowedHost(downloadURL) {
		setUpdateError("更新代理链接不安全")
		return
	}

	if err := downloadFile(downloadURL, fpkFile, func(progress float64) {
		globalUpdateState.mu.Lock()
		globalUpdateState.updateProgress = 20 + int(progress*40)
		globalUpdateState.mu.Unlock()
	}); err != nil {
		setUpdateError("下载更新包失败: " + err.Error())
		return
	}

	// Verify downloaded file
	stat, err := os.Stat(fpkFile)
	if err != nil {
		setUpdateError("无法读取下载的文件")
		return
	}
	if stat.Size() == 0 {
		os.Remove(fpkFile)
		setUpdateError("下载的文件为空")
		return
	}
	maxDownloadBytes := config.Get().Update.MaxDownloadBytes
	if stat.Size() > maxDownloadBytes {
		os.Remove(fpkFile)
		setUpdateError("下载的文件过大")
		return
	}
	if asset.Size != nil {
		diff := stat.Size() - *asset.Size
		if diff < 0 {
			diff = -diff
		}
		if diff > 1024 {
			os.Remove(fpkFile)
			setUpdateError(fmt.Sprintf("文件大小不匹配：期望 %d 字节，实际 %d 字节", *asset.Size, stat.Size()))
			return
		}
	}

	// Verify SHA256 integrity when the release provides a checksum asset.
	// A mismatched checksum aborts the update; absence only warns (for
	// backward compatibility with older releases).
	if err := verifyUpdateChecksum(releaseInfo, asset, fpkFile, updateDir); err != nil {
		os.Remove(fpkFile)
		setUpdateError("更新包完整性校验失败: " + err.Error())
		return
	}

	globalUpdateState.mu.Lock()
	globalUpdateState.updateMessage = "正在创建配置文件..."
	globalUpdateState.updateProgress = 65
	globalUpdateState.mu.Unlock()

	port := os.Getenv("LOGMANAGER_PORT")
	if port == "" {
		port = fmt.Sprintf("%d", config.Get().Port)
	}
	configContent := fmt.Sprintf(`# 更新配置文件
wizard_app_port=%s
wizard_data_action=keep
wizard_backup_before_upgrade=yes
`, port)
	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		setUpdateError("创建配置文件失败: " + err.Error())
		return
	}

	globalUpdateState.mu.Lock()
	globalUpdateState.updateMessage = "正在解压更新包..."
	globalUpdateState.updateProgress = 70
	globalUpdateState.mu.Unlock()

	if err := extractTar(fpkFile, updateDir); err != nil {
		setUpdateError("解压失败: " + err.Error())
		return
	}

	// Clean up fpk file
	os.Remove(fpkFile)

	globalUpdateState.mu.Lock()
	globalUpdateState.updateMessage = "正在安装更新..."
	globalUpdateState.updateProgress = 80
	globalUpdateState.mu.Unlock()

	// Detect volume
	volumeNum := "1"
	normalizedDir := filepath.ToSlash(updateDir)
	re := regexp.MustCompile(`/vol(\d+)/`)
	if matches := re.FindStringSubmatch(normalizedDir); len(matches) > 1 {
		volumeNum = matches[1]
	}

	// Set default volume (non-fatal if fails)
	_ = runCommand("appcenter-cli", []string{"default-volume", volumeNum}, updateDir, 60000)

	// Install
	installArgs := []string{"install-local", "--env", "config.env", "--volume", volumeNum}
	if err := runCommandDetached("appcenter-cli", installArgs, updateDir); err != nil {
		slog.Warn("update install command failed (may be non-fatal)", "error", err)
	}

	globalUpdateState.mu.Lock()
	globalUpdateState.updateMessage = "更新完成，应用将重启..."
	globalUpdateState.updateProgress = 100
	globalUpdateState.updating = false
	globalUpdateState.mu.Unlock()

	services.AddSecurityAuditLog("APP_UPDATED", map[string]interface{}{
		"fromVersion": os.Getenv("TRIM_APPVER"),
		"toVersion":   releaseInfo.Version,
	}, nil)
}

func setUpdateError(message string) {
	globalUpdateState.mu.Lock()
	globalUpdateState.updating = false
	globalUpdateState.updateMessage = "更新失败: " + message
	globalUpdateState.updateProgress = 0
	globalUpdateState.mu.Unlock()

	services.AddSecurityAuditLog("UPDATE_FAILED", map[string]interface{}{
		"error": message,
	}, nil)

	slog.Error("update failed", "error", message)
}

func ensureSafeUpdateDir(updateDir string) bool {
	normalized := filepath.ToSlash(updateDir)
	allowedDirs := config.Get().Update.AllowedUpdateDirs
	for _, dir := range allowedDirs {
		normalizedDir := filepath.ToSlash(dir)
		if strings.HasPrefix(normalized, normalizedDir+"/") || normalized == normalizedDir {
			return true
		}
	}
	return false
}

// maxExtractBytes caps the total bytes extracted from an update package to
// guard against decompression bombs.
const maxExtractBytes = 1 << 30 // 1 GiB

// extractTar safely extracts a (optionally gzipped) tar archive into targetDir
// using Go's archive/tar with strict per-member path validation. It replaces
// the previous `tar -xf` shell-out, whose traversal protection depends on the
// system tar implementation.
func extractTar(fpkFile, targetDir string) error {
	f, err := os.Open(fpkFile)
	if err != nil {
		return err
	}
	defer f.Close()

	br := bufio.NewReader(f)
	magic, _ := br.Peek(2)
	var tr *tar.Reader
	if len(magic) == 2 && magic[0] == 0x1f && magic[1] == 0x8b {
		gz, err := gzip.NewReader(br)
		if err != nil {
			return fmt.Errorf("解压 gzip 失败: %w", err)
		}
		defer gz.Close()
		tr = tar.NewReader(gz)
	} else {
		tr = tar.NewReader(br)
	}
	cleanTarget := filepath.Clean(targetDir)
	withinTarget := func(p string) bool {
		return p == cleanTarget || strings.HasPrefix(p, cleanTarget+string(os.PathSeparator))
	}

	var total int64
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("读取 tar 失败: %w", err)
		}

		name := filepath.Clean(hdr.Name)
		if filepath.IsAbs(name) || name == ".." || strings.HasPrefix(name, ".."+string(os.PathSeparator)) {
			return fmt.Errorf("tar 包含非法路径: %s", hdr.Name)
		}
		dest := filepath.Join(cleanTarget, name)
		if !withinTarget(dest) {
			return fmt.Errorf("tar 路径越界: %s", hdr.Name)
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(dest, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			total += hdr.Size
			if total > maxExtractBytes {
				return fmt.Errorf("解压内容超过大小限制")
			}
			if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
				return err
			}
			out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode)&0755)
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return err
			}
			if err := out.Close(); err != nil {
				return err
			}
		case tar.TypeSymlink:
			// Allow symlinks only when the resolved target stays inside targetDir.
			if filepath.IsAbs(hdr.Linkname) {
				return fmt.Errorf("tar 符号链接越界: %s -> %s", hdr.Name, hdr.Linkname)
			}
			resolved := filepath.Clean(filepath.Join(filepath.Dir(dest), hdr.Linkname))
			if !withinTarget(resolved) {
				return fmt.Errorf("tar 符号链接越界: %s -> %s", hdr.Name, hdr.Linkname)
			}
			os.Remove(dest)
			if err := os.Symlink(hdr.Linkname, dest); err != nil {
				return fmt.Errorf("创建符号链接失败: %w", err)
			}
		default:
			// Skip hard links, devices, fifos etc. — not needed for updates.
		}
	}
	return nil
}

func runCommand(command string, args []string, workDir string, timeoutMs int64) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutMs)*time.Millisecond)
	defer cancel()

	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Dir = workDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		slog.Debug("command failed (may be non-fatal)", "command", command, "args", args, "output", string(output), "error", err)
	}
	return err
}

func runCommandDetached(command string, args []string, workDir string) error {
	cmd := exec.Command(command, args...)
	cmd.Dir = workDir

	if err := cmd.Start(); err != nil {
		return err
	}

	// Let the process run independently - wait in background
	go cmd.Wait()

	return nil
}

// ---------- Status Handler ----------

func updateStatusHandler(c *gin.Context) {
	globalUpdateState.mu.RLock()
	status := gin.H{
		"success":        true,
		"ready":          globalUpdateState.ready,
		"updating":       globalUpdateState.updating,
		"updateProgress": globalUpdateState.updateProgress,
		"updateMessage":  globalUpdateState.updateMessage,
	}
	globalUpdateState.mu.RUnlock()

	c.JSON(http.StatusOK, status)
}

// ---------- Utility Functions ----------

func compareVersions(v1, v2 string) int {
	parts1 := parseVersion(v1)
	parts2 := parseVersion(v2)

	maxLen := len(parts1)
	if len(parts2) > maxLen {
		maxLen = len(parts2)
	}

	for i := 0; i < maxLen; i++ {
		var num1, num2 int
		if i < len(parts1) {
			num1 = parts1[i]
		}
		if i < len(parts2) {
			num2 = parts2[i]
		}
		if num1 > num2 {
			return 1
		}
		if num1 < num2 {
			return -1
		}
	}
	return 0
}

func parseVersion(v string) []int {
	v = strings.TrimLeft(v, "vV")
	parts := strings.Split(v, ".")
	nums := make([]int, 0, len(parts))
	for _, p := range parts {
		n := 0
		fmt.Sscanf(p, "%d", &n)
		nums = append(nums, n)
	}
	return nums
}

func isValidGitHubAsset(asset configAsset) bool {
	if asset.Name == "" || asset.BrowserDownloadURL == "" {
		return false
	}
	return utils.IsValidGitHubURL(asset.BrowserDownloadURL)
}

func isHTTPSURL(rawURL string) bool {
	if !utils.IsValidURL(rawURL) {
		return false
	}
	return strings.HasPrefix(rawURL, "https://")
}

func isAllowedHost(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil || u.Scheme != "https" {
		return false
	}
	host := strings.ToLower(u.Hostname())
	for _, allowed := range allowedGitHubHosts {
		if host == allowed || strings.HasSuffix(host, "."+allowed) {
			return true
		}
	}
	return false
}

// findChecksumAsset locates a checksum asset for the given fpk asset in the
// release: either "<asset>.sha256" or a shared "checksums.txt"/"SHA256SUMS".
func findChecksumAsset(info *releaseInfo, asset *configAsset) *configAsset {
	for i, a := range info.Assets {
		if a.Name == asset.Name+".sha256" {
			return &info.Assets[i]
		}
	}
	for i, a := range info.Assets {
		if a.Name == "checksums.txt" || a.Name == "SHA256SUMS" || a.Name == "sha256sums.txt" {
			return &info.Assets[i]
		}
	}
	return nil
}

// expectedSHA256For parses checksum file content ("<hash>  <filename>" lines)
// and returns the hash for the given file name.
func expectedSHA256For(content, fileName string) string {
	for _, line := range strings.Split(content, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && strings.TrimPrefix(fields[1], "*") == fileName {
			return fields[0]
		}
		// Single-hash .sha256 file without filename
		if len(fields) == 1 && len(fields[0]) == 64 {
			return fields[0]
		}
	}
	return ""
}

// verifyUpdateChecksum downloads the release checksum asset (if any) and
// verifies the downloaded fpk file's SHA256 against it.
func verifyUpdateChecksum(info *releaseInfo, asset *configAsset, fpkFile, updateDir string) error {
	checksumAsset := findChecksumAsset(info, asset)
	if checksumAsset == nil {
		slog.Warn("release 未提供 SHA256 校验文件，跳过完整性校验", "asset", asset.Name)
		return nil
	}

	checksumFile := filepath.Join(updateDir, "update.checksums")
	defer os.Remove(checksumFile)

	downloadURL := binaryProxy + checksumAsset.BrowserDownloadURL
	if !isHTTPSURL(downloadURL) || !isAllowedHost(downloadURL) {
		return fmt.Errorf("校验文件链接不安全")
	}
	if err := downloadFile(downloadURL, checksumFile, nil); err != nil {
		return fmt.Errorf("下载校验文件失败: %w", err)
	}

	data, err := os.ReadFile(checksumFile)
	if err != nil {
		return err
	}
	expected := expectedSHA256For(string(data), asset.Name)
	if expected == "" {
		return fmt.Errorf("校验文件中未找到 %s 的哈希", asset.Name)
	}

	f, err := os.Open(fpkFile)
	if err != nil {
		return err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	actual := hex.EncodeToString(h.Sum(nil))
	if !strings.EqualFold(actual, strings.TrimSpace(expected)) {
		return fmt.Errorf("SHA256 不匹配（期望 %s，实际 %s）", expected, actual)
	}
	slog.Info("更新包 SHA256 校验通过", "asset", asset.Name)
	return nil
}

func downloadFile(url, dest string, onProgress func(float64)) error {
	if !isHTTPSURL(url) {
		return fmt.Errorf("仅允许HTTPS下载")
	}
	if !isAllowedHost(url) {
		return fmt.Errorf("下载地址不在允许列表")
	}

	client := &http.Client{
		Timeout: time.Duration(config.Get().Update.DownloadTimeoutMs) * time.Millisecond,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) > config.Get().Update.MaxRedirects {
				return fmt.Errorf("重定向次数过多")
			}
			if !isHTTPSURL(req.URL.String()) || !isAllowedHost(req.URL.String()) {
				return fmt.Errorf("重定向地址不安全")
			}
			return nil
		},
	}

	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("下载失败: HTTP %d", resp.StatusCode)
	}

	totalSize := resp.ContentLength
	maxDownloadBytes := config.Get().Update.MaxDownloadBytes
	if totalSize > 0 && totalSize > maxDownloadBytes {
		return fmt.Errorf("下载内容超过大小限制")
	}

	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()

	var downloaded int64
	buf := make([]byte, 32*1024)
	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, writeErr := f.Write(buf[:n]); writeErr != nil {
				return writeErr
			}
			downloaded += int64(n)

			if downloaded > maxDownloadBytes {
				f.Close()
				os.Remove(dest)
				return fmt.Errorf("下载内容超过大小限制")
			}

			if totalSize > 0 && onProgress != nil {
				onProgress(float64(downloaded) / float64(totalSize))
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return readErr
		}
	}

	return nil
}
