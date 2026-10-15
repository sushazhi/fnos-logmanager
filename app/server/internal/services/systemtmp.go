package services

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/sushazhi/fnos-logmanager/internal/utils"
)

// errDeviceInfoUnavailable 表示无法读取文件系统设备号（Windows 上恒定如此）。
//
// 调用方必须把它当作"无法判定"，绝不能当作"不同文件系统"：两者对应完全相反的
// 用户可见文案（见 systemTempOnSystemDisk）。
var errDeviceInfoUnavailable = errors.New("device id unavailable")

// ---------------------------------------------------------------------------
// 系统临时目录清理（/tmp、/var/tmp）
// ---------------------------------------------------------------------------
//
// 这两个目录不在 /volN 下，无法用存储卷规则表描述，因此单独一套判定逻辑。
//
// 真机勘察显示 fnOS 的系统分区只有 6.76 GiB、可用约 2.45 GiB，是整台设备最紧张的
// 分区；扫描 /volN 完全看不到这两处，所以必须单独列出。它们是否与系统分区同处一个
// 文件系统由运行时实测决定（systemTempOnSystemDisk），不做静态断言——/tmp 有可能
// 是 tmpfs，那时清理释放的是内存而不是磁盘空间。
//
// 与 /volN 上的缓存目录不同，临时目录里的条目"叫什么名字"完全不能说明它是否
// 可删（用户和应用都可以起任意名字）。因此判定依据只能是运行期事实，这里用双闸门：
//
//  1. 没有任何进程打开它（枚举 /proc/<pid>/{fd,cwd,exe} 得出）；
//  2. 闲置时间已超过 systemTempMinAge。
//
// 任一闸门无法确认，就不提供删除。这比 systemd-tmpfiles 的默认策略更保守：
// 后者只按 mtime 判断，会在应用短暂关闭文件时误删。

// systemTempRoots 是纳入清理范围的系统临时目录。
//
// 声明为变量而不是常量数组，是为了让单元测试能指向临时构造的目录树。
var systemTempRoots = []string{"/tmp", "/var/tmp"}

// systemTempMinAge 是系统临时条目"闲置"多久之后才允许清理。
//
// 只靠"是否被打开"并不够：安装程序、解压、转码等操作会在两个步骤之间短暂关闭
// 文件，此时它看起来是闲置的。再加一道时间闸门可以覆盖这种间隙。
const systemTempMinAge = 24 * time.Hour

// systemDiskMount 是系统分区挂载点。
//
// 真机上最紧张的分区是它（6.76 GiB），而不是任何一个存储卷，所以必须单独展示，
// 否则用户只会看到 /vol1 还剩 185 GiB，完全意识不到系统分区已经快满了。
const systemDiskMount = "/"

// listOpenPaths 返回当前所有进程正在使用的绝对路径集合。
//
// 契约：返回 nil 表示"**无法判定**"（读取进程信息失败），此时清理一律拒绝；
// 返回空集合是合法结果，表示确实没有任何进程持有文件。
// 两者的区别必须保留，直接把"读不到"当成"没人用"会导致误删。
//
// 默认实现返回 nil，即无法判定。Linux 上的真实实现见 systemtmp_linux.go。
var listOpenPaths = func() map[string]bool { return nil }

var (
	openPathsMu     sync.Mutex
	openPathsCache  map[string]bool
	openPathsCached time.Time
)

// openPathsTTL 是 /proc 枚举结果的复用时长。
//
// 一次扫描会逐个条目做判定，若每项都重新遍历 /proc，开销会随进程数×条目数增长。
// 短暂复用是安全的：真正的兜底是 systemTempMinAge 那道时间闸门。
const openPathsTTL = 5 * time.Second

// openPathsSnapshot 返回当前打开的路径集合，nil 表示无法判定。
func openPathsSnapshot() map[string]bool {
	openPathsMu.Lock()
	defer openPathsMu.Unlock()
	if openPathsCache != nil && time.Since(openPathsCached) < openPathsTTL {
		return openPathsCache
	}
	fresh := listOpenPaths()
	if fresh == nil {
		// 不缓存失败结果，避免一次读取失败让清理在 TTL 内持续被拒绝。
		return nil
	}
	openPathsCache = fresh
	openPathsCached = time.Now()
	return fresh
}

// resetOpenPathsCache 清空 /proc 枚举缓存。供测试与清理入口使用。
func resetOpenPathsCache() {
	openPathsMu.Lock()
	openPathsCache = nil
	openPathsCached = time.Time{}
	openPathsMu.Unlock()
}

// systemTempRootFor 返回路径所属的系统临时根目录，不在其中时返回 ""。
//
// 比较前统一成 `/` 分隔：生产环境是 Linux（`\` 是合法文件名字符，转换不改变
// 任何真实路径），而单元测试在 Windows 上运行、t.TempDir 返回 `\` 分隔的路径。
// 不统一的话，同一份判定逻辑会在开发机上得出与真机不同的结论。
//
// 用 root+"/" 前缀比较，确保 /tmpfoo 不会被误判为 /tmp 下的条目。
func systemTempRootFor(absPath string) string {
	p := filepath.ToSlash(absPath)
	for _, root := range systemTempRoots {
		r := filepath.ToSlash(root)
		if p == r || strings.HasPrefix(p, r+"/") {
			return root
		}
	}
	return ""
}

// systemTempEntryName 判断路径是否是某个系统临时根的**一级子项**。
//
// 只处理一级子项是刻意的取舍：粒度可预测、每个条目能独立做占用判定，
// 用户看到的每一项都对应一个具体的文件或目录，不会出现"删掉一整棵子树"的意外。
// 根目录自身与更深层的路径都返回 ok=false。
func systemTempEntryName(absPath string) (root, name string, ok bool) {
	root = systemTempRootFor(absPath)
	if root == "" {
		return "", "", false
	}
	rel := strings.TrimPrefix(filepath.ToSlash(absPath), filepath.ToSlash(root))
	rel = strings.TrimPrefix(rel, "/")
	if rel == "" || strings.Contains(rel, "/") {
		return "", "", false
	}
	return root, rel, true
}

// spacePathInUse 判断路径自身或其内部是否有文件正被进程使用。
//
// 只检查"自身"和"内部"两个方向：
//   - 自身被打开（例如某进程的 cwd 就是它，或它被当作文件打开）；
//   - 内部有文件被打开（目录里还有活跃读写，删掉会打断对方）。
//
// 不需要检查祖先：删除 /tmp/foo 时，某进程的 cwd 为 /tmp（父目录）并不受影响。
func spacePathInUse(absPath string, openPaths map[string]bool) bool {
	if openPaths[absPath] {
		return true
	}
	// 同样做分隔符归一化：/proc 给出的永远是 `/` 形式，而测试里的临时目录
	// 在 Windows 上是 `\` 形式，不归一化会导致"内部文件被占用"被漏判。
	norm := filepath.ToSlash(absPath)
	prefix := norm + "/"
	for p := range openPaths {
		if strings.HasPrefix(filepath.ToSlash(p), prefix) {
			return true
		}
	}
	return false
}

// systemTempBlockReason 返回条目不可清理的原因；可以清理时返回空字符串。
//
// 返回原因而不是布尔值，是为了让界面上直接显示"为什么这个看着没用的目录不能删"。
func systemTempBlockReason(absPath string) string {
	openPaths := openPathsSnapshot()
	if openPaths == nil {
		return "当前无法确认该条目是否正被进程使用（读取进程信息失败），暂不提供删除"
	}
	if spacePathInUse(absPath, openPaths) {
		return "有进程正在使用该条目，删除可能影响正在运行的程序"
	}
	fi, err := os.Lstat(absPath)
	if err != nil {
		return "路径不存在"
	}
	if age := time.Since(fi.ModTime()); age < systemTempMinAge {
		return fmt.Sprintf("最近 %d 小时内仍被修改，暂不提供删除（系统临时文件需闲置满 %d 小时）",
			int(systemTempMinAge.Hours()), int(systemTempMinAge.Hours()))
	}
	return ""
}

// systemTempEntryDeletable 判断系统临时目录下的条目当前是否可以安全删除。
func systemTempEntryDeletable(absPath string) bool {
	return systemTempBlockReason(absPath) == ""
}

// stripDeletedSuffix 去掉内核为已删除文件附加的 " (deleted)" 后缀。
//
// 一个进程可以继续持有已被 unlink 的文件的 fd，内核会把该 fd 的目标显示为
// "/path (deleted)"。保留原路径可以让这种条目也被识别为"仍在使用"，
// 从而避免在对方还没释放时重复处理同一路径。
func stripDeletedSuffix(target string) string {
	return strings.TrimSuffix(target, " (deleted)")
}

// matchSystemTempForPath 校验一个系统临时目录路径是否允许清理。
//
// 与存储卷规则一致，这里是清理操作的准入判断，会独立复算运行期状态，
// 不依赖扫描结果——扫描与清理之间进程可能已经开始使用该条目。
func matchSystemTempForPath(absPath string) (spaceRule, bool) {
	root, _, ok := systemTempEntryName(absPath)
	if !ok {
		return spaceRule{}, false
	}
	// 根目录本身若是符号链接，说明 /tmp 被指向了别处，此时拒绝整个根，
	// 避免顺着链接删到规则之外的位置。
	if fi, err := os.Lstat(root); err != nil || fi.Mode()&os.ModeSymlink != 0 {
		return spaceRule{}, false
	}
	if isAnySymlinkComponent(absPath) {
		return spaceRule{}, false
	}
	if !systemTempEntryDeletable(absPath) {
		return spaceRule{}, false
	}
	return spaceRule{
		Pattern:  root + "/*",
		Category: "system-temp",
		Safe:     true,
		Strategy: "delete",
		Reason: "系统临时文件，当前没有任何进程使用且已闲置超过 " +
			fmt.Sprintf("%d", int(systemTempMinAge.Hours())) + " 小时",
	}, true
}

// buildSystemTempItem 为一个系统临时条目构造展示项。
//
// 不可清理的条目同样会被列出（Safe=false、InUse=true），因为用户需要看到
// "系统分区被什么占着"，而不是让它们从界面上消失。
func buildSystemTempItem(path string) SpaceItem {
	size, files, _, modTime, _ := spacePathStats(path, spaceScanEntryCap)
	fi, err := os.Lstat(path)
	isDir := err == nil && fi.IsDir()

	kind := "文件"
	if isDir {
		kind = "目录"
	}

	safe := true
	inUse := false
	reason := "系统临时" + kind + "，当前没有任何进程使用，可安全删除"
	if block := systemTempBlockReason(path); block != "" {
		safe = false
		inUse = true
		reason = block
	}

	return SpaceItem{
		Path:          path,
		Name:          filepath.Base(path),
		Size:          size,
		SizeFormatted: utils.FormatBytes(size),
		IsDir:         isDir,
		FileCount:     files,
		Reason:        reason,
		Safe:          safe,
		InUse:         inUse,
		Strategy:      "delete",
		Modified:      modTime,
	}
}

// systemDeviceOf 是 deviceOf 的注入点，只为测试服务。
//
// 与 listOpenPaths 同样的理由：Windows 上 deviceOf 永远返回错误，
// 而"同分区 / 不同分区 / 无法判定"这三种结论对应三种不同的用户可见文案，
// 必须在开发机上也能全部验证到。
var systemDeviceOf = deviceOf

// systemTempOnSystemDisk 报告 systemTempRoots 是否与 systemDiskMount 位于同一
// 文件系统，供容量条提示文案使用。
//
// 为什么不写死：/tmp 完全可能是 tmpfs（内存盘），此时清理释放的是内存而不是
// 系统分区空间，提示"/tmp 位于系统分区"就是错的。而 trim-cli 的 file/storage
// 子命令只接受 /volN 路径，读不到 /tmp 的挂载信息，所以只能由服务端自己判断。
//
// 返回 (same, known)：读不到设备号时 known=false，调用方必须按"未知"处理，
// 不能当作"不同文件系统"——两者对应完全相反的用户可见结论。
func systemTempOnSystemDisk() (same bool, known bool) {
	diskDev, err := systemDeviceOf(systemDiskMount)
	if err != nil {
		return false, false
	}
	roots := 0
	for _, root := range systemTempRoots {
		fi, err := os.Lstat(root)
		if err != nil || fi.Mode()&os.ModeSymlink != 0 || !fi.IsDir() {
			continue
		}
		dev, err := systemDeviceOf(root)
		if err != nil {
			return false, false
		}
		roots++
		// 只要有一个根不在系统分区上，就不能笼统地说"都在系统分区"。
		if dev != diskDev {
			return false, true
		}
	}
	if roots == 0 {
		return false, false
	}
	return true, true
}

// systemTempEntryScannable 报告一个目录项是否应作为候选清理目标。
//
// 排除两类：
//   - 符号链接：删掉它释放不了任何空间，却可能让依赖该链接的程序失效；
//     而且顺着链接做体积统计会统计到链接指向的目标，数字具有误导性。
//   - "." / ".." 与非一级子项（os.ReadDir 不会返回这两个名字，此处为防御）。
func systemTempEntryScannable(e os.DirEntry) bool {
	name := e.Name()
	if name == "" || name == "." || name == ".." {
		return false
	}
	return e.Type()&os.ModeSymlink == 0
}

// scanSystemTempItems 扫描 /tmp 与 /var/tmp 的一级子项。
func scanSystemTempItems() []SpaceItem {
	var items []SpaceItem
	for _, root := range systemTempRoots {
		entries, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !systemTempEntryScannable(e) {
				continue
			}
			item := buildSystemTempItem(filepath.Join(root, e.Name()))
			// 0 字节的普通文件释放不了空间，列出来只是噪音；
			// 与规则表扫描（buildSpaceItem）的取舍保持一致。
			if item.Size == 0 && !item.IsDir {
				continue
			}
			items = append(items, item)
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Size > items[j].Size })
	return items
}
