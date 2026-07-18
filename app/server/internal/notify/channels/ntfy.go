package channels

import (
	"encoding/base64"
	"log/slog"

	"github.com/sushazhi/fnos-logmanager/internal/notify"
)

func encodeRFC2047(text string) string {
	encoded := base64.StdEncoding.EncodeToString([]byte(text))
	return "=?utf-8?B?" + encoded + "?="
}

type Ntfy struct {
	enabled bool
}

func (c *Ntfy) Name() string {
	return "ntfy"
}

func (c *Ntfy) Enabled() bool {
	return c.enabled
}

func (c *Ntfy) SetEnabled(enabled bool) {
	c.enabled = enabled
}

func (c *Ntfy) Send(text, desp string) notify.NotifyResult {
	if !notify.HasConfig("NTFY_TOPIC") {
		return notify.NotifyResult{Success: false, Message: "NTFY_TOPIC 未配置"}
	}

	ntfyURL := notify.GetConfig("NTFY_URL")
	if ntfyURL == "" {
		ntfyURL = "https://ntfy.sh"
	}
	ntfyTopic := notify.GetConfig("NTFY_TOPIC")
	ntfyPriority := notify.GetConfig("NTFY_PRIORITY")
	if ntfyPriority == "" {
		ntfyPriority = "3"
	}
	ntfyToken := notify.GetConfig("NTFY_TOKEN")
	ntfyUsername := notify.GetConfig("NTFY_USERNAME")
	ntfyPassword := notify.GetConfig("NTFY_PASSWORD")
	ntfyActions := notify.GetConfig("NTFY_ACTIONS")

	url := ntfyURL + "/" + ntfyTopic

	headers := map[string]string{
		"Title":    encodeRFC2047(text),
		"Priority": ntfyPriority,
		"Icon":     "https://qn.whyour.cn/logo.png",
	}

	// Authentication
	if ntfyToken != "" {
		headers["Authorization"] = "Bearer " + ntfyToken
	} else if ntfyUsername != "" && ntfyPassword != "" {
		auth := base64.StdEncoding.EncodeToString([]byte(ntfyUsername + ":" + ntfyPassword))
		headers["Authorization"] = "Basic " + auth
	}

	// Actions
	if ntfyActions != "" {
		headers["Actions"] = encodeRFC2047(ntfyActions)
	}

	resp, err := notify.HTTPPost(url, notify.HttpRequestOptions{
		Body:    desp,
		Headers: headers,
	})
	if err != nil {
		slog.Error("Ntfy 发送失败", "error", err)
		return notify.NotifyResult{Success: false, Message: err.Error(), Err: err}
	}

	if resp.StatusCode == 200 {
		slog.Info("Ntfy 发送成功")
		return notify.NotifyResult{Success: true, Message: "发送成功"}
	}

	slog.Warn("Ntfy 发送失败", "status", resp.StatusCode, "body", resp.Body)
	return notify.NotifyResult{Success: false, Message: "发送失败"}
}

func init() {
	ch := &Ntfy{}
	if notify.HasConfig("NTFY_TOPIC") {
		ch.SetEnabled(true)
	}
	notify.Registry.Register(ch)
}
