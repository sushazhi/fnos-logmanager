package routes

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	qrgen "github.com/skip2/go-qrcode"
	"github.com/sushazhi/fnos-logmanager/internal/middleware"
	"github.com/sushazhi/fnos-logmanager/internal/notify"
	"github.com/sushazhi/fnos-logmanager/internal/notify/channels"
	"github.com/sushazhi/fnos-logmanager/internal/services"
)

// Supported notification channel types (23 in total).
var supportedChannelTypes = []map[string]interface{}{
	{"type": "bark", "name": "Bark", "fields": []string{"BARK_PUSH", "BARK_ICON", "BARK_SOUND", "BARK_GROUP", "BARK_LEVEL", "BARK_ARCHIVE", "BARK_URL"}},
	{"type": "dingtalk", "name": "钉钉机器人", "fields": []string{"DINGTALK_TOKEN", "DINGTALK_SECRET"}},
	{"type": "feishu", "name": "飞书机器人", "fields": []string{"FEISHU_WEBHOOK", "FEISHU_SECRET"}},
	{"type": "feishu_app", "name": "飞书企业应用", "fields": []string{"FEISHU_WEBHOOK", "FEISHU_SECRET"}},
	{"type": "wecom", "name": "企业微信机器人", "fields": []string{"WECOM_KEY", "WECOM_PROXY"}},
	{"type": "wecom_app", "name": "企业微信应用", "fields": []string{"WECOM_QYDX_AGENT_ID", "WECOM_QYDX_CORP_ID", "WECOM_QYDX_SECRET", "WECOM_QYDX_TO_USER"}},
	{"type": "wechat_bot", "name": "企业微信智能机器人", "fields": []string{"WECHAT_BOT_KEY"}},
	{"type": "telegram", "name": "Telegram", "fields": []string{"TG_BOT_TOKEN", "TG_USER_ID", "TG_API_HOST"}},
	{"type": "serverchan", "name": "Server酱", "fields": []string{"SERVERCHAN_KEY", "SERVERCHAN_URL"}},
	{"type": "pushplus", "name": "PushPlus", "fields": []string{"PUSHPLUS_TOKEN", "PUSHPLUS_TOPIC"}},
	{"type": "webhook", "name": "自定义Webhook", "fields": []string{"WEBHOOK_URL", "WEBHOOK_METHOD", "WEBHOOK_CONTENT_TYPE"}},
	{"type": "ntfy", "name": "Ntfy", "fields": []string{"NTFY_URL", "NTFY_TOPIC", "NTFY_PRIORITY", "NTFY_TOKEN", "NTFY_USERNAME", "NTFY_PASSWORD", "NTFY_ACTIONS"}},
	{"type": "gotify", "name": "Gotify", "fields": []string{"GOTIFY_URL", "GOTIFY_TOKEN", "GOTIFY_PRIORITY"}},
	{"type": "pushdeer", "name": "PushDeer", "fields": []string{"PUSHDEER_KEY", "PUSHDEER_URL"}},
	{"type": "qqbot", "name": "QQ机器人", "fields": []string{"QQ_APP_ID", "QQ_APP_SECRET", "QQ_OPENID", "QQ_GROUP_OPENID"}},
	{"type": "wechat_claw", "name": "微信 ClawBot", "fields": []string{"WECHAT_CLAWBOT_BOT_TOKEN", "WECHAT_CLAWBOT_BASE_URL", "WECHAT_CLAWBOT_TO_USER", "WECHAT_CLAWBOT_ACCOUNT_ID"}},
	{"type": "igot", "name": "iGot", "fields": []string{"IGOT_PUSH_KEY"}},
	{"type": "synology-chat", "name": "Synology Chat", "fields": []string{"CHAT_URL", "CHAT_TOKEN"}},
	{"type": "qmsg", "name": "QMsg", "fields": []string{"QMSG_KEY", "QMSG_TYPE"}},
	{"type": "pushme", "name": "PushMe", "fields": []string{"PUSHME_KEY"}},
	{"type": "wxpusher", "name": "WxPusher", "fields": []string{"WXPUSHER_APP_TOKEN"}},
	{"type": "aibotk", "name": "AIBotK", "fields": []string{"AIBOTK_KEY", "AIBOTK_TYPE", "AIBOTK_NAME"}},
	{"type": "weplusbot", "name": "WePlusBot", "fields": []string{"WE_PLUS_BOT_TOKEN"}},
}

// allowedConfigKeys are keys permitted when creating or updating a channel.
var allowedConfigKeys = map[string]bool{
	"BARK_PUSH": true, "BARK_ICON": true, "BARK_SOUND": true, "BARK_GROUP": true, "BARK_LEVEL": true, "BARK_ARCHIVE": true, "BARK_URL": true,
	"DINGTALK_TOKEN": true, "DINGTALK_SECRET": true,
	"FEISHU_WEBHOOK": true, "FEISHU_SECRET": true,
	"WECOM_KEY": true, "WECOM_PROXY": true, "WECOM_QYDX_AGENT_ID": true, "WECOM_QYDX_CORP_ID": true, "WECOM_QYDX_SECRET": true, "WECOM_QYDX_TO_USER": true,
	"WECHAT_BOT_KEY": true,
	"TG_BOT_TOKEN": true, "TG_USER_ID": true, "TG_API_HOST": true,
	"SERVERCHAN_KEY": true, "SERVERCHAN_URL": true,
	"PUSHPLUS_TOKEN": true, "PUSHPLUS_TOPIC": true,
	"WEBHOOK_URL": true, "WEBHOOK_METHOD": true, "WEBHOOK_CONTENT_TYPE": true,
	"NTFY_URL": true, "NTFY_TOPIC": true, "NTFY_PRIORITY": true, "NTFY_TOKEN": true, "NTFY_USERNAME": true, "NTFY_PASSWORD": true, "NTFY_ACTIONS": true,
	"GOTIFY_URL": true, "GOTIFY_TOKEN": true, "GOTIFY_PRIORITY": true,
	"PUSHDEER_KEY": true, "PUSHDEER_URL": true,
	"QQ_APP_ID": true, "QQ_APP_SECRET": true, "QQ_OPENID": true, "QQ_GROUP_OPENID": true,
	"WECHAT_CLAWBOT_BOT_TOKEN": true, "WECHAT_CLAWBOT_BASE_URL": true, "WECHAT_CLAWBOT_TO_USER": true, "WECHAT_CLAWBOT_ACCOUNT_ID": true,
	"IGOT_PUSH_KEY": true,
	"CHAT_URL": true, "CHAT_TOKEN": true,
	"QMSG_KEY": true, "QMSG_TYPE": true,
	"PUSHME_KEY": true,
	"WXPUSHER_APP_TOKEN": true,
	"AIBOTK_KEY": true, "AIBOTK_TYPE": true, "AIBOTK_NAME": true,
	"WE_PLUS_BOT_TOKEN": true,
}

// validChannelTypes is the set of accepted channel type identifiers.
var validChannelTypes = map[string]bool{
	"bark": true, "dingtalk": true, "feishu": true, "feishu_app": true,
	"wecom": true, "wecom_app": true, "wechat_bot": true,
	"telegram": true, "serverchan": true, "pushplus": true,
	"webhook": true, "ntfy": true, "gotify": true, "pushdeer": true,
	"qqbot": true, "wechat_claw": true,
	"igot": true, "synology-chat": true, "qmsg": true, "pushme": true,
	"wxpusher": true, "aibotk": true, "weplusbot": true,
}

// Global notification store with file persistence.
var notifStore *services.NotificationStore

// InitNotificationStore initializes the persistent notification store.
func InitNotificationStore(dataDir string) {
	notifStore = services.NewNotificationStore(filepath.Join(dataDir, "notifications"))
}

// GetNotifStore returns the global notification store instance.
func GetNotifStore() *services.NotificationStore {
	return notifStore
}

// generateID creates a unique identifier.
func generateID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return fmt.Sprintf("%d-%s", time.Now().UnixMilli(), hex.EncodeToString(b))
}

// RegisterNotificationRoutes registers all notification-related routes.
func RegisterNotificationRoutes(rg *gin.RouterGroup) {
	// Initialize store if not already done
	if notifStore == nil {
		notifStore = services.NewNotificationStore("data/notifications")
	}

	// ==================== Settings ====================
	rg.GET("/settings", middleware.ValidateToken, middleware.APIRateLimit(120, 60000), getNotificationSettings)
	rg.POST("/settings", middleware.ValidateToken, middleware.ValidateCSRF, updateNotificationSettings)

	// ==================== Channels ====================
	rg.GET("/channels", middleware.ValidateToken, middleware.APIRateLimit(120, 60000), listChannels)
	rg.GET("/channels/types", middleware.ValidateToken, middleware.APIRateLimit(120, 60000), listChannelTypes)
	rg.POST("/channels", middleware.ValidateToken, middleware.ValidateCSRF, middleware.SensitiveActionRateLimit(10, 300000), addChannel)
	rg.PUT("/channels/:name", middleware.ValidateToken, middleware.ValidateCSRF, middleware.SensitiveActionRateLimit(10, 300000), updateChannel)
	rg.DELETE("/channels/:name", middleware.ValidateToken, middleware.ValidateCSRF, middleware.SensitiveActionRateLimit(10, 300000), deleteChannel)
	rg.POST("/channels/:name/test", middleware.ValidateToken, middleware.ValidateCSRF, middleware.SensitiveActionRateLimit(5, 60000), testChannel)

	// ==================== Rules ====================
	rg.GET("/rules", middleware.ValidateToken, middleware.APIRateLimit(120, 60000), listRules)
	rg.GET("/rules/:id", middleware.ValidateToken, middleware.APIRateLimit(120, 60000), getRule)
	rg.POST("/rules", middleware.ValidateToken, middleware.ValidateCSRF, middleware.SensitiveActionRateLimit(10, 300000), addRule)
	rg.PUT("/rules/:id", middleware.ValidateToken, middleware.ValidateCSRF, middleware.SensitiveActionRateLimit(10, 300000), updateRule)
	rg.DELETE("/rules/:id", middleware.ValidateToken, middleware.ValidateCSRF, middleware.SensitiveActionRateLimit(10, 300000), deleteRule)
	rg.POST("/rules/:id/toggle", middleware.ValidateToken, middleware.ValidateCSRF, toggleRule)
	rg.POST("/rules/:id/test", middleware.ValidateToken, middleware.ValidateCSRF, testRuleMatch)

	// ==================== History ====================
	rg.GET("/history", middleware.ValidateToken, middleware.APIRateLimit(120, 60000), getNotificationHistory)
	rg.POST("/history/clean", middleware.ValidateToken, middleware.ValidateCSRF, clearNotificationHistory)

	// ==================== Stats ====================
	rg.GET("/stats", middleware.ValidateToken, middleware.APIRateLimit(120, 60000), getNotificationStats)

	// ==================== Monitor ====================
	rg.GET("/monitor/status", middleware.ValidateToken, middleware.APIRateLimit(120, 60000), getMonitorStatus)
	rg.POST("/monitor/start", middleware.ValidateToken, middleware.ValidateCSRF, startMonitor)
	rg.POST("/monitor/stop", middleware.ValidateToken, middleware.ValidateCSRF, stopMonitor)
	rg.POST("/monitor/check", middleware.ValidateToken, middleware.ValidateCSRF, triggerMonitorCheck)

	// ==================== QQ Bot ====================
	rg.POST("/qqbot/event", middleware.APIRateLimit(120, 60000), qqBotEventHandler)
	rg.GET("/qqbot/captured", middleware.ValidateToken, middleware.APIRateLimit(120, 60000), getQQBotCaptured)
	rg.POST("/qqbot/listen/start", middleware.ValidateToken, middleware.ValidateCSRF, startQQBotListen)
	rg.POST("/qqbot/listen/stop", middleware.ValidateToken, middleware.ValidateCSRF, stopQQBotListen)

	// ==================== WeChat Claw ====================
	rg.GET("/wechat-claw/qrcode", middleware.ValidateToken, middleware.APIRateLimit(30, 60000), wechatClawQRCode)
	rg.GET("/wechat-claw/status", middleware.ValidateToken, middleware.APIRateLimit(60, 60000), wechatClawStatus)
	rg.GET("/wechat-claw/captured", middleware.ValidateToken, middleware.APIRateLimit(120, 60000), wechatClawCaptured)
	rg.GET("/wechat-claw/updates", middleware.ValidateToken, wechatClawUpdates)

	// ==================== WebSocket (no ValidateToken - WS handler authenticates internally) ====================
	rg.GET("/ws", notifyWSHandler)
}

func notifyWSHandler(c *gin.Context) {
	services.GetNotifyHub().ServeWS(c.Writer, c.Request)
}

// ==================== Settings Handlers ====================

func getNotificationSettings(c *gin.Context) {
	settings := notifStore.GetSettings()
	c.JSON(http.StatusOK, gin.H{"settings": settings})
}

func updateNotificationSettings(c *gin.Context) {
	var body map[string]interface{}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求数据"})
		return
	}

	settings := notifStore.GetSettings()

	if enabled, ok := body["enabled"]; ok {
		if val, ok2 := enabled.(bool); ok2 {
			settings.Enabled = val
		}
	}
	if interval, ok := body["checkInterval"]; ok {
		if val, ok2 := interval.(float64); ok2 && val >= 5000 && val <= 300000 {
			settings.CheckInterval = int(val)
		}
	}
	if days, ok := body["maxHistoryDays"]; ok {
		if val, ok2 := days.(float64); ok2 && val >= 1 && val <= 30 {
			settings.MaxHistoryDays = int(val)
		}
	}
	if count, ok := body["maxHistoryCount"]; ok {
		if val, ok2 := count.(float64); ok2 && val >= 100 && val <= 10000 {
			settings.MaxHistoryCount = int(val)
		}
	}

	_ = notifStore.UpdateSettings(settings)

	// Restart monitor if enabled state changed
	if settings.Enabled {
		_ = services.StartMonitor()
	} else {
		_ = services.StopMonitor()
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "settings": settings})
}

// ==================== Channel Handlers ====================

// channelConfigToMap converts a NotificationChannelConfig to map for JSON response.
func channelConfigToMap(ch services.NotificationChannelConfig) map[string]interface{} {
	m := map[string]interface{}{
		"id":      ch.ID,
		"name":    ch.Name,
		"channel": ch.Channel,
		"enabled": ch.Enabled,
	}
	for k, v := range ch.Config {
		m[k] = v
	}
	return m
}

func listChannels(c *gin.Context) {
	channels := notifStore.GetChannels()
	result := make([]map[string]interface{}, len(channels))
	for i, ch := range channels {
		result[i] = channelConfigToMap(ch)
	}
	c.JSON(http.StatusOK, gin.H{"channels": result})
}

func listChannelTypes(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"types": supportedChannelTypes})
}

func addChannel(c *gin.Context) {
	var body map[string]interface{}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求数据"})
		return
	}

	channelType, _ := body["channel"].(string)
	name, _ := body["name"].(string)

	if channelType == "" || name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少必要参数"})
		return
	}

	if !validChannelTypes[channelType] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "不支持的渠道类型"})
		return
	}

	configMap, _ := body["config"].(map[string]interface{})
	filteredConfig := make(map[string]interface{})
	for k, v := range configMap {
		if allowedConfigKeys[k] {
			filteredConfig[k] = v
		}
	}

	ch := services.NotificationChannelConfig{
		ID:      generateID(),
		Name:    name,
		Channel: channelType,
		Enabled: true,
		Config:  filteredConfig,
	}

	if err := notifStore.AddChannel(ch); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "channel": channelConfigToMap(ch)})
}

func updateChannel(c *gin.Context) {
	channelName := c.Param("name")

	var body map[string]interface{}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求数据"})
		return
	}

	// Mapping from frontend camelCase field names to config UPPER_CASE keys.
	// The frontend may send camelCase (e.g., qqOpenId) while the backend
	// stores config under UPPER_CASE keys (e.g., QQ_OPENID).
	var frontendToConfigKey = map[string]string{
		"qqOpenId":           "QQ_OPENID",
		"qqGroupOpenId":      "QQ_GROUP_OPENID",
		"qqAppId":            "QQ_APP_ID",
		"qqAppSecret":        "QQ_APP_SECRET",
		"wechatClawBotToken": "WECHAT_CLAWBOT_BOT_TOKEN",
		"wechatClawBaseUrl":  "WECHAT_CLAWBOT_BASE_URL",
		"wechatClawToUser":   "WECHAT_CLAWBOT_TO_USER",
		"wechatClawAccountId":"WECHAT_CLAWBOT_ACCOUNT_ID",
	}

	// Filter allowed fields.
	// Config keys (allowedConfigKeys) are routed into a "config" sub-map
	// so that UpdateChannel merges them into the channel's Config map.
	// CamelCase frontend keys are also accepted and mapped to UPPER_CASE.
	updates := make(map[string]interface{})
	ensureConfig := func() {
		if updates["config"] == nil {
			updates["config"] = make(map[string]interface{})
		}
	}
	for k, v := range body {
		if k == "enabled" || k == "name" {
			updates[k] = v
		} else if allowedConfigKeys[k] {
			ensureConfig()
			updates["config"].(map[string]interface{})[k] = v
		} else if configKey, ok := frontendToConfigKey[k]; ok {
			if s, ok2 := v.(string); ok2 && s != "" {
				ensureConfig()
				updates["config"].(map[string]interface{})[configKey] = s
			}
		}
	}

	if err := notifStore.UpdateChannel(channelName, updates); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

func deleteChannel(c *gin.Context) {
	channelName := c.Param("name")
	if err := notifStore.DeleteChannel(channelName); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func testChannel(c *gin.Context) {
	channelName := c.Param("name")
	ch := notifStore.GetChannel(channelName)
	if ch == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "渠道不存在"})
		return
	}

	chType := ch.Channel

	// QQ bot special handling: if no openID yet, start WebSocket listener to capture one
	if chType == "qqbot" {
		hasOpenID := ""
		hasGroupOpenID := ""
		if ch.Config != nil {
			if v, ok := ch.Config["QQ_OPENID"].(string); ok {
				hasOpenID = v
			}
			if v, ok := ch.Config["QQ_GROUP_OPENID"].(string); ok {
				hasGroupOpenID = v
			}
		}
		// Check if already captured
		capturedOpenID, capturedGroupOpenID := services.GetQQBotCaptured()
		if hasOpenID == "" && hasGroupOpenID == "" && (capturedOpenID != "" || capturedGroupOpenID != "") {
			c.JSON(http.StatusOK, gin.H{
				"result": map[string]interface{}{
					"success":      false,
					"message":      "已获取到 OpenID，请保存后再测试发送",
					"openid":       capturedOpenID,
					"groupOpenid":  capturedGroupOpenID,
				},
			})
			return
		}
		if hasOpenID == "" && hasGroupOpenID == "" {
			// Push channel config to notify system so StartQQBotListen can read it
			if ch.Config != nil {
				configStr := make(map[string]string, len(ch.Config))
				for key, val := range ch.Config {
					if str, ok := val.(string); ok {
						configStr[key] = str
					}
				}
				notify.SetConfigs(configStr)
			}
			// Start the WebSocket listener (actual implementation, not mock)
			listenResult, listenMsg := services.StartQQBotListen()
			if !listenResult {
				c.JSON(http.StatusOK, gin.H{
					"result": map[string]interface{}{
						"success": false,
						"message": "启动监听失败: " + listenMsg,
					},
				})
				return
			}
			c.JSON(http.StatusOK, gin.H{
				"result": map[string]interface{}{
					"success": false,
					"message": "已启动监听，请在60秒内给QQ机器人发消息。发完后再次点击\"测试\"查看是否获取到openID",
				},
			})
			return
		}
	}

	// WeChat Claw special handling
	if chType == "wechat_claw" {
		toUser := ""
		if ch.Config != nil {
			if v, ok := ch.Config["WECHAT_CLAWBOT_TO_USER"].(string); ok {
				toUser = v
			}
		}
		if toUser == "" {
			// Check if bot token already exists (captured from QR login or configured)
			capturedToken, _, _ := channels.GetCapturedCredentials()
			botToken := ""
			if ch.Config != nil {
				if v, ok := ch.Config["WECHAT_CLAWBOT_BOT_TOKEN"].(string); ok {
					botToken = v
				}
			}
			if capturedToken != "" || botToken != "" {
				c.JSON(http.StatusOK, gin.H{
					"result": map[string]interface{}{
						"success": false,
						"message": "凭证已就绪，请填写发送目标（用户ID）后再次测试",
					},
				})
			} else {
				c.JSON(http.StatusOK, gin.H{
					"result": map[string]interface{}{
						"success": false,
						"message": "Bot Token 未配置，请先扫码登录",
					},
				})
			}
			return
		}
	}

	// Send test through the specific channel using the notify package
	configMap := ch.Config
	if configMap == nil {
		configMap = make(map[string]interface{})
	}

	err := services.SendNotificationToChannel("测试通知", "这是来自 fnos-logmanager 的测试通知", chType, configMap)
	if err != nil {
		slog.Error("渠道测试失败", "channel", channelName, "type", chType, "error", err)
		c.JSON(http.StatusOK, gin.H{
			"result": map[string]interface{}{
				"success": false,
				"message": err.Error(),
			},
		})
		return
	}

	// Update last interacted user for WeChat ClawBot on successful test
	if chType == "wechat_claw" {
		toUser := ""
		if ch.Config != nil {
			if v, ok := ch.Config["WECHAT_CLAWBOT_TO_USER"].(string); ok {
				toUser = v
			}
		}
		if toUser != "" {
			services.SetClawBotLastInteractedUser(toUser)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"result": map[string]interface{}{
			"success": true,
			"message": "测试通知已发送",
		},
	})
}

// ==================== Rule Handlers ====================

func listRules(c *gin.Context) {
	rules := notifStore.GetRules()
	c.JSON(http.StatusOK, gin.H{"rules": rules})
}

func getRule(c *gin.Context) {
	ruleID := c.Param("id")
	rule := notifStore.GetRule(ruleID)
	if rule == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "规则不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"rule": rule})
}

func ruleToStore(rule map[string]interface{}) services.NotificationRule {
	r := services.NotificationRule{
		ID:               getStringField(rule, "id"),
		Name:             getStringField(rule, "name"),
		Status:           "enabled",
		AppName:          getStringField(rule, "appName"),
		LogLevel:         getStringField(rule, "logLevel"),
		Cooldown:         60,
		MaxNotifications: 10,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	if chs, ok := rule["channels"].([]interface{}); ok {
		for _, ch := range chs {
			if s, ok := ch.(string); ok {
				r.Channels = append(r.Channels, s)
			}
		}
	}
	if lps, ok := rule["logPaths"].([]interface{}); ok {
		for _, lp := range lps {
			if s, ok := lp.(string); ok {
				r.LogPaths = append(r.LogPaths, s)
			}
		}
	}
	if kws, ok := rule["keywords"].([]interface{}); ok {
		for _, kw := range kws {
			if s, ok := kw.(string); ok {
				r.Keywords = append(r.Keywords, s)
			}
		}
	}
	if eks, ok := rule["excludeKeywords"].([]interface{}); ok {
		for _, ek := range eks {
			if s, ok := ek.(string); ok {
				r.ExcludeKeywords = append(r.ExcludeKeywords, s)
			}
		}
	}

	if r.LogLevel == "" {
		r.LogLevel = "all"
	}

	if v, ok := rule["pattern"].(string); ok {
		r.Pattern = v
	}
	if v, ok := rule["status"].(string); ok {
		r.Status = v
	}
	if v, ok := rule["cooldown"].(float64); ok {
		r.Cooldown = int(v)
	}
	if v, ok := rule["maxNotifications"].(float64); ok {
		r.MaxNotifications = int(v)
	}
	if v, ok := rule["quietHoursStart"].(string); ok {
		r.QuietHoursStart = v
	}
	if v, ok := rule["quietHoursEnd"].(string); ok {
		r.QuietHoursEnd = v
	}
	if iws, ok := rule["ipWhitelist"].([]interface{}); ok {
		for _, iw := range iws {
			if s, ok := iw.(string); ok {
				r.IPWhitelist = append(r.IPWhitelist, s)
			}
		}
	}

	return r
}

func addRule(c *gin.Context) {
	var body map[string]interface{}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求数据"})
		return
	}

	name, _ := body["name"].(string)
	appName, _ := body["appName"].(string)
	logLevel, _ := body["logLevel"].(string)

	if name == "" || appName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少必要参数"})
		return
	}

	channels, ok := body["channels"].([]interface{})
	if !ok || len(channels) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "至少需要指定一个通知渠道"})
		return
	}

	chStr := make([]string, len(channels))
	for i, ch := range channels {
		if s, ok := ch.(string); ok {
			chStr[i] = s
		}
	}

	rule := services.NotificationRule{
		ID:               generateID(),
		Name:             name,
		Status:           "enabled",
		AppName:          appName,
		LogLevel:         logLevel,
		Channels:         chStr,
		Cooldown:         60,
		MaxNotifications: 10,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	if lps, ok := body["logPaths"].([]interface{}); ok {
		for _, lp := range lps {
			if s, ok := lp.(string); ok {
				rule.LogPaths = append(rule.LogPaths, s)
			}
		}
	}
	if kws, ok := body["keywords"].([]interface{}); ok {
		for _, kw := range kws {
			if s, ok := kw.(string); ok {
				rule.Keywords = append(rule.Keywords, s)
			}
		}
	}
	if eks, ok := body["excludeKeywords"].([]interface{}); ok {
		for _, ek := range eks {
			if s, ok := ek.(string); ok {
				rule.ExcludeKeywords = append(rule.ExcludeKeywords, s)
			}
		}
	}
	if v, ok := body["pattern"].(string); ok {
		rule.Pattern = v
	}
	if v, ok := body["cooldown"].(float64); ok {
		rule.Cooldown = int(v)
	}
	if v, ok := body["maxNotifications"].(float64); ok {
		rule.MaxNotifications = int(v)
	}
	if v, ok := body["quietHoursStart"].(string); ok {
		rule.QuietHoursStart = v
	}
	if v, ok := body["quietHoursEnd"].(string); ok {
		rule.QuietHoursEnd = v
	}
	if iws, ok := body["ipWhitelist"].([]interface{}); ok {
		for _, iw := range iws {
			if s, ok := iw.(string); ok {
				rule.IPWhitelist = append(rule.IPWhitelist, s)
			}
		}
	}

	if err := notifStore.AddRule(rule); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "rule": rule})
}

func updateRule(c *gin.Context) {
	ruleID := c.Param("id")

	var body map[string]interface{}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求数据"})
		return
	}

	existing := notifStore.GetRule(ruleID)
	if existing == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "规则不存在"})
		return
	}

	// Apply updates
	allowedRuleKeys := map[string]bool{
		"name": true, "appName": true, "logPaths": true, "logLevel": true,
		"keywords": true, "excludeKeywords": true, "pattern": true,
		"channels": true, "cooldown": true, "maxNotifications": true,
		"quietHoursStart": true, "quietHoursEnd": true, "status": true,
		"ipWhitelist": true,
	}

	updated := *existing
	for k, v := range body {
		if allowedRuleKeys[k] {
			switch k {
			case "name":
				if s, ok := v.(string); ok {
					updated.Name = s
				}
			case "appName":
				if s, ok := v.(string); ok {
					updated.AppName = s
				}
			case "logLevel":
				if s, ok := v.(string); ok {
					updated.LogLevel = s
				}
			case "status":
				if s, ok := v.(string); ok {
					updated.Status = s
				}
			case "pattern":
				if s, ok := v.(string); ok {
					updated.Pattern = s
				}
			case "cooldown":
				if f, ok := v.(float64); ok {
					updated.Cooldown = int(f)
				}
			case "maxNotifications":
				if f, ok := v.(float64); ok {
					updated.MaxNotifications = int(f)
				}
			case "quietHoursStart":
				if s, ok := v.(string); ok {
					updated.QuietHoursStart = s
				}
			case "quietHoursEnd":
				if s, ok := v.(string); ok {
					updated.QuietHoursEnd = s
				}
			case "channels":
				if arr, ok := v.([]interface{}); ok {
					var strs []string
					for _, item := range arr {
						if s, ok := item.(string); ok {
							strs = append(strs, s)
						}
					}
					updated.Channels = strs
				}
			case "keywords":
				if arr, ok := v.([]interface{}); ok {
					var strs []string
					for _, item := range arr {
						if s, ok := item.(string); ok {
							strs = append(strs, s)
						}
					}
					updated.Keywords = strs
				}
			case "excludeKeywords":
				if arr, ok := v.([]interface{}); ok {
					var strs []string
					for _, item := range arr {
						if s, ok := item.(string); ok {
							strs = append(strs, s)
						}
					}
					updated.ExcludeKeywords = strs
				}
			case "logPaths":
				if arr, ok := v.([]interface{}); ok {
					var strs []string
					for _, item := range arr {
						if s, ok := item.(string); ok {
							strs = append(strs, s)
						}
					}
					updated.LogPaths = strs
				}
			case "ipWhitelist":
				if arr, ok := v.([]interface{}); ok {
					var strs []string
					for _, item := range arr {
						if s, ok := item.(string); ok {
							strs = append(strs, s)
						}
					}
					updated.IPWhitelist = strs
				}
			}
		}
	}

	if err := notifStore.UpdateRule(ruleID, updated); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "rule": updated})
}

func deleteRule(c *gin.Context) {
	ruleID := c.Param("id")
	if err := notifStore.DeleteRule(ruleID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func toggleRule(c *gin.Context) {
	ruleID := c.Param("id")

	rule := notifStore.GetRule(ruleID)
	if rule == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "规则不存在"})
		return
	}

	if rule.Status == "enabled" {
		rule.Status = "disabled"
	} else {
		rule.Status = "enabled"
	}

	if err := notifStore.UpdateRule(ruleID, *rule); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "status": rule.Status})
}

func testRuleMatch(c *gin.Context) {
	ruleID := c.Param("id")

	var body struct {
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.Content == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少测试内容"})
		return
	}

	rule := notifStore.GetRule(ruleID)
	if rule == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "规则不存在"})
		return
	}

	// Simple keyword matching
	matched := false
	for _, kw := range rule.Keywords {
		if kw != "" && strings.Contains(body.Content, kw) {
			matched = true
			break
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"result": map[string]interface{}{
			"matched": matched,
			"message": "测试完成",
		},
	})
}

// ==================== History Handlers ====================

func getNotificationHistory(c *gin.Context) {
	limit := 100
	if l := c.Query("limit"); l != "" {
		if parsed, err := fmt.Sscanf(l, "%d", &limit); err != nil || parsed != 1 || limit <= 0 || limit > 500 {
			limit = 100
		}
	}

	history := notifStore.GetHistory(limit)
	c.JSON(http.StatusOK, gin.H{"history": history})
}

func clearNotificationHistory(c *gin.Context) {
	_ = notifStore.ClearHistory()
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// ==================== Stats Handler ====================

func getNotificationStats(c *gin.Context) {
	channels := notifStore.GetChannels()
	rules := notifStore.GetRules()
	history := notifStore.GetHistory(1000)
	settings := notifStore.GetSettings()

	// Count enabled/disabled
	ruleCount := len(rules)
	channelCount := len(channels)
	historyCount := len(history)

	stats := services.NotificationStats{
		TotalSent:  historyCount,
		ByChannel:  make(map[string]services.ChannelStat),
		ByRule:     make(map[string]services.ChannelStat),
	}

	for _, h := range history {
		if h.Success {
			stats.SuccessCount++
		} else {
			stats.FailCount++
		}

		cs := stats.ByChannel[h.Channel]
		cs.Sent++
		if h.Success {
			cs.Success++
		} else {
			cs.Failed++
		}
		stats.ByChannel[h.Channel] = cs
	}

	c.JSON(http.StatusOK, gin.H{
		"stats": map[string]interface{}{
			"channelCount":  channelCount,
			"ruleCount":     ruleCount,
			"historyCount":  historyCount,
			"totalSent":     stats.TotalSent,
			"totalFailed":   stats.FailCount,
			"enabled":       settings.Enabled,
		},
	})
}

// ==================== Monitor Handlers ====================

func getMonitorStatus(c *gin.Context) {
	status := services.GetMonitorStatus()
	c.JSON(http.StatusOK, gin.H{"status": status})
}

func startMonitor(c *gin.Context) {
	if err := services.StartMonitor(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "status": services.GetMonitorStatus()})
}

func stopMonitor(c *gin.Context) {
	if err := services.StopMonitor(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "status": services.GetMonitorStatus()})
}

func triggerMonitorCheck(c *gin.Context) {
	if err := services.TriggerMonitorCheck(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "status": services.GetMonitorStatus()})
}

// ==================== QQ Bot Handlers ====================

func qqBotEventHandler(c *gin.Context) {
	var body map[string]interface{}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "message": "invalid event"})
		return
	}
	if _, ok := body["t"]; !ok {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "message": "invalid event"})
		return
	}
	services.HandleQQBotEvent(body)
	c.JSON(http.StatusOK, gin.H{"code": 0})
}

func getQQBotCaptured(c *gin.Context) {
	openID, groupOpenID := services.GetQQBotCaptured()
	c.JSON(http.StatusOK, gin.H{
		"openId":      openID,
		"groupOpenId": groupOpenID,
	})
}

func startQQBotListen(c *gin.Context) {
	success, message := services.StartQQBotListen()
	if !success {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": message,
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": message,
	})
}

func stopQQBotListen(c *gin.Context) {
	services.StopQQBotListen()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "监听已停止",
	})
}

// ==================== WeChat Claw Handlers ====================

func wechatClawQRCode(c *gin.Context) {
	success, qrcode, qrcodeURL, message := services.GetClawBotQRCode()
	if !success {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": message,
		})
		return
	}

	// Try to get base64 QR code image
	var qrcodeBase64 string

	// Case 1: API already returned a data URL
	if strings.HasPrefix(qrcodeURL, "data:image/") {
		qrcodeBase64 = strings.Replace(qrcodeURL, "data:image/png;base64,", "", 1)
	} else if qrcodeURL != "" {
		// Case 2: Encode URL as QR code image
		if png, err := qrgen.Encode(qrcodeURL, qrgen.Medium, 280); err == nil {
			qrcodeBase64 = base64.StdEncoding.EncodeToString(png)
		}
	} else if qrcode != "" {
		// Case 3: Encode qrcode ID as QR code image
		if png, err := qrgen.Encode(qrcode, qrgen.Medium, 280); err == nil {
			qrcodeBase64 = base64.StdEncoding.EncodeToString(png)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success":      true,
		"qrcode":       qrcode,
		"qrcodeUrl":    qrcodeURL,
		"qrcodeBase64": qrcodeBase64,
		"message":      message,
	})
}

func wechatClawStatus(c *gin.Context) {
	qrcode := c.Query("qrcode")
	if qrcode == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "缺少 qrcode 参数"})
		return
	}

	success, status, token, accountID, scannerID, message := services.CheckClawBotQRCodeStatus(qrcode)
	c.JSON(http.StatusOK, gin.H{
		"success":   success,
		"status":    status,
		"token":     token,
		"accountId": accountID,
		"scannerId": scannerID,
		"message":   message,
		"qrcode":    qrcode,
	})
}

func wechatClawCaptured(c *gin.Context) {
	botToken, accountID, baseURL := services.GetClawBotCaptured()
	c.JSON(http.StatusOK, gin.H{
		"botToken":  botToken,
		"accountId": accountID,
		"baseUrl":   baseURL,
	})
}

func wechatClawUpdates(c *gin.Context) {
	success, messages, message := services.GetClawBotUpdates()
	if !success {
		c.JSON(http.StatusOK, gin.H{
			"success":  false,
			"messages": []interface{}{},
			"message":  message,
		})
		return
	}

	// Convert to []interface{} for JSON
	items := make([]interface{}, len(messages))
	for i, m := range messages {
		items[i] = m
	}
	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"messages": items,
		"message":  message,
	})
}

// getStringField safely extracts a string from a map.
func getStringField(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}
