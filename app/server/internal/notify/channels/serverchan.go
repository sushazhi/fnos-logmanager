package channels

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"regexp"
	"strings"

	"github.com/sushazhi/fnos-logmanager/internal/notify"
)

type ServerChan struct {
	enabled bool
}

func (c *ServerChan) Name() string            { return "serverChan" }
func (c *ServerChan) Enabled() bool           { return c.enabled }
func (c *ServerChan) SetEnabled(enabled bool) { c.enabled = enabled }

func (c *ServerChan) Send(text, desp string) notify.NotifyResult {
	if !notify.HasConfig("PUSH_KEY") {
		return notify.NotifyResult{Success: false, Message: "PUSH_KEY 未配置"}
	}

	pushKey := notify.GetConfig("PUSH_KEY")

	// Replace single newlines with double newlines (ServerChan requires double newlines for proper line breaks)
	formattedDesp := strings.NewReplacer("\r\n", "\n\n", "\r", "\n\n", "\n", "\n\n").Replace(desp)

	// Determine if Turbo version or old version
	re := regexp.MustCompile(`(?i)^sctp(\d+)t`)
	matches := re.FindStringSubmatch(pushKey)
	var pushURL string
	if len(matches) > 1 && matches[1] != "" {
		pushURL = fmt.Sprintf("https://%s.push.ft07.com/send/%s.send", matches[1], pushKey)
	} else {
		pushURL = fmt.Sprintf("https://sctapi.ftqq.com/%s.send", pushKey)
	}

	body := fmt.Sprintf("text=%s&desp=%s", url.QueryEscape(text), url.QueryEscape(formattedDesp))

	resp, err := notify.HTTPPost(pushURL, notify.HttpRequestOptions{
		Body:    body,
		Headers: map[string]string{"Content-Type": "application/x-www-form-urlencoded"},
	})
	if err != nil {
		slog.Error("Server酱 发送失败", "err", err)
		return notify.NotifyResult{Success: false, Message: fmt.Sprintf("发送失败: %v", err), Err: err}
	}

	// Try top-level errno first, then nested data.errno
	var topResult struct {
		Errno  float64 `json:"errno"`
		Errmsg string  `json:"errmsg"`
		Data   *struct {
			Errno float64 `json:"errno"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(resp.Body), &topResult); err != nil {
		slog.Error("Server酱 解析响应失败", "err", err)
		return notify.NotifyResult{Success: false, Message: "解析响应失败", Err: err}
	}

	if topResult.Errno == 0 || (topResult.Data != nil && topResult.Data.Errno == 0) {
		slog.Info("Server酱 发送成功")
		return notify.NotifyResult{Success: true, Message: "发送成功"}
	}

	if topResult.Errno == 1024 {
		slog.Warn("Server酱 发送频率限制", "msg", topResult.Errmsg)
		return notify.NotifyResult{Success: false, Message: topResult.Errmsg}
	}

	slog.Warn("Server酱 发送失败", "body", resp.Body)
	return notify.NotifyResult{Success: false, Message: "发送失败"}
}

func init() {
	ch := &ServerChan{}
	if notify.HasConfig("PUSH_KEY") {
		ch.SetEnabled(true)
	}
	notify.Registry.Register(ch)
}
