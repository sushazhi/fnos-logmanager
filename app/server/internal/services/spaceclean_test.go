package services

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestGlobSegment 覆盖 `*` 段的匹配语义。清理功能的安全性依赖这里：
// `*` 不能跨越路径分隔符（段在调用前已拆分），且首尾段必须锚定。
func TestGlobSegment(t *testing.T) {
	cases := []struct {
		pattern string
		name    string
		want    bool
	}{
		{"*", "anything", true},
		{"*", "", false},
		{"exact", "exact", true},
		{"exact", "exactx", false},
		{"exact", "xexact", false},
		{"*-json.log", "abc-json.log", true},
		{"*-json.log", "abc-json.log.1", false},
		{"*-json.log", "json.log", false},
		{"trim.ai-runtime-*", "trim.ai-runtime-rk3568", true},
		{"trim.ai-runtime-*", "trim.ai-runtime-", true},
		{"trim.ai-runtime-*", "xtrim.ai-runtime-x", false},
		{"a*b", "ab", true},
		{"a*b", "axxb", true},
		{"a*b", "axxc", false},
		{"a*b", "xaxb", false},
		{"*a*", "xay", true},
		{"*a*", "xy", false},
	}
	for _, c := range cases {
		if got := globSegment(c.pattern, c.name); got != c.want {
			t.Errorf("globSegment(%q, %q) = %v, want %v", c.pattern, c.name, got, c.want)
		}
	}
}

// TestMatchSegments 确认段数必须严格相等，避免 `@appdata/*` 误匹配更深层路径。
func TestMatchSegments(t *testing.T) {
	cases := []struct {
		pattern string
		path    string
		want    bool
	}{
		{"@appdata/*/npm-cache/_cacache", "@appdata/deepseek.harness/npm-cache/_cacache", true},
		{"@appdata/*/npm-cache/_cacache", "@appdata/x/npm-cache/_cacache/extra", false},
		{"@appdata/*/npm-cache/_cacache", "@appdata/npm-cache/_cacache", false},
		{"@apptemp/*", "@apptemp/logmanager", true},
		{"@apptemp/*", "@apptemp", false},
		{"appcenter-downloads/*", "appcenter-downloads/upload", true},
		{"@appdata/*/config/cache", "@appdata/moviepilot/config/cache", true},
		{"@appdata/*/config/cache", "@appdata/moviepilot/config/cachex", false},
		{"*/AppData/*/dep_cache", "1000/AppData/qinglong/dep_cache", true},
		{"*/AppData/*/dep_cache", "1000/AppData/dep_cache", false},
	}
	for _, c := range cases {
		got := matchSegments(splitRel(c.pattern), splitRel(c.path))
		if got != c.want {
			t.Errorf("matchSegments(%q, %q) = %v, want %v", c.pattern, c.path, got, c.want)
		}
	}
}

// TestMatchRuleForPathRejectsOutsidePaths 是清理功能最关键的回归测试：
// 任何不在规则表内的路径都必须被拒绝，尤其是 /vol1/1000 用户数据、
// which 是全盘最敏感的区域。
func TestMatchRuleForPathRejectsOutsidePaths(t *testing.T) {
	rejected := []string{
		"",                                      // 空路径
		"relative/path",                         // 相对路径
		"/vol1",                                 // 卷根
		"/vol1/1000",                            // 用户主目录
		"/vol1/1000/photos",                     // 用户相册
		"/vol1/1000/pt",                         // 用户下载的影视
		"/vol1/1000/AppData",                    // 用户 AppData 根
		"/vol1/@appdata",                        // 应用数据根
		"/vol1/@appdata/moviepilot",             // 应用数据目录本身
		"/vol1/@appdata/moviepilot/config",      // 应用配置
		"/vol1/@appshare/qbittorrent/downloads", // 共享下载内容
		"/vol1/@appcenter/moviepilot",           // 已安装应用目录
		"/vol1/@sysappmeta/ai-manager/models",   // 系统 AI 模型（仅提示，不可删）
		"/vol1/docker/overlay2",                 // Docker 镜像层（仅提示，不可删）
		"/vol1/@apptemp",                        // 临时根（规则只允许其子项）
		"/etc/passwd",                           // 非 /volN 路径
		"/vol1/@appdata/../@appdata/moviepilot/config", // 含 .. 的规避尝试
		"/vol1/1000/../../etc",                         // 目录穿越尝试
	}
	for _, p := range rejected {
		if _, ok := matchRuleForPath(p); ok {
			t.Errorf("matchRuleForPath(%q) allowed, want rejected", p)
		}
	}
}

// TestMatchRuleForPathAcceptsRuleTargets 确认规则表内的目标确实被接受，
// 否则清理功能会「全部跳过」而静默失效。
func TestMatchRuleForPathAcceptsRuleTargets(t *testing.T) {
	accepted := map[string]string{
		"/vol1/@appdata/deepseek.harness/pnpm-home/store":    "delete",
		"/vol1/@appdata/deepseek.harness/npm-cache/_cacache": "delete",
		"/vol1/@appdata/deepseek.harness/npm-cache/_npx":     "delete",
		"/vol1/@appdata/moviepilot/home/.cache/pnpm":         "delete",
		"/vol1/@appdata/moviepilot/home/.cache/go-build":     "delete",
		"/vol1/@appdata/moviepilot/config/cache":             "delete",
		"/vol1/thumb/1000":                                   "delete",
		"/vol1/appcenter-downloads/upload":                   "delete",
		"/vol1/1000/AppData/qinglong/dep_cache":              "delete",
		"/vol1/docker/containers/abc123/abc123-json.log":     "truncate",
		"/vol1/@appdata/moviepilot/config/database_backup":   "recycle",
	}
	for p, wantStrategy := range accepted {
		rule, ok := matchRuleForPath(p)
		if !ok {
			t.Errorf("matchRuleForPath(%q) rejected, want accepted", p)
			continue
		}
		if !rule.Safe {
			t.Errorf("matchRuleForPath(%q) returned unsafe rule", p)
		}
		if rule.Strategy != wantStrategy {
			t.Errorf("matchRuleForPath(%q) strategy = %q, want %q", p, rule.Strategy, wantStrategy)
		}
	}
}

// TestMatchRuleForPathRejectsBrowserBinaries 确认那几个"名字像缓存、实为已下载
// 二进制"的路径在服务端被拒绝，即使有人绕过前端直接调接口也删不掉。
func TestMatchRuleForPathRejectsBrowserBinaries(t *testing.T) {
	for _, p := range []string{
		"/vol1/@appdata/moviepilot/config/.browser",
		"/vol1/@appdata/moviepilot/config/.browser/cloakbrowser",
		"/vol1/@appdata/deepseek.harness/home/.cache",
		"/vol1/@appdata/deepseek.harness/home/.cache/puppeteer",
		"/vol1/@appdata/deepseek.harness/home/.cache/puppeteer/chrome",
		"/vol1/@appdata/deepseek.harness/pnpm-env",
	} {
		if rule, ok := matchRuleForPath(p); ok {
			t.Errorf("matchRuleForPath(%q) accepted with strategy %q, want rejected: "+
				"该路径是已下载的浏览器/工具本体，删除后无法自动恢复", p, rule.Strategy)
		}
	}
}

// TestRuleTargetSafeTempDirAccepted 确认非本应用的 @apptemp 子目录
// 仍然走规则表正常放行（只有自身临时目录被额外排除）。
func TestRuleTargetSafeTempDirAccepted(t *testing.T) {
	p := "/vol1/@apptemp/qbittorrent"
	rule, ok := matchRuleForPath(p)
	if !ok {
		t.Fatalf("matchRuleForPath(%q) rejected, want accepted", p)
	}
	if rule.Strategy != "delete" {
		t.Errorf("strategy = %q, want delete", rule.Strategy)
	}
}

// TestRuntimeDepsRulesAreUnsafe 确认「删掉会导致应用起不来」的目录
// 绝不允许通过清理接口删除。
func TestRuntimeDepsRulesAreUnsafe(t *testing.T) {
	unsafeTargets := []string{
		"/vol1/@appdata/deepseek.harness/dsh-runtime/node_modules",
		"/vol1/@appdata/someapp/node_modules",
	}
	for _, p := range unsafeTargets {
		if _, ok := matchRuleForPath(p); ok {
			t.Errorf("matchRuleForPath(%q) allowed a runtime-dependency path", p)
		}
	}
}

// TestCleanSpaceItemsSkipsUnmatchedPaths 确认清理接口对规则外路径
// 只返回 skipped，且不触碰文件系统。
func TestCleanSpaceItemsSkipsUnmatchedPaths(t *testing.T) {
	// 用临时目录构造真实存在的敏感路径；它不在规则表内，必须原样保留。
	tmp := t.TempDir()
	victim := filepath.Join(tmp, "user-data")
	if err := os.MkdirAll(victim, 0o755); err != nil {
		t.Fatal(err)
	}
	keep := filepath.Join(victim, "important.txt")
	if err := os.WriteFile(keep, []byte("do not delete"), 0o644); err != nil {
		t.Fatal(err)
	}

	result := CleanSpaceItems([]string{victim, keep}, false, "1000")

	if result.Deleted != 0 || result.Freed != 0 {
		t.Errorf("unexpected deletion: deleted=%d freed=%d", result.Deleted, result.Freed)
	}
	for _, item := range result.Items {
		if item.Status != "skipped" {
			t.Errorf("path %q status = %q, want skipped", item.Path, item.Status)
		}
	}
	if _, err := os.Stat(keep); err != nil {
		t.Fatalf("rule-external file was removed: %v", err)
	}
}

// TestCleanSpaceItemsDryRunDoesNotModify 确认试运行不产生任何文件系统变更。
func TestCleanSpaceItemsDryRunDoesNotModify(t *testing.T) {
	tmp := t.TempDir()
	target := filepath.Join(tmp, "cache")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "f"), []byte("12345"), 0o644); err != nil {
		t.Fatal(err)
	}

	result := CleanSpaceItems([]string{target}, true, "1000")

	if result.Deleted != 0 || result.Recycled != 0 || result.Truncated != 0 {
		t.Errorf("dry run modified state: %+v", result)
	}
	if _, err := os.Stat(target); err != nil {
		t.Fatalf("dry run removed target: %v", err)
	}
}

// fakeFileInfo 实现 os.FileInfo，用于在不创建真实文件的情况下测试类型判定。
type fakeFileInfo struct {
	mode os.FileMode
}

func (f fakeFileInfo) Name() string       { return "fake" }
func (f fakeFileInfo) Size() int64        { return 0 }
func (f fakeFileInfo) Mode() os.FileMode  { return f.mode }
func (f fakeFileInfo) ModTime() time.Time { return time.Time{} }
func (f fakeFileInfo) IsDir() bool        { return f.mode&os.ModeDir != 0 }
func (f fakeFileInfo) Sys() interface{}   { return nil }

// TestSpaceExpandAcceptInfoRejectsSymlinks 覆盖符号链接这条约束。
//
// 判定只依赖 os.FileInfo.Mode，因此用假的 FileInfo 就能在任何平台验证——
// 不必依赖"当前平台能否创建符号链接"（Windows 上需要额外权限）。
func TestSpaceExpandAcceptInfoRejectsSymlinks(t *testing.T) {
	cases := []struct {
		name string
		fi   os.FileInfo
		last bool
		want bool
	}{
		{"普通文件·最后一段", fakeFileInfo{mode: 0}, true, true},
		{"目录·最后一段", fakeFileInfo{mode: os.ModeDir}, true, true},
		{"目录·中间段", fakeFileInfo{mode: os.ModeDir}, false, true},
		{"普通文件·中间段（无法继续展开）", fakeFileInfo{mode: 0}, false, false},
		{"符号链接·最后一段（必须拒绝）", fakeFileInfo{mode: os.ModeSymlink}, true, false},
		{"符号链接·中间段（必须拒绝）", fakeFileInfo{mode: os.ModeSymlink}, false, false},
		{"符号链接指向目录（必须拒绝）", fakeFileInfo{mode: os.ModeSymlink | os.ModeDir}, true, false},
		{"符号链接指向目录·中间段（必须拒绝）", fakeFileInfo{mode: os.ModeSymlink | os.ModeDir}, false, false},
		{"nil FileInfo", nil, true, false},
	}
	for _, c := range cases {
		if got := spaceExpandAcceptInfo(c.fi, c.last); got != c.want {
			t.Errorf("%s: spaceExpandAcceptInfo = %v, want %v", c.name, got, c.want)
		}
	}
}

// TestSpaceExpandAcceptRejectsSymlinks 直接覆盖"规则展开不跟随符号链接"这条安全规则。
//
// 用 Lstat 的结果做判定无法在 Windows 上构造（创建符号链接需要额外权限），
// 因此这里通过 spaceExpandAccept 的返回值来验证：不存在、符号链接都拒绝，
// 非最后一段还要求是目录。
func TestSpaceExpandAcceptRejectsSymlinks(t *testing.T) {
	root := t.TempDir()

	dir := filepath.Join(root, "adir")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(root, "afile")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(root, "nope")

	// 最后一段：存在即可（文件、目录都接受）。
	if !spaceExpandAccept(dir, true) {
		t.Error("存在的目录应被接受")
	}
	if !spaceExpandAccept(file, true) {
		t.Error("存在的文件应被接受")
	}
	// 不存在必须拒绝。
	if spaceExpandAccept(missing, true) {
		t.Error("不存在的路径必须拒绝")
	}
	// 非最后一段：必须是目录，否则无法继续展开。
	if !spaceExpandAccept(dir, false) {
		t.Error("非最后一段的目录应被接受")
	}
	if spaceExpandAccept(file, false) {
		t.Error("非最后一段的文件必须拒绝")
	}
	if spaceExpandAccept(missing, false) {
		t.Error("不存在的中间段必须拒绝")
	}

	// 符号链接在能创建时一并验证；权限不足则跳过这一段，
	// 上面的存在性/目录性断言已经覆盖了同一函数的分支结构。
	if err := os.Symlink(dir, filepath.Join(root, "link")); err == nil {
		link := filepath.Join(root, "link")
		if spaceExpandAccept(link, true) {
			t.Error("符号链接必须被拒绝（最后一段）")
		}
		if spaceExpandAccept(link, false) {
			t.Error("符号链接必须被拒绝（中间段）")
		}
	}
}

// TestExpandSpacePatternRejectsSymlinks 端到端确认规则展开不跟随符号链接，
// 否则软链接可能把删除操作引到规则之外的位置。权限不足时跳过。
func TestExpandSpacePatternRejectsSymlinks(t *testing.T) {
	root := t.TempDir()
	// 构造 vol 根：fakevol/@appdata/app/pnpm-home/store（真实目标）
	target := filepath.Join(root, "real-store")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	appDir := filepath.Join(root, "fakevol", "@appdata", "app", "pnpm-home")
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// pnpm-home/store 是指向 real-store 的软链接：必须被拒绝
	link := filepath.Join(appDir, "store")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink unsupported on this platform: %v", err)
	}

	got := expandSpacePattern(filepath.Join(root, "fakevol"), "@appdata/*/pnpm-home/store")
	if len(got) != 0 {
		t.Errorf("expandSpacePattern followed a symlink and returned %v", got)
	}
}

// TestSpaceCategoryDefsCoverRules 确认规则表引用的分类都有元数据定义，
// 否则分类会在扫描结果里静默消失。
func TestSpaceCategoryDefsCoverRules(t *testing.T) {
	for _, rule := range spaceRules {
		if _, ok := spaceCategoryDefs[rule.Category]; !ok {
			t.Errorf("rule pattern %q references undefined category %q", rule.Pattern, rule.Category)
		}
	}
	for _, id := range spaceCategoryOrder {
		if _, ok := spaceCategoryDefs[id]; !ok {
			t.Errorf("spaceCategoryOrder references undefined category %q", id)
		}
	}
	if len(spaceCategoryOrder) != len(spaceCategoryDefs) {
		t.Errorf("spaceCategoryOrder has %d entries but %d categories defined",
			len(spaceCategoryOrder), len(spaceCategoryDefs))
	}
}

// TestSpaceRulesAreWellFormed 确认规则本身的策略取值合法。
func TestSpaceRulesAreWellFormed(t *testing.T) {
	valid := map[string]bool{"delete": true, "recycle": true, "truncate": true, "none": true}
	for _, rule := range spaceRules {
		if !valid[rule.Strategy] {
			t.Errorf("rule %q has invalid strategy %q", rule.Pattern, rule.Strategy)
		}
		if rule.Safe && rule.Strategy == "none" {
			t.Errorf("rule %q is marked safe but has strategy none", rule.Pattern)
		}
		if !rule.Safe && rule.Strategy != "none" {
			t.Errorf("rule %q is unsafe but has strategy %q", rule.Pattern, rule.Strategy)
		}
		if rule.Reason == "" {
			t.Errorf("rule %q has no reason", rule.Pattern)
		}
	}
}

// spaceForbiddenPatterns 列出**绝对不能**出现在清理规则表里的宽规则。
//
// 这些路径真机勘察过：名字像缓存，里面装的却是不可自动重建的东西（已下载好的
// 浏览器二进制、包管理器本体）。一旦写成宽规则，用户勾选删除后设备会坏掉，
// 因此在这里用测试锁死，防止以后有人"顺手简化"规则时又把它加回来。
var spaceForbiddenPatterns = map[string]string{
	"@appdata/*/config/.browser": "@appdata/{app}/config/.browser/cloakbrowser/chromium-<版本> 是装好的 Chromium 本体（真机 685 MB / 672 文件），不是缓存",
	"@appdata/*/home/.cache":     "@appdata/{app}/home/.cache/puppeteer/{chrome,chrome-headless-shell} 是 puppeteer 下载的浏览器（真机 691 MB），应改用逐个包管理器缓存的窄规则",
	"@appdata/*/pnpm-env":        "@appdata/{app}/pnpm-env 是 pnpm 工具本体（node_modules/.bin/pnpm），不是缓存",
	"@appdata/*/dsh-runtime":     "@appdata/{app}/dsh-runtime 是应用运行期依赖，删除后应用无法启动",
	"@appdata/*/dsh-data":        "应用自身数据目录，不是缓存",
	"@appdata/*/plugins":         "应用插件目录，不是缓存",
}

// TestSpaceRulesAvoidKnownDangerousPatterns 是本仓库最重要的一条规则安全回归。
//
// 开发过程中确实写出过 `@appdata/*/config/.browser` 和 `@appdata/*/home/.cache`
// 这两条宽规则，真机核对目录内容后才发现在删浏览器本体。删除动作不可逆且
// 离线不可恢复，所以这类模式必须被测试挡住，而不是靠 code review 记住。
func TestSpaceRulesAvoidKnownDangerousPatterns(t *testing.T) {
	for _, rule := range spaceRules {
		if why, bad := spaceForbiddenPatterns[rule.Pattern]; bad {
			t.Errorf("规则表出现危险宽规则 %q（分类 %s）：%s", rule.Pattern, rule.Category, why)
		}
	}
}

// TestSpaceNoticesCoverNonReclaimableBinaries 确认那几个"不可删除但占用大"的
// 目录都有对应的提示项，否则它们会从界面上完全消失，用户既不能删也看不到占用。
func TestSpaceNoticesCoverNonReclaimableBinaries(t *testing.T) {
	want := []string{
		"@appdata/*/config/.browser",
		"@appdata/*/home/.cache/puppeteer",
		"@appdata/*/pnpm-env",
	}
	noticed := map[string]bool{}
	for _, n := range spaceNoticePatterns {
		noticed[n.Pattern] = true
	}
	for _, p := range want {
		if !noticed[p] {
			t.Errorf("提示项缺少 %q：该目录不可删除，必须至少让用户看到它占了多少空间", p)
		}
	}
}

// TestSpaceNarrowCacheRulesReplaceBroadOnes 确认把宽规则拆窄之后，真正可重建的
// 包管理器缓存仍然被覆盖——否则修安全问题会顺手丢掉清理能力。
func TestSpaceNarrowCacheRulesReplaceBroadOnes(t *testing.T) {
	want := []string{
		"@appdata/*/home/.cache/pnpm",
		"@appdata/*/home/.cache/pip",
		"@appdata/*/home/.cache/node-gyp",
		"@appdata/*/home/.cache/go-build",
	}
	have := map[string]bool{}
	for _, rule := range spaceRules {
		have[rule.Pattern] = true
	}
	for _, p := range want {
		if !have[p] {
			t.Errorf("规则表缺少窄规则 %q：home/.cache 下可重建的缓存应逐个命中", p)
		}
	}
}

// TestSpaceIsSelfTemp 确认本应用自己的临时目录被识别，
// 从而在清理时永不删除自身正在使用的目录。
func TestSpaceIsSelfTemp(t *testing.T) {
	yes := []string{
		"/vol1/@apptemp/logmanager",
		"/vol1/@apptemp/logmanager/sub/dir",
		"/vol2/@apptemp/trim.logmanager",
	}
	for _, p := range yes {
		if !spaceIsSelfTemp(p) {
			t.Errorf("spaceIsSelfTemp(%q) = false, want true", p)
		}
	}
	no := []string{
		"/vol1/@apptemp/qbittorrent",
		"/vol1/@appdata/logmanager",
		"/vol1/@apptemp",
		"/vol1/@apptemp/logmanagerx",
	}
	for _, p := range no {
		if spaceIsSelfTemp(p) {
			t.Errorf("spaceIsSelfTemp(%q) = true, want false", p)
		}
	}
}

// TestMatchRuleForPathRejectsSelfTemp 确认即使 @apptemp 规则命中，
// 本应用自己的临时目录也必须被拒绝。
func TestMatchRuleForPathRejectsSelfTemp(t *testing.T) {
	for _, p := range []string{
		"/vol1/@apptemp/logmanager",
		"/vol1/@apptemp/logmanager/cache",
	} {
		if _, ok := matchRuleForPath(p); ok {
			t.Errorf("matchRuleForPath(%q) allowed the app's own temp dir", p)
		}
	}
}

// TestSpaceIsRunningAppPathRejectsTraversal 确认应用名解析不会被路径穿越影响。
func TestSpaceIsRunningAppPathRejectsTraversal(t *testing.T) {
	// 含分隔符的"应用名"必须立即返回 false，不能拼进 /var/apps 路径。
	for _, p := range []string{
		"/vol1/@apptemp/a/b",
		"/vol1/@appdata/whatever",
		"/vol1/@apptemp",
		"/vol1/@apptemp/",
	} {
		if spaceIsRunningAppPath(p) {
			t.Errorf("spaceIsRunningAppPath(%q) = true, want false", p)
		}
	}
}

// TestAppProcessAliveRejectsBadNames 确认非法应用名不会触发任何文件系统访问。
func TestAppProcessAliveRejectsBadNames(t *testing.T) {
	for _, app := range []string{"", "../../etc", "a/b", "x\x00y", "..", "a..b/c"} {
		if appProcessAlive("/vol1", app) {
			t.Errorf("appProcessAlive(%q) = true, want false", app)
		}
		if appProcessAlive("", app) {
			t.Errorf("appProcessAlive(\"\", %q) = true, want false", app)
		}
	}
	// 不存在的应用必然返回 false。
	if appProcessAlive("/vol1", "definitely-not-installed-app-xyz") {
		t.Error("appProcessAlive reported a nonexistent app as running")
	}
}

// TestSpaceIsRunningAppPathRequiresApptemp 确认只有 @apptemp 下的目录
// 才会触发"运行中应用"判定，其他数据目录不受影响。
func TestSpaceIsRunningAppPathRequiresApptemp(t *testing.T) {
	for _, p := range []string{
		"/vol1/@appdata/someapp",
		"/vol1/@appshare/someapp",
		"/vol1/@appconf/someapp",
		"/vol1/1000/someapp",
	} {
		if spaceIsRunningAppPath(p) {
			t.Errorf("spaceIsRunningAppPath(%q) = true, want false", p)
		}
	}
}

// TestNormalizeVolPath 覆盖清理路径归一化，它是所有删除操作的入口。
func TestNormalizeVolPath(t *testing.T) {
	ok := map[string]string{
		"/vol1/@appdata/x":     "/vol1/@appdata/x",
		"/vol1//@appdata//x":   "/vol1/@appdata/x",
		"/vol1/./@appdata/./x": "/vol1/@appdata/x",
		"/vol1/@appdata/x/":    "/vol1/@appdata/x",
	}
	for in, want := range ok {
		if got := normalizeVolPath(in); got != want {
			t.Errorf("normalizeVolPath(%q) = %q, want %q", in, got, want)
		}
	}
	bad := []string{
		"",
		"relative/path",
		"/vol1/../etc",
		"/vol1/@appdata/../../etc",
		"/vol1\\@appdata",
		"/vol1/@appdata/x\x00y",
		"/vol1/@appdata/x\ny",
		"vol1/@appdata",
	}
	for _, in := range bad {
		if got := normalizeVolPath(in); got != "" {
			t.Errorf("normalizeVolPath(%q) = %q, want rejected", in, got)
		}
	}
}

// TestVolRootOf 覆盖卷根提取，matchRuleForPath 依赖它做路径切分。
func TestVolRootOf(t *testing.T) {
	cases := map[string]string{
		"/vol1/@appdata": "vol1",
		"/vol12/x":       "vol12",
		"/vol1":          "vol1",
		"/volumex":       "",
		"/etc/passwd":    "",
		"":               "",
	}
	for in, want := range cases {
		if got := volRootOf(in); got != want {
			t.Errorf("volRootOf(%q) = %q, want %q", in, got, want)
		}
	}
}
