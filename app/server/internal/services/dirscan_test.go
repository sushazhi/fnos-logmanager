package services

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// scanTestDir 返回一个可用于扫描测试的临时目录。
//
// ScanDirResult 会先经过 utils.SafePath 归一化，而 SafePath 只接受 POSIX 绝对路径
// （Windows 的 D:\... 会被判为非法并返回空串），因此在 Windows 上直接跳过。
func scanTestDir(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("SafePath 仅接受 POSIX 绝对路径，Windows 下跳过目录扫描测试")
	}
	return t.TempDir()
}

func writeScanTestFile(t *testing.T, root, rel string, size int) {
	t.Helper()
	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, make([]byte, size), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestScanDirResultCountsLogsAndArchives(t *testing.T) {
	root := scanTestDir(t)

	writeScanTestFile(t, root, "app.log", 10)
	writeScanTestFile(t, root, "app.log.1", 20)
	writeScanTestFile(t, root, "nested/deep.log", 30)
	writeScanTestFile(t, root, "archive/old.gz", 40)
	// 非日志目录必须被跳过，否则会被当成日志计入统计
	writeScanTestFile(t, root, "node_modules/pkg/ignored.log", 50)

	InvalidateDirScanCache()
	scan := ScanDirResult(root)

	if !scan.Exists {
		t.Fatal("expected directory to exist")
	}
	if len(scan.LogFiles) != 3 {
		t.Fatalf("log files = %d (%v), want 3", len(scan.LogFiles), scan.LogFiles)
	}
	if len(scan.ArchiveFiles) != 1 {
		t.Fatalf("archive files = %d (%v), want 1", len(scan.ArchiveFiles), scan.ArchiveFiles)
	}
	if scan.LogSize != 60 {
		t.Errorf("log size = %d, want 60", scan.LogSize)
	}
	if scan.ArchiveSize != 40 {
		t.Errorf("archive size = %d, want 40", scan.ArchiveSize)
	}
	if scan.LargeLogFiles != 0 {
		t.Errorf("large log files = %d, want 0", scan.LargeLogFiles)
	}
}

func TestScanDirResultCacheInvalidation(t *testing.T) {
	root := scanTestDir(t)
	writeScanTestFile(t, root, "a.log", 1)

	InvalidateDirScanCache()
	if got := len(ScanDirResult(root).LogFiles); got != 1 {
		t.Fatalf("log files = %d, want 1", got)
	}

	// TTL 内复用缓存结果（首页并发请求只应遍历一次目录）
	writeScanTestFile(t, root, "b.log", 1)
	if got := len(ScanDirResult(root).LogFiles); got != 1 {
		t.Fatalf("cached log files = %d, want 1 (cache should be reused)", got)
	}

	// 写操作后失效缓存，下一次读取必须立即反映变更
	InvalidateDirScanCache()
	if got := len(ScanDirResult(root).LogFiles); got != 2 {
		t.Fatalf("log files after invalidate = %d, want 2", got)
	}
}

func TestScanDirResultMissingDir(t *testing.T) {
	InvalidateDirScanCache()
	scan := ScanDirResult(filepath.Join(t.TempDir(), "does-not-exist"))
	if scan.Exists {
		t.Fatal("expected Exists=false for a missing directory")
	}
	if len(scan.LogFiles) != 0 || len(scan.ArchiveFiles) != 0 {
		t.Fatalf("expected empty scan result, got %d logs / %d archives",
			len(scan.LogFiles), len(scan.ArchiveFiles))
	}
}

// TestIgnoredLogDirsCoversObservedPackageCaches 锁定 @appdata 性能修复的跳过名单。
//
// PERF(@appdata, 2026-09): 真机实测 /vol1/@appdata 有 77,741 个条目但只有 60 个日志，
// 其中 deepseek.harness 一个应用贡献 76,160 个（node_modules / pnpm store / npm cache）。
// 老的名单只认 node_modules 与 .pnpm-store，对实际目录名（pnpm-home、npm-cache、
// dsh-runtime）完全不匹配，导致整棵树被完整递归。
func TestIgnoredLogDirsCoversObservedPackageCaches(t *testing.T) {
	mustIgnore := []string{
		// 真机实测到的实际目录名
		"node_modules", "pnpm-home", "pnpm-env", "npm-cache", "dsh-runtime",
		"_cacache", "pnpm-store", ".pnpm-store", ".npm",
		// 语言虚拟环境
		"venv", ".venv", "site-packages", "__pycache__",
		// 构建 / VCS
		".git", "dist", ".cache",
	}
	for _, name := range mustIgnore {
		if !isIgnoredLogDir(name) {
			t.Errorf("isIgnoredLogDir(%q) = false, want true (package cache must be skipped)", name)
		}
	}
}

// TestIgnoredLogDirsKeepsRealLogContainers 保证跳过策略不会误伤真实日志。
//
// deepseek.harness 根目录下的 info.log / harness.log 是有价值的日志；
// dsh-data/profiles 是容器目录，深层存放 .plugin-manager/logs/*/pnpm.log，
// 因此它们都不能被整体剪掉（真正的大头是其下的 node_modules，由该名单单独剪）。
func TestIgnoredLogDirsKeepsRealLogContainers(t *testing.T) {
	mustKeep := []string{
		"dsh-data", "profiles", "logs", "log", "data", "build", "target",
	}
	for _, name := range mustKeep {
		if isIgnoredLogDir(name) {
			t.Errorf("isIgnoredLogDir(%q) = true, want false (may contain real logs)", name)
		}
	}
}

// TestDirScanResultMetaForAvoidsRestat 验证遍历阶段缓存的文件元信息可被直接取用。
func TestDirScanResultMetaForAvoidsRestat(t *testing.T) {
	scan := &dirScanResult{
		fileMeta: map[string]dirFileMeta{
			"/tmp/app.log": {Size: 123, ModTime: time.Unix(1700000000, 0)},
		},
	}
	meta, ok := scan.metaFor("/tmp/app.log")
	if !ok {
		t.Fatal("expected cached meta to be found")
	}
	if meta.Size != 123 {
		t.Errorf("size = %d, want 123", meta.Size)
	}

	// 未缓存且文件不存在时必须返回 false，让调用方跳过而不是塞入零值条目。
	if _, ok := scan.metaFor("/tmp/definitely-missing-xyz.log"); ok {
		t.Error("expected ok=false for a missing file")
	}
}
