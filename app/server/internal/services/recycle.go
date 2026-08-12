package services

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/sushazhi/fnos-logmanager/internal/config"
	"github.com/sushazhi/fnos-logmanager/internal/types"
	"github.com/sushazhi/fnos-logmanager/internal/utils"
)

// recycleRetention is how long moved items stay in the recycle bin before
// being permanently removed. 默认 24 小时自动清空。
const recycleRetention = 24 * time.Hour

// recycleSubDir is the app-specific subfolder inside the user's trash folder.
// Using a dedicated subfolder keeps items moved by this app separate from
// anything the user put into the recycle bin themselves, so auto-purge can
// never touch unrelated files.
const recycleSubDir = "logmanager_cleanup"

// defaultRecycleUID is the fallback uid used when the current request does not
// carry an fnOS gateway uid. 用户级回收站路径形如 /vol1/1000/.@#local/trash。
const defaultRecycleUID = "1000"

// appDataDirsToRecycle are the app-data roots whose uninstalled-app leftover
// directories can be moved into the recycle bin. @appcenter (the app store
// itself) is intentionally excluded to avoid touching system bookkeeping.
var appDataDirsToRecycle = []string{
	"/vol1/@appdata",
	"/vol1/@appshare",
	"/vol1/@appconf",
	"/vol1/@apphome",
	"/vol1/@apptemp",
	"/vol1/@appmeta",
}

// reservedAppDirs are top-level directory names under @app* that must never be
// treated as uninstalled-app leftovers (system dirs, our own data, shared
// mountpoints, etc.).
var reservedAppDirs = map[string]bool{
	"logmanager": true,
	"home":       true,
	"System":     true,
	"@System":    true,
}

// volRe matches a storage volume root such as /vol1 or /vol2.
var volRe = regexp.MustCompile(`^/(vol\d+)(?:/|$)`)

// trashState holds the set of recycle-bin roots that have been used by this app.
// It is persisted so the background auto-purge task can operate without a
// request context and across restarts.
var (
	trashStateMu sync.Mutex
	trashRoots   = map[string]bool{}
	trashRootsFile string
)

// volRootOf returns the leading /volN component of a path, or "" if none.
func volRootOf(p string) string {
	m := volRe.FindStringSubmatch(p)
	if len(m) > 1 {
		return m[1]
	}
	return ""
}

// recycleDirForPath returns the app's dedicated recycle subfolder for a given
// source path, following fnOS's per-user trash convention:
//
//	/{volN}/{uid}/.@#local/trash/logmanager_cleanup
func recycleDirForPath(srcPath, uid string) string {
	vol := volRootOf(srcPath)
	if vol == "" {
		vol = "vol1"
	}
	if uid == "" {
		uid = defaultRecycleUID
	}
	return filepath.Join("/", vol, uid, ".@#local", "trash", recycleSubDir)
}

// getTrashRoots returns the currently known recycle-bin roots.
func getTrashRoots() []string {
	trashStateMu.Lock()
	defer trashStateMu.Unlock()
	roots := make([]string, 0, len(trashRoots))
	for r := range trashRoots {
		roots = append(roots, r)
	}
	return roots
}

// recordTrashRoot remembers a recycle root and persists it to disk.
func recordTrashRoot(root string) {
	if root == "" {
		return
	}
	trashStateMu.Lock()
	added := !trashRoots[root]
	trashRoots[root] = true
	file := trashRootsFile
	trashStateMu.Unlock()
	if added && file != "" {
		if err := saveTrashRoots(file); err != nil {
			slog.Warn("failed to persist recycle roots", "error", err)
		}
	}
}

// loadTrashRoots loads persisted recycle-bin roots.
func loadTrashRoots(file string) {
	trashStateMu.Lock()
	trashRootsFile = file
	trashStateMu.Unlock()

	data, err := os.ReadFile(file)
	if err != nil {
		if !os.IsNotExist(err) {
			slog.Warn("failed to read recycle roots", "error", err)
		}
		return
	}
	var list []string
	if err := json.Unmarshal(data, &list); err != nil {
		slog.Warn("failed to parse recycle roots", "error", err)
		return
	}
	trashStateMu.Lock()
	for _, r := range list {
		if r != "" {
			trashRoots[r] = true
		}
	}
	trashStateMu.Unlock()
}

func saveTrashRoots(file string) error {
	trashStateMu.Lock()
	list := make([]string, 0, len(trashRoots))
	for r := range trashRoots {
		list = append(list, r)
	}
	trashStateMu.Unlock()

	if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(file, data, 0644)
}

// moveToTrash moves src into the recycle dir while preserving its relative
// path under the volume root. For example:
//
//	/vol1/@appdata/redis  ->  trashDir/@appdata/redis
//
// Keeping the relative-path structure (no long prefix) makes items immediately
// readable in the file manager and lets restore compute the original absolute
// path from the volume root + relative path. Same-volume rename is preferred;
// a cross-volume copy+remove is used as a fallback. The original absolute path
// is still recorded in a manifest so restore is exact even after suffix-based
// de-duplication.
func moveToTrash(src, trashDir, rel string) error {
	dest := uniqueDest(filepath.Join(trashDir, rel))
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return fmt.Errorf("创建回收站目录失败: %w", err)
	}

	if err := os.Rename(src, dest); err == nil {
		recordTrashRoot(trashDir)
		recordTrashManifest(trashDir, rel, src)
		return nil
	}

	// Cross-device (different volume): copy then remove.
	if err := copyDirTree(src, dest); err != nil {
		return fmt.Errorf("移动到回收站失败: %w", err)
	}
	if err := os.RemoveAll(src); err != nil {
		return fmt.Errorf("移动后清理源目录失败: %w", err)
	}
	recordTrashRoot(trashDir)
	recordTrashManifest(trashDir, rel, src)
	return nil
}

// uniqueDest returns dest if it does not exist, otherwise appends a numeric
// suffix (-1, -2, ...) to avoid clobbering an existing recycle item.
func uniqueDest(dest string) string {
	if _, err := os.Lstat(dest); os.IsNotExist(err) {
		return dest
	}
	dir := filepath.Dir(dest)
	ext := filepath.Ext(dest)
	base := filepath.Base(dest)
	name := strings.TrimSuffix(base, ext)
	for i := 1; ; i++ {
		cand := filepath.Join(dir, fmt.Sprintf("%s-%d%s", name, i, ext))
		if _, err := os.Lstat(cand); os.IsNotExist(err) {
			return cand
		}
	}
}

// trashManifestFileName is the manifest mapping each recycle item's relative
// path (under the volume root) to its original absolute path. It is stored
// inside each recycle dir so the original location stays co-located with the
// moved data and restore is exact even after suffix-based de-duplication.
const trashManifestFileName = "restore_manifest.json"

// recordTrashManifest records the original absolute path of a moved item keyed
// by its relative path inside the recycle dir.
func recordTrashManifest(trashDir, relPath, originalPath string) {
	manifestPath := filepath.Join(trashDir, trashManifestFileName)

	m := map[string]string{}
	if data, err := os.ReadFile(manifestPath); err == nil {
		_ = json.Unmarshal(data, &m)
	}
	m[relPath] = originalPath

	if data, err := json.MarshalIndent(m, "", "  "); err == nil {
		if err := os.WriteFile(manifestPath, data, 0644); err != nil {
			slog.Warn("failed to write recycle manifest", "error", err)
		}
	}
}

// readTrashManifest returns the item-name -> original-path map of a recycle dir.
func readTrashManifest(trashDir string) map[string]string {
	manifestPath := filepath.Join(trashDir, trashManifestFileName)
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil
	}
	m := map[string]string{}
	if err := json.Unmarshal(data, &m); err != nil {
		return nil
	}
	return m
}

// copyDirTree recursively copies dir into dest.
func copyDirTree(src, dest string) error {
	info, err := os.Lstat(src)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		// Skip symlinks to avoid traversing outside the allowed tree.
		return nil
	}
	// Skip special files (sockets, FIFOs, device nodes, etc.). They are runtime
	// artifacts that cannot be read as regular files and carry no data worth
	// keeping in the recycle bin — copying them would fail (e.g. "open
	// /path/xxx.sock: no such device or address") and abort the whole move.
	if !info.Mode().IsRegular() && !info.IsDir() {
		return nil
	}
	if !info.IsDir() {
		return copyFile(src, dest)
	}
	if err := os.MkdirAll(dest, info.Mode().Perm()); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if err := copyDirTree(filepath.Join(src, entry.Name()), filepath.Join(dest, entry.Name())); err != nil {
			return err
		}
	}
	return nil
}

func copyFile(src, dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return err
	}
	// Skip special files defensively (should have been filtered by copyDirTree).
	info, err := os.Lstat(src)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return nil
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dest, data, 0644)
}

// isReservedAppDir reports whether a top-level directory name under an @app*
// root must never be moved to the recycle bin.
func isReservedAppDir(name string) bool {
	if reservedAppDirs[name] {
		return true
	}
	// Skip entries that don't look like application data dirs (dot/system files).
	if strings.HasPrefix(name, ".") || strings.HasPrefix(name, "@") {
		return true
	}
	return false
}

// CleanUninstalledAppDirsToTrash moves non-empty leftover directories of
// uninstalled applications into the fnOS per-user recycle bin so they can be
// recovered. It only ever touches directories whose name is NOT in the
// currently installed app set, and every target is verified again before
// moving. Items are auto-purged by the recycle cleaner after 24 hours.
func CleanUninstalledAppDirsToTrash(uid string) (types.RecycleCleanResult, error) {
	result := types.RecycleCleanResult{}
	if uid == "" {
		uid = defaultRecycleUID
	}

	// Critical safety: only ever clean leftover dirs of apps we KNOW are
	// uninstalled. Fetch the installed set directly and abort the whole clean
	// if we cannot reliably determine it, otherwise we might move directories
	// of installed apps into the recycle bin.
	installedApps, err := execAppcenterList()
	if err != nil {
		return result, fmt.Errorf("无法获取已安装应用列表，已取消清理: %w", err)
	}
	installedSet := make(map[string]bool, len(installedApps))
	for _, app := range installedApps {
		installedSet[app] = true
	}

	// Consider any configured @app* data root, including custom volumes.
	dirsToScan := append([]string{}, appDataDirsToRecycle...)
	for _, dir := range config.Get().LogDirs {
		if isAppDataRoot(dir) && !containsStr(dirsToScan, dir) {
			dirsToScan = append(dirsToScan, dir)
		}
	}

	for _, baseDir := range dirsToScan {
		normalizedBase := utils.SafePath(baseDir)
		if normalizedBase == "" {
			continue
		}
		if _, err := os.Stat(normalizedBase); os.IsNotExist(err) {
			continue
		}

		entries, err := os.ReadDir(normalizedBase)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %s", baseDir, err.Error()))
			continue
		}

		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			appName := entry.Name()
			// MUST be an uninstalled app — never touch anything installed.
			if installedSet[appName] {
				continue
			}
			if isReservedAppDir(appName) {
				continue
			}

			src := filepath.Join(normalizedBase, appName)
			// Defense in depth: re-verify symlink safety before moving.
			if utils.IsSymlinkPath(src) {
				continue
			}
			trashDir := recycleDirForPath(normalizedBase, uid)
			rel := filepath.Join(filepath.Base(normalizedBase), appName)
			if err := moveToTrash(src, trashDir, rel); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("%s: %s", src, err.Error()))
				continue
			}
			result.Moved++
			result.Dirs = append(result.Dirs, src)
		}
	}

	return result, nil
}

// RecycleItem describes one item currently sitting in a recycle dir.
type RecycleItem struct {
	Name          string    `json:"name"`
	RelPath       string    `json:"relPath"`
	Root          string    `json:"root"`
	TrashPath     string    `json:"trashPath"`
	OriginalPath  string    `json:"originalPath"`
	Size          int64     `json:"size"`
	SizeFormatted string    `json:"sizeFormatted"`
	Modified      time.Time `json:"modified"`
}

// ListRecycleItems lists items currently in all known recycle dirs, including
// each item's original location recorded at move time.
func ListRecycleItems() []RecycleItem {
	roots := getTrashRoots()
	if len(roots) == 0 {
		roots = []string{recycleDirForPath("/vol1", defaultRecycleUID)}
	}

	var items []RecycleItem
	for _, root := range roots {
		normalized := utils.SafePath(root)
		if normalized == "" {
			continue
		}
		manifest := readTrashManifest(normalized)

		for rel, original := range manifest {
			item := filepath.Join(normalized, rel)
			info, err := os.Stat(item)
			if err != nil {
				continue
			}
			items = append(items, RecycleItem{
				Name:          filepath.Base(rel),
				RelPath:       rel,
				Root:          normalized,
				TrashPath:     item,
				OriginalPath:  original,
				Size:          dirSize(item),
				SizeFormatted: utils.FormatBytes(dirSize(item)),
				Modified:      info.ModTime(),
			})
		}
	}
	return items
}

// isValidRestoreTarget reports whether original is a safe restore target — an
// absolute path under an fnOS app-data root on a storage volume. This is a
// defense-in-depth guard against a tampered manifest moving files to arbitrary
// system paths during restore.
func isValidRestoreTarget(original string) bool {
	o := utils.SafePath(original)
	if o == "" || o != original {
		return false
	}
	for _, root := range appDataDirsToRecycle {
		if o == root || strings.HasPrefix(o, root+"/") {
			return true
		}
	}
	return false
}

// RestoreRecycleItems restores the given recycle items back to their original
// absolute locations, as recorded in the manifest. Items whose original
// location is missing or unavailable are reported as errors.
func RestoreRecycleItems(root string, rels []string) (restored int, errors []string) {
	if root == "" {
		root = recycleDirForPath("/vol1", defaultRecycleUID)
	}
	normalized := utils.SafePath(root)
	if normalized == "" {
		return 0, []string{"无效的回收站根路径"}
	}

	manifest := readTrashManifest(normalized)
	for _, rel := range rels {
		original, ok := manifest[rel]
		if !ok {
			errors = append(errors, fmt.Sprintf("未找到回收站记录: %s", rel))
			continue
		}
		// Defense in depth: rel must resolve to a path strictly inside the
		// recycle root (no ".." traversal), and original must be a legitimate
		// app-data path — not an arbitrary location controlled by a tampered
		// manifest.
		relSafe := utils.SafePath(rel)
		if relSafe == "" {
			errors = append(errors, fmt.Sprintf("拒绝还原非法路径: %s", rel))
			continue
		}
		src := filepath.Join(normalized, relSafe)
		if !strings.HasPrefix(src, normalized+"/") && src != normalized {
			errors = append(errors, fmt.Sprintf("拒绝还原越界路径: %s", rel))
			continue
		}
		if !isValidRestoreTarget(original) {
			errors = append(errors, fmt.Sprintf("拒绝还原到非法位置: %s", original))
			continue
		}
		if utils.IsSymlinkPath(src) {
			errors = append(errors, fmt.Sprintf("拒绝还原符号链接: %s", rel))
			continue
		}
		// Re-create the parent directory so restore works even if the original
		// app-data root folder was recreated or is missing.
		if err := os.MkdirAll(filepath.Dir(original), 0755); err != nil {
			errors = append(errors, fmt.Sprintf("创建目标目录失败: %s", err.Error()))
			continue
		}
		// Prefer same-filesystem rename; fall back to cross-device copy+remove
		// when the recycle bin and the original location live on different
		// mount points (e.g. /vol1/@appdata vs /vol1/1000/.@#local/trash).
		if err := os.Rename(src, original); err != nil {
			if cpErr := copyDirTree(src, original); cpErr != nil {
				errors = append(errors, fmt.Sprintf("还原失败: %s: %s", original, err.Error()))
				continue
			}
			if rmErr := os.RemoveAll(src); rmErr != nil {
				errors = append(errors, fmt.Sprintf("还原后清理回收站目录失败: %s: %s", src, rmErr.Error()))
				continue
			}
		}
		delete(manifest, rel)
		restored++
	}

	if data, err := json.MarshalIndent(manifest, "", "  "); err == nil {
		_ = os.WriteFile(filepath.Join(normalized, trashManifestFileName), data, 0644)
	}
	return restored, errors
}

// dirSize computes the total size in bytes of a directory tree.
func dirSize(path string) int64 {
	var size int64
	_ = filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err == nil && info != nil && !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size
}

// appDataRootNames are the fnOS app-data root folder names (relative to a
// storage volume) that hold per-app data directories.
var appDataRootNames = []string{
	"@appdata",
	"@appshare",
	"@appconf",
	"@apphome",
	"@apptemp",
	"@appmeta",
}

// isAppDataRoot reports whether a log dir is an fnOS app-data root on any
// storage volume (e.g. /vol1/@appdata, /vol2/@appshare) whose one-level
// children are per-app directories.
func isAppDataRoot(dir string) bool {
	m := volRe.FindStringSubmatch(dir)
	if len(m) != 2 {
		return false
	}
	base := strings.TrimPrefix(strings.TrimPrefix(dir, m[1]), "/")
	for _, name := range appDataRootNames {
		if base == name {
			return true
		}
	}
	return false
}

func containsStr(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Auto-purge: permanently remove recycle items older than 24 hours.
// ---------------------------------------------------------------------------

type recycleCleaner struct {
	stopCh   chan struct{}
	stopOnce sync.Once
}

var globalRecycleCleaner *recycleCleaner

// InitRecycleCleaner loads persisted recycle roots and schedules a one-time
// stale-item sweep at startup so leftovers from a previous run don't linger.
func InitRecycleCleaner(dataDir string) error {
	globalRecycleCleaner = &recycleCleaner{stopCh: make(chan struct{})}
	rootsFile := filepath.Join(dataDir, "config", "trash_roots.json")
	loadTrashRoots(rootsFile)

	// Clean expired items once at startup.
	go globalRecycleCleaner.cleanExpired()
	return nil
}

// StartRecycleCleaner starts the hourly auto-purge loop.
func StartRecycleCleaner() error {
	if globalRecycleCleaner == nil {
		return fmt.Errorf("回收站清理器未初始化")
	}
	go globalRecycleCleaner.run()
	slog.Info("回收站自动清空调度已启动", "retention", recycleRetention.String())
	return nil
}

// StopRecycleCleaner stops the auto-purge loop.
func StopRecycleCleaner() error {
	if globalRecycleCleaner == nil {
		return nil
	}
	globalRecycleCleaner.stopOnce.Do(func() {
		close(globalRecycleCleaner.stopCh)
	})
	return nil
}

func (rc *recycleCleaner) run() {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-rc.stopCh:
			return
		case <-ticker.C:
			rc.cleanExpired()
		}
	}
}

// cleanExpired removes items under all known recycle roots that are older than
// the retention period.
func (rc *recycleCleaner) cleanExpired() {
	roots := getTrashRoots()
	if len(roots) == 0 {
		// No recorded roots yet — sweep the default path so stale data from a
		// previous version is still cleaned.
		roots = []string{recycleDirForPath("/vol1", defaultRecycleUID)}
	}

	cutoff := time.Now().Add(-recycleRetention)
	for _, root := range roots {
		normalized := utils.SafePath(root)
		if normalized == "" {
			continue
		}

		manifest := readTrashManifest(normalized)
		changed := false

		// Only purge items that we know about through the manifest. This avoids
		// ever treating structural subfolders (@appdata, @appshare) or the
		// manifest itself as an expired item.
		for rel, original := range manifest {
			item := filepath.Join(normalized, rel)
			info, err := os.Stat(item)
			if err != nil {
				// Item already gone — drop the stale record.
				delete(manifest, rel)
				changed = true
				continue
			}
			if info.ModTime().After(cutoff) {
				continue
			}
			if err := os.RemoveAll(item); err != nil {
				slog.Warn("清空过期回收站项目失败", "path", item, "error", err)
				continue
			}
			slog.Info("回收站项目已过期自动清空", "path", item, "original", original)
			delete(manifest, rel)
			changed = true
		}

		if changed {
			if data, err := json.MarshalIndent(manifest, "", "  "); err == nil {
				_ = os.WriteFile(filepath.Join(normalized, trashManifestFileName), data, 0644)
			}
		}
	}
}
