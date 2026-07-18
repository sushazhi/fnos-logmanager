package channels

import (
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/sushazhi/fnos-logmanager/internal/notify"
)

type Telegram struct {
	enabled bool
}

func (c *Telegram) Name() string            { return "telegram" }
func (c *Telegram) Enabled() bool           { return c.enabled }
func (c *Telegram) SetEnabled(enabled bool) { c.enabled = enabled }

func (c *Telegram) Send(text, desp string) notify.NotifyResult {
	if !notify.HasConfig("TG_BOT_TOKEN", "TG_USER_ID") {
		return notify.NotifyResult{Success: false, Message: "TG_BOT_TOKEN 或 TG_USER_ID 未配置"}
	}

	tgBotToken := notify.GetConfig("TG_BOT_TOKEN")
	tgUserID := notify.GetConfig("TG_USER_ID")
	tgAPIHost := notify.GetConfig("TG_API_HOST")
	if tgAPIHost == "" {
		tgAPIHost = "https://api.telegram.org"
	}

	url := tgAPIHost + "/bot" + tgBotToken + "/sendMessage"

	content := text + "\n\n" + desp
	if len(content) > 4096 {
		content = content[:4096]
	}

	body := map[string]string{
		"chat_id":    tgUserID,
		"text":       content,
		"parse_mode": "HTML",
	}

	resp, err := notify.HTTPPost(url, notify.HttpRequestOptions{JSON: body})
	if err != nil {
		slog.Error("Telegram 发送失败", "err", err)
		return notify.NotifyResult{Success: false, Message: fmt.Sprintf("发送失败: %v", err), Err: err}
	}

	var result struct {
		Ok          bool   `json:"ok"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal([]byte(resp.Body), &result); err != nil {
		slog.Error("Telegram 解析响应失败", "err", err)
		return notify.NotifyResult{Success: false, Message: "解析响应失败", Err: err}
	}

	if result.Ok {
		slog.Info("Telegram 发送成功")
		return notify.NotifyResult{Success: true, Message: "发送成功"}
	}

	msg := result.Description
	if msg == "" {
		msg = "发送失败"
	}
	slog.Warn("Telegram 发送失败", "body", resp.Body)
	return notify.NotifyResult{Success: false, Message: msg}
}

func init() {
	ch := &Telegram{}
	if notify.HasConfig("TG_BOT_TOKEN", "TG_USER_ID") {
		ch.SetEnabled(true)
	}
	notify.Registry.Register(ch)
}
