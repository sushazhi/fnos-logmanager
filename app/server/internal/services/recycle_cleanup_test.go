package services

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestLowerSetCaseInsensitive(t *testing.T) {
	set := lowerSet([]string{"Qbittorrent", " emby ", "", "Redis"})
	// set holds lowercase keys; callers lowercase lookups (verified here too)
	for _, name := range []string{"QBITTORRENT", "emby", "EMBY", "redis", "Redis"} {
		if !set[strings.ToLower(name)] {
			t.Errorf("lowerSet should contain lowercased %q", name)
		}
	}
	if set[""] {
		t.Error("lowerSet must not contain empty entries")
	}
}

func TestAllowedSet(t *testing.T) {
	if allowedSet(nil) != nil {
		t.Error("empty selection must return nil filter")
	}
	m := allowedSet([]string{"/vol1/@appdata/redis"})
	if !m["/vol1/@appdata/redis"] {
		t.Error("selection must contain the given path")
	}
	if m["/vol1/@appdata/other"] {
		t.Error("selection must not contain other paths")
	}
}

func TestLinkTargetReExtractsAppName(t *testing.T) {
	cases := []struct {
		target string
		app    string
		ok     bool
	}{
		{"/var/apps/redis/bin/redis-server", "redis", true},
		{"/vol1/@appdata/xunlei/data", "xunlei", true},
		{"/vol2/@appconf/Emby/config", "Emby", true},
		{"/usr/bin/foo", "", false},
		{"/var/log/apps/x", "", false},
	}
	for _, c := range cases {
		m := linkTargetRe.FindStringSubmatch(c.target)
		if c.ok {
			if m == nil || m[1] != c.app {
				t.Errorf("target %q: want app %q, got %v", c.target, c.app, m)
			}
		} else if m != nil {
			t.Errorf("target %q: expected no match, got %v", c.target, m)
		}
	}
}

func TestRiskOfRoot(t *testing.T) {
	cases := map[string]string{
		"@apptemp":  "low",
		"@appconf":  "medium",
		"@appmeta":  "medium",
		"@appdata":  "high",
		"@apphome":  "high",
		"@appshare": "high",
	}
	for root, want := range cases {
		if got := riskOfRoot(root); got != want {
			t.Errorf("riskOfRoot(%q) = %q, want %q", root, got, want)
		}
	}
}

func TestOrphanUserNameRe(t *testing.T) {
	valid := []string{"docker-redis", "docker-my_app.v2"}
	for _, name := range valid {
		if !orphanUserNameRe.MatchString(name) {
			t.Errorf("%q must be a valid orphan user name", name)
		}
	}
	invalid := []string{"root", "docker-", "docker-bad;rm", "xdocker-redis", "docker-a b"}
	for _, name := range invalid {
		if orphanUserNameRe.MatchString(name) {
			t.Errorf("%q must be rejected as orphan user name", name)
		}
	}
}

func TestRecycleRetentionDurationMin(t *testing.T) {
	d := recycleRetentionDuration()
	if d.Hours() < 1 {
		t.Errorf("retention must be at least 1 hour, got %v", d)
	}
}

// requireLinux skips the test on non-Linux platforms. The cleanup feature
// targets fnOS (Linux) and its POSIX "/volN/..." paths cannot round-trip
// through utils.SafePath / filepath on Windows, where the safety rules are
// therefore exercised by the platform-independent tests only.
func requireLinux(t *testing.T) {
	t.Helper()
	if runtime.GOOS != "linux" {
		t.Skipf("filesystem layout test requires Linux (running on %s)", runtime.GOOS)
	}
}

// newScanRootFixture creates a fake app-data root (e.g. /glm_x/@appdata) at the
// filesystem root, matching fnOS's real "/volN/@app*" layout so the scan's
// path validation (utils.SafePath) accepts it.
func newScanRootFixture(t *testing.T, rootType string) (root, rootAbs string) {
	t.Helper()
	root = "/glm_scan_test_" + strconv.FormatInt(time.Now().UnixNano(), 36) + "/" + rootType
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Skipf("cannot create fixture dir %s: %v", root, err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	return root, root
}

func makeAppDir(t *testing.T, base, name string) string {
	t.Helper()
	dir := filepath.Join(base, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "data.bin"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func findCandidate(cands []LeftoverCandidate, app string) *LeftoverCandidate {
	for i := range cands {
		if cands[i].App == app {
			return &cands[i]
		}
	}
	return nil
}

// Gap #1: appcenter-cli names and @app* dir names may differ in case
// (qBittorrent vs qbittorrent); installed apps must never be flagged.
func TestScanLeftoverDirsCaseInsensitiveInstalled(t *testing.T) {
	requireLinux(t)
	root, rootAbs := newScanRootFixture(t, "@appdata")
	makeAppDir(t, rootAbs, "qBittorrent")
	makeAppDir(t, rootAbs, "redis") // not installed -> must be flagged

	installed := lowerSet([]string{"qbittorrent"})
	cands, errs := scanLeftoverDirsOnRoots([]string{root}, installed, nil, nil)
	if len(errs) != 0 {
		t.Fatalf("unexpected scan errors: %v", errs)
	}
	if findCandidate(cands, "qBittorrent") != nil {
		t.Error("installed app dir (case-insensitive match) must not be flagged as leftover")
	}
	c := findCandidate(cands, "redis")
	if c == nil {
		t.Fatal("uninstalled app dir was not flagged")
	}
	if c.Risk != "high" || c.RootType != "data" || c.Size <= 0 {
		t.Errorf("unexpected candidate: %+v", c)
	}
	if !strings.HasSuffix(c.Path, filepath.Join("@appdata", "redis")) {
		t.Errorf("candidate path %q does not point at the leftover dir", c.Path)
	}
}

func TestScanLeftoverDirsSkipsReservedHiddenAndEmpty(t *testing.T) {
	requireLinux(t)
	root, rootAbs := newScanRootFixture(t, "@appdata")
	for _, name := range []string{"home", "@System", ".hidden", "emptydir"} {
		if err := os.MkdirAll(filepath.Join(rootAbs, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	makeAppDir(t, rootAbs, "realleftover")

	cands, errs := scanLeftoverDirsOnRoots([]string{root}, map[string]bool{}, nil, nil)
	if len(errs) != 0 {
		t.Fatalf("unexpected scan errors: %v", errs)
	}
	for _, reserved := range []string{"home", "@System", ".hidden", "emptydir"} {
		if findCandidate(cands, reserved) != nil {
			t.Errorf("reserved/hidden/empty dir %q must not be flagged", reserved)
		}
	}
	if findCandidate(cands, "realleftover") == nil {
		t.Error("non-empty uninstalled dir must be flagged")
	}
}

// Gap #2: @appshare dir names need not match app names; any share referenced
// by an installed app (via /var/apps/{app}/shares symlinks) must be protected.
func TestScanLeftoverDirsShareReferencedProtected(t *testing.T) {
	requireLinux(t)
	root, rootAbs := newScanRootFixture(t, "@appshare")
	shared := makeAppDir(t, rootAbs, "媒体库")
	makeAppDir(t, rootAbs, "orphanshare")

	usedShares := map[string]bool{shared: true}
	cands, errs := scanLeftoverDirsOnRoots([]string{root}, map[string]bool{}, usedShares, nil)
	if len(errs) != 0 {
		t.Fatalf("unexpected scan errors: %v", errs)
	}
	if findCandidate(cands, "媒体库") != nil {
		t.Error("share referenced by an installed app must not be flagged as leftover")
	}
	c := findCandidate(cands, "orphanshare")
	if c == nil {
		t.Fatal("unreferenced @appshare dir was not flagged")
	}
	if c.Risk != "high" {
		t.Errorf("@appshare leftover risk must be high, got %q", c.Risk)
	}
}

// Gap #2 (second protection): an @appshare dir owned by an installed app's
// runtime user must not be flagged even when nothing references it. Needs a
// real uid -> name resolution, so it only runs where that exists.
func TestScanLeftoverDirsShareOwnerProtected(t *testing.T) {
	requireLinux(t)
	root, rootAbs := newScanRootFixture(t, "@appshare")
	makeAppDir(t, rootAbs, "emby-media")

	owner := uidToName(strconv.Itoa(os.Getuid()))
	if owner == "" {
		t.Skip("cannot resolve current uid to a user name on this platform")
	}
	installedUsers := lowerSet([]string{owner, "docker-emby"})
	cands, errs := scanLeftoverDirsOnRoots([]string{root}, map[string]bool{}, nil, installedUsers)
	if len(errs) != 0 {
		t.Fatalf("unexpected scan errors: %v", errs)
	}
	if findCandidate(cands, "emby-media") != nil {
		t.Error("appshare dir owned by an installed app user must not be flagged")
	}
}

// Bug-13 regression: two same-name moves must both survive in the recycle bin
// via the -1 suffix, each with its own manifest/times entry keyed by the REAL
// on-disk relative path.
func TestMoveToTrashRoundtripAndSuffixes(t *testing.T) {
	requireLinux(t)
	trash := t.TempDir()
	srcDir := t.TempDir()

	src1 := makeAppDir(t, srcDir, "redis")
	if err := moveToTrash(src1, trash, "/vol1/@appdata/redis"); err != nil {
		t.Fatalf("first move failed: %v", err)
	}
	if _, err := os.Stat(src1); !os.IsNotExist(err) {
		t.Fatal("source dir still exists after moveToTrash")
	}
	dest1 := filepath.Join(trash, "@appdata", "redis")
	if _, err := os.Stat(filepath.Join(dest1, "data.bin")); err != nil {
		t.Fatalf("moved content missing: %v", err)
	}
	// Manifest keys are volume-root-relative paths built with the host's path
	// separators (filepath.Rel), so construct the expected key the same way.
	key1 := filepath.Join("@appdata", "redis")
	manifest := readTrashManifest(trash)
	if manifest[key1] != src1 {
		t.Errorf("manifest original = %q, want %q", manifest[key1], src1)
	}
	if readTrashTimes(trash)[key1] <= 0 {
		t.Error("move time must be recorded for the auto-purge")
	}

	src2 := makeAppDir(t, srcDir, "redis")
	if err := moveToTrash(src2, trash, "/vol1/@appdata/redis"); err != nil {
		t.Fatalf("second move failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(trash, "@appdata", "redis-1", "data.bin")); err != nil {
		t.Fatalf("suffixed recycle item missing: %v", err)
	}
	manifest = readTrashManifest(trash)
	if manifest[filepath.Join("@appdata", "redis-1")] != src2 {
		t.Errorf("suffixed manifest entry missing: %v", manifest)
	}
	if manifest[key1] != src1 {
		t.Error("first manifest entry must survive the second same-name move")
	}
}
