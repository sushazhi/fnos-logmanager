package channels

import (
	"fmt"
	"log/slog"
	"net/url"

	"github.com/sushazhi/fnos-logmanager/internal/notify"
)

type Chat struct {
	enabled bool
}

func (c *Chat) Name() string {
	return "synology-chat"
}

func (c *Chat) Enabled() bool {
	return c.enabled
}

func (c *Chat) SetEnabled(enabled bool) {
	c.enabled = enabled
}

func (c *Chat) Send(text, desp string) notify.NotifyResult {
	if !notify.HasConfig("CHAT_URL", "CHAT_TOKEN") {
		return notify.NotifyResult{Success: false, Message: "CHAT 配置不完整"}
	}

	chatURL := notify.GetConfig("CHAT_URL")
	chatToken := notify.GetConfig("CHAT_TOKEN")
	fullURL := chatURL + chatToken

	// URL-encode the description
	encodedDesp := url.QueryEscape(desp)
	body := fmt.Sprintf(`payload={"text":"%s\n%s"}`, text, encodedDesp)

	resp, err := notify.HTTPPost(fullURL, notify.HttpRequestOptions{
		Body: body,
		Headers: map[string]string{
			"Content-Type": "application/x-www-form-urlencoded",
		},
	})
	if err != nil {
		slog.Error("Synology Chat 发送失败", "error", err)
		return notify.NotifyResult{Success: false, Message: err.Error(), Err: err}
	}

	if resp.StatusCode == 200 {
		slog.Info("Synology Chat 发送成功")
		return notify.NotifyResult{Success: true, Message: "发送成功"}
	}

	slog.Warn("Synology Chat 发送失败", "status", resp.StatusCode, "body", resp.Body)
	return notify.NotifyResult{Success: false, Message: "发送失败"}
}

func init() {
	ch := &Chat{}
	if notify.HasConfig("CHAT_URL", "CHAT_TOKEN") {
		ch.SetEnabled(true)
	}
	notify.Registry.Register(ch)
}
