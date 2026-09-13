package notify

import (
	"log/slog"
	"os"
	"reflect"
	"strconv"
	"strings"
	"sync"
)

// pushConfig holds the loaded channel configuration.
//
// This struct is rewritten at runtime by SetConfigs / RestoreConfig (for example
// when temporarily overriding config to test a single channel) while multiple
// notification goroutines read it concurrently, so every read and write must go
// through configMu to avoid a data race.
var (
	configMu   sync.RWMutex
	pushConfig ChannelConfig
)

// init loads channel config from environment variables at package init time.
func init() {
	loadConfigFromEnv()
}

// loadConfigFromEnv reads all ChannelConfig fields from environment variables.
func loadConfigFromEnv() {
	configMu.Lock()
	defer configMu.Unlock()

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

	// DingTalk App (企业内部应用)
	v.DD_APP_KEY = os.Getenv("DD_APP_KEY")
	v.DD_APP_SECRET = os.Getenv("DD_APP_SECRET")
	v.DD_APP_ROBOT_CODE = os.Getenv("DD_APP_ROBOT_CODE")
	v.DD_APP_USER_IDS = os.Getenv("DD_APP_USER_IDS")

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

	// QMsg
	v.QMSG_KEY = os.Getenv("QMSG_KEY")
	v.QMSG_QQ = os.Getenv("QMSG_QQ")

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
//
// 通过反射按 json tag 查找字段，而不是维护一份手写的 key->字段 映射表。
// 手写映射表曾导致新渠道字段遗漏（例如 GOTIFY_PRIORITY 定义了字段并加载了
// 环境变量，却因为在 switch 中漏了 case 而永远读不出来），因此这里改为与
// SetConfigs / SnapshotConfig 共用同一套 json tag 约定。
func GetConfig(key string) string {
	if key == "" {
		return ""
	}
	configMu.RLock()
	defer configMu.RUnlock()
	return configValueLocked(key)
}

// configValueLocked 在已持有读锁的前提下按 json tag 取值。
func configValueLocked(key string) string {
	cfgVal := reflect.ValueOf(&pushConfig).Elem()
	cfgType := cfgVal.Type()
	for i := 0; i < cfgType.NumField(); i++ {
		field := cfgType.Field(i)
		tagName, _, _ := strings.Cut(field.Tag.Get("json"), ",")
		if tagName != key {
			continue
		}
		f := cfgVal.Field(i)
		switch f.Kind() {
		case reflect.String:
			return f.String()
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			// 非字符串字段（如 GOTIFY_PRIORITY）只有非零值才有意义，
			// 返回 "0" 会让 HasConfig 误判为已配置。
			if v := f.Int(); v != 0 {
				return strconv.FormatInt(v, 10)
			}
			return ""
		default:
			return ""
		}
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
//
// 与 GetConfig 一样通过 json tag 定位字段，因此新增渠道字段无需再修改此函数。
// 兼容旧调用方传入的历史别名（例如 DINGTALK_TOKEN -> DD_BOT_TOKEN）。
func SetConfig(key, value string) {
	configMu.Lock()
	defer configMu.Unlock()
	setConfigLocked(key, value)
}

// configAliases 把历史/前端遗留的 key 归一为 ChannelConfig 的真实 json tag。
var configAliases = map[string]string{
	"DINGTALK_TOKEN":      "DD_BOT_TOKEN",
	"DINGTALK_SECRET":     "DD_BOT_SECRET",
	"FEISHU_WEBHOOK":      "FSKEY",
	"FEISHU_SECRET":       "FSSECRET",
	"SERVERCHAN_KEY":      "PUSH_KEY",
	"SERVERCHAN_URL":      "TG_API_HOST",
	"PUSHPLUS_TOKEN":      "PUSH_PLUS_TOKEN",
	"PUSHPLUS_TOPIC":      "PUSH_PLUS_TO",
	"PUSHDEER_KEY":        "DEER_KEY",
	"PUSHDEER_URL":        "DEER_URL",
	"WECOM_KEY":           "QYWX_KEY",
	"WECOM_QYDX_AGENT_ID": "QYWX_AM",
	"WECHAT_BOT_KEY":      "WECHAT_BOT_ID",
	"QMSG_TYPE":           "QMSG_QQ",
}

func setConfigLocked(key, value string) {
	if resolved, ok := configAliases[key]; ok {
		key = resolved
	}
	cfgVal := reflect.ValueOf(&pushConfig).Elem()
	cfgType := cfgVal.Type()
	for i := 0; i < cfgType.NumField(); i++ {
		field := cfgType.Field(i)
		tagName, _, _ := strings.Cut(field.Tag.Get("json"), ",")
		if tagName != key {
			continue
		}
		f := cfgVal.Field(i)
		if !f.CanSet() {
			return
		}
		switch f.Kind() {
		case reflect.String:
			f.SetString(value)
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			// 数值字段（如 GOTIFY_PRIORITY）：空值表示未配置，以便 GetConfig
			// 同样返回空串；否则 HasConfig 会把 0 误判为已配置。
			if value == "" {
				f.SetInt(0)
				return
			}
			n, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				slog.Warn("SetConfig received a non-numeric value for an integer field",
					"key", key, "value", value)
				return
			}
			f.SetInt(n)
		}
		return
	}
}

// SnapshotConfig returns a copy of the current global channel configuration.
// Used to temporarily override config for a single-channel send and then
// restore it, so a one-off (e.g. test) send never pollutes the config that all
// channels read.
func SnapshotConfig() map[string]string {
	configMu.RLock()
	defer configMu.RUnlock()

	cfgVal := reflect.ValueOf(&pushConfig).Elem()
	cfgType := cfgVal.Type()
	snap := make(map[string]string)
	for i := 0; i < cfgType.NumField(); i++ {
		field := cfgType.Field(i)
		jsonTag := field.Tag.Get("json")
		if jsonTag == "" {
			continue
		}
		tagName, _, _ := strings.Cut(jsonTag, ",")
		f := cfgVal.Field(i)
		if f.Kind() == reflect.String {
			if s := f.String(); s != "" {
				snap[tagName] = s
			}
		}
	}
	return snap
}

// RestoreConfig overwrites the current global channel configuration with the
// given snapshot. Pass the value previously returned by SnapshotConfig.
func RestoreConfig(snapshot map[string]string) {
	// A blank config means nothing was set; avoid wiping env-loaded defaults.
	if len(snapshot) == 0 {
		return
	}

	configMu.Lock()
	defer configMu.Unlock()

	cfgVal := reflect.ValueOf(&pushConfig).Elem()
	cfgType := cfgVal.Type()
	// First clear all string fields, then re-apply the snapshot values so keys
	// absent from the snapshot (that were only set temporarily) are removed.
	for i := 0; i < cfgType.NumField(); i++ {
		field := cfgType.Field(i)
		if field.Tag.Get("json") == "" {
			continue
		}
		f := cfgVal.Field(i)
		if f.CanSet() && f.Kind() == reflect.String {
			f.SetString("")
		}
	}
	for key, value := range snapshot {
		if value == "" {
			continue
		}
		setConfigLocked(key, value)
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

	configMu.Lock()
	defer configMu.Unlock()

	for key, value := range configs {
		if value == "" {
			slog.Debug("SetConfigs skipping empty value", "key", key)
			continue
		}
		setConfigLocked(key, value)
	}
}
