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
		"app.log":                    true,
		"app.log.1":                  true,
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
