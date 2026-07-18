package channels

import (
	"log/slog"

	"github.com/sushazhi/fnos-logmanager/internal/notify"
)

type PushMe struct {
	enabled bool
}

func (c *PushMe) Name() string {
	return "pushme"
}

func (c *PushMe) Enabled() bool {
	return c.enabled
}

func (c *PushMe) SetEnabled(enabled bool) {
	c.enabled = enabled
}

func (c *PushMe) Send(text, desp string) notify.NotifyResult {
	if !notify.HasConfig("PUSHME_KEY") {
		return notify.NotifyResult{Success: false, Message: "PUSHME_KEY 未配置"}
	}

	pushmeKey := notify.GetConfig("PUSHME_KEY")
	url := "https://push.i-i.me?pushkey=" + pushmeKey

	body := map[string]string{
		"title":   text,
		"content": desp,
	}

	resp, err := notify.HTTPPost(url, notify.HttpRequestOptions{
		JSON: body,
	})
	if err != nil {
		slog.Error("PushMe 发送失败", "error", err)
		return notify.NotifyResult{Success: false, Message: err.Error(), Err: err}
	}

	if resp.StatusCode == 200 {
		slog.Info("PushMe 发送成功")
		return notify.NotifyResult{Success: true, Message: "发送成功"}
	}

	slog.Warn("PushMe 发送失败", "status", resp.StatusCode, "body", resp.Body)
	return notify.NotifyResult{Success: false, Message: "发送失败"}
}

func init() {
	ch := &PushMe{}
	if notify.HasConfig("PUSHME_KEY") {
		ch.SetEnabled(true)
	}
	notify.Registry.Register(ch)
}
