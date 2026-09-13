package notify

import (
	"encoding/json"
	"testing"
)

func TestAsPositiveInt(t *testing.T) {
	cases := []struct {
		name     string
		value    interface{}
		fallback int
		want     int
	}{
		{"float from json", float64(300), 5, 300},
		{"json number", json.Number("42"), 5, 42},
		{"int value", 7, 5, 7},
		{"zero falls back", float64(0), 5, 5},
		{"negative falls back", float64(-3), 5, 5},
		{"nil falls back", nil, 5, 5},
		{"string falls back", "abc", 5, 5},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := asPositiveInt(tc.value, tc.fallback); got != tc.want {
				t.Fatalf("asPositiveInt(%v, %d) = %d, want %d", tc.value, tc.fallback, got, tc.want)
			}
		})
	}
}

func TestAsString(t *testing.T) {
	if got := asString("hello"); got != "hello" {
		t.Fatalf("asString(string) = %q", got)
	}
	// 非字符串必须归一为空串，避免把 nil 拼进凭据
	if got := asString(nil); got != "" {
		t.Fatalf("asString(nil) = %q, want empty", got)
	}
	if got := asString(123); got != "" {
		t.Fatalf("asString(number) = %q, want empty", got)
	}
}

func TestCancelDingTalkRegistration(t *testing.T) {
	const code = "test-device-code-cancel"

	dingTalkRegMu.Lock()
	dingTalkRegs[code] = &DingTalkRegistration{DeviceCode: code}
	dingTalkRegMu.Unlock()

	CancelDingTalkRegistration(code)

	dingTalkRegMu.Lock()
	_, exists := dingTalkRegs[code]
	dingTalkRegMu.Unlock()
	if exists {
		t.Fatal("CancelDingTalkRegistration did not remove the attempt")
	}

	// 空字符串与未登记的值都不应 panic
	CancelDingTalkRegistration("")
	CancelDingTalkRegistration("never-registered")
}

// 轮询必须只接受本进程发起过的注册，防止把任意字符串透传给上游。
func TestPollDingTalkRegistrationRejectsUnknownAttempt(t *testing.T) {
	_, err := PollDingTalkRegistration("not-a-real-device-code")
	if err == nil {
		t.Fatal("expected an error for an unknown device code")
	}
	var authErr *DeviceAuthError
	if !asDeviceAuthError(err, &authErr) {
		t.Fatalf("expected *DeviceAuthError, got %T", err)
	}
	if authErr.Code != "unknown-attempt" {
		t.Fatalf("Code = %q, want %q", authErr.Code, "unknown-attempt")
	}
}

func TestPollDingTalkRegistrationRejectsEmptyCode(t *testing.T) {
	_, err := PollDingTalkRegistration("   ")
	if err == nil {
		t.Fatal("expected an error for an empty device code")
	}
	var authErr *DeviceAuthError
	if !asDeviceAuthError(err, &authErr) {
		t.Fatalf("expected *DeviceAuthError, got %T", err)
	}
	if authErr.Code != "invalid-argument" {
		t.Fatalf("Code = %q, want %q", authErr.Code, "invalid-argument")
	}
}

func TestPruneDingTalkRegsRemovesExpired(t *testing.T) {
	const expired = "test-device-code-expired"
	const fresh = "test-device-code-fresh"

	dingTalkRegMu.Lock()
	// ExpiresAt 远早于当前时间，超出保留窗口后应被清理
	dingTalkRegs[expired] = &DingTalkRegistration{DeviceCode: expired, ExpiresAt: 1}
	dingTalkRegs[fresh] = &DingTalkRegistration{DeviceCode: fresh, ExpiresAt: 0}
	pruneDingTalkRegsLocked()
	_, expiredExists := dingTalkRegs[expired]
	_, freshExists := dingTalkRegs[fresh]
	delete(dingTalkRegs, fresh)
	dingTalkRegMu.Unlock()

	if expiredExists {
		t.Fatal("pruneDingTalkRegsLocked did not remove the expired attempt")
	}
	// ExpiresAt 为 0 表示服务端未返回有效期，不应被误删
	if !freshExists {
		t.Fatal("pruneDingTalkRegsLocked removed an attempt without an expiry")
	}
}

// asDeviceAuthError 是本测试内的类型断言辅助，避免直接 import errors 仅为一处使用。
func asDeviceAuthError(err error, target **DeviceAuthError) bool {
	if e, ok := err.(*DeviceAuthError); ok {
		*target = e
		return true
	}
	return false
}
