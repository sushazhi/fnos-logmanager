package channels

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"

	"github.com/sushazhi/fnos-logmanager/internal/notify"
)

type PushDeer struct {
	enabled bool
}

func (c *PushDeer) Name() string            { return "pushdeer" }
func (c *PushDeer) Enabled() bool           { return c.enabled }
func (c *PushDeer) SetEnabled(enabled bool) { c.enabled = enabled }

func (c *PushDeer) Send(text, desp string) notify.NotifyResult {
	if !notify.HasConfig("DEER_KEY") {
		return notify.NotifyResult{Success: false, Message: "DEER_KEY 未配置"}
	}

	deerKey := notify.GetConfig("DEER_KEY")
	deerURL := notify.GetConfig("DEER_URL")
	if deerURL == "" {
		deerURL = "https://api2.pushdeer.com/message/push"
	}

	// PushDeer recommends URL-encoding the content
	encodedDesp := url.QueryEscape(desp)
	body := fmt.Sprintf("pushkey=%s&text=%s&desp=%s&type=markdown", deerKey, text, encodedDesp)

	resp, err := notify.HTTPPost(deerURL, notify.HttpRequestOptions{
		Body:    body,
		Headers: map[string]string{"Content-Type": "application/x-www-form-urlencoded"},
	})
	if err != nil {
		slog.Error("PushDeer 发送失败", "err", err)
		return notify.NotifyResult{Success: false, Message: fmt.Sprintf("发送失败: %v", err), Err: err}
	}

	var result struct {
		Content *struct {
			Result []interface{} `json:"result"`
		} `json:"content"`
	}
	if err := json.Unmarshal([]byte(resp.Body), &result); err != nil {
		slog.Error("PushDeer 解析响应失败", "err", err)
		return notify.NotifyResult{Success: false, Message: "解析响应失败", Err: err}
	}

	if result.Content != nil && len(result.Content.Result) > 0 {
		slog.Info("PushDeer 发送成功")
		return notify.NotifyResult{Success: true, Message: "发送成功"}
	}

	slog.Warn("PushDeer 发送失败", "body", resp.Body)
	return notify.NotifyResult{Success: false, Message: "发送失败"}
}

func init() {
	ch := &PushDeer{}
	if notify.HasConfig("DEER_KEY") {
		ch.SetEnabled(true)
	}
	notify.Registry.Register(ch)
}
