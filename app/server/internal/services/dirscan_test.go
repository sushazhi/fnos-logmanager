package services

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
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
