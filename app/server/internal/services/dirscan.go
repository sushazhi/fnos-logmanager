package services

import (
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/sushazhi/fnos-logmanager/internal/utils"
)

// 首页（/api/dirs 与 /api/logs/stats）原先各自对每个日志目录做两次全量递归遍历
// （一次找 .log，一次找归档），再对每个命中文件 os.Stat 求和；两个接口并发触发时
// 同一棵树最多被遍历四遍。日志目录数量（v0.8.0 起 11 个）与目录体积都在增长，
// 首页因此明显变慢。
//
// 这里的做法是：每个目录只遍历一次，同时收集日志/归档并累加体积，结果按 TTL 缓存，
// 并用 single-flight 让并发调用共享同一次遍历。

// largeLogFileSize 是「大文件」的判定阈值（与首页统计卡片口径一致）。
const largeLogFileSize = 10 * 1024 * 1024

// dirScanCacheTTL 是目录扫描结果的缓存时长。
//
// 首页会同时发起 /api/dirs 与 /api/logs/stats，TTL 只需覆盖这类并发即可；
// 任何写操作（清空/删除/清理/备份）都会通过 InvalidateDirScanCache 立即失效缓存，
// 因此不存在「清理后统计不更新」的问题。
const dirScanCacheTTL = 15 * time.Second

// dirScanConcurrency 是跨目录并行扫描的并发上限。目录扫描是 I/O 密集型，
// 但机械盘上并发过高反而会加剧寻道抖动，这里取一个保守值。
const dirScanConcurrency = 4

// dirScanResult 是单个日志目录一次遍历的聚合结果。
type dirScanResult struct {
	Exists        bool
	LogFiles      []string
	ArchiveFiles  []string
	LogSize       int64
	ArchiveSize   int64
	LargeLogFiles int

	// fileMeta 保存遍历时顺带取得的文件元信息（大小与修改时间），
	// 让 ListLogFiles 无需再对每个命中文件做一次 os.Stat。
	// 键为文件绝对路径。
	fileMeta map[string]dirFileMeta
}

// dirFileMeta 是遍历时缓存下来的单个文件元信息。
type dirFileMeta struct {
	Size    int64
	ModTime time.Time
}

// metaFor 返回已缓存的文件元信息；miss 时回退到一次 os.Stat。
//
// 正常情况下元信息在遍历阶段就已填充（省掉每个文件一次 stat 系统调用），
// 回退分支仅用于极端情况（TOCTOU：遍历后被替换/删除的文件）。
func (r *dirScanResult) metaFor(path string) (dirFileMeta, bool) {
	if r == nil {
		return dirFileMeta{}, false
	}
	if meta, ok := r.fileMeta[path]; ok {
		return meta, true
	}
	info, err := os.Stat(path)
	if err != nil {
		return dirFileMeta{}, false
	}
	return dirFileMeta{Size: info.Size(), ModTime: info.ModTime()}, true
}

type dirScanCacheEntry struct {
	result *dirScanResult
	at     time.Time
}

// dirScanInflight 表示一次正在进行的扫描，并发调用通过 done 等待并复用结果。
type dirScanInflight struct {
	done   chan struct{}
	result *dirScanResult
}

var (
	dirScanMu      sync.Mutex
	dirScanCache   = make(map[string]*dirScanCacheEntry)
	dirScanRunning = make(map[string]*dirScanInflight)
)

// InvalidateDirScanCache 清空目录扫描缓存。
//
// 任何可能改变日志目录内容的写操作（清空日志、删除日志、清理、备份等）之后都应调用，
// 否则随后的 /api/dirs、/api/logs/stats 会在 TTL 内继续返回旧统计。
func InvalidateDirScanCache() {
	dirScanMu.Lock()
	if len(dirScanCache) > 0 {
		dirScanCache = make(map[string]*dirScanCacheEntry)
	}
	dirScanMu.Unlock()
}

// ScanDirResult 返回目录 dir 的扫描聚合结果（按 TTL 缓存）。
//
// 同一目录的并发调用只会真正遍历一次文件系统。
func ScanDirResult(dir string) *dirScanResult {
	normalized := utils.SafePath(dir)
	if normalized == "" {
		return &dirScanResult{}
	}

	dirScanMu.Lock()
	if entry, ok := dirScanCache[normalized]; ok && time.Since(entry.at) < dirScanCacheTTL {
		result := entry.result
		dirScanMu.Unlock()
		return result
	}
	if running, ok := dirScanRunning[normalized]; ok {
		dirScanMu.Unlock()
		<-running.done
		return running.result
	}
	running := &dirScanInflight{done: make(chan struct{})}
	dirScanRunning[normalized] = running
	dirScanMu.Unlock()

	result := walkDirForLogs(normalized)

	dirScanMu.Lock()
	running.result = result
	dirScanCache[normalized] = &dirScanCacheEntry{result: result, at: time.Now()}
	delete(dirScanRunning, normalized)
	dirScanMu.Unlock()
	close(running.done)

	return result
}

// walkDirForLogs 遍历一个目录，一次收集日志文件、归档文件、体积与大文件数量。
//
// 过滤规则与 findFiles + isLogFile/isArchiveFile 完全一致（跳过符号链接与非日志目录），
// 只是把「两次 findFiles + 逐文件 os.Stat」合并为一次 WalkDir，并直接使用
// DirEntry.Info() 拿到的文件信息，省掉每个文件一次 stat 系统调用。
func walkDirForLogs(normalizedDir string) *dirScanResult {
	result := &dirScanResult{}

	info, err := os.Stat(normalizedDir)
	if err != nil || !info.IsDir() {
		return result
	}
	result.Exists = true

	start := time.Now()
	_ = filepath.WalkDir(normalizedDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			if os.IsPermission(err) {
				return nil
			}
			// 与 findFiles 保持一致：非权限错误终止遍历，保留已收集的部分结果。
			return err
		}
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}
		if d.IsDir() {
			if isIgnoredLogDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if len(result.LogFiles)+len(result.ArchiveFiles) >= findFilesHardCap {
			return filepath.SkipDir
		}

		name := d.Name()
		isLog := isLogFile(name)
		isArchive := isArchiveFile(name)
		if !isLog && !isArchive {
			return nil
		}

		size := int64(0)
		var modTime time.Time
		if fileInfo, infoErr := d.Info(); infoErr == nil {
			size = fileInfo.Size()
			modTime = fileInfo.ModTime()
		}
		if result.fileMeta == nil {
			result.fileMeta = make(map[string]dirFileMeta)
		}
		result.fileMeta[path] = dirFileMeta{Size: size, ModTime: modTime}
		if isLog {
			result.LogFiles = append(result.LogFiles, path)
			result.LogSize += size
			if size >= largeLogFileSize {
				result.LargeLogFiles++
			}
		}
		if isArchive {
			result.ArchiveFiles = append(result.ArchiveFiles, path)
			result.ArchiveSize += size
		}
		return nil
	})

	slog.Debug("dir scan done",
		"dir", normalizedDir,
		"logs", len(result.LogFiles),
		"archives", len(result.ArchiveFiles),
		"duration", time.Since(start))

	return result
}

// scanDirsParallel 并行扫描多个目录，保持与入参一致的下标顺序。
func scanDirsParallel(dirs []string) []*dirScanResult {
	results := make([]*dirScanResult, len(dirs))
	if len(dirs) == 0 {
		return results
	}

	workers := dirScanConcurrency
	if workers > len(dirs) {
		workers = len(dirs)
	}

	var wg sync.WaitGroup
	indexes := make(chan int)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range indexes {
				results[idx] = ScanDirResult(dirs[idx])
			}
		}()
	}
	for i := range dirs {
		indexes <- i
	}
	close(indexes)
	wg.Wait()

	return results
}
