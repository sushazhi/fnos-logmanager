package channels

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/sushazhi/fnos-logmanager/internal/notify"
)

// 钉钉企业内部应用（新版 API，api.dingtalk.com）。
//
// 与「钉钉机器人」（dingtalk.go，群机器人 Webhook）是两套不同的接入方式：
//   - 群机器人：固定 Webhook + access_token，直接 POST 即可，无需换取凭证。
//   - 企业内部应用：用 AppKey(AppID) + AppSecret 换取 accessToken，
//     再调用机器人发消息接口主动推送给指定用户。
//
// 本渠道对应扫码注册（/app/registration/*）产出的 AppKey / AppSecret。

const dingTalkAppAPIBase = "https://api.dingtalk.com"

// accessToken 有效期约 7200 秒，提前 5 分钟刷新，避免边界失效。
const dingTalkAppTokenRefreshAhead = 5 * time.Minute

var (
	dingTalkAppTokenMu     sync.Mutex
	dingTalkAppTokenCache  = map[string]dingTalkAppToken{}
	dingTalkAppHTTPTimeout = 15 * time.Second
)

type dingTalkAppToken struct {
	Token     string
	ExpiresAt time.Time
}

// DingTalkApp 是钉钉企业内部应用渠道。
type DingTalkApp struct {
	enabled bool
}

func (c *DingTalkApp) Name() string {
	return "dingtalk-app"
}

func (c *DingTalkApp) Enabled() bool {
	return c.enabled
}

func (c *DingTalkApp) SetEnabled(enabled bool) {
	c.enabled = enabled
}

func (c *DingTalkApp) Send(text, desp string) notify.NotifyResult {
	if !notify.HasConfig("DD_APP_KEY", "DD_APP_SECRET") {
		return notify.NotifyResult{
			Success: false,
			Message: "DD_APP_KEY 或 DD_APP_SECRET 未配置，请先扫码创建应用",
		}
	}

	appKey := notify.GetConfig("DD_APP_KEY")
	appSecret := notify.GetConfig("DD_APP_SECRET")
	robotCode := notify.GetConfig("DD_APP_ROBOT_CODE")
	if robotCode == "" {
		// 机器人 code 通常与应用 AppKey 一致，未单独配置时回落到 AppKey。
		robotCode = appKey
	}
	userIDs := splitAndTrim(notify.GetConfig("DD_APP_USER_IDS"))

	if len(userIDs) == 0 {
		return notify.NotifyResult{
			Success: false,
			Message: "DD_APP_USER_IDS 未配置，无法确定推送目标",
		}
	}

	token, err := getDingTalkAppToken(appKey, appSecret)
	if err != nil {
		slog.Error("钉钉应用获取 accessToken 失败", "error", err)
		return notify.NotifyResult{Success: false, Message: err.Error(), Err: err}
	}

	title := text
	if len([]rune(title)) > 64 {
		title = string([]rune(title)[:64])
	}

	msg := map[string]interface{}{
		"msgtype": "markdown",
		"markdown": map[string]string{
			"title": title,
			"text":  text + "\n\n" + desp,
		},
	}

	failed := 0
	// unknown 统计「请求已发出但结果不确定」的次数：这类失败重试可能造成
	// 重复消息，因此不能与明确失败同等对待。
	unknown := 0
	var lastErr string
	for _, userID := range userIDs {
		body := map[string]interface{}{
			"robotCode": robotCode,
			"userIds":   []string{userID},
			"msgKey":    "sampleMarkdown",
			"msgParam":  mustJSON(msg["markdown"]),
		}

		resp, err := notify.HTTPPost(dingTalkAppAPIBase+"/v1.0/robot/oToMessages/batchSend", notify.HttpRequestOptions{
			JSON:    body,
			Headers: map[string]string{"x-acs-dingtalk-access-token": token},
		})
		if err != nil {
			// 超时/连接中断：请求可能已被服务端处理，结果未知
			if isUncertainNetworkError(err) {
				unknown++
			} else {
				failed++
			}
			lastErr = err.Error()
			slog.Warn("钉钉应用发送失败", "userID", userID, "error", err)
			continue
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			// 服务端明确返回了状态码，属于可判定的失败
			failed++
			lastErr = fmt.Sprintf("HTTP %d: %s", resp.StatusCode, truncateForLog(resp.Body, 200))
			slog.Warn("钉钉应用发送失败", "userID", userID, "status", resp.StatusCode)
		}
	}

	if failed == 0 && unknown == 0 {
		slog.Info("钉钉应用发送成功", "count", len(userIDs))
		return notify.NotifyResult{Success: true, Message: "发送成功", Outcome: notify.OutcomeDelivered}
	}
	if failed == 0 && unknown > 0 {
		return notify.NotifyResult{
			Success: false,
			Message: fmt.Sprintf("结果未确认: %d/%d 未能确认是否送达", unknown, len(userIDs)),
			Outcome: notify.OutcomeUnknown,
		}
	}
	if failed+unknown == len(userIDs) {
		if lastErr == "" {
			lastErr = "发送失败"
		}
		return notify.NotifyResult{Success: false, Message: lastErr, Outcome: notify.OutcomeFailed}
	}
	// 部分成功：报告实际结果，避免调用方误认为全部送达。
	return notify.NotifyResult{
		Success: true,
		Message: fmt.Sprintf("部分成功: %d/%d", len(userIDs)-failed-unknown, len(userIDs)),
		Outcome: notify.OutcomeDelivered,
	}
}

// isUncertainNetworkError 判断错误是否属于「请求可能已送达但无法确认」。
// 超时与连接中断都可能发生在服务端已处理之后，因此重试有重复风险。
func isUncertainNetworkError(err error) bool {
	if err == nil {
		return false
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	msg := strings.ToLower(err.Error())
	for _, marker := range []string{"timeout", "timed out", "deadline exceeded", "connection reset", "eof", "broken pipe"} {
		if strings.Contains(msg, marker) {
			return true
		}
	}
	return false
}

// getDingTalkAppToken 换取 accessToken，带进程内缓存。
func getDingTalkAppToken(appKey, appSecret string) (string, error) {
	cacheKey := appKey

	dingTalkAppTokenMu.Lock()
	if cached, ok := dingTalkAppTokenCache[cacheKey]; ok {
		if time.Now().Before(cached.ExpiresAt) {
			token := cached.Token
			dingTalkAppTokenMu.Unlock()
			return token, nil
		}
		delete(dingTalkAppTokenCache, cacheKey)
	}
	dingTalkAppTokenMu.Unlock()

	resp, err := notify.HTTPPost(dingTalkAppAPIBase+"/v1.0/oauth2/accessToken", notify.HttpRequestOptions{
		JSON: map[string]string{
			"appKey":    appKey,
			"appSecret": appSecret,
		},
	})
	if err != nil {
		return "", fmt.Errorf("获取 accessToken 失败: %w", err)
	}

	var result struct {
		AccessToken string `json:"accessToken"`
		ExpireIn    int    `json:"expireIn"`
		Code        string `json:"code"`
		Message     string `json:"message"`
	}
	if err := json.Unmarshal([]byte(resp.Body), &result); err != nil {
		return "", fmt.Errorf("解析 accessToken 响应失败 (HTTP %d)", resp.StatusCode)
	}

	if result.AccessToken == "" {
		msg := result.Message
		if msg == "" {
			msg = truncateForLog(resp.Body, 200)
		}
		return "", fmt.Errorf("获取 accessToken 失败: %s", msg)
	}

	expireIn := result.ExpireIn
	if expireIn <= 0 {
		expireIn = 7200
	}

	dingTalkAppTokenMu.Lock()
	dingTalkAppTokenCache[cacheKey] = dingTalkAppToken{
		Token:     result.AccessToken,
		ExpiresAt: time.Now().Add(time.Duration(expireIn)*time.Second - dingTalkAppTokenRefreshAhead),
	}
	dingTalkAppTokenMu.Unlock()

	return result.AccessToken, nil
}

// splitAndTrim 按逗号切分并去除空白，忽略空项。
func splitAndTrim(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if v := strings.TrimSpace(p); v != "" {
			out = append(out, v)
		}
	}
	return out
}

func mustJSON(v interface{}) string {
	raw, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(raw)
}

func truncateForLog(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}

func init() {
	ch := &DingTalkApp{}
	if notify.HasConfig("DD_APP_KEY", "DD_APP_SECRET") {
		ch.SetEnabled(true)
	}
	notify.Registry.Register(ch)
}
