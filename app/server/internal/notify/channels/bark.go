package channels

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/sushazhi/fnos-logmanager/internal/notify"
)

type Bark struct {
	enabled bool
}

func (c *Bark) Name() string            { return "bark" }
func (c *Bark) Enabled() bool           { return c.enabled }
func (c *Bark) SetEnabled(enabled bool) { c.enabled = enabled }

func (c *Bark) Send(text, desp string) notify.NotifyResult {
	if !notify.HasConfig("BARK_PUSH") {
		return notify.NotifyResult{Success: false, Message: "BARK_PUSH 未配置"}
	}

	barkPush := notify.GetConfig("BARK_PUSH")
	if !strings.HasPrefix(barkPush, "http") {
		barkPush = "https://api.day.app/" + barkPush
	}

	body := map[string]string{
		"title":     text,
		"body":      desp,
		"icon":      notify.GetConfig("BARK_ICON"),
		"sound":     notify.GetConfig("BARK_SOUND"),
		"group":     notify.GetConfig("BARK_GROUP"),
		"isArchive": notify.GetConfig("BARK_ARCHIVE"),
		"level":     notify.GetConfig("BARK_LEVEL"),
		"url":       notify.GetConfig("BARK_URL"),
	}

	resp, err := notify.HTTPPost(barkPush, notify.HttpRequestOptions{JSON: body})
	if err != nil {
		slog.Error("Bark 发送失败", "err", err)
		return notify.NotifyResult{Success: false, Message: fmt.Sprintf("发送失败: %v", err), Err: err}
	}

	var result struct {
		Code    float64 `json:"code"`
		Message string  `json:"message"`
	}
	if err := json.Unmarshal([]byte(resp.Body), &result); err != nil {
		slog.Error("Bark 解析响应失败", "err", err)
		return notify.NotifyResult{Success: false, Message: "解析响应失败", Err: err}
	}

	if result.Code == 200 {
		slog.Info("Bark 发送成功")
		return notify.NotifyResult{Success: true, Message: "发送成功"}
	}

	msg := result.Message
	if msg == "" {
		msg = "发送失败"
	}
	slog.Warn("Bark 发送失败", "body", resp.Body)
	return notify.NotifyResult{Success: false, Message: msg}
}

func init() {
	ch := &Bark{}
	if notify.HasConfig("BARK_PUSH") {
		ch.SetEnabled(true)
	}
	notify.Registry.Register(ch)
}
