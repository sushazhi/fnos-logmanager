package services

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestSimulatedAppdataScanIsFastAndComplete 在模拟的 @appdata 结构上验证性能修复。
//
// 该结构复刻真机实测的形态：一个应用（deepseek.harness）贡献了绝大部分条目
// （node_modules / pnpm-home / npm-cache），但只有 3 个真日志；其余是噪音。
// 测试同时断言「快」与「不丢日志」两个目标。
func TestSimulatedAppdataScanIsFastAndComplete(t *testing.T) {
	if os.Getenv("APPDATA_SIM") == "" {
		t.Skip("set APPDATA_SIM to the simulated tree root to run")
	}
	root := os.Getenv("APPDATA_SIM")

	InvalidateDirScanCache()
	start := time.Now()
	scan := ScanDirResult(root)
	elapsed := time.Since(start)

	if !scan.Exists {
		t.Fatalf("simulated root %s does not exist", root)
	}
	t.Logf("scan of %s took %v, found %d logs / %d archives",
		root, elapsed, len(scan.LogFiles), len(scan.ArchiveFiles))

	// --- 正确性：真日志一个都不能丢 ---
	wantLogs := []string{
		filepath.Join(root, "deepseek.harness", "harness.log"),
		filepath.Join(root, "deepseek.harness", "info.log"),
		filepath.Join(root, "deepseek.harness", "dsh-data", "profiles", "web",
			".plugin-manager", "logs", "operation-AAA", "pnpm.log"),
		filepath.Join(root, "fndepot", "fndepot.log"),
	}
	got := make(map[string]bool, len(scan.LogFiles))
	for _, f := range scan.LogFiles {
		got[f] = true
	}
	for _, want := range wantLogs {
		if !got[want] {
			t.Errorf("MISSING real log: %s", want)
		}
	}

	// --- 正确性：包缓存里的噪音不能被当成日志 ---
	for _, f := range scan.LogFiles {
		for _, bad := range []string{"node_modules", "pnpm-home", "npm-cache"} {
			if containsSeg(f, bad) {
				t.Errorf("noise leaked into log list: %s", f)
			}
		}
	}

	if elapsed > 250*time.Millisecond {
		t.Errorf("scan too slow: %v (expected well under 250ms)", elapsed)
	}
}

func containsSeg(path, seg string) bool {
	for _, p := range strings.Split(path, string(filepath.Separator)) {
		if p == seg {
			return true
		}
	}
	return false
}
