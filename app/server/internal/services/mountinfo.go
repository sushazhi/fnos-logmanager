package services

import (
	"errors"
	"path"
	"strconv"
	"strings"
)

// errMountTableUnavailable 表示当前平台读不到挂载表（Windows 上恒定如此）。
//
// 它是「这个路径是不是挂载点」的第三种答案：不知道。调用方必须把「不知道」和
// 「不是挂载点」分开处理——只有后者才允许因此少显示一行磁盘占用。
var errMountTableUnavailable = errors.New("mount table unavailable")

// externalContainerMount 是飞牛存放外接存储的容器目录。
//
// 真机实测：外接盘挂在 /vol00/<挂载名>（如 /vol00/WDC WD10SDRW-34A0XS0），
// /vol00 自身只是根文件系统上的一个普通目录，不是任何分区，不能当作一行磁盘。
const externalContainerMount = "/vol00"

// 磁盘类别，与前端约定的 kind 取值。
const (
	volumeKindSystem   = "system"   // 系统空间（/）
	volumeKindStorage  = "storage"  // 存储空间N（/volN）
	volumeKindExternal = "external" // 外接存储（/vol00/<挂载名>）
	volumeKindOther    = "other"    // 其它真实挂载点
)

// mountTableFn 是 readMountTable 的注入点，只为测试服务：让 Windows 上也能用
// 真实格式的挂载表文本驱动解析与判定逻辑。
var mountTableFn = readMountTable

// unescapeMountField 还原挂载表字段里的八进制转义。
//
// /proc 约定空格 \040、制表符 \011、换行 \012、反斜杠 \134。外接盘挂载名常带
// 空格（真机为 /vol00/WDC WD10SDRW-34A0XS0），不还原就会与真实路径对不上，
// 挂载点判定随即失效——那正是这一整块逻辑要防的错。
func unescapeMountField(s string) string {
	if !strings.Contains(s, `\`) {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+4 <= len(s) {
			if v, ok := octalByte(s[i+1 : i+4]); ok {
				b.WriteByte(v)
				i += 3
				continue
			}
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

// octalByte 解析三位八进制转义（"040" → 空格）。
func octalByte(s string) (byte, bool) {
	if len(s) != 3 {
		return 0, false
	}
	v := 0
	for i := 0; i < 3; i++ {
		c := s[i]
		if c < '0' || c > '7' {
			return 0, false
		}
		v = v*8 + int(c-'0')
	}
	return byte(v), true
}

// parseMountInfo 解析 /proc/self/mountinfo，返回挂载点集合。
//
// 每行空格分隔：第 5 个字段（下标 4）恒为挂载点，"-" 之后才是文件系统类型与
// 挂载源。这里只要挂载点，字段数不足的畸形行直接跳过，不猜。
//
// 挂载表里每一行都对应一个真实挂载点，因此解析出的集合可以直接当作「是挂载点」
// 的权威答案——这正是它比 file is-mount-point 之类接口可靠的地方（真机上后者对
// /vol1 也返回 hasMount:false，问的是「下面还有没有嵌套挂载」，不是「自身是否是
// 挂载点」）。
func parseMountInfo(data string) map[string]bool {
	out := map[string]bool{}
	for _, line := range strings.Split(data, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}
		if mp := unescapeMountField(fields[4]); mp != "" {
			out[path.Clean(mp)] = true
		}
	}
	return out
}

// parseMounts 解析 /proc/mounts，返回挂载点集合（第 2 个字段）。
func parseMounts(data string) map[string]bool {
	out := map[string]bool{}
	for _, line := range strings.Split(data, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		if mp := unescapeMountField(fields[1]); mp != "" {
			out[path.Clean(mp)] = true
		}
	}
	return out
}

// mountPointSet 返回本机所有挂载点，known=false 表示读不到挂载表。
//
// 空表一律当作「不知道」：一行都解析不出来时若把「不在表里」当成「不是挂载点」，
// 会把真实磁盘整片藏起来，那比多显示一行严重得多。
func mountPointSet() (map[string]bool, bool) {
	data, err := mountTableFn()
	if err != nil || data == "" {
		return nil, false
	}
	if set := parseMountInfo(data); len(set) > 0 {
		return set, true
	}
	if set := parseMounts(data); len(set) > 0 {
		return set, true
	}
	return nil, false
}

// mountOracle 判断路径是否为真实挂载点，并在一次扫描内复用挂载表。
//
// devOf 是设备号读取，默认 deviceOf；测试可替换。挂载表只读一次，避免每判定一个
// 路径就重新读一遍 /proc。
type mountOracle struct {
	set   map[string]bool
	known bool
	devOf func(string) (uint64, error)
}

func newMountOracle() mountOracle {
	set, known := mountPointSet()
	return mountOracle{set: set, known: known, devOf: systemDeviceOf}
}

// isMount 报告 p 是否是真实挂载点，known=false 表示无法判定。
//
// 两级判定：
//  1. 挂载表（权威）。表里没有 p，p 就不是挂载点——/vol00、/vol02 这类「看着像
//     存储卷、实为根文件系统上的普通目录」就死在这里。
//  2. 挂载表不可用时退化为比较设备号：目录与父目录同属一个文件系统，说明父目录
//     上没挂别的东西，它不该单独占一行（容量数字也会与父目录完全重复）。
//
// 两级都判不出来时返回 known=false，调用方必须保留候选：宁可多一行，也不能把
// 真实磁盘藏起来。
func (o mountOracle) isMount(p string) (is bool, known bool) {
	clean := path.Clean(p)
	if o.known {
		return o.set[clean], true
	}
	parent := path.Dir(clean)
	if parent == clean {
		return true, true // 根目录自身必是挂载点
	}
	devOf := o.devOf
	if devOf == nil {
		devOf = systemDeviceOf
	}
	self, err := devOf(clean)
	if err != nil {
		return false, false
	}
	up, err := devOf(parent)
	if err != nil {
		return false, false
	}
	return self != up, true
}

// acceptMount 报告路径是否该作为磁盘/扫描目标保留：判不出来时保留。
func (o mountOracle) acceptMount(p string) bool {
	is, known := o.isMount(p)
	return !known || is
}

// isStorageVolumeName 报告目录名是否是存储空间目录（vol1、vol2 …）。
//
// 排除 vol00：那是外接存储的容器目录。只认纯数字后缀，volumex、vol 这类不算。
// 单独抽出来是因为它同时守着两件事：「不是存储空间」（vol00）与「不是存储卷」
// （volumex）——两者都不该出现在磁盘列表里。
func isStorageVolumeName(name string) bool {
	num, ok := strings.CutPrefix(name, "vol")
	if !ok || num == "" {
		return false
	}
	for i := 0; i < len(num); i++ {
		if num[i] < '0' || num[i] > '9' {
			return false
		}
	}
	n, err := strconv.Atoi(num)
	return err == nil && n > 0
}

// volIndex 返回存储空间序号 N（/vol1 → 1）；不是存储空间根目录时返回 -1。
//
// 只看路径最后一段，因此 /vol1/@appdata 这类子路径不会被误判成存储空间。
func volIndex(p string) int {
	name := path.Base(path.Clean(p))
	if !isStorageVolumeName(name) {
		return -1
	}
	n, _ := strconv.Atoi(strings.TrimPrefix(name, "vol"))
	return n
}

// classifyMount 把一个真实挂载点归类，并给出飞牛语义名。
//
// 飞牛存储管理里这三类分别叫「系统空间」「存储空间N」「外接存储」。开放接口文档
// 明确要求应用页面不要直接展示 /vol1/... 这类内部路径，因此标签由服务端统一给出，
// 前端只负责显示，避免两边各写一套路径语义。
//
// ok=false 表示这个路径不该单独占一行（例如 /vol00 容器目录本身）。
func classifyMount(mount string) (kind, label string, ok bool) {
	clean := path.Clean(mount)
	switch {
	case clean == systemDiskMount:
		return volumeKindSystem, "系统空间", true
	case clean == externalContainerMount:
		return "", "", false
	case strings.HasPrefix(clean, externalContainerMount+"/"):
		name := strings.TrimPrefix(clean, externalContainerMount+"/")
		if name == "" || strings.Contains(name, "/") {
			return "", "", false
		}
		return volumeKindExternal, "外接存储（" + name + "）", true
	}
	if n := volIndex(clean); n > 0 {
		return volumeKindStorage, "存储空间" + strconv.Itoa(n), true
	}
	// 其它真实挂载点（手工挂载等）：没有飞牛语义名，显示真实路径。
	return volumeKindOther, clean, true
}
