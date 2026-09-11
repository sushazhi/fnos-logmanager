package middleware

import (
	"log/slog"
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
	ClientIP     string
	SessionToken string
	GatewayUID   string
}

// SessionTokenHeader 是前端携带应用会话 token 的自定义请求头名。
//
// 切勿改用标准的 Authorization 头：fnOS 1.2.0604 起，统一网关会把请求中的
// `Authorization: Bearer <token>` 当成 fnOS 自身的票据（ticket/token）先行校验，
// 校验不通过时直接短路返回纯文本 `invalid token`（HTTP 200），请求根本不会到达本服务，
// 前端表现为 `Unexpected token 'i', "invalid token" is not valid JSON`。
const SessionTokenHeader = "X-Session-Token"

// tokenFromHeader 依次从自定义头与（历史遗留的）Authorization 头读取 token。
func tokenFromHeader(c *gin.Context) string {
	if t := c.GetHeader(SessionTokenHeader); t != "" {
		return t
	}
	// 兼容旧版前端：仍接受 Authorization: Bearer（该请求只有在网关放行后才会到达这里）
	auth := c.GetHeader("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return auth[7:]
	}
	return ""
}

// GetSessionToken extracts the session token from the request.
func GetSessionToken(c *gin.Context) string {
	// Check cookie first
	token, err := c.Cookie("session_token")
	if err == nil && token != "" {
		return token
	}

	// Check custom header / legacy Authorization header
	if headerToken := tokenFromHeader(c); headerToken != "" {
		return headerToken
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
	isGatewayMode := config.IsGatewayMode()

	if isGatewayMode {
		uid := c.GetHeader("X-Trim-Userid")
		// The X-Trim-Userid header is only trustworthy when the connection
		// genuinely comes from the local gateway proxy (loopback). Otherwise
		// a remote client could forge the header and impersonate any user.
		if uid != "" && utils.IsLoopbackAddr(c.Request.RemoteAddr) {
			slog.Debug("auth gateway ok: X-Trim-Userid present",
				"uid", uid, "path", c.Request.URL.Path, "clientIP", clientIP)
			sessionToken := services.GetOrCreateUserSession(uid)
			c.Set("sessionToken", sessionToken)
			c.Set("clientIP", clientIP)
			c.Set("gatewayUID", uid)
			c.Next()
			return
		}
		if uid != "" {
			slog.Warn("auth gateway: X-Trim-Userid from non-loopback ignored",
				"uid", uid, "path", c.Request.URL.Path, "remoteAddr", c.Request.RemoteAddr)
		}
		slog.Debug("auth gateway: X-Trim-Userid missing, falling to token",
			"path", c.Request.URL.Path, "clientIP", clientIP)
	}

	// Try multiple token sources: cookie → Bearer → query
	// resolveToken only returns a token if its session validates,
	// or "" if no token source produced a valid session.
	token := resolveToken(c)
	if token != "" {
		c.Set("sessionToken", token)
		c.Set("clientIP", clientIP)
		c.Next()
		return
	}

	slog.Warn("auth FAILED",
		"path", c.Request.URL.Path, "clientIP", clientIP,
		"isGatewayMode", isGatewayMode)

	services.AddAuditLog("auth_failed", map[string]interface{}{
		"path":     c.Request.URL.Path,
		"clientIP": clientIP,
	}, c)

	c.Error(apperrors.NewAuthenticationError("需要认证"))
	c.Abort()
}

// resolveToken tries multiple token sources in order: cookie → Bearer → query.
// It validates each source's token and returns the first valid one.
// Returns "" if no valid token is found from any source.
func resolveToken(c *gin.Context) string {
	// 1) Cookie
	token, err := c.Cookie("session_token")
	if err == nil && token != "" {
		if services.ValidateSession(token) {
			return token
		}
	}

	// 2) Custom header (X-Session-Token) / legacy Authorization: Bearer header
	if headerToken := tokenFromHeader(c); headerToken != "" {
		if services.ValidateSession(headerToken) {
			return headerToken
		}
	}

	// 3) Query param (whitelisted paths only)
	queryToken := c.Query("token")
	if queryToken != "" {
		cfg := config.Get()
		if cfg.Auth.AllowQueryToken {
			for _, path := range cfg.Auth.QueryTokenPaths {
				if c.Request.URL.Path == path && services.ValidateSession(queryToken) {
					return queryToken
				}
			}
		}
	}

	return ""
}

// RequireAdmin middleware ensures the request is made by an administrator.
// In gateway mode the fnOS gateway injects x-trim-isadmin for admin users. The
// admin flag header is only trusted when the request genuinely comes from the
// local gateway proxy (loopback); a remote client must not be able to forge it
// to escalate privileges.
// In standalone (single-user) mode every authenticated session is an admin.
func RequireAdmin(c *gin.Context) {
	if config.IsGatewayMode() {
		isAdminHeader := c.GetHeader("x-trim-isadmin") == "true"
		// Only trust the admin flag from the local gateway (loopback).
		trustedAdmin := isAdminHeader && utils.IsLoopbackAddr(c.Request.RemoteAddr)
		if !trustedAdmin {
			services.AddAuditLog("admin_required", map[string]interface{}{
				"path": c.Request.URL.Path,
			}, c)

			c.Error(apperrors.NewAppError("需要管理员权限", 403, "FORBIDDEN"))
			c.Abort()
			return
		}
	}
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
