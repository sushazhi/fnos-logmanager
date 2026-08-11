package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// mcpSettings manages the MCP configuration persisted to
// <DataDir>/mcp-config.json. Environment variables provide defaults; the JSON
// file overrides them at runtime so the frontend can configure MCP without
// restarting the process (note: the dedicated listener port still requires a
// restart to take effect).
type mcpSettings struct {
	mu       sync.Mutex
	filePath string
}

var mcpCfg = &mcpSettings{}

// MCPFileConfig is the on-disk representation of the MCP settings.
type MCPFileConfig struct {
	Enabled  *bool   `json:"enabled,omitempty"`
	APIKey   *string `json:"apiKey,omitempty"`
	AppName  *string `json:"appName,omitempty"`
	Port     *int    `json:"port,omitempty"`
	BindAddr *string `json:"bindAddr,omitempty"`
}

// mcpFilePath returns the path to the MCP config file for the given data dir.
func mcpFilePath(dataDir string) string {
	return filepath.Join(dataDir, "mcp-config.json")
}

// LoadMCPConfig merges environment defaults with the persisted file config and
// returns the effective MCP configuration.
func LoadMCPConfig() MCPConfig {
	base := MCPConfig{
		Enabled:  getEnvBool("MCP_ENABLED", true),
		APIKey:   os.Getenv("MCP_API_KEY"),
		AppName:  getEnvOrDefault("MCP_APP_NAME", "fnos-logmanager"),
		Port:     getEnvInt("MCP_PORT", 0),
		BindAddr: getEnvOrDefault("MCP_BIND_ADDR", "0.0.0.0"),
	}

	fileCfg := ReadMCPFileConfig()
	if fileCfg.Enabled != nil {
		base.Enabled = *fileCfg.Enabled
	}
	if fileCfg.APIKey != nil {
		base.APIKey = *fileCfg.APIKey
	}
	if fileCfg.AppName != nil {
		base.AppName = *fileCfg.AppName
	}
	if fileCfg.Port != nil {
		base.Port = *fileCfg.Port
	}
	if fileCfg.BindAddr != nil {
		base.BindAddr = *fileCfg.BindAddr
	}
	return base
}

// currentDataDir returns the data directory from the environment, matching
// loadConfig(). It intentionally does NOT call Get() to avoid a sync.Once
// re-entrancy deadlock during initial config loading.
func currentDataDir() string {
	return getEnvOrDefault("LOGMANAGER_DATA_DIR", "/vol1/@appdata/logmanager")
}

// ReadMCPFileConfig reads the persisted MCP config file (if any).
func ReadMCPFileConfig() MCPFileConfig {
	mcpCfg.mu.Lock()
	defer mcpCfg.mu.Unlock()
	mcpCfg.filePath = mcpFilePath(currentDataDir())

	var out MCPFileConfig
	data, err := os.ReadFile(mcpCfg.filePath)
	if err != nil {
		return out
	}
	_ = json.Unmarshal(data, &out)
	return out
}

// SaveMCPConfig persists the MCP configuration to disk.
func SaveMCPConfig(cfg MCPFileConfig) error {
	mcpCfg.mu.Lock()
	defer mcpCfg.mu.Unlock()

	mcpCfg.filePath = mcpFilePath(currentDataDir())
	dir := filepath.Dir(mcpCfg.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(mcpCfg.filePath, data, 0644)
}
