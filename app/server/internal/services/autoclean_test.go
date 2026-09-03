package services

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"
)

// buildCleanTree creates a directory tree aged 30 days (except recent.log)
// that mixes real log/archive files with user data a clean rule must never
// touch. Returns the root and a map of relative path -> must the default
// delete matcher select it.
func buildCleanTree(t *testing.T) (string, map[string]bool) {
	t.Helper()

	root := t.TempDir()
	files := map[string]bool{
		"app.log":   true,
		"app.log.1": true,
		// dash-rotated names are NOT matched by isLogFile's existing contract
		// (same as the one-time CleanLogFiles path) — documents that behavior
		"nginx.log-20240101":         false,
		"archives/old.gz":            true,
		"archives/old.tar.gz":        true,
		"BT_backup/seed.torrent":     false,
		"BT_backup/seed.fastresume":  false,
		"media/movie.mkv":            false,
		"venv/pkg/service-2.json.gz": false,
		"recent.log":                 false,
	}

	old := time.Now().AddDate(0, 0, -30)
	for rel := range files {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("data"), 0o644); err != nil {
			t.Fatal(err)
		}
		// recent.log stays fresh; everything else is aged past any cutoff
		mt := old
		if rel == "recent.log" {
			mt = time.Now()
		}
		if err := os.Chtimes(path, mt, mt); err != nil {
			t.Fatal(err)
		}
	}
	return root, files
}

func TestMatchCleanFilesDefaultScopeDelete(t *testing.T) {
	root, want := buildCleanTree(t)

	matchName, err := cleanNameMatcher("", "delete")
	if err != nil {
		t.Fatal(err)
	}
	matches := matchCleanFiles([]string{root}, matchName, 0, 0, 1)

	got := make([]string, 0, len(matches))
	for _, m := range matches {
		rel, err := filepath.Rel(root, m.Path)
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, filepath.ToSlash(rel))
	}
	sort.Strings(got)

	wantPaths := make([]string, 0, len(want))
	for rel, selected := range want {
		if selected {
			wantPaths = append(wantPaths, rel)
		}
	}
	sort.Strings(wantPaths)

	if strings.Join(got, ",") != strings.Join(wantPaths, ",") {
		t.Fatalf("default delete scope mismatch:\n got: %v\nwant: %v", got, wantPaths)
	}
}

func TestMatchCleanFilesDefaultScopeTruncateExcludesArchives(t *testing.T) {
	root, _ := buildCleanTree(t)

	matchName, err := cleanNameMatcher("", "truncate")
	if err != nil {
		t.Fatal(err)
	}
	matches := matchCleanFiles([]string{root}, matchName, 0, 0, 1)

	for _, m := range matches {
		if isArchiveFile(filepath.Base(m.Path)) {
			t.Errorf("truncate default scope must not select archives, got %s", m.Path)
		}
	}
	// The live log is still selected
	found := false
	for _, m := range matches {
		if filepath.Base(m.Path) == "app.log" {
			found = true
		}
	}
	if !found {
		t.Error("truncate default scope should select app.log")
	}
}

func TestCleanNameMatcherExplicitPattern(t *testing.T) {
	root, _ := buildCleanTree(t)

	matchName, err := cleanNameMatcher(`\.torrent$`, "delete")
	if err != nil {
		t.Fatal(err)
	}
	matches := matchCleanFiles([]string{root}, matchName, 0, 0, 0)
	if len(matches) != 1 || filepath.Base(matches[0].Path) != "seed.torrent" {
		t.Fatalf("explicit pattern should select exactly seed.torrent, got %v", matches)
	}

	if _, err := cleanNameMatcher("([", "delete"); err == nil {
		t.Error("invalid pattern must return an error")
	}
}

func TestExecuteCleanMatchesDeleteKeepsUserData(t *testing.T) {
	root, _ := buildCleanTree(t)

	matchName, err := cleanNameMatcher("", "delete")
	if err != nil {
		t.Fatal(err)
	}
	matches := matchCleanFiles([]string{root}, matchName, 0, 0, 1)
	result := executeCleanMatches(matches, "delete", 0)

	if result.Cleaned != len(matches) || len(result.Errors) != 0 {
		t.Fatalf("unexpected result: cleaned=%d errors=%v", result.Cleaned, result.Errors)
	}

	for rel, shouldMatch := range map[string]bool{
		"app.log":                   true,
		"archives/old.gz":           true,
		"BT_backup/seed.torrent":    false,
		"BT_backup/seed.fastresume": false,
		"media/movie.mkv":           false,
	} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); shouldMatch == (err == nil) {
			t.Errorf("%s: exists=%v, want exists=%v", rel, err == nil, !shouldMatch)
		}
	}
}

func TestExecuteCleanMatchesTruncatePreservesArchives(t *testing.T) {
	root, _ := buildCleanTree(t)

	matchName, err := cleanNameMatcher("", "truncate")
	if err != nil {
		t.Fatal(err)
	}
	matches := matchCleanFiles([]string{root}, matchName, 0, 0, 1)
	result := executeCleanMatches(matches, "truncate", 0)

	if result.Cleaned != len(matches) || len(result.Errors) != 0 {
		t.Fatalf("unexpected result: cleaned=%d errors=%v", result.Cleaned, result.Errors)
	}

	logPath := filepath.Join(root, "app.log")
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != 0 {
		t.Errorf("app.log should be truncated to 0 bytes, got %d", len(data))
	}

	gzPath := filepath.Join(root, "archives", "old.gz")
	data, err = os.ReadFile(gzPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "data" {
		t.Errorf("old.gz must stay intact, got %q", string(data))
	}
}

func TestExecuteCleanMatchesMaxFilesCap(t *testing.T) {
	root, _ := buildCleanTree(t)

	matchName, err := cleanNameMatcher("", "delete")
	if err != nil {
		t.Fatal(err)
	}
	matches := matchCleanFiles([]string{root}, matchName, 0, 0, 1)
	if len(matches) < 2 {
		t.Fatalf("need at least 2 matches, got %d", len(matches))
	}

	result := executeCleanMatches(matches, "delete", 1)
	if result.Cleaned != 1 {
		t.Errorf("cap of 1 should stop after one cleaned file, cleaned=%d", result.Cleaned)
	}
}

func TestExecuteCleanRuleDeleteUninstalledDispatch(t *testing.T) {
	// CleanUninstalledLogs scans config.Get().LogDirs — on a REAL fnOS host
	// (linux + /vol1 present) the test would perform an actual cleanup, so
	// skip there. Windows dev boxes may mirror /vol1 as <cwd-drive>:\vol1 for
	// mocking; running there is safe because backslash paths never yield an
	// appName and nothing gets deleted.
	if runtime.GOOS == "linux" {
		if _, err := os.Stat("/vol1/@appdata"); err == nil {
			t.Skip("real fnOS app dirs present; skip to avoid a real uninstalled-app cleanup")
		}
	}

	// Seed the installed-apps cache so CleanUninstalledLogs never shells out to
	// appcenter-cli (absent off-fnOS); restore afterwards.
	cachedInstalledAppsMu.Lock()
	prevApps, prevAt := cachedInstalledApps, installedAppsCacheAt
	cachedInstalledApps = []string{"myapp"}
	installedAppsCacheAt = time.Now()
	cachedInstalledAppsMu.Unlock()
	defer func() {
		cachedInstalledAppsMu.Lock()
		cachedInstalledApps, installedAppsCacheAt = prevApps, prevAt
		cachedInstalledAppsMu.Unlock()
	}()

	result, err := executeCleanRule(&CleanRule{Action: "deleteUninstalled"})
	if err != nil {
		t.Fatalf("scheduled deleteUninstalled rule must dispatch to CleanUninstalledLogs instead of failing, got: %v", err)
	}
	_ = result
}

// seedInstalledApps installs a fake installed-app list into the process-global
// cache so dir-judging phases run deterministically without appcenter-cli.
func seedInstalledApps(t *testing.T, apps []string) {
	t.Helper()
	cachedInstalledAppsMu.Lock()
	prevApps, prevAt := cachedInstalledApps, installedAppsCacheAt
	cachedInstalledApps = apps
	installedAppsCacheAt = time.Now()
	cachedInstalledAppsMu.Unlock()
	t.Cleanup(func() {
		cachedInstalledAppsMu.Lock()
		cachedInstalledApps, installedAppsCacheAt = prevApps, prevAt
		cachedInstalledAppsMu.Unlock()
	})
}

func TestCleanEmptyItemsRemovesOnlyZeroByteLogFiles(t *testing.T) {
	seedInstalledApps(t, []string{"myapp"})

	root := t.TempDir()
	zeroFiles := []string{"empty.log", "archives/old.gz"}
	kept := map[string]string{
		"app.log":                "data",
		"archives/full.gz":       "data",
		"BT_backup/seed.torrent": "", // 0-byte but outside the default log matcher
	}
	for rel, content := range kept {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, rel := range zeroFiles {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte{}, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	matchName, err := cleanNameMatcher("", "cleanEmpty")
	if err != nil {
		t.Fatal(err)
	}
	result := cleanEmptyItems([]string{root}, matchName, 0)

	if result.Cleaned != len(zeroFiles) {
		t.Fatalf("cleaned=%d, want %d (errors: %v)", result.Cleaned, len(zeroFiles), result.Errors)
	}
	for _, rel := range zeroFiles {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); !os.IsNotExist(err) {
			t.Errorf("%s should have been removed", rel)
		}
	}
	for rel, content := range kept {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			t.Errorf("%s must be kept: %v", rel, err)
			continue
		}
		if string(data) != content {
			t.Errorf("%s content changed: got %q", rel, data)
		}
	}
}

func TestCleanEmptyItemsExplicitPatternWidensScope(t *testing.T) {
	seedInstalledApps(t, []string{"myapp"})

	root := t.TempDir()
	for _, rel := range []string{"placeholder.lock", "cache.torrent"} {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.WriteFile(path, []byte{}, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	matchName, err := cleanNameMatcher(`\.torrent$`, "cleanEmpty")
	if err != nil {
		t.Fatal(err)
	}
	result := cleanEmptyItems([]string{root}, matchName, 0)

	if result.Cleaned != 1 {
		t.Fatalf("cleaned=%d, want 1 (errors: %v)", result.Cleaned, result.Errors)
	}
	if _, err := os.Stat(filepath.Join(root, "cache.torrent")); !os.IsNotExist(err) {
		t.Error("cache.torrent should have been removed")
	}
	if _, err := os.Stat(filepath.Join(root, "placeholder.lock")); err != nil {
		t.Error("placeholder.lock must be kept: pattern did not select it")
	}
}

func TestCleanEmptyItemsSkipsInstalledAppsFiles(t *testing.T) {
	// App-name extraction splits on "/", so only forward-slash (linux) paths
	// yield app names; on Windows this test would silently do nothing.
	if runtime.GOOS != "linux" {
		t.Skip("requires forward-slash path parsing; run on linux/CI")
	}
	seedInstalledApps(t, []string{"myapp"})

	root := t.TempDir()
	appdata := filepath.Join(root, "@appdata")
	for _, rel := range []string{"@appdata/myapp/empty.log", "@appdata/ghostapp/empty.log", "plain.log"} {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte{}, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(appdata, "ghostapp", "emptydir"), 0o755); err != nil {
		t.Fatal(err)
	}

	matchName, err := cleanNameMatcher("", "cleanEmpty")
	if err != nil {
		t.Fatal(err)
	}
	result := cleanEmptyItems([]string{root}, matchName, 0)

	// ghostapp/empty.log + plain.log (files) + ghostapp/emptydir + ghostapp
	// (dirs, cascaded once their contents emptied)
	if result.Cleaned != 4 {
		t.Fatalf("cleaned=%d, want 4 (errors: %v)", result.Cleaned, result.Errors)
	}
	if _, err := os.Stat(filepath.Join(appdata, "myapp", "empty.log")); err != nil {
		t.Error("installed app's 0-byte file must never be removed")
	}
	if _, err := os.Stat(filepath.Join(appdata, "ghostapp", "empty.log")); !os.IsNotExist(err) {
		t.Error("uninstalled app's 0-byte file should have been removed")
	}
	if _, err := os.Stat(filepath.Join(appdata, "ghostapp")); !os.IsNotExist(err) {
		t.Error("uninstalled app's emptied dir tree should have been removed")
	}
	if _, err := os.Stat(filepath.Join(root, "plain.log")); !os.IsNotExist(err) {
		t.Error("non-app-dir 0-byte file should have been removed")
	}
}

func TestRemoveEmptyUninstalledDirs(t *testing.T) {
	// App-name extraction splits on "/", so only forward-slash (linux) paths
	// yield app names; on Windows this test would silently do nothing.
	if runtime.GOOS != "linux" {
		t.Skip("requires forward-slash path parsing; run on linux/CI")
	}

	root := t.TempDir()
	appdata := filepath.Join(root, "@appdata")
	for _, rel := range []string{
		"ghostapp/logs",
		"ghostapp/data",
		"ghostapp/empty/nested",
		"myapp/logs",
		"logmanager/keep",
	} {
		if err := os.MkdirAll(filepath.Join(appdata, filepath.FromSlash(rel)), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(appdata, "ghostapp", "data", "db.sqlite"), []byte("db"), 0o644); err != nil {
		t.Fatal(err)
	}

	installed := map[string]bool{"myapp": true}
	removed, nonEmpty := removeEmptyUninstalledDirs([]string{root}, installed)

	// ghostapp/logs, ghostapp/data/logs, ghostapp/empty/nested, ghostapp/empty
	if removed != 4 {
		t.Fatalf("removed=%d, want 4", removed)
	}
	for _, rel := range []string{"ghostapp/logs", "ghostapp/data/logs", "ghostapp/empty"} {
		if _, err := os.Stat(filepath.Join(appdata, filepath.FromSlash(rel))); !os.IsNotExist(err) {
			t.Errorf("%s should have been removed", rel)
		}
	}
	if _, err := os.Stat(filepath.Join(appdata, "ghostapp", "data", "db.sqlite")); err != nil {
		t.Error("ghostapp/data must be kept: it is non-empty and only reported")
	}
	if _, err := os.Stat(filepath.Join(appdata, "myapp", "logs")); err != nil {
		t.Error("dirs of an installed app must never be removed")
	}
	if _, err := os.Stat(filepath.Join(appdata, "logmanager", "keep")); err != nil {
		t.Error("reserved app dirs must never be removed")
	}
	if _, err := os.Stat(root); err != nil {
		t.Error("walk root must never be removed")
	}

	want := filepath.Join(appdata, "ghostapp")
	if len(nonEmpty) != 1 || nonEmpty["ghostapp"] != want {
		t.Fatalf("nonEmpty=%v, want map[ghostapp:%s]", nonEmpty, want)
	}
}
