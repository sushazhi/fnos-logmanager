package channels

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/sushazhi/fnos-logmanager/internal/notify"
	"github.com/gorilla/websocket"
)

const (
	qqAPIBase    = "https://api.sgroup.qq.com"
	qqTokenURL   = "https://bots.qq.com/app/getAppAccessToken"
	qqWSMaxReconnect = 5
)

// QQ token cache
type qqTokenCache struct {
	token     string
	expiresAt int64
	appID     string
}

var (
	qqCachedToken    *qqTokenCache
	qqCachedTokenMu  sync.RWMutex
)

// Captured openIDs (from WebSocket events or configured)
var (
	capturedOpenID      string
	capturedGroupOpenID string
	capturedMu          sync.RWMutex
)

// WebSocket listener state
var (
	qqWSClient           *websocket.Conn
	qqWSHeartbeatStop    chan struct{}
	qqWSReconnectAttempts int
	qqWSMu               sync.Mutex
)

// getQQAccessToken gets a cached or fresh access token for QQ Bot.
func getQQAccessToken(appID, appSecret string) (string, error) {
	now := time.Now().UnixMilli()

	qqCachedTokenMu.RLock()
	cached := qqCachedToken
	qqCachedTokenMu.RUnlock()

	if cached != nil && now < cached.expiresAt-5*60*1000 && cached.appID == appID {
		return cached.token, nil
	}

	if cached != nil && cached.appID != appID {
		qqCachedTokenMu.Lock()
		qqCachedToken = nil
		qqCachedTokenMu.Unlock()
	}

	type tokenResponse struct {
		AccessToken string      `json:"access_token"`
		ExpiresIn   json.Number `json:"expires_in"`
	}

	if len(appID) > 4 {
		slog.Info("QQ token请求凭证", "appId_preview", appID[:4]+"****", "appId_len", len(appID), "secret_len", len(appSecret))
	} else {
		slog.Warn("QQ Token请求 appID 为空或过短", "appId_len", len(appID), "secret_len", len(appSecret))
	}

	resp, err := notify.HTTPPost(qqTokenURL, notify.HttpRequestOptions{
		JSON: map[string]string{
			"appId":       appID,
			"clientSecret": appSecret,
		},
	})
	if err != nil {
		return "", fmt.Errorf("获取QQ token失败: %w", err)
	}

	if resp.StatusCode != 200 {
		slog.Error("QQ token API返回非200状态码", "statusCode", resp.StatusCode, "body", resp.Body)
		return "", fmt.Errorf("获取Token失败: HTTP %d — %s", resp.StatusCode, resp.Body)
	}

	var result tokenResponse
	if err := json.Unmarshal([]byte(resp.Body), &result); err != nil {
		return "", fmt.Errorf("解析QQ token响应失败(status=%d): %w", resp.StatusCode, err)
	}

	if result.AccessToken == "" {
		// Try alternate field names that the QQ Bot API might return
		var fallback struct {
			AccessToken string `json:"accessToken"`
		}
		if err := json.Unmarshal([]byte(resp.Body), &fallback); err == nil && fallback.AccessToken != "" {
			result.AccessToken = fallback.AccessToken
		}
	}

	if result.AccessToken == "" {
		slog.Error("QQ token响应缺少access_token", "statusCode", resp.StatusCode, "body", resp.Body)
		return "", fmt.Errorf("获取Token失败: 响应中缺少access_token (HTTP %d) — body: %s", resp.StatusCode, resp.Body)
	}

	expiresIn := 7200
	if result.ExpiresIn != "" {
		if n, err := result.ExpiresIn.Int64(); err == nil && n > 0 {
			expiresIn = int(n)
		}
	}

	qqCachedTokenMu.Lock()
	qqCachedToken = &qqTokenCache{
		token:     result.AccessToken,
		expiresAt: now + int64(expiresIn)*1000,
		appID:     appID,
	}
	qqCachedTokenMu.Unlock()

	return result.AccessToken, nil
}

// sendQQMessage sends a message to a QQ target (user or group).
func sendQQMessage(accessToken, target, content string, isGroup, useMarkdown bool) error {
	path := "/v2/users/" + target + "/messages"
	if isGroup {
		path = "/v2/groups/" + target + "/messages"
	}

	var body map[string]interface{}
	if useMarkdown {
		body = map[string]interface{}{
			"markdown": map[string]string{"content": content},
			"msg_type": 2,
		}
	} else {
		body = map[string]interface{}{
			"content":  content,
			"msg_type": 0,
		}
	}

	_, err := notify.HTTPPost(qqAPIBase+path, notify.HttpRequestOptions{
		JSON: body,
		Headers: map[string]string{
			"Authorization": "QQBot " + accessToken,
		},
	})
	if err != nil {
		return err
	}

	return nil
}

// HandleQQBotEvent processes a QQ Bot event (called from WebSocket or HTTP callback).
func HandleQQBotEvent(event map[string]interface{}) {
	if event == nil {
		return
	}

	t, _ := event["t"].(string)
	d, _ := event["d"].(map[string]interface{})

	if d == nil {
		return
	}

	switch t {
	case "C2C_MESSAGE_CREATE":
		author, _ := d["author"].(map[string]interface{})
		if author != nil {
			openID, _ := author["user_openid"].(string)
			if openID == "" {
				openID, _ = author["member_openid"].(string)
			}
			if openID != "" {
				capturedMu.Lock()
				capturedOpenID = openID
				capturedMu.Unlock()
				slog.Info("QQ机器人捕获到用户 openID", "openid", openID)
			}
		}

	case "GROUP_AT_MESSAGE_CREATE":
		groupOpenID, _ := d["group_openid"].(string)
		if groupOpenID != "" {
			capturedMu.Lock()
			capturedGroupOpenID = groupOpenID
			capturedMu.Unlock()
			slog.Info("QQ机器人捕获到群 openID", "groupOpenid", groupOpenID)
		}
	}
}

// GetCapturedOpenIds returns the captured user and group open IDs.
func GetCapturedOpenIds() (openID, groupOpenID string) {
	capturedMu.RLock()
	defer capturedMu.RUnlock()
	return capturedOpenID, capturedGroupOpenID
}

// StartQQBotListener starts a WebSocket listener for QQ Bot to capture openIDs.
// Uses gorilla/websocket which is already in go.mod.
func StartQQBotListener() (success bool, message string) {
	qqWSMu.Lock()
	defer qqWSMu.Unlock()

	if qqWSClient != nil {
		return true, "已在监听中"
	}

	if !notify.HasConfig("QQ_APP_ID", "QQ_APP_SECRET") {
		return false, "请先配置 QQ_APP_ID 和 QQ_APP_SECRET"
	}

	appID := notify.GetConfig("QQ_APP_ID")
	appSecret := notify.GetConfig("QQ_APP_SECRET")

	slog.Info("QQ机器人获取AccessToken...")
	accessToken, err := getQQAccessToken(appID, appSecret)
	if err != nil {
		slog.Error("QQ机器人获取AccessToken失败", "error", err)
		return false, "获取AccessToken失败: " + err.Error()
	}
	slog.Info("QQ机器人AccessToken获取成功")

	// Get gateway URL via GET request
	slog.Info("QQ机器人获取WebSocket gateway URL...")
	type gatewayResponse struct {
		URL string `json:"url"`
	}

	gatewayResp, err := notify.HTTPGet(qqAPIBase+"/gateway", notify.HttpRequestOptions{
		Headers: map[string]string{
			"Authorization": "QQBot " + accessToken,
		},
	})
	if err != nil {
		slog.Warn("QQ机器人获取gateway失败", "error", err)
		return false, "获取gateway失败: " + err.Error()
	}

	var gwResult gatewayResponse
	if err := json.Unmarshal([]byte(gatewayResp.Body), &gwResult); err != nil || gwResult.URL == "" {
		slog.Warn("QQ机器人获取gateway响应解析失败")
		return false, "获取gateway失败: 无法解析响应"
	}

	slog.Info("QQ机器人获取gateway成功", "url", gwResult.URL)

	// Connect WebSocket
	connected := make(chan bool, 1)

	go func() {
		dialer := websocket.DefaultDialer
		dialer.HandshakeTimeout = 10 * time.Second

		socket, _, err := dialer.Dial(gwResult.URL, nil)
		if err != nil {
			slog.Warn("QQ机器人WebSocket连接失败", "error", err)
			connected <- false
			return
		}

		qqWSClient = socket
		qqWSReconnectAttempts = 0
		qqWSHeartbeatStop = make(chan struct{})

		slog.Info("QQ机器人 WebSocket 已连接")

		// Send identify
		identifyPayload := map[string]interface{}{
			"op": 2,
			"d": map[string]interface{}{
				"token":   "QQBot " + accessToken,
				"intents": (1 << 30) | (1 << 25),
				"shard":   []int{0, 1},
			},
		}
		identifyData, _ := json.Marshal(identifyPayload)
		if err := socket.WriteMessage(websocket.TextMessage, identifyData); err != nil {
			slog.Warn("QQ机器人发送identify失败", "error", err)
			socket.Close()
			qqWSClient = nil
			connected <- false
			return
		}

		connected <- true

		// Read loop
		for {
			_, raw, err := socket.ReadMessage()
			if err != nil {
				slog.Info("QQ机器人 WebSocket 读取结束", "error", err)
				close(qqWSHeartbeatStop)
				qqWSMu.Lock()
				qqWSClient = nil
				qqWSMu.Unlock()
				return
			}

			var payload map[string]interface{}
			if err := json.Unmarshal(raw, &payload); err != nil {
				slog.Warn("QQ机器人 WebSocket 消息解析失败", "error", err)
				continue
			}

			op, _ := payload["op"].(float64)
			t, _ := payload["t"].(string)
			d, _ := payload["d"].(map[string]interface{})

			slog.Info("QQ机器人 WebSocket 收到消息", "op", op, "t", t)

			// Handle hello (op 10) - start heartbeat
			if int(op) == 10 {
				heartbeatInterval := 30000
				if d != nil {
					if hi, ok := d["heartbeat_interval"].(float64); ok {
						heartbeatInterval = int(hi)
					}
				}

				go func(interval int) {
					ticker := time.NewTicker(time.Duration(interval) * time.Millisecond)
					defer ticker.Stop()

					for {
						select {
						case <-ticker.C:
							qqWSMu.Lock()
							client := qqWSClient
							qqWSMu.Unlock()
							if client != nil {
								heartbeatData, _ := json.Marshal(map[string]interface{}{
									"op": 1,
									"d":  nil,
								})
								if err := client.WriteMessage(websocket.TextMessage, heartbeatData); err != nil {
									slog.Warn("QQ机器人发送心跳失败", "error", err)
									return
								}
							}
						case <-qqWSHeartbeatStop:
							return
						}
					}
				}(heartbeatInterval)
			}

			// Handle dispatch (op 0)
			if int(op) == 0 && t != "" {
				HandleQQBotEvent(payload)
			}
		}
	}()

	select {
	case ok := <-connected:
		if ok {
			return true, "WebSocket已连接，等待消息中"
		}
		return false, "无法连接QQ平台WebSocket，请检查网络和配置"
	case <-time.After(15 * time.Second):
		return false, "WebSocket连接超时"
	}
}

// StopQQBotListener stops the WebSocket listener.
func StopQQBotListener() {
	qqWSMu.Lock()
	defer qqWSMu.Unlock()

	if qqWSHeartbeatStop != nil {
		close(qqWSHeartbeatStop)
		qqWSHeartbeatStop = nil
	}
	if qqWSClient != nil {
		qqWSClient.Close()
		qqWSClient = nil
	}
	slog.Info("QQ机器人监听已停止")
}

// -------- qqbotChannel --------

type QQBot struct {
	enabled bool
}

func (c *QQBot) Name() string {
	return "qqbot"
}

func (c *QQBot) Enabled() bool {
	return c.enabled
}

func (c *QQBot) SetEnabled(enabled bool) {
	c.enabled = enabled
}

func (c *QQBot) Send(text, desp string) notify.NotifyResult {
	if !notify.HasConfig("QQ_APP_ID", "QQ_APP_SECRET") {
		return notify.NotifyResult{Success: false, Message: "QQ机器人配置不完整，需要 QQ_APP_ID 和 QQ_APP_SECRET"}
	}

	QQ_APP_ID := notify.GetConfig("QQ_APP_ID")
	QQ_APP_SECRET := notify.GetConfig("QQ_APP_SECRET")
	QQ_OPENID := notify.GetConfig("QQ_OPENID")
	QQ_GROUP_OPENID := notify.GetConfig("QQ_GROUP_OPENID")

	type target struct {
		id      string
		isGroup bool
	}

	var targets []target
	if QQ_GROUP_OPENID != "" {
		targets = append(targets, target{id: QQ_GROUP_OPENID, isGroup: true})
	}
	if QQ_OPENID != "" {
		targets = append(targets, target{id: QQ_OPENID, isGroup: false})
	}

	if len(targets) == 0 {
		openID, groupOpenID := GetCapturedOpenIds()
		hint := "未配置 openID。请点击\"获取openID\"按钮，然后给机器人发一条消息"
		if openID != "" {
			hint += "\n\n已捕获到用户 openID: " + openID + "，请填入 QQ_OPENID"
		}
		if groupOpenID != "" {
			hint += "\n已捕获到群 openID: " + groupOpenID + "，请填入 QQ_GROUP_OPENID"
		}
		return notify.NotifyResult{Success: false, Message: hint}
	}

	content := "**" + text + "**\n\n" + desp

	accessToken, err := getQQAccessToken(QQ_APP_ID, QQ_APP_SECRET)
	if err != nil {
		slog.Error("QQ机器人获取token失败", "error", err)
		return notify.NotifyResult{Success: false, Message: err.Error(), Err: err}
	}

	successCount := 0
	for _, t := range targets {
		// Try markdown first
		err := sendQQMessage(accessToken, t.id, content, t.isGroup, true)
		if err != nil {
			errMsg := err.Error()
			if strings.Contains(errMsg, "markdown") || strings.Contains(errMsg, "11244") || strings.Contains(errMsg, "权限") {
				// Fallback to text
				textContent := "【" + text + "】\n\n" + desp
				err = sendQQMessage(accessToken, t.id, textContent, t.isGroup, false)
				if err != nil {
					slog.Warn("QQ机器人发送失败", "target", t.id, "error", err)
				} else {
					successCount++
				}
			} else {
				slog.Warn("QQ机器人发送失败", "target", t.id, "error", err)
			}
		} else {
			successCount++
		}
	}

	if successCount > 0 {
		slog.Info("QQ机器人推送成功")
		return notify.NotifyResult{Success: true, Message: "发送成功"}
	}

	return notify.NotifyResult{Success: false, Message: "发送失败"}
}

func init() {
	ch := &QQBot{}
	if notify.HasConfig("QQ_APP_ID", "QQ_APP_SECRET") {
		ch.SetEnabled(true)
	}
	notify.Registry.Register(ch)
}
