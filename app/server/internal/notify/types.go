package notify

// NotifyChannel defines the interface for a notification channel.
type NotifyChannel interface {
	// Name returns the channel name (e.g., "bark", "dingtalk").
	Name() string
	// Enabled returns whether the channel is enabled.
	Enabled() bool
	// SetEnabled enables or disables the channel.
	SetEnabled(enabled bool)
	// Send sends a notification with the given title and description.
	// Returns success status and an optional error message.
	Send(text, desp string) NotifyResult
}

// NotifyResult represents the result of a notification send attempt.
type NotifyResult struct {
	Success bool
	Message string
	Err     error
}

// NotifyResultJSON is the JSON-serializable form of NotifyResult.
type NotifyResultJSON struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}

// ToJSON converts a NotifyResult to its JSON form.
func (r NotifyResult) ToJSON() NotifyResultJSON {
	return NotifyResultJSON{
		Success: r.Success,
		Message: r.Message,
	}
}

// ChannelConfig holds all notification channel configuration values.
// Fields are loaded from environment variables.
type ChannelConfig struct {
	// Bark
	BARK_PUSH    string `json:"BARK_PUSH,omitempty"`
	BARK_ARCHIVE string `json:"BARK_ARCHIVE,omitempty"`
	BARK_GROUP   string `json:"BARK_GROUP,omitempty"`
	BARK_SOUND   string `json:"BARK_SOUND,omitempty"`
	BARK_ICON    string `json:"BARK_ICON,omitempty"`
	BARK_LEVEL   string `json:"BARK_LEVEL,omitempty"`
	BARK_URL     string `json:"BARK_URL,omitempty"`

	// DingTalk
	DD_BOT_SECRET string `json:"DD_BOT_SECRET,omitempty"`
	DD_BOT_TOKEN  string `json:"DD_BOT_TOKEN,omitempty"`

	// Feishu
	FSKEY            string `json:"FSKEY,omitempty"`
	FSSECRET         string `json:"FSSECRET,omitempty"`
	FEISHU_APP_ID    string `json:"FEISHU_APP_ID,omitempty"`
	FEISHU_APP_SECRET string `json:"FEISHU_APP_SECRET,omitempty"`
	FEISHU_USER_ID   string `json:"FEISHU_USER_ID,omitempty"`

	// Gotify
	GOTIFY_URL      string `json:"GOTIFY_URL,omitempty"`
	GOTIFY_TOKEN    string `json:"GOTIFY_TOKEN,omitempty"`
	GOTIFY_PRIORITY int    `json:"GOTIFY_PRIORITY,omitempty"`

	// iGot
	IGOT_PUSH_KEY string `json:"IGOT_PUSH_KEY,omitempty"`

	// ServerChan
	PUSH_KEY string `json:"PUSH_KEY,omitempty"`

	// PushDeer
	DEER_KEY string `json:"DEER_KEY,omitempty"`
	DEER_URL string `json:"DEER_URL,omitempty"`

	// Synology Chat
	CHAT_URL   string `json:"CHAT_URL,omitempty"`
	CHAT_TOKEN string `json:"CHAT_TOKEN,omitempty"`

	// PushPlus
	PUSH_PLUS_TOKEN      string `json:"PUSH_PLUS_TOKEN,omitempty"`
	PUSH_PLUS_USER       string `json:"PUSH_PLUS_USER,omitempty"`
	PUSH_PLUS_TEMPLATE   string `json:"PUSH_PLUS_TEMPLATE,omitempty"`
	PUSH_PLUS_CHANNEL    string `json:"PUSH_PLUS_CHANNEL,omitempty"`
	PUSH_PLUS_WEBHOOK    string `json:"PUSH_PLUS_WEBHOOK,omitempty"`
	PUSH_PLUS_CALLBACKURL string `json:"PUSH_PLUS_CALLBACKURL,omitempty"`
	PUSH_PLUS_TO         string `json:"PUSH_PLUS_TO,omitempty"`

	// WePlusBot
	WE_PLUS_BOT_TOKEN    string `json:"WE_PLUS_BOT_TOKEN,omitempty"`
	WE_PLUS_BOT_RECEIVER string `json:"WE_PLUS_BOT_RECEIVER,omitempty"`
	WE_PLUS_BOT_VERSION  string `json:"WE_PLUS_BOT_VERSION,omitempty"`

	// QMsg
	QMSG_KEY  string `json:"QMSG_KEY,omitempty"`
	QMSG_TYPE string `json:"QMSG_TYPE,omitempty"`

	// WeChat Work
	QYWX_ORIGIN string `json:"QYWX_ORIGIN,omitempty"`
	QYWX_AM     string `json:"QYWX_AM,omitempty"`
	QYWX_KEY    string `json:"QYWX_KEY,omitempty"`

	// WeChat Bot (Smart)
	WECHAT_BOT_ID      string `json:"WECHAT_BOT_ID,omitempty"`
	WECHAT_BOT_SECRET  string `json:"WECHAT_BOT_SECRET,omitempty"`
	WECHAT_BOT_CHAT_ID string `json:"WECHAT_BOT_CHAT_ID,omitempty"`
	WECHAT_BOT_WS_URL  string `json:"WECHAT_BOT_WS_URL,omitempty"`

	// Telegram
	TG_BOT_TOKEN string `json:"TG_BOT_TOKEN,omitempty"`
	TG_USER_ID   string `json:"TG_USER_ID,omitempty"`
	TG_API_HOST  string `json:"TG_API_HOST,omitempty"`

	// AIBotK
	AIBOTK_KEY  string `json:"AIBOTK_KEY,omitempty"`
	AIBOTK_TYPE string `json:"AIBOTK_TYPE,omitempty"`
	AIBOTK_NAME string `json:"AIBOTK_NAME,omitempty"`

	// PushMe
	PUSHME_KEY string `json:"PUSHME_KEY,omitempty"`

	// Webhook
	WEBHOOK_URL         string `json:"WEBHOOK_URL,omitempty"`
	WEBHOOK_BODY        string `json:"WEBHOOK_BODY,omitempty"`
	WEBHOOK_HEADERS     string `json:"WEBHOOK_HEADERS,omitempty"`
	WEBHOOK_METHOD      string `json:"WEBHOOK_METHOD,omitempty"`
	WEBHOOK_CONTENT_TYPE string `json:"WEBHOOK_CONTENT_TYPE,omitempty"`

	// Ntfy
	NTFY_URL      string `json:"NTFY_URL,omitempty"`
	NTFY_TOPIC    string `json:"NTFY_TOPIC,omitempty"`
	NTFY_PRIORITY string `json:"NTFY_PRIORITY,omitempty"`
	NTFY_TOKEN    string `json:"NTFY_TOKEN,omitempty"`
	NTFY_USERNAME string `json:"NTFY_USERNAME,omitempty"`
	NTFY_PASSWORD string `json:"NTFY_PASSWORD,omitempty"`
	NTFY_ACTIONS  string `json:"NTFY_ACTIONS,omitempty"`

	// WxPusher
	WXPUSHER_APP_TOKEN string `json:"WXPUSHER_APP_TOKEN,omitempty"`
	WXPUSHER_TOPIC_IDS string `json:"WXPUSHER_TOPIC_IDS,omitempty"`
	WXPUSHER_UIDS      string `json:"WXPUSHER_UIDS,omitempty"`

	// QQ Bot
	QQ_APP_ID        string `json:"QQ_APP_ID,omitempty"`
	QQ_APP_SECRET    string `json:"QQ_APP_SECRET,omitempty"`
	QQ_OPENID        string `json:"QQ_OPENID,omitempty"`
	QQ_GROUP_OPENID  string `json:"QQ_GROUP_OPENID,omitempty"`

	// WeChat ClawBot
	WECHAT_CLAWBOT_BOT_TOKEN  string `json:"WECHAT_CLAWBOT_BOT_TOKEN,omitempty"`
	WECHAT_CLAWBOT_BASE_URL   string `json:"WECHAT_CLAWBOT_BASE_URL,omitempty"`
	WECHAT_CLAWBOT_TO_USER    string `json:"WECHAT_CLAWBOT_TO_USER,omitempty"`
	WECHAT_CLAWBOT_ACCOUNT_ID string `json:"WECHAT_CLAWBOT_ACCOUNT_ID,omitempty"`
}
