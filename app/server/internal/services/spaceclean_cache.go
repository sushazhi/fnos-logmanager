package services

// 系统空间扫描的缓存层：持久化快照 + stale-while-revalidate + 单飞 + 定期刷新。
//
// 为什么需要这一层：一次全量扫描要把 @appdata / docker / appcenter-downloads 等
// 目录树逐项 stat（见 scanSpaceCategories），真机上要几秒到几十秒。原来的缓存只
// 活在进程内存里、TTL 只有 30 秒，于是「隔一会儿再打开面板」「应用重启或升级」
// 「刚清理完」这三种最常见的情形都退化成阻塞式全量扫描，用户看到的就是长时间骨架屏。
//
// 现在的策略：
//   - 扫描结果落盘到 {DataDir}/config/space_scan_cache.json（原子写），重启后仍可用；
//   - 未超过新鲜期（spaceScanFreshTTL）直接返回快照，不做任何重扫；
//   - 超过新鲜期先返回旧快照，同时后台重扫（stale-while-revalidate），前端轮询收敛；
//   - 同一时刻只允许一次全量扫描在飞（单飞），并发打开面板不会把磁盘 IO 放大成 N 倍；
//   - 后台按 spaceScanRefreshInterval 定期检查，快照老了就悄悄刷新。
//
// 安全不变式：快照里存的是**路径与体积**，不是授权。清理时仍然逐条走
// matchCleanablePath 做服务端二次校验（见 CleanSpaceItems），快照里的路径即使
// 已经不在规则表内也会被跳过。因此持久化不会放宽任何删除权限。

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/sushazhi/fnos-logmanager/internal/utils"
)

const (
	// spaceScanFreshTTL 是「可以直接相信、不做任何重扫」的时长。
	//
	// 比原来的 30 秒大一个数量级：面板上的数据晚 5 分钟几乎没有影响，而每次打开
	// 都重扫的代价是用户实打实的等待。
	spaceScanFreshTTL = 5 * time.Minute

	// spaceScanRefreshInterval 是后台定期刷新间隔。
	spaceScanRefreshInterval = 30 * time.Minute

	// spaceScanStartupDelay 是启动后首次预热的延迟：避开进程启动期的 IO 高峰
	// （会话恢复、监控与自动清理的初始化也在同一时间读盘）。
	spaceScanStartupDelay = 20 * time.Second

	// spaceScanSnapshotVersion 是快照文件格式版本，结构变更时递增，旧文件直接丢弃。
	spaceScanSnapshotVersion = 1

	// spaceScanSnapshotMaxAge 是快照可被信任的最长时间。停机数周后重启，旧数据
	// 已经不能代表当前磁盘状态，宁可重新扫一次。
	spaceScanSnapshotMaxAge = 7 * 24 * time.Hour

	// spaceScanSnapshotFileName 是快照文件名，位于 {DataDir}/config/ 下。
	spaceScanSnapshotFileName = "space_scan_cache.json"
)

// spaceScanSnapshot 是落盘包装：带上格式版本与写入时间，便于识别格式变更，
// 也便于排查「这份数据是什么时候写下去的」。
type spaceScanSnapshot struct {
	Version int              `json:"version"`
	SavedAt time.Time        `json:"savedAt"`
	Result  SpaceUsageResult `json:"result"`
}

// spaceScanFlight 表示一次正在进行的全量扫描；done 关闭后 result / err 可读。
type spaceScanFlight struct {
	done   chan struct{}
	result SpaceUsageResult
	err    error
}

var (
	spaceScanMu sync.Mutex
	// spaceScanCache 是最近一次完整扫描结果；可能是本次启动扫出来的，也可能是
	// 从磁盘快照载入的。
	spaceScanCache *SpaceUsageResult
	// spaceScanNeedsRefresh 表示快照仍然可展示、但磁盘状态已经变了（例如刚清理过），
	// 下一次读取即使没超过新鲜期也要顺手后台重扫。
	spaceScanNeedsRefresh bool
	// spaceScanGeneration 是失效代数：扫描启动时记下，落地时若已变化就丢弃结果，
	// 避免把「清理之前开始、清理之后结束」的数据当成最新状态缓存起来。
	spaceScanGeneration uint64
	// spaceScanInflight 是当前在飞的全量扫描，用于单飞。
	spaceScanInflight *spaceScanFlight
	// spaceScanSnapshotPath 是快照文件路径；为空表示不落盘（未初始化）。
	spaceScanSnapshotPath string
)

// spaceScanExecute 是真正执行全量扫描的注入点，只为测试服务。
//
// 与 systemDeviceOf 同样的理由：真机上才存在 /volN，而单飞、SWR、持久化这些逻辑
// 必须在开发机上也能完整验证，不能被「没有存储卷」提前挡掉。
var spaceScanExecute = scanSpaceUsageUncached

// spaceScanMounts 是「枚举存储卷 + 挂载判定」的注入点，只为测试服务。
//
// 返回 oracle 是为了让同一份挂载表贯穿「筛选存储卷」与后续的挂载判定，避免每个请求
// 重复读挂载表。这里直接调 spaceVolumesIn 而不是 spaceVolumes：后者内部会自己读一次
// 挂载表，那样一次请求要读两遍。
var spaceScanMounts = func() ([]string, mountOracle) {
	oracle := newMountOracle()
	return spaceVolumesIn("/", oracle), oracle
}

// ---------------------------------------------------------------------------
// 生命周期
// ---------------------------------------------------------------------------

// InitSpaceScanCache 指定快照文件位置，并尝试载入上次的快照。
//
// 载入失败不算错误：首次启动没有快照文件是正常情况，此时退化为一次全量扫描。
func InitSpaceScanCache(dataDir string) error {
	if dataDir == "" {
		return fmt.Errorf("空间扫描缓存未指定数据目录")
	}
	path := filepath.Join(dataDir, "config", spaceScanSnapshotFileName)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("创建空间扫描缓存目录失败: %w", err)
	}

	spaceScanMu.Lock()
	spaceScanSnapshotPath = path
	spaceScanMu.Unlock()

	// 配对装配停止通道：Start/Stop 都要求 globalSpaceScanRefresher 非 nil。
	initSpaceScanRefresher()

	snapshot, err := loadSpaceScanSnapshot(path)
	if err != nil {
		slog.Debug("空间扫描快照不可用，将在首次请求时扫描", "path", path, "error", err)
		return nil
	}

	spaceScanMu.Lock()
	spaceScanCache = snapshot
	spaceScanNeedsRefresh = false
	spaceScanMu.Unlock()

	slog.Info("已载入空间扫描快照",
		"scannedAt", snapshot.ScannedAt.Format(time.RFC3339),
		"age", time.Since(snapshot.ScannedAt).Truncate(time.Second).String(),
		"categories", len(snapshot.Categories),
		"reclaimable", snapshot.Reclaimable)
	return nil
}

// spaceScanRefresher 是后台定期刷新的生命周期持有者。
type spaceScanRefresher struct {
	stopCh    chan struct{}
	stopOnce  sync.Once
	startOnce sync.Once
}

var globalSpaceScanRefresher *spaceScanRefresher

// StartSpaceScanRefresher 启动后台定期刷新：启动后延迟预热一次，之后按间隔检查，
// 只在快照过期时才真正重扫。
func StartSpaceScanRefresher() error {
	if globalSpaceScanRefresher == nil {
		return fmt.Errorf("空间扫描缓存未初始化")
	}
	globalSpaceScanRefresher.startOnce.Do(func() {
		go globalSpaceScanRefresher.run()
	})
	slog.Info("空间扫描后台刷新已启动",
		"interval", spaceScanRefreshInterval.String(),
		"freshTTL", spaceScanFreshTTL.String())
	return nil
}

// StopSpaceScanRefresher 停止后台刷新循环。
func StopSpaceScanRefresher() error {
	if globalSpaceScanRefresher == nil {
		return nil
	}
	globalSpaceScanRefresher.stopOnce.Do(func() {
		close(globalSpaceScanRefresher.stopCh)
	})
	return nil
}

func (r *spaceScanRefresher) run() {
	// 首次预热用一次性定时器，之后交给 ticker；两者都只做「过期才扫」的判断。
	startup := time.NewTimer(spaceScanStartupDelay)
	defer startup.Stop()
	ticker := time.NewTicker(spaceScanRefreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-r.stopCh:
			return
		case <-startup.C:
			refreshSpaceScanIfStale()
		case <-ticker.C:
			refreshSpaceScanIfStale()
		}
	}
}

// initSpaceScanRefresher 由 InitSpaceScanCache 调用，装配停止通道。
//
// 只在还没有持有者时新建：重复 Init（测试、将来的热重载）若直接覆盖指针，之前
// 那个 refresher 的 goroutine 就再也没人能停——Stop 只能看到新指针，旧循环会一直
// 活到进程结束。保留已有实例即可，反正它的 stopCh 还没被关闭。
func initSpaceScanRefresher() {
	if globalSpaceScanRefresher != nil {
		return
	}
	globalSpaceScanRefresher = &spaceScanRefresher{stopCh: make(chan struct{})}
}

// refreshSpaceScanIfStale 在快照缺失或过期时执行一次后台重扫。
func refreshSpaceScanIfStale() {
	// 没有存储卷时不必重扫：此时扫描只会返回「未检测到存储卷」，徒增日志噪音。
	vols, _ := spaceScanMounts()
	if len(vols) == 0 {
		slog.Debug("跳过空间扫描预热：未检测到存储卷")
		return
	}

	spaceScanMu.Lock()
	stale := spaceScanCache == nil ||
		spaceScanNeedsRefresh ||
		time.Since(spaceScanCache.ScannedAt) >= spaceScanFreshTTL
	spaceScanMu.Unlock()

	if !stale {
		return
	}
	flight, leader := beginSpaceScan()
	<-flight.done
	if leader && flight.err != nil {
		slog.Warn("空间扫描后台刷新失败", "error", flight.err)
	}
}

// ---------------------------------------------------------------------------
// 读取
// ---------------------------------------------------------------------------

// cloneSpaceUsageResult 深拷贝一份结果，重点是那些会被就地改写的切片。
//
// 只做值拷贝是不够的：Categories 是切片，Items 又是切片，浅拷贝后调用方拿到的
// 仍与缓存共享底层数组，任何 append/改写都会静默污染缓存（甚至改变已经发出、
// 正在被 JSON 序列化的响应）。分类与条目数量不多，全量复制最省心。
func cloneSpaceUsageResult(src *SpaceUsageResult) SpaceUsageResult {
	dst := *src
	if src.Categories != nil {
		dst.Categories = make([]SpaceCategory, len(src.Categories))
		for i, cat := range src.Categories {
			copied := cat
			if cat.Items != nil {
				copied.Items = make([]SpaceItem, len(cat.Items))
				copy(copied.Items, cat.Items)
			}
			dst.Categories[i] = copied
		}
	}
	if src.Volumes != nil {
		dst.Volumes = make([]VolumeUsage, len(src.Volumes))
		copy(dst.Volumes, src.Volumes)
	}
	if src.Notices != nil {
		dst.Notices = make([]SpaceNotice, len(src.Notices))
		copy(dst.Notices, src.Notices)
	}
	if src.Errors != nil {
		dst.Errors = make([]string, len(src.Errors))
		copy(dst.Errors, src.Errors)
	}
	return dst
}

// cachedSpaceSnapshot 返回当前可用的快照：先内存，再磁盘。
//
// 返回的是独立副本，调用方可以随意改写字段而不会污染缓存。
func cachedSpaceSnapshot() *SpaceUsageResult {
	spaceScanMu.Lock()
	if spaceScanCache != nil {
		cached := cloneSpaceUsageResult(spaceScanCache)
		spaceScanMu.Unlock()
		return &cached
	}
	gen := spaceScanGeneration
	path := spaceScanSnapshotPath
	spaceScanMu.Unlock()

	if path == "" {
		return nil
	}

	snapshot, err := loadSpaceScanSnapshot(path)
	if err != nil {
		slog.Debug("空间扫描快照不可读", "path", path, "error", err)
		return nil
	}

	spaceScanMu.Lock()
	defer spaceScanMu.Unlock()
	// 读盘期间可能已经有扫描落地，或者缓存被硬失效：两种情况都以内存状态为准。
	if spaceScanCache != nil {
		cached := cloneSpaceUsageResult(spaceScanCache)
		return &cached
	}
	if gen != spaceScanGeneration {
		return nil
	}
	spaceScanCache = snapshot
	cached := cloneSpaceUsageResult(snapshot)
	return &cached
}

// spaceScanRefreshNeeded 报告快照是否被标记为「需要重扫」（例如刚清理过）。
func spaceScanRefreshNeeded() bool {
	spaceScanMu.Lock()
	defer spaceScanMu.Unlock()
	return spaceScanNeedsRefresh
}

// spaceScanRunning 报告当前是否有全量扫描在飞。
func spaceScanRunning() bool {
	spaceScanMu.Lock()
	defer spaceScanMu.Unlock()
	return spaceScanInflight != nil
}

// ensureSpaceScanRefresh 在需要时启动后台重扫，并返回「此刻是否有扫描在飞」，
// 供前端显示「正在后台更新」。
//
// 先注册再返回：若等到 goroutine 起来才置位，同一个请求里返回的 refreshing 可能
// 是 false，前端就不会去轮询，后台结果要等到用户下次手动刷新才看得到。
func ensureSpaceScanRefresh(stale bool) bool {
	if stale {
		beginSpaceScan()
	}
	return spaceScanRunning()
}

// spaceScanView 把一份结果整理成对外响应：运行期字段实时重算，并标注数据时效。
//
// 容量条与 /tmp 归属这类信息读起来很便宜（每个挂载点一次 statfs、两次 Lstat），
// 实时重算可以避免「磁盘容量是新的、分类体积是旧的」这种自相矛盾的展示；
// 而分类体积必须来自快照，因为重算它就是那几秒到几十秒的全量扫描。
func spaceScanView(result SpaceUsageResult, vols []string, oracle mountOracle, stale, refreshing bool) SpaceUsageResult {
	view := result
	// 重算失败（例如瞬时读不到挂载表）时保留快照里的容量行：宁可显示旧数字，
	// 也不要在界面上把磁盘整片抹掉。
	if len(vols) > 0 {
		if live := scanVolumeUsage(spaceDiskMounts(vols, nil)); len(live) > 0 {
			view.Volumes = live
		}
	}
	view.TempOnSystemDisk = nil
	if same, known := systemTempOnSystemDisk(); known {
		view.TempOnSystemDisk = &same
	}
	view.CacheAgeMs = 0
	view.CacheAgeSeconds = 0
	if !view.ScannedAt.IsZero() {
		if age := time.Since(view.ScannedAt); age > 0 {
			view.CacheAgeMs = age.Milliseconds()
			view.CacheAgeSeconds = int64(age.Seconds())
		}
	}
	view.Stale = stale
	view.Refreshing = refreshing
	return view
}

// spaceScanVolumeSetChanged 判断当前存储卷集合是否与快照记录的不同。
//
// 插拔硬盘后旧快照的分类必然不完整，必须重扫。快照里的 Volumes 还包含系统分区与
// 外接存储，因此只校验「当前存在的存储卷是否都被快照覆盖」，不去比较数量。
func spaceScanVolumeSetChanged(snapshot []VolumeUsage, vols []string) bool {
	if len(snapshot) == 0 {
		return true
	}
	known := make(map[string]bool, len(snapshot))
	for _, v := range snapshot {
		known[v.Mount] = true
	}
	for _, v := range vols {
		if !known[v] {
			return true
		}
	}
	return false
}

// recomputeSpaceTotals 依据条目重算分类与总量的汇总字段。
//
// 扫描与「清理后就地剔除条目」都需要这套汇总，抽出来保证两处口径一致。
func recomputeSpaceTotals(result *SpaceUsageResult) {
	result.Reclaimable = 0
	result.CategoriesWithItems = 0
	for i := range result.Categories {
		cat := &result.Categories[i]
		cat.ItemCount = len(cat.Items)
		cat.TotalSize = 0
		cat.Reclaimable = 0
		for _, item := range cat.Items {
			cat.TotalSize += item.Size
			if item.Safe {
				cat.Reclaimable += item.Size
			}
		}
		if cat.Items == nil {
			cat.Items = []SpaceItem{}
		}
		cat.TotalSizeFormatted = utils.FormatBytes(cat.TotalSize)
		cat.ReclaimableFormatted = utils.FormatBytes(cat.Reclaimable)

		result.Reclaimable += cat.Reclaimable
		if cat.ItemCount > 0 {
			result.CategoriesWithItems++
		}
	}
	result.ReclaimableText = utils.FormatBytes(result.Reclaimable)
}

// ---------------------------------------------------------------------------
// 对外入口
// ---------------------------------------------------------------------------

// ScanSpaceUsage 返回系统空间占用。
//
//	force=false：优先返回已有数据（内存或磁盘快照），超过新鲜期时先返回旧数据，
//	            同时后台重扫，前端据此轮询收敛——面板打开不再是「等一次全量扫描」；
//	force=true：等待一次真正的全量扫描返回（界面上的「重新扫描」走这条路）。
//
// 注意 force=true 也会加入正在进行的后台扫描而不是另起一次：那次扫描产出的就是
// 当下最新的数据，为此再遍历一遍磁盘没有意义。
func ScanSpaceUsage(force bool) (SpaceUsageResult, error) {
	vols, oracle := spaceScanMounts()
	// 没有存储卷时保持原有的报错语义：这比返回一份空结果更能说明问题。
	if len(vols) == 0 {
		return SpaceUsageResult{}, fmt.Errorf("未检测到存储卷（/volN）")
	}

	if force {
		flight, _ := beginSpaceScan()
		<-flight.done
		if flight.err != nil {
			return SpaceUsageResult{}, flight.err
		}
		return spaceScanView(flight.result, vols, oracle, false, false), nil
	}

	if cached := cachedSpaceSnapshot(); cached != nil {
		age := time.Since(cached.ScannedAt)
		stale := spaceScanRefreshNeeded() ||
			age >= spaceScanFreshTTL ||
			spaceScanVolumeSetChanged(cached.Volumes, vols)
		refreshing := ensureSpaceScanRefresh(stale)
		return spaceScanView(*cached, vols, oracle, stale, refreshing), nil
	}

	// 没有任何可用快照（首次启动且磁盘上没有快照文件）：只能同步扫一次。
	flight, _ := beginSpaceScan()
	<-flight.done
	if flight.err != nil {
		return SpaceUsageResult{}, flight.err
	}
	return spaceScanView(flight.result, vols, oracle, false, false), nil
}

// InvalidateSpaceScanCache 硬失效：丢弃内存快照并删除已落盘的快照文件。
//
// 用在「磁盘状态已经不可信」的场合。清理操作走的是 pruneSpaceSnapshotAfterClean
// ——它只剔除被删条目并标记待刷新，让用户下次打开面板仍能立刻看到数据。
func InvalidateSpaceScanCache() {
	spaceScanMu.Lock()
	spaceScanGeneration++
	spaceScanCache = nil
	spaceScanNeedsRefresh = true
	path := spaceScanSnapshotPath
	spaceScanMu.Unlock()

	if path == "" {
		return
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		slog.Warn("删除空间扫描快照失败", "path", path, "error", err)
	}
}

// pruneSpaceSnapshotAfterClean 从快照里剔除本次真正被清理掉的条目。
//
// 为什么不直接清空缓存：清空之后用户再打开面板又要等一次全量扫描——那正是本文件
// 要消灭的体验。而被删的条目已经不在磁盘上，把它们的体积继续留在快照里会让面板
// 显示「还能释放 X」。所以就地剔除并重算汇总，同时置 needsRefresh，让下一次读取
// 在返回旧数据的同时后台重扫，把剩余体积校正到真实值。
//
// 试运行不会调用这里：它没有产生任何磁盘变更，快照依然有效。
func pruneSpaceSnapshotAfterClean(cleaned []SpaceCleanItemResult) {
	gone := make(map[string]bool)
	for _, item := range cleaned {
		switch item.Status {
		case "deleted", "recycled", "truncated":
			gone[item.Path] = true
		}
	}
	if len(gone) == 0 {
		return
	}

	spaceScanMu.Lock()
	defer spaceScanMu.Unlock()

	spaceScanNeedsRefresh = true
	if spaceScanCache == nil {
		return
	}

	changed := false
	for i := range spaceScanCache.Categories {
		cat := &spaceScanCache.Categories[i]
		// 重新分配切片而不是就地过滤：这样快照里的底层数组不会被改写，
		// 与已经发出的响应彻底解耦。
		kept := make([]SpaceItem, 0, len(cat.Items))
		for _, item := range cat.Items {
			if gone[item.Path] {
				changed = true
				continue
			}
			kept = append(kept, item)
		}
		cat.Items = kept
	}
	if !changed {
		return
	}

	recomputeSpaceTotals(spaceScanCache)

	if path := spaceScanSnapshotPath; path != "" {
		snapshot := *spaceScanCache
		saveSpaceScanSnapshot(path, &snapshot)
	}
}

// ---------------------------------------------------------------------------
// 扫描
// ---------------------------------------------------------------------------

// beginSpaceScan 注册并启动一次全量扫描，返回它的完成通道。
//
// 已有扫描在飞时直接复用（leader=false）：面板可能在多个标签页或多个客户端同时
// 打开，每次打开都触发全量遍历会把磁盘 IO 放大成 N 倍。等待方拿到的是同一次扫描
// 的结果，时效性没有损失。
func beginSpaceScan() (flight *spaceScanFlight, leader bool) {
	spaceScanMu.Lock()
	if inflight := spaceScanInflight; inflight != nil {
		spaceScanMu.Unlock()
		return inflight, false
	}
	flight = &spaceScanFlight{done: make(chan struct{})}
	spaceScanInflight = flight
	gen := spaceScanGeneration
	spaceScanMu.Unlock()

	go spaceScanWorker(flight, gen)
	return flight, true
}

// spaceScanWorker 执行扫描并落地结果。
func spaceScanWorker(flight *spaceScanFlight, gen uint64) {
	result, err := spaceScanExecute()

	spaceScanMu.Lock()
	flight.result, flight.err = result, err
	spaceScanInflight = nil
	// 扫描期间发生过失效（例如用户清理了空间）时丢弃本次结果：它反映的是清理前的
	// 磁盘状态，缓存下来会让面板长时间显示错误数字。
	if err == nil && gen == spaceScanGeneration {
		stored := result
		spaceScanCache = &stored
		spaceScanNeedsRefresh = false
		if path := spaceScanSnapshotPath; path != "" {
			// 落盘放在临界区内：快照文件不大，而「内存与磁盘一致」比省这几毫秒重要。
			saveSpaceScanSnapshot(path, &stored)
		}
	}
	close(flight.done)
	spaceScanMu.Unlock()

	if err == nil {
		slog.Debug("space scan done",
			"volumes", len(result.Volumes),
			"categories", len(result.Categories),
			"reclaimable", result.Reclaimable,
			"duration", time.Duration(result.DurationMs)*time.Millisecond)
	}
}

// ---------------------------------------------------------------------------
// 落盘
// ---------------------------------------------------------------------------

// saveSpaceScanSnapshot 原子落盘：先写同目录临时文件再 rename，避免进程在写盘
// 途中被杀时留下半截 JSON（同 bookmark.go 的做法）。
//
// 快照是纯缓存，任何失败都只记录日志：磁盘满或权限异常不应该让扫描结果本身失败。
func saveSpaceScanSnapshot(path string, result *SpaceUsageResult) {
	data, err := json.MarshalIndent(spaceScanSnapshot{
		Version: spaceScanSnapshotVersion,
		SavedAt: time.Now(),
		Result:  *result,
	}, "", "  ")
	if err != nil {
		slog.Warn("序列化空间扫描快照失败", "error", err)
		return
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		slog.Warn("创建空间扫描快照目录失败", "path", dir, "error", err)
		return
	}

	tmpFile := path + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0o600); err != nil {
		slog.Warn("写入空间扫描快照失败", "path", tmpFile, "error", err)
		return
	}
	if err := os.Rename(tmpFile, path); err != nil {
		slog.Warn("替换空间扫描快照失败", "path", path, "error", err)
		_ = os.Remove(tmpFile)
	}
}

// loadSpaceScanSnapshot 读取并校验快照文件。
func loadSpaceScanSnapshot(path string) (*SpaceUsageResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var snapshot spaceScanSnapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return nil, fmt.Errorf("快照解析失败: %w", err)
	}
	if snapshot.Version != spaceScanSnapshotVersion {
		return nil, fmt.Errorf("快照格式版本不匹配（文件 %d，当前 %d）", snapshot.Version, spaceScanSnapshotVersion)
	}
	if snapshot.Result.ScannedAt.IsZero() {
		return nil, fmt.Errorf("快照缺少扫描时间")
	}

	age := time.Since(snapshot.Result.ScannedAt)
	if age > spaceScanSnapshotMaxAge {
		return nil, fmt.Errorf("快照过旧（%s）", age.Truncate(time.Hour).String())
	}
	// 扫描时间明显落在未来说明系统时钟被调整过，此时 age 恒为负、快照会一直显得
	// 「新鲜」，所以直接判为不可信。
	if age < -spaceScanFreshTTL {
		return nil, fmt.Errorf("快照时间戳异常（%s）", snapshot.Result.ScannedAt.Format(time.RFC3339))
	}

	result := snapshot.Result
	// 保证切片字段非 nil，避免前端拿到 null 又要额外判空。
	if result.Categories == nil {
		result.Categories = []SpaceCategory{}
	}
	if result.Notices == nil {
		result.Notices = []SpaceNotice{}
	}
	if result.Volumes == nil {
		result.Volumes = []VolumeUsage{}
	}
	return &result, nil
}
