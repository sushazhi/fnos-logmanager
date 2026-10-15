package services

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/sushazhi/fnos-logmanager/internal/utils"
)

// 系统分区（/）清理规则。
//
// 背景：/volN 规则表（spaceRules）只覆盖存储卷，而真机上最紧张的恰恰是
// 6.8 GiB 的系统分区（eMMC，btrfs）。实测该分区上可无痛回收的项目全部位于
// /var、/usr、/boot，`trim-cli` 又只接受 /vol{v}/... 路径，无法从外部枚举，
// 因此必须由运行在设备上、以 root 身份运行的本应用来扫描。
//
// 与 /volN 规则表的三点关键差异：
//
//  1. **绝对路径匹配**：这些路径不在任何卷下，不能复用「相对卷根」的展开逻辑，
//     因此单独一张表 + 单独的匹配函数（matchSystemPath）。
//  2. **更保守**：系统分区上的文件删错会导致设备无法启动或系统服务异常。
//     凡是涉及包管理器的目录一律**不直删**，改为走包管理器语义
//     （见 systemCommandRules），或只做提示。
//  3. **绝不碰内核与引导**：/boot、/usr/lib/modules、/usr/lib/linux-image-*
//     一概不属于本表；内核清理由 routes/kernel.go 的专用功能负责，
//     那里有 uname -r + extlinux.conf 双重校验。

// systemPathRule 描述系统分区上一条可清理的绝对路径。
type systemPathRule struct {
	// Pattern 是绝对路径模式，用 `/` 分段，`*` 精确匹配一段。
	Pattern string
	// Category 复用 spaceCategoryOrder 里的分类 ID。
	Category string
	// Strategy 决定清理方式：delete（直接删除）/ truncate（截断）。
	//
	// 本表**不使用** recycle：回收站位于 /volN，把系统分区的东西移过去
	// 并不能可靠地解决系统分区空间问题，反而增加跨分区搬运的风险。
	Strategy string
	// Reason 展示给用户。
	Reason string
	// MaxAge > 0 时，只有最后修改时间早于该时长才提供清理。
	// 用于日志类目标：正在被写入的日志截断后仍会继续增长，
	// 但对"刚刚还在写"的日志做操作没有意义，也更容易与轮转竞争。
	MaxAge time.Duration
	// DirectChildren 为 true 时只处理该目录的直接子项（各自的单文件），
	// 而不是把整个目录当一个条目。日志目录需要这样按文件列出。
	DirectChildren bool
}

// systemPathRules 是系统分区上可直删/截断的目标表。
//
// 设计原则：只收录「删掉后系统仍能正常工作、且能自行重建」的东西。
var systemPathRules = []systemPathRule{
	// ---- apt / dpkg 缓存与索引（可再生）----
	//
	// /var/cache/apt/*.bin 是 apt 的二进制索引缓存，apt-get update 会重建；
	// /var/lib/apt/lists 是软件源索引，同样可重建。
	// 这两个目录都由 apt 管理，直删虽然能释放空间，但会让 apt 的
	// "哪些索引是新的"判断与磁盘不一致；因此：
	//   - 只删明确的缓存文件（*.bin），不动 apt 的其他状态文件；
	//   - lists 目录单独用 apt-get clean 语义处理（见 systemCommandRules）。
	{Pattern: "/var/cache/apt/pkgcache.bin", Category: "system-pkg-cache", Strategy: "delete",
		Reason: "apt 二进制索引缓存，apt-get update 会自动重建"},
	{Pattern: "/var/cache/apt/srcpkgcache.bin", Category: "system-pkg-cache", Strategy: "delete",
		Reason: "apt 源码索引缓存，apt-get update 会自动重建"},

	// ---- 系统日志：必须截断而非删除 ----
	//
	// syslog / kern.log / auth.log 等由 rsyslog 持有打开的 fd。直接 unlink 后
	// rsyslog 仍往已删除的 inode 写入，空间不会立即释放，且新日志会"消失"，
	// 直到服务重启。截断则保留 inode，rsyslog 无需重启即继续正常写入。
	// 这与 docker/*-json.log 的处理策略一致。
	{Pattern: "/var/log/syslog", Category: "system-logs", Strategy: "truncate", MaxAge: time.Hour,
		Reason: "系统日志，截断后 rsyslog 无需重启即可继续写入"},
	{Pattern: "/var/log/syslog.1", Category: "system-logs", Strategy: "truncate", MaxAge: time.Hour,
		Reason: "已轮转的系统日志历史"},
	{Pattern: "/var/log/kern.log", Category: "system-logs", Strategy: "truncate", MaxAge: time.Hour,
		Reason: "内核日志，截断后继续写入"},
	{Pattern: "/var/log/kern.log.1", Category: "system-logs", Strategy: "truncate", MaxAge: time.Hour,
		Reason: "已轮转的内核日志历史"},
	{Pattern: "/var/log/auth.log", Category: "system-logs", Strategy: "truncate", MaxAge: time.Hour,
		Reason: "认证日志，截断后继续写入"},
	{Pattern: "/var/log/auth.log.1", Category: "system-logs", Strategy: "truncate", MaxAge: time.Hour,
		Reason: "已轮转的认证日志历史"},
	{Pattern: "/var/log/cron.log", Category: "system-logs", Strategy: "truncate", MaxAge: time.Hour,
		Reason: "定时任务日志，截断后继续写入"},
	{Pattern: "/var/log/cron.log.1", Category: "system-logs", Strategy: "truncate", MaxAge: time.Hour,
		Reason: "已轮转的定时任务日志历史"},
	{Pattern: "/var/log/daemon.log", Category: "system-logs", Strategy: "truncate", MaxAge: time.Hour,
		Reason: "守护进程日志，截断后继续写入"},
	{Pattern: "/var/log/messages", Category: "system-logs", Strategy: "truncate", MaxAge: time.Hour,
		Reason: "系统消息日志，截断后继续写入"},
	{Pattern: "/var/log/user.log", Category: "system-logs", Strategy: "truncate", MaxAge: time.Hour,
		Reason: "用户态日志，截断后继续写入"},

	// 轮转日志（syslog.2.gz 等）。它们已经归档、不再被写入，直接删除即可；
	// 这里按「直接子项」逐文件列出，避免把整个 /var/log 当一个条目对待。
	{Pattern: "/var/log", Category: "system-logs", Strategy: "delete", MaxAge: 0,
		DirectChildren: true,
		Reason:         "已轮转归档的日志文件，删除不影响正在写入的日志"},

	// ---- 服务日志（各自独立目录）----
	{Pattern: "/var/log/samba/*", Category: "system-logs", Strategy: "truncate", MaxAge: time.Hour,
		Reason: "Samba 服务日志，截断后服务无需重启即可继续写入"},
	{Pattern: "/var/log/trim_open_gateway/*", Category: "system-logs", Strategy: "truncate", MaxAge: time.Hour,
		Reason: "fnOS 开放网关日志，截断后继续写入"},
	{Pattern: "/var/log/trim_app_center/*", Category: "system-logs", Strategy: "truncate", MaxAge: time.Hour,
		Reason: "应用中心日志，截断后继续写入"},
	{Pattern: "/var/log/cloud_storage_dav/*", Category: "system-logs", Strategy: "truncate", MaxAge: time.Hour,
		Reason: "云存储服务日志，截断后继续写入"},

	// ---- sysstat 历史采样 ----
	//
	// sar 文件是历史性能采样，删除只丢失历史曲线，不影响采集本身。
	// 它们不会被任何进程长期持有，因此直接删除。
	{Pattern: "/var/log/sysstat/*", Category: "system-logs", Strategy: "delete",
		Reason: "sysstat 历史性能采样数据，删除后按日重新生成"},

	// ---- 包管理残留（非 apt 管理的零散文件）----
	{Pattern: "/var/lib/smartmontools/drivedb/drivedb.h.old", Category: "system-residue", Strategy: "delete",
		Reason: "smartmontools 硬盘库的旧版本备份文件"},
	{Pattern: "/var/lib/apt/cdroms.list~", Category: "system-residue", Strategy: "delete",
		Reason: "apt 编辑 cdroms.list 时留下的备份文件"},
	{Pattern: "/etc/rsyslog.conf.dpkg-dist", Category: "system-residue", Strategy: "delete",
		Reason: "dpkg 升级时保留的旧配置副本（.dpkg-dist），非当前生效配置"},
	{Pattern: "/etc/logrotate.d/rsyslog.dpkg-dist", Category: "system-residue", Strategy: "delete",
		Reason: "dpkg 升级时保留的旧配置副本（.dpkg-dist），非当前生效配置"},
	{Pattern: "/etc/modprobe.d/mdadm.conf.dpkg-dist", Category: "system-residue", Strategy: "delete",
		Reason: "dpkg 升级时保留的旧配置副本（.dpkg-dist），非当前生效配置"},
	{Pattern: "/var/lib/nfs/sm.bak", Category: "system-residue", Strategy: "delete",
		Reason: "NFS 状态迁移文件残留"},
	// 注意：**没有** /boot/vmlinuz.old 规则。
	// 它是悬空符号链接（指向已删除的旧内核），确实是残留，但 /boot 在兜底拒绝
	// 名单里——引导分区上的任何删除都必须由内核清理功能带着 uname -r +
	// extlinux.conf 双重校验来做，不能走通用规则表。它只作为提示项展示
	// （见 scanSystemBGradeNotices 之外的 dangling 提示）。
}

// systemCommandRule 描述一条「必须交给包管理器执行」的清理目标。
//
// 这类目标**不能**直接 os.Remove：它们由 apt/dpkg 管理，绕过包管理器直删会让
// 包数据库与实际文件不一致。真机上已经出现过这种不一致的代价——一次手工 rm
// 把 8 个内核包留在 hi（半安装）态，并漏掉了 36 MB 的孤儿目录。
//
// 因此这里只提供命令与说明，由用户在设备上执行（本应用不代为运行）。
type systemCommandRule struct {
	// ID 用于去重与前端 key。
	ID string
	// Category 复用分类 ID。
	Category string
	// Path 是体积统计所依据的目录（用于展示"能回收多少"）。
	Path string
	// Title / Reason 展示给用户。
	Title  string
	Reason string
	// Command 是建议用户执行的命令（不在本应用内运行）。
	Command string
	// Safe 恒为 false：这里的东西不由本应用清理，只作提示。
	Safe bool
}

// systemCommandRules 是「需由包管理器处理」的目标表。
var systemCommandRules = []systemCommandRule{
	{
		ID:       "apt-lists",
		Category: "system-pkg-mgr",
		Path:     "/var/lib/apt/lists",
		Title:    "apt 软件源索引",
		Reason:   "由 apt 管理。直接删除会让 apt 无法判断哪些索引是新的，建议用 apt-get clean 语义清理，需要时 apt-get update 会重建",
		Command:  "apt-get clean && rm -rf /var/lib/apt/lists/*",
	},
}

// systemPathIsForbidden 是系统分区清理的**兜底拒绝名单**。
//
// 即使规则表被误改，这些前缀也永远不允许出现在清理结果里。这是纵深防御的
// 最后一层：系统分区上删错目录的后果（设备无法启动）远比其他分区严重。
var systemPathIsForbidden = []string{
	"/boot",                      // 引导
	"/usr/lib/modules",           // 内核模块（内核清理由专用功能负责）
	"/usr/src",                   // 内核头文件
	"/usr/lib/linux-image-",      // 内核设备树（目录名是 linux-image-<ver>，故带连字符）
	"/lib/modules",               // 内核模块（非 usr-merge 布局）
	"/etc/passwd", "/etc/shadow", // 账号
	"/etc/fstab", "/etc/hosts", // 挂载与解析
	"/var/lib/dpkg",              // 包数据库
	"/var/lib/postgresql",        // 应用数据库
	"/usr/trim",                  // fnOS 系统核心
	"/usr/lib/aarch64-linux-gnu", // 系统库
	"/usr/lib/x86_64-linux-gnu",  // 系统库
	"/usr/local/apps",            // 应用数据
	"/usr/share/doc",             // 文档（B 档，只提示不删）
	"/usr/share/man",             // 手册（B 档，只提示不删）
	"/usr/share/locale",          // 语言包（B 档，只提示不删）
	"/usr/local/go",              // Go 工具链（B 档，只提示不删）
	"/usr/lib/firmware",          // 固件（B 档，只提示不删）
	"/usr/share/AAVMF",           // ARM 虚拟机固件（B 档，只提示不删）
	"/usr/share/proj",            // 被 libproj25 依赖
	"/usr/share/pocketsphinx",    // 被 libpocketsphinx3 依赖
	"/usr/lib/gcc",               // 编译工具链
	"/boot/extlinux",             // 引导配置
}

// systemPathForbidden 报告路径是否落在兜底拒绝名单内。
//
// 匹配按**路径段**而非裸字符串前缀：拒绝 `/boot` 时要命中 `/boot/vmlinuz`，
// 但不能误伤 `/bootleg`。以 `-` 结尾的项（`/usr/lib/linux-image-`）本身就是
// 前缀而非完整目录名，按字符串前缀匹配。
func systemPathForbidden(absPath string) bool {
	for _, p := range systemPathIsForbidden {
		if absPath == p || strings.HasPrefix(absPath, p+"/") {
			return true
		}
		if strings.HasSuffix(p, "-") && strings.HasPrefix(absPath, p) {
			return true
		}
	}
	return false
}

// matchSystemRule 用系统规则表校验一个绝对路径。
//
// 与 matchRuleForPath 同样的安全契约：只返回 Safe 规则命中的路径，
// 调用方必须把前端输入当作不可信数据。
func matchSystemRule(absPath string) (spaceRule, bool) {
	safe := normalizeVolPath(absPath)
	if safe == "" {
		return spaceRule{}, false
	}
	// 兜底拒绝名单优先于规则表。
	if systemPathForbidden(safe) {
		return spaceRule{}, false
	}
	// 拒绝符号链接本身及其路径中的任何符号链接组件。系统分区上尤其重要：
	// /var/log 等目录常有指向别处的软链接（例如 /var/log -> /tmp/log）。
	if isAnySymlinkComponent(safe) {
		return spaceRule{}, false
	}

	for _, r := range systemPathRules {
		if matchSegments(splitRel(r.Pattern), splitRel(safe)) {
			return spaceRule{
				Pattern:  r.Pattern,
				Category: r.Category,
				Safe:     true,
				Strategy: r.Strategy,
				Reason:   r.Reason,
			}, true
		}
	}
	return spaceRule{}, false
}

// matchSystemTempOrRule 在系统临时目录规则与系统分区规则之间做派发。
func matchSystemPath(absPath string) (spaceRule, bool) {
	if rule, ok := matchSystemRule(absPath); ok {
		return rule, true
	}
	return matchSystemTempForPath(absPath)
}

// systemRootDanglingNotice 报告 /boot/vmlinuz.old 这类悬空符号链接。
//
// 它是旧内核被删除后遗留的断链，本身无害但也无用。之所以只提示不清理：
// /boot 属引导分区，任何删除都必须与内核清理功能一起做
// （那里有 uname -r + extlinux.conf 的双重校验），通用规则表不得触及。
func systemRootDanglingNotice() []SpaceNotice {
	const p = "/boot/vmlinuz.old"
	fi, err := os.Lstat(p)
	if err != nil {
		return nil
	}
	if fi.Mode()&os.ModeSymlink == 0 {
		return nil
	}
	if _, err := os.Stat(p); err == nil {
		// 链接目标仍存在，不是悬空链接。
		return nil
	}
	return []SpaceNotice{{
		Name:          "悬空的内核符号链接",
		Path:          p,
		Size:          0,
		SizeFormatted: "0 B",
		Reason:        "指向已删除旧内核的断链。/boot 属引导分区，本应用不在此处执行删除；如需清理请与内核清理一并确认引导配置后再操作",
	}}
}

// systemRotatedLogRe 判断一个 /var/log 直接子项是否是「已轮转归档」的日志，
// 也就是安全可删的那些。
//
// 接受两种形式：
//
//	<name>.<数字>              例：syslog.1、kern.log.4
//	<name>.<数字>.<压缩后缀>    例：syslog.2.gz、auth.log.3.xz
//	<name>.<压缩后缀>          例：messages.gz
//	<name>.<数字>.old          例：daemon.log.1.old
//
// 刻意**不**接受当前正在写入的裸文件名（syslog、kern.log）：它们由 truncate
// 规则处理，不在这里删除。这个区分是本表最容易搞错的地方，
// 因此抽成独立纯函数以便直接测试。
func systemRotatedLogRe(name string) bool {
	if name == "" || strings.HasPrefix(name, ".") {
		return false
	}
	lower := strings.ToLower(name)

	// 先剥离压缩/备份后缀（可叠加，如 .1.gz）。
	body := lower
	for {
		stripped := false
		for _, s := range []string{".gz", ".xz", ".bz2", ".zst", ".old"} {
			if strings.HasSuffix(body, s) && len(body) > len(s) {
				body = strings.TrimSuffix(body, s)
				stripped = true
				break
			}
		}
		if !stripped {
			break
		}
	}

	// body 现在应是 <name> 或 <name>.<数字>。
	// 取最后一段：若为纯数字，则它是轮转序号；否则要求 body != lower
	// （即至少剥离过一个后缀，说明它是归档文件而非正在写入的裸文件）。
	idx := strings.LastIndex(body, ".")
	if idx > 0 && idx < len(body)-1 {
		num := body[idx+1:]
		if isAllDigits(num) {
			return true
		}
	}

	// 没有轮转序号：只有在剥离过后缀的情况下才算归档文件。
	// 这样 "syslog"（未剥离）被拒绝，而 "messages.gz"（剥离了 .gz）被接受。
	return body != lower && body != ""
}

// isAllDigits 报告 s 非空且全为 ASCII 数字。
func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// systemChildIsRotatedLog 报告某个 /var/log 直接子项是否可作为「已轮转日志」清理。
//
// 独立出来是因为这里有一个容易忽略的陷阱：/var/log 下还有**目录**
// （samba/、sysstat/、journal/ 等），它们各自有更精确的规则或属于提示项，
// 绝不能被 DirectChildren 规则误当成文件删掉。
func systemChildIsRotatedLog(path string) bool {
	fi, err := os.Lstat(path)
	if err != nil {
		return false
	}
	if fi.IsDir() || fi.Mode()&os.ModeSymlink != 0 {
		return false
	}
	return systemRotatedLogRe(filepath.Base(path))
}

// scanSystemPathItems 扫描系统分区规则表，产出分类与提示。
//
// 与 /volN 规则表不同，这里的路径是绝对的、不按卷展开。
func scanSystemPathItems() (map[string][]SpaceItem, []SpaceNotice) {
	items := map[string][]SpaceItem{}
	var notices []SpaceNotice

	for _, r := range systemPathRules {
		if r.DirectChildren {
			entries, err := os.ReadDir(r.Pattern)
			if err != nil {
				continue
			}
			for _, e := range entries {
				full := filepath.Join(r.Pattern, e.Name())
				if !systemChildIsRotatedLog(full) {
					continue
				}
				if it, ok := buildSystemItem(full, r); ok {
					items[r.Category] = append(items[r.Category], it)
				}
			}
			continue
		}

		// 含 `*` 的模式：只展开一层（本表的 `*` 都表示"某目录下的直接子项"）。
		if strings.Contains(r.Pattern, "*") {
			dir := r.Pattern[:strings.LastIndex(r.Pattern, "/")]
			seg := r.Pattern[strings.LastIndex(r.Pattern, "/")+1:]
			entries, err := os.ReadDir(dir)
			if err != nil {
				continue
			}
			for _, e := range entries {
				name := e.Name()
				if !globSegment(seg, name) {
					continue
				}
				full := filepath.Join(dir, name)
				if it, ok := buildSystemItem(full, r); ok {
					items[r.Category] = append(items[r.Category], it)
				}
			}
			continue
		}

		if it, ok := buildSystemItem(r.Pattern, r); ok {
			items[r.Category] = append(items[r.Category], it)
		}
	}

	// 「需包管理器处理」的目标只作提示，不参与清理。
	for _, c := range systemCommandRules {
		if c.Safe {
			continue
		}
		size, _, _, _, _ := spacePathStats(c.Path, spaceNoticeEntryCap)
		if size <= 0 {
			continue
		}
		notices = append(notices, SpaceNotice{
			Name:          c.Title,
			Path:          c.Path,
			Size:          size,
			SizeFormatted: utils.FormatBytes(size),
			Reason:        c.Reason + "（建议命令：" + c.Command + "）",
		})
	}

	// B 档：占用大但属功能性组件，只提示不删。
	notices = append(notices, scanSystemBGradeNotices()...)

	// 引导分区上的悬空链接（只提示，不在本表清理范围内）。
	notices = append(notices, systemRootDanglingNotice()...)

	for cat := range items {
		sort.Slice(items[cat], func(i, j int) bool {
			return items[cat][i].Size > items[cat][j].Size
		})
	}
	return items, notices
}

// systemBGradePaths 是「可删但需确认」的 B 档条目，只提示不清理。
var systemBGradePaths = []struct {
	Path   string
	Name   string
	Reason string
}{
	{Path: "/usr/share/AAVMF", Name: "ARM 虚拟机 UEFI 固件",
		Reason: "本机无任何虚拟机时可省，但将来建 ARM 虚拟机就需要；属功能组件，请自行确认后再处理"},
	{Path: "/usr/share/locale", Name: "非中英文语言包",
		Reason: "保留 zh_CN/en 可省大部分空间，但手工删除会让 dpkg -V 报缺失；建议用 localepurge"},
	{Path: "/usr/share/doc", Name: "软件包文档",
		Reason: "纯文档，但手工删除会让 dpkg -V 报缺失"},
	{Path: "/usr/share/man", Name: "手册页",
		Reason: "纯文档，但手工删除会让 dpkg -V 报缺失"},
	{Path: "/usr/local/go", Name: "Go 工具链",
		Reason: "仅在需要从源码编译 Go 程序时使用；当前无 go 进程"},
	{Path: "/usr/lib/firmware", Name: "设备固件",
		Reason: "本机无无线接口时可省，但插入 USB 无线网卡就需要；属功能性组件"},
	{Path: "/usr/lib/gcc", Name: "GCC 工具链",
		Reason: "仅在本地编译时使用"},
	{Path: "/usr/share/proj", Name: "PROJ 坐标系数据",
		Reason: "被 libproj25 依赖，可能被相册定位功能使用"},
	{Path: "/usr/share/pocketsphinx", Name: "语音识别语言模型",
		Reason: "被 libpocketsphinx3 依赖，删除会连带卸载依赖链"},
}

// scanSystemBGradeNotices 统计 B 档路径体积，作为提示项返回。
func scanSystemBGradeNotices() []SpaceNotice {
	var out []SpaceNotice
	for _, b := range systemBGradePaths {
		size, _, _, _, _ := spacePathStats(b.Path, spaceNoticeEntryCap)
		if size <= 0 {
			continue
		}
		out = append(out, SpaceNotice{
			Name:          b.Name,
			Path:          b.Path,
			Size:          size,
			SizeFormatted: utils.FormatBytes(size),
			Reason:        b.Reason,
		})
	}
	return out
}

// buildSystemItem 为一条系统分区路径构造条目。
func buildSystemItem(path string, r systemPathRule) (SpaceItem, bool) {
	fi, err := os.Lstat(path)
	if err != nil {
		return SpaceItem{}, false
	}
	// 符号链接一律不接受。
	if fi.Mode()&os.ModeSymlink != 0 {
		return SpaceItem{}, false
	}
	// 兜底拒绝名单。
	if systemPathForbidden(path) {
		return SpaceItem{}, false
	}
	// 空文件没有清理价值，不展示。
	if !fi.IsDir() && fi.Size() == 0 {
		return SpaceItem{}, false
	}

	size, files, _, modTime, _ := spacePathStats(path, spaceScanEntryCap)

	// MaxAge：日志类目标要求闲置足够久。
	if r.MaxAge > 0 && !modTime.IsZero() && time.Since(modTime) < r.MaxAge {
		return SpaceItem{}, false
	}

	return SpaceItem{
		Path:          path,
		Name:          filepath.Base(path),
		Size:          size,
		SizeFormatted: utils.FormatBytes(size),
		IsDir:         fi.IsDir(),
		FileCount:     files,
		Reason:        r.Reason,
		Safe:          true,
		Strategy:      r.Strategy,
		Modified:      modTime,
	}, true
}
