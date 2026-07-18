package channels

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/sushazhi/fnos-logmanager/internal/notify"
)

type PushPlus struct {
	enabled bool
}

func (c *PushPlus) Name() string            { return "pushplus" }
func (c *PushPlus) Enabled() bool           { return c.enabled }
func (c *PushPlus) SetEnabled(enabled bool) { c.enabled = enabled }

func (c *PushPlus) Send(text, desp string) notify.NotifyResult {
	if !notify.HasConfig("PUSH_PLUS_TOKEN") {
		return notify.NotifyResult{Success: false, Message: "PUSH_PLUS_TOKEN 未配置"}
	}

	token := notify.GetConfig("PUSH_PLUS_TOKEN")
	user := notify.GetConfig("PUSH_PLUS_USER")
	template := notify.GetConfig("PUSH_PLUS_TEMPLATE")
	if template == "" {
		template = "html"
	}
	channel := notify.GetConfig("PUSH_PLUS_CHANNEL")
	if channel == "" {
		channel = "wechat"
	}
	webhook := notify.GetConfig("PUSH_PLUS_WEBHOOK")
	callbackURL := notify.GetConfig("PUSH_PLUS_CALLBACKURL")
	to := notify.GetConfig("PUSH_PLUS_TO")

	// Replace newlines with <br> for HTML template
	formattedDesp := strings.NewReplacer("\r\n", "<br>", "\r", "<br>", "\n", "<br>").Replace(desp)

	url := "https://www.pushplus.plus/send"
	body := map[string]string{
		"token":       token,
		"title":       text,
		"content":     formattedDesp,
		"topic":       user,
		"template":    template,
		"channel":     channel,
		"webhook":     webhook,
		"callbackUrl": callbackURL,
		"to":          to,
	}

	resp, err := notify.HTTPPost(url, notify.HttpRequestOptions{JSON: body})
	if err != nil {
		slog.Error("PushPlus 发送失败", "err", err)
		return notify.NotifyResult{Success: false, Message: fmt.Sprintf("发送失败: %v", err), Err: err}
	}

	var result struct {
		Code float64 `json:"code"`
		Msg  string  `json:"msg"`
		Data string  `json:"data"`
	}
	if err := json.Unmarshal([]byte(resp.Body), &result); err != nil {
		slog.Error("PushPlus 解析响应失败", "err", err)
		return notify.NotifyResult{Success: false, Message: "解析响应失败", Err: err}
	}

	if result.Code == 200 {
		slog.Info("PushPlus 发送成功", "flowId", result.Data)
		return notify.NotifyResult{Success: true, Message: "发送成功"}
	}

	msg := result.Msg
	if msg == "" {
		msg = "发送失败"
	}
	slog.Warn("PushPlus 发送失败", "body", resp.Body)
	return notify.NotifyResult{Success: false, Message: msg}
}

func init() {
	ch := &PushPlus{}
	if notify.HasConfig("PUSH_PLUS_TOKEN") {
		ch.SetEnabled(true)
	}
	notify.Registry.Register(ch)
}
