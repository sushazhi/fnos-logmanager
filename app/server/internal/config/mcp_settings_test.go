package config

import (
	"testing"
	"time"
)

// TestLoadConfigNoDeadlock guards against a sync.Once re-entrancy deadlock:
// loadConfig() -> LoadMCPConfig() -> ReadMCPFileConfig() must NOT call Get()
// again (which would block forever). This regression was the cause of a
// Bad Gateway on startup.
func TestLoadConfigNoDeadlock(t *testing.T) {
	done := make(chan struct{})
	go func() {
		defer close(done)
		// loadConfig exercises the LoadMCPConfig path without relying on the
		// singleton once. It must complete without hanging.
		cfg := loadConfig()
		if cfg == nil {
			t.Error("loadConfig() returned nil")
		}
		_ = cfg.MCP
	}()

	select {
	case <-done:
		// ok, no deadlock
	case <-time.After(5 * time.Second):
		t.Fatal("loadConfig() deadlocked (sync.Once re-entrancy)")
	}
}

// TestGetNoDeadlock verifies Get() completes in a fresh process without the
// re-entrancy hang.
func TestGetNoDeadlock(t *testing.T) {
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = Get()
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Get() deadlocked")
	}
}

// TestReadMCPFileConfigUsesEnv verifies the config file is read from the data
// dir derived from the environment, not from the (possibly not-yet-initialized)
// singleton.
func TestReadMCPFileConfigUsesEnv(t *testing.T) {
	tmp := t.TempDir()
	// Set env before calling so currentDataDir() resolves to the temp dir.
	t.Setenv("LOGMANAGER_DATA_DIR", tmp)
	_ = SaveMCPConfig(MCPFileConfig{Enabled: boolPtr(true), Port: intPtr(8090)})

	cfg := ReadMCPFileConfig()
	if cfg.Enabled == nil || !*cfg.Enabled {
		t.Error("expected enabled=true from saved config")
	}
	if cfg.Port == nil || *cfg.Port != 8090 {
		t.Errorf("expected port 8090, got %v", cfg.Port)
	}
}

func boolPtr(b bool) *bool { return &b }
func intPtr(i int) *int    { return &i }
