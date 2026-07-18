package routes

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
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
	githubRepo   = "sushazhi/fnos-logmanager"
	githubAPI    = "https://api.github.com"
	binaryProxy  = "https://ghfast.top/"
	rawProxyURL  = "https://ghfast.top/https://raw.githubusercontent.com/sushazhi/fnos-logmanager/main/version.json"
)

var allowedGitHubHosts = []string{
	"github.com",
	"api.github.com",
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
	checkCache     *cachedCheck
	checkCacheMu   sync.Mutex
)

// RegisterUpdateRoutes registers update routes under the given router group.
func RegisterUpdateRoutes(rg *gin.RouterGroup) {
	rg.GET("/version", middleware.ValidateToken, updateVersionHandler)
	rg.GET("/check", middleware.ValidateToken, updateCheckHandler)
	rg.POST("/install", middleware.ValidateToken, middleware.ValidateCSRF, middleware.SensitiveActionRateLimit(1, 600000), updateInstallHandler)
	rg.GET("/status", middleware.ValidateToken, updateStatusHandler)
}

// ---------- Version Handler ----------

func updateVersionHandler(c *gin.Context) {
	currentVersion := os.Getenv("TRIM_APPVER")
	if currentVersion == "" {
		currentVersion = "0.0.0"
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"version": currentVersion,
		"appName": "飞牛日志管理",
		"platform": os.Getenv("TRIM_ARCH"),
	})
}

// ---------- Check Handler ----------

type gitHubReleaseAPI struct {
	TagName    string             `json:"tag_name"`
	Body       string             `json:"body"`
	PublishedAt string            `json:"published_at"`
	Assets     []gitHubAssetAPI   `json:"assets"`
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
		"success":       true,
		"currentVersion": currentVersion,
		"latestVersion": latestRelease.Version,
		"hasUpdate":     hasUpdate,
		"changelog":     latestRelease.Changelog,
		"publishedAt":   latestRelease.PublishedAt,
		"message":       "已是最新版本",
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

	// Run update in background
	go performUpdate()
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

func extractTar(fpkFile, targetDir string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "tar", "-xf", fpkFile, "-C", targetDir)

	var stderrBuf strings.Builder
	cmd.Stderr = &stderrBuf

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("tar 解压失败: %s", strings.TrimSpace(stderrBuf.String()))
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
	// For raw URL strings, just check if it starts with the proxy URL or
	// matches an allowed host.
	// Simple string-level check for the proxy prefix
	if strings.HasPrefix(rawURL, binaryProxy) {
		return true
	}

	// Check GitHub API
	if strings.HasPrefix(rawURL, githubAPI) {
		return true
	}

	// Check raw.githubusercontent.com (proxy fallback)
	if strings.HasPrefix(rawURL, rawProxyURL) || strings.HasPrefix(rawURL, "https://raw.githubusercontent.com") {
		return true
	}

	// For URLs from GitHub API, they should start with allowed hosts
	for _, host := range allowedGitHubHosts {
		if strings.Contains(rawURL, host) {
			return true
		}
	}

	return false
}

func downloadFile(url, dest string, onProgress func(float64)) error {
	if !isHTTPSURL(url) {
		return fmt.Errorf("仅允许HTTPS下载")
	}
	if !isAllowedHost(url) {
		return fmt.Errorf("下载地址不在允许列表")
	}

	client := &http.Client{
		Timeout:   time.Duration(config.Get().Update.DownloadTimeoutMs) * time.Millisecond,
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
