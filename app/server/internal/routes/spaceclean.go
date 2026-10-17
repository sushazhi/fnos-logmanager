package routes

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/sushazhi/fnos-logmanager/internal/middleware"
	"github.com/sushazhi/fnos-logmanager/internal/services"
)

// RegisterSpaceCleanRoutes registers system space analysis & cleanup routes.
//
// 扫描是重操作（大量 stat），因此读接口也加了限流；清理是敏感写操作，
// 除管理员 + CSRF 之外还单独限流，避免误点连发。
func RegisterSpaceCleanRoutes(rg *gin.RouterGroup) {
	rg.GET("/space/usage", middleware.ValidateToken, middleware.APIRateLimit(30, 60000), spaceUsageHandler)
	rg.POST("/space/clean", middleware.ValidateToken, middleware.RequireAdmin, middleware.ValidateCSRF, middleware.SensitiveActionRateLimit(6, 300000), spaceCleanHandler)
}

// spaceUsageHandler scans (or returns cached) disk-space usage by category.
//
// Query: ?refresh=true forces a rescan, bypassing the TTL cache.
func spaceUsageHandler(c *gin.Context) {
	force := c.Query("refresh") == "true" || c.Query("refresh") == "1"

	result, err := services.ScanSpaceUsage(force)
	if err != nil {
		slog.Warn("space usage scan failed", "error", err)
		// 错误路径也要带上时效字段：前端的"正在后台更新"提示与轮询逻辑依赖它们，
		// 缺失会变成 undefined，判断行为因响应形态而异。
		c.JSON(http.StatusOK, gin.H{
			"categories":      []services.SpaceCategory{},
			"notices":         []services.SpaceNotice{},
			"volumes":         []services.VolumeUsage{},
			"cacheAgeMs":      int64(0),
			"cacheAgeSeconds": int64(0),
			"stale":           false,
			"refreshing":      false,
			"error":           err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, result)
}

type spaceCleanBody struct {
	// Paths are absolute paths previously returned by /api/space/usage. They are
	// re-validated against the server-side rule table before any deletion.
	Paths  []string `json:"paths"`
	DryRun bool     `json:"dryRun"`
}

// maxSpaceCleanPaths caps how many items one request may clean, so a single
// call cannot mass-delete through a compromised or buggy client.
const maxSpaceCleanPaths = 200

func spaceCleanHandler(c *gin.Context) {
	var body spaceCleanBody
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的参数"})
		return
	}
	if len(body.Paths) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请选择要清理的项目"})
		return
	}
	if len(body.Paths) > maxSpaceCleanPaths {
		c.JSON(http.StatusBadRequest, gin.H{"error": "单次清理条目过多，请分批执行"})
		return
	}

	uid := gatewayUIDFromContext(c)
	result := services.CleanSpaceItems(body.Paths, body.DryRun, uid)

	// 只审计真正执行了删除的调用；dry-run 不落审计避免噪声。
	if !body.DryRun {
		services.AddAuditLog("space_clean", map[string]interface{}{
			"requested": len(body.Paths),
			"deleted":   result.Deleted,
			"recycled":  result.Recycled,
			"truncated": result.Truncated,
			"failed":    result.Failed,
			"freed":     result.FreedFormatted,
			"paths":     truncatePathsForAudit(body.Paths),
		}, c)
	}

	c.JSON(http.StatusOK, result)
}

// gatewayUIDFromContext extracts the fnOS gateway uid (trusted, injected by the
// gateway proxy) and falls back to the process uid in standalone mode.
func gatewayUIDFromContext(c *gin.Context) string {
	if v, ok := c.Get("gatewayUID"); ok {
		if s := gatewayUIDToString(v); s != "" {
			return s
		}
	}
	return services.CurrentUID()
}

// truncatePathsForAudit keeps audit entries readable when a large batch is
// cleaned, while preserving enough detail to trace what was removed.
func truncatePathsForAudit(paths []string) []string {
	const maxAuditPaths = 20
	if len(paths) <= maxAuditPaths {
		return paths
	}
	out := make([]string, 0, maxAuditPaths+1)
	out = append(out, paths[:maxAuditPaths]...)
	out = append(out, "... 其余 "+strconv.Itoa(len(paths)-maxAuditPaths)+" 条已省略")
	return out
}
