package channels

import (
	"log/slog"

	"github.com/sushazhi/fnos-logmanager/internal/notify"
)

type WePlusBot struct {
	enabled bool
}

func (c *WePlusBot) Name() string {
	return "weplusbot"
}

func (c *WePlusBot) Enabled() bool {
	return c.enabled
}

func (c *WePlusBot) SetEnabled(enabled bool) {
	c.enabled = enabled
}

func (c *WePlusBot) Send(text, desp string) notify.NotifyResult {
	if !notify.HasConfig("WE_PLUS_BOT_TOKEN") {
		return notify.NotifyResult{Success: false, Message: "WE_PLUS_BOT_TOKEN 未配置"}
	}

	wePlusBotToken := notify.GetConfig("WE_PLUS_BOT_TOKEN")
	wePlusBotReceiver := notify.GetConfig("WE_PLUS_BOT_RECEIVER")
	wePlusBotVersion := notify.GetConfig("WE_PLUS_BOT_VERSION")
	if wePlusBotVersion == "" {
		wePlusBotVersion = "pro"
	}

	var url string
	if wePlusBotVersion == "personal" {
		url = "https://personal.weplusbot.com/send"
	} else {
		url = "https://pro.weplusbot.com/send"
	}

	body := map[string]string{
		"token":    wePlusBotToken,
		"receiver": wePlusBotReceiver,
		"title":    text,
		"content":  desp,
	}

	resp, err := notify.HTTPPost(url, notify.HttpRequestOptions{
		JSON: body,
	})
	if err != nil {
		slog.Error("微加机器人发送失败", "error", err)
		return notify.NotifyResult{Success: false, Message: err.Error(), Err: err}
	}

	if resp.StatusCode == 200 {
		slog.Info("微加机器人发送成功")
		return notify.NotifyResult{Success: true, Message: "发送成功"}
	}

	slog.Warn("微加机器人发送失败", "status", resp.StatusCode, "body", resp.Body)
	return notify.NotifyResult{Success: false, Message: "发送失败"}
}

func init() {
	ch := &WePlusBot{}
	if notify.HasConfig("WE_PLUS_BOT_TOKEN") {
		ch.SetEnabled(true)
	}
	notify.Registry.Register(ch)
}
