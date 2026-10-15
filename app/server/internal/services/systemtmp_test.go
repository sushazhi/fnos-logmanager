package services

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// stubOpenPaths 用固定集合替换 /proc 枚举结果，使判定逻辑可在任意平台测试。
// 不传参数表示"确实没有任何进程持有文件"（空集合）。
func stubOpenPaths(t *testing.T, paths ...string) {
	t.Helper()
	set := make(map[string]bool, len(paths))
	for _, p := range paths {
		set[p] = true
	}
	prev := listOpenPaths
	listOpenPaths = func() map[string]bool { return set }
	resetOpenPathsCache()
	t.Cleanup(func() {
		listOpenPaths = prev
		resetOpenPathsCache()
	})
}

// stubOpenPathsUnavailable 模拟"读取进程信息失败"：listOpenPaths 返回 nil。
// 必须与空集合区分开——把"读不到"当成"没人用"正是会导致误删的那类错误。
func stubOpenPathsUnavailable(t *testing.T) {
	t.Helper()
	prev := listOpenPaths
	listOpenPaths = func() map[string]bool { return nil }
	resetOpenPathsCache()
	t.Cleanup(func() {
		listOpenPaths = prev
		resetOpenPathsCache()
	})
}

// stubSystemTempRoots 把系统临时根指向测试目录，避免测试触碰真实 /tmp。
func stubSystemTempRoots(t *testing.T, roots ...string) {
	t.Helper()
	prev := systemTempRoots
	systemTempRoots = roots
	t.Cleanup(func() { systemTempRoots = prev })
}

// makeTempEntry 在 root 下造一个条目，并把 mtime 设为 age 之前。
func makeTempEntry(t *testing.T, root, name string, isDir bool, age time.Duration) string {
	t.Helper()
	p := filepath.Join(root, name)
	if isDir {
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", p, err)
		}
		if err := os.WriteFile(filepath.Join(p, "f"), []byte("0123456789"), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
	} else {
		if err := os.WriteFile(p, []byte("hello"), 0o644); err != nil {
			t.Fatalf("write %s: %v", p, err)
		}
	}
	when := time.Now().Add(-age)
	if err := os.Chtimes(p, when, when); err != nil {
		t.Fatalf("chtimes: %v", err)
	}
	return p
}

// TestSystemTempRootForMatchesOnlyRealRoots 确认前缀判断不会把 /tmpfoo 当成
// /tmp 下的条目——否则一个同名前缀目录就能绕过整套判定。
func TestSystemTempRootForMatchesOnlyRealRoots(t *testing.T) {
	stubSystemTempRoots(t, "/tmp", "/var/tmp")
	yes := map[string]string{
		"/tmp":               "/tmp",
		"/tmp/a":             "/tmp",
		"/tmp/a/b":           "/tmp",
		"/var/tmp":           "/var/tmp",
		"/var/tmp/x":         "/var/tmp",
		"/var/tmpfoo":        "",
		"/tmpfoo":            "",
		"/tmpfoo/a":          "",
		"/var":               "",
		"/":                  "",
		"/vol1/tmp":          "",
		"/vol1/@apptemp/foo": "",
	}
	for p, want := range yes {
		if got := systemTempRootFor(p); got != want {
			t.Errorf("systemTempRootFor(%q) = %q, want %q", p, got, want)
		}
	}
}

// TestSystemTempEntryNameOnlyAcceptsDirectChildren 确认只接受一级子项：
// 根目录本身与更深层路径都不作为独立清理目标。
func TestSystemTempEntryNameOnlyAcceptsDirectChildren(t *testing.T) {
	stubSystemTempRoots(t, "/tmp", "/var/tmp")
	cases := []struct {
		path string
		want bool
	}{
		{"/tmp/stale", true},
		{"/tmp/f.bin", true},
		{"/tmp", false},
		{"/tmp/", false},
		{"/tmp/a/b", false},
		{"/var/tmp/old", true},
		{"/var/tmp", false},
		{"/var/tmp/a/b/c", false},
		{"/elsewhere/x", false},
	}
	for _, c := range cases {
		_, name, ok := systemTempEntryName(c.path)
		if ok != c.want {
			t.Errorf("systemTempEntryName(%q) ok = %v, want %v", c.path, ok, c.want)
		}
		if ok && name == "" {
			t.Errorf("systemTempEntryName(%q) returned empty name", c.path)
		}
	}
}

// TestSystemTempEntryDeletable 是这套逻辑的核心回归：只有同时满足
// 「无人使用」和「闲置足够久」才允许删除。
func TestSystemTempEntryDeletable(t *testing.T) {
	root := t.TempDir()
	stubSystemTempRoots(t, root)

	old := makeTempEntry(t, root, "old", false, 48*time.Hour)
	fresh := makeTempEntry(t, root, "fresh", false, time.Minute)
	oldDir := makeTempEntry(t, root, "olddir", true, 48*time.Hour)

	t.Run("闲置且无人使用可删", func(t *testing.T) {
		stubOpenPaths(t)
		if !systemTempEntryDeletable(old) {
			t.Errorf("闲置 48 小时的条目应可删除，原因: %s", systemTempBlockReason(old))
		}
		if !systemTempEntryDeletable(oldDir) {
			t.Errorf("闲置 48 小时的目录应可删除，原因: %s", systemTempBlockReason(oldDir))
		}
	})

	t.Run("刚被修改不可删", func(t *testing.T) {
		stubOpenPaths(t)
		if systemTempEntryDeletable(fresh) {
			t.Error("1 分钟前修改的条目不应可删：安装/解压过程会在步骤间短暂关闭文件")
		}
	})

	t.Run("自身被占用不可删", func(t *testing.T) {
		stubOpenPaths(t, old)
		if systemTempEntryDeletable(old) {
			t.Error("被进程直接打开的条目不应可删")
		}
	})

	t.Run("内部文件被占用不可删", func(t *testing.T) {
		stubOpenPaths(t, filepath.Join(oldDir, "f"))
		if systemTempEntryDeletable(oldDir) {
			t.Error("内部文件被打开的目录不应可删")
		}
	})

	t.Run("父目录被占用不影响子项", func(t *testing.T) {
		stubOpenPaths(t, root)
		if !systemTempEntryDeletable(old) {
			t.Error("父目录被占用不应阻止删除子项：删除子项不影响父目录的使用者")
		}
	})

	t.Run("无法判定时一律不可删", func(t *testing.T) {
		stubOpenPathsUnavailable(t)
		if systemTempEntryDeletable(old) {
			t.Error("无法确认占用情况时必须拒绝删除（fail-closed）")
		}
		if reason := systemTempBlockReason(old); !strings.Contains(reason, "无法确认") {
			t.Errorf("原因应说明无法确认占用情况，实际: %q", reason)
		}
	})
}

// TestMatchSystemTempForPathRejects 确认清理准入对系统临时目录同样严格。
func TestMatchSystemTempForPathRejects(t *testing.T) {
	root := t.TempDir()
	stubSystemTempRoots(t, root)
	old := makeTempEntry(t, root, "old", false, 48*time.Hour)
	oldDir := makeTempEntry(t, root, "olddir", true, 48*time.Hour)
	fresh := makeTempEntry(t, root, "fresh", false, time.Minute)

	t.Run("合法条目放行", func(t *testing.T) {
		stubOpenPaths(t)
		for _, p := range []string{old, oldDir} {
			rule, ok := matchSystemTempForPath(p)
			if !ok {
				t.Fatalf("matchSystemTempForPath(%q) 应放行", p)
			}
			if rule.Strategy != "delete" {
				t.Errorf("策略 = %q, want delete", rule.Strategy)
			}
			if rule.Category != "system-temp" {
				t.Errorf("分类 = %q, want system-temp", rule.Category)
			}
		}
	})

	rejected := []struct {
		name string
		path string
	}{
		{"根目录本身", root},
		{"更深的路径", filepath.Join(oldDir, "f")},
		{"规则之外的路径", "/vol1/1000/pt"},
		{"系统路径", "/etc/passwd"},
		{"刚修改的条目", fresh},
	}
	for _, c := range rejected {
		t.Run(c.name, func(t *testing.T) {
			stubOpenPaths(t)
			if _, ok := matchSystemTempForPath(c.path); ok {
				t.Errorf("matchSystemTempForPath(%q) 应拒绝", c.path)
			}
		})
	}

	t.Run("被占用时拒绝", func(t *testing.T) {
		stubOpenPaths(t, old)
		if _, ok := matchSystemTempForPath(old); ok {
			t.Error("被占用的条目必须拒绝：扫描到清理之间进程可能已开始使用")
		}
	})

	t.Run("无法判定时拒绝", func(t *testing.T) {
		stubOpenPathsUnavailable(t)
		if _, ok := matchSystemTempForPath(old); ok {
			t.Error("无法读取 /proc 时必须拒绝删除")
		}
	})
}

// TestMatchCleanablePathCoversBothRuleFamilies 确认统一准入同时覆盖两套规则，
// 且规则表之外的路径仍然被拒绝。
func TestMatchCleanablePathCoversBothRuleFamilies(t *testing.T) {
	root := t.TempDir()
	stubSystemTempRoots(t, root)
	stubOpenPaths(t)
	old := makeTempEntry(t, root, "old", false, 48*time.Hour)

	if _, ok := matchCleanablePath(old); !ok {
		t.Errorf("系统临时条目 %q 应可通过统一准入", old)
	}
	if _, ok := matchCleanablePath("/vol1/@appdata/x/pnpm-home/store"); !ok {
		t.Error("存储卷缓存规则应仍可通过统一准入")
	}
	for _, p := range []string{"/vol1/1000/pt", "/etc/passwd", "/vol1/@appdata/moviepilot/config/.browser"} {
		if _, ok := matchCleanablePath(p); ok {
			t.Errorf("matchCleanablePath(%q) 应拒绝", p)
		}
	}
}

// TestBuildSystemTempItemMarksInUse 确认不可删的条目仍会出现在结果里并带上原因，
// 否则用户看不到"系统分区被什么占着"。
func TestBuildSystemTempItemMarksInUse(t *testing.T) {
	root := t.TempDir()
	stubSystemTempRoots(t, root)

	old := makeTempEntry(t, root, "old", true, 48*time.Hour)
	stubOpenPaths(t)
	item := buildSystemTempItem(old)
	if !item.Safe || item.InUse {
		t.Errorf("闲置条目应为安全且未占用，实际 safe=%v inUse=%v", item.Safe, item.InUse)
	}
	if item.Size <= 0 || item.FileCount <= 0 {
		t.Errorf("应统计出体积与文件数，实际 size=%d files=%d", item.Size, item.FileCount)
	}
	if !item.IsDir {
		t.Error("目录条目 IsDir 应为 true")
	}

	stubOpenPaths(t, filepath.Join(old, "f"))
	item = buildSystemTempItem(old)
	if item.Safe || !item.InUse {
		t.Errorf("被占用条目应为不安全且标记占用，实际 safe=%v inUse=%v", item.Safe, item.InUse)
	}
	if item.Reason == "" {
		t.Error("不可删条目必须给出原因，供界面显示")
	}
	if item.Size == 0 {
		t.Error("不可删条目也应显示占用大小")
	}
}

// stubDeviceOf 把设备号查询替换成按路径前缀查表，用于在任意平台上
// 构造"同分区 / 不同分区 / 读不到"三种情形。
func stubDeviceOf(t *testing.T, table map[string]uint64, failFor map[string]bool) {
	t.Helper()
	orig := systemDeviceOf
	systemDeviceOf = func(path string) (uint64, error) {
		if failFor[path] {
			return 0, os.ErrPermission
		}
		if dev, ok := table[path]; ok {
			return dev, nil
		}
		return 0, os.ErrNotExist
	}
	t.Cleanup(func() { systemDeviceOf = orig })
}

// TestSystemTempOnSystemDiskOutcomes 用注入的设备号把三种结论都锁住。
//
// 这三种结论对应三种不同的用户可见文案，因此不能在开发机上只验证其中一种：
//   - 同分区   → 提示"/tmp 与 /var/tmp 位于此处"
//   - 不同分区 → 必须说明两者不在该分区，否则用户会算错可回收空间
//   - 无法判定 → 什么都不说，绝不能默认成同分区
func TestSystemTempOnSystemDiskOutcomes(t *testing.T) {
	root := t.TempDir()
	a := filepath.Join(root, "tmp")
	b := filepath.Join(root, "vartmp")
	for _, d := range []string{a, b} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	t.Run("两个根都在系统分区", func(t *testing.T) {
		stubSystemTempRoots(t, a, b)
		stubDeviceOf(t, map[string]uint64{"/": 100, a: 100, b: 100}, nil)
		same, known := systemTempOnSystemDisk()
		if !known || !same {
			t.Errorf("应判定为同分区，实际 same=%v known=%v", same, known)
		}
	})

	t.Run("其中一个根不在系统分区", func(t *testing.T) {
		stubSystemTempRoots(t, a, b)
		stubDeviceOf(t, map[string]uint64{"/": 100, a: 100, b: 200}, nil)
		same, known := systemTempOnSystemDisk()
		if !known || same {
			t.Errorf("应判定为不同分区，实际 same=%v known=%v", same, known)
		}
	})

	t.Run("系统分区设备号读不到", func(t *testing.T) {
		stubSystemTempRoots(t, a, b)
		stubDeviceOf(t, map[string]uint64{a: 100, b: 100}, map[string]bool{"/": true})
		same, known := systemTempOnSystemDisk()
		if known {
			t.Errorf("读不到系统分区设备号时必须无法判定，实际 same=%v known=%v", same, known)
		}
		if same {
			t.Error("无法判定时 same 必须为 false")
		}
	})

	t.Run("某个根的设备号读不到", func(t *testing.T) {
		stubSystemTempRoots(t, a, b)
		stubDeviceOf(t, map[string]uint64{"/": 100, a: 100}, map[string]bool{b: true})
		same, known := systemTempOnSystemDisk()
		if known {
			t.Errorf("某个根读不到设备号时必须无法判定，实际 same=%v known=%v", same, known)
		}
		if same {
			t.Error("无法判定时 same 必须为 false")
		}
	})
}

// TestSystemTempOnSystemDiskFailsClosed 覆盖"无法判定"与"判定为不同文件系统"
// 这两种结果不能被混淆：
//   - 读不到设备号时必须 known=false，前端据此不显示任何断言；
//   - 能读到设备号时结论必须确定，known=true。
//
// 这条区分很关键：把"未知"当成"同分区"会向用户断言 /tmp 占用着系统分区，
// 而 /tmp 完全可能是 tmpfs（吃内存不吃磁盘），那是一条错误结论。
func TestSystemTempOnSystemDiskFailsClosed(t *testing.T) {
	root := t.TempDir()
	stubSystemTempRoots(t, root)

	// 系统分区的设备号必须可读，函数才会真正走到比较逻辑；
	// 根目录用真实存在的临时目录，从而在 Windows 上也能执行到"读到根设备号"。
	stubDeviceOf(t, map[string]uint64{"/": 100, root: 200}, nil)

	same, known := systemTempOnSystemDisk()
	if !known {
		t.Fatal("设备号可读时应能给出结论")
	}
	if same {
		t.Error("设备号不同必须判定为不同分区，不能默认成同分区")
	}
}

// TestSystemTempOnSystemDiskUnknownWhenRootsMissing 确认所有临时根都不存在时
// 返回"无法判定"而不是"同分区"：没有可比较的对象就不该给出结论。
//
// 必须把系统分区的设备号 stub 成可读：否则在 Windows 上 deviceOf("/") 会先
// 返回错误，函数在读根目录之前就提前返回"无法判定"，这条断言会因为错误的原因
// 通过，而 roots == 0 这道闸门从未被真正执行到（变异测试会漏掉它）。
func TestSystemTempOnSystemDiskUnknownWhenRootsMissing(t *testing.T) {
	stubSystemTempRoots(t, filepath.Join(t.TempDir(), "definitely-missing"))
	stubDeviceOf(t, map[string]uint64{"/": 100}, nil)

	same, known := systemTempOnSystemDisk()
	if known {
		t.Errorf("根目录不存在时应无法判定，实际 same=%v known=%v", same, known)
	}
	if same {
		t.Error("无法判定时 same 必须为 false")
	}
}

// TestSystemTempEntryScannableRejectsSymlinks 覆盖符号链接的排除规则。
//
// 直接测判定函数而不是真造符号链接：Windows 上创建符号链接需要额外权限
// （SeCreateSymbolicLinkPrivilege），会让这条安全规则在开发机上永远被跳过。
// os.DirEntry 是接口，用 fakeDirEntry 就能构造出任意 Type()。
func TestSystemTempEntryScannableRejectsSymlinks(t *testing.T) {
	cases := []struct {
		name  string
		entry os.DirEntry
		want  bool
	}{
		{"普通文件", fakeTypeEntry{name: "a.log", mode: 0}, true},
		{"目录", fakeTypeEntry{name: "build", mode: os.ModeDir}, true},
		{"符号链接", fakeTypeEntry{name: "link", mode: os.ModeSymlink}, false},
		{"符号链接指向目录", fakeTypeEntry{name: "linkdir", mode: os.ModeSymlink | os.ModeDir}, false},
		{"空名字", fakeTypeEntry{name: "", mode: 0}, false},
		{"当前目录", fakeTypeEntry{name: ".", mode: 0}, false},
		{"上级目录", fakeTypeEntry{name: "..", mode: 0}, false},
	}
	for _, c := range cases {
		if got := systemTempEntryScannable(c.entry); got != c.want {
			t.Errorf("%s: systemTempEntryScannable = %v, want %v", c.name, got, c.want)
		}
	}
}

// fakeTypeEntry 实现 os.DirEntry，可自由指定 Type()，用于测试类型判定。
type fakeTypeEntry struct {
	name string
	mode os.FileMode
}

func (f fakeTypeEntry) Name() string               { return f.name }
func (f fakeTypeEntry) IsDir() bool                { return f.mode&os.ModeDir != 0 }
func (f fakeTypeEntry) Type() os.FileMode          { return f.mode }
func (f fakeTypeEntry) Info() (os.FileInfo, error) { return nil, os.ErrInvalid }

// TestScanSystemTempItemsSkipsSymlinks 在能创建符号链接的平台上补一条端到端验证；
// 权限不足时跳过，核心规则已由 TestSystemTempEntryScannableRejectsSymlinks 覆盖。
func TestScanSystemTempItemsSkipsSymlinks(t *testing.T) {
	root := t.TempDir()
	stubSystemTempRoots(t, root)
	stubOpenPaths(t)

	real := makeTempEntry(t, root, "real", false, 48*time.Hour)
	if err := os.Symlink(real, filepath.Join(root, "link")); err != nil {
		t.Skipf("无法创建符号链接: %v", err)
	}

	items := scanSystemTempItems()
	for _, it := range items {
		if filepath.Base(it.Path) == "link" {
			t.Error("符号链接不应作为清理目标")
		}
	}
	if len(items) != 1 {
		t.Errorf("应只扫描到 1 个条目，实际 %d 个", len(items))
	}
}

// TestScanSystemTempItemsSortsBySizeDesc 确认结果按占用降序，
// 让用户第一眼看到最占空间的条目。
func TestScanSystemTempItemsSortsBySizeDesc(t *testing.T) {
	root := t.TempDir()
	stubSystemTempRoots(t, root)
	stubOpenPaths(t)

	// big 内容 100 字节、small 内容 5 字节，均设为闲置。
	big := filepath.Join(root, "big")
	if err := os.MkdirAll(big, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(big, "f"), make([]byte, 100), 0o644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(big, old, old); err != nil {
		t.Fatal(err)
	}
	makeTempEntry(t, root, "small", false, 48*time.Hour)

	items := scanSystemTempItems()
	if len(items) != 2 {
		t.Fatalf("应扫描到 2 个条目，实际 %d", len(items))
	}
	if items[0].Size < items[1].Size {
		t.Errorf("结果应按占用降序，实际 %d 在 %d 之前", items[0].Size, items[1].Size)
	}
}

// TestScanSystemTempItemsSkipsEmptyFiles 确认 0 字节文件不列出：
// 它们释放不了空间，列出来只是噪音。空目录仍然保留，
// 因为界面需要显示"这里有个空目录"以便用户理解分区占用构成。
func TestScanSystemTempItemsSkipsEmptyFiles(t *testing.T) {
	root := t.TempDir()
	stubSystemTempRoots(t, root)
	stubOpenPaths(t)

	old := time.Now().Add(-48 * time.Hour)
	emptyFile := filepath.Join(root, "empty")
	if err := os.WriteFile(emptyFile, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(emptyFile, old, old); err != nil {
		t.Fatal(err)
	}
	emptyDir := filepath.Join(root, "emptydir")
	if err := os.MkdirAll(emptyDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(emptyDir, old, old); err != nil {
		t.Fatal(err)
	}

	names := map[string]bool{}
	for _, it := range scanSystemTempItems() {
		names[filepath.Base(it.Path)] = true
	}
	if names["empty"] {
		t.Error("0 字节文件不应列出")
	}
	if !names["emptydir"] {
		t.Error("空目录仍应列出，供用户了解占用构成")
	}
}

// TestSystemTempItemsRollIntoCategory 确认系统临时条目会进入分类统计，
// 且只有标记为安全的条目计入可释放空间。
func TestSystemTempItemsRollIntoCategory(t *testing.T) {
	root := t.TempDir()
	stubSystemTempRoots(t, root)

	// 三个条目覆盖三种状态：可清理、被占用、刚修改。
	// 用返回的路径作为 key 断言，避免依赖 basename 的巧合。
	makeTempEntry(t, root, "idle", true, 48*time.Hour)
	busy := makeTempEntry(t, root, "busy", true, 48*time.Hour)
	makeTempEntry(t, root, "fresh", false, time.Minute)
	// busy 内部文件被占用。
	stubOpenPaths(t, filepath.Join(busy, "f"))

	cats, _, _ := scanSpaceCategories(nil)
	var found *SpaceCategory
	for i := range cats {
		if cats[i].ID == "system-temp" {
			found = &cats[i]
		}
	}
	if found == nil {
		t.Fatal("扫描结果应包含 system-temp 分类")
	}
	if found.ItemCount != 3 {
		t.Errorf("应列出 3 个条目（含不可删的），实际 %d", found.ItemCount)
	}
	if found.TotalSize <= 0 {
		t.Error("应统计出总占用")
	}
	// 可释放空间的精确值：三个条目里只有 idle 是安全的。
	// 用等号而非大小比较，才能真正锁住"被占用/刚修改的条目不计入"这一点。
	sizeOf := map[string]int64{}
	for _, it := range found.Items {
		sizeOf[filepath.Base(it.Path)] = it.Size
	}
	if sizeOf["idle"] == 0 {
		t.Fatal("结果中应包含 idle 条目")
	}
	if found.Reclaimable != sizeOf["idle"] {
		t.Errorf("可释放空间应恰为 idle 的 %d 字节，实际 %d（busy/fresh 不应计入）",
			sizeOf["idle"], found.Reclaimable)
	}
	if found.TotalSize != sizeOf["idle"]+sizeOf["busy"]+sizeOf["fresh"] {
		t.Errorf("总占用应为三项之和 %d，实际 %d",
			sizeOf["idle"]+sizeOf["busy"]+sizeOf["fresh"], found.TotalSize)
	}

	var sawBusy, sawIdle bool
	for _, it := range found.Items {
		switch filepath.Base(it.Path) {
		case "busy":
			sawBusy = true
			if it.Safe || !it.InUse {
				t.Errorf("被占用条目应 safe=false inUse=true，实际 safe=%v inUse=%v", it.Safe, it.InUse)
			}
		case "idle":
			sawIdle = true
			if !it.Safe || it.InUse {
				t.Errorf("闲置条目应 safe=true inUse=false，实际 safe=%v inUse=%v", it.Safe, it.InUse)
			}
		}
	}
	if !sawBusy || !sawIdle {
		t.Error("被占用与闲置的条目都应出现在结果中")
	}
}

// TestSystemTempCategoryRegistered 确认新分类已登记元数据，
// 否则扫描结果会在聚合阶段被静默丢弃。
func TestSystemTempCategoryRegistered(t *testing.T) {
	def, ok := spaceCategoryDefs["system-temp"]
	if !ok {
		t.Fatal("system-temp 分类缺少元数据定义")
	}
	if def.Name == "" || def.Description == "" || def.Risk == "" || def.Icon == "" {
		t.Errorf("system-temp 分类元数据不完整: %+v", def)
	}
	found := false
	for _, id := range spaceCategoryOrder {
		if id == "system-temp" {
			found = true
		}
	}
	if !found {
		t.Error("system-temp 未出现在 spaceCategoryOrder 中，前端不会渲染该分类")
	}
}

// TestStripDeletedSuffix 确认内核的 "(deleted)" 后缀被去除，
// 使「进程仍持有已删除文件的 fd」也能被识别为该路径仍在使用。
func TestStripDeletedSuffix(t *testing.T) {
	if got := stripDeletedSuffix("/tmp/x (deleted)"); got != "/tmp/x" {
		t.Errorf("stripDeletedSuffix = %q, want /tmp/x", got)
	}
	if got := stripDeletedSuffix("/tmp/y"); got != "/tmp/y" {
		t.Errorf("stripDeletedSuffix = %q, want /tmp/y", got)
	}
}
