package services

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 系统分区清理规则的安全性测试。
//
// 这一组的重点是**拒绝**而不是接受：系统分区上删错目录会让设备无法启动，
// 后果远比其他分区严重。因此每条"必须拒绝"的用例都比"能够接受"更重要。

// TestSystemPathForbiddenCoversCriticalPaths 锁定兜底拒绝名单。
//
// 即使规则表被误改，这些前缀也必须永远不被接受。这是纵深防御的最后一层。
func TestSystemPathForbiddenCoversCriticalPaths(t *testing.T) {
	mustReject := []string{
		// 引导与内核：删错设备起不来
		"/boot",
		"/boot/vmlinuz-6.18.18.c1145-trim",
		"/boot/extlinux/extlinux.conf",
		"/usr/lib/modules/6.18.18.c1145-trim",
		"/lib/modules/6.18.18.c1145-trim",
		"/usr/src/linux-headers-6.18.18.c1145-trim",
		"/usr/lib/linux-image-6.18.18.c1145-trim",
		// 系统命脉
		"/etc/passwd",
		"/etc/shadow",
		"/etc/fstab",
		"/var/lib/dpkg/status",
		"/var/lib/postgresql",
		"/usr/trim/bin/trim",
		"/usr/lib/aarch64-linux-gnu/libc.so.6",
		"/usr/local/apps/@appdata",
		// B 档：只提示不删
		"/usr/share/doc",
		"/usr/share/man",
		"/usr/share/locale",
		"/usr/local/go",
		"/usr/lib/firmware",
		"/usr/share/AAVMF",
		"/usr/share/proj",
		"/usr/share/pocketsphinx",
		"/usr/lib/gcc",
	}
	for _, p := range mustReject {
		if !systemPathForbidden(p) {
			t.Errorf("systemPathForbidden(%q) = false，必须为 true（该路径在拒绝名单内）", p)
		}
	}
}

// TestSystemPathForbiddenAllowsCleanablePaths 确认拒绝名单没有过宽——
// 否则整个功能会变成永远扫不到东西的空壳。
func TestSystemPathForbiddenAllowsCleanablePaths(t *testing.T) {
	mustAllow := []string{
		"/var/log/syslog",
		"/var/log/syslog.1",
		"/var/log/kern.log",
		"/var/log/sysstat/sar01",
		"/var/cache/apt/pkgcache.bin",
		"/var/lib/apt/lists/partial",
		"/var/lib/smartmontools/drivedb/drivedb.h.old",
		"/etc/rsyslog.conf.dpkg-dist",
	}
	for _, p := range mustAllow {
		if systemPathForbidden(p) {
			t.Errorf("systemPathForbidden(%q) = true，拒绝名单过宽，该路径应可清理", p)
		}
	}

	// /boot 下的路径一律拒绝，包括那个看似无害的悬空链接：
	// 引导分区上的删除必须由内核清理功能带着双重校验来做。
	for _, p := range []string{"/boot/vmlinuz.old", "/boot/anything"} {
		if !systemPathForbidden(p) {
			t.Errorf("systemPathForbidden(%q) = false，引导分区必须整体拒绝", p)
		}
	}
}

// TestSystemPathForbiddenIsSegmentAware 确认拒绝名单按路径段匹配，
// 不会因为裸字符串前缀而误伤名字相近的无关目录。
func TestSystemPathForbiddenIsSegmentAware(t *testing.T) {
	// /boot 的拒绝不能蔓延到 /bootleg。
	if systemPathForbidden("/bootleg/data") {
		t.Error("/bootleg 不应被 /boot 规则误伤 —— 匹配必须按路径段")
	}
	// /usr/src 的拒绝不能蔓延到 /usr/srcfoo。
	if systemPathForbidden("/usr/srcfoo") {
		t.Error("/usr/srcfoo 不应被 /usr/src 规则误伤")
	}
	// 但带连字符的前缀项要能匹配 linux-image-<ver> 的各种版本。
	for _, p := range []string{
		"/usr/lib/linux-image-6.18.18.c1090-trim",
		"/usr/lib/linux-image-6.18.18.c1145-trim",
	} {
		if !systemPathForbidden(p) {
			t.Errorf("systemPathForbidden(%q) = false，内核设备树目录必须拒绝", p)
		}
	}
}

// TestMatchSystemRuleOnlyAcceptsTableEntries 确认准入只认规则表。
func TestMatchSystemRuleOnlyAcceptsTableEntries(t *testing.T) {
	t.Run("规则表内的路径被接受", func(t *testing.T) {
		rule, ok := matchSystemRule("/var/cache/apt/pkgcache.bin")
		if !ok {
			t.Fatal("规则表内的路径应被接受")
		}
		if rule.Strategy != "delete" {
			t.Errorf("策略 = %q, 期望 delete", rule.Strategy)
		}
		if !rule.Safe {
			t.Error("该规则应为 Safe")
		}
	})

	t.Run("日志采用截断策略而非删除", func(t *testing.T) {
		rule, ok := matchSystemRule("/var/log/syslog")
		if !ok {
			t.Fatal("/var/log/syslog 应被接受")
		}
		if rule.Strategy != "truncate" {
			t.Fatalf("策略 = %q, 期望 truncate —— 直接删除会让 rsyslog 继续写已 unlink 的 inode，"+
				"空间不释放且日志消失", rule.Strategy)
		}
	})

	t.Run("拒绝名单内的路径即使像规则也拒绝", func(t *testing.T) {
		// /var/log 下的 sysstat 规则存在，但 /boot 下没有任何规则，
		// 这里验证拒绝名单压制一切。
		if _, ok := matchSystemRule("/boot/vmlinuz-6.18.18.c1145-trim"); ok {
			t.Error("/boot 下的路径必须被拒绝")
		}
	})

	// 这条用**同时命中规则表、又被拒绝名单覆盖**的路径，才能真正检验
	// "拒绝名单参与准入判断"——否则把 matchSystemRule 里的那次调用删掉，
	// 测试依然会通过（这正是变异测试抓出来的问题）。
	//
	// 做法：临时把一条规则的目标路径也加入拒绝名单，然后验证它被拒。
	t.Run("规则命中但落在拒绝名单内时必须拒绝", func(t *testing.T) {
		orig := systemPathIsForbidden
		systemPathIsForbidden = append(append([]string{}, orig...), "/var/log")
		t.Cleanup(func() { systemPathIsForbidden = orig })

		// 前置条件：该路径本来确实能命中规则，否则这条测试没有意义。
		if !matchSegments(splitRel("/var/log/sysstat/*"), splitRel("/var/log/sysstat/sar01")) {
			t.Fatal("前置条件不成立：/var/log/sysstat/sar01 应能命中规则")
		}
		// 同一路径不在拒绝名单时应当通过 —— 证明"被拒"确实是拒绝名单造成的。
		if _, ok := matchSystemRule("/var/log/sysstat/sar01"); ok {
			t.Error("路径命中规则但同时落在拒绝名单内时必须拒绝 —— " +
				"说明 matchSystemRule 没有调用 systemPathForbidden")
		}
	})

	// 反向对照：不修改拒绝名单时，同一条路径必须被接受。
	// 与上一条合起来才能证明"拒绝"来自拒绝名单本身，
	// 而不是碰巧因为别的原因被拒——变异测试正是靠这对关系生效。
	t.Run("同一路径不在拒绝名单时被接受（对照）", func(t *testing.T) {
		rule, ok := matchSystemRule("/var/log/sysstat/sar01")
		if !ok {
			t.Fatal("对照条件不成立：/var/log/sysstat/sar01 应被接受")
		}
		if rule.Strategy != "delete" {
			t.Errorf("策略 = %q, 期望 delete", rule.Strategy)
		}
	})

	t.Run("规则表外的路径被拒绝", func(t *testing.T) {
		for _, p := range []string{
			"/var/log/random-unknown.log",
			"/var/lib/something",
			"/usr/share/something",
			"/etc/nginx/nginx.conf",
		} {
			if _, ok := matchSystemRule(p); ok {
				t.Errorf("matchSystemRule(%q) 被接受，但它不在规则表内", p)
			}
		}
	})

	t.Run("路径穿越被拒绝", func(t *testing.T) {
		for _, p := range []string{
			"/var/log/../../etc/passwd",
			"/var/cache/apt/../../../etc/shadow",
			"//var//log//../etc/passwd",
		} {
			if _, ok := matchSystemRule(p); ok {
				t.Errorf("matchSystemRule(%q) 被接受，含 .. 的路径必须拒绝", p)
			}
		}
	})

	// 这条用注入的探测器验证"符号链接被拒绝"确实在准入路径上生效。
	// 真实建链在 Windows 上需要额外权限，端到端测试会被跳过，
	// 因此这里通过 seam 构造：一条命中规则的路径，其某段是符号链接。
	t.Run("路径中任一段是符号链接时必须拒绝", func(t *testing.T) {
		orig := symlinkProbeFn
		t.Cleanup(func() { symlinkProbeFn = orig })

		// /var/log 被标记为符号链接，/var/log/syslog 命中规则。
		symlinkProbeFn = func(p string) (bool, bool) {
			switch p {
			case "/", "/var":
				return false, true
			case "/var/log":
				return true, true // 是链接
			default:
				return false, true
			}
		}
		if _, ok := matchSystemRule("/var/log/syslog"); ok {
			t.Error("路径中含符号链接组件时必须拒绝 —— 否则可借软链接逃逸到规则之外")
		}
	})

	t.Run("探测器无法判定时也必须拒绝", func(t *testing.T) {
		orig := symlinkProbeFn
		t.Cleanup(func() { symlinkProbeFn = orig })
		symlinkProbeFn = func(string) (bool, bool) { return false, false }
		// 组件"不存在"时 isAnySymlinkComponent 返回 false，这是刻意设计：
		// 路径不存在由调用方另行报错，而不是在这里被当成非法。
		// 但真正判定不了的场景（探测器为 nil）必须保守拒绝。
		if !anySymlinkComponentWith("/var/log/syslog", nil) {
			t.Error("探测器为 nil 时必须保守拒绝")
		}
	})

	t.Run("相对路径被拒绝", func(t *testing.T) {
		for _, p := range []string{
			"var/log/syslog",
			"./var/log/syslog",
			"",
		} {
			if _, ok := matchSystemRule(p); ok {
				t.Errorf("matchSystemRule(%q) 被接受，非绝对路径必须拒绝", p)
			}
		}
	})
}

// TestSystemRotatedLogRe 锁定"什么算已轮转日志"。
//
// 这条规则最容易搞错：把正在写入的裸文件名当成可删，会让日志凭空消失；
// 反过来漏掉轮转文件则拿不到空间。两种错误都要防。
func TestSystemRotatedLogRe(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		// --- 应接受：已轮转/归档 ---
		{"syslog.1", true},
		{"syslog.2.gz", true},
		{"kern.log.1", true},
		{"kern.log.4.gz", true},
		{"auth.log.3.gz", true},
		{"messages.1", true},
		{"messages.2.gz", true},
		{"daemon.log.old", true},
		{"some.log.xz", true},
		{"other.log.bz2", true},
		{"thing.log.zst", true},

		// --- 必须拒绝：正在写入的裸文件名 ---
		// 这些由 truncate 规则处理，绝不能在这里被 unlink。
		{"syslog", false},
		{"kern.log", false},
		{"auth.log", false},
		{"messages", false},
		{"cron.log", false},
		{"daemon.log", false},

		// --- 必须拒绝：非轮转形式 ---
		{"", false},
		{".hidden", false},
		{"README", false},
		{"nginx.conf", false},
		{"journal", false},
		{"wtmp", false},
		{"btmp", false},
		{"lastlog", false},
		// 点号后不是数字，不算轮转
		{"syslog.backup", false},
		{"app.log.tmp", false},
		// 只有后缀、没有主体
		{".gz", false},
		{".old", false},
		// 数字后缀但主体为空
		{".1", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := systemRotatedLogRe(tt.name); got != tt.want {
				t.Errorf("systemRotatedLogRe(%q) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

// TestSystemChildIsRotatedLogRejectsDirectories 是最重要的一条：
// /var/log 下还有 samba/、sysstat/、journal/ 等**目录**，
// DirectChildren 规则绝不能被它们误当成文件删掉。
func TestSystemChildIsRotatedLogRejectsDirectories(t *testing.T) {
	dir := t.TempDir()

	// 目录：即使名字看起来像轮转日志也必须拒绝
	dirAsLog := filepath.Join(dir, "syslog.1")
	if err := os.Mkdir(dirAsLog, 0o755); err != nil {
		t.Fatal(err)
	}
	if systemChildIsRotatedLog(dirAsLog) {
		t.Error("目录 syslog.1 被当成可删日志 —— 目录绝不能被该规则删除")
	}

	// 普通目录名
	sub := filepath.Join(dir, "samba")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if systemChildIsRotatedLog(sub) {
		t.Error("目录 samba 被当成可删日志")
	}

	// 真文件：接受
	fileAsLog := filepath.Join(dir, "kern.log.2.gz")
	if err := os.WriteFile(fileAsLog, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !systemChildIsRotatedLog(fileAsLog) {
		t.Error("已轮转日志文件 kern.log.2.gz 应被接受")
	}

	// 正在写入的裸文件：拒绝
	live := filepath.Join(dir, "syslog")
	if err := os.WriteFile(live, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if systemChildIsRotatedLog(live) {
		t.Error("正在写入的 syslog 不应被 DirectChildren 规则删除（应由 truncate 处理）")
	}

	// 不存在的路径：拒绝
	if systemChildIsRotatedLog(filepath.Join(dir, "nope.1")) {
		t.Error("不存在的路径不应被接受")
	}
}

// TestSystemPathRulesHaveValidStrategies 确认规则表本身没有写错策略。
//
// 本表刻意不使用 recycle：回收站在 /volN，跨分区搬运对解决系统分区空间问题
// 没有帮助，反而增加风险。
func TestSystemPathRulesHaveValidStrategies(t *testing.T) {
	valid := map[string]bool{"delete": true, "truncate": true}
	for _, r := range systemPathRules {
		if !valid[r.Strategy] {
			t.Errorf("规则 %q 的策略 %q 非法（只允许 delete/truncate）", r.Pattern, r.Strategy)
		}
		if r.Category == "" {
			t.Errorf("规则 %q 缺少分类", r.Pattern)
		}
		if r.Reason == "" {
			t.Errorf("规则 %q 缺少说明", r.Pattern)
		}
		// 用 POSIX 判定而不是 filepath.IsAbs：规则表描述的是 Linux 上的路径，
		// 而测试可能跑在 Windows 上（filepath.IsAbs 会拒绝 /var/...）。
		if !strings.HasPrefix(r.Pattern, "/") {
			t.Errorf("规则 %q 不是绝对路径 —— 系统分区规则必须是绝对路径", r.Pattern)
		}
	}
}

// TestSystemPathRulesNeverOverlapForbidden 是规则表与拒绝名单的一致性检查：
// 任何规则都不允许落在拒绝名单内。
func TestSystemPathRulesNeverOverlapForbidden(t *testing.T) {
	for _, r := range systemPathRules {
		// 把 `*` 换成占位名后检查前缀。
		probe := r.Pattern
		if i := indexByte(probe, '*'); i >= 0 {
			probe = probe[:i] + "x"
		}
		if systemPathForbidden(probe) {
			t.Errorf("规则 %q 落在拒绝名单内 —— 规则表与拒绝名单冲突", r.Pattern)
		}
	}
	for _, c := range systemCommandRules {
		if !c.Safe && c.Command == "" {
			t.Errorf("包管理器规则 %q 未给出建议命令", c.ID)
		}
	}
	if systemCommandRules[0].Safe {
		t.Error("包管理器规则必须 Safe=false：本应用不代为执行包管理器操作")
	}
}

// indexByte 是 strings.IndexByte 的本地别名，避免为一个调用引入额外 import。
func indexByte(s string, b byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == b {
			return i
		}
	}
	return -1
}

// TestAnySymlinkComponentWith 用注入的探测器验证符号链接防线。
//
// 不用真实符号链接：Windows 上创建链接需要额外权限，端到端测试会被跳过，
// 而这条恰恰是最关键的安全规则。注入探测函数后它在任何平台都能被真正验证。
func TestAnySymlinkComponentWith(t *testing.T) {
	// existsAt 返回一个探测器：只有出现在 set 里的路径"存在"。
	existsAt := func(set map[string]bool, links map[string]bool) func(string) (bool, bool) {
		return func(p string) (bool, bool) {
			if !set[p] {
				return false, false
			}
			return links[p], true
		}
	}

	t.Run("全程无链接时接受", func(t *testing.T) {
		set := map[string]bool{"/": true, "/var": true, "/var/log": true, "/var/log/syslog": true}
		probe := existsAt(set, map[string]bool{})
		if anySymlinkComponentWith("/var/log/syslog", probe) {
			t.Error("无符号链接的路径不应被判为含链接")
		}
	})

	t.Run("中间组件是链接时拒绝", func(t *testing.T) {
		// /var/log 是链接（真机上 /var/log -> /tmp/log 这种情况真实存在）
		set := map[string]bool{"/": true, "/var": true, "/var/log": true, "/var/log/syslog": true}
		probe := existsAt(set, map[string]bool{"/var/log": true})
		if !anySymlinkComponentWith("/var/log/syslog", probe) {
			t.Error("路径中含符号链接组件时必须拒绝 —— 否则可借软链接逃逸到规则之外")
		}
	})

	t.Run("末段本身是链接时拒绝", func(t *testing.T) {
		set := map[string]bool{"/": true, "/var": true, "/var/log": true, "/var/log/syslog": true}
		probe := existsAt(set, map[string]bool{"/var/log/syslog": true})
		if !anySymlinkComponentWith("/var/log/syslog", probe) {
			t.Error("末段是符号链接时必须拒绝")
		}
	})

	t.Run("根目录是链接时拒绝", func(t *testing.T) {
		probe := existsAt(map[string]bool{"/": true}, map[string]bool{"/": true})
		if !anySymlinkComponentWith("/var/log", probe) {
			t.Error("根目录本身是符号链接时必须拒绝")
		}
	})

	t.Run("组件不存在时不算含链接（交由调用方报路径不存在）", func(t *testing.T) {
		// /var/log 存在但 syslog 不存在
		set := map[string]bool{"/": true, "/var": true, "/var/log": true}
		probe := existsAt(set, map[string]bool{})
		if anySymlinkComponentWith("/var/log/syslog", probe) {
			t.Error("组件不存在时应返回 false —— 是否「非法」由调用方另行判断")
		}
	})

	t.Run("探测器为 nil 时保守拒绝", func(t *testing.T) {
		if !anySymlinkComponentWith("/var/log/syslog", nil) {
			t.Error("无法判定时必须保守拒绝，不能放行")
		}
	})
}

// TestMatchSystemRuleRejectsSymlinkComponents 是上面那条规则在准入层的体现。
// 真实符号链接可用时顺带验证；不可用时该子测试跳过，但规则的验证已由
// TestAnySymlinkComponentWith 覆盖，不依赖平台。
func TestMatchSystemRuleRejectsSymlinkComponents(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "real-target")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("当前平台无法创建符号链接（规则本身已由 TestAnySymlinkComponentWith 覆盖）: %v", err)
	}
	if _, ok := matchSystemRule(link); ok {
		t.Error("符号链接本身必须被拒绝")
	}
	if _, ok := matchSystemRule(filepath.Join(link, "syslog")); ok {
		t.Error("路径中含符号链接组件时必须被拒绝")
	}
}
