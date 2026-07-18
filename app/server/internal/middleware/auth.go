package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/sushazhi/fnos-logmanager/internal/config"
	apperrors "github.com/sushazhi/fnos-logmanager/internal/errors"
	"github.com/sushazhi/fnos-logmanager/internal/services"
	"github.com/sushazhi/fnos-logmanager/internal/utils"
)

// AuthenticatedRequest extends gin.Context with auth info.
type AuthenticatedRequest struct {
	*gin.Context
	ClientIP    string
	SessionToken string
	GatewayUID  string
}

// GetSessionToken extracts the session token from the request.
func GetSessionToken(c *gin.Context) string {
	// Check cookie first
	token, err := c.Cookie("session_token")
	if err == nil && token != "" {
		return token
	}

	// Check Authorization header
	auth := c.GetHeader("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return auth[7:]
	}

	// Check query parameter (limited paths only)
	queryToken := c.Query("token")
	if queryToken != "" {
		cfg := config.Get()
		if cfg.Auth.AllowQueryToken {
			for _, path := range cfg.Auth.QueryTokenPaths {
				if c.Request.URL.Path == path {
					return queryToken
				}
			}
		}
	}

	return ""
}

// ValidateToken middleware validates the session token.
func ValidateToken(c *gin.Context) {
	clientIP := utils.GetClientIP(c.Request)
	token := GetSessionToken(c)

	isGatewayMode := config.IsGatewayMode()
	if isGatewayMode {
		uid := c.GetHeader("X-Trim-Uid")
		if uid != "" {
			// Gateway mode: gateway handles auth, set local session for WS
			sessionToken := services.CreateSession(uid)
			c.Set("sessionToken", sessionToken)
			c.Set("clientIP", clientIP)
			c.Set("gatewayUID", uid)
			c.Next()
			return
		}
	}

	// Direct mode: validate session
	if token == "" || !services.ValidateSession(token) {
		// Log audit
		services.AddAuditLog("auth_failed", map[string]interface{}{
			"path":     c.Request.URL.Path,
			"clientIP": clientIP,
		}, c)

		c.Error(apperrors.NewAuthenticationError("需要认证"))
		c.Abort()
		return
	}

	c.Set("sessionToken", token)
	c.Set("clientIP", clientIP)
	c.Next()
}

// ValidateCSRF middleware validates the CSRF token.
func ValidateCSRF(c *gin.Context) {
	// Gateway mode: skip CSRF check
	if config.IsGatewayMode() {
		c.Next()
		return
	}

	token, _ := c.Get("sessionToken")
	sessionToken, _ := token.(string)

	csrfToken := c.GetHeader("X-CSRF-Token")
	if csrfToken == "" {
		csrfToken = c.Query("csrf_token")
	}

	if sessionToken == "" || !services.ValidateCSRFToken(sessionToken, csrfToken) {
		clientIP := utils.GetClientIP(c.Request)
		services.AddAuditLog("csrf_failed", map[string]interface{}{
			"path":     c.Request.URL.Path,
			"clientIP": clientIP,
		}, c)

		c.Error(apperrors.NewCSRFError("CSRF验证失败"))
		c.Abort()
		return
	}

	c.Next()
}

// CheckValidation checks for express-validator style validation errors.
// In Go we use Gin's binding/validation instead.
func CheckValidation(c *gin.Context) {
	// Gin's ShouldBindJSON etc already handles this.
	// If binding fails, c.Errors will contain the error.
	if len(c.Errors) > 0 {
		c.Abort()
		return
	}
	c.Next()
}

// GetSessionTokenValue extracts the session token value from context.
func GetSessionTokenValue(c *gin.Context) string {
	if token, exists := c.Get("sessionToken"); exists {
		return token.(string)
	}
	return ""
}
