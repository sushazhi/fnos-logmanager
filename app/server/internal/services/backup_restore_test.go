package services

import (
	"archive/tar"
	"compress/gzip"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/sushazhi/fnos-logmanager/internal/config"
	"github.com/sushazhi/fnos-logmanager/internal/types"
)

func TestMapTarMember(t *testing.T) {
	cases := []struct {
		name   string
		member string
		want   string
		ok     bool
	}{
		{"normal", "backup-2026-01-02T03-04-05.000000000/vol1/@appdata/app/app.log", "/vol1/@appdata/app/app.log", true},
		{"dot-slash prefix", "./backup-2026-01-02/vol1/@appdata/app/app.log", "/vol1/@appdata/app/app.log", true},
		{"traversal", "backup-2026/../../etc/passwd", "", false},
		{"dot segment", "backup-2026/vol1/./x.log", "", false},
		{"no backup root", "etc/passwd", "", false},
		{"absolute", "/etc/passwd", "", false},
		{"backslash", `backup-2026\vol1\app.log`, "", false},
		{"empty segment", "backup-2026//x.log", "", false},
		{"member in backup root", "backup-2026", "", false},
		{"traversal in backup root", "backup-../../etc/x", "", false},
		{"backup root only", "backup-2026/", "", false},
	}
	for _, tc := range cases {
		got, ok := mapTarMember(tc.member)
		if ok != tc.ok || (ok && got != tc.want) {
			t.Errorf("mapTarMember(%q) = (%q, %v), want (%q, %v)", tc.member, got, ok, tc.want, tc.ok)
		}
	}
}

// buildTestArchive writes a tar.gz with the given members into backupDir and
// returns the archive path.
type testMember struct {
	name string
	body string
	link string // non-empty → symlink member
}

func buildTestArchive(t *testing.T, backupDir string, members []testMember) string {
	t.Helper()
	archivePath := filepath.Join(backupDir, "backup-test.tar.gz")
	f, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	for _, m := range members {
		var hdr *tar.Header
		if m.link != "" {
			hdr, err = tar.FileInfoHeader(&fakeLinkFileInfo{name: filepath.Base(m.name)}, m.link)
		} else {
			hdr, err = tar.FileInfoHeader(&fakeRegFileInfo{name: filepath.Base(m.name), size: int64(len(m.body))}, "")
		}
		if err != nil {
			t.Fatal(err)
		}
		hdr.Name = m.name
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if m.link == "" {
			if _, err := tw.Write([]byte(m.body)); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return archivePath
}

type fakeRegFileInfo struct {
	name string
	size int64
}

func (f *fakeRegFileInfo) Name() string       { return f.name }
func (f *fakeRegFileInfo) Size() int64        { return f.size }
func (f *fakeRegFileInfo) Mode() os.FileMode  { return 0644 }
func (f *fakeRegFileInfo) ModTime() time.Time { return time.Time{} }
func (f *fakeRegFileInfo) IsDir() bool        { return false }
func (f *fakeRegFileInfo) Sys() interface{}   { return nil }
func (f *fakeRegFileInfo) Type() os.FileMode  { return 0 }
func (f *fakeRegFileInfo) Info() (os.FileInfo, error) { return nil, nil }

type fakeLinkFileInfo struct {
	name string
}

func (f *fakeLinkFileInfo) Name() string      { return f.name }
func (f *fakeLinkFileInfo) Size() int64       { return 0 }
func (f *fakeLinkFileInfo) Mode() os.FileMode { return os.ModeSymlink | 0777 }
func (f *fakeLinkFileInfo) ModTime() time.Time { return time.Time{} }
func (f *fakeLinkFileInfo) IsDir() bool       { return false }
func (f *fakeLinkFileInfo) Sys() interface{}  { return nil }
func (f *fakeLinkFileInfo) Type() os.FileMode { return os.ModeSymlink }
func (f *fakeLinkFileInfo) Info() (os.FileInfo, error) { return nil, nil }

// TestPreviewAndRestoreBackup exercises the archive pipeline end-to-end.
// It runs on Linux only: SafePath rejects unix-style paths elsewhere.
func TestPreviewAndRestoreBackup(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("SafePath is unix-path oriented; run this on linux")
	}

	tmp := t.TempDir()
	logDir := filepath.Join(tmp, "logs")
	backupBase := filepath.Join(tmp, "backup")
	for _, d := range []string{logDir, backupBase} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatal(err)
		}
	}

	cfg := config.Get()
	oldLogDirs, oldBase := cfg.LogDirs, cfg.Backup.BaseDir
	cfg.LogDirs = []string{logDir}
	cfg.Backup.BaseDir = backupBase
	defer func() { cfg.LogDirs, cfg.Backup.BaseDir = oldLogDirs, oldBase }()

	relLogDir := strings.TrimPrefix(logDir, "/")
	existing := filepath.Join(logDir, "exists.log")
	if err := os.WriteFile(existing, []byte("current"), 0644); err != nil {
		t.Fatal(err)
	}

	archivePath := buildTestArchive(t, backupBase, []testMember{
		{name: "backup-t/" + relLogDir + "/app.log", body: "hello log"},
		{name: "backup-t/" + relLogDir + "/nested/dir/inner.log", body: "nested"},
		{name: "backup-t/" + relLogDir + "/exists.log", body: "from backup"},
		{name: "backup-t/" + relLogDir + "/link.log", link: "/etc/passwd"},
		{name: "backup-t/../evil.txt", body: "evil"},
		{name: "backup-t/etc/outside.log", body: "outside"},
		{name: "backup-t/" + strings.TrimPrefix(backupBase, "/") + "/self.tar.gz", body: "self"},
	})

	t.Run("preview", func(t *testing.T) {
		preview, err := PreviewBackup(archivePath, 200)
		if err != nil {
			t.Fatal(err)
		}
		// Regular members counted: app.log, inner.log, exists.log, outside.log.
		// evil.txt / self.tar.gz are rejected before counting; symlink member is
		// skipped by type and never counted.
		if preview.TotalFiles != 4 {
			t.Fatalf("expected 4 counted members, got %d", preview.TotalFiles)
		}
		if preview.DeniedFiles != 3 {
			t.Fatalf("expected 3 denied members (traversal/outside/backup-dir), got %d", preview.DeniedFiles)
		}
		var existsEntry *types.BackupPreviewEntry
		for i := range preview.Entries {
			if strings.HasSuffix(preview.Entries[i].TargetPath, "exists.log") {
				existsEntry = &preview.Entries[i]
			}
		}
		if existsEntry == nil || !existsEntry.Exists {
			t.Fatal("existing target should be flagged in preview")
		}
	})

	t.Run("restore skip-existing", func(t *testing.T) {
		result, err := RestoreBackup(archivePath, types.RestoreOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if result.Restored != 2 {
			t.Fatalf("expected 2 restored, got %d", result.Restored)
		}
		if result.Skipped != 4 { // exists.log + traversal + outside + backup-dir
			t.Fatalf("expected 4 skipped, got %d", result.Skipped)
		}
		data, err := os.ReadFile(filepath.Join(logDir, "app.log"))
		if err != nil || string(data) != "hello log" {
			t.Fatalf("restored content mismatch: %q err=%v", data, err)
		}
		if _, err := os.Stat(filepath.Join(logDir, "nested/dir/inner.log")); err != nil {
			t.Fatalf("nested restore missing: %v", err)
		}
		data, _ = os.ReadFile(existing)
		if string(data) != "current" {
			t.Fatalf("existing file must not be clobbered, got %q", data)
		}
	})

	t.Run("restore overwrite", func(t *testing.T) {
		result, err := RestoreBackup(archivePath, types.RestoreOptions{Overwrite: true})
		if err != nil {
			t.Fatal(err)
		}
		if result.Restored != 3 {
			t.Fatalf("expected 3 restored with overwrite, got %d", result.Restored)
		}
		data, _ := os.ReadFile(existing)
		if string(data) != "from backup" {
			t.Fatalf("overwrite should restore backup content, got %q", data)
		}
	})

	t.Run("restore rejects outside backup dir", func(t *testing.T) {
		if _, err := RestoreBackup(filepath.Join(tmp, "other.tar.gz"), types.RestoreOptions{}); err == nil {
			t.Fatal("expected error for archive outside backup dir")
		}
	})
}
