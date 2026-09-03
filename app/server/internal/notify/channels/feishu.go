package channels

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/sushazhi/fnos-logmanager/internal/notify"
)

func feishuSign(secret string, timestamp int64) string {
	stringToSign := fmt.Sprintf("%d\n%s", timestamp, secret)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(stringToSign))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

// -------- feishuBotChannel --------

type FeishuBot struct {
	enabled bool
}

func (c *FeishuBot) Name() string {
	return "feishu-bot"
}

func (c *FeishuBot) Enabled() bool {
	return c.enabled
}

func (c *FeishuBot) SetEnabled(enabled bool) {
	c.enabled = enabled
}

func (c *FeishuBot) Send(text, desp string) notify.NotifyResult {
	if !notify.HasConfig("FSKEY") {
		return notify.NotifyResult{Success: false, Message: "FSKEY 未配置"}
	}

	FSKEY := notify.GetConfig("FSKEY")
	FSSECRET := notify.GetConfig("FSSECRET")

	messageContent := text + "\n\n" + desp

	var body map[string]interface{}

	if FSSECRET != "" {
		timestamp := time.Now().Unix()
		sign := feishuSign(FSSECRET, timestamp)

		body = map[string]interface{}{
			"msg_type":  "text",
			"content":   map[string]string{"text": messageContent},
			"timestamp": fmt.Sprintf("%d", timestamp),
			"sign":      sign,
		}
	} else {
		// Without secret - use rich text (post) message
		body = map[string]interface{}{
			"msg_type": "post",
			"content": map[string]interface{}{
				"post": map[string]interface{}{
					"zh_cn": map[string]interface{}{
						"title": text,
						"content": [][]map[string]string{
							{{"tag": "text", "text": desp}},
						},
					},
				},
			},
		}
	}

	url := "https://open.feishu.cn/open-apis/bot/v2/hook/" + FSKEY

	resp, err := notify.HTTPPost(url, notify.HttpRequestOptions{
		JSON: body,
	})
	if err != nil {
		slog.Error("飞书发送通知调用API失败", "error", err)
		return notify.NotifyResult{Success: false, Message: err.Error(), Err: err}
	}

	var result struct {
		StatusCode *int   `json:"StatusCode"`
		Code       *int   `json:"code"`
		Msg        string `json:"msg"`
	}
	if err := json.Unmarshal([]byte(resp.Body), &result); err != nil {
		slog.Warn("飞书发送通知解析响应失败", "error", err)
		return notify.NotifyResult{Success: false, Message: "解析响应失败"}
	}

	if (result.StatusCode != nil && *result.StatusCode == 0) ||
		(result.Code != nil && *result.Code == 0) {
		slog.Info("飞书发送通知消息成功")
		return notify.NotifyResult{Success: true, Message: "发送成功"}
	}

	slog.Warn("飞书发送通知消息异常", "msg", result.Msg)
	return notify.NotifyResult{Success: false, Message: result.Msg}
}

// -------- feishuAppChannel --------

var feishuTokenCache struct {
	token     string
	expiresAt int64
}

func getFeishuAppToken() (string, error) {
	appID := notify.GetConfig("FEISHU_APP_ID")
	appSecret := notify.GetConfig("FEISHU_APP_SECRET")

	now := time.Now().UnixMilli()
	if feishuTokenCache.token != "" && now < feishuTokenCache.expiresAt {
		return feishuTokenCache.token, nil
	}

	type tokenResponse struct {
		Code              int    `json:"code"`
		TenantAccessToken string `json:"tenant_access_token"`
		Expire            int    `json:"expire"`
		Msg               string `json:"msg"`
	}

	url := "https://open.feishu.cn/open-apis/auth/v3/tenant_access_token/internal"
	resp, err := notify.HTTPPost(url, notify.HttpRequestOptions{
		JSON: map[string]string{
			"app_id":     appID,
			"app_secret": appSecret,
		},
	})
	if err != nil {
		slog.Error("飞书获取token失败", "error", err)
		return "", err
	}

	var result tokenResponse
	if err := json.Unmarshal([]byte(resp.Body), &result); err != nil {
		slog.Error("飞书获取token解析失败", "error", err)
		return "", err
	}

	if result.Code != 0 {
		slog.Error("飞书获取token异常", "msg", result.Msg)
		return "", fmt.Errorf("飞书获取token异常: %s", result.Msg)
	}

	// Cache token, expire 5 minutes early
	feishuTokenCache.token = result.TenantAccessToken
	feishuTokenCache.expiresAt = now + int64(result.Expire-300)*1000

	return result.TenantAccessToken, nil
}

type FeishuApp struct {
	enabled bool
}

func (c *FeishuApp) Name() string {
	return "feishu-app"
}

func (c *FeishuApp) Enabled() bool {
	return c.enabled
}

func (c *FeishuApp) SetEnabled(enabled bool) {
	c.enabled = enabled
}

func (c *FeishuApp) Send(text, desp string) notify.NotifyResult {
	if !notify.HasConfig("FEISHU_APP_ID", "FEISHU_APP_SECRET") {
		return notify.NotifyResult{Success: false, Message: "FEISHU_APP_ID 或 FEISHU_APP_SECRET 未配置"}
	}

	FEISHU_USER_ID := notify.GetConfig("FEISHU_USER_ID")

	token, err := getFeishuAppToken()
	if err != nil {
		return notify.NotifyResult{Success: false, Message: "获取token失败: " + err.Error(), Err: err}
	}

	messageContent := text + "\n\n" + desp

	contentJSON, _ := json.Marshal(map[string]string{"text": messageContent})

	body := map[string]interface{}{
		"receive_id": FEISHU_USER_ID,
		"msg_type":   "text",
		"content":    string(contentJSON),
	}

	url := "https://open.feishu.cn/open-apis/im/v1/messages?receive_id_type=union_id"
	resp, err := notify.HTTPPost(url, notify.HttpRequestOptions{
		JSON: body,
		Headers: map[string]string{
			"Authorization": "Bearer " + token,
		},
	})
	if err != nil {
		slog.Error("飞书企业应用发送失败", "error", err)
		return notify.NotifyResult{Success: false, Message: err.Error(), Err: err}
	}

	var result struct {
		Code *int   `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := json.Unmarshal([]byte(resp.Body), &result); err != nil {
		slog.Warn("飞书企业应用解析响应失败", "error", err)
		return notify.NotifyResult{Success: false, Message: "解析响应失败"}
	}

	if result.Code != nil && *result.Code == 0 {
		slog.Info("飞书企业应用发送成功")
		return notify.NotifyResult{Success: true, Message: "发送成功"}
	}

	slog.Warn("飞书企业应用发送失败", "msg", result.Msg)
	return notify.NotifyResult{Success: false, Message: result.Msg}
}

func init() {
	bot := &FeishuBot{}
	if notify.HasConfig("FSKEY") {
		bot.SetEnabled(true)
	}
	notify.Registry.Register(bot)

	app := &FeishuApp{}
	if notify.HasConfig("FEISHU_APP_ID", "FEISHU_APP_SECRET") {
		app.SetEnabled(true)
	}
	notify.Registry.Register(app)
}
