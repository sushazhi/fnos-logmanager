package routes

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// 本文件锁定 /api/space/usage 的对外 JSON 契约。
//
// 前端（SpaceCleanPanel.vue / api.ts）依赖这几个时效字段决定「显示骨架屏」还是
// 「显示旧数据 + 后台更新中 + 轮询收敛」。字段一旦漏发就会变成 undefined，
// 组件会静默退化回阻塞式扫描——也就是说，这类回归不会报错，只会让体验悄悄变差。
// 因此这里在 HTTP 层断言字段必然存在。

func getUsageRaw(t *testing.T, query string) (int, []byte) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/space/usage"+query, nil)
	w := httptest.NewRecorder()
	newSpaceCleanTestRouter().ServeHTTP(w, req)
	return w.Code, w.Body.Bytes()
}

// TestSpaceUsageResponseAlwaysCarriesFreshnessFields 覆盖成功与失败两条路径。
//
// 两条路径都要有：前端在 error 分支下同样会读这些字段。
func TestSpaceUsageResponseAlwaysCarriesFreshnessFields(t *testing.T) {
	// 开发机（Windows）上没有 /volN，成功分支拿不到真实数据；这里只断言
	// "只要能返回 200 JSON，时效字段就必须在"，无论它是成功结果还是错误对象。
	code, body := getUsageRaw(t, "")
	if code != http.StatusOK {
		t.Fatalf("接口应始终返回 200（错误也以 error 字段表达），实际 %d", code)
	}

	var payload map[string]json.RawMessage
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("响应不是合法 JSON: %v (body=%s)", err, body)
	}

	for _, key := range []string{"cacheAgeMs", "cacheAgeSeconds", "stale", "refreshing"} {
		raw, ok := payload[key]
		if !ok {
			t.Fatalf("响应缺少时效字段 %q（body=%s）", key, body)
		}
		if string(raw) == "null" {
			t.Fatalf("时效字段 %q 不应为 null", key)
		}
	}

	// 类型也要对：前端把它们当数字/布尔用，字符串会让判断静默失效。
	var ageMs int64
	if err := json.Unmarshal(payload["cacheAgeMs"], &ageMs); err != nil {
		t.Fatalf("cacheAgeMs 应为整数: %v", err)
	}
	var stale, refreshing bool
	if err := json.Unmarshal(payload["stale"], &stale); err != nil {
		t.Fatalf("stale 应为布尔值: %v", err)
	}
	if err := json.Unmarshal(payload["refreshing"], &refreshing); err != nil {
		t.Fatalf("refreshing 应为布尔值: %v", err)
	}
}

// TestSpaceUsageRefreshQueryIsAccepted 锁定 force 语义的入口：
// refresh=true / refresh=1 都要被识别，其余取值不能意外触发强制扫描。
//
// 这里只验证请求被正常处理（不因参数形态 4xx/5xx）：真正的强制扫描行为由
// services 层的 TestScanSpaceUsageForceWaitsForScan 覆盖。
func TestSpaceUsageRefreshQueryIsAccepted(t *testing.T) {
	for _, query := range []string{"", "?refresh=true", "?refresh=1", "?refresh=false", "?refresh=yes"} {
		code, body := getUsageRaw(t, query)
		if code != http.StatusOK {
			t.Fatalf("query=%q 应返回 200，实际 %d", query, code)
		}
		if len(body) == 0 {
			t.Fatalf("query=%q 返回了空响应", query)
		}
	}
}
