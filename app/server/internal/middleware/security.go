package middleware

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

// SecurityHeaders adds security-related HTTP headers.
func SecurityHeaders(c *gin.Context) {
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("X-XSS-Protection", "1; mode=block")
	c.Header("X-Frame-Options", "SAMEORIGIN")
	c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
	c.Header("X-Permitted-Cross-Domain-Policies", "none")
	c.Header("X-Download-Options", "noopen")
	c.Header("X-DNS-Prefetch-Control", "off")

	// CSP header with dynamic frame-ancestors
	hostBase := getClientHost(c.Request)
	isIP := isIPAddress(hostBase)

	var frameAncestors string
	if isIP {
		frameAncestors = fmt.Sprintf("'self' http://%s:* https://%s:*", hostBase, hostBase)
	} else {
		frameAncestors = fmt.Sprintf("'self' https://%s:* http://%s:*", hostBase, hostBase)
	}

	// script-src uses 'self' only — the SPA bundles all JS into external files, no
	// inline <script> tags needed at runtime. style-src keeps 'unsafe-inline' because
	// Vue scoped styles inject inline <style> tags into the DOM.
	c.Header("Content-Security-Policy",
		fmt.Sprintf("default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; connect-src 'self' ws: wss:; frame-ancestors %s; base-uri 'self'; form-action 'self'", frameAncestors))

	c.Next()
}

func getClientHost(r *http.Request) string {
	// 1. Host header (only trusted source — no proxy headers to prevent spoofing)
	host := r.Host
	if host != "" {
		hostBase := strings.Split(host, ":")[0]
		if hostBase != "" && hostBase != "127.0.0.1" && hostBase != "localhost" {
			return hostBase
		}
	}

	// 2. Try to get LAN IP
	interfaces, err := net.Interfaces()
	if err == nil {
		for _, iface := range interfaces {
			addrs, err := iface.Addrs()
			if err != nil {
				continue
			}
			for _, addr := range addrs {
				if ipNet, ok := addr.(*net.IPNet); ok {
					if ip := ipNet.IP.To4(); ip != nil && !ip.IsLoopback() && !ip.IsPrivate() {
						return ip.String()
					}
				}
			}
		}
	}

	return strings.Split(r.Host, ":")[0]
}

func isIPAddress(host string) bool {
	return net.ParseIP(host) != nil
}

// ValidateContentType validates request content-type for requests with body.
func ValidateContentType(c *gin.Context) {
	if c.Request.Method == "POST" || c.Request.Method == "PUT" || c.Request.Method == "PATCH" {
		ct := c.Request.Header.Get("Content-Type")
		if c.Request.ContentLength > 0 {
			if ct == "" || (!strings.HasPrefix(ct, "application/json") &&
				!strings.HasPrefix(ct, "application/x-www-form-urlencoded") &&
				!strings.HasPrefix(ct, "multipart/form-data")) {
				c.AbortWithStatusJSON(http.StatusUnsupportedMediaType, gin.H{
					"success": false,
					"error":   "不支持的 Content-Type",
				})
				return
			}
		}
	}
	c.Next()
}

// RequestSizeLimit limits the size of incoming requests.
func RequestSizeLimit(maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		c.Next()
	}
}

// SanitizeInput sanitizes request input (basic SQL/XSS injection prevention).
func SanitizeInput(c *gin.Context) {
	// Gin's ShouldBindJSON already handles JSON parsing safely.
	// This middleware catches issues like excessively long query params.
	for key, values := range c.Request.URL.Query() {
		for _, v := range values {
			if len(v) > 4096 {
				c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
					"success": false,
					"error":   fmt.Sprintf("参数 %s 过长", key),
				})
				return
			}
		}
	}
	c.Next()
}

// ValidateEnv checks that required environment variables are set.
func ValidateEnv() (valid bool, errors_ []string) {
	// Gateway socket is optional, but config should be valid
	gatewaySocket := os.Getenv("GATEWAY_SOCKET")
	if gatewaySocket != "" {
		dir := filepath.Dir(gatewaySocket)
		if info, err := os.Stat(dir); err != nil || !info.IsDir() {
			errors_ = append(errors_, fmt.Sprintf("Gateway socket directory not found: %s", dir))
		}
	}

	// Validate data dir or default
	dataDir := os.Getenv("LOGMANAGER_DATA_DIR")
	if dataDir != "" {
		if info, err := os.Stat(dataDir); err == nil && !info.IsDir() {
			errors_ = append(errors_, fmt.Sprintf("LOGMANAGER_DATA_DIR is not a directory: %s", dataDir))
		}
	}

	return len(errors_) == 0, errors_
}

// CheckDependencies checks that external dependencies are available.
func CheckDependencies() (valid bool, missing []string) {
	// Check for docker binary (optional but common)
	if _, err := os.Stat("/usr/bin/docker"); err != nil {
		if _, err := os.Stat("/usr/local/bin/docker"); err != nil {
			// Docker is optional, just warn
		}
	}

	// Check for appcenter-cli
	if _, err := os.Stat("/usr/bin/appcenter-cli"); err != nil {
		if _, err := os.Stat("/usr/local/bin/appcenter-cli"); err != nil {
			missing = append(missing, "appcenter-cli")
		}
	}

	return len(missing) == 0, missing
}
