package channels

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"log/slog"
	"net/url"
	"time"

	"github.com/sushazhi/fnos-logmanager/internal/notify"
)

type DingTalk struct {
	enabled bool
}

func (c *DingTalk) Name() string {
	return "dingtalk"
}

func (c *DingTalk) Enabled() bool {
	return c.enabled
}

func (c *DingTalk) SetEnabled(enabled bool) {
	c.enabled = enabled
}

func (c *DingTalk) Send(text, desp string) notify.NotifyResult {
	if !notify.HasConfig("DD_BOT_TOKEN") {
		return notify.NotifyResult{Success: false, Message: "DD_BOT_TOKEN 未配置"}
	}

	ddBotToken := notify.GetConfig("DD_BOT_TOKEN")
	ddBotSecret := notify.GetConfig("DD_BOT_SECRET")

	webhookURL := "https://oapi.dingtalk.com/robot/send?access_token=" + ddBotToken

	// HMAC-SHA256 signature
	if ddBotSecret != "" {
		now := time.Now().UnixMilli()
		stringToSign := fmt.Sprintf("%d\n%s", now, ddBotSecret)
		mac := hmac.New(sha256.New, []byte(ddBotSecret))
		mac.Write([]byte(stringToSign))
		sign := base64.StdEncoding.EncodeToString(mac.Sum(nil))
		webhookURL += fmt.Sprintf("&timestamp=%d&sign=%s", now, url.QueryEscape(sign))
	}

	// Truncate title to 64 chars and content to 15000 chars
	title := text
	if len(title) > 64 {
		title = title[:64]
	}
	content := desp
	if len(content) > 15000 {
		content = content[:15000]
	}

	body := map[string]interface{}{
		"msgtype": "markdown",
		"markdown": map[string]string{
			"title": title,
			"text":  content,
		},
	}

	resp, err := notify.HTTPPost(webhookURL, notify.HttpRequestOptions{
		JSON: body,
	})
	if err != nil {
		slog.Error("钉钉发送失败", "error", err)
		return notify.NotifyResult{Success: false, Message: err.Error(), Err: err}
	}

	if resp.StatusCode == 200 {
		slog.Info("钉钉发送成功")
		return notify.NotifyResult{Success: true, Message: "发送成功"}
	}

	slog.Warn("钉钉发送失败", "status", resp.StatusCode, "body", resp.Body)
	return notify.NotifyResult{Success: false, Message: "发送失败"}
}

func init() {
	ch := &DingTalk{}
	if notify.HasConfig("DD_BOT_TOKEN") {
		ch.SetEnabled(true)
	}
	notify.Registry.Register(ch)
}
