package routes

import (
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/sushazhi/fnos-logmanager/internal/config"
	"github.com/sushazhi/fnos-logmanager/internal/mcp"
	"github.com/sushazhi/fnos-logmanager/internal/middleware"
	"github.com/sushazhi/fnos-logmanager/internal/services"
)

// mcpServer is the global MCP Streamable HTTP server instance, initialized in
// SetupRouter from configuration.
var mcpServer *mcp.Server

// GetMCPServer returns the MCP Streamable HTTP server instance (may be nil if
// MCP is disabled). Used by main to optionally expose the endpoint on a
// dedicated listener for external AI agents.
func GetMCPServer() *mcp.Server {
	return mcpServer
}

// SetupRouter configures all routes and middleware on the Gin engine.
func SetupRouter(uiDir string) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)

	r := gin.New()
	r.Use(gin.Recovery())

	// Apply security and parsing middleware
	r.Use(middleware.SecurityHeaders)
	r.Use(middleware.ValidateContentType)
	r.Use(middleware.SanitizeInput)
	r.Use(middleware.RateLimit)

	// Express-equivalent static file middleware: tries to serve ANY request path from uiDir.
	// Unlike r.Static("/assets", dir) which is prefix-locked, this handles cases where
	// gateway prefix differs or URL resolution produces non-standard paths.
	// (e.g. browser at /app/logmanager resolves ./assets/foo.css to /app/assets/foo.css)
	r.Use(middleware.ServeStatic(uiDir))

	// SPA fallback: serve index.html with injected <base> tag so relative asset URLs
	// (from Vite's base: './') resolve against the gateway prefix, not the browser URL.
	r.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api") {
			middleware.NotFoundHandler(c)
			return
		}
		// Injects <base href="/app/logmanager/"> so ./assets/foo.css resolves
		// to /app/logmanager/assets/foo.css regardless of current SPA route.
		baseHref := config.Get().GatewayPrefix
		middleware.ServeIndexWithBase(uiDir, baseHref)(c)
	})

	// Global error handler
	r.Use(middleware.ErrorHandler)

	// MCP (Model Context Protocol) Streamable HTTP server.
	// 仅初始化全局 MCP server 实例，供独立端口监听器使用（外部 AI Agent 通过
	// 独立端口访问，绕过 fnOS 网关认证）。网关路径的 /mcp 路由已移除。
	registerMCPRoute(r)

	// API routes
	api := r.Group("/api")

	// MCP configuration management (frontend settings panel).
	registerMCPConfigRoutes(api)
	{
		// Auth routes (no auth needed for login)
		auth := api.Group("/auth")
		auth.Use(middleware.APIRateLimit(30, 60000)) // 30 req/min per IP for auth endpoints
		RegisterAuthRoutes(auth)

		// Log routes
		RegisterLogRoutes(api)

		// Docker routes
		RegisterDockerRoutes(api)

		// Update routes
		update := api.Group("/update")
		RegisterUpdateRoutes(update)

		// Notification routes
		notifications := api.Group("/notifications")
		RegisterNotificationRoutes(notifications)

		// Event logger routes
		eventLogger := api.Group("/eventlogger")
		RegisterEventLoggerRoutes(eventLogger)

		// Kernel module routes
		RegisterKernelRoutes(api)

		// Utility routes (P2: path conversion via fnOS trim API)
		utils := api.Group("/utils")
		RegisterUtilRoutes(utils)
	}

	// Health endpoint (no auth)
	r.GET("/api/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":  "ok",
			"version": config.Get().GatewayPrefix,
		})
	})

	return r
}

// registerMCPRoute initializes the MCP Streamable HTTP server.
// It reads its configuration from the shared config and environment.
// 仅创建全局 MCP server 实例供独立端口监听器使用，不再注册网关路径的 /mcp 路由：
// fnOS 统一网关会拦截该路径（返回 "invalid token"，外部 AI Agent 无 fnOS 登录态
// 连不上），该网关入口形同虚设。外部 AI Agent（QwenPAW/OpenClaw/Hermes）统一通过
// 独立端口访问（受 MCP_API_KEY 保护），因此移除多余的网关 MCP 路由。
func registerMCPRoute(_ *gin.Engine) {
	version := os.Getenv("TRIM_APPVER")
	if version == "" {
		version = "0.0.0"
	}
	// 实时读取 MCP 配置（mcp-config.json），使 apiKey / enabled 保存后立即生效。
	// enabled / apiKey 由 isAuthorized 每次请求实时读取（LoadMCPConfig），
	// 因此在设置界面"启用 MCP 服务器"后立即生效，无需重启进程。
	liveCfg := config.LoadMCPConfig()
	appName := liveCfg.AppName
	if appName == "" {
		appName = "fnos-logmanager"
	}

	mcpServer = mcp.New(appName, version, liveCfg.APIKey)

	// Initialize the trim API client early so MCP tools that rely on it are ready.
	services.GetTrimClient()
}
