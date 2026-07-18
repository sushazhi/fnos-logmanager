package routes

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/sushazhi/fnos-logmanager/internal/config"
	"github.com/sushazhi/fnos-logmanager/internal/middleware"
	"github.com/sushazhi/fnos-logmanager/internal/services"
	"github.com/sushazhi/fnos-logmanager/internal/types"
)

// RegisterEventLoggerRoutes registers event logger routes under the given router group.
func RegisterEventLoggerRoutes(rg *gin.RouterGroup) {
	rg.GET("/status", middleware.ValidateToken, eventLoggerStatusHandler)
	rg.GET("/config", middleware.ValidateToken, eventLoggerGetConfigHandler)
	rg.PUT("/config", middleware.ValidateToken, middleware.ValidateCSRF, eventLoggerUpdateConfigHandler)
	rg.GET("/stats", middleware.ValidateToken, eventLoggerStatsHandler)
	rg.GET("/sources", middleware.ValidateToken, eventLoggerSourcesHandler)
	rg.GET("/events", middleware.ValidateToken, eventLoggerEventsHandler)
	rg.POST("/check", middleware.ValidateToken, middleware.ValidateCSRF, eventLoggerCheckHandler)
	rg.POST("/start", middleware.ValidateToken, middleware.ValidateCSRF, eventLoggerStartHandler)
	rg.POST("/stop", middleware.ValidateToken, middleware.ValidateCSRF, eventLoggerStopHandler)
	rg.POST("/restart", middleware.ValidateToken, middleware.ValidateCSRF, eventLoggerRestartHandler)
}

func eventLoggerStatusHandler(c *gin.Context) {
	status := services.GetEventLoggerStatus()
	c.JSON(http.StatusOK, status)
}

func eventLoggerGetConfigHandler(c *gin.Context) {
	cfg := config.Get().EventLogger
	c.JSON(http.StatusOK, gin.H{
		"dbPath":               cfg.DBPath,
		"enabled":              cfg.Enabled,
		"checkInterval":        cfg.CheckInterval,
		"eventTypes":           cfg.EventTypes,
		"minSeverity":          cfg.MinSeverity,
		"notificationChannels": cfg.NotificationChannels,
	})
}

type updateEventLoggerConfigRequest struct {
	DBPath               *string  `json:"dbPath"`
	Enabled              *bool    `json:"enabled"`
	CheckInterval        *int     `json:"checkInterval"`
	EventTypes           []string `json:"eventTypes"`
	MinSeverity          *string  `json:"minSeverity"`
	NotificationChannels []string `json:"notificationChannels"`
}

func eventLoggerUpdateConfigHandler(c *gin.Context) {
	var req updateEventLoggerConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		cfg := config.Get().EventLogger
		c.JSON(http.StatusOK, gin.H{
			"dbPath":               cfg.DBPath,
			"enabled":              cfg.Enabled,
			"checkInterval":        cfg.CheckInterval,
			"eventTypes":           cfg.EventTypes,
			"minSeverity":          cfg.MinSeverity,
			"notificationChannels": cfg.NotificationChannels,
		})
		return
	}

	// Apply updates to current config
	cur := config.Get().EventLogger
	if req.DBPath != nil {
		cur.DBPath = *req.DBPath
	}
	if req.Enabled != nil {
		cur.Enabled = *req.Enabled
	}
	if req.CheckInterval != nil {
		cur.CheckInterval = *req.CheckInterval
	}
	if len(req.EventTypes) > 0 {
		cur.EventTypes = req.EventTypes
	}
	if req.MinSeverity != nil {
		cur.MinSeverity = *req.MinSeverity
	}
	if len(req.NotificationChannels) > 0 {
		cur.NotificationChannels = req.NotificationChannels
	}

	// Persist to disk so it survives restart
	if err := config.UpdateEventLoggerConfig(cur); err != nil {
		slog.Error("failed to save event logger config", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存配置失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":              true,
		"dbPath":               cur.DBPath,
		"enabled":              cur.Enabled,
		"checkInterval":        cur.CheckInterval,
		"eventTypes":           cur.EventTypes,
		"minSeverity":          cur.MinSeverity,
		"notificationChannels": cur.NotificationChannels,
	})
}

func eventLoggerStatsHandler(c *gin.Context) {
	stats := types.EventLoggerStats{
		TotalEvents: 0,
	}
	entries, total, err := services.GetEventLogs(services.EventLogFilter{Limit: 1000})
	if err == nil {
		stats.TotalEvents = total
		stats.RecentEvents = entries
		if len(stats.RecentEvents) > 100 {
			stats.RecentEvents = stats.RecentEvents[:100]
		}
	}
	c.JSON(http.StatusOK, stats)
}

func eventLoggerSourcesHandler(c *gin.Context) {
	sources := services.GetEventSources()
	if sources == nil {
		sources = []string{}
	}
	c.JSON(http.StatusOK, sources)
}

// eventLogsQuery defines validated query parameters for GET /events.
type eventLogsQuery struct {
	Limit     int    `form:"limit"     binding:"omitempty,min=1,max=1000"`
	Offset    int    `form:"offset"    binding:"omitempty,min=0"`
	Severity  string `form:"severity"  binding:"omitempty,oneof=debug info warning error critical"`
	Source    string `form:"source"    binding:"omitempty,max=128"`
	EventType string `form:"eventType" binding:"omitempty,max=64"`
	Search    string `form:"search"    binding:"omitempty,max=256"`
	StartTime string `form:"startTime" binding:"omitempty,max=64"`
	EndTime   string `form:"endTime"   binding:"omitempty,max=64"`
}

func eventLoggerEventsHandler(c *gin.Context) {
	var q eventLogsQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	// Apply defaults
	if q.Limit <= 0 {
		q.Limit = 100
	}

	entries, total, err := services.GetEventLogs(services.EventLogFilter{
		Limit:     q.Limit,
		Offset:    q.Offset,
		Severity:  q.Severity,
		Source:    q.Source,
		EventType: q.EventType,
		Search:    q.Search,
		StartTime: q.StartTime,
		EndTime:   q.EndTime,
	})
	if err != nil {
		entries = []types.EnhancedEventLogEntry{}
	}

	if entries == nil {
		entries = []types.EnhancedEventLogEntry{}
	}

	c.JSON(http.StatusOK, gin.H{
		"events": entries,
		"total":  total,
		"limit":  q.Limit,
		"offset": q.Offset,
	})
}

func eventLoggerCheckHandler(c *gin.Context) {
	if err := services.ForceEventLoggerCheck(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Check completed"})
}

func eventLoggerStartHandler(c *gin.Context) {
	status := services.GetEventLoggerStatus()

	// Auto-initialize if not yet initialized (e.g. EVENTLOGGER_ENABLED=false at startup)
	if !status.DBAccessible {
		cfg := config.Get()
		slog.Info("event logger not initialized, initializing now", "dbPath", cfg.EventLogger.DBPath)
		if err := services.InitEventLogger(cfg.EventLogger.DBPath); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "初始化事件日志失败: " + err.Error()})
			return
		}
	}

	if err := services.StartEventLogger(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Persist enabled state so it auto-starts after restart (matching TS behavior)
	cur := config.Get().EventLogger
	cur.Enabled = true
	if err := config.UpdateEventLoggerConfig(cur); err != nil {
		slog.Warn("failed to persist event logger enabled state", "error", err)
	}

	c.JSON(http.StatusOK, services.GetEventLoggerStatus())
}

func eventLoggerStopHandler(c *gin.Context) {
	if err := services.StopEventLogger(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Persist disabled state so it stays stopped after restart
	cur := config.Get().EventLogger
	cur.Enabled = false
	if err := config.UpdateEventLoggerConfig(cur); err != nil {
		slog.Warn("failed to persist event logger disabled state", "error", err)
	}

	c.JSON(http.StatusOK, services.GetEventLoggerStatus())
}

func eventLoggerRestartHandler(c *gin.Context) {
	services.CloseEventLogger()
	cfg := config.Get()
	if err := services.InitEventLogger(cfg.EventLogger.DBPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "重新初始化事件日志失败: " + err.Error()})
		return
	}
	if err := services.StartEventLogger(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, services.GetEventLoggerStatus())
}
