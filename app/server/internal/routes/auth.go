package routes

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/sushazhi/fnos-logmanager/internal/config"
	"github.com/sushazhi/fnos-logmanager/internal/middleware"
	"github.com/sushazhi/fnos-logmanager/internal/services"
)

func RegisterAuthRoutes(rg *gin.RouterGroup) {
	rg.POST("/logout", middleware.ValidateToken, middleware.ValidateCSRF, logoutHandler)
	rg.GET("/status", statusHandler)
	rg.GET("/csrf-token", csrfTokenHandler)
}

func logoutHandler(c *gin.Context) {
	sessionToken := middleware.GetSessionToken(c)
	if sessionToken != "" {
		services.DeleteSession(sessionToken)
	}
	c.SetCookie("session_token", "", -1, "/", "", false, true)
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func statusHandler(c *gin.Context) {
	cfg := config.Get()
	isGatewayMode := cfg.GatewaySocket != ""

	if isGatewayMode {
		sessionToken := services.CreateSession("local")
		c.SetCookie("session_token", sessionToken, 86400, "/", "", false, true)
		c.JSON(http.StatusOK, gin.H{
			"initialized":   true,
			"isLoggedIn":    true,
			"isAdmin":       c.GetHeader("x-trim-isadmin") == "true",
			"username":      c.GetHeader("x-trim-username"),
			"sessionToken":  sessionToken,
			"sessionExpiry": 86400000,
		})
		return
	}

	sessionToken := middleware.GetSessionToken(c)
	isLoggedIn := sessionToken != "" && services.ValidateSession(sessionToken)

	if !isLoggedIn {
		sessionToken = services.CreateSession("local")
		c.SetCookie("session_token", sessionToken, 86400, "/", "", false, true)
		isLoggedIn = true
	}

	csrfToken := services.GetCSRFToken(sessionToken)
	c.JSON(http.StatusOK, gin.H{
		"initialized":   true,
		"isLoggedIn":    isLoggedIn,
		"csrfToken":     csrfToken,
		"sessionToken":  sessionToken,
		"sessionExpiry": 86400000,
	})
}

func csrfTokenHandler(c *gin.Context) {
	sessionToken := middleware.GetSessionToken(c)
	if sessionToken != "" && services.ValidateSession(sessionToken) {
		csrfToken := services.GetCSRFToken(sessionToken)
		c.JSON(http.StatusOK, gin.H{"csrfToken": csrfToken})
	} else {
		c.JSON(http.StatusOK, gin.H{"csrfToken": nil})
	}
}
