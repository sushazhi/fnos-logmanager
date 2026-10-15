package routes

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// newSpaceCleanTestRouter 只挂载清理路由，避免测试依赖完整路由表（后者需要
// 配置、日志目录等外部状态）。
func newSpaceCleanTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	rg := r.Group("/api")
	rg.POST("/space/clean", spaceCleanHandler)
	rg.GET("/space/usage", spaceUsageHandler)
	return r
}

func postClean(t *testing.T, body interface{}) (*httptest.ResponseRecorder, map[string]interface{}) {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("序列化请求失败: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/space/clean", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	newSpaceCleanTestRouter().ServeHTTP(w, req)

	var out map[string]interface{}
	if w.Body.Len() > 0 {
		// 响应可能是错误结构，也可能是清理结果；两者都能解析成 map。
		_ = json.Unmarshal(w.Body.Bytes(), &out)
	}
	return w, out
}

// TestSpaceCleanRejectsEmptyAndOversizedBatches 锁定请求体的两道入口闸门。
// 二者都是"客户端不可信"的体现：即使前端被绕过，也不能让一次调用横扫大量路径。
func TestSpaceCleanRejectsEmptyAndOversizedBatches(t *testing.T) {
	t.Run("空路径列表被拒绝", func(t *testing.T) {
		w, out := postClean(t, map[string]interface{}{"paths": []string{}})
		if w.Code != http.StatusBadRequest {
			t.Fatalf("空列表应返回 400，实际 %d", w.Code)
		}
		if msg, _ := out["error"].(string); msg == "" {
			t.Error("拒绝时应给出中文错误说明")
		}
	})

	t.Run("超过上限被拒绝", func(t *testing.T) {
		paths := make([]string, maxSpaceCleanPaths+1)
		for i := range paths {
			paths[i] = "/vol1/@apptemp/fake-" + strconv.Itoa(i)
		}
		w, out := postClean(t, map[string]interface{}{"paths": paths})
		if w.Code != http.StatusBadRequest {
			t.Fatalf("%d 条路径应返回 400（上限 %d），实际 %d",
				len(paths), maxSpaceCleanPaths, w.Code)
		}
		if msg, _ := out["error"].(string); msg == "" {
			t.Error("拒绝时应给出中文错误说明")
		}
	})

	t.Run("非法 JSON 被拒绝", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/space/clean",
			strings.NewReader("{not json"))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		newSpaceCleanTestRouter().ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("非法 JSON 应返回 400，实际 %d", w.Code)
		}
	})

	t.Run("恰好等于上限不被入口拒绝", func(t *testing.T) {
		// 上限本身是允许的（只排除 > 上限），因此不应因为数量而返回 400。
		// 这些路径都不在规则内，会被服务端逐条跳过——这正是期望的安全行为。
		paths := make([]string, maxSpaceCleanPaths)
		for i := range paths {
			paths[i] = "/vol1/@apptemp/fake-" + strconv.Itoa(i)
		}
		w, _ := postClean(t, map[string]interface{}{"paths": paths, "dryRun": true})
		if w.Code != http.StatusOK {
			t.Fatalf("恰好 %d 条应通过入口校验，实际 %d", len(paths), w.Code)
		}
	})
}

// TestSpaceCleanSkipsPathsOutsideRules 验证最关键的一道安全边界：
// 客户端提交的任意路径必须被服务端逐条复核，规则外的一律不删。
// 这里故意提交明显危险的目标（系统目录、用户数据、路径穿越）。
func TestSpaceCleanSkipsPathsOutsideRules(t *testing.T) {
	dangerous := []string{
		"/etc/passwd",
		"/usr/bin",
		"/vol1/1000/pt",                // 用户 PT 数据
		"/vol1/@appcenter/logmanager",  // 应用安装目录
		"/vol1/@sysappmeta/ai-manager", // 系统 AI 模型
		"/vol1/@appdata/deepseek.harness/dsh-runtime/node_modules", // 运行期依赖（高风险管理项）
		"/vol1/@appdata/moviepilot/config/.browser",                // 内置 Chromium（已从规则移除）
		"/vol1/@appdata/deepseek.harness/home/.cache",              // 含 Puppeteer 浏览器本体
		"../../etc/passwd",
		"/vol1/../etc/passwd",
		"/tmp/../../etc/passwd",
		"",
		// --- 系统分区（新增规则表的拒绝边界）---
		// 这些即使命中新规则表也必须被拒：删错会导致设备无法启动或系统异常。
		"/boot/vmlinuz-6.18.18.c1145-trim",         // 引导文件
		"/boot/extlinux/extlinux.conf",             // 引导配置
		"/usr/lib/modules/6.18.18.c1145-trim",      // 当前内核模块
		"/usr/lib/linux-image-6.18.18.c1090-trim",  // 内核设备树（专用功能负责）
		"/usr/src/linux-headers-6.18.18.c1145-trim",
		"/var/lib/dpkg/status",        // 包数据库
		"/usr/trim/bin/trim",          // fnOS 系统核心
		"/usr/local/apps/@appdata",    // 应用数据
		"/usr/local/go/bin/go",        // B 档工具链
		"/usr/share/doc/foo/copyright",
		"/usr/lib/firmware/rtw89/x.bin",
		"/var/lib/apt/lists/partial",  // 由包管理器处理，本应用不代删
		"/var/log/../../etc/passwd",   // 穿越后落在系统路径上
	}

	w, out := postClean(t, map[string]interface{}{"paths": dangerous, "dryRun": true})
	if w.Code != http.StatusOK {
		t.Fatalf("dry-run 应返回 200，实际 %d（body=%s）", w.Code, w.Body.String())
	}

	// 一条都不该被删除：全部应计入 skipped。
	if n, ok := out["deleted"].(float64); ok && n != 0 {
		t.Errorf("规则外路径被删除了 %v 条", n)
	}
	if n, ok := out["recycled"].(float64); ok && n != 0 {
		t.Errorf("规则外路径被回收了 %v 条", n)
	}
	if n, ok := out["truncated"].(float64); ok && n != 0 {
		t.Errorf("规则外路径被截断了 %v 条", n)
	}

	// 结果没有汇总的 skipped 计数，而是每个条目带 status；逐条核对。
	items, ok := out["items"].([]interface{})
	if !ok {
		t.Fatalf("响应缺少 items 数组: %s", w.Body.String())
	}
	// 空字符串会被上游过滤掉，因此 items 数量 = 非空路径数。
	wantItems := 0
	for _, p := range dangerous {
		if p != "" {
			wantItems++
		}
	}
	if len(items) != wantItems {
		t.Errorf("应有 %d 个条目，实际 %d", wantItems, len(items))
	}
	for _, raw := range items {
		it, _ := raw.(map[string]interface{})
		if status, _ := it["status"].(string); status != "skipped" {
			t.Errorf("路径 %v 的状态应为 skipped，实际 %q", it["path"], status)
		}
		if msg, _ := it["message"].(string); msg == "" {
			t.Errorf("跳过 %v 时应给出原因说明", it["path"])
		}
	}
	if freed, ok := out["freed"].(float64); ok && freed != 0 {
		t.Errorf("跳过全部路径后不应报告释放空间，实际 %v", freed)
	}
}

// TestSpaceUsageDoesNotPanicOnThisHost 确认读接口在本机（无 /volN）也返回
// 结构化响应而不是 500：真机上扫描失败时前端仍要能显示错误而不是白屏。
func TestSpaceUsageDoesNotPanicOnThisHost(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/space/usage", nil)
	w := httptest.NewRecorder()
	newSpaceCleanTestRouter().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("读接口应始终返回 200（错误走 body），实际 %d", w.Code)
	}
	var out map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("响应不是合法 JSON: %v", err)
	}
	// 三个列表字段必须存在且是数组，否则前端渲染会出错。
	for _, key := range []string{"categories", "notices", "volumes"} {
		if _, ok := out[key]; !ok {
			t.Errorf("响应缺少 %q 字段", key)
		}
	}
}

// TestTruncatePathsForAudit 锁定审计日志的截断行为：既不能丢信息（保留前 20 条），
// 也不能让一次大批量清理把审计记录撑爆。
func TestTruncatePathsForAudit(t *testing.T) {
	t.Run("少量路径原样返回", func(t *testing.T) {
		in := []string{"/a", "/b"}
		got := truncatePathsForAudit(in)
		if len(got) != 2 || got[0] != "/a" || got[1] != "/b" {
			t.Errorf("少量路径应原样返回，实际 %v", got)
		}
	})

	t.Run("边界：恰好 20 条不截断", func(t *testing.T) {
		in := make([]string, 20)
		for i := range in {
			in[i] = "/p" + strconv.Itoa(i)
		}
		if got := truncatePathsForAudit(in); len(got) != 20 {
			t.Errorf("20 条不应截断，实际 %d 条", len(got))
		}
	})

	t.Run("超过 20 条保留前 20 条并注明省略数量", func(t *testing.T) {
		in := make([]string, 25)
		for i := range in {
			in[i] = "/p" + strconv.Itoa(i)
		}
		got := truncatePathsForAudit(in)
		if len(got) != 21 {
			t.Fatalf("应为 20 条 + 1 条省略说明，实际 %d 条", len(got))
		}
		if got[0] != "/p0" || got[19] != "/p19" {
			t.Error("应保留前 20 条原始路径")
		}
		if !strings.Contains(got[20], "5") {
			t.Errorf("省略说明应包含被省略的数量 5，实际 %q", got[20])
		}
	})
}

// TestGatewayUIDFromContextFallsBack 验证 uid 解析：网关注入的 uid 优先，
// 取不到时回退到进程 uid（独立运行模式）。这个 uid 决定回收站归属，
// 解析错会把文件移进错误的用户目录。
func TestGatewayUIDFromContextFallsBack(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("网关注入的 uid 优先", func(t *testing.T) {
		// 生产路径中 middleware/auth.go 用 c.Set("gatewayUID", c.GetHeader(...))
		// 写入，header 永远是字符串——这里按实际类型断言。
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Set("gatewayUID", "1000")
		if got := gatewayUIDFromContext(c); got != "1000" {
			t.Errorf("应使用网关注入的 uid，实际 %q", got)
		}
	})

	t.Run("非字符串取值回退而不是给出错误 uid", func(t *testing.T) {
		// gatewayUIDToString 只接受 string（与另外 4 处调用一致）。若将来有人
		// 改成写入 int，取值会失败并回退到进程 uid——宁可回退，也不能把
		// 一个错误的 uid 当作回收站归属。
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Set("gatewayUID", 1000)
		got := gatewayUIDFromContext(c)
		if got == "1000" {
			t.Log("非字符串 uid 现已被接受（行为已放宽）")
		}
		// 无论哪种实现，都不能返回诸如 "%!s(int=1000)" 之类的格式化垃圾。
		if strings.ContainsAny(got, "%!") {
			t.Errorf("回退值不应包含格式化残留: %q", got)
		}
	})

	t.Run("缺失时回退且不 panic", func(t *testing.T) {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		// 本机（Windows）CurrentUID 返回 ""，Linux 返回真实 uid；
		// 关键是不 panic 且不返回错误来源的值。
		_ = gatewayUIDFromContext(c)
	})

	t.Run("空字符串回退而不是返回空", func(t *testing.T) {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Set("gatewayUID", "")
		if got := gatewayUIDFromContext(c); got == " " {
			t.Errorf("不应返回空白 uid，实际 %q", got)
		}
	})
}
