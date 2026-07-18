package channels

import (
	"encoding/json"
	"log/slog"
	"strconv"
	"strings"

	"github.com/sushazhi/fnos-logmanager/internal/notify"
)

// -------- wechatWorkBotChannel --------

type WechatWorkBot struct {
	enabled bool
}

func (c *WechatWorkBot) Name() string {
	return "wechat-work-bot"
}

func (c *WechatWorkBot) Enabled() bool {
	return c.enabled
}

func (c *WechatWorkBot) SetEnabled(enabled bool) {
	c.enabled = enabled
}

func (c *WechatWorkBot) Send(text, desp string) notify.NotifyResult {
	if !notify.HasConfig("QYWX_KEY") {
		return notify.NotifyResult{Success: false, Message: "QYWX_KEY 未配置"}
	}

	QYWX_KEY := notify.GetConfig("QYWX_KEY")
	QYWX_ORIGIN := notify.GetConfig("QYWX_ORIGIN")
	if QYWX_ORIGIN == "" {
		QYWX_ORIGIN = "https://qyapi.weixin.qq.com"
	}

	url := QYWX_ORIGIN + "/cgi-bin/webhook/send?key=" + QYWX_KEY

	body := map[string]interface{}{
		"msgtype": "text",
		"text": map[string]string{
			"content": text + "\n\n" + desp,
		},
	}

	resp, err := notify.HTTPPost(url, notify.HttpRequestOptions{
		JSON: body,
	})
	if err != nil {
		slog.Error("企业微信机器人发送失败", "error", err)
		return notify.NotifyResult{Success: false, Message: err.Error(), Err: err}
	}

	var result struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	if err := json.Unmarshal([]byte(resp.Body), &result); err != nil {
		slog.Warn("企业微信机器人解析响应失败", "error", err)
		return notify.NotifyResult{Success: false, Message: "解析响应失败"}
	}

	if result.ErrCode == 0 {
		slog.Info("企业微信机器人发送成功")
		return notify.NotifyResult{Success: true, Message: "发送成功"}
	}

	slog.Warn("企业微信机器人发送失败", "msg", result.ErrMsg)
	return notify.NotifyResult{Success: false, Message: result.ErrMsg}
}

// -------- wechatWorkAppChannel --------

type WechatWorkApp struct {
	enabled bool
}

func (c *WechatWorkApp) Name() string {
	return "wechat-work-app"
}

func (c *WechatWorkApp) Enabled() bool {
	return c.enabled
}

func (c *WechatWorkApp) SetEnabled(enabled bool) {
	c.enabled = enabled
}

func (c *WechatWorkApp) Send(text, desp string) notify.NotifyResult {
	if !notify.HasConfig("QYWX_AM") {
		return notify.NotifyResult{Success: false, Message: "QYWX_AM 未配置"}
	}

	QYWX_AM := notify.GetConfig("QYWX_AM")
	QYWX_ORIGIN := notify.GetConfig("QYWX_ORIGIN")
	if QYWX_ORIGIN == "" {
		QYWX_ORIGIN = "https://qyapi.weixin.qq.com"
	}

	parts := strings.Split(QYWX_AM, ",")
	if len(parts) < 5 {
		return notify.NotifyResult{Success: false, Message: "QYWX_AM 格式错误"}
	}
	corpid := parts[0]
	corpsecret := parts[1]
	touser := parts[2]
	agentid := parts[3]
	msgtype := parts[4]

	if corpid == "" || corpsecret == "" || agentid == "" {
		return notify.NotifyResult{Success: false, Message: "QYWX_AM 格式错误"}
	}

	// Step 1: Get access_token
	tokenURL := QYWX_ORIGIN + "/cgi-bin/gettoken"
	tokenResp, err := notify.HTTPPost(tokenURL, notify.HttpRequestOptions{
		JSON: map[string]string{
			"corpid":     corpid,
			"corpsecret": corpsecret,
		},
	})
	if err != nil {
		slog.Error("企业微信获取token失败", "error", err)
		return notify.NotifyResult{Success: false, Message: err.Error(), Err: err}
	}

	var tokenResult struct {
		AccessToken string `json:"access_token"`
		ErrCode     int    `json:"errcode"`
	}
	if err := json.Unmarshal([]byte(tokenResp.Body), &tokenResult); err != nil {
		slog.Warn("企业微信获取token解析失败", "error", err)
		return notify.NotifyResult{Success: false, Message: "获取token失败"}
	}

	if tokenResult.AccessToken == "" {
		slog.Warn("企业微信获取token失败")
		return notify.NotifyResult{Success: false, Message: "获取token失败"}
	}

	// Step 2: Send message
	sendURL := QYWX_ORIGIN + "/cgi-bin/message/send?access_token=" + tokenResult.AccessToken

	userID := touser
	if userID == "" {
		userID = "@all"
	}

	agentID, _ := strconv.Atoi(agentid)

	var messageBody map[string]interface{}
	switch msgtype {
	case "0":
		messageBody = map[string]interface{}{
			"msgtype": "textcard",
			"textcard": map[string]string{
				"title":       text,
				"description": desp,
				"url":         "https://github.com/whyour/qinglong",
				"btntxt":      "更多",
			},
		}
	default:
		messageBody = map[string]interface{}{
			"msgtype": "text",
			"text": map[string]string{
				"content": text + "\n\n" + desp,
			},
		}
	}

	sendBody := map[string]interface{}{
		"touser":  userID,
		"agentid": agentID,
		"safe":    "0",
	}
	for k, v := range messageBody {
		sendBody[k] = v
	}

	sendResp, err := notify.HTTPPost(sendURL, notify.HttpRequestOptions{
		JSON: sendBody,
	})
	if err != nil {
		slog.Error("企业微信应用发送失败", "error", err)
		return notify.NotifyResult{Success: false, Message: err.Error(), Err: err}
	}

	var sendResult struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	if err := json.Unmarshal([]byte(sendResp.Body), &sendResult); err != nil {
		slog.Warn("企业微信应用解析响应失败", "error", err)
		return notify.NotifyResult{Success: false, Message: "解析响应失败"}
	}

	if sendResult.ErrCode == 0 {
		slog.Info("企业微信应用发送成功")
		return notify.NotifyResult{Success: true, Message: "发送成功"}
	}

	slog.Warn("企业微信应用发送失败", "msg", sendResult.ErrMsg)
	return notify.NotifyResult{Success: false, Message: sendResult.ErrMsg}
}

func init() {
	bot := &WechatWorkBot{}
	if notify.HasConfig("QYWX_KEY") {
		bot.SetEnabled(true)
	}
	notify.Registry.Register(bot)

	app := &WechatWorkApp{}
	if notify.HasConfig("QYWX_AM") {
		app.SetEnabled(true)
	}
	notify.Registry.Register(app)
}
