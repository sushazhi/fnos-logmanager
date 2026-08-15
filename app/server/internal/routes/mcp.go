package routes

import (
	"fmt"
	"net"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/sushazhi/fnos-logmanager/internal/config"
	"github.com/sushazhi/fnos-logmanager/internal/middleware"
	"github.com/sushazhi/fnos-logmanager/internal/services"
)

// mcpConfigResponse is the MCP settings object returned to and accepted from
// the frontend. The API key is write-only: it is returned masked and only a
// non-empty value is persisted.
type mcpConfigResponse struct {
	Enabled  bool   `json:"enabled"`
	APIKey   string `json:"apiKey,omitempty"`
	AppName  string `json:"appName"`
	Port     int    `json:"port"`
	BindAddr string `json:"bindAddr"`
	// Endpoint is a read-only hint for the agent, based on the configured port.
	Endpoint string `json:"endpoint"`
	// HostIP is the NAS LAN IP, used by the frontend to replace the <NAS-IP>
	// placeholder in the endpoint label.
	HostIP string `json:"hostIp"`
}

// registerMCPConfigRoutes mounts the MCP configuration management endpoints.
// These endpoints manage the MCP API key and independent listener port, so they
// must be protected like other sensitive configuration endpoints:
//   - GET requires an authenticated session
//   - PUT additionally requires admin privileges and CSRF validation
func registerMCPConfigRoutes(api *gin.RouterGroup) {
	api.GET("/mcp/config", middleware.ValidateToken, getMCPConfig)
	api.PUT("/mcp/config", middleware.ValidateToken, middleware.RequireAdmin, middleware.ValidateCSRF, saveMCPConfig)
}

// getMCPConfig returns the effective MCP configuration. The API key is masked.
func getMCPConfig(c *gin.Context) {
	cfg := config.LoadMCPConfig()

	// 外部 AI Agent（QwenPAW/OpenClaw/Hermes）无法通过 fnOS 网关认证
	// （网关会拦截 /app/logmanager/mcp 并返回 "invalid token"），因此必须通过
	// 独立端口访问。未配置独立端口时 endpoint 返回空，前端提示需配置端口。
	endpoint := ""
	if cfg.Port > 0 {
		endpoint = fmt.Sprintf(":<%d>/mcp", cfg.Port)
	}

	c.JSON(http.StatusOK, mcpConfigResponse{
		Enabled:  cfg.Enabled,
		APIKey:   maskKey(cfg.APIKey),
		AppName:  cfg.AppName,
		Port:     cfg.Port,
		BindAddr: cfg.BindAddr,
		Endpoint: endpoint,
		HostIP:   getLocalIP(),
	})
}

// getLocalIP returns the primary non-loopback IPv4 address of this host,
// used to substitute the <NAS-IP> placeholder in the MCP endpoint label.
func getLocalIP() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil || ip.IsLoopback() || ip.To4() == nil {
				continue
			}
			return ip.String()
		}
	}
	return ""
}

// saveMCPConfig persists the MCP configuration.
func saveMCPConfig(c *gin.Context) {
	var req mcpConfigResponse
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求参数"})
		return
	}

	if req.Port < 0 || req.Port > 65535 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的端口号"})
		return
	}

	current := config.ReadMCPFileConfig()
	fileCfg := config.MCPFileConfig{
		Enabled:  &req.Enabled,
		AppName:  &req.AppName,
		Port:     &req.Port,
		BindAddr: &req.BindAddr,
	}

	// Preserve the existing API key unless a new one was provided (we never
	// return the real key to the client, so an empty input means "keep it").
	key := req.APIKey
	if key == "" && current.APIKey != nil {
		key = *current.APIKey
	}
	fileCfg.APIKey = &key

	if err := config.SaveMCPConfig(fileCfg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存 MCP 配置失败"})
		return
	}

	// Runtime API key / enabled take effect immediately; the dedicated listener
	// port requires a process restart, which the frontend is told about.
	services.AddSecurityAuditLog("MCP_CONFIG_UPDATE", map[string]interface{}{
		"enabled": req.Enabled, "port": req.Port,
	}, c)

	oldPort := currentPort(current)
	portChanged := req.Port != oldPort
	c.JSON(http.StatusOK, gin.H{
		"ok":              true,
		"portChanged":     portChanged,
		"requiresRestart": portChanged && req.Port != 0,
	})
}

// currentPort returns the currently persisted port (0 if unset).
func currentPort(cfg config.MCPFileConfig) int {
	if cfg.Port != nil {
		return *cfg.Port
	}
	return 0
}

// maskKey masks most of the API key for display.
func maskKey(key string) string {
	if key == "" {
		return ""
	}
	if len(key) <= 4 {
		return "****"
	}
	return key[:2] + "****" + key[len(key)-2:]
}
