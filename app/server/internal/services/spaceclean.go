package services

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/sushazhi/fnos-logmanager/internal/utils"
)

// 系统空间清理（Space Clean）。
//
// 与既有的「日志清理」「已卸载应用残留清理」不同，这里处理的是**可再生缓存**：
// 包管理器 store、浏览器缓存、缩略图、安装包、容器日志等。它们体积大、删掉不影响
// 业务数据，但分散在 @appdata / docker / appcenter-downloads 等多个位置，
// 用户很难手工找到。
//
// 安全模型（三层，缺一不可）：
//
//  1. **规则表驱动**：条目不是「遍历全盘找大目录」，而是由固定规则表（pattern +
//     safe 标记）精确展开。规则里的目录名是**逐段精确匹配**，不做前缀剪枝，
//     避免误伤同名前缀的无关目录。
//  2. **服务端二次校验**：清理接口不信任前端传来的路径，落盘前用同一张规则表
//     重新匹配 `matchRuleForPath`，只有命中且 safe 的路径才会被删除。
//  3. **删除方式分级**：纯缓存直接删除（立即释放空间），有留存价值的数据
//     （数据库备份）移入回收站，Docker 容器日志只做截断（移动会破坏 dockerd 的 fd）。
//
// `runtime-deps` 这类删掉会导致应用起不来的目录被显式标记为 Safe=false，
// 只做展示不参与清理。

// ---------------------------------------------------------------------------
// 对外结构
// ---------------------------------------------------------------------------

// SpaceItem 是一条可清理的具体条目（一个目录或一个文件）。
type SpaceItem struct {
	Path          string `json:"path"`
	DisplayPath   string `json:"displayPath,omitempty"`
	Name          string `json:"name"`
	Size          int64  `json:"size"`
	SizeFormatted string `json:"sizeFormatted"`
	IsDir         bool   `json:"isDir"`
	FileCount     int64  `json:"fileCount"`
	Reason        string `json:"reason"`
	// Safe 为 false 时不可删除，仅作为「占用大但不建议删」的提示展示。
	Safe bool `json:"safe"`
	// InUse 表示该条目近期仍被写入（很可能正被运行中的应用使用），
	// 因此本轮不提供删除。这是运行期探测结果，不代表路径本身有风险。
	InUse bool `json:"inUse"`
	// Strategy 决定清理方式：delete（直接删除）/ recycle（移入回收站）/ truncate（截断）。
	Strategy string `json:"strategy"`
	// Modified 是条目的最后修改时间，便于用户判断是否还在被写入。
	Modified time.Time `json:"modified"`
}

// SpaceCategory 是一组同类条目。
type SpaceCategory struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	// Risk 取值 low / medium / high，用于前端配色与默认勾选策略。
	Risk      string `json:"risk"`
	Icon      string `json:"icon"`
	ItemCount int    `json:"itemCount"`
	// TotalSize 是该分类下所有条目之和（含不可删除项）。
	TotalSize          int64  `json:"totalSize"`
	TotalSizeFormatted string `json:"totalSizeFormatted"`
	// Reclaimable 只统计 Safe 条目，也就是真正能释放的空间。
	Reclaimable          int64       `json:"reclaimable"`
	ReclaimableFormatted string      `json:"reclaimableFormatted"`
	Items                []SpaceItem `json:"items"`
}

// SpaceNotice 是「占用很大但不能删」的提示项，解释为什么不能删。
type SpaceNotice struct {
	Name          string `json:"name"`
	Path          string `json:"path"`
	Size          int64  `json:"size"`
	SizeFormatted string `json:"sizeFormatted"`
	Reason        string `json:"reason"`
	Truncated     bool   `json:"truncated"`
}

// VolumeUsage 是单个存储卷的使用情况，用于前端可视化容量条。
type VolumeUsage struct {
	Mount          string  `json:"mount"`
	Total          int64   `json:"total"`
	Used           int64   `json:"used"`
	Free           int64   `json:"free"`
	UsedPercent    float64 `json:"usedPercent"`
	TotalFormatted string  `json:"totalFormatted"`
	UsedFormatted  string  `json:"usedFormatted"`
	FreeFormatted  string  `json:"freeFormatted"`
}

// SpaceUsageResult 是整次扫描的结果。
type SpaceUsageResult struct {
	ScannedAt           time.Time       `json:"scannedAt"`
	DurationMs          int64           `json:"durationMs"`
	Volumes             []VolumeUsage   `json:"volumes"`
	Categories          []SpaceCategory `json:"categories"`
	Notices             []SpaceNotice   `json:"notices"`
	Reclaimable         int64           `json:"reclaimable"`
	ReclaimableText     string          `json:"reclaimableFormatted"`
	CategoriesWithItems int             `json:"categoriesWithItems"`
	// TempOnSystemDisk 说明 /tmp、/var/tmp 是否与系统分区同处一个文件系统。
	// nil 表示无法判定——此时前端不能显示"/tmp 位于系统分区"这类断言。
	TempOnSystemDisk *bool    `json:"tempOnSystemDisk,omitempty"`
	Errors           []string `json:"errors,omitempty"`
}

// SpaceCleanItemResult 是单条目的清理结果。
type SpaceCleanItemResult struct {
	Path       string `json:"path"`
	Status     string `json:"status"` // deleted | recycled | truncated | skipped | failed
	FreedBytes int64  `json:"freedBytes"`
	Message    string `json:"message,omitempty"`
}

// SpaceCleanResult 是一次清理操作的汇总。
type SpaceCleanResult struct {
	Freed          int64                  `json:"freed"`
	FreedFormatted string                 `json:"freedFormatted"`
	Deleted        int                    `json:"deleted"`
	Recycled       int                    `json:"recycled"`
	Truncated      int                    `json:"truncated"`
	Failed         int                    `json:"failed"`
	DryRun         bool                   `json:"dryRun"`
	Items          []SpaceCleanItemResult `json:"items"`
	Errors         []string               `json:"errors,omitempty"`
}

// ---------------------------------------------------------------------------
// 规则表
// ---------------------------------------------------------------------------

// spaceRule 描述一类可清理目标。
//
// Pattern 是相对存储卷根的路径，用 `/` 分段；`*` 精确匹配**一段**。
// 例如 `@appdata/*/pnpm-home/store` 只会命中 /volN/@appdata/<app>/pnpm-home/store。
//
// 规则必须精确到「确实可重建的那一层」。真机上出现过两类陷阱目录，
// 名字看着像缓存、里面装的却是下载好的二进制，**不能**用宽规则整目录删：
//
//   - @appdata/{app}/config/.browser/cloakbrowser/chromium-<版本>/
//     真机实测 685 MB / 672 个文件，是装好的 Chromium 本体；
//   - @appdata/{app}/home/.cache/puppeteer/{chrome,chrome-headless-shell}/
//     真机实测 691 MB，是 puppeteer 下载的浏览器。
//
// 这些删掉后需要联网重新下载才能恢复，离线设备会直接坏掉，因此它们只作为
// 「占用大但不可删除」的提示项展示（见 spaceNoticePatterns），不进本规则表。
// 与之相对，home/.cache 下真正可重建的部分（pnpm、pip、node-gyp、go-build）
// 用逐个子目录的窄规则命中，不整体删 home/.cache。
type spaceRule struct {
	Pattern  string
	Category string
	// Safe=false 的条目只展示、不清理。
	Safe bool
	// Strategy 见 SpaceItem.Strategy。
	Strategy string
	Reason   string
}

// spaceCategoryDef 是一个分类的元数据。
type spaceCategoryDef struct {
	ID          string
	Name        string
	Description string
	Risk        string
	Icon        string
}

// spaceCategoryOrder 决定前端的展示顺序（低风险在前，便于用户先清理安全的）。
var spaceCategoryOrder = []string{
	"app-center-downloads",
	"pkg-cache",
	"browser-cache",
	"thumb-cache",
	"app-temp",
	"docker-logs",
	"app-cache",
	"container-cache",
	"db-backup",
	"system-pkg-cache",
	"system-logs",
	"system-residue",
	"system-pkg-mgr",
	"system-temp",
	"runtime-deps",
}

var spaceCategoryDefs = map[string]spaceCategoryDef{
	"app-center-downloads": {
		ID:          "app-center-downloads",
		Name:        "应用中心安装包",
		Description: "应用中心下载/上传安装包后的残留。应用安装完成后不再需要，可重新下载。",
		Risk:        "low",
		Icon:        "package",
	},
	"pkg-cache": {
		ID:          "pkg-cache",
		Name:        "包管理器缓存",
		Description: "pnpm store、npm _cacache/_npx 等依赖缓存。删除后安装依赖会重新下载，不影响已装依赖（已通过硬链接引用）。",
		Risk:        "low",
		Icon:        "layers",
	},
	"browser-cache": {
		ID:          "browser-cache",
		Name:        "运行时下载缓存",
		Description: "应用 home 目录下 pnpm / pip / node-gyp / go-build 的下载与编译缓存。删除后下次安装或编译时自动重新下载，不影响已安装内容。",
		Risk:        "low",
		Icon:        "globe",
	},
	"thumb-cache": {
		ID:          "thumb-cache",
		Name:        "缩略图缓存",
		Description: "相册/文件管理生成的缩略图。删除后浏览时会自动重新生成。",
		Risk:        "low",
		Icon:        "image",
	},
	"app-temp": {
		ID:          "app-temp",
		Name:        "应用临时目录",
		Description: "@apptemp 下的临时数据。应用重启后不再需要。",
		Risk:        "low",
		Icon:        "trash",
	},
	"docker-logs": {
		ID:          "docker-logs",
		Name:        "Docker 容器日志",
		Description: "容器 stdout/stderr 日志（*-json.log）。采用「截断」而非删除，容器无需重启即可释放空间。",
		Risk:        "low",
		Icon:        "container",
	},
	"app-cache": {
		ID:          "app-cache",
		Name:        "应用运行缓存",
		Description: "应用自身声明的可重建缓存目录。",
		Risk:        "low",
		Icon:        "database",
	},
	"container-cache": {
		ID:          "container-cache",
		Name:        "容器依赖缓存",
		Description: "容器内应用（如青龙）的依赖缓存。删除后容器下次执行任务会重新安装依赖。",
		Risk:        "medium",
		Icon:        "box",
	},
	"db-backup": {
		ID:          "db-backup",
		Name:        "应用数据库备份",
		Description: "应用自动生成的数据库备份。删除会丢失历史备份，因此移入回收站而非直接删除，可在保留期内还原。",
		Risk:        "medium",
		Icon:        "save",
	},
	"system-temp": {
		ID:          "system-temp",
		Name:        "系统临时目录",
		Description: "系统临时目录 /tmp 与 /var/tmp 中的条目。只有在「没有任何进程正在使用」且「闲置超过 24 小时」时才提供删除。这两处不属于任何存储卷，扫描 /volN 不会看到它们，因此单独列出。",
		Risk:        "medium",
		Icon:        "clock",
	},
	"system-pkg-cache": {
		ID:          "system-pkg-cache",
		Name:        "系统包缓存（系统分区）",
		Description: "系统分区上 apt 的二进制/源码索引缓存。删除后 apt-get update 会自动重建，不影响已安装的软件包。真机实测这两项合计约 77 MB。",
		Risk:        "low",
		Icon:        "package",
	},
	"system-logs": {
		ID:          "system-logs",
		Name:        "系统日志（系统分区）",
		Description: "系统分区上的系统与服务日志。正在写入的日志采用「截断」而非删除——rsyslog 等持有打开的文件描述符，删除后空间不会立即释放且日志会消失；已轮转归档的历史日志则直接删除。",
		Risk:        "low",
		Icon:        "file-text",
	},
	"system-residue": {
		ID:          "system-residue",
		Name:        "系统零散残留",
		Description: "dpkg 升级时留下的旧配置副本（.dpkg-dist）、软件包的旧版本备份、悬空的内核符号链接等。体积很小，但属于确定无用的残留。",
		Risk:        "low",
		Icon:        "trash",
	},
	"system-pkg-mgr": {
		ID:          "system-pkg-mgr",
		Name:        "需包管理器处理（仅提示）",
		Description: "由 apt/dpkg 管理的目录（如 /var/lib/apt/lists）。绕过包管理器直接删除会让包数据库与磁盘不一致，因此本应用不代为清理，只给出建议命令，由你在设备上执行。",
		Risk:        "medium",
		Icon:        "terminal",
	},
	"runtime-deps": {
		ID:          "runtime-deps",
		Name:        "应用运行期依赖（不建议删除）",
		Description: "应用运行所依赖的 node_modules。删除会导致对应应用无法启动，需重装应用才能恢复。仅展示，不参与清理。",
		Risk:        "high",
		Icon:        "alert",
	},
}

// spaceRules 是唯一的清理目标定义表。新增清理目标只应改这里。
var spaceRules = []spaceRule{
	// 应用中心安装包残留（整目录删除；不再展开子项，避免同一体积被计两次）
	{Pattern: "appcenter-downloads/*", Category: "app-center-downloads", Safe: true, Strategy: "delete",
		Reason: "应用中心安装包残留，安装完成后已无用"},

	// Docker 容器日志：只截断。删除文件会让 dockerd 继续往已 unlink 的 inode 写，
	// 空间不会立即释放且日志从此丢失；截断则容器无需重启即可回收空间。
	{Pattern: "docker/containers/*/*-json.log", Category: "docker-logs", Safe: true, Strategy: "truncate",
		Reason: "Docker 容器 stdout/stderr 日志，截断后容器无需重启即可释放空间"},

	// Node 包管理器缓存
	{Pattern: "@appdata/*/pnpm-home/store", Category: "pkg-cache", Safe: true, Strategy: "delete",
		Reason: "pnpm 内容寻址 store，已装依赖通过硬链接引用，删除不影响运行"},
	{Pattern: "@appdata/*/npm-cache/_cacache", Category: "pkg-cache", Safe: true, Strategy: "delete",
		Reason: "npm 下载缓存，可重新下载"},
	{Pattern: "@appdata/*/npm-cache/_npx", Category: "pkg-cache", Safe: true, Strategy: "delete",
		Reason: "npx 临时安装包缓存"},
	{Pattern: "@appdata/*/npm-cache/_logs", Category: "pkg-cache", Safe: true, Strategy: "delete",
		Reason: "npm 调试日志"},
	// 注意：**没有** `@appdata/*/pnpm-env` 规则。真机实测该目录是 pnpm 工具本体
	// （package.json + node_modules/.bin/{pnpm,pnpx,pnx,pn}），不是缓存；
	// 删掉后所有依赖安装都会失败，且不会自动重建。它只作为提示项展示。
	{Pattern: "@appdata/*/home/.npm", Category: "pkg-cache", Safe: true, Strategy: "delete",
		Reason: "用户级 npm 缓存"},

	// 应用内置浏览器缓存。
	//
	// 这里刻意**不**写 `@appdata/*/home/.cache` 和 `@appdata/*/config/.browser`
	// 这两条宽规则：真机实测它们里面装着下载好的浏览器二进制，不是缓存。
	//   - /vol1/@appdata/moviepilot/config/.browser/cloakbrowser/chromium-146.x
	//     685 MB / 672 文件；
	//   - /vol1/@appdata/deepseek.harness/home/.cache/puppeteer/{chrome,
	//     chrome-headless-shell} 691 MB。
	// 删掉后需联网重新下载才能恢复，只作为提示项展示。
	// 真正可重建的包管理器缓存逐个窄匹配，既安全又能拿到同样的空间。
	{Pattern: "@appdata/*/home/.cache/pnpm", Category: "browser-cache", Safe: true, Strategy: "delete",
		Reason: "pnpm 下载缓存，缺少的包会自动重新下载"},
	{Pattern: "@appdata/*/home/.cache/pip", Category: "browser-cache", Safe: true, Strategy: "delete",
		Reason: "pip 下载缓存，缺少的包会自动重新下载"},
	{Pattern: "@appdata/*/home/.cache/node-gyp", Category: "browser-cache", Safe: true, Strategy: "delete",
		Reason: "node-gyp 编译缓存，需要时自动重新编译"},
	{Pattern: "@appdata/*/home/.cache/go-build", Category: "browser-cache", Safe: true, Strategy: "delete",
		Reason: "Go 编译缓存，需要时自动重新编译"},

	// 缩略图
	{Pattern: "thumb/*", Category: "thumb-cache", Safe: true, Strategy: "delete",
		Reason: "缩略图缓存，浏览时自动重建"},

	// 应用临时目录
	{Pattern: "@apptemp/*", Category: "app-temp", Safe: true, Strategy: "delete",
		Reason: "应用临时目录，应用重启后不再需要"},

	// 应用运行缓存
	{Pattern: "@appdata/*/config/cache", Category: "app-cache", Safe: true, Strategy: "delete",
		Reason: "应用声明可重建的缓存目录"},

	// 容器依赖缓存（用户目录下的 docker 应用数据）
	{Pattern: "*/AppData/*/dep_cache", Category: "container-cache", Safe: true, Strategy: "delete",
		Reason: "容器应用依赖缓存，下次执行任务时重新安装"},

	// 数据库备份（移入回收站，保留可还原）
	{Pattern: "@appdata/*/config/database_backup", Category: "db-backup", Safe: true, Strategy: "recycle",
		Reason: "应用数据库备份，移入回收站后仍可在保留期内还原"},

	// 运行期依赖（不可删除）
	{Pattern: "@appdata/*/dsh-runtime/node_modules", Category: "runtime-deps", Safe: false, Strategy: "none",
		Reason: "应用运行期依赖，删除后应用无法启动，需重装应用才能恢复"},
	{Pattern: "@appdata/*/node_modules", Category: "runtime-deps", Safe: false, Strategy: "none",
		Reason: "应用运行期依赖，删除后应用无法启动，需重装应用才能恢复"},
}

// spaceNoticePatterns 是「占用大但不可删除」的提示项定义。
var spaceNoticePatterns = []struct {
	Pattern string
	Name    string
	Reason  string
}{
	{Pattern: "@sysappmeta/ai-manager/models", Name: "AI 智能助手模型",
		Reason: "系统 AI 服务正在使用，删除会导致智能识别相关功能不可用"},
	{Pattern: "@appcenter/trim.ai-runtime-*/models", Name: "AI 运行库模型",
		Reason: "系统 AI 运行时依赖，删除会导致相关功能不可用"},
	{Pattern: "@appcenter/*/python", Name: "应用内置 Python 运行时",
		Reason: "已安装应用的运行环境，删除会导致应用无法启动"},
	{Pattern: "docker/overlay2", Name: "Docker 镜像层",
		Reason: "容器运行时数据，请通过 Docker 管理器删除未使用镜像，不要手工删除"},
	{Pattern: "docker/tmp", Name: "Docker 临时数据",
		Reason: "容器运行时临时数据，可能正被运行中的容器使用"},

	// 名字像缓存、实为已下载好的浏览器二进制。真机实测两者各占 685 MB / 691 MB，
	// 删掉后必须联网重新下载才能恢复，因此只提示不清理。
	{Pattern: "@appdata/*/config/.browser", Name: "应用内置 Chromium 浏览器",
		Reason: "这是已经下载安装好的 Chromium 本体（真机实测约 685 MB），不是缓存；删除后需要联网重新下载，离线设备会导致应用无法工作"},
	{Pattern: "@appdata/*/home/.cache/puppeteer", Name: "Puppeteer 下载的浏览器",
		Reason: "这是 puppeteer 已下载的 Chrome（真机实测约 691 MB），不是可再生的缓存；删除后需要联网重新下载"},
	{Pattern: "@appdata/*/pnpm-env", Name: "pnpm 工具本体",
		Reason: "这是 pnpm 包管理器自身的安装目录（node_modules/.bin/pnpm），不是缓存；删除后依赖安装会失败，且不会自动重建"},
}

// ---------------------------------------------------------------------------
// 扫描
// ---------------------------------------------------------------------------

var (
	spaceScanMu     sync.Mutex
	spaceScanCache  *SpaceUsageResult
	spaceScanCached time.Time
)

// spaceScanCacheTTL 是扫描结果缓存时长。扫描涉及大量 stat，面板内多次刷新
// 不应重复全量遍历；清理操作会主动失效缓存。
const spaceScanCacheTTL = 30 * time.Second

// spaceScanWorkers 是分类间并行扫描的并发上限。
const spaceScanWorkers = 4

// spaceScanEntryCap 是单条目统计的遍历上限，防止异常目录树拖死扫描。
const spaceScanEntryCap = 400000

// spaceNoticeEntryCap 是提示项体积统计的遍历上限（提示项数量多，取更保守的上限）。
const spaceNoticeEntryCap = 20000

// InvalidateSpaceScanCache 清空空间扫描缓存。任何清理操作之后都应调用。
func InvalidateSpaceScanCache() {
	spaceScanMu.Lock()
	spaceScanCache = nil
	spaceScanMu.Unlock()
}

// spaceVolumes 返回宿主机上的存储卷根目录（/volN）。
func spaceVolumes() []string {
	entries, err := os.ReadDir("/")
	if err != nil {
		return nil
	}
	var vols []string
	for _, e := range entries {
		if e.IsDir() && volRe.MatchString("/"+e.Name()+"/") {
			vols = append(vols, "/"+e.Name())
		}
	}
	sort.Strings(vols)
	return vols
}

// globSegment 判断单个路径段是否匹配带 `*` 的模式（`*` 匹配任意子串）。
func globSegment(pattern, name string) bool {
	if pattern == "*" {
		return name != ""
	}
	if !strings.Contains(pattern, "*") {
		return pattern == name
	}
	parts := strings.Split(pattern, "*")
	rest := name
	for i, part := range parts {
		if part == "" {
			continue
		}
		idx := strings.Index(rest, part)
		if idx < 0 {
			return false
		}
		// 第一段必须是前缀
		if i == 0 && idx != 0 {
			return false
		}
		rest = rest[idx+len(part):]
	}
	// 最后一段必须是后缀
	last := parts[len(parts)-1]
	return last == "" || strings.HasSuffix(name, last)
}

// matchSegments 判断路径分段是否与模式分段逐段匹配（长度必须相等）。
func matchSegments(pattern, path []string) bool {
	if len(pattern) != len(path) {
		return false
	}
	for i := range pattern {
		if !globSegment(pattern[i], path[i]) {
			return false
		}
	}
	return true
}

// splitRel 把相对卷根的路径拆成非空分段。
func splitRel(p string) []string {
	var out []string
	for _, s := range strings.Split(p, "/") {
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

// expandSpacePattern 把一条规则模式展开为当前文件系统上真实存在的路径。
//
// 逐段下钻：普通段直接拼接并检查存在性，`*` 段读取目录项展开。全程用 Lstat
// 拒绝符号链接，避免通过软链接逃逸到规则之外的位置。
func expandSpacePattern(vol, pattern string) []string {
	segments := splitRel(pattern)
	if len(segments) == 0 {
		return nil
	}

	current := []string{vol}
	for i, seg := range segments {
		last := i == len(segments)-1
		var next []string
		for _, base := range current {
			if strings.Contains(seg, "*") {
				entries, err := os.ReadDir(base)
				if err != nil {
					continue
				}
				for _, e := range entries {
					name := e.Name()
					if !globSegment(seg, name) {
						continue
					}
					full := filepath.Join(base, name)
					if spaceExpandAccept(full, last) {
						next = append(next, full)
					}
				}
				continue
			}

			full := filepath.Join(base, seg)
			if !spaceExpandAccept(full, last) {
				continue
			}
			next = append(next, full)
		}
		current = next
		if len(current) == 0 {
			return nil
		}
	}
	return current
}

// spaceExpandAccept 判断规则展开时是否接受一个候选路径。
//
// 抽成独立函数是为了让"拒绝符号链接"这条安全规则能被直接测试：
// 在 Windows 上创建符号链接需要额外权限，端到端测试会因此被跳过。
func spaceExpandAccept(full string, last bool) bool {
	fi, err := os.Lstat(full)
	if err != nil {
		return false
	}
	return spaceExpandAcceptInfo(fi, last)
}

// spaceExpandAcceptInfo 是 spaceExpandAccept 的纯判定部分：给定文件信息决定取舍。
//
// 三条约束：
//   - 不能是符号链接：跟随链接会把清理范围延伸到规则之外，
//     也可能顺着链接删到别的挂载点；
//   - 是最后一段时，存在即可（普通文件与目录都接受）；
//   - 不是最后一段时必须是目录，否则无法继续向下展开。
//
// 与文件系统读取分离，使符号链接这条约束在任何平台都能被测试到——
// 判定只依赖 os.FileInfo 的 Mode，不需要真的能创建链接。
func spaceExpandAcceptInfo(fi os.FileInfo, last bool) bool {
	if fi == nil {
		return false
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		return false
	}
	if last {
		return true
	}
	return fi.IsDir()
}

// spacePathStats 统计目录/文件的大小、文件数与目录数，跳过符号链接。
func spacePathStats(path string, capEntries int) (size int64, files int64, dirs int64, modTime time.Time, truncated bool) {
	fi, err := os.Lstat(path)
	if err != nil {
		return 0, 0, 0, time.Time{}, false
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		return 0, 0, 0, time.Time{}, false
	}
	modTime = fi.ModTime()
	if !fi.IsDir() {
		return fi.Size(), 1, 0, modTime, false
	}

	count := 0
	_ = filepath.WalkDir(path, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}
		count++
		if count > capEntries {
			truncated = true
			return filepath.SkipAll
		}
		if d.IsDir() {
			dirs++
			return nil
		}
		if info, ierr := d.Info(); ierr == nil {
			size += info.Size()
			files++
		}
		return nil
	})
	return size, files, dirs, modTime, truncated
}

// buildSpaceItem 为一个已匹配的路径构造条目。
func buildSpaceItem(path string, rule spaceRule) SpaceItem {
	size, files, _, modTime, _ := spacePathStats(path, spaceScanEntryCap)
	fi, err := os.Lstat(path)
	isDir := err == nil && fi.IsDir()

	safe := rule.Safe
	reason := rule.Reason
	inUse := false

	// 运行期安全检查：正在运行的应用（或本应用自己）的临时目录不提供删除，
	// 避免清理动作影响正在工作的服务。判定依据是 pid 是否存活（权威信号），
	// 而不是"最近是否被写入"——后者会漏掉空闲但仍在运行的应用。
	if safe {
		if spaceIsSelfTemp(path) {
			safe = false
			inUse = true
			reason = "本应用（logmanager）自己的临时目录，不提供删除"
		} else if spaceIsRunningAppPath(path) {
			safe = false
			inUse = true
			reason = "所属应用当前正在运行，删除临时目录可能影响其正常工作，请先停用该应用"
		}
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
		Strategy:      rule.Strategy,
		Modified:      modTime,
	}
}

// scanSpaceCategories 按规则表扫描并聚合成分类。
func scanSpaceCategories(vols []string) ([]SpaceCategory, []string, []SpaceNotice) {
	type catAcc struct {
		items []SpaceItem
		seen  map[string]bool
	}
	acc := map[string]*catAcc{}
	var mu sync.Mutex
	var errs []string
	// 系统分区规则表产出的「仅提示」条目（包管理器类 + B 档）。
	var extraNotices []SpaceNotice

	// 先展开所有规则的目标路径，再并行统计体积（体积统计是纯 I/O）。
	type job struct {
		path string
		rule spaceRule
	}
	var jobs []job
	seenPath := map[string]bool{}
	for _, vol := range vols {
		for _, rule := range spaceRules {
			for _, p := range expandSpacePattern(vol, rule.Pattern) {
				// 同一路径可能被多条规则命中（如 * 与显式命名），只保留首个。
				if seenPath[p] {
					continue
				}
				seenPath[p] = true
				jobs = append(jobs, job{path: p, rule: rule})
			}
		}
	}

	workers := spaceScanWorkers
	if workers > len(jobs) {
		workers = len(jobs)
	}
	if workers < 1 {
		workers = 1
	}

	indexes := make(chan int)
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range indexes {
				j := jobs[idx]
				item := buildSpaceItem(j.path, j.rule)
				if item.Size == 0 && !item.IsDir {
					continue
				}
				mu.Lock()
				a := acc[j.rule.Category]
				if a == nil {
					a = &catAcc{seen: map[string]bool{}}
					acc[j.rule.Category] = a
				}
				if !a.seen[item.Path] {
					a.seen[item.Path] = true
					a.items = append(a.items, item)
				}
				mu.Unlock()
			}
		}()
	}
	for i := range jobs {
		indexes <- i
	}
	close(indexes)
	wg.Wait()

	// 系统临时目录不在 /volN 下，无法用存储卷规则描述，单独扫描后并入同一份分类。
	if items := scanSystemTempItems(); len(items) > 0 {
		a := &catAcc{seen: map[string]bool{}}
		for _, it := range items {
			if a.seen[it.Path] {
				continue
			}
			a.seen[it.Path] = true
			a.items = append(a.items, it)
		}
		acc["system-temp"] = a
	}

	// 系统分区（/）上的规则表。这些路径同样不在 /volN 下，trim-cli 也无法从
	// 外部枚举，必须由本应用以 root 身份扫描。真机上最紧张的正是这个分区。
	sysItems, sysNotices := scanSystemPathItems()
	for catID, items := range sysItems {
		a := acc[catID]
		if a == nil {
			a = &catAcc{seen: map[string]bool{}}
			acc[catID] = a
		}
		for _, it := range items {
			if a.seen[it.Path] {
				continue
			}
			a.seen[it.Path] = true
			a.items = append(a.items, it)
		}
	}
	// 包管理器类与 B 档只作提示，附加到全局提示列表。
	extraNotices = append(extraNotices, sysNotices...)

	var out []SpaceCategory
	for _, id := range spaceCategoryOrder {
		def, ok := spaceCategoryDefs[id]
		if !ok {
			continue
		}
		a := acc[id]
		cat := SpaceCategory{
			ID:          def.ID,
			Name:        def.Name,
			Description: def.Description,
			Risk:        def.Risk,
			Icon:        def.Icon,
		}
		if a != nil {
			items := a.items
			sort.Slice(items, func(i, j int) bool { return items[i].Size > items[j].Size })
			cat.Items = items
			cat.ItemCount = len(items)
			for _, it := range items {
				cat.TotalSize += it.Size
				if it.Safe {
					cat.Reclaimable += it.Size
				}
			}
		}
		if cat.Items == nil {
			cat.Items = []SpaceItem{}
		}
		cat.TotalSizeFormatted = utils.FormatBytes(cat.TotalSize)
		cat.ReclaimableFormatted = utils.FormatBytes(cat.Reclaimable)
		out = append(out, cat)
	}
	return out, errs, extraNotices
}

// scanSpaceNotices 统计「占用大但不能删」的提示项。
func scanSpaceNotices(vols []string) []SpaceNotice {
	var notices []SpaceNotice
	seen := map[string]bool{}
	for _, vol := range vols {
		for _, def := range spaceNoticePatterns {
			for _, p := range expandSpacePattern(vol, def.Pattern) {
				if seen[p] {
					continue
				}
				seen[p] = true
				size, _, _, _, truncated := spacePathStats(p, spaceNoticeEntryCap)
				if size == 0 {
					continue
				}
				notices = append(notices, SpaceNotice{
					Name:          def.Name,
					Path:          p,
					Size:          size,
					SizeFormatted: utils.FormatBytes(size),
					Reason:        def.Reason,
					Truncated:     truncated,
				})
			}
		}
	}
	sort.Slice(notices, func(i, j int) bool { return notices[i].Size > notices[j].Size })
	return notices
}

// scanVolumeUsage 读取存储卷容量，用于前端容量条可视化。
//
// 除 /volN 外，还额外纳入系统分区（systemDiskMount）：真机上最紧张的恰恰是它
// （6.76 GiB，可用约 2.46 GiB），而 /tmp、/var/tmp 都落在其上。若只显示存储卷，
// 用户会看到 /vol1 还剩 185 GiB，完全意识不到系统分区已经快满。
func scanVolumeUsage(vols []string) []VolumeUsage {
	mounts := append([]string{}, vols...)
	if !containsString(mounts, systemDiskMount) {
		mounts = append(mounts, systemDiskMount)
	}
	var out []VolumeUsage
	for _, v := range mounts {
		total, free, avail, err := diskUsageOf(v)
		if err != nil || total == 0 {
			continue
		}
		used := int64(total - free)
		percent := float64(0)
		if total > 0 {
			percent = float64(used) / float64(total) * 100
		}
		out = append(out, VolumeUsage{
			Mount:          v,
			Total:          int64(total),
			Used:           used,
			Free:           int64(avail),
			UsedPercent:    percent,
			TotalFormatted: utils.FormatBytes(int64(total)),
			UsedFormatted:  utils.FormatBytes(used),
			FreeFormatted:  utils.FormatBytes(int64(avail)),
		})
	}
	return out
}

// containsString 报告切片中是否已包含 s。
func containsString(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// ScanSpaceUsage 扫描系统空间占用，返回分类、提示与容量信息。
//
// 结果按 TTL 缓存；清理操作会失效缓存。
func ScanSpaceUsage(force bool) (SpaceUsageResult, error) {
	spaceScanMu.Lock()
	if !force && spaceScanCache != nil && time.Since(spaceScanCached) < spaceScanCacheTTL {
		cached := *spaceScanCache
		spaceScanMu.Unlock()
		return cached, nil
	}
	spaceScanMu.Unlock()

	start := time.Now()
	vols := spaceVolumes()
	if len(vols) == 0 {
		return SpaceUsageResult{}, fmt.Errorf("未检测到存储卷（/volN）")
	}

	categories, errs, sysNotices := scanSpaceCategories(vols)
	notices := scanSpaceNotices(vols)
	// 系统分区产出的提示（包管理器类 + B 档功能组件）并入同一列表。
	notices = append(notices, sysNotices...)
	volumes := scanVolumeUsage(vols)

	result := SpaceUsageResult{
		ScannedAt:  time.Now(),
		DurationMs: time.Since(start).Milliseconds(),
		Volumes:    volumes,
		Categories: categories,
		Notices:    notices,
		Errors:     errs,
	}
	// 只有确实读到了设备号才给出结论；读不到就留 nil，前端据此不显示断言。
	if same, known := systemTempOnSystemDisk(); known {
		result.TempOnSystemDisk = &same
	}
	for _, c := range categories {
		result.Reclaimable += c.Reclaimable
		if c.ItemCount > 0 {
			result.CategoriesWithItems++
		}
	}
	result.ReclaimableText = utils.FormatBytes(result.Reclaimable)

	spaceScanMu.Lock()
	spaceScanCache = &result
	spaceScanCached = time.Now()
	spaceScanMu.Unlock()

	slog.Debug("space scan done",
		"volumes", len(volumes),
		"categories", len(categories),
		"reclaimable", result.Reclaimable,
		"duration", time.Since(start))

	return result, nil
}

// spaceSelfTempPrefixes are temp paths owned by this app itself.
//
// /volN/@apptemp/logmanager 是本应用自己的临时目录：正在运行的 logmanager 可能
// 正从中读写，把它清理掉属于「自己删自己」，因此在扫描阶段就排除。
var spaceSelfTempPrefixes = []string{
	"@apptemp/logmanager",
	"@apptemp/trim.logmanager",
}

// spaceIsSelfTemp 判断路径是否属于本应用的临时目录。
func spaceIsSelfTemp(absPath string) bool {
	vol := volRootOf(absPath)
	if vol == "" {
		return false
	}
	rel := strings.TrimPrefix(absPath, "/"+vol+"/")
	for _, p := range spaceSelfTempPrefixes {
		if rel == p || strings.HasPrefix(rel, p+"/") {
			return true
		}
	}
	return false
}

// spaceIsRunningAppPath 判断路径是否属于某个「正在运行」的应用。
//
// 依据是该应用在 @appdata/{app}（即 TRIM_PKGVAR）或 /var/apps/{app} 下写了
// pid 文件且该 pid 仍存活。@apptemp 里同名目录属于该应用的运行期临时区，
// 删除运行中应用的临时目录可能导致其异常。
func spaceIsRunningAppPath(absPath string) bool {
	vol := volRootOf(absPath)
	if vol == "" {
		return false
	}
	rel := strings.TrimPrefix(absPath, "/"+vol+"/")
	if !strings.HasPrefix(rel, "@apptemp/") {
		return false
	}
	app := strings.TrimPrefix(rel, "@apptemp/")
	if app == "" || strings.Contains(app, "/") {
		return false
	}
	return appProcessAlive(vol, app)
}

// CurrentUID returns the effective process uid as a decimal string. Used as the
// recycle-bin owner when the request carries no fnOS gateway uid (standalone
// mode). Returns "" on platforms without a uid concept.
func CurrentUID() string {
	return currentUID()
}

// ---------------------------------------------------------------------------
// 清理
// ---------------------------------------------------------------------------

// isAnySymlinkComponent reports whether p itself, or any of its path
// components, is a symbolic link.
//
// 与 utils.IsSymlinkPath 的区别：这里在遇到不存在的组件时立即返回 false 而不是
// 继续尝试。清理目标可能已经不存在（例如用户在两个标签页里重复清理），此时不应
// 把它当成「非法路径」——调用方会另行 stat 并报「路径不存在」。
//
// 实际探测走 symlinkProbeFn 注入缝隙，使这条安全规则在无法创建符号链接的平台上
// （Windows 需额外权限）也能被真正测试，而不是被静默跳过。
var symlinkProbeFn = lstatIsSymlink

func isAnySymlinkComponent(p string) bool {
	return anySymlinkComponentWith(p, symlinkProbeFn)
}

// lstatIsSymlink 是默认的组件探测实现：Lstat 该路径并判断是否为符号链接。
// 返回 (isSymlink, exists)。读取失败一律视为不存在。
func lstatIsSymlink(path string) (bool, bool) {
	fi, err := os.Lstat(path)
	if err != nil {
		return false, false
	}
	return fi.Mode()&os.ModeSymlink != 0, true
}

// anySymlinkComponentWith 是 isAnySymlinkComponent 的判定主体，探测函数可注入。
//
// 之所以把探测抽出来：在 Windows 上创建符号链接需要额外权限，端到端测试会因此
// 被跳过，而"拒绝符号链接"恰恰是清理功能最关键的一条安全规则。注入探测函数后，
// 这条规则在开发机上也能被真正验证，而不是静默跳过。
func anySymlinkComponentWith(p string, probe func(string) (bool, bool)) bool {
	if probe == nil {
		return true // 无法判定 → 保守拒绝
	}
	// 根目录本身也可能是符号链接。
	if isLink, exists := probe("/"); exists && isLink {
		return true
	}
	current := "/"
	for _, seg := range splitRel(p) {
		current = current + seg
		isLink, exists := probe(current)
		if !exists {
			// 组件不存在：路径本身无效或已被清理，交由调用方另行处理。
			return false
		}
		if isLink {
			return true
		}
		current += "/"
	}
	return false
}

// normalizeVolPath normalizes a canonical fnOS path (/volN/...) into a clean,
// POSIX-style form.
//
// 这里刻意不使用 filepath.Abs/Clean：清理目标是 Linux 上的 /volN 路径，而构建与
// 单元测试可能运行在 Windows 上，filepath 会按宿主平台解释分隔符，使同一份规则表
// 在开发机上匹配失败。规则匹配必须与宿主平台无关，因此统一按 POSIX 语义处理：
// 要求绝对路径，拒绝控制字符、反斜杠与任何 `..` 段，并折叠重复斜杠。
//
// 返回 "" 表示路径非法。
func normalizeVolPath(p string) string {
	if p == "" || len(p) > 4096 {
		return ""
	}
	if strings.ContainsAny(p, "\x00\r\n") {
		return ""
	}
	// 反斜杠在 Linux 上是合法文件名字符，但正常 fnOS 路径不会出现；拒绝它可以
	// 避免 Windows 形式的路径绕过按 `/` 分段的校验。
	if strings.Contains(p, "\\") {
		return ""
	}
	if !strings.HasPrefix(p, "/") {
		return ""
	}
	segs := strings.Split(p, "/")
	out := make([]string, 0, len(segs))
	for _, s := range segs {
		switch s {
		case "", ".":
			continue
		case "..":
			// 任何 `..` 段都直接拒绝，而不是尝试解析——解析容易出错，
			// 而合法目标路径里根本不会出现。
			return ""
		default:
			out = append(out, s)
		}
	}
	if len(out) == 0 {
		return ""
	}
	return "/" + strings.Join(out, "/")
}

// matchRuleForPath 用规则表校验一个绝对路径。
//
// 这是清理路径的唯一准入判断：只有命中规则表、且规则标记为 Safe 的路径才允许删除。
// 前端传来的路径一律当作不可信输入，必须经过这里。
func matchRuleForPath(absPath string) (spaceRule, bool) {
	safe := normalizeVolPath(absPath)
	if safe == "" {
		return spaceRule{}, false
	}
	// 拒绝符号链接本身及其路径中的任何符号链接组件，防止借助软链接把删除
	// 操作引到规则之外的位置。路径不存在时该检查返回 false（由调用方另行 stat）。
	if isAnySymlinkComponent(safe) {
		return spaceRule{}, false
	}

	vol := volRootOf(safe)
	if vol == "" {
		return spaceRule{}, false
	}
	rel := strings.TrimPrefix(safe, "/"+vol+"/")
	pathSegs := splitRel(rel)
	if len(pathSegs) == 0 {
		return spaceRule{}, false
	}

	for _, rule := range spaceRules {
		if !rule.Safe {
			continue
		}
		if !matchSegments(splitRel(rule.Pattern), pathSegs) {
			continue
		}
		// 本应用自身的临时目录永不清理：正在运行的 logmanager 可能正在使用它。
		if spaceIsSelfTemp(safe) {
			return spaceRule{}, false
		}
		// 运行中应用的临时目录同样拒绝。这里独立复算，不依赖扫描结果，
		// 使「扫描后被启动的应用」也无法被清理。
		if spaceIsRunningAppPath(safe) {
			return spaceRule{}, false
		}
		return rule, true
	}
	return spaceRule{}, false
}

// matchCleanablePath 是清理操作唯一的准入判断，覆盖三套规则来源：
//
//   - /volN 下的存储卷规则表（matchRuleForPath）；
//   - 系统分区上的固定路径（matchSystemRule）；
//   - 系统分区上的 /tmp、/var/tmp（matchSystemTempForPath）。
//
// 三套都必须独立复算运行期状态，不信任前端传来的任何信息。
func matchCleanablePath(absPath string) (spaceRule, bool) {
	if rule, ok := matchRuleForPath(absPath); ok {
		return rule, true
	}
	return matchSystemPath(absPath)
}

// CleanSpaceItems 清理指定的条目路径。
//
// paths 是前端选择的绝对路径集合；每条都会重新经过 matchCleanablePath 校验，
// 未命中规则（或已被进程占用）的路径被记为该条跳过，不会执行任何文件操作。
// dryRun 为 true 时只做校验与体积统计，不做任何修改。
func CleanSpaceItems(paths []string, dryRun bool, uid string) SpaceCleanResult {
	result := SpaceCleanResult{DryRun: dryRun, Items: []SpaceCleanItemResult{}}
	seen := map[string]bool{}

	// 清空 /proc 枚举缓存：扫描到清理之间进程可能已经开始使用某条目，
	// 复用扫描期的快照会让这道闸门失效。
	resetOpenPathsCache()
	defer resetOpenPathsCache()

	for _, raw := range paths {
		if raw == "" || seen[raw] {
			continue
		}
		seen[raw] = true

		// 统一走 POSIX 归一化，保证与实际匹配/操作的是同一个路径字符串。
		abs := normalizeVolPath(raw)
		if abs == "" {
			result.Items = append(result.Items, SpaceCleanItemResult{
				Path: raw, Status: "skipped", Message: "路径不合法，已跳过",
			})
			continue
		}
		rule, ok := matchCleanablePath(abs)
		if !ok {
			result.Items = append(result.Items, SpaceCleanItemResult{
				Path:    raw,
				Status:  "skipped",
				Message: "路径不在允许清理的规则内，或当前正被使用，已跳过",
			})
			continue
		}

		size, _, _, _, _ := spacePathStats(abs, spaceScanEntryCap)
		if _, err := os.Lstat(abs); err != nil {
			result.Items = append(result.Items, SpaceCleanItemResult{
				Path: raw, Status: "skipped", Message: "路径不存在",
			})
			continue
		}

		if dryRun {
			result.Items = append(result.Items, SpaceCleanItemResult{
				Path: raw, Status: "skipped", FreedBytes: size,
				Message: "试运行：将按 " + rule.Strategy + " 处理，可释放 " + utils.FormatBytes(size),
			})
			continue
		}

		itemResult := SpaceCleanItemResult{Path: raw, FreedBytes: size}
		switch rule.Strategy {
		case "delete":
			if err := os.RemoveAll(abs); err != nil {
				itemResult.Status = "failed"
				itemResult.FreedBytes = 0
				itemResult.Message = err.Error()
				result.Failed++
				result.Errors = append(result.Errors, abs+": "+err.Error())
			} else {
				itemResult.Status = "deleted"
				result.Deleted++
				result.Freed += size
			}

		case "recycle":
			trashDir := recycleDirForPath(abs, uid)
			if err := moveToTrash(abs, trashDir, abs); err != nil {
				itemResult.Status = "failed"
				itemResult.FreedBytes = 0
				itemResult.Message = err.Error()
				result.Failed++
				result.Errors = append(result.Errors, abs+": "+err.Error())
			} else {
				itemResult.Status = "recycled"
				itemResult.Message = "已移入回收站，可在保留期内还原"
				result.Recycled++
				result.Freed += size
			}

		case "truncate":
			if err := os.Truncate(abs, 0); err != nil {
				itemResult.Status = "failed"
				itemResult.FreedBytes = 0
				itemResult.Message = err.Error()
				result.Failed++
				result.Errors = append(result.Errors, abs+": "+err.Error())
			} else {
				itemResult.Status = "truncated"
				itemResult.Message = "已截断为 0 字节"
				result.Truncated++
				result.Freed += size
			}

		default:
			itemResult.Status = "skipped"
			itemResult.FreedBytes = 0
			itemResult.Message = "该条目不允许清理"
		}
		result.Items = append(result.Items, itemResult)
	}

	result.FreedFormatted = utils.FormatBytes(result.Freed)
	InvalidateSpaceScanCache()
	InvalidateDirScanCache()
	return result
}
