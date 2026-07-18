package channels

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	"github.com/sushazhi/fnos-logmanager/internal/notify"
)

type IGot struct {
	enabled bool
}

func (c *IGot) Name() string            { return "igot" }
func (c *IGot) Enabled() bool           { return c.enabled }
func (c *IGot) SetEnabled(enabled bool) { c.enabled = enabled }

func (c *IGot) Send(text, desp string) notify.NotifyResult {
	if !notify.HasConfig("IGOT_PUSH_KEY") {
		return notify.NotifyResult{Success: false, Message: "IGOT_PUSH_KEY 未配置"}
	}

	pushKey := notify.GetConfig("IGOT_PUSH_KEY")

	// Validate key format (24 alphanumeric characters)
	matched, _ := regexp.MatchString("^[a-zA-Z0-9]{24}$", pushKey)
	if !matched {
		return notify.NotifyResult{Success: false, Message: "IGOT_PUSH_KEY 无效"}
	}

	url := fmt.Sprintf("https://push.hellyw.com/%s", strings.ToLower(pushKey))
	body := fmt.Sprintf("title=%s&content=%s", text, desp)

	resp, err := notify.HTTPPost(url, notify.HttpRequestOptions{
		Body:    body,
		Headers: map[string]string{"Content-Type": "application/x-www-form-urlencoded"},
	})
	if err != nil {
		slog.Error("iGot 发送失败", "err", err)
		return notify.NotifyResult{Success: false, Message: fmt.Sprintf("发送失败: %v", err), Err: err}
	}

	var result struct {
		Ret    float64 `json:"ret"`
		ErrMsg string  `json:"errMsg"`
	}
	if err := json.Unmarshal([]byte(resp.Body), &result); err != nil {
		slog.Error("iGot 解析响应失败", "err", err)
		return notify.NotifyResult{Success: false, Message: "解析响应失败", Err: err}
	}

	if result.Ret == 0 {
		slog.Info("iGot 发送成功")
		return notify.NotifyResult{Success: true, Message: "发送成功"}
	}

	msg := result.ErrMsg
	if msg == "" {
		msg = "发送失败"
	}
	slog.Warn("iGot 发送失败", "body", resp.Body)
	return notify.NotifyResult{Success: false, Message: msg}
}

func init() {
	ch := &IGot{}
	if notify.HasConfig("IGOT_PUSH_KEY") {
		ch.SetEnabled(true)
	}
	notify.Registry.Register(ch)
}
