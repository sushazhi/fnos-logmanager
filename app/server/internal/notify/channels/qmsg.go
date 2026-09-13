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
	if !notify.HasConfig("QMSG_KEY") {
		return notify.NotifyResult{Success: false, Message: "QMSG_KEY 未配置"}
	}

	qmsgKey := notify.GetConfig("QMSG_KEY")
	qmsgQQ := notify.GetConfig("QMSG_QQ")

	// QMsg v3 接口：POST https://qmsg.zendee.cn/v3/send/{key}
	// 旧版 /{type}/{key} 路径已废弃；group 推送自 2022-10-01 起不再支持，
	// 因此这里统一使用 v3 send 私聊推送。
	urlStr := fmt.Sprintf("https://qmsg.zendee.cn/v3/send/%s", url.PathEscape(qmsgKey))

	// API 单条消息上限 1000 字符，超出会被拒绝，这里按字符截断。
	content := text + "\n\n" + strings.ReplaceAll(desp, "----", "-")
	content = notify.TruncateRunes(content, 1000)

	form := map[string]string{"msg": content}
	// qq 为可选参数；未配置时由服务端使用该 Key 绑定的默认 QQ。
	if qmsgQQ != "" {
		form["qq"] = qmsgQQ
	}

	resp, err := notify.HTTPPost(urlStr, notify.HttpRequestOptions{
		Form: form,
	})
	if err != nil {
		slog.Error("Qmsg 发送失败", "err", err)
		return notify.NotifyResult{Success: false, Message: fmt.Sprintf("发送失败: %v", err), Err: err}
	}

	// v3 统一返回 JSON，以 success 字段判断业务结果（code 仅 0/500）。
	var result struct {
		Success bool   `json:"success"`
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal([]byte(resp.Body), &result); err != nil {
		slog.Error("Qmsg 解析响应失败", "err", err, "status", resp.StatusCode)
		return notify.NotifyResult{Success: false, Message: "解析响应失败", Err: err}
	}

	if result.Success {
		slog.Info("Qmsg 发送成功")
		return notify.NotifyResult{Success: true, Message: "发送成功"}
	}

	msg := result.Message
	if msg == "" {
		msg = "发送失败"
	}
	slog.Warn("Qmsg 发送失败", "code", result.Code, "message", msg)
	return notify.NotifyResult{Success: false, Message: msg}
}

// truncateRunes 已提升为 notify 包的共享实现（notify.TruncateRunes），
// 这里保留薄封装以免改动调用点语义。

func init() {
	ch := &QMsg{}
	if notify.HasConfig("QMSG_KEY") {
		ch.SetEnabled(true)
	}
	notify.Registry.Register(ch)
}
