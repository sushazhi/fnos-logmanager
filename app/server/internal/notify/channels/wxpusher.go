package channels

import (
	"fmt"
	"html"
	"log/slog"
	"strconv"
	"strings"

	"github.com/sushazhi/fnos-logmanager/internal/notify"
)

type WxPusher struct {
	enabled bool
}

func (c *WxPusher) Name() string {
	return "wxpusher"
}

func (c *WxPusher) Enabled() bool {
	return c.enabled
}

func (c *WxPusher) SetEnabled(enabled bool) {
	c.enabled = enabled
}

func (c *WxPusher) Send(text, desp string) notify.NotifyResult {
	if !notify.HasConfig("WXPUSHER_APP_TOKEN") {
		return notify.NotifyResult{Success: false, Message: "WXPUSHER_APP_TOKEN 未配置"}
	}

	wxpusherAppToken := notify.GetConfig("WXPUSHER_APP_TOKEN")
	wxpusherTopicIDs := notify.GetConfig("WXPUSHER_TOPIC_IDS")
	wxpusherUIDs := notify.GetConfig("WXPUSHER_UIDS")

	// Parse topicIds from semicolon-separated string
	var topicIds []int
	if wxpusherTopicIDs != "" {
		parts := strings.Split(wxpusherTopicIDs, ";")
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p != "" {
				if id, err := strconv.Atoi(p); err == nil {
					topicIds = append(topicIds, id)
				}
			}
		}
	}

	// Parse uids from semicolon-separated string
	var uids []string
	if wxpusherUIDs != "" {
		parts := strings.Split(wxpusherUIDs, ";")
		for _, uid := range parts {
			uid = strings.TrimSpace(uid)
			if uid != "" {
				uids = append(uids, uid)
			}
		}
	}

	// At least one of topicIds or uids must be provided
	if len(topicIds) == 0 && len(uids) == 0 {
		return notify.NotifyResult{Success: false, Message: "WXPUSHER_TOPIC_IDS 和 WXPUSHER_UIDS 至少设置一个"}
	}

	url := "https://wxpusher.zjiecode.com/api/send/message"
	body := map[string]interface{}{
		"appToken":       wxpusherAppToken,
		"content":        fmt.Sprintf("<h1>%s</h1><br/><div style='white-space: pre-wrap;'>%s</div>", html.EscapeString(text), html.EscapeString(desp)),
		"summary":        html.EscapeString(text),
		"contentType":    2, // HTML format
		"topicIds":       topicIds,
		"uids":           uids,
		"verifyPayType":  0,
	}

	resp, err := notify.HTTPPost(url, notify.HttpRequestOptions{
		JSON: body,
	})
	if err != nil {
		slog.Error("WxPusher 发送失败", "error", err)
		return notify.NotifyResult{Success: false, Message: err.Error(), Err: err}
	}

	if resp.StatusCode == 200 {
		slog.Info("WxPusher 发送成功")
		return notify.NotifyResult{Success: true, Message: "发送成功"}
	}

	slog.Warn("WxPusher 发送失败", "status", resp.StatusCode, "body", resp.Body)
	return notify.NotifyResult{Success: false, Message: "发送失败"}
}

func init() {
	ch := &WxPusher{}
	if notify.HasConfig("WXPUSHER_APP_TOKEN") {
		ch.SetEnabled(true)
	}
	notify.Registry.Register(ch)
}
