package channels

import (
	"fmt"
	"log/slog"

	"github.com/sushazhi/fnos-logmanager/internal/notify"
)

type AIBotK struct {
	enabled bool
}

func (c *AIBotK) Name() string {
	return "aibotk"
}

func (c *AIBotK) Enabled() bool {
	return c.enabled
}

func (c *AIBotK) SetEnabled(enabled bool) {
	c.enabled = enabled
}

func (c *AIBotK) Send(text, desp string) notify.NotifyResult {
	if !notify.HasConfig("AIBOTK_KEY", "AIBOTK_TYPE", "AIBOTK_NAME") {
		return notify.NotifyResult{Success: false, Message: "AIBOTK 配置不完整"}
	}

	aiBotKKey := notify.GetConfig("AIBOTK_KEY")
	aiBotKType := notify.GetConfig("AIBOTK_TYPE")
	aiBotKName := notify.GetConfig("AIBOTK_NAME")

	var url string
	var body map[string]interface{}

	switch aiBotKType {
	case "room":
		url = "https://api-bot.aibotk.com/openapi/v1/chat/room"
		body = map[string]interface{}{
			"apiKey":   aiBotKKey,
			"roomName": aiBotKName,
			"message": map[string]interface{}{
				"type":    1,
				"content": fmt.Sprintf("【青龙快讯】\n\n%s\n%s", text, desp),
			},
		}
	case "contact":
		url = "https://api-bot.aibotk.com/openapi/v1/chat/contact"
		body = map[string]interface{}{
			"apiKey": aiBotKKey,
			"name":   aiBotKName,
			"message": map[string]interface{}{
				"type":    1,
				"content": fmt.Sprintf("【青龙快讯】\n\n%s\n%s", text, desp),
			},
		}
	default:
		return notify.NotifyResult{Success: false, Message: "AIBOTK_TYPE 类型错误"}
	}

	resp, err := notify.HTTPPost(url, notify.HttpRequestOptions{
		JSON: body,
	})
	if err != nil {
		slog.Error("智能微秘书发送失败", "error", err)
		return notify.NotifyResult{Success: false, Message: err.Error(), Err: err}
	}

	if resp.StatusCode == 200 {
		slog.Info("智能微秘书发送成功")
		return notify.NotifyResult{Success: true, Message: "发送成功"}
	}

	slog.Warn("智能微秘书发送失败", "status", resp.StatusCode, "body", resp.Body)
	return notify.NotifyResult{Success: false, Message: "发送失败"}
}

func init() {
	ch := &AIBotK{}
	if notify.HasConfig("AIBOTK_KEY", "AIBOTK_TYPE", "AIBOTK_NAME") {
		ch.SetEnabled(true)
	}
	notify.Registry.Register(ch)
}
