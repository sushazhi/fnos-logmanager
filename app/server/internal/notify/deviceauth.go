package notify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// 钉钉应用扫码注册（device registration）流程封装。
//
// 该协议对应钉钉开放平台的「扫码创建应用」能力，流程为三步：
//  1. POST /app/registration/init  —— 取 nonce
//  2. POST /app/registration/begin —— 用 nonce 换 device_code 与 verification_uri_complete
//  3. POST /app/registration/poll  —— 轮询 device_code，成功后返回 client_id / client_secret
//
// 返回的 client_id / client_secret 即渠道实现所需的 DD_BOT_TOKEN / DD_BOT_SECRET
// （群机器人 Webhook 场景下分别为 access_token 与加签密钥），避免用户手工复制。

const (
	dingTalkRegistrationBaseURL = "https://oapi.dingtalk.com"
	dingTalkRegistrationSource  = "DING_DWS_CLAW"

	// 注册会话有效期与轮询间隔的兜底值，防止服务端未返回时无限等待。
	dingTalkDefaultExpiresIn    = 7200
	dingTalkDefaultPollInterval = 5
)

// DeviceAuthError 表示一次可安全对外描述的注册失败。
type DeviceAuthError struct {
	Code   string
	Action string
	Err    error
}

func (e *DeviceAuthError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("钉钉注册失败(%s): %v", e.Action, e.Err)
	}
	return fmt.Sprintf("钉钉注册失败(%s)", e.Action)
}

func (e *DeviceAuthError) Unwrap() error { return e.Err }

// DingTalkRegistration 是一次已开始的扫码注册会话。
type DingTalkRegistration struct {
	DeviceCode      string `json:"deviceCode"`
	VerificationURL string `json:"verificationUrl"`
	UserCode        string `json:"userCode,omitempty"`
	ExpiresAt       int64  `json:"expiresAt"`
	PollIntervalMs  int    `json:"pollIntervalMs"`
}

// DingTalkRegistrationPoll 是一次轮询结果。
type DingTalkRegistrationPoll struct {
	// Status 取值为 WAITING / SUCCESS / FAIL / EXPIRED / UNKNOWN。
	Status       string `json:"status"`
	ClientID     string `json:"clientId,omitempty"`
	ClientSecret string `json:"clientSecret,omitempty"`
	FailReason   string `json:"failReason,omitempty"`
}

// 注册会话内存表：device_code 属于敏感信息，只在 Host 内部保留，不回传浏览器。
var (
	dingTalkRegMu   sync.Mutex
	dingTalkRegs    = map[string]*DingTalkRegistration{}
	dingTalkRegTTL  = 15 * time.Minute
	dingTalkHTTPCli = &http.Client{Timeout: 20 * time.Second}
)

// StartDingTalkRegistration 发起一次扫码注册，返回二维码所需信息。
func StartDingTalkRegistration() (*DingTalkRegistration, error) {
	initResp, err := dingTalkRegistrationPost("/app/registration/init", map[string]string{
		"source": dingTalkRegistrationSource,
	})
	if err != nil {
		return nil, &DeviceAuthError{Code: "init-failed", Action: "initialization", Err: err}
	}

	nonce, _ := initResp["nonce"].(string)
	nonce = strings.TrimSpace(nonce)
	if nonce == "" {
		return nil, &DeviceAuthError{
			Code:   "missing-nonce",
			Action: "initialization",
			Err:    fmt.Errorf("响应未包含 nonce"),
		}
	}

	beginResp, err := dingTalkRegistrationPost("/app/registration/begin", map[string]string{
		"nonce": nonce,
	})
	if err != nil {
		return nil, &DeviceAuthError{Code: "begin-failed", Action: "begin", Err: err}
	}

	deviceCode := strings.TrimSpace(asString(beginResp["device_code"]))
	verificationURL := strings.TrimSpace(asString(beginResp["verification_uri_complete"]))
	if deviceCode == "" || verificationURL == "" {
		return nil, &DeviceAuthError{
			Code:   "incomplete-registration",
			Action: "begin",
			Err:    fmt.Errorf("响应未包含完整的二维码信息"),
		}
	}

	expiresIn := asPositiveInt(beginResp["expires_in"], dingTalkDefaultExpiresIn)
	pollSeconds := asPositiveInt(beginResp["interval"], dingTalkDefaultPollInterval)

	reg := &DingTalkRegistration{
		DeviceCode:      deviceCode,
		VerificationURL: verificationURL,
		UserCode:        strings.TrimSpace(asString(beginResp["user_code"])),
		ExpiresAt:       time.Now().Add(time.Duration(expiresIn) * time.Second).UnixMilli(),
		PollIntervalMs:  pollSeconds * 1000,
	}

	dingTalkRegMu.Lock()
	pruneDingTalkRegsLocked()
	dingTalkRegs[deviceCode] = reg
	dingTalkRegMu.Unlock()

	return reg, nil
}

// PollDingTalkRegistration 轮询一次注册状态。成功后返回凭据。
func PollDingTalkRegistration(deviceCode string) (*DingTalkRegistrationPoll, error) {
	deviceCode = strings.TrimSpace(deviceCode)
	if deviceCode == "" {
		return nil, &DeviceAuthError{Code: "invalid-argument", Action: "poll", Err: fmt.Errorf("缺少 deviceCode")}
	}

	// 只接受本进程发起过的注册，避免把任意字符串当作 device_code 转发给上游。
	dingTalkRegMu.Lock()
	known := dingTalkRegs[deviceCode]
	dingTalkRegMu.Unlock()
	if known == nil {
		return nil, &DeviceAuthError{
			Code:   "unknown-attempt",
			Action: "poll",
			Err:    fmt.Errorf("注册会话不存在或已过期，请重新获取二维码"),
		}
	}

	resp, err := dingTalkRegistrationPost("/app/registration/poll", map[string]string{
		"device_code": deviceCode,
	})
	if err != nil {
		return nil, &DeviceAuthError{Code: "poll-failed", Action: "poll", Err: err}
	}

	rawStatus := strings.ToUpper(strings.TrimSpace(asString(resp["status"])))
	switch rawStatus {
	case "WAITING", "SUCCESS", "FAIL", "EXPIRED":
	default:
		rawStatus = "UNKNOWN"
	}

	result := &DingTalkRegistrationPoll{
		Status:       rawStatus,
		ClientID:     strings.TrimSpace(asString(resp["client_id"])),
		ClientSecret: strings.TrimSpace(asString(resp["client_secret"])),
		FailReason:   strings.TrimSpace(asString(resp["fail_reason"])),
	}

	// 终态即从内存表移除，成功凭据不留存。
	if rawStatus == "SUCCESS" || rawStatus == "FAIL" || rawStatus == "EXPIRED" {
		dingTalkRegMu.Lock()
		delete(dingTalkRegs, deviceCode)
		dingTalkRegMu.Unlock()
	}

	return result, nil
}

// CancelDingTalkRegistration 丢弃一次注册会话（用户刷新二维码或关闭面板）。
func CancelDingTalkRegistration(deviceCode string) {
	deviceCode = strings.TrimSpace(deviceCode)
	if deviceCode == "" {
		return
	}
	dingTalkRegMu.Lock()
	delete(dingTalkRegs, deviceCode)
	dingTalkRegMu.Unlock()
}

func pruneDingTalkRegsLocked() {
	now := time.Now().UnixMilli()
	for code, reg := range dingTalkRegs {
		if reg.ExpiresAt > 0 && now > reg.ExpiresAt+int64(dingTalkRegTTL/time.Millisecond) {
			delete(dingTalkRegs, code)
		}
	}
}

// dingTalkRegistrationPost 向钉钉注册端点发送 JSON POST 并校验 errcode。
func dingTalkRegistrationPost(path string, body map[string]string) (map[string]interface{}, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, dingTalkRegistrationBaseURL+path, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := dingTalkHTTPCli.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("响应不是合法 JSON (HTTP %d)", resp.StatusCode)
	}

	// 钉钉统一用 errcode 表达业务结果，0 为成功。
	if code, ok := parsed["errcode"]; ok {
		if n, ok := toFloat(code); ok && n != 0 {
			msg := asString(parsed["errmsg"])
			if msg == "" {
				msg = fmt.Sprintf("errcode=%v", code)
			}
			return nil, fmt.Errorf("钉钉返回错误: %s", msg)
		}
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	return parsed, nil
}

func asString(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func toFloat(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	}
	return 0, false
}

func asPositiveInt(v interface{}, fallback int) int {
	if n, ok := toFloat(v); ok && n > 0 {
		return int(n)
	}
	return fallback
}
