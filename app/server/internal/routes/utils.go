package routes

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/sushazhi/fnos-logmanager/internal/middleware"
	"github.com/sushazhi/fnos-logmanager/internal/services"
)

// RegisterUtilRoutes registers utility API routes (path conversion, etc.)
func RegisterUtilRoutes(rg *gin.RouterGroup) {
	// P2: Path conversion via trim API
	rg.POST("/convert-path", middleware.ValidateToken, convertPathHandler)
}

type convertPathBody struct {
	Path     string `json:"path"`
	Language string `json:"language"`
}

func convertPathHandler(c *gin.Context) {
	var body convertPathBody
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的参数"})
		return
	}

	if body.Path == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少路径参数"})
		return
	}

	client := services.GetTrimClient()
	semanticPath, err := client.ConvertPath(body.Path, body.Language)
	if err != nil {
		slog.Warn("convertPath failed", "path", body.Path, "error", err)
		// Fallback: return original path
		c.JSON(http.StatusOK, gin.H{
			"semanticPath": body.Path,
			"displayPath":  body.Path,
			"original":     body.Path,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"semanticPath": semanticPath,
		"displayPath":  semanticPath,
		"original":     body.Path,
	})
}
