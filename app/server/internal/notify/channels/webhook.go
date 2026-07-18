package channels

import (
	"encoding/json"
	"log/slog"
	"strings"

	"github.com/sushazhi/fnos-logmanager/internal/notify"
)

type Webhook struct {
	enabled bool
}

func (c *Webhook) Name() string {
	return "webhook"
}

func (c *Webhook) Enabled() bool {
	return c.enabled
}

func (c *Webhook) SetEnabled(enabled bool) {
	c.enabled = enabled
}

func (c *Webhook) Send(text, desp string) notify.NotifyResult {
	if !notify.HasConfig("WEBHOOK_URL") {
		return notify.NotifyResult{Success: false, Message: "WEBHOOK_URL 未配置"}
	}

	webhookURL := notify.GetConfig("WEBHOOK_URL")
	webhookBody := notify.GetConfig("WEBHOOK_BODY")
	webhookHeaders := notify.GetConfig("WEBHOOK_HEADERS")
	webhookMethod := notify.GetConfig("WEBHOOK_METHOD")
	if webhookMethod == "" {
		webhookMethod = "POST"
	}
	webhookContentType := notify.GetConfig("WEBHOOK_CONTENT_TYPE")
	if webhookContentType == "" {
		webhookContentType = "application/json"
	}

	headers := map[string]string{
		"content-type": webhookContentType,
	}

	// Parse custom headers from JSON
	if webhookHeaders != "" {
		var customHeaders map[string]string
		if err := json.Unmarshal([]byte(webhookHeaders), &customHeaders); err != nil {
			slog.Warn("WEBHOOK_HEADERS 解析失败", "error", err)
		} else {
			for k, v := range customHeaders {
				headers[k] = v
			}
		}
	}

	// Build body with template variables
	var body string
	if webhookBody != "" {
		body = strings.ReplaceAll(webhookBody, "${title}", text)
		body = strings.ReplaceAll(body, "${content}", desp)
	} else {
		b, _ := json.Marshal(map[string]string{
			"title":   text,
			"content": desp,
		})
		body = string(b)
	}

	resp, err := notify.HTTPRequest(webhookURL, notify.HttpRequestOptions{
		Method:  webhookMethod,
		Headers: headers,
		Body:    body,
	})
	if err != nil {
		slog.Error("Webhook 发送失败", "error", err)
		return notify.NotifyResult{Success: false, Message: err.Error(), Err: err}
	}

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		slog.Info("Webhook 发送成功", "statusCode", resp.StatusCode)
		return notify.NotifyResult{Success: true, Message: "发送成功"}
	}

	slog.Warn("Webhook 发送失败", "statusCode", resp.StatusCode, "body", resp.Body)
	return notify.NotifyResult{Success: false, Message: "HTTP " + string(rune(resp.StatusCode))}
}

func init() {
	ch := &Webhook{}
	if notify.HasConfig("WEBHOOK_URL") {
		ch.SetEnabled(true)
	}
	notify.Registry.Register(ch)
}
