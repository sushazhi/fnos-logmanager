package channels

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"strings"

	"github.com/sushazhi/fnos-logmanager/internal/notify"
)

type QMsg struct {
	enabled bool
}

func (c *QMsg) Name() string            { return "qmsg" }
func (c *QMsg) Enabled() bool           { return c.enabled }
func (c *QMsg) SetEnabled(enabled bool) { c.enabled = enabled }

func (c *QMsg) Send(text, desp string) notify.NotifyResult {
	if !notify.HasConfig("QMSG_KEY", "QMSG_TYPE") {
		return notify.NotifyResult{Success: false, Message: "QMSG 配置不完整"}
	}

	qmsgKey := notify.GetConfig("QMSG_KEY")
	qmsgType := notify.GetConfig("QMSG_TYPE")
	urlStr := fmt.Sprintf("https://qmsg.zendee.cn/%s/%s", qmsgType, qmsgKey)

	// Replace special characters
	content := text + "\n\n" + strings.ReplaceAll(desp, "----", "-")
	body := fmt.Sprintf("msg=%s", url.QueryEscape(content))

	resp, err := notify.HTTPPost(urlStr, notify.HttpRequestOptions{
		Body:    body,
		Headers: map[string]string{"Content-Type": "application/x-www-form-urlencoded"},
	})
	if err != nil {
		slog.Error("Qmsg 发送失败", "err", err)
		return notify.NotifyResult{Success: false, Message: fmt.Sprintf("发送失败: %v", err), Err: err}
	}

	var result struct {
		Code float64 `json:"code"`
	}
	if err := json.Unmarshal([]byte(resp.Body), &result); err != nil {
		slog.Error("Qmsg 解析响应失败", "err", err)
		return notify.NotifyResult{Success: false, Message: "解析响应失败", Err: err}
	}

	if result.Code == 0 {
		slog.Info("Qmsg 发送成功")
		return notify.NotifyResult{Success: true, Message: "发送成功"}
	}

	slog.Warn("Qmsg 发送失败", "body", resp.Body)
	return notify.NotifyResult{Success: false, Message: "发送失败"}
}

func init() {
	ch := &QMsg{}
	if notify.HasConfig("QMSG_KEY", "QMSG_TYPE") {
		ch.SetEnabled(true)
	}
	notify.Registry.Register(ch)
}
