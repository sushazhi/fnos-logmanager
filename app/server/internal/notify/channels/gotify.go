package channels

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"

	"github.com/sushazhi/fnos-logmanager/internal/notify"
)

type Gotify struct {
	enabled bool
}

func (c *Gotify) Name() string            { return "gotify" }
func (c *Gotify) Enabled() bool           { return c.enabled }
func (c *Gotify) SetEnabled(enabled bool) { c.enabled = enabled }

func (c *Gotify) Send(text, desp string) notify.NotifyResult {
	if !notify.HasConfig("GOTIFY_URL", "GOTIFY_TOKEN") {
		return notify.NotifyResult{Success: false, Message: "Gotify 配置不完整"}
	}

	gotifyURL := notify.GetConfig("GOTIFY_URL")
	gotifyToken := notify.GetConfig("GOTIFY_TOKEN")
	gotifyPriority := notify.GetConfig("GOTIFY_PRIORITY")
	if gotifyPriority == "" {
		gotifyPriority = "0"
	}

	apiURL := fmt.Sprintf("%s/message?token=%s", gotifyURL, gotifyToken)

	body := fmt.Sprintf("title=%s&message=%s&priority=%s",
		url.QueryEscape(text), url.QueryEscape(desp), gotifyPriority)

	resp, err := notify.HTTPPost(apiURL, notify.HttpRequestOptions{
		Body:    body,
		Headers: map[string]string{"Content-Type": "application/x-www-form-urlencoded"},
	})
	if err != nil {
		slog.Error("Gotify 发送失败", "err", err)
		return notify.NotifyResult{Success: false, Message: fmt.Sprintf("发送失败: %v", err), Err: err}
	}

	var result struct {
		ID      float64 `json:"id"`
		Message string  `json:"message"`
	}
	if err := json.Unmarshal([]byte(resp.Body), &result); err != nil {
		slog.Error("Gotify 解析响应失败", "err", err)
		return notify.NotifyResult{Success: false, Message: "解析响应失败", Err: err}
	}

	if result.ID != 0 {
		slog.Info("Gotify 发送成功")
		return notify.NotifyResult{Success: true, Message: "发送成功"}
	}

	msg := result.Message
	if msg == "" {
		msg = "发送失败"
	}
	slog.Warn("Gotify 发送失败", "body", resp.Body)
	return notify.NotifyResult{Success: false, Message: msg}
}

func init() {
	ch := &Gotify{}
	if notify.HasConfig("GOTIFY_URL", "GOTIFY_TOKEN") {
		ch.SetEnabled(true)
	}
	notify.Registry.Register(ch)
}
