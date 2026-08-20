package services

import (
	"fmt"
	"log/slog"

	"github.com/sushazhi/fnos-logmanager/internal/notify"
)

// NotifyRequest represents a notification to send.
type NotifyRequest struct {
	Title   string `json:"title"`
	Message string `json:"message"`
	Type    string `json:"type"` // info, warning, error
}

// routeToNotifyName maps route-layer channel type names to notify registry channel names.
var routeToNotifyName = map[string]string{
	"bark":          "bark",
	"dingtalk":      "dingtalk",
	"feishu":        "feishu-bot",
	"feishu_app":    "feishu-app",
	"wecom":         "wechat-work-bot",
	"wecom_app":     "wechat-work-app",
	"wechat_bot":    "wechat-bot",
	"telegram":      "telegram",
	"serverchan":    "serverChan",
	"pushplus":      "pushplus",
	"webhook":       "webhook",
	"ntfy":          "ntfy",
	"gotify":        "gotify",
	"pushdeer":      "pushdeer",
	"qqbot":         "qqbot",
	"wechat_claw":   "wechat-claw-bot",
	"igot":          "igot",
	"synology-chat": "synology-chat",
	"qmsg":          "qmsg",
	"pushme":        "pushme",
	"wxpusher":      "wxpusher",
	"aibotk":        "aibotk",
	"weplusbot":     "weplusbot",
}

// SendNotification sends a notification through all enabled notification channels.
func SendNotification(req NotifyRequest) error {
	slog.Info("sending notification", "title", req.Title, "type", req.Type)

	result := notify.SendNotify(req.Title, req.Message)
	if !result.Success {
		return fmt.Errorf("notification failed: %s", result.Message)
	}
	return nil
}

// SendTestNotification sends a test notification through all enabled channels.
func SendTestNotification() error {
	return SendNotification(NotifyRequest{
		Title:   "测试通知",
		Message: "这是来自 fnos-logmanager 的测试通知",
		Type:    "info",
	})
}

// SendNotificationToChannel sends a notification through a specific channel by route type name.
// Config keys are set into the notify runtime config before sending.
func SendNotificationToChannel(title, message string, channelType string, config map[string]interface{}) error {
	// Map the route-layer channel type to notify registry channel name
	notifyName, ok := routeToNotifyName[channelType]
	if !ok {
		notifyName = channelType
	}

	// Convert map[string]interface{} to map[string]string for SetConfigs.
	// Non-string values (numbers, booleans) are rendered with fmt.Sprint so
	// numeric config like priorities/timeouts is not silently dropped.
	configStr := make(map[string]string, len(config))
	for key, val := range config {
		switch v := val.(type) {
		case string:
			configStr[key] = v
		case nil:
			// skip
		default:
			configStr[key] = fmt.Sprint(v)
		}
	}
	{
		keys := make([]string, 0, len(configStr))
		for k := range configStr {
			keys = append(keys, k)
		}
		slog.Info("推送通知渠道配置", "channel", channelType, "keys", keys, "qqAppId_present", configStr["QQ_APP_ID"] != "")
	}

	// Get the channel from registry and enable it temporarily
	ch := notify.Registry.Get(notifyName)
	if ch == nil {
		return fmt.Errorf("通知渠道 %s (notify: %s) 未注册", channelType, notifyName)
	}

	// Set the channel config for this send, then restore the previous global
	// config afterwards. notify.SetConfigs writes the process-global config
	// that ALL channels read, so without snapshot/restore a one-off send would
	// permanently overwrite every other channel's credentials.
	snapshot := notify.SnapshotConfig()
	notify.SetConfigs(configStr)
	defer notify.RestoreConfig(snapshot)

	wasEnabled := ch.Enabled()
	if !wasEnabled {
		ch.SetEnabled(true)
	}

	result := ch.Send(title, message)

	if !wasEnabled {
		ch.SetEnabled(false)
	}

	if !result.Success {
		return fmt.Errorf("%s 发送失败: %s", channelType, result.Message)
	}
	return nil
}

// notifyEmail (deprecated - use notify package instead)
func notifyEmail(body string) {
	req := NotifyRequest{
		Title:   "Log Monitor Alert",
		Message: body,
		Type:    "warning",
	}
	_ = SendNotification(req)
}

// notifySyslog (deprecated - use slog directly instead)
func notifySyslog(body string) {
	slog.Warn("syslog notification", "message", body)
}

// SyncNotifyStoreToRegistry syncs the persisted notification store channel configurations
// to the runtime notification registry. This ensures channels configured via the UI
// (which are saved to notification-config.json) remain active after a restart/upgrade,
// rather than relying solely on environment variables which are only set at process start.
func SyncNotifyStoreToRegistry() {
	if notifStore == nil {
		slog.Warn("SyncNotifyStoreToRegistry: notification store not initialized, skipping")
		return
	}

	channels := notifStore.GetChannels()
	synced := 0
	for _, ch := range channels {
		// Map route-layer channel type to notify registry name
		notifyName, ok := routeToNotifyName[ch.Channel]
		if !ok {
			notifyName = ch.Channel
		}

		regCh := notify.Registry.Get(notifyName)
		if regCh == nil {
			slog.Warn("通知渠道未注册，跳过同步", "name", ch.Name, "channel", ch.Channel, "registryName", notifyName)
			continue
		}

		// Set config values from persisted store (convert map[string]interface{} to map[string]string)
		if len(ch.Config) > 0 {
			configStr := make(map[string]string, len(ch.Config))
			for key, val := range ch.Config {
				switch v := val.(type) {
				case string:
					configStr[key] = v
				case nil:
					// skip
				default:
					configStr[key] = fmt.Sprint(v)
				}
			}
			if len(configStr) > 0 {
				notify.SetConfigs(configStr)
			}
		}

		// Apply persisted enabled state to runtime registry
		regCh.SetEnabled(ch.Enabled)

		if ch.Enabled {
			slog.Debug("通知渠道已从持久配置启用", "name", ch.Name, "channel", ch.Channel)
		}
		synced++
	}

	if synced > 0 {
		slog.Info("通知渠道配置同步完成", "synced", synced, "total", len(channels))
	}
}
