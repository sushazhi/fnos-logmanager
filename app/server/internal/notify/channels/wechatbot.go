package channels

import (
	"encoding/json"
	"log/slog"
	"strings"

	"github.com/sushazhi/fnos-logmanager/internal/notify"
)

// WechatBot implements the "wechat-bot" channel for 企业微信智能机器人.
// The full implementation requires WebSocket for bidirectional communication.
// This version uses an HTTP-based approach for sending messages.
type WechatBot struct {
	enabled bool
}

func (c *WechatBot) Name() string {
	return "wechat-bot"
}

func (c *WechatBot) Enabled() bool {
	return c.enabled
}

func (c *WechatBot) SetEnabled(enabled bool) {
	c.enabled = enabled
}

func (c *WechatBot) Send(text, desp string) notify.NotifyResult {
	if !notify.HasConfig("WECHAT_BOT_ID", "WECHAT_BOT_SECRET") {
		return notify.NotifyResult{Success: false, Message: "WECHAT_BOT_ID 或 WECHAT_BOT_SECRET 未配置"}
	}

	botID := notify.GetConfig("WECHAT_BOT_ID")
	botSecret := notify.GetConfig("WECHAT_BOT_SECRET")
	chatID := notify.GetConfig("WECHAT_BOT_CHAT_ID")
	wsURL := notify.GetConfig("WECHAT_BOT_WS_URL")
	if wsURL == "" {
		wsURL = "wss://openws.work.weixin.qq.com"
	}

	// Convert wss:// to https:// for HTTP API calls
	apiURL := strings.Replace(wsURL, "wss://", "https://", 1)

	content := text + "\n\n" + desp

	body := map[string]interface{}{
		"bot_id":    botID,
		"bot_secret": botSecret,
		"chat_id":   chatID,
		"msg_type":  "markdown",
		"markdown": map[string]string{
			"content": "**" + text + "**\n\n" + desp,
		},
	}

	resp, err := notify.HTTPPost(apiURL+"/cgi-bin/bot/send", notify.HttpRequestOptions{
		JSON: body,
	})
	if err != nil {
		slog.Error("企业微信智能机器人发送失败", "error", err)
		return notify.NotifyResult{Success: false, Message: err.Error(), Err: err}
	}

	var result struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	if err := json.Unmarshal([]byte(resp.Body), &result); err != nil {
		// Non-JSON response or unknown API - try sending as raw content
		slog.Warn("企业微信智能机器人响应解析失败，使用 raw 格式发送", "error", err)
		rawResp, rawErr := notify.HTTPPost(apiURL+"/cgi-bin/bot/send", notify.HttpRequestOptions{
			Body: content,
			Headers: map[string]string{
				"Content-Type": "text/plain",
			},
		})
		if rawErr != nil {
			slog.Error("企业微信智能机器人 raw 发送失败", "error", rawErr)
			return notify.NotifyResult{Success: false, Message: rawErr.Error(), Err: rawErr}
		}
		if rawResp.StatusCode == 200 {
			slog.Info("企业微信智能机器人 raw 发送成功")
			return notify.NotifyResult{Success: true, Message: "发送成功"}
		}
		return notify.NotifyResult{Success: false, Message: "发送失败"}
	}

	if result.ErrCode == 0 {
		slog.Info("企业微信智能机器人发送成功")
		return notify.NotifyResult{Success: true, Message: "发送成功"}
	}

	slog.Warn("企业微信智能机器人发送失败", "errmsg", result.ErrMsg)
	return notify.NotifyResult{Success: false, Message: result.ErrMsg}
}

func init() {
	ch := &WechatBot{}
	if notify.HasConfig("WECHAT_BOT_ID", "WECHAT_BOT_SECRET") {
		ch.SetEnabled(true)
	}
	notify.Registry.Register(ch)
}
