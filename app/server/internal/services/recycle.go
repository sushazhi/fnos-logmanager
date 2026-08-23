package services

import (
	"encoding/json"
	"fmt"
	"io"
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
	// rel may be an ABSOLUTE source path (e.g. "/vol1/@appdata/redis") passed
	// by callers to preserve the original location. filepath.Join silently
	// DROPS the trashDir prefix when a later element is absolute (Go
	// semantics), which previously "moved" the item by renaming it in place
	// and wrote manifest keys full of ".." that could never be purged or
	// restored. Normalize rel to a path relative to the recycle root first:
	// strip the leading "/volN/" volume root, falling back to a generic
	// root-relative path for non-volume absolute paths.
	relPath := rel
	if filepath.IsAbs(rel) {
		if vol := volRootOf(rel); vol != "" {
			relPath = strings.TrimPrefix(rel, "/"+vol+"/")
		} else if v, err := filepath.Rel("/", rel); err == nil && v != ".." && !strings.HasPrefix(v, ".."+string(filepath.Separator)) {
			relPath = v
		}
	}
	relPath = filepath.Clean(relPath)
	if relPath == "" || relPath == "." || relPath == ".." || strings.HasPrefix(relPath, ".."+string(filepath.Separator)) {
		return fmt.Errorf("无效的回收站相对路径: %s", rel)
	}

	dest := uniqueDest(filepath.Join(trashDir, relPath))
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return fmt.Errorf("创建回收站目录失败: %w", err)
	}

	// FIX(bug 13): uniqueDest may append a numeric suffix (redis -> redis-1)
	// to avoid clobbering a previous same-name item. The manifest and times
	// records must be keyed by the ACTUAL on-disk relative path (dest's rel),
	// not the unsuffixed input rel — otherwise a second same-name move
	// overwrites the first entry, making that first item invisible, un-restorable
	// and never purged (permanent orphaned disk usage).
	relKey, err := filepath.Rel(trashDir, dest)
	if err != nil || relKey == "" || relKey == "." {
		relKey = rel
	}

	movedAt := time.Now()
	if err := os.Rename(src, dest); err == nil {
		recordTrashRoot(trashDir)
		recordTrashManifest(trashDir, relKey, src)
		recordTrashTime(trashDir, relKey, movedAt)
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
	recordTrashManifest(trashDir, relKey, src)
	recordTrashTime(trashDir, relKey, movedAt)
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

// trashTimesFileName records the unix-second time at which each recycle item
// was moved into the recycle bin, keyed by the item's relative path. The move
// time is authoritative for the 24h retention window: unlike a directory's
// mtime (which os.Rename preserves from the source app-data dir), it is the
// actual moment the item entered the recycle bin, so auto-purge fires reliably
// ~24h after the move regardless of how old the leftover data itself is.
const trashTimesFileName = "restore_manifest_times.json"

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

// recordTrashTime persists the unix-second time at which a recycle item was
// moved into the recycle bin. This timestamp drives the 24h auto-purge, so it
// must be recorded even if the manifest already exists from an older version.
func recordTrashTime(trashDir, relPath string, movedAt time.Time) {
	timesPath := filepath.Join(trashDir, trashTimesFileName)

	m := map[string]int64{}
	if data, err := os.ReadFile(timesPath); err == nil {
		_ = json.Unmarshal(data, &m)
	}
	m[relPath] = movedAt.Unix()

	if data, err := json.MarshalIndent(m, "", "  "); err == nil {
		if err := os.WriteFile(timesPath, data, 0644); err != nil {
			slog.Warn("failed to write recycle move-times", "error", err)
		}
	}
}

// readTrashTimes returns the rel-path -> unix-second move-time map of a recycle
// dir. Entries for items that are no longer present are tolerated by callers.
func readTrashTimes(trashDir string) map[string]int64 {
	timesPath := filepath.Join(trashDir, trashTimesFileName)
	data, err := os.ReadFile(timesPath)
	if err != nil {
		return nil
	}
	m := map[string]int64{}
	if err := json.Unmarshal(data, &m); err != nil {
		return nil
	}
	return m
}

// writeTrashTimes persists the given rel-path -> move-time map, dropping entries
// for items that were purged or restored.
func writeTrashTimes(trashDir string, m map[string]int64) {
	if data, err := json.MarshalIndent(m, "", "  "); err == nil {
		_ = os.WriteFile(filepath.Join(trashDir, trashTimesFileName), data, 0644)
	}
}

// copyDirTree recursively copies dir into dest.
func copyDirTree(src, dest string) error {
	info, err := os.Lstat(src)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		// FIX(bug 16): recreate the symlink instead of silently dropping it.
		// A hard-boundary check keeps us from following the target (we never
		// read through it), but preserving the link itself means a cross-volume
		// restore doesn't lose the app's symlinked config/data layout.
		target, err := os.Readlink(src)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
			return err
		}
		return os.Symlink(target, dest)
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

	// FIX(bug 16): stream the content with io.Copy instead of slurping the whole
	// file into memory with os.ReadFile. Leftover app-data dirs (notably log
	// dirs) can be multi-GB; buffering them entirely risks OOM on the NAS.
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode().Perm())
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	// Preserve the source file's modification time so the copied item behaves
	// like the original.
	return os.Chtimes(dest, info.ModTime(), info.ModTime())
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
			// rel MUST be an absolute path rooted at "/", because
			// RestoreRecycleItems re-validates it via utils.SafePath which
			// rejects anything not starting with "/". Using only
			// filepath.Base(normalizedBase) produced a bare "@appdata/xunlei"
			// key that always failed SafePath, making every restore
			// impossible. Build rel from the full absolute normalizedBase so
			// it is both a legal SafePath and stays strictly inside trashDir.
			rel := filepath.Join(normalizedBase, appName)
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
	MovedAt       time.Time `json:"movedAt,omitempty"`
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
		times := readTrashTimes(normalized)

		for rel, original := range manifest {
			// Only list entries that resolve strictly inside the recycle root;
			// skip tampered/legacy entries whose ".." would escape it (they are
			// also skipped by the auto-purge, so listing them would be misleading).
			if rel == "" || strings.ContainsAny(rel, "\x00\r\n") {
				continue
			}
			item := filepath.Clean(filepath.Join(normalized, rel))
			if item != normalized && !strings.HasPrefix(item, normalized+"/") {
				continue
			}
			info, err := os.Stat(item)
			if err != nil {
				continue
			}
			// Prefer the authoritative move time; fall back to the directory
			// mtime for items moved before the times file existed.
			movedAt := info.ModTime()
			if unix, ok := times[rel]; ok && unix > 0 {
				movedAt = time.Unix(unix, 0)
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
				MovedAt:       movedAt,
			})
		}
	}
	return items
}

// isValidRestoreTarget reports whether original is a safe restore target — an
// absolute path under an fnOS app-data root on a storage volume. This is a
// defense-in-depth guard against a tampered manifest moving files to arbitrary
// system paths during restore.
//
// FIX(bug 15): CleanUninstalledAppDirsToTrash scans custom volumes too (e.g.
// /vol2/@appdata from config.LogDirs), but this check only accepted the
// hard-coded /vol1 roots, so items moved from a custom volume could be sent to
// the recycle bin but never restored. Validate generically: any /volN followed
// by an @app* data-root folder is a legitimate restore target.
func isValidRestoreTarget(original string) bool {
	o := utils.SafePath(original)
	if o == "" || o != original {
		return false
	}
	m := volRe.FindStringSubmatch(o)
	if len(m) != 2 {
		return false
	}
	// Strip the leading "/volN/" prefix and require the very next path segment
	// to be an app-data root (@appdata etc.). Build the prefix from m[1] (the
	// bare "volN") so we don't depend on whether m[0] includes the trailing
	// slash — the previous version used m[0]+"/", which produced a double slash
	// ("/vol1//") that never matched, so EVERY legitimate target was rejected
	// and restore always failed.
	rest := strings.TrimPrefix(o, "/"+m[1]+"/")
	firstSeg := rest
	if idx := strings.IndexByte(rest, '/'); idx >= 0 {
		firstSeg = rest[:idx]
	}
	for _, name := range appDataRootNames {
		if firstSeg == name {
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
	times := readTrashTimes(normalized)
	for _, rel := range rels {
		// rels come from the client but are matched against manifest keys.
		// Reject empty/absolute/traversal values up front so they can never
		// alias the recycle root itself (an empty rel would resolve src to the
		// root, and the "srcSafe != normalized" guard alone would not reject it).
		if rel == "" || rel == "." || filepath.IsAbs(rel) || strings.ContainsAny(rel, "\x00\r\n") {
			errors = append(errors, fmt.Sprintf("拒绝还原非法路径: %s", rel))
			continue
		}
		original, ok := manifest[rel]
		if !ok {
			errors = append(errors, fmt.Sprintf("未找到回收站记录: %s", rel))
			continue
		}
		// Resolve the on-disk source path the same way ListRecycleItems builds
		// it: relative to the trash root. filepath.Join collapses any ".." that
		// a tampered key might contain.
		src := filepath.Join(normalized, rel)
		// Defense in depth: src must resolve to a path strictly inside the
		// recycle root (no ".." traversal, no escape), and original must be a
		// legitimate app-data path — not an arbitrary location controlled by a
		// tampered manifest.
		srcSafe := utils.SafePath(src)
		if srcSafe == "" {
			errors = append(errors, fmt.Sprintf("拒绝还原非法路径: %s", rel))
			continue
		}
		if !strings.HasPrefix(srcSafe, normalized+"/") && srcSafe != normalized {
			errors = append(errors, fmt.Sprintf("拒绝还原越界路径: %s", rel))
			continue
		}
		src = srcSafe
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
		delete(times, rel)
		restored++
	}

	if data, err := json.MarshalIndent(manifest, "", "  "); err == nil {
		_ = os.WriteFile(filepath.Join(normalized, trashManifestFileName), data, 0644)
	}
	writeTrashTimes(normalized, times)
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
	// The segment immediately after "/volN/" must be an app-data root folder.
	// Use "/"+m[1]+"/" as the prefix so it works regardless of whether m[0]
	// includes the trailing slash; the previous TrimPrefix(m[1]) left the
	// leading slash in place and always produced "vol1/@appdata" (never matched).
	base := strings.TrimPrefix(dir, "/"+m[1]+"/")
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
		times := readTrashTimes(normalized)
		changed := false

		// Only purge items that we know about through the manifest. This avoids
		// ever treating structural subfolders (@appdata, @appshare) or the
		// manifest itself as an expired item.
		for rel, original := range manifest {
			// FIX(bug 14): the manifest lives inside a user-accessible recycle
			// dir, so its rel values are attacker-controllable. Guard against a
			// tampered entry using ".." to escape the recycle root and point
			// RemoveAll at an arbitrary path — mirror the safety checks used by
			// RestoreRecycleItems. Never follow symlinks out of the tree either.
			//
			// The manifest key is a path RELATIVE to the recycle root, so
			// utils.SafePath cannot be applied directly: it rejects non-absolute
			// paths, which would silently reject EVERY legitimate entry and
			// permanently disable the 24h auto-purge (reported bug). Resolve
			// the absolute path and validate the RESULT stays inside the root.
			if rel == "" || strings.ContainsAny(rel, "\x00\r\n") {
				slog.Warn("回收站清扫拒绝非法相对路径", "rel", rel)
				continue
			}
			item := filepath.Clean(filepath.Join(normalized, rel))
			if item != normalized && !strings.HasPrefix(item, normalized+"/") {
				slog.Warn("回收站清扫拒绝越界路径", "rel", rel)
				continue
			}
			if utils.IsSymlinkPath(item) {
				slog.Warn("回收站清扫拒绝符号链接", "rel", rel)
				continue
			}
			info, err := os.Stat(item)
			if err != nil {
				// Item already gone — drop the stale record.
				delete(manifest, rel)
				delete(times, rel)
				changed = true
				continue
			}

			// Age is measured from the recorded move time when available
			// (authoritative). For items moved by older versions that predate
			// the times file, fall back to the directory mtime so they still get
			// purged rather than lingering forever.
			var age time.Time
			if unix, ok := times[rel]; ok && unix > 0 {
				age = time.Unix(unix, 0)
			} else {
				age = info.ModTime()
			}
			if age.After(cutoff) {
				continue
			}
			if err := os.RemoveAll(item); err != nil {
				slog.Warn("清空过期回收站项目失败", "path", item, "error", err)
				continue
			}
			slog.Info("回收站项目已过期自动清空", "path", item, "original", original)
			delete(manifest, rel)
			delete(times, rel)
			changed = true
		}

		if changed {
			if data, err := json.MarshalIndent(manifest, "", "  "); err == nil {
				_ = os.WriteFile(filepath.Join(normalized, trashManifestFileName), data, 0644)
			}
			writeTrashTimes(normalized, times)
		}

		// Structural subfolders (@appdata, @appshare, ...) are only containers
		// created to mirror the source app-data layout. Once every item inside a
		// subfolder has been purged (and so is no longer tracked in the manifest),
		// remove that empty subfolder too so the recycle dir returns to a clean
		// state. Safety rules:
		//   - only consider dirs whose name is an app-data root (@appdata etc.);
		//   - only remove a subfolder if it is now EMPTY, so we never touch any
		//     unrelated file the user may have placed in the recycle dir;
		//   - never remove the manifest / times files themselves.
		for _, dirName := range appDataRootNames {
			sub := filepath.Join(normalized, dirName)
			info, err := os.Lstat(sub)
			if err != nil || !info.IsDir() {
				continue
			}
			if dirIsEmpty(sub) {
				if err := os.RemoveAll(sub); err != nil {
					slog.Warn("清理回收站空子目录失败", "path", sub, "error", err)
					continue
				}
				slog.Info("已清理回收站空子目录", "path", sub)
			}
		}
	}
}

// dirIsEmpty reports whether the directory at path contains no entries at all.
func dirIsEmpty(path string) bool {
	entries, err := os.ReadDir(path)
	if err != nil {
		return false
	}
	return len(entries) == 0
}
