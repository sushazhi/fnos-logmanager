package services

import (
	"os"
	"path"
	"testing"
)

// 这一组测试守住用户在真机上看到的那条缺陷：磁盘与分区使用情况里多出了
// /vol00、/vol02 两行。
//
// 真机（fnOS OECT，aarch64）实测事实，测试数据即照此构造：
//   - `storage overview` 只报告一个挂载点 /vol1（storId 1）；
//   - `storage removable` 报告外接盘 sdb1，path = vol00/WDC WD10SDRW-34A0XS0；
//   - `file prop` 证实 /vol00 与 /vol02 都存在（普通目录），/vol0、/vol01、/vol03
//     不存在；
//   - 内核把带空格的挂载点写成 /vol00/WDC\040WD10SDRW-34A0XS0。
const fnosMountInfoFixture = `23 28 0:22 / / rw,relatime shared:1 - btrfs /dev/mmcblk0p2 rw,ssd
24 23 0:24 / /boot rw,relatime shared:2 - ext4 /dev/mmcblk0p1 rw
30 23 0:25 / /tmp rw,relatime shared:3 - tmpfs tmpfs rw
41 23 179:0 / /vol1 rw,relatime shared:10 - ext4 /dev/dm-0 rw
52 23 8:17 / /vol00/WDC\040WD10SDRW-34A0XS0 rw,relatime shared:20 - exfat /dev/sdb1 rw
`

// stubMountTable 用一个固定的挂载表驱动挂载点判定。
func stubMountTable(t *testing.T, data string) {
	t.Helper()
	orig := mountTableFn
	mountTableFn = func() (string, error) { return data, nil }
	t.Cleanup(func() { mountTableFn = orig })
}

// stubMountDeviceOf 固定设备号，让 Windows 上也能走到「挂载表不可用」的回退分支。
//
// 注意：它替换的是 systemDeviceOf（与 systemtmp 共用同一个注入点），因此桩的键必须
// 与 isMount 实际查询的字符串逐字一致。
func stubMountDeviceOf(t *testing.T, table map[string]uint64) {
	t.Helper()
	orig := systemDeviceOf
	systemDeviceOf = func(p string) (uint64, error) {
		if dev, ok := table[p]; ok {
			return dev, nil
		}
		return 0, errDeviceInfoUnavailable
	}
	t.Cleanup(func() { systemDeviceOf = orig })
}

func TestUnescapeMountField(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"/vol1", "/vol1"},
		{`/vol00/WDC\040WD10SDRW-34A0XS0`, "/vol00/WDC WD10SDRW-34A0XS0"},
		{`/mnt/a\011b`, "/mnt/a\tb"},
		{`/mnt/a\012b`, "/mnt/a\nb"},
		{`/mnt/a\134b`, `/mnt/a\b`},
		// 不完整的转义原样保留：宁可路径对不上挂载表（退化为「保留候选」），
		// 也不能把字节改坏后指向别的路径。
		{`/mnt/a\04`, `/mnt/a\04`},
		{`/mnt/a\x41`, `/mnt/a\x41`},
	}
	for _, c := range cases {
		if got := unescapeMountField(c.in); got != c.want {
			t.Errorf("unescapeMountField(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestParseMountInfoAndMounts(t *testing.T) {
	set := parseMountInfo(fnosMountInfoFixture)
	for _, want := range []string{"/", "/boot", "/tmp", "/vol1", "/vol00/WDC WD10SDRW-34A0XS0"} {
		if !set[want] {
			t.Errorf("mountinfo 未解析出挂载点 %q", want)
		}
	}
	// 真机上 /vol00 与 /vol02 都不在挂载表里——它们只是普通目录。
	for _, unwanted := range []string{"/vol00", "/vol02"} {
		if set[unwanted] {
			t.Errorf("mountinfo 不该把 %q 当成挂载点", unwanted)
		}
	}
	// 同一份数据用 /proc/mounts 格式解析（字段 2 是挂载点）。
	mset := parseMounts("/dev/mmcblk0p2 / btrfs rw 0 0\n/dev/dm-0 /vol1 ext4 rw 0 0\n")
	if !mset["/"] || !mset["/vol1"] {
		t.Errorf("parseMounts 丢失挂载点: %v", mset)
	}
	// 畸形行不猜。
	if got := parseMountInfo("garbage\n\n1 2 3 4\n"); len(got) != 0 {
		t.Errorf("畸形行应被跳过，却得到 %v", got)
	}
}

func TestIsStorageVolumeName(t *testing.T) {
	cases := []struct {
		name string
		want bool
	}{
		{"vol1", true},
		{"vol2", true},
		{"vol12", true},
		// 真机实测存在的普通目录，不是存储空间。
		{"vol00", false},
		{"vol", false},
		{"volumex", false},
		{"vol1x", false},
		{"volume", false},
		{"vol-1", false},
		{"sda", false},
	}
	for _, c := range cases {
		if got := isStorageVolumeName(c.name); got != c.want {
			t.Errorf("isStorageVolumeName(%q) = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestVolIndexOnlyMatchesVolumeRoot(t *testing.T) {
	cases := map[string]int{
		"/vol1":          1,
		"/vol12":         12,
		"/vol1/@appdata": -1, // 子路径不是存储空间根
		"/vol00":         -1,
		"/vol00/WD":      -1,
		"/volumex":       -1,
		"/tmp":           -1,
	}
	for p, want := range cases {
		if got := volIndex(p); got != want {
			t.Errorf("volIndex(%q) = %d, want %d", p, got, want)
		}
	}
}

func TestClassifyMount(t *testing.T) {
	cases := []struct {
		mount    string
		wantKind string
		wantText string
		wantOK   bool
	}{
		{"/", volumeKindSystem, "系统空间", true},
		{"/vol1", volumeKindStorage, "存储空间1", true},
		{"/vol2", volumeKindStorage, "存储空间2", true},
		{"/vol12", volumeKindStorage, "存储空间12", true},
		// /vol00 是外接存储容器，自身不是磁盘；它下面的一级目录才是外接盘。
		{"/vol00", "", "", false},
		{"/vol00/WDC WD10SDRW-34A0XS0", volumeKindExternal, "外接存储（WDC WD10SDRW-34A0XS0）", true},
		// 容器下的多级路径同样不是独立磁盘。
		{"/vol00/a/b", "", "", false},
		// 其它真实挂载点没有飞牛语义名，显示真实路径而不是编一个。
		{"/mnt/backup", volumeKindOther, "/mnt/backup", true},
	}
	for _, c := range cases {
		kind, label, ok := classifyMount(c.mount)
		if ok != c.wantOK || kind != c.wantKind || label != c.wantText {
			t.Errorf("classifyMount(%q) = (%q, %q, %v), want (%q, %q, %v)",
				c.mount, kind, label, ok, c.wantKind, c.wantText, c.wantOK)
		}
	}
}

func TestMountOracleUsesMountTable(t *testing.T) {
	stubMountTable(t, fnosMountInfoFixture)
	oracle := newMountOracle()

	for _, p := range []string{"/", "/vol1", "/vol00/WDC WD10SDRW-34A0XS0"} {
		if is, known := oracle.isMount(p); !known || !is {
			t.Errorf("isMount(%q) = (%v, %v), want (true, true)", p, is, known)
		}
	}
	// 关键断言：这两个目录真实存在，但不是挂载点，因此必须被挡掉。
	for _, p := range []string{"/vol00", "/vol02"} {
		if is, known := oracle.isMount(p); !known || is {
			t.Errorf("isMount(%q) = (%v, %v), want (false, true)", p, is, known)
		}
	}
}

func TestMountOracleFallsBackToDeviceID(t *testing.T) {
	// 挂载表读不到时（Windows、受限容器）改用设备号比较，而不是一律保留。
	stubMountTable(t, "")
	stubMountDeviceOf(t, map[string]uint64{
		"/":                           100,
		"/vol1":                       200,
		"/vol02":                      100, // 与 / 同一个文件系统 → 不是独立分区
		"/vol00":                      100, // 容器目录同样在根文件系统上
		"/vol00/WDC WD10SDRW-34A0XS0": 300,
	})
	oracle := newMountOracle()
	cases := map[string]bool{
		"/":                           true,
		"/vol1":                       true,
		"/vol00/WDC WD10SDRW-34A0XS0": true,
		"/vol02":                      false,
		"/vol00":                      false,
	}
	for p, want := range cases {
		if is, known := oracle.isMount(p); !known || is != want {
			t.Errorf("isMount(%q) = (%v, %v), want (%v, true)", p, is, known, want)
		}
	}
}

func TestMountOracleKeepsCandidateWhenUnknown(t *testing.T) {
	// 两种都判不出来时必须保留候选：宁可多一行，也不能把真实磁盘藏起来。
	stubMountTable(t, "")
	stubMountDeviceOf(t, map[string]uint64{}) // 所有设备号读取都失败
	oracle := newMountOracle()

	if is, known := oracle.isMount("/vol1"); known {
		t.Errorf("设备号不可用时 isMount 应返回 known=false，却得到 (%v, true)", is)
	}
	if !oracle.acceptMount("/vol1") {
		t.Error("无法判定时必须保留候选 /vol1")
	}

	// 挂载表存在但一行都认不出来时同样算「不知道」：此时「不在表里」并不代表
	// 「没挂载」，据此过滤会把真实磁盘整片藏起来。
	stubMountTable(t, "garbage\n\n")
	if _, known := newMountOracle().isMount("/vol1"); known {
		t.Error("不可解析的挂载表应视为不知道")
	}
}

func TestSpaceVolumesInSkipsNonMounts(t *testing.T) {
	// 用临时目录模拟根文件系统：真机上 /vol00、/vol02 确实存在，只是没被挂载。
	root := t.TempDir()
	for _, name := range []string{"vol1", "vol00", "vol02", "volumex"} {
		if err := os.Mkdir(path.Join(root, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// 挂载表按绝对路径判定，临时目录下的路径不在表里，因此这里用设备号回退：
	// 让挂载表不可用，靠设备号区分「真挂载」与「根文件系统上的普通目录」。
	// 键必须与 spaceVolumesIn/isMount 实际拼接出的字符串一致。
	stubMountTable(t, "")
	stubMountDeviceOf(t, map[string]uint64{
		root:                       100,
		path.Join(root, "vol00"):   100,
		path.Join(root, "vol02"):   100,
		path.Join(root, "volumex"): 100,
		path.Join(root, "vol1"):    200,
	})

	got := spaceVolumesIn(root, newMountOracle())
	want := path.Join(root, "vol1")
	if len(got) != 1 || got[0] != want {
		t.Fatalf("spaceVolumesIn = %v, want [%s]", got, want)
	}
}

func TestSpaceDiskMountsOrdering(t *testing.T) {
	got := spaceDiskMounts([]string{"/vol1"}, []string{"/vol00/WDC WD10SDRW-34A0XS0"})
	// 外接存储不参与「系统空间清理」：即便被传进来也不能出现在磁盘列表里。
	want := []string{"/", "/vol1"}
	if len(got) != len(want) {
		t.Fatalf("spaceDiskMounts = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("spaceDiskMounts = %v, want %v（系统空间必须第一行、外接存储不得出现）", got, want)
		}
	}
	// 重复传入不应产生重复行（重复行的容量数字会完全一样）。
	if dup := spaceDiskMounts([]string{"/vol1", "/"}, []string{"/vol1"}); len(dup) != 2 {
		t.Fatalf("去重失败: %v", dup)
	}
}

// 外接存储既不该被组装进磁盘列表，也不该在 scanVolumeUsage 里漏出来——
// 后一层是防御旧快照/误传参数的兜底。
func TestSpaceDiskListsExcludeExternal(t *testing.T) {
	orig := volumeUsageOf
	volumeUsageOf = func(string) (uint64, uint64, uint64, error) {
		const total, free = 100 << 30, 60 << 30
		return total, free, total - free, nil
	}
	t.Cleanup(func() { volumeUsageOf = orig })

	out := scanVolumeUsage(spaceDiskMounts(
		[]string{"/vol1", "/vol00/WDC WD10SDRW-34A0XS0"},
		[]string{"/vol00/WDC WD10SDRW-34A0XS0"},
	))
	wantMounts := []string{"/", "/vol1"}
	if len(out) != len(wantMounts) {
		t.Fatalf("scanVolumeUsage 行数 = %d, want %d: %+v", len(out), len(wantMounts), out)
	}
	for i, v := range out {
		if v.Mount != wantMounts[i] {
			t.Errorf("第 %d 行 Mount = %q, want %q", i, v.Mount, wantMounts[i])
		}
		if v.Kind == volumeKindExternal {
			t.Errorf("外接存储不该出现在系统空间清理里: %+v", v)
		}
	}
}

func TestScanVolumeUsageCarriesSemanticLabels(t *testing.T) {
	orig := volumeUsageOf
	volumeUsageOf = func(string) (uint64, uint64, uint64, error) {
		const total, free = 100 << 30, 60 << 30
		return total, free, total - free, nil
	}
	t.Cleanup(func() { volumeUsageOf = orig })

	// /vol00 容器目录与外接盘混在候选里，都必须被剔除；/ 由 scanVolumeUsage 自行补上。
	out := scanVolumeUsage([]string{"/vol1", "/vol00", "/vol00/WDC WD10SDRW-34A0XS0"})
	if len(out) != 2 {
		t.Fatalf("scanVolumeUsage 行数 = %d, want 2: %+v", len(out), out)
	}
	wantKinds := []string{volumeKindStorage, volumeKindSystem}
	wantLabels := []string{"存储空间1", "系统空间"}
	wantMounts := []string{"/vol1", "/"}
	for i, v := range out {
		if v.Kind != wantKinds[i] || v.Label != wantLabels[i] {
			t.Errorf("第 %d 行 = (%q, %q), want (%q, %q)", i, v.Kind, v.Label, wantKinds[i], wantLabels[i])
		}
		// 原始路径必须保留：前端 :key 与清理规则都依赖它。
		if v.Mount != wantMounts[i] {
			t.Errorf("第 %d 行 Mount = %q, want %q", i, v.Mount, wantMounts[i])
		}
		if v.Mount == "/vol00" {
			t.Error("/vol00 容器目录不该出现在磁盘列表里")
		}
	}
	// 40 GiB / 100 GiB。
	if p := out[0].UsedPercent; p < 39.9 || p > 40.1 {
		t.Errorf("UsedPercent = %v, want ~40", p)
	}
}
