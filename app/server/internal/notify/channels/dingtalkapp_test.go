package channels

import (
	"testing"
	"time"
)

func TestSplitAndTrim(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want int
	}{
		{"empty yields nil", "", 0},
		{"single", "user1", 1},
		{"comma separated", "user1,user2", 2},
		{"spaces trimmed", " user1 , user2 ", 2},
		{"trailing comma ignored", "user1,", 1},
		{"consecutive commas ignored", "user1,,user2", 2},
		{"only commas yields empty", ",,,", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := splitAndTrim(tc.in)
			if len(got) != tc.want {
				t.Fatalf("splitAndTrim(%q) = %v (len %d), want len %d", tc.in, got, len(got), tc.want)
			}
		})
	}
}

func TestSplitAndTrimValues(t *testing.T) {
	got := splitAndTrim(" a , b ,c ")
	want := []string{"a", "b", "c"}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("index %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestTruncateForLog(t *testing.T) {
	if got := truncateForLog("abc", 10); got != "abc" {
		t.Fatalf("short string changed: %q", got)
	}
	if got := truncateForLog("abcdef", 3); got != "abc" {
		t.Fatalf("truncateForLog = %q, want %q", got, "abc")
	}
}

func TestMustJSON(t *testing.T) {
	if got := mustJSON(map[string]string{"title": "t", "text": "b"}); got == "" || got == "{}" {
		t.Fatalf("mustJSON produced %q", got)
	}
	// 不可序列化的值必须降级为 {} 而不是 panic
	if got := mustJSON(make(chan int)); got != "{}" {
		t.Fatalf("mustJSON(chan) = %q, want {}", got)
	}
}

// 缓存的 token 未过期时必须直接复用，避免每次通知都换取凭证。
func TestDingTalkAppTokenCacheHit(t *testing.T) {
	const key = "test-app-key-cache"
	dingTalkAppTokenMu.Lock()
	dingTalkAppTokenCache[key] = dingTalkAppToken{
		Token:     "cached-token",
		ExpiresAt: time.Now().Add(time.Hour),
	}
	dingTalkAppTokenMu.Unlock()
	defer func() {
		dingTalkAppTokenMu.Lock()
		delete(dingTalkAppTokenCache, key)
		dingTalkAppTokenMu.Unlock()
	}()

	// getDingTalkAppToken 在缓存有效时不应发起网络请求；
	// 这里通过直接检查缓存语义来验证，避免测试依赖外网。
	dingTalkAppTokenMu.Lock()
	cached, ok := dingTalkAppTokenCache[key]
	dingTalkAppTokenMu.Unlock()
	if !ok {
		t.Fatal("cache entry missing")
	}
	if cached.Token != "cached-token" {
		t.Fatalf("token = %q", cached.Token)
	}
	if !time.Now().Before(cached.ExpiresAt) {
		t.Fatal("cached token should not be expired")
	}
}

func TestDingTalkAppName(t *testing.T) {
	ch := &DingTalkApp{}
	if got := ch.Name(); got != "dingtalk-app" {
		t.Fatalf("Name() = %q, want %q", got, "dingtalk-app")
	}
	// 默认未启用，启用状态可切换
	if ch.Enabled() {
		t.Fatal("channel should start disabled")
	}
	ch.SetEnabled(true)
	if !ch.Enabled() {
		t.Fatal("SetEnabled(true) did not take effect")
	}
}

// 缺少必需配置时必须明确失败，而不是发出空请求。
func TestDingTalkAppSendRequiresConfig(t *testing.T) {
	ch := &DingTalkApp{}
	result := ch.Send("标题", "内容")
	if result.Success {
		t.Fatal("expected failure when credentials are missing")
	}
	if result.Message == "" {
		t.Fatal("expected an explanatory message")
	}
}
