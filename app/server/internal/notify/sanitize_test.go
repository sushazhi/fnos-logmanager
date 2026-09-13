package notify

import (
	"strings"
	"testing"
)

func TestTruncateRunesDoesNotSplitChinese(t *testing.T) {
	// 按字节截断会把 3 字节的中文切成半个字符；按字符截断必须完整保留。
	s := "这是一段中文内容用于验证截断不会切碎多字节字符"
	got := TruncateRunes(s, 5)

	if want := "这是一段中"; got != want {
		t.Fatalf("TruncateRunes = %q, want %q", got, want)
	}
	// 结果必须仍是合法 UTF-8（不含替换字符）
	if strings.ContainsRune(got, '\uFFFD') {
		t.Fatalf("truncation produced invalid UTF-8: %q", got)
	}
	if len([]rune(got)) != 5 {
		t.Fatalf("rune length = %d, want 5", len([]rune(got)))
	}
}

func TestTruncateRunesEdgeCases(t *testing.T) {
	if got := TruncateRunes("abc", 10); got != "abc" {
		t.Fatalf("short string changed: %q", got)
	}
	if got := TruncateRunes("abc", 0); got != "" {
		t.Fatalf("max=0 should yield empty, got %q", got)
	}
	if got := TruncateRunes("abc", -1); got != "" {
		t.Fatalf("negative max should yield empty, got %q", got)
	}
	if got := TruncateRunes("", 5); got != "" {
		t.Fatalf("empty string should stay empty, got %q", got)
	}
}

func TestSanitizeForMessageMasksAssignments(t *testing.T) {
	cases := []struct {
		name  string
		input string
		leak  string
	}{
		{"token assignment", "request failed: token=abcdef123456", "abcdef123456"},
		{"secret assignment", `{"client_secret": "supersecretvalue"}`, "supersecretvalue"},
		{"access token", "access_token=zzzz9999 rejected", "zzzz9999"},
		{"password", "password: hunter2hunter2", "hunter2hunter2"},
		{"signature", "sign=QUJDREVGR0hJSktMTU5PUA==", "QUJDREVGR0hJSktMTU5PUA=="},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := SanitizeForMessage(tc.input)
			if strings.Contains(got, tc.leak) {
				t.Fatalf("sanitized output still contains the credential %q: %q", tc.leak, got)
			}
		})
	}
}

func TestSanitizeForMessageKeepsContext(t *testing.T) {
	// 脱敏不应把整条消息清空——排错仍需要看到错误的结构与状态码。
	got := SanitizeForMessage("HTTP 401: token=abc123def456 unauthorized")
	if !strings.Contains(got, "HTTP 401") {
		t.Fatalf("sanitizer removed useful context: %q", got)
	}
	if strings.Contains(got, "abc123def456") {
		t.Fatalf("credential leaked: %q", got)
	}
}

func TestSanitizeForMessageEmpty(t *testing.T) {
	if got := SanitizeForMessage(""); got != "" {
		t.Fatalf("empty input should yield empty, got %q", got)
	}
}

func TestOutcomeResolution(t *testing.T) {
	cases := []struct {
		name    string
		result  NotifyResult
		outcome Outcome
		retry   bool
	}{
		{"explicit delivered", NotifyResult{Success: true, Outcome: OutcomeDelivered}, OutcomeDelivered, false},
		{"explicit failed", NotifyResult{Success: false, Outcome: OutcomeFailed}, OutcomeFailed, true},
		{"explicit unknown", NotifyResult{Success: false, Outcome: OutcomeUnknown}, OutcomeUnknown, false},
		// 尚未设置 Outcome 的既有渠道实现按 Success 推导
		{"legacy success", NotifyResult{Success: true}, OutcomeDelivered, false},
		{"legacy failure", NotifyResult{Success: false}, OutcomeFailed, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.result.ResolveOutcome(); got != tc.outcome {
				t.Fatalf("ResolveOutcome = %q, want %q", got, tc.outcome)
			}
			if got := tc.result.Retryable(); got != tc.retry {
				t.Fatalf("Retryable = %v, want %v", got, tc.retry)
			}
		})
	}
}

// 结果未知绝不能被当作可重试：超时可能已经送达，重试会造成重复消息。
func TestUnknownOutcomeIsNotRetryable(t *testing.T) {
	r := NotifyResult{Success: false, Outcome: OutcomeUnknown}
	if r.Retryable() {
		t.Fatal("OutcomeUnknown must not be retryable")
	}
}

func TestSendNotifyResultAggregation(t *testing.T) {
	r := SendNotifyResult{
		Total:     3,
		Delivered: 2,
		Failed:    1,
		Results: []ChannelOutcome{
			{Name: "a", Outcome: OutcomeDelivered},
			{Name: "b", Outcome: OutcomeDelivered},
			{Name: "c", Outcome: OutcomeFailed},
		},
	}
	if r.AllDelivered() {
		t.Fatal("AllDelivered should be false when a channel failed")
	}
	if !r.HasRetryable() {
		t.Fatal("HasRetryable should be true with one definite failure")
	}
	if got := r.Summary(); got != "2/3" {
		t.Fatalf("Summary = %q, want %q", got, "2/3")
	}

	// 仅存在未知渠道时：不算全部送达，但也不可重试
	u := SendNotifyResult{
		Total:   1,
		Unknown: 1,
		Results: []ChannelOutcome{{Name: "a", Outcome: OutcomeUnknown}},
	}
	if u.AllDelivered() {
		t.Fatal("AllDelivered should be false with an unknown channel")
	}
	if u.HasRetryable() {
		t.Fatal("HasRetryable should be false when only unknown outcomes exist")
	}
	if got := u.Summary(); !strings.Contains(got, "未知") {
		t.Fatalf("Summary should mention unknown: %q", got)
	}
}

// 空渠道列表不能被判定为「全部送达」。
func TestSendNotifyResultEmptyIsNotSuccess(t *testing.T) {
	var r SendNotifyResult
	if r.AllDelivered() {
		t.Fatal("empty result must not be AllDelivered")
	}
}
