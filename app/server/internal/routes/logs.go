package routes

import (
	"bufio"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sushazhi/fnos-logmanager/internal/config"
	"github.com/sushazhi/fnos-logmanager/internal/middleware"
	"github.com/sushazhi/fnos-logmanager/internal/services"
	"github.com/sushazhi/fnos-logmanager/internal/types"
	"github.com/sushazhi/fnos-logmanager/internal/utils"
)

const (
	maxPathLength    = 4096
	maxPatternLength = 100
)

// RegisterLogRoutes registers all log management routes on the given router group.
func RegisterLogRoutes(rg *gin.RouterGroup) {
	// Filter settings
	rg.GET("/settings/filter", middleware.ValidateToken, getFilterSettingsHandler)
	rg.POST("/settings/filter", middleware.ValidateToken, middleware.ValidateCSRF, setFilterSettingsHandler)

	// Directories and app names
	rg.GET("/dirs", middleware.ValidateToken, getDirsHandler)
	rg.GET("/appnames", middleware.ValidateToken, getAppNamesHandler)

	// Log listing and search
	rg.GET("/logs/list", middleware.ValidateToken, listLogsHandler)
	rg.GET("/logs/large", middleware.ValidateToken, listLargeLogsHandler)
	rg.GET("/logs/search", middleware.ValidateToken, searchLogsHandler)
	rg.GET("/logs/stats", middleware.ValidateToken, getLogStatsHandler)

	// Log file operations
	rg.GET("/log/tail", middleware.ValidateToken, tailLogHandler)
	rg.GET("/log/content", middleware.ValidateToken, getLogContentHandler)
	rg.GET("/log/export", middleware.ValidateToken, exportLogHandler)
	rg.POST("/log/truncate", middleware.ValidateToken, middleware.RequireAdmin, middleware.ValidateCSRF, middleware.SensitiveActionRateLimit(10, 300000), truncateLogHandler)
	rg.POST("/log/delete", middleware.ValidateToken, middleware.RequireAdmin, middleware.ValidateCSRF, middleware.SensitiveActionRateLimit(5, 300000), deleteLogHandler)
	rg.POST("/logs/clean", middleware.ValidateToken, middleware.RequireAdmin, middleware.ValidateCSRF, middleware.SensitiveActionRateLimit(3, 300000), cleanLogsHandler)

	// Backup
	rg.POST("/logs/backup", middleware.ValidateToken, middleware.RequireAdmin, middleware.ValidateCSRF, middleware.SensitiveActionRateLimit(3, 600000), backupLogsHandler)
	rg.GET("/backups/list", middleware.ValidateToken, listBackupsHandler)
	rg.POST("/backups/delete", middleware.ValidateToken, middleware.RequireAdmin, middleware.ValidateCSRF, middleware.SensitiveActionRateLimit(5, 300000), deleteBackupHandler)
	rg.POST("/backups/clean", middleware.ValidateToken, middleware.RequireAdmin, middleware.ValidateCSRF, middleware.SensitiveActionRateLimit(3, 300000), cleanBackupsHandler)

	// Archives
	rg.GET("/archives/list", middleware.ValidateToken, listArchivesHandler)
	rg.GET("/archive/content", middleware.ValidateToken, getArchiveContentHandler)
	rg.POST("/archives/delete", middleware.ValidateToken, middleware.RequireAdmin, middleware.ValidateCSRF, middleware.SensitiveActionRateLimit(5, 300000), deleteArchiveHandler)

	// Audit
	rg.GET("/audit/log", middleware.ValidateToken, getAuditLogHandler)

	// Dirs
	rg.POST("/dirs/clean-empty", middleware.ValidateToken, middleware.RequireAdmin, middleware.ValidateCSRF, middleware.SensitiveActionRateLimit(3, 300000), cleanEmptyDirsHandler)
	rg.POST("/dirs/clean-uninstalled-trash", middleware.ValidateToken, middleware.RequireAdmin, middleware.ValidateCSRF, middleware.SensitiveActionRateLimit(3, 300000), cleanUninstalledTrashHandler)
	rg.GET("/dirs/recycle-list", middleware.ValidateToken, listRecycleItemsHandler)
	rg.POST("/dirs/recycle-restore", middleware.ValidateToken, middleware.RequireAdmin, middleware.ValidateCSRF, middleware.SensitiveActionRateLimit(10, 300000), restoreRecycleItemsHandler)
	rg.POST("/dirs/unauthorize", middleware.ValidateToken, middleware.ValidateCSRF, middleware.SensitiveActionRateLimit(10, 300000), unauthorizeDirHandler)

	// P2: Shared (app-level) directory authorization management
	// Shared folders are managed by an admin and visible to all users.
	rg.GET("/dirs/shared", middleware.ValidateToken, middleware.APIRateLimit(60, 60000), getSharedDirsHandler)
	rg.POST("/dirs/shared-unauthorize", middleware.ValidateToken, middleware.RequireAdmin, middleware.ValidateCSRF, middleware.SensitiveActionRateLimit(10, 300000), unauthorizeSharedDirHandler)

	// Auto-clean rules
	rg.GET("/auto-clean/rules", middleware.ValidateToken, listCleanRulesHandler)
	rg.POST("/auto-clean/rules", middleware.ValidateToken, middleware.RequireAdmin, middleware.ValidateCSRF, middleware.SensitiveActionRateLimit(10, 300000), createCleanRuleHandler)
	rg.PUT("/auto-clean/rules/:id", middleware.ValidateToken, middleware.RequireAdmin, middleware.ValidateCSRF, middleware.SensitiveActionRateLimit(10, 300000), updateCleanRuleHandler)
	rg.DELETE("/auto-clean/rules/:id", middleware.ValidateToken, middleware.RequireAdmin, middleware.ValidateCSRF, middleware.SensitiveActionRateLimit(10, 300000), deleteCleanRuleHandler)
	rg.POST("/auto-clean/rules/:id/toggle", middleware.ValidateToken, middleware.RequireAdmin, middleware.ValidateCSRF, middleware.SensitiveActionRateLimit(10, 300000), toggleCleanRuleHandler)
	rg.POST("/auto-clean/rules/:id/execute", middleware.ValidateToken, middleware.RequireAdmin, middleware.ValidateCSRF, middleware.SensitiveActionRateLimit(5, 300000), executeCleanRuleHandler)

	// Bookmarks
	rg.GET("/bookmarks", middleware.ValidateToken, listBookmarksHandler)
	rg.POST("/bookmarks", middleware.ValidateToken, middleware.ValidateCSRF, createBookmarkHandler)
	rg.DELETE("/bookmarks/:id", middleware.ValidateToken, middleware.ValidateCSRF, deleteBookmarkHandler)
	rg.PUT("/bookmarks/:id", middleware.ValidateToken, middleware.ValidateCSRF, updateBookmarkHandler)

	// SSE stream
	rg.GET("/logs/stream", logStreamHandler)
}

// ---------------------------------------------------------------------------
// Health
// ---------------------------------------------------------------------------

func healthHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"version": "1.0.0",
	})
}

// ---------------------------------------------------------------------------
// Filter settings
// ---------------------------------------------------------------------------

func getFilterSettingsHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"enabled": utils.IsFilterEnabled(),
	})
}

type filterSettingsBody struct {
	Enabled bool `json:"enabled"`
}

func setFilterSettingsHandler(c *gin.Context) {
	var body filterSettingsBody
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的参数"})
		return
	}
	utils.SetFilterEnabled(body.Enabled)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"enabled": utils.IsFilterEnabled(),
	})
}

// ---------------------------------------------------------------------------
// Directories and app names
// ---------------------------------------------------------------------------

func getDirsHandler(c *gin.Context) {
	dirs, err := services.GetDirsInfo()
	if err != nil {
		slog.Error("failed to get dirs info", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取目录信息失败"})
		return
	}

	// P0: Identify the current user (gateway or standalone session)
	gatewayUID, _ := c.Get("gatewayUID")
	sessionToken, _ := c.Get("sessionToken")
	uid := utils.GetUserIDFromContext(
		gatewayUIDToString(gatewayUID),
		sessionTokenToString(sessionToken),
	)

	// P0: Merge user-authorized folders (trim.file.getUserAccessibleFolders).
	// Keeps custom dirs added via the file picker visible after page refresh
	// and service restart.
	var userPaths []string
	if uid != "" {
		if userDirs := services.GetUserDirsInfo(uid); len(userDirs) > 0 {
			seen := make(map[string]bool, len(dirs))
			for _, d := range dirs {
				seen[d.Path] = true
			}
			for _, ud := range userDirs {
				if !seen[ud.Path] {
					dirs = append(dirs, ud)
					seen[ud.Path] = true
				}
				userPaths = append(userPaths, ud.Path)
			}
		}
	}

	// P0: Filter directories by ACL permission.
	// The allowed set is config LogDirs ∪ user-authorized folders, so custom
	// dirs outside the default config remain accessible while still being
	// validated against the trim.file.checkUserACL result.
	//
	// IMPORTANT: Default system directories (config.LogDirs, e.g. @appdata,
	// @appconf, @appshare, ...) are always shown — they are the app's base set
	// and are visible to all users. ACL filtering applies only to custom /
	// user-authorized folders, otherwise system dirs like @appshare would
	// disappear when the trim ACL returns no "readable" grant for the base dir.
	// Admins see all directories (default + custom) without ACL filtering, so
	// they can manage system log dirs. Non-admins still get ACL filtering for
	// custom/user dirs.
	isAdmin := c.GetHeader("x-trim-isadmin") == "true"
	if uid != "" && !isAdmin {
		// Build the set of default dirs that must always remain visible.
		defaultSet := make(map[string]bool, len(config.Get().LogDirs))
		for _, d := range config.Get().LogDirs {
			defaultSet[utils.SafePath(d)] = true
		}

		allowedDirs := config.Get().LogDirs
		if len(userPaths) > 0 {
			allowedDirs = append(allowedDirs, userPaths...)
		}
		filtered := make([]types.DirInfo, 0, len(dirs))
		for _, dir := range dirs {
			if defaultSet[utils.SafePath(dir.Path)] {
				// Default system dir: always keep visible.
				filtered = append(filtered, dir)
				continue
			}
			if dir.Path != "" {
				allowed, _ := utils.CheckUserFileACL(uid, dir.Path, allowedDirs, "readable")
				if allowed {
					filtered = append(filtered, dir)
				}
			} else {
				filtered = append(filtered, dir)
			}
		}
		dirs = filtered
	}

	// P2: Convert directory paths to display paths
	for i := range dirs {
		dirs[i].DisplayPath = convertPathToDisplay(dirs[i].Path)
	}

	c.JSON(http.StatusOK, gin.H{
		"dirs":    dirs,
		"isAdmin": c.GetHeader("x-trim-isadmin") == "true",
	})
}

func gatewayUIDToString(v interface{}) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func sessionTokenToString(v interface{}) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func getAppNamesHandler(c *gin.Context) {
	names, err := services.GetAppNames()
	if err != nil {
		slog.Error("failed to get app names", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取应用名称失败"})
		return
	}
	c.JSON(http.StatusOK, names)
}

// ---------------------------------------------------------------------------
// Directory authorization management
// ---------------------------------------------------------------------------

type unauthorizeDirBody struct {
	Path string `json:"path"`
}

// unauthorizeDirHandler removes the current user's fnOS authorization for a
// custom log directory (trim.file.delUserAccessibleFolder).
func unauthorizeDirHandler(c *gin.Context) {
	var body unauthorizeDirBody
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的参数"})
		return
	}
	if body.Path == "" || len(body.Path) > maxPathLength {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少路径参数"})
		return
	}

	gatewayUID, _ := c.Get("gatewayUID")
	sessionToken, _ := c.Get("sessionToken")
	uid := utils.GetUserIDFromContext(
		gatewayUIDToString(gatewayUID),
		sessionTokenToString(sessionToken),
	)
	if uid == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无法识别当前用户"})
		return
	}

	// Security: only allow removing authorization for paths the user actually
	// authorized (trim.file.getUserAccessibleFolders) or default config dirs.
	folders, err := services.GetTrimClient().GetUserAccessibleFolders(uid)
	authorized := false
	if err == nil {
		for _, f := range folders {
			if f == body.Path {
				authorized = true
				break
			}
		}
	}
	if !authorized && !utils.IsAllowedPath(body.Path, config.Get().LogDirs) {
		c.JSON(http.StatusForbidden, gin.H{"error": "路径不在允许范围内"})
		return
	}

	if err := services.DelUserDir(uid, body.Path); err != nil {
		slog.Warn("unauthorize dir failed", "uid", uid, "path", body.Path, "error", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "移除授权失败"})
		return
	}

	services.AddAuditLog("dir_unauthorize", map[string]interface{}{
		"path": body.Path,
	}, c)

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// getSharedDirsHandler returns the app-level shared folders
// (trim.file.getSharedAccessibleFolders) as DirInfo entries.
// Shared folders are configured by an admin via pickSharedFile and are
// visible to all users of the app.
func getSharedDirsHandler(c *gin.Context) {
	shared, err := services.GetTrimClient().GetSharedAccessibleFolders()
	if err != nil {
		// The shared-access scope may be unavailable; degrade gracefully.
		slog.Debug("get shared folders failed", "error", err)
		c.JSON(http.StatusOK, gin.H{"dirs": []types.DirInfo{}})
		return
	}

	dirs := make([]types.DirInfo, 0, len(shared))
	for _, p := range shared {
		info := services.GetDirInfo(p)
		info.IsShared = true
		info.DisplayPath = convertPathToDisplay(p)
		dirs = append(dirs, info)
	}

	c.JSON(http.StatusOK, gin.H{"dirs": dirs})
}

// unauthorizeSharedDirHandler removes an admin-configured shared folder
// (trim.file.delSharedAccessibleFolder). Admin only.
func unauthorizeSharedDirHandler(c *gin.Context) {
	var body unauthorizeDirBody
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的参数"})
		return
	}
	if body.Path == "" || len(body.Path) > maxPathLength {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少路径参数"})
		return
	}

	// Security: only allow removing shared folders that actually exist in the
	// app's shared folder list.
	shared, err := services.GetTrimClient().GetSharedAccessibleFolders()
	authorized := false
	if err == nil {
		for _, f := range shared {
			if f == body.Path {
				authorized = true
				break
			}
		}
	}
	if !authorized {
		c.JSON(http.StatusForbidden, gin.H{"error": "路径不在共享授权范围内"})
		return
	}

	if _, err := services.GetTrimClient().DelSharedAccessibleFolder(body.Path); err != nil {
		slog.Warn("unauthorize shared dir failed", "path", body.Path, "error", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "移除共享授权失败"})
		return
	}

	services.AddAuditLog("dir_shared_unauthorize", map[string]interface{}{
		"path": body.Path,
	}, c)

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// ---------------------------------------------------------------------------
// Log listing and search
// ---------------------------------------------------------------------------

type listLogsQuery struct {
	Dir   string `form:"dir"`
	Limit int    `form:"limit"`
}

func listLogsHandler(c *gin.Context) {
	var q listLogsQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的参数"})
		return
	}

	limit := 100
	if q.Limit > 0 {
		limit = utils.Clamp(q.Limit, 1, 500)
	}

	if q.Dir != "" {
		if len(q.Dir) > maxPathLength {
			c.JSON(http.StatusBadRequest, gin.H{"error": "路径过长"})
			return
		}
		if !checkFileACL(c, q.Dir, "readable") {
			c.JSON(http.StatusForbidden, gin.H{"error": "不允许访问此目录"})
			return
		}
	}

	logs, err := services.ListLogFiles(q.Dir, limit)
	if err != nil {
		slog.Error("failed to list log files", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取日志列表失败"})
		return
	}
	if logs == nil {
		logs = []types.LogFile{}
	}

	// P2: Convert paths to display paths
	for i := range logs {
		logs[i].DisplayPath = convertPathToDisplay(logs[i].Path)
	}

	c.JSON(http.StatusOK, gin.H{
		"logs":  logs,
		"total": len(logs),
	})
}

type listLargeLogsQuery struct {
	Threshold string `form:"threshold"`
	Limit     int    `form:"limit"`
}

func listLargeLogsHandler(c *gin.Context) {
	var q listLargeLogsQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的参数"})
		return
	}

	limit := 50
	if q.Limit > 0 {
		limit = utils.Clamp(q.Limit, 1, 200)
	}

	thresholdStr := "10M"
	if q.Threshold != "" {
		if !utils.IsValidThreshold(q.Threshold) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "无效的大小阈值"})
			return
		}
		thresholdStr = q.Threshold
	}

	thresholdBytes := utils.ParseSizeThreshold(thresholdStr)

	logs, err := services.ListLargeLogFiles(thresholdBytes, limit)
	if err != nil {
		slog.Error("failed to list large log files", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取大文件列表失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"logs": logs})
}

type searchLogsQuery struct {
	Type      string `form:"type"`
	Threshold string `form:"threshold"`
	Pattern   string `form:"pattern"`
	Limit     int    `form:"limit"`
}

func searchLogsHandler(c *gin.Context) {
	var q searchLogsQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的参数"})
		return
	}

	limit := 50
	if q.Limit > 0 {
		limit = utils.Clamp(q.Limit, 1, 200)
	}

	if q.Type == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请指定搜索类型"})
		return
	}

	var logs []types.LogFile

	switch q.Type {
	case "size":
		sizeThreshold := q.Threshold
		if sizeThreshold == "" {
			sizeThreshold = "10M"
		}
		if !utils.IsValidThreshold(sizeThreshold) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "无效的大小阈值"})
			return
		}
		thresholdBytes := utils.ParseSizeThreshold(sizeThreshold)
		var err error
		logs, err = services.ListLargeLogFiles(thresholdBytes, limit)
		if err != nil {
			slog.Error("failed to search by size", "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "搜索失败"})
			return
		}

	case "name":
		if q.Pattern == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "请输入文件名模式"})
			return
		}
		if len(q.Pattern) > maxPatternLength {
			c.JSON(http.StatusBadRequest, gin.H{"error": "搜索模式过长"})
			return
		}
		safePattern := sanitizePattern(q.Pattern)
		if safePattern == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "无效的搜索模式"})
			return
		}
		var err error
		logs, err = services.SearchLogFilesByName(safePattern, limit)
		if err != nil {
			slog.Error("failed to search by name", "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "搜索失败"})
			return
		}

	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的搜索类型"})
		return
	}

	if len(logs) > limit {
		logs = logs[:limit]
	}

	c.JSON(http.StatusOK, gin.H{
		"logs":  logs,
		"total": len(logs),
	})
}

func sanitizePattern(pattern string) string {
	if pattern == "" || len(pattern) > maxPatternLength {
		return ""
	}
	return utils.EscapeRegExp(pattern)
}

// ---------------------------------------------------------------------------
// Log stats
// ---------------------------------------------------------------------------

func getLogStatsHandler(c *gin.Context) {
	stats, err := services.GetLogStats()
	if err != nil {
		slog.Error("failed to get log stats", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取统计信息失败"})
		return
	}
	c.JSON(http.StatusOK, stats)
}

// ---------------------------------------------------------------------------
// Log tail (read from offset)
// ---------------------------------------------------------------------------

type tailLogQuery struct {
	Path   string `form:"path"`
	Offset int64  `form:"offset"`
}

func tailLogHandler(c *gin.Context) {
	var q tailLogQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的参数"})
		return
	}

	if q.Path == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少文件路径"})
		return
	}
	if len(q.Path) > maxPathLength {
		c.JSON(http.StatusBadRequest, gin.H{"error": "路径过长"})
		return
	}
	if !checkFileACL(c, q.Path, "readable") {
		c.JSON(http.StatusForbidden, gin.H{"error": "不允许访问此文件"})
		return
	}

	if q.Offset < 0 {
		q.Offset = -1
	}

	normalizedPath := utils.SafePath(q.Path)
	info, err := os.Stat(normalizedPath)
	if err != nil {
		if os.IsNotExist(err) {
			c.JSON(http.StatusOK, gin.H{
				"content":    "",
				"offset":     0,
				"totalSize":  0,
				"deleted":    true,
			})
			return
		}
		slog.Error("failed to stat file for tail", "path", q.Path, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取文件失败"})
		return
	}

	fileSize := info.Size()

	if q.Offset < 0 || q.Offset >= fileSize {
		c.JSON(http.StatusOK, gin.H{
			"content":   "",
			"offset":    fileSize,
			"totalSize": fileSize,
		})
		return
	}

	maxBytes := int64(512 * 1024) // 512KB
	end := q.Offset + maxBytes
	if end > fileSize {
		end = fileSize
	}

	f, err := os.Open(normalizedPath)
	if err != nil {
		slog.Error("failed to open file for tail", "path", q.Path, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取文件失败"})
		return
	}
	defer f.Close()

	_, err = f.Seek(q.Offset, io.SeekStart)
	if err != nil {
		slog.Error("failed to seek file for tail", "path", q.Path, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取文件失败"})
		return
	}

	readSize := end - q.Offset
	buf := make([]byte, readSize)
	n, err := io.ReadFull(f, buf)
	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		slog.Error("failed to read file for tail", "path", q.Path, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取文件失败"})
		return
	}

	content := string(buf[:n])
	content = utils.FilterSensitiveInfo(content)

	c.JSON(http.StatusOK, gin.H{
		"content":   content,
		"offset":    end,
		"totalSize": fileSize,
	})
}

// ---------------------------------------------------------------------------
// Log content
// ---------------------------------------------------------------------------

type logContentQuery struct {
	Path     string `form:"path"`
	MaxLines int    `form:"maxLines"`
	Offset   int    `form:"offset"`
	Tail     string `form:"tail"`
}

func getLogContentHandler(c *gin.Context) {
	var q logContentQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的参数"})
		return
	}

	if q.Path == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少文件路径"})
		return
	}
	if len(q.Path) > maxPathLength {
		c.JSON(http.StatusBadRequest, gin.H{"error": "路径过长"})
		return
	}
	if !checkFileACL(c, q.Path, "readable") {
		c.JSON(http.StatusForbidden, gin.H{"error": "不允许访问此文件"})
		return
	}

	maxLines := 5000
	if q.MaxLines > 0 {
		maxLines = utils.Clamp(q.MaxLines, 100, 200000)
	}

	offset := 0
	if q.Offset > 0 {
		offset = q.Offset
	}

	tail := q.Tail == "true"

	options := types.ReadLogOptions{
		MaxLines: maxLines,
		Offset:   offset,
		Tail:     tail,
	}

	result, err := services.ReadLogFile(q.Path, options)
	if err != nil {
		// Check if it's a file-too-large error with size info
		slog.Error("failed to read log file", "path", q.Path, "error", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"content":       utils.FilterSensitiveInfo(result.Content),
		"totalLines":    result.TotalLines,
		"size":          result.Size,
		"sizeFormatted": result.SizeFormatted,
		"truncated":     result.Truncated,
		"hasMore":       result.HasMore,
	})
}

// ---------------------------------------------------------------------------
// Log export
// ---------------------------------------------------------------------------

type exportLogQuery struct {
	Path   string `form:"path"`
	Format string `form:"format"`
}

func exportLogHandler(c *gin.Context) {
	var q exportLogQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的参数"})
		return
	}

	if q.Path == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少文件路径"})
		return
	}
	if len(q.Path) > maxPathLength {
		c.JSON(http.StatusBadRequest, gin.H{"error": "路径过长"})
		return
	}
	if !checkFileACL(c, q.Path, "readable") {
		c.JSON(http.StatusForbidden, gin.H{"error": "不允许访问此文件"})
		return
	}

	format := "txt"
	if q.Format != "" {
		format = q.Format
	}
	if format != "txt" && format != "json" && format != "csv" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的导出格式"})
		return
	}

	options := types.ReadLogOptions{
		MaxLines: 200000,
	}
	result, err := services.ReadLogFile(q.Path, options)
	if err != nil {
		slog.Error("failed to read log file for export", "path", q.Path, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "导出失败"})
		return
	}

	// Build safe filename
	basename := filepath.Base(q.Path)
	safeName := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' || r == '-' || r == '_' {
			return r
		}
		return '_'
	}, basename)
	timestamp := time.Now().Format("20060102_150405")
	exportName := fmt.Sprintf("%s_%s", safeName, timestamp)

	// Audit
	services.AddAuditLog("log_export", map[string]interface{}{
		"path":   utils.SafePath(q.Path),
		"format": format,
	}, c)

	switch format {
	case "txt":
		content := utils.FilterSensitiveInfo(result.Content)
		c.Header("Content-Type", "text/plain; charset=utf-8")
		c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.txt"`, exportName))
		c.String(http.StatusOK, content)

	case "json":
		lines := strings.Split(result.Content, "\n")
		type jsonLine struct {
			Line    int    `json:"line"`
			Content string `json:"content"`
		}
		data := make([]jsonLine, 0, len(lines))
		for i, line := range lines {
			data = append(data, jsonLine{
				Line:    i + 1,
				Content: line,
			})
		}
		exportJSON := gin.H{
			"source":      utils.SafePath(q.Path),
			"exportedAt":  time.Now().Format(time.RFC3339),
			"totalLines":  result.TotalLines,
			"lines":       data,
		}
		c.Header("Content-Type", "application/json; charset=utf-8")
		c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.json"`, exportName))
		c.JSON(http.StatusOK, exportJSON)

	case "csv":
		lines := strings.Split(result.Content, "\n")
		c.Header("Content-Type", "text/csv; charset=utf-8")
		c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.csv"`, exportName))

		// Write CSV manually
		var b strings.Builder
		b.WriteString("\"line\",\"content\"\n")
		for i, line := range lines {
			escaped := strings.ReplaceAll(line, "\"", "\"\"")
			b.WriteString(fmt.Sprintf("\"%d\",\"%s\"\n", i+1, escaped))
		}
		c.String(http.StatusOK, b.String())
	}
}

// ---------------------------------------------------------------------------
// Truncate / Delete log
// ---------------------------------------------------------------------------

type pathBody struct {
	Path string `json:"path"`
}

func truncateLogHandler(c *gin.Context) {
	var body pathBody
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的参数"})
		return
	}

	if body.Path == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少文件路径"})
		return
	}
	if len(body.Path) > maxPathLength {
		c.JSON(http.StatusBadRequest, gin.H{"error": "路径过长"})
		return
	}
	if !checkFileACL(c, body.Path, "writable") {
		c.JSON(http.StatusForbidden, gin.H{"error": "不允许访问此文件"})
		return
	}

	if err := services.TruncateLogFile(body.Path); err != nil {
		slog.Error("failed to truncate log file", "path", body.Path, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	services.AddAuditLog("log_truncate", map[string]interface{}{
		"path": utils.SafePath(body.Path),
	}, c)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "日志已清空",
	})
}

func deleteLogHandler(c *gin.Context) {
	var body pathBody
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的参数"})
		return
	}

	if body.Path == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少文件路径"})
		return
	}
	if len(body.Path) > maxPathLength {
		c.JSON(http.StatusBadRequest, gin.H{"error": "路径过长"})
		return
	}
	if !checkFileACL(c, body.Path, "deletable") {
		c.JSON(http.StatusForbidden, gin.H{"error": "不允许访问此文件"})
		return
	}

	if err := services.DeleteLogFile(body.Path); err != nil {
		slog.Error("failed to delete log file", "path", body.Path, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	services.AddAuditLog("log_delete", map[string]interface{}{
		"path": utils.SafePath(body.Path),
	}, c)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "日志文件已删除",
	})
}

// ---------------------------------------------------------------------------
// Clean logs
// ---------------------------------------------------------------------------

type cleanLogsBody struct {
	Threshold string `json:"threshold"`
	Days      *int   `json:"days"`
	Action    string `json:"action"`
}

func cleanLogsHandler(c *gin.Context) {
	var body cleanLogsBody
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的参数"})
		return
	}

	action := body.Action
	if action == "" {
		action = "truncate"
	}

	if !utils.IsValidAction(action) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的操作类型"})
		return
	}

	// deleteUninstalled does not need threshold/days
	if action == "deleteUninstalled" {
		result, err := services.CleanUninstalledLogs()
		if err != nil {
			slog.Error("failed to clean uninstalled logs", "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "清理失败"})
			return
		}
		services.AddAuditLog("logs_clean", map[string]interface{}{
			"action":  action,
			"cleaned": result.Cleaned,
		}, c)
		c.JSON(http.StatusOK, result)
		return
	}

	if body.Threshold == "" && body.Days == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请指定清理条件"})
		return
	}

	var thresholdBytes *int64
	if body.Threshold != "" {
		if !utils.IsValidThreshold(body.Threshold) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "无效的大小阈值"})
			return
		}
		tb := utils.ParseSizeThreshold(body.Threshold)
		thresholdBytes = &tb
	}

	var days *int
	if body.Days != nil {
		d := utils.Clamp(*body.Days, 1, 365)
		days = &d
	}

	options := types.CleanLogOptions{
		ThresholdBytes: thresholdBytes,
		Days:           days,
		Action:         action,
	}

	result, err := services.CleanLogFiles(options)
	if err != nil {
		slog.Error("failed to clean log files", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "清理失败"})
		return
	}

	services.AddAuditLog("logs_clean", map[string]interface{}{
		"action":   action,
		"threshold": body.Threshold,
		"days":     body.Days,
		"cleaned":  result.Cleaned,
	}, c)

	c.JSON(http.StatusOK, result)
}

// ---------------------------------------------------------------------------
// Backup and backups management
// ---------------------------------------------------------------------------

func backupLogsHandler(c *gin.Context) {
	result := services.PerformBackup()

	// No files collected — treat as failure
	if result.Files == 0 {
		msg := "备份失败: 没有找到日志文件"
		if len(result.Errors) > 0 {
			msg = "备份失败: " + result.Errors[len(result.Errors)-1]
		}
		slog.Warn("backup produced no files", "errors", result.Errors)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success":    false,
			"backupPath": result.BackupPath,
			"files":      0,
			"backupSize": "0B",
			"errors":     result.Errors,
			"error":      msg,
		})
		return
	}

	// Some files collected — success (with warnings if any)
	if len(result.Errors) > 0 {
		slog.Warn("backup created with warnings", "path", result.BackupPath,
			"files", result.Files, "size", result.BackupSize, "warnings", result.Errors)
	}
	slog.Info("backup created", "path", result.BackupPath, "files", result.Files, "size", result.BackupSize)
	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"backupPath": result.BackupPath,
		"files":      result.Files,
		"backupSize": result.BackupSize,
		"errors":     result.Errors,
		"message":    "备份已创建",
	})
}

func listBackupsHandler(c *gin.Context) {
	backups := services.ListBackups()
	c.JSON(http.StatusOK, gin.H{
		"backups": backups,
		"total":   len(backups),
	})
}

func deleteBackupHandler(c *gin.Context) {
	var body struct {
		Path string `json:"path"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的参数"})
		return
	}
	if body.Path == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少备份文件路径"})
		return
	}

	safePath := utils.SafePath(body.Path)
	if safePath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的备份路径"})
		return
	}
	cfg := config.Get()
	if !strings.HasPrefix(safePath, cfg.Backup.BaseDir) || !strings.HasSuffix(safePath, ".tar.gz") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "只能删除备份目录下的 .tar.gz 文件"})
		return
	}

	if err := services.DeleteBackup(body.Path); err != nil {
		slog.Error("failed to delete backup", "path", body.Path, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "删除备份失败: " + err.Error(),
		})
		return
	}
	slog.Info("backup deleted", "path", body.Path)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "备份已删除",
	})
}

type cleanBackupsBody struct {
	Days int `json:"days"`
}

func cleanBackupsHandler(c *gin.Context) {
	var body cleanBackupsBody
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的参数"})
		return
	}
	days := 30
	if body.Days > 0 {
		days = utils.Clamp(body.Days, 1, 365)
	}

	deleted, err := services.CleanOldBackups(days)
	if err != nil {
		slog.Error("failed to clean old backups", "days", days, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"deleted": deleted,
			"message": "清理旧备份失败: " + err.Error(),
		})
		return
	}
	slog.Info("old backups cleaned", "deleted", deleted, "days", days)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"deleted": deleted,
		"message": fmt.Sprintf("已删除 %d 个旧备份(超过%d天)", deleted, days),
	})
}

// ---------------------------------------------------------------------------
// Archives
// ---------------------------------------------------------------------------

type listArchivesQuery struct {
	Limit int `form:"limit"`
}

func listArchivesHandler(c *gin.Context) {
	var q listArchivesQuery
	_ = c.ShouldBindQuery(&q)

	limit := 50
	if q.Limit > 0 {
		limit = utils.Clamp(q.Limit, 1, 200)
	}

	archives, err := services.ListArchiveFiles(limit)
	if err != nil {
		slog.Error("failed to list archive files", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取归档列表失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"archives": archives,
		"total":    len(archives),
	})
}

type archiveContentQuery struct {
	Path  string `form:"path"`
	Lines int    `form:"lines"`
}

func getArchiveContentHandler(c *gin.Context) {
	var q archiveContentQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的参数"})
		return
	}

	if q.Path == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少文件路径"})
		return
	}
	if len(q.Path) > maxPathLength {
		c.JSON(http.StatusBadRequest, gin.H{"error": "路径过长"})
		return
	}
	if !checkFileACL(c, q.Path, "readable") {
		c.JSON(http.StatusForbidden, gin.H{"error": "不允许访问此文件"})
		return
	}

	maxLines := 50
	if q.Lines > 0 {
		maxLines = utils.Clamp(q.Lines, 1, 500)
	}

	content, truncated, err := readArchiveContent(q.Path, maxLines)
	if err != nil {
		slog.Error("failed to read archive content", "path", q.Path, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取归档文件失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"content":   utils.FilterSensitiveInfo(content),
		"truncated": truncated,
	})
}

// isSafeFilePath checks if a path contains only characters safe for exec.Command arguments.
// This prevents argument injection via specially crafted paths.
func isSafeFilePath(p string) bool {
	// Only allow characters typically found in safe file paths.
	// exec.Command with array arguments doesn't use a shell, but external tools
	// may interpret special characters as flags or options.
	for _, c := range p {
		if c > 127 {
			continue // allow UTF-8 multibyte characters
		}
		if c >= ' ' && c <= '~' {
			// Printable ASCII - only allow safe characters
			switch {
			case c >= 'a' && c <= 'z':
			case c >= 'A' && c <= 'Z':
			case c >= '0' && c <= '9':
			case c == '/' || c == '.' || c == '-' || c == '_':
			case c == ':' || c == '@' || c == '+':
			case c == '=' || c == ',' || c == '~':
			case c == '(' || c == ')' || c == ' ':
			default:
				return false
			}
		} else {
			// Non-printable ASCII - reject
			return false
		}
	}
	return true
}

// readArchiveContent reads content from compressed archive files.
func readArchiveContent(path string, maxLines int) (string, bool, error) {
	normalizedPath := utils.SafePath(path)
	if normalizedPath == "" {
		return "", false, fmt.Errorf("无效的文件路径")
	}

	if !isSafeFilePath(normalizedPath) {
		return "", false, fmt.Errorf("文件路径包含不安全字符")
	}

	lower := strings.ToLower(normalizedPath)

	var cmd *exec.Cmd
	switch {
	case strings.HasSuffix(lower, ".gz") || strings.HasSuffix(lower, ".tgz"):
		cmd = exec.Command("zcat", normalizedPath)
	case strings.HasSuffix(lower, ".bz2"):
		cmd = exec.Command("bzcat", normalizedPath)
	case strings.HasSuffix(lower, ".xz"):
		cmd = exec.Command("xzcat", normalizedPath)
	case strings.HasSuffix(lower, ".zip"):
		cmd = exec.Command("unzip", "-p", normalizedPath)
	case strings.HasSuffix(lower, ".tar"):
		cmd = exec.Command("tar", "-xOf", normalizedPath)
	case strings.HasSuffix(lower, ".tar.gz") || strings.HasSuffix(lower, ".tar.bz2") || strings.HasSuffix(lower, ".tar.xz"):
		cmd = exec.Command("tar", "-xOf", normalizedPath)
	case strings.HasSuffix(lower, ".7z"):
		cmd = exec.Command("7z", "x", "-so", normalizedPath)
	case strings.HasSuffix(lower, ".rar"):
		cmd = exec.Command("unrar", "p", normalizedPath)
	default:
		// Try reading as plain text
		f, err := os.Open(normalizedPath)
		if err != nil {
			return "", false, err
		}
		defer f.Close()

		scanner := bufio.NewScanner(f)
		scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
		var lines []string
		for scanner.Scan() && len(lines) < maxLines {
			lines = append(lines, scanner.Text())
		}
		if err := scanner.Err(); err != nil {
			return "", false, err
		}
		truncated := false
		if scanner.Scan() {
			truncated = true
		}
		return strings.Join(lines, "\n"), truncated, nil
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", false, fmt.Errorf("无法创建管道: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return "", false, fmt.Errorf("无法启动解压命令: %w", err)
	}

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	var lines []string
	for scanner.Scan() && len(lines) < maxLines {
		lines = append(lines, scanner.Text())
	}

	// We need to wait for the command to finish and consume remaining output
	// to check if there are more lines
	truncated := false
	if scanner.Scan() {
		truncated = true
	}
	// Drain the rest
	for scanner.Scan() {
	}

	cmd.Wait()

	if err := scanner.Err(); err != nil {
		return strings.Join(lines, "\n"), truncated, nil
	}

	return strings.Join(lines, "\n"), truncated, nil
}

func deleteArchiveHandler(c *gin.Context) {
	var body pathBody
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的参数"})
		return
	}

	if body.Path == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少文件路径"})
		return
	}
	if len(body.Path) > maxPathLength {
		c.JSON(http.StatusBadRequest, gin.H{"error": "路径过长"})
		return
	}
	if !checkFileACL(c, body.Path, "deletable") {
		c.JSON(http.StatusForbidden, gin.H{"error": "不允许访问此文件"})
		return
	}

	if err := services.DeleteLogFile(body.Path); err != nil {
		slog.Error("failed to delete archive file", "path", body.Path, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	services.AddAuditLog("archive_delete", map[string]interface{}{
		"path": utils.SafePath(body.Path),
	}, c)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "归档文件已删除",
	})
}

// ---------------------------------------------------------------------------
// Audit log
// ---------------------------------------------------------------------------

func getAuditLogHandler(c *gin.Context) {
	services.CleanOldAuditLogs()
	logs, err := services.GetAuditLogs(100)
	if err != nil {
		slog.Error("failed to get audit logs", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取审计日志失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"logs": logs})
}

// ---------------------------------------------------------------------------
// Clean empty dirs
// ---------------------------------------------------------------------------

func cleanEmptyDirsHandler(c *gin.Context) {
	result, err := services.CleanEmptyAppDirs()
	if err != nil {
		slog.Error("failed to clean empty dirs", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "清理空目录失败"})
		return
	}

	cleaned := 0
	if v, ok := result["cleaned"].(int); ok {
		cleaned = v
	}
	dirs := []string{}
	if v, ok := result["dirs"].([]string); ok {
		dirs = v
	}
	errs := []string{}
	if v, ok := result["errors"].([]string); ok {
		errs = v
	}

	services.AddAuditLog("dirs_clean_empty", map[string]interface{}{
		"cleaned": cleaned,
		"dirs":    dirs,
	}, c)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"cleaned": cleaned,
		"dirs":    dirs,
		"errors":  errs,
		"message": fmt.Sprintf("已删除 %d 个空文件夹", cleaned),
	})
}

// ---------------------------------------------------------------------------
// Clean uninstalled-app leftovers into the recycle bin
// ---------------------------------------------------------------------------

func cleanUninstalledTrashHandler(c *gin.Context) {
	gatewayUID, _ := c.Get("gatewayUID")
	sessionToken, _ := c.Get("sessionToken")
	uid := utils.GetUserIDFromContext(
		gatewayUIDToString(gatewayUID),
		sessionTokenToString(sessionToken),
	)

	result, err := services.CleanUninstalledAppDirsToTrash(uid)
	if err != nil {
		slog.Error("failed to clean uninstalled app dirs into trash", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "清理已卸载应用残余目录失败"})
		return
	}

	services.AddAuditLog("dirs_clean_uninstalled_trash", map[string]interface{}{
		"moved": result.Moved,
		"dirs":  result.Dirs,
	}, c)

	message := fmt.Sprintf("已将 %d 个已卸载应用残留目录移入回收站，24小时后自动清空", result.Moved)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"moved":   result.Moved,
		"dirs":    result.Dirs,
		"errors":  result.Errors,
		"message": message,
	})
}

// listRecycleItemsHandler returns the current recycle-bin contents with each
// item's original location, so users can find and manually restore them.
func listRecycleItemsHandler(c *gin.Context) {
	items := services.ListRecycleItems()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"items":   items,
	})
}

// restoreRecycleItemsHandler restores one or more recycle items back to their
// original locations.
func restoreRecycleItemsHandler(c *gin.Context) {
	var req struct {
		Root string   `json:"root"`
		Rels []string `json:"rels"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || len(req.Rels) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	restored, errs := services.RestoreRecycleItems(req.Root, req.Rels)

	services.AddAuditLog("dirs_clean_uninstalled_restore", map[string]interface{}{
		"restored": restored,
		"rels":     req.Rels,
	}, c)

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"restored": restored,
		"errors":   errs,
		"message":  fmt.Sprintf("已还原 %d 个项目", restored),
	})
}

// ---------------------------------------------------------------------------
// Auto-clean rules
// ---------------------------------------------------------------------------

func listCleanRulesHandler(c *gin.Context) {
	rules := services.GetCleanRules()
	c.JSON(http.StatusOK, gin.H{"rules": rules})
}

type createCleanRuleBody struct {
	Name            string   `json:"name"`
	Enabled         bool     `json:"enabled"`
	Schedule        string   `json:"schedule"`
	LogDirs         []string `json:"logDirs"`
	FilePattern     string   `json:"filePattern"`
	MinSizeBytes    int64    `json:"minSizeBytes"`
	MaxSizeBytes    int64    `json:"maxSizeBytes"`
	RetentionDays   int      `json:"retentionDays"`
	Action          string   `json:"action"`
	MaxFilesToClean int      `json:"maxFilesToClean"`
	Description     string   `json:"description"`
}

func createCleanRuleHandler(c *gin.Context) {
	var body createCleanRuleBody
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的参数"})
		return
	}

	if body.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "规则名称不能为空"})
		return
	}
	if body.Schedule == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "调度表达式不能为空"})
		return
	}
	if body.Action != "" && !utils.IsValidAction(body.Action) {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("无效的操作类型: %s", body.Action)})
		return
	}

	rule := services.CleanRule{
		Name:            body.Name,
		Enabled:         body.Enabled,
		Schedule:        body.Schedule,
		LogDirs:         body.LogDirs,
		FilePattern:     body.FilePattern,
		MinSizeBytes:    body.MinSizeBytes,
		MaxSizeBytes:    body.MaxSizeBytes,
		RetentionDays:   body.RetentionDays,
		Action:          body.Action,
		MaxFilesToClean: body.MaxFilesToClean,
		Description:     body.Description,
	}

	created, err := services.CreateCleanRule(rule)
	if err != nil {
		slog.Error("failed to create clean rule", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	services.AddAuditLog("auto_clean_rule_add", map[string]interface{}{
		"ruleId": created.ID,
		"name":   created.Name,
	}, c)

	c.JSON(http.StatusOK, gin.H{"rule": created})
}

type updateCleanRuleBody struct {
	Name            string   `json:"name"`
	Enabled         bool     `json:"enabled"`
	Schedule        string   `json:"schedule"`
	LogDirs         []string `json:"logDirs"`
	FilePattern     string   `json:"filePattern"`
	MinSizeBytes    int64    `json:"minSizeBytes"`
	MaxSizeBytes    int64    `json:"maxSizeBytes"`
	RetentionDays   int      `json:"retentionDays"`
	Action          string   `json:"action"`
	MaxFilesToClean int      `json:"maxFilesToClean"`
	Description     string   `json:"description"`
}

func updateCleanRuleHandler(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少规则ID"})
		return
	}

	var body updateCleanRuleBody
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的参数"})
		return
	}

	// Check the rule exists first
	existing, err := services.GetCleanRule(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "规则不存在"})
		return
	}

	// Build update from existing with overrides
	update := *existing
	if body.Name != "" {
		update.Name = body.Name
	}
	if body.Schedule != "" {
		update.Schedule = body.Schedule
	}
	if len(body.LogDirs) > 0 {
		update.LogDirs = body.LogDirs
	}
	if body.FilePattern != "" {
		update.FilePattern = body.FilePattern
	}
	if body.MinSizeBytes > 0 {
		update.MinSizeBytes = body.MinSizeBytes
	}
	if body.MaxSizeBytes > 0 {
		update.MaxSizeBytes = body.MaxSizeBytes
	}
	if body.RetentionDays > 0 {
		update.RetentionDays = body.RetentionDays
	}
	if body.Action != "" {
		if !utils.IsValidAction(body.Action) {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("无效的操作类型: %s", body.Action)})
			return
		}
		update.Action = body.Action
	}
	if body.MaxFilesToClean > 0 {
		update.MaxFilesToClean = body.MaxFilesToClean
	}
	if body.Description != "" {
		update.Description = body.Description
	}

	updated, err := services.UpdateCleanRule(id, update)
	if err != nil {
		slog.Error("failed to update clean rule", "id", id, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	services.AddAuditLog("auto_clean_rule_update", map[string]interface{}{
		"ruleId": id,
	}, c)

	c.JSON(http.StatusOK, gin.H{"rule": updated})
}

func deleteCleanRuleHandler(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少规则ID"})
		return
	}

	if err := services.DeleteCleanRule(id); err != nil {
		if strings.Contains(err.Error(), "规则不存在") {
			c.JSON(http.StatusNotFound, gin.H{"error": "规则不存在"})
			return
		}
		slog.Error("failed to delete clean rule", "id", id, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	services.AddAuditLog("auto_clean_rule_delete", map[string]interface{}{
		"ruleId": id,
	}, c)

	c.JSON(http.StatusOK, gin.H{"success": true})
}

func toggleCleanRuleHandler(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少规则ID"})
		return
	}

	// Get current rule to toggle
	rule, err := services.GetCleanRule(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "规则不存在"})
		return
	}

	if err := services.ToggleCleanRule(id, !rule.Enabled); err != nil {
		slog.Error("failed to toggle clean rule", "id", id, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	services.AddAuditLog("auto_clean_rule_toggle", map[string]interface{}{
		"ruleId":  id,
		"enabled": !rule.Enabled,
	}, c)

	// Re-fetch updated rule
	updated, _ := services.GetCleanRule(id)
	c.JSON(http.StatusOK, gin.H{"rule": updated})
}

func executeCleanRuleHandler(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少规则ID"})
		return
	}

	result, err := services.RunCleanRuleNow(id)
	if err != nil {
		if strings.Contains(err.Error(), "规则不存在") {
			c.JSON(http.StatusNotFound, gin.H{"error": "规则不存在"})
			return
		}
		slog.Error("failed to execute clean rule", "id", id, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	services.AddAuditLog("auto_clean_manual", map[string]interface{}{
		"ruleId":  id,
		"cleaned": result.Cleaned,
	}, c)

	c.JSON(http.StatusOK, gin.H{
		"cleaned": result.Cleaned,
		"errors":  result.Errors,
	})
}

// ---------------------------------------------------------------------------
// Bookmarks
// ---------------------------------------------------------------------------

func listBookmarksHandler(c *gin.Context) {
	bookmarks, err := services.LoadBookmarks()
	if err != nil {
		slog.Error("failed to load bookmarks", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取书签失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"bookmarks": bookmarks})
}

type createBookmarkBody struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	IsDocker bool   `json:"isDocker"`
}

func createBookmarkHandler(c *gin.Context) {
	var body createBookmarkBody
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的参数"})
		return
	}

	if body.Path == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少路径"})
		return
	}
	if len(body.Path) > maxPathLength {
		c.JSON(http.StatusBadRequest, gin.H{"error": "路径过长"})
		return
	}

	if body.IsDocker {
		if !utils.IsValidContainerName(body.Path) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "无效的容器名称"})
			return
		}
	} else {
		if !checkFileACL(c, body.Path, "readable") {
			c.JSON(http.StatusForbidden, gin.H{"error": "不允许访问此路径"})
			return
		}
	}

	displayName := body.Name
	if displayName == "" {
		displayName = filepath.Base(body.Path)
	}

	bookmark, err := services.AddBookmark(displayName, body.Path, body.IsDocker)
	if err != nil {
		slog.Error("failed to add bookmark", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "添加书签失败"})
		return
	}

	services.AddAuditLog("bookmark_add", map[string]interface{}{
		"id":       bookmark.ID,
		"path":     utils.SafePath(body.Path),
		"isDocker": body.IsDocker,
	}, c)

	c.JSON(http.StatusOK, gin.H{"bookmark": bookmark})
}

type updateBookmarkBody struct {
	Name string `json:"name"`
}

func updateBookmarkHandler(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少书签ID"})
		return
	}

	var body updateBookmarkBody
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的参数"})
		return
	}

	if body.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "名称不能为空"})
		return
	}

	bookmark := services.UpdateBookmark(id, body.Name)
	if bookmark == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "书签不存在"})
		return
	}

	services.AddAuditLog("bookmark_update", map[string]interface{}{
		"id":   id,
		"name": body.Name,
	}, c)

	c.JSON(http.StatusOK, gin.H{"bookmark": bookmark})
}

func deleteBookmarkHandler(c *gin.Context) {
	id := c.Param("id")
	path := c.Query("path")
	isDocker := c.Query("isDocker") == "true"
	if id == "" && path == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少书签ID或路径"})
		return
	}

	deleted := false
	if id != "" {
		deleted = services.DeleteBookmark(id)
	}
	if !deleted && path != "" {
		deleted = services.DeleteBookmarkByPath(path, isDocker)
	}

	if !deleted {
		c.JSON(http.StatusNotFound, gin.H{"error": "书签不存在"})
		return
	}

	services.AddAuditLog("bookmark_delete", map[string]interface{}{
		"id": id,
	}, c)

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// ---------------------------------------------------------------------------
// Log stream WebSocket
// ---------------------------------------------------------------------------

func logStreamHandler(c *gin.Context) {
	services.ServeLogStreamWS(c.Writer, c.Request)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// parseInt64OrZero parses a string as int64, returning 0 on failure.
func parseInt64OrZero(s string) int64 {
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0
	}
	return n
}

// checkFileACL is a centralized helper that performs path whitelist + ACL check.
// It extracts user info from gin.Context and returns true if access is allowed.
// P0: Integrated ACL permission check via fnOS trim API.
//
// Admins (x-trim-isadmin=true) bypass the trim ACL so they can manage system
// log directories under @appdata/@appconf/etc. The path whitelist (IsAllowedPath)
// is still enforced to prevent access outside the configured log dirs.
func checkFileACL(c *gin.Context, filePath string, action string) bool {
	// Admins have full access to system log directories. The x-trim-isadmin
	// header is only trusted when it was injected by the trusted gateway, i.e.
	// when the request genuinely arrived via the loopback unix-socket proxy.
	// Trusting this header on a directly-reachable connection would let a
	// remote client forge it and bypass the per-user ACL.
	if config.IsGatewayMode() && c.GetHeader("x-trim-isadmin") == "true" && utils.IsLoopbackAddr(c.Request.RemoteAddr) {
		return utils.IsAllowedPath(filePath, config.Get().LogDirs)
	}

	gatewayUID, _ := c.Get("gatewayUID")
	sessionToken, _ := c.Get("sessionToken")
	uid := utils.GetUserIDFromContext(
		gatewayUIDToString(gatewayUID),
		sessionTokenToString(sessionToken),
	)

	allowed, reason := utils.CheckUserFileACL(uid, filePath, config.Get().LogDirs, action)
	if !allowed {
		if reason == "" {
			reason = "不允许访问此文件"
		}
		return false
	}
	return true
}

// convertPathToDisplay converts an internal path to a user-friendly display path.
// P2: Uses trim.file.convertPath to make paths more readable.
// Falls back to the raw path when the trim API is unavailable (e.g. 403 on
// installs without open-platform api-scope permission).
func convertPathToDisplay(path string) string {
	client := services.GetTrimClient()
	display, err := client.ConvertPath(path, "")
	if err != nil || display == "" {
		return path
	}
	return display
}
