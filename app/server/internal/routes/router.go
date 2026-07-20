package routes

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/sushazhi/fnos-logmanager/internal/config"
	"github.com/sushazhi/fnos-logmanager/internal/middleware"
)

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

	// API routes
	api := r.Group("/api")
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
