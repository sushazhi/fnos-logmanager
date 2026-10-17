package services

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// 本文件只测缓存层自身的行为：单飞、TTL/SWR、持久化、失效与清理后的剔除。
//
// 全部通过 spaceScanExecute / spaceScanMounts 两个注入点驱动，因此在 Windows
// 开发机上也能跑——真机之外没有 /volN，直接调 ScanSpaceUsage 只会得到
// 「未检测到存储卷」。这是这套测试能在本地挡住并发与持久化回归的前提。

// spaceScanTestEnv 把缓存层恢复成干净状态，并把注入点指向测试替身。
//
// 缓存层用的是包级变量（与生产一致：它本来就是进程级单例），测试之间必须显式
// 复位，否则前一个用例的快照会漏到下一个。
func spaceScanTestEnv(t *testing.T, result SpaceUsageResult, err error) *int64 {
	t.Helper()

	origExecute := spaceScanExecute
	origMounts := spaceScanMounts
	origCache, origNeeds, origGen, origInflight, origPath := snapshotSpaceScanState()
	origRefresher := globalSpaceScanRefresher

	var calls int64
	spaceScanExecute = func() (SpaceUsageResult, error) {
		atomic.AddInt64(&calls, 1)
		return result, err
	}
	spaceScanMounts = func() ([]string, mountOracle) {
		return []string{"/vol1"}, mountOracle{}
	}

	setSpaceScanState(nil, false, 0, nil, "")

	t.Cleanup(func() {
		spaceScanExecute = origExecute
		spaceScanMounts = origMounts
		setSpaceScanState(origCache, origNeeds, origGen, origInflight, origPath)
		globalSpaceScanRefresher = origRefresher
	})
	return &calls
}

// snapshotSpaceScanState 取出缓存层的全部包级状态（含在飞的扫描指针）。
func snapshotSpaceScanState() (cache *SpaceUsageResult, needs bool, gen uint64, inflight *spaceScanFlight, path string) {
	spaceScanMu.Lock()
	defer spaceScanMu.Unlock()
	return spaceScanCache, spaceScanNeedsRefresh, spaceScanGeneration, spaceScanInflight, spaceScanSnapshotPath
}

func setSpaceScanState(cache *SpaceUsageResult, needs bool, gen uint64, inflight *spaceScanFlight, path string) {
	spaceScanMu.Lock()
	defer spaceScanMu.Unlock()
	spaceScanCache, spaceScanNeedsRefresh, spaceScanGeneration = cache, needs, gen
	spaceScanInflight, spaceScanSnapshotPath = inflight, path
}

// testSpaceResult 造一份结构完整、时间可控的扫描结果。
func testSpaceResult(scannedAt time.Time, reclaimable int64) SpaceUsageResult {
	result := SpaceUsageResult{
		ScannedAt:   scannedAt,
		DurationMs:  12,
		Categories:  []SpaceCategory{{ID: "logs", Name: "日志", Items: []SpaceItem{{Path: "/vol1/logs/a.log", Size: reclaimable, Safe: true}}}},
		Notices:     []SpaceNotice{},
		Volumes:     []VolumeUsage{{Mount: "/vol1"}},
		Reclaimable: reclaimable,
	}
	recomputeSpaceTotals(&result)
	return result
}

// TestScanSpaceUsageServesSnapshotImmediately 是本次改动的核心诉求：
// 面板再次打开时不能再走全量扫描。
func TestScanSpaceUsageServesSnapshotImmediately(t *testing.T) {
	fresh := testSpaceResult(time.Now().Add(-time.Minute), 1024)
	calls := spaceScanTestEnv(t, fresh, nil)
	setSpaceScanState(&fresh, false, 0, nil, "")

	got, err := ScanSpaceUsage(false)
	if err != nil {
		t.Fatalf("读取快照失败: %v", err)
	}
	if n := atomic.LoadInt64(calls); n != 0 {
		t.Fatalf("新鲜快照不应触发扫描，却扫描了 %d 次", n)
	}
	if got.Reclaimable != fresh.Reclaimable {
		t.Fatalf("可回收体积应为 %d，实际 %d", fresh.Reclaimable, got.Reclaimable)
	}
	if got.Stale || got.Refreshing {
		t.Fatalf("新鲜数据不应标记为过期/刷新中，实际 stale=%v refreshing=%v", got.Stale, got.Refreshing)
	}
	if got.CacheAgeMs <= 0 {
		t.Fatalf("缓存年龄应为正数，实际 %d", got.CacheAgeMs)
	}
}

// TestScanSpaceUsageStaleServesOldAndRefreshes 覆盖 stale-while-revalidate：
// 过期数据要立刻返回，同时在后台重扫——用户不该为一次刷新等一个全量扫描。
func TestScanSpaceUsageStaleServesOldAndRefreshes(t *testing.T) {
	old := testSpaceResult(time.Now().Add(-2*spaceScanFreshTTL), 2048)
	calls := spaceScanTestEnv(t, testSpaceResult(time.Now(), 4096), nil)
	setSpaceScanState(&old, false, 0, nil, "")

	start := time.Now()
	got, err := ScanSpaceUsage(false)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("读取过期快照失败: %v", err)
	}
	// 返回的必须是旧数据（否则就是同步扫描了）。
	if got.Reclaimable != 2048 {
		t.Fatalf("过期时应先返回旧数据，期望 2048，实际 %d", got.Reclaimable)
	}
	if !got.Stale {
		t.Fatal("超过新鲜期的数据应标记 stale")
	}
	if elapsed > time.Second {
		t.Fatalf("过期读取不应阻塞，实际耗时 %v", elapsed)
	}

	// 后台扫描应已完成（替身是瞬时的）。
	waitFor(t, 2*time.Second, func() bool {
		cache, _, _, inflight, _ := snapshotSpaceScanState()
		return inflight == nil && cache != nil && cache.Reclaimable == 4096
	}, "后台刷新未在限期内完成")
	if n := atomic.LoadInt64(calls); n != 1 {
		t.Fatalf("后台应恰好扫描 1 次，实际 %d", n)
	}

	// 再读一次：这次应拿到刷新后的新鲜数据。
	again, err := ScanSpaceUsage(false)
	if err != nil {
		t.Fatalf("二次读取失败: %v", err)
	}
	if again.Reclaimable != 4096 || again.Stale {
		t.Fatalf("后台刷新后应返回新数据且不再 stale，实际 reclaimable=%d stale=%v", again.Reclaimable, again.Stale)
	}
}

// TestScanSpaceUsageForceWaitsForScan 保证「重新扫描」按钮仍然是阻塞且强制的。
func TestScanSpaceUsageForceWaitsForScan(t *testing.T) {
	fresh := testSpaceResult(time.Now(), 512)
	calls := spaceScanTestEnv(t, testSpaceResult(time.Now(), 8192), nil)
	setSpaceScanState(&fresh, false, 0, nil, "")

	got, err := ScanSpaceUsage(true)
	if err != nil {
		t.Fatalf("强制扫描失败: %v", err)
	}
	if got.Reclaimable != 8192 {
		t.Fatalf("force=true 应返回新扫描结果 8192，实际 %d", got.Reclaimable)
	}
	if n := atomic.LoadInt64(calls); n != 1 {
		t.Fatalf("强制扫描应执行 1 次，实际 %d", n)
	}
	if got.Stale || got.Refreshing {
		t.Fatalf("刚扫完不应标记 stale/refreshing，实际 stale=%v refreshing=%v", got.Stale, got.Refreshing)
	}
}

// TestScanSpaceUsageSingleFlight 是并发安全的关键：
// 多个面板同时打开不能把磁盘 IO 放大成 N 倍。
func TestScanSpaceUsageSingleFlight(t *testing.T) {
	var calls int64
	release := make(chan struct{})
	origExecute := spaceScanExecute
	origMounts := spaceScanMounts
	origCache, origNeeds, origGen, origInflight, origPath := snapshotSpaceScanState()
	t.Cleanup(func() {
		spaceScanExecute = origExecute
		spaceScanMounts = origMounts
		setSpaceScanState(origCache, origNeeds, origGen, origInflight, origPath)
	})

	spaceScanExecute = func() (SpaceUsageResult, error) {
		atomic.AddInt64(&calls, 1)
		<-release // 卡住，确保所有 goroutine 都撞在同一次扫描上
		return testSpaceResult(time.Now(), 4096), nil
	}
	spaceScanMounts = func() ([]string, mountOracle) { return []string{"/vol1"}, mountOracle{} }
	setSpaceScanState(nil, false, 0, nil, "")

	const workers = 8
	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			// 首次请求会同步扫描；其余请求应加入这一次扫描。
			result, err := ScanSpaceUsage(false)
			if err != nil || result.Reclaimable != 4096 {
				t.Errorf("并发扫描结果异常: reclaimable=%d err=%v", result.Reclaimable, err)
			}
		}()
	}

	waitFor(t, 2*time.Second, func() bool { return atomic.LoadInt64(&calls) == 1 }, "扫描未启动")
	close(release)
	wg.Wait()

	if n := atomic.LoadInt64(&calls); n != 1 {
		t.Fatalf("单飞失败：%d 个并发请求触发了 %d 次扫描", workers, n)
	}
}

// TestSpaceScanSnapshotRoundTrip 覆盖重启/升级后的冷启动路径。
func TestSpaceScanSnapshotRoundTrip(t *testing.T) {
	dir := t.TempDir()
	if err := InitSpaceScanCache(dir); err != nil {
		t.Fatalf("初始化缓存失败: %v", err)
	}

	original := testSpaceResult(time.Now().Add(-time.Minute), 777)
	saveSpaceScanSnapshot(filepath.Join(dir, "config", spaceScanSnapshotFileName), &original)

	// 模拟重启：清空内存状态，只留磁盘快照。
	setSpaceScanState(nil, false, 0, nil, filepath.Join(dir, "config", spaceScanSnapshotFileName))

	got := cachedSpaceSnapshot()
	if got == nil {
		t.Fatal("重启后应能从磁盘快照恢复数据")
	}
	if got.Reclaimable != 777 {
		t.Fatalf("恢复的可回收体积应为 777，实际 %d", got.Reclaimable)
	}
	if len(got.Categories) != 1 || got.Categories[0].ItemCount != 1 {
		t.Fatalf("恢复的分类结构不完整: %+v", got.Categories)
	}
}

// TestInitSpaceScanCacheIgnoresBadSnapshot 保证坏快照只会退化为一次全量扫描，
// 而不是让服务起不来或返回半截数据。
func TestInitSpaceScanCacheIgnoresBadSnapshot(t *testing.T) {
	cases := map[string]string{
		"截断的 JSON":   `{"version":1,"result":{"scannedAt":`,
		"版本不匹配":      `{"version":999,"savedAt":"2026-01-01T00:00:00Z","result":{"scannedAt":"2026-01-01T00:00:00Z"}}`,
		"缺少扫描时间":     `{"version":1,"savedAt":"2026-01-01T00:00:00Z","result":{}}`,
		"空文件":        ``,
		"JSON 但不是对象": `[]`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "config", spaceScanSnapshotFileName)
			if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
				t.Fatal(err)
			}
			setSpaceScanState(nil, false, 0, nil, "")

			if err := InitSpaceScanCache(dir); err != nil {
				t.Fatalf("坏快照不应让初始化失败: %v", err)
			}
			if got := cachedSpaceSnapshot(); got != nil {
				t.Fatalf("坏快照不应被采用，却拿到了 %+v", got)
			}
		})
	}
}

// TestLoadSpaceScanSnapshotRejectsTooOld：停机很久之后的快照已不能代表当前磁盘。
func TestLoadSpaceScanSnapshotRejectsTooOld(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, spaceScanSnapshotFileName)
	stale := testSpaceResult(time.Now().Add(-spaceScanSnapshotMaxAge-time.Hour), 100)
	saveSpaceScanSnapshot(path, &stale)

	if _, err := loadSpaceScanSnapshot(path); err == nil {
		t.Fatal("超过最长信任期的快照应被拒绝")
	}
}

// TestLoadSpaceScanSnapshotRejectsFutureTimestamp：系统时钟被改过时快照会一直
// 显得「新鲜」，必须判为不可信。
func TestLoadSpaceScanSnapshotRejectsFutureTimestamp(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, spaceScanSnapshotFileName)
	future := testSpaceResult(time.Now().Add(48*time.Hour), 100)
	saveSpaceScanSnapshot(path, &future)

	if _, err := loadSpaceScanSnapshot(path); err == nil {
		t.Fatal("时间戳落在未来的快照应被拒绝")
	}
}

// TestInvalidateSpaceScanCacheDropsSnapshotFile：硬失效必须连磁盘快照一起清掉，
// 否则重启后会复活一份已经不可信的数据。
func TestInvalidateSpaceScanCacheDropsSnapshotFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config", spaceScanSnapshotFileName)
	result := testSpaceResult(time.Now(), 100)
	saveSpaceScanSnapshot(path, &result)

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("快照文件应已写入: %v", err)
	}

	setSpaceScanState(&result, false, 0, nil, path)
	InvalidateSpaceScanCache()

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("硬失效后快照文件应被删除，stat 返回 %v", err)
	}
	if cache, needs, _, _, _ := snapshotSpaceScanState(); cache != nil || !needs {
		t.Fatalf("硬失效后内存应为空且标记待刷新，实际 cache=%v needs=%v", cache != nil, needs)
	}
}

// TestInvalidatedScanIsNotPublished 覆盖「扫描中发生清理」的竞态：
// 清理之前开始、清理之后结束的扫描结果不能被当成最新状态缓存下来。
func TestInvalidatedScanIsNotPublished(t *testing.T) {
	origExecute := spaceScanExecute
	origMounts := spaceScanMounts
	origCache, origNeeds, origGen, origInflight, origPath := snapshotSpaceScanState()
	t.Cleanup(func() {
		spaceScanExecute = origExecute
		spaceScanMounts = origMounts
		setSpaceScanState(origCache, origNeeds, origGen, origInflight, origPath)
	})

	entered := make(chan struct{})
	release := make(chan struct{})
	spaceScanExecute = func() (SpaceUsageResult, error) {
		close(entered)
		<-release
		return testSpaceResult(time.Now(), 4096), nil
	}
	spaceScanMounts = func() ([]string, mountOracle) { return []string{"/vol1"}, mountOracle{} }
	setSpaceScanState(nil, false, 0, nil, "")

	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = ScanSpaceUsage(true)
	}()

	<-entered
	InvalidateSpaceScanCache() // 扫描进行中，磁盘状态变了
	close(release)
	<-done

	if cache, _, _, _, _ := snapshotSpaceScanState(); cache != nil {
		t.Fatalf("被失效的扫描结果不应被发布，实际 cache=%+v", cache)
	}
}

// TestPruneSpaceSnapshotAfterClean：清理后快照要立刻反映「这些条目已经没了」，
// 否则面板会继续显示「还能释放 X」。
func TestPruneSpaceSnapshotAfterClean(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config", spaceScanSnapshotFileName)

	keep := SpaceItem{Path: "/vol1/logs/keep.log", Size: 300, Safe: true}
	drop := SpaceItem{Path: "/vol1/logs/drop.log", Size: 700, Safe: true}
	shown := SpaceItem{Path: "/vol1/docker/keep-img", Size: 500, Safe: false}
	result := SpaceUsageResult{
		ScannedAt: time.Now(),
		Categories: []SpaceCategory{
			{ID: "logs", Name: "日志", Items: []SpaceItem{keep, drop}},
			{ID: "docker", Name: "容器", Items: []SpaceItem{shown}},
		},
	}
	recomputeSpaceTotals(&result)
	setSpaceScanState(&result, false, 0, nil, path)

	pruneSpaceSnapshotAfterClean([]SpaceCleanItemResult{
		{Path: "/vol1/logs/drop.log", Status: "deleted"},
		{Path: "/vol1/logs/keep.log", Status: "skipped"}, // 没删掉的不该被剔除
	})

	cache, needs, _, _, _ := snapshotSpaceScanState()
	if cache == nil {
		t.Fatal("快照应保留而不是被清空")
	}
	if !needs {
		t.Fatal("剔除条目后应标记待刷新")
	}
	if cache.Categories[0].ItemCount != 1 {
		t.Fatalf("日志分类应只剩 1 项，实际 %d", cache.Categories[0].ItemCount)
	}
	if cache.Categories[0].Items[0].Path != keep.Path {
		t.Fatalf("应保留未被删除的条目，实际 %+v", cache.Categories[0].Items)
	}
	// 可回收只统计 Safe 条目：删掉 drop.log 后剩 keep.log 的 300。
	if cache.Reclaimable != 300 {
		t.Fatalf("可回收体积应重算为 300，实际 %d", cache.Reclaimable)
	}
	// 不可清理的展示项仍然计入总占用，但从不计入可回收。
	if cache.CategoriesWithItems != 2 {
		t.Fatalf("两个分类都还有条目，实际 %d", cache.CategoriesWithItems)
	}
}

// TestPruneSpaceSnapshotKeepsSharedSlice 守住一个微妙的别名问题：
// 已经发给前端的响应与快照共享 Items 的底层数组，剔除时必须重新分配。
func TestPruneSpaceSnapshotKeepsSharedSlice(t *testing.T) {
	items := []SpaceItem{{Path: "/vol1/a", Size: 1, Safe: true}, {Path: "/vol1/b", Size: 2, Safe: true}}
	result := SpaceUsageResult{
		ScannedAt:  time.Now(),
		Categories: []SpaceCategory{{ID: "logs", Name: "日志", Items: items}},
	}
	setSpaceScanState(&result, false, 0, nil, "")

	// 模拟一份已经发送出去的响应：它与快照共享底层数组。
	sent := result
	sent.Categories = append([]SpaceCategory{}, result.Categories...)

	pruneSpaceSnapshotAfterClean([]SpaceCleanItemResult{{Path: "/vol1/a", Status: "deleted"}})

	if len(sent.Categories[0].Items) != 2 {
		t.Fatalf("已发出的响应不应被就地改写，实际剩 %d 项", len(sent.Categories[0].Items))
	}
	cache, _, _, _, _ := snapshotSpaceScanState()
	if len(cache.Categories[0].Items) != 1 {
		t.Fatalf("快照应剔除 1 项，实际剩 %d 项", len(cache.Categories[0].Items))
	}
}

// TestPruneSpaceSnapshotIgnoresDryRun：试运行没有改动磁盘，快照不该被动。
func TestPruneSpaceSnapshotIgnoresDryRun(t *testing.T) {
	result := testSpaceResult(time.Now(), 100)
	setSpaceScanState(&result, false, 0, nil, "")

	// 试运行的结果里没有 deleted/recycled/truncated 状态，剔除应为空操作。
	pruneSpaceSnapshotAfterClean([]SpaceCleanItemResult{
		{Path: "/vol1/logs/a.log", Status: "skipped"},
	})

	_, needs, _, _, _ := snapshotSpaceScanState()
	if needs {
		t.Fatal("没有任何实际删除时不应标记待刷新")
	}
}

// TestSpaceScanVolumeSetChanged：插拔硬盘后旧快照的分类必然不完整。
func TestSpaceScanVolumeSetChanged(t *testing.T) {
	snapshot := []VolumeUsage{{Mount: "/"}, {Mount: "/vol1"}}
	if spaceScanVolumeSetChanged(snapshot, []string{"/vol1"}) {
		t.Fatal("存储卷集合未变化时不应判定为变化")
	}
	if !spaceScanVolumeSetChanged(snapshot, []string{"/vol1", "/vol2"}) {
		t.Fatal("新增存储卷应判定为变化")
	}
	if !spaceScanVolumeSetChanged(nil, []string{"/vol1"}) {
		t.Fatal("空快照应判定为变化")
	}
}

// TestSpaceScanViewKeepsVolumesOnProbeFailure：瞬时读不到挂载表时宁可显示旧容量，
// 也不能把磁盘整片抹掉。
func TestSpaceScanViewKeepsVolumesOnProbeFailure(t *testing.T) {
	result := testSpaceResult(time.Now(), 100)
	result.Volumes = []VolumeUsage{{Mount: "/vol1", Total: 1000, UsedFormatted: "1 KB"}}

	// vol 列表为空 → 不重算容量行，保留快照里的内容。
	view := spaceScanView(result, nil, mountOracle{}, true, false)
	if len(view.Volumes) != 1 || view.Volumes[0].Mount != "/vol1" {
		t.Fatalf("读不到挂载信息时应保留快照容量行，实际 %+v", view.Volumes)
	}
	if !view.Stale || view.Refreshing {
		t.Fatalf("时效标记应原样透传，实际 stale=%v refreshing=%v", view.Stale, view.Refreshing)
	}
}

// TestRecomputeSpaceTotalsCountsOnlySafeAsReclaimable 守住展示口径：
// Safe=false 的条目计入占用但绝不计入「可释放」。
func TestRecomputeSpaceTotalsCountsOnlySafeAsReclaimable(t *testing.T) {
	result := SpaceUsageResult{
		Categories: []SpaceCategory{
			{ID: "a", Items: []SpaceItem{{Path: "/vol1/a", Size: 100, Safe: true}, {Path: "/vol1/b", Size: 50, Safe: false}}},
			{ID: "b", Items: nil},
		},
	}
	recomputeSpaceTotals(&result)

	if result.Reclaimable != 100 {
		t.Fatalf("可回收应为 100（只算 Safe），实际 %d", result.Reclaimable)
	}
	if result.Categories[0].TotalSize != 150 {
		t.Fatalf("分类占用应为 150（含不可清理项），实际 %d", result.Categories[0].TotalSize)
	}
	if result.CategoriesWithItems != 1 {
		t.Fatalf("空分类不应计入有内容的分类数，实际 %d", result.CategoriesWithItems)
	}
	if result.Categories[1].Items == nil {
		t.Fatal("空分类的 Items 应为空切片而非 nil，避免前端拿到 null")
	}
	if result.ReclaimableText == "" {
		t.Fatal("应生成可读体积文本")
	}
}

// TestSpaceScanSnapshotJSONIsStable 锁住落盘格式：字段名变更会让旧快照静默失效。
func TestSpaceScanSnapshotJSONIsStable(t *testing.T) {
	body, err := json.Marshal(spaceScanSnapshot{Version: spaceScanSnapshotVersion, SavedAt: time.Now(), Result: testSpaceResult(time.Now(), 1)})
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"version", "savedAt", "result"} {
		if _, ok := decoded[key]; !ok {
			t.Fatalf("快照 JSON 缺少字段 %q", key)
		}
	}
}

// TestStartSpaceScanRefresherRequiresInit：未初始化时应给出明确错误而不是静默空转。
func TestStartSpaceScanRefresherRequiresInit(t *testing.T) {
	orig := globalSpaceScanRefresher
	t.Cleanup(func() { globalSpaceScanRefresher = orig })
	globalSpaceScanRefresher = nil

	if err := StartSpaceScanRefresher(); err == nil {
		t.Fatal("未初始化时启动后台刷新应报错")
	}
	// 停止未初始化的刷新器不应 panic。
	if err := StopSpaceScanRefresher(); err != nil {
		t.Fatalf("停止未初始化的刷新器不应报错: %v", err)
	}
}

// TestInitSpaceScanCacheIsIdempotentAndStoppable 覆盖启停配对：
// Init 之后 Start 必须可用，Stop 之后重复 Stop 不能 panic（close 两次会 panic）。
func TestInitSpaceScanCacheIsIdempotentAndStoppable(t *testing.T) {
	origRefresher := globalSpaceScanRefresher
	origCache, origNeeds, origGen, origInflight, origPath := snapshotSpaceScanState()
	t.Cleanup(func() {
		globalSpaceScanRefresher = origRefresher
		setSpaceScanState(origCache, origNeeds, origGen, origInflight, origPath)
	})

	if err := InitSpaceScanCache(t.TempDir()); err != nil {
		t.Fatalf("初始化失败: %v", err)
	}
	if err := StartSpaceScanRefresher(); err != nil {
		t.Fatalf("启动后台刷新失败: %v", err)
	}
	// 重复启动不应起第二个循环。
	if err := StartSpaceScanRefresher(); err != nil {
		t.Fatalf("重复启动应幂等: %v", err)
	}
	if err := StopSpaceScanRefresher(); err != nil {
		t.Fatalf("停止失败: %v", err)
	}
	if err := StopSpaceScanRefresher(); err != nil {
		t.Fatalf("重复停止应幂等: %v", err)
	}
}

// TestRepeatedInitKeepsSingleRefresher 覆盖一个容易漏掉的 goroutine 泄漏：
// 重复 Init 若直接替换 globalSpaceScanRefresher，先启动的循环就再也停不掉了
// ——Stop 只看得见新指针。这里断言 Init 不会换掉已有实例。
func TestRepeatedInitKeepsSingleRefresher(t *testing.T) {
	origRefresher := globalSpaceScanRefresher
	origCache, origNeeds, origGen, origInflight, origPath := snapshotSpaceScanState()
	t.Cleanup(func() {
		globalSpaceScanRefresher = origRefresher
		setSpaceScanState(origCache, origNeeds, origGen, origInflight, origPath)
	})
	globalSpaceScanRefresher = nil

	dir := t.TempDir()
	if err := InitSpaceScanCache(dir); err != nil {
		t.Fatalf("首次初始化失败: %v", err)
	}
	first := globalSpaceScanRefresher
	if first == nil {
		t.Fatal("首次初始化应装配 refresher")
	}

	if err := StartSpaceScanRefresher(); err != nil {
		t.Fatalf("启动失败: %v", err)
	}

	// 再初始化一次（模拟测试重复装配或未来热重载）。
	if err := InitSpaceScanCache(dir); err != nil {
		t.Fatalf("重复初始化失败: %v", err)
	}
	if globalSpaceScanRefresher != first {
		t.Fatal("重复初始化替换了 refresher，已启动的循环将无法被停止（goroutine 泄漏）")
	}

	// Stop 必须能停掉真正在跑的那个循环：关闭后 select 会立刻返回。
	if err := StopSpaceScanRefresher(); err != nil {
		t.Fatalf("停止失败: %v", err)
	}
	select {
	case <-first.stopCh:
	case <-time.After(time.Second):
		t.Fatal("停止后 stopCh 未被关闭")
	}
}

// TestRefreshSpaceScanIfStaleSkipsWithoutVolumes：没有存储卷时不该反复全盘扫描。
func TestRefreshSpaceScanIfStaleSkipsWithoutVolumes(t *testing.T) {
	origMounts := spaceScanMounts
	origCache, origNeeds, origGen, origInflight, origPath := snapshotSpaceScanState()
	t.Cleanup(func() {
		spaceScanMounts = origMounts
		setSpaceScanState(origCache, origNeeds, origGen, origInflight, origPath)
	})

	var calls int64
	origExecute := spaceScanExecute
	t.Cleanup(func() { spaceScanExecute = origExecute })
	spaceScanExecute = func() (SpaceUsageResult, error) {
		atomic.AddInt64(&calls, 1)
		return SpaceUsageResult{}, nil
	}
	spaceScanMounts = func() ([]string, mountOracle) { return nil, mountOracle{} }
	setSpaceScanState(nil, true, 0, nil, "")

	refreshSpaceScanIfStale()
	if n := atomic.LoadInt64(&calls); n != 0 {
		t.Fatalf("没有存储卷时不应扫描，实际 %d 次", n)
	}
}

// TestCachedSpaceSnapshotPrefersMemory：内存状态永远优先于磁盘，避免读到旧文件。
func TestCachedSpaceSnapshotPrefersMemory(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config", spaceScanSnapshotFileName)
	onDisk := testSpaceResult(time.Now(), 111)
	saveSpaceScanSnapshot(path, &onDisk)

	inMemory := testSpaceResult(time.Now(), 222)
	setSpaceScanState(&inMemory, false, 0, nil, path)

	got := cachedSpaceSnapshot()
	if got == nil || got.Reclaimable != 222 {
		t.Fatalf("应优先返回内存快照 222，实际 %+v", got)
	}
}

// TestCachedSpaceSnapshotReturnsCopy：返回副本，调用方改写不能污染缓存。
func TestCachedSpaceSnapshotReturnsCopy(t *testing.T) {
	inMemory := testSpaceResult(time.Now(), 100)
	setSpaceScanState(&inMemory, false, 0, nil, "")

	got := cachedSpaceSnapshot()
	if got == nil {
		t.Fatal("应返回快照")
	}
	got.Reclaimable = 99999
	got.Categories[0].Items = nil

	again := cachedSpaceSnapshot()
	if again.Reclaimable == 99999 || again.Categories[0].Items == nil {
		t.Fatal("缓存被调用方改写污染了")
	}
}

// waitFor 轮询等待条件成立，超时即失败——用于等待后台 goroutine 的产物。
func waitFor(t *testing.T, timeout time.Duration, cond func() bool, msg string) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatal(msg)
}
