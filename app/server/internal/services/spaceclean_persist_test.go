package services

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// 本文件覆盖「跨进程重启」这一最关键、也最难靠单测发现的问题：
// 用户抱怨的场景里，「升级应用后重开面板」本来就要等一次全量扫描。
//
// 这里不再注入 spaceScanExecute，而是走真实的磁盘路径：写入一份快照、彻底丢弃
// 内存状态、再让 InitSpaceScanCache 从磁盘恢复。这样验证的是「字段有没有落盘」
// 而不只是「内存里对不对」——JSON tag 写错、字段未导出、指针被忽略这类问题只有
// 真正序列化一次才会暴露。

// TestSnapshotSurvivesProcessRestart 模拟应用重启：内存全丢，只有磁盘文件还在。
func TestSnapshotSurvivesProcessRestart(t *testing.T) {
	dataDir := t.TempDir()

	result := SpaceUsageResult{
		ScannedAt:   time.Now().Add(-2 * time.Minute),
		DurationMs:  8421,
		Reclaimable: 12345678,
		Categories: []SpaceCategory{{
			ID: "appcache", Name: "应用运行缓存", Reclaimable: 12345678, ItemCount: 2,
			Items: []SpaceItem{
				{Path: "/vol1/@appdata/foo/cache", Size: 12000000, Safe: true, FileCount: 42, Reason: "可再生缓存"},
				{Path: "/vol1/@appdata/bar/cache", Size: 345678, Safe: true, FileCount: 7, Reason: "可再生缓存"},
			},
		}},
		Notices: []SpaceNotice{{Path: "/vol1/@appdata/baz/chromium", Name: "Chromium 本体", Size: 700000000, Reason: "不是缓存"}},
		// Volumes 必须覆盖注入点返回的 /vol1：否则 spaceScanVolumeSetChanged 会
		// 判定「存储卷集合变了」而正确地触发重扫，这里就测不到「纯复用快照」了。
		Volumes: []VolumeUsage{{Mount: "/vol1", Label: "存储空间1", Kind: "storage", Total: 100, Used: 60, UsedFormatted: "60 B"}},
		Errors:  []string{"某个目录读不到"},
	}
	recomputeSpaceTotals(&result)

	// 第一次启动：服务端扫描后落盘。
	if err := InitSpaceScanCache(dataDir); err != nil {
		t.Fatalf("初始化失败: %v", err)
	}
	path := filepath.Join(dataDir, "config", spaceScanSnapshotFileName)
	saveSpaceScanSnapshot(path, &result)

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("快照未落盘: %v", err)
	}

	// 进程重启：内存里什么都不剩。
	setSpaceScanState(nil, false, 0, nil, "")

	// 第二次启动：从磁盘恢复，且不应触发任何扫描。
	var scanCalls int64
	origExecute := spaceScanExecute
	origMounts := spaceScanMounts
	origRefresher := globalSpaceScanRefresher
	origCache, origNeeds, origGen, origInflight, origPath := snapshotSpaceScanState()
	t.Cleanup(func() {
		spaceScanExecute = origExecute
		spaceScanMounts = origMounts
		globalSpaceScanRefresher = origRefresher
		setSpaceScanState(origCache, origNeeds, origGen, origInflight, origPath)
	})
	spaceScanExecute = func() (SpaceUsageResult, error) {
		scanCalls++
		t.Error("重启后应直接复用磁盘快照，不该重新全量扫描")
		return SpaceUsageResult{}, nil
	}
	spaceScanMounts = func() ([]string, mountOracle) { return []string{"/vol1"}, mountOracle{} }

	if err := InitSpaceScanCache(dataDir); err != nil {
		t.Fatalf("重启后初始化失败: %v", err)
	}

	got, err := ScanSpaceUsage(false)
	if err != nil {
		t.Fatalf("重启后读取失败: %v", err)
	}
	if scanCalls != 0 {
		t.Fatalf("不应触发扫描，实际 %d 次", scanCalls)
	}

	// 逐层核对：任何一层丢字段都说明只测了「内存里对」而没测「真的存下来了」。
	if got.Reclaimable != 12345678 {
		t.Fatalf("可回收体积丢失: %d", got.Reclaimable)
	}
	if got.DurationMs != 8421 {
		t.Fatalf("扫描耗时应保留原值 8421，实际 %d", got.DurationMs)
	}
	if len(got.Categories) != 1 || got.Categories[0].ItemCount != 2 {
		t.Fatalf("分类丢失: %+v", got.Categories)
	}
	items := got.Categories[0].Items
	if len(items) != 2 {
		t.Fatalf("条目丢失: %+v", items)
	}
	if items[0].Path != "/vol1/@appdata/foo/cache" || items[0].Size != 12000000 || !items[0].Safe {
		t.Fatalf("条目字段丢失: %+v", items[0])
	}
	if items[0].FileCount != 42 || items[0].Reason != "可再生缓存" {
		t.Fatalf("条目附加字段丢失: %+v", items[0])
	}
	if len(got.Notices) != 1 || got.Notices[0].Size != 700000000 {
		t.Fatalf("提示项丢失: %+v", got.Notices)
	}
	if len(got.Errors) != 1 {
		t.Fatalf("错误列表丢失: %+v", got.Errors)
	}
	// 落盘的 Volumes 来自上次扫描，但对外服务时会按当前挂载表重算，
	// 所以这里允许它不原样返回；至少不能是 nil 崩掉。
	_ = got.Volumes

	// 时效字段必须由服务端算，且这份 2 分钟前的数据仍在新鲜期内。
	if got.CacheAgeSeconds < 60 || got.CacheAgeSeconds > 300 {
		t.Fatalf("缓存年龄应在 60~300 秒，实际 %d", got.CacheAgeSeconds)
	}
	if got.Stale {
		// 2 分钟 < 5 分钟新鲜期：仍在期内，不该提示用户「数据可能过期」。
		t.Fatal("2 分钟前的数据仍在新鲜期内，不应标记 stale")
	}
}

// TestRepeatedRestartKeepsServingFromDisk 覆盖「反复重启」：
// 每次重启都应继续命中快照，而不是因为某处把文件删了而退回全量扫描。
func TestRepeatedRestartKeepsServingFromDisk(t *testing.T) {
	dataDir := t.TempDir()
	result := testSpaceResult(time.Now().Add(-time.Minute), 4242)

	if err := InitSpaceScanCache(dataDir); err != nil {
		t.Fatal(err)
	}
	saveSpaceScanSnapshot(filepath.Join(dataDir, "config", spaceScanSnapshotFileName), &result)

	origExecute := spaceScanExecute
	origMounts := spaceScanMounts
	origRefresher := globalSpaceScanRefresher
	origCache, origNeeds, origGen, origInflight, origPath := snapshotSpaceScanState()
	t.Cleanup(func() {
		spaceScanExecute = origExecute
		spaceScanMounts = origMounts
		globalSpaceScanRefresher = origRefresher
		setSpaceScanState(origCache, origNeeds, origGen, origInflight, origPath)
	})
	spaceScanExecute = func() (SpaceUsageResult, error) {
		t.Error("重启后不应全量扫描")
		return SpaceUsageResult{}, nil
	}
	spaceScanMounts = func() ([]string, mountOracle) { return []string{"/vol1"}, mountOracle{} }

	for i := 0; i < 3; i++ {
		setSpaceScanState(nil, false, 0, nil, "")
		if err := InitSpaceScanCache(dataDir); err != nil {
			t.Fatalf("第 %d 次重启初始化失败: %v", i+1, err)
		}
		got, err := ScanSpaceUsage(false)
		if err != nil {
			t.Fatalf("第 %d 次重启读取失败: %v", i+1, err)
		}
		if got.Reclaimable != 4242 {
			t.Fatalf("第 %d 次重启数据不对: %d", i+1, got.Reclaimable)
		}
	}
}

// TestSnapshotFilePermissions 锁定落盘权限：快照含完整路径清单，
// 不该对同机其他用户可读。
func TestSnapshotFilePermissions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config", spaceScanSnapshotFileName)
	result := testSpaceResult(time.Now(), 1)
	saveSpaceScanSnapshot(path, &result)

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	// Windows 不落实 Unix 权限位（os.Stat 恒返回 0666/0777），断言只对目标平台有意义。
	// 真机是 Linux，因此这里在 Linux 上严格校验、其他平台跳过而不是放宽断言——
	// 放宽会让这条防护在真机上悄悄失效。
	if runtime.GOOS != "windows" {
		if perm := info.Mode().Perm(); perm != 0o600 {
			t.Fatalf("快照文件权限应为 0600，实际 %o", perm)
		}
		dirInfo, err := os.Stat(filepath.Dir(path))
		if err != nil {
			t.Fatal(err)
		}
		if perm := dirInfo.Mode().Perm(); perm != 0o700 {
			t.Fatalf("快照目录权限应为 0700，实际 %o", perm)
		}
	}
}

// TestSaveSpaceScanSnapshotLeavesNoTempFiles 确认原子写不会留下残骸：
// 升级/重启反复发生，临时文件堆积会污染 config 目录。
func TestSaveSpaceScanSnapshotLeavesNoTempFiles(t *testing.T) {
	dir := t.TempDir()
	configDir := filepath.Join(dir, "config")
	path := filepath.Join(configDir, spaceScanSnapshotFileName)

	for i := 0; i < 3; i++ {
		result := testSpaceResult(time.Now(), int64(i))
		saveSpaceScanSnapshot(path, &result)
	}

	entries, err := os.ReadDir(configDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Fatalf("config 目录应只剩快照文件，实际 %v", names)
	}
}

// TestSaveSpaceScanSnapshotSurvivesUnwritablePath：写盘失败不能让请求失败，
// 只是失去持久化能力（内存缓存仍然有效）。
func TestSaveSpaceScanSnapshotSurvivesUnwritablePath(t *testing.T) {
	// 用一个不可能创建的路径：父级是普通文件。
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	result := testSpaceResult(time.Now(), 1)
	// 不应 panic，也不应返回错误（函数签名无返回值）。
	saveSpaceScanSnapshot(filepath.Join(blocker, "config", spaceScanSnapshotFileName), &result)
}
