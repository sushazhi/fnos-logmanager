package notify

import (
	"log/slog"
	"os"
	"reflect"
	"strconv"
	"strings"
)

// pushConfig holds the loaded channel configuration.
var pushConfig ChannelConfig

// init loads channel config from environment variables at package init time.
func init() {
	loadConfigFromEnv()
}

// loadConfigFromEnv reads all ChannelConfig fields from environment variables.
func loadConfigFromEnv() {
	v := &pushConfig

	// Bark
	v.BARK_PUSH = os.Getenv("BARK_PUSH")
	v.BARK_ARCHIVE = os.Getenv("BARK_ARCHIVE")
	v.BARK_GROUP = os.Getenv("BARK_GROUP")
	v.BARK_SOUND = os.Getenv("BARK_SOUND")
	v.BARK_ICON = os.Getenv("BARK_ICON")
	v.BARK_LEVEL = os.Getenv("BARK_LEVEL")
	v.BARK_URL = os.Getenv("BARK_URL")

	// DingTalk
	v.DD_BOT_SECRET = os.Getenv("DD_BOT_SECRET")
	v.DD_BOT_TOKEN = os.Getenv("DD_BOT_TOKEN")

	// Feishu
	v.FSKEY = os.Getenv("FSKEY")
	v.FSSECRET = os.Getenv("FSSECRET")
	v.FEISHU_APP_ID = os.Getenv("FEISHU_APP_ID")
	v.FEISHU_APP_SECRET = os.Getenv("FEISHU_APP_SECRET")
	v.FEISHU_USER_ID = os.Getenv("FEISHU_USER_ID")

	// Gotify
	v.GOTIFY_URL = os.Getenv("GOTIFY_URL")
	v.GOTIFY_TOKEN = os.Getenv("GOTIFY_TOKEN")
	if p := os.Getenv("GOTIFY_PRIORITY"); p != "" {
		if n, err := strconv.Atoi(p); err == nil {
			v.GOTIFY_PRIORITY = n
		}
	}

	// iGot
	v.IGOT_PUSH_KEY = os.Getenv("IGOT_PUSH_KEY")

	// ServerChan
	v.PUSH_KEY = os.Getenv("PUSH_KEY")

	// PushDeer
	v.DEER_KEY = os.Getenv("DEER_KEY")
	v.DEER_URL = os.Getenv("DEER_URL")

	// Synology Chat
	v.CHAT_URL = os.Getenv("CHAT_URL")
	v.CHAT_TOKEN = os.Getenv("CHAT_TOKEN")

	// PushPlus
	v.PUSH_PLUS_TOKEN = os.Getenv("PUSH_PLUS_TOKEN")
	v.PUSH_PLUS_USER = os.Getenv("PUSH_PLUS_USER")
	v.PUSH_PLUS_TEMPLATE = envOrDefault("PUSH_PLUS_TEMPLATE", "html")
	v.PUSH_PLUS_CHANNEL = envOrDefault("PUSH_PLUS_CHANNEL", "wechat")
	v.PUSH_PLUS_WEBHOOK = os.Getenv("PUSH_PLUS_WEBHOOK")
	v.PUSH_PLUS_CALLBACKURL = os.Getenv("PUSH_PLUS_CALLBACKURL")
	v.PUSH_PLUS_TO = os.Getenv("PUSH_PLUS_TO")

	// WePlusBot
	v.WE_PLUS_BOT_TOKEN = os.Getenv("WE_PLUS_BOT_TOKEN")
	v.WE_PLUS_BOT_RECEIVER = os.Getenv("WE_PLUS_BOT_RECEIVER")
	v.WE_PLUS_BOT_VERSION = envOrDefault("WE_PLUS_BOT_VERSION", "pro")

	// QMsg
	v.QMSG_KEY = os.Getenv("QMSG_KEY")
	v.QMSG_TYPE = os.Getenv("QMSG_TYPE")

	// WeChat Work
	v.QYWX_ORIGIN = envOrDefault("QYWX_ORIGIN", "https://qyapi.weixin.qq.com")
	v.QYWX_AM = os.Getenv("QYWX_AM")
	v.QYWX_KEY = os.Getenv("QYWX_KEY")

	// WeChat Bot (Smart)
	v.WECHAT_BOT_ID = os.Getenv("WECHAT_BOT_ID")
	v.WECHAT_BOT_SECRET = os.Getenv("WECHAT_BOT_SECRET")
	v.WECHAT_BOT_CHAT_ID = os.Getenv("WECHAT_BOT_CHAT_ID")
	v.WECHAT_BOT_WS_URL = envOrDefault("WECHAT_BOT_WS_URL", "wss://openws.work.weixin.qq.com")

	// Telegram
	v.TG_BOT_TOKEN = os.Getenv("TG_BOT_TOKEN")
	v.TG_USER_ID = os.Getenv("TG_USER_ID")
	v.TG_API_HOST = envOrDefault("TG_API_HOST", "https://api.telegram.org")

	// AIBotK
	v.AIBOTK_KEY = os.Getenv("AIBOTK_KEY")
	v.AIBOTK_TYPE = os.Getenv("AIBOTK_TYPE")
	v.AIBOTK_NAME = os.Getenv("AIBOTK_NAME")

	// PushMe
	v.PUSHME_KEY = os.Getenv("PUSHME_KEY")

	// Webhook
	v.WEBHOOK_URL = os.Getenv("WEBHOOK_URL")
	v.WEBHOOK_BODY = os.Getenv("WEBHOOK_BODY")
	v.WEBHOOK_HEADERS = os.Getenv("WEBHOOK_HEADERS")
	v.WEBHOOK_METHOD = os.Getenv("WEBHOOK_METHOD")
	v.WEBHOOK_CONTENT_TYPE = envOrDefault("WEBHOOK_CONTENT_TYPE", "application/json")

	// Ntfy
	v.NTFY_URL = os.Getenv("NTFY_URL")
	v.NTFY_TOPIC = os.Getenv("NTFY_TOPIC")
	v.NTFY_PRIORITY = envOrDefault("NTFY_PRIORITY", "3")
	v.NTFY_TOKEN = os.Getenv("NTFY_TOKEN")
	v.NTFY_USERNAME = os.Getenv("NTFY_USERNAME")
	v.NTFY_PASSWORD = os.Getenv("NTFY_PASSWORD")
	v.NTFY_ACTIONS = os.Getenv("NTFY_ACTIONS")

	// WxPusher
	v.WXPUSHER_APP_TOKEN = os.Getenv("WXPUSHER_APP_TOKEN")
	v.WXPUSHER_TOPIC_IDS = os.Getenv("WXPUSHER_TOPIC_IDS")
	v.WXPUSHER_UIDS = os.Getenv("WXPUSHER_UIDS")

	// QQ Bot
	v.QQ_APP_ID = os.Getenv("QQ_APP_ID")
	v.QQ_APP_SECRET = os.Getenv("QQ_APP_SECRET")
	v.QQ_OPENID = os.Getenv("QQ_OPENID")
	v.QQ_GROUP_OPENID = os.Getenv("QQ_GROUP_OPENID")

	// WeChat ClawBot
	v.WECHAT_CLAWBOT_BOT_TOKEN = os.Getenv("WECHAT_CLAWBOT_BOT_TOKEN")
	v.WECHAT_CLAWBOT_BASE_URL = envOrDefault("WECHAT_CLAWBOT_BASE_URL", "https://ilinkai.weixin.qq.com")
	v.WECHAT_CLAWBOT_TO_USER = os.Getenv("WECHAT_CLAWBOT_TO_USER")
	v.WECHAT_CLAWBOT_ACCOUNT_ID = os.Getenv("WECHAT_CLAWBOT_ACCOUNT_ID")
}

func envOrDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

// GetConfig returns a channel configuration value by key name.
func GetConfig(key string) string {
	cfg := &pushConfig
	switch key {
	case "BARK_PUSH":
		return cfg.BARK_PUSH
	case "BARK_ARCHIVE":
		return cfg.BARK_ARCHIVE
	case "BARK_GROUP":
		return cfg.BARK_GROUP
	case "BARK_SOUND":
		return cfg.BARK_SOUND
	case "BARK_ICON":
		return cfg.BARK_ICON
	case "BARK_LEVEL":
		return cfg.BARK_LEVEL
	case "BARK_URL":
		return cfg.BARK_URL
	case "DD_BOT_SECRET":
		return cfg.DD_BOT_SECRET
	case "DD_BOT_TOKEN":
		return cfg.DD_BOT_TOKEN
	case "FSKEY":
		return cfg.FSKEY
	case "FSSECRET":
		return cfg.FSSECRET
	case "FEISHU_APP_ID":
		return cfg.FEISHU_APP_ID
	case "FEISHU_APP_SECRET":
		return cfg.FEISHU_APP_SECRET
	case "FEISHU_USER_ID":
		return cfg.FEISHU_USER_ID
	case "GOTIFY_URL":
		return cfg.GOTIFY_URL
	case "GOTIFY_TOKEN":
		return cfg.GOTIFY_TOKEN
	case "IGOT_PUSH_KEY":
		return cfg.IGOT_PUSH_KEY
	case "PUSH_KEY":
		return cfg.PUSH_KEY
	case "DEER_KEY":
		return cfg.DEER_KEY
	case "DEER_URL":
		return cfg.DEER_URL
	case "CHAT_URL":
		return cfg.CHAT_URL
	case "CHAT_TOKEN":
		return cfg.CHAT_TOKEN
	case "PUSH_PLUS_TOKEN":
		return cfg.PUSH_PLUS_TOKEN
	case "PUSH_PLUS_USER":
		return cfg.PUSH_PLUS_USER
	case "PUSH_PLUS_TEMPLATE":
		return cfg.PUSH_PLUS_TEMPLATE
	case "PUSH_PLUS_CHANNEL":
		return cfg.PUSH_PLUS_CHANNEL
	case "PUSH_PLUS_WEBHOOK":
		return cfg.PUSH_PLUS_WEBHOOK
	case "PUSH_PLUS_CALLBACKURL":
		return cfg.PUSH_PLUS_CALLBACKURL
	case "PUSH_PLUS_TO":
		return cfg.PUSH_PLUS_TO
	case "WE_PLUS_BOT_TOKEN":
		return cfg.WE_PLUS_BOT_TOKEN
	case "WE_PLUS_BOT_RECEIVER":
		return cfg.WE_PLUS_BOT_RECEIVER
	case "QMSG_KEY":
		return cfg.QMSG_KEY
	case "QMSG_TYPE":
		return cfg.QMSG_TYPE
	case "QYWX_ORIGIN":
		return cfg.QYWX_ORIGIN
	case "QYWX_AM":
		return cfg.QYWX_AM
	case "QYWX_KEY":
		return cfg.QYWX_KEY
	case "WECHAT_BOT_ID":
		return cfg.WECHAT_BOT_ID
	case "WECHAT_BOT_SECRET":
		return cfg.WECHAT_BOT_SECRET
	case "WECHAT_BOT_CHAT_ID":
		return cfg.WECHAT_BOT_CHAT_ID
	case "TG_BOT_TOKEN":
		return cfg.TG_BOT_TOKEN
	case "TG_USER_ID":
		return cfg.TG_USER_ID
	case "TG_API_HOST":
		return cfg.TG_API_HOST
	case "AIBOTK_KEY":
		return cfg.AIBOTK_KEY
	case "AIBOTK_TYPE":
		return cfg.AIBOTK_TYPE
	case "AIBOTK_NAME":
		return cfg.AIBOTK_NAME
	case "PUSHME_KEY":
		return cfg.PUSHME_KEY
	case "WEBHOOK_URL":
		return cfg.WEBHOOK_URL
	case "WEBHOOK_BODY":
		return cfg.WEBHOOK_BODY
	case "WEBHOOK_HEADERS":
		return cfg.WEBHOOK_HEADERS
	case "WEBHOOK_METHOD":
		return cfg.WEBHOOK_METHOD
	case "WEBHOOK_CONTENT_TYPE":
		return cfg.WEBHOOK_CONTENT_TYPE
	case "NTFY_URL":
		return cfg.NTFY_URL
	case "NTFY_TOPIC":
		return cfg.NTFY_TOPIC
	case "NTFY_PRIORITY":
		return cfg.NTFY_PRIORITY
	case "NTFY_TOKEN":
		return cfg.NTFY_TOKEN
	case "NTFY_USERNAME":
		return cfg.NTFY_USERNAME
	case "NTFY_PASSWORD":
		return cfg.NTFY_PASSWORD
	case "NTFY_ACTIONS":
		return cfg.NTFY_ACTIONS
	case "WXPUSHER_APP_TOKEN":
		return cfg.WXPUSHER_APP_TOKEN
	case "WXPUSHER_TOPIC_IDS":
		return cfg.WXPUSHER_TOPIC_IDS
	case "WXPUSHER_UIDS":
		return cfg.WXPUSHER_UIDS
	case "QQ_APP_ID":
		return cfg.QQ_APP_ID
	case "QQ_APP_SECRET":
		return cfg.QQ_APP_SECRET
	case "QQ_OPENID":
		return cfg.QQ_OPENID
	case "QQ_GROUP_OPENID":
		return cfg.QQ_GROUP_OPENID
	case "WECHAT_CLAWBOT_BOT_TOKEN":
		return cfg.WECHAT_CLAWBOT_BOT_TOKEN
	case "WECHAT_CLAWBOT_BASE_URL":
		return cfg.WECHAT_CLAWBOT_BASE_URL
	case "WECHAT_CLAWBOT_TO_USER":
		return cfg.WECHAT_CLAWBOT_TO_USER
	case "WECHAT_CLAWBOT_ACCOUNT_ID":
		return cfg.WECHAT_CLAWBOT_ACCOUNT_ID
	}
	return ""
}

// HasConfig checks if all the given config keys have non-empty values.
func HasConfig(keys ...string) bool {
	for _, key := range keys {
		if GetConfig(key) == "" {
			return false
		}
	}
	return true
}

// SetConfig sets a channel configuration value at runtime.
func SetConfig(key, value string) {
	cfg := &pushConfig
	switch key {
	case "BARK_PUSH":
		cfg.BARK_PUSH = value
	case "DD_BOT_TOKEN":
		cfg.DD_BOT_TOKEN = value
	case "DINGTALK_TOKEN":
		cfg.DD_BOT_TOKEN = value
	case "DINGTALK_SECRET":
		cfg.DD_BOT_SECRET = value
	case "FSKEY":
		cfg.FSKEY = value
	case "FEISHU_APP_ID":
		cfg.FEISHU_APP_ID = value
	case "FEISHU_APP_SECRET":
		cfg.FEISHU_APP_SECRET = value
	case "QQ_APP_ID":
		cfg.QQ_APP_ID = value
	case "QQ_APP_SECRET":
		cfg.QQ_APP_SECRET = value
	case "QQ_OPENID":
		cfg.QQ_OPENID = value
	case "QQ_GROUP_OPENID":
		cfg.QQ_GROUP_OPENID = value
	case "WECHAT_CLAWBOT_BOT_TOKEN":
		cfg.WECHAT_CLAWBOT_BOT_TOKEN = value
	case "WECHAT_CLAWBOT_ACCOUNT_ID":
		cfg.WECHAT_CLAWBOT_ACCOUNT_ID = value
	default:
		// Use a generic approach for less common keys
	}
}

// SetConfigs bulk-sets channel configuration from a map of key-value pairs.
// Uses reflection to match keys against the ChannelConfig struct's JSON tags,
// supporting all registered channel config keys.
func SetConfigs(configs map[string]string) {
	if len(configs) == 0 {
		slog.Warn("SetConfigs called with empty configs")
		return
	}

	cfgVal := reflect.ValueOf(&pushConfig).Elem()
	cfgType := cfgVal.Type()

	for key, value := range configs {
		if value == "" {
			slog.Debug("SetConfigs skipping empty value", "key", key)
			continue
		}
		found := false
		for i := range cfgType.NumField() {
			field := cfgType.Field(i)
			jsonTag := field.Tag.Get("json")
			if jsonTag == "" {
				continue
			}
			// JSON tag format: "KEY,omitempty" or "KEY"
			tagName, _, _ := strings.Cut(jsonTag, ",")
			if tagName == key {
				f := cfgVal.Field(i)
				if f.CanSet() && f.Kind() == reflect.String {
					f.SetString(value)
					found = true
					slog.Debug("SetConfigs set via reflection", "key", key)
				}
				break
			}
		}
		if !found {
			slog.Debug("SetConfigs fallback to SetConfig", "key", key)
			// Fallback to per-key SetConfig for backward compatibility
			SetConfig(key, value)
		}
	}
}
