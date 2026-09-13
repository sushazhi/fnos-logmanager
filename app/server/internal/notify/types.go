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

// Outcome 描述一次投递的终态。
//
// 关键区别在于「确定失败」与「结果未知」：前者可以安全重试，后者因为
// 可能已经送达，重试会造成重复轰炸，必须由调用方决定而非自动重试。
type Outcome string

const (
	// OutcomeDelivered 渠道明确确认已送达。
	OutcomeDelivered Outcome = "delivered"
	// OutcomeFailed 渠道明确拒绝或可确定的失败，重试是安全的。
	OutcomeFailed Outcome = "failed"
	// OutcomeUnknown 超时或连接中断等无法判定是否送达的情况，禁止自动重试。
	OutcomeUnknown Outcome = "unknown"
)

// NotifyResult represents the result of a notification send attempt.
type NotifyResult struct {
	Success bool
	Message string
	Err     error
	// Outcome 是 Success 的细化语义；未显式设置时由 Success 推导。
	Outcome Outcome
}

// ResolveOutcome 返回调用方应使用的终态。
// 兼容尚未设置 Outcome 的既有渠道实现：按 Success 推导为 Delivered/Failed。
func (r NotifyResult) ResolveOutcome() Outcome {
	switch r.Outcome {
	case OutcomeDelivered, OutcomeFailed, OutcomeUnknown:
		return r.Outcome
	}
	if r.Success {
		return OutcomeDelivered
	}
	return OutcomeFailed
}

// Retryable 表示该结果是否可以安全地自动重试。
// 只有「明确失败」可以；「结果未知」重试会导致重复消息。
func (r NotifyResult) Retryable() bool {
	return r.ResolveOutcome() == OutcomeFailed
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

	// DingTalk App (企业内部应用，新版 API)
	DD_APP_KEY        string `json:"DD_APP_KEY,omitempty"`
	DD_APP_SECRET     string `json:"DD_APP_SECRET,omitempty"`
	DD_APP_ROBOT_CODE string `json:"DD_APP_ROBOT_CODE,omitempty"`
	DD_APP_USER_IDS   string `json:"DD_APP_USER_IDS,omitempty"`

	// Feishu
	FSKEY             string `json:"FSKEY,omitempty"`
	FSSECRET          string `json:"FSSECRET,omitempty"`
	FEISHU_APP_ID     string `json:"FEISHU_APP_ID,omitempty"`
	FEISHU_APP_SECRET string `json:"FEISHU_APP_SECRET,omitempty"`
	FEISHU_USER_ID    string `json:"FEISHU_USER_ID,omitempty"`

	// Gotify
	GOTIFY_URL      string `json:"GOTIFY_URL,omitempty"`
	GOTIFY_TOKEN    string `json:"GOTIFY_TOKEN,omitempty"`
	GOTIFY_PRIORITY int    `json:"GOTIFY_PRIORITY,omitempty"`

	// ServerChan
	PUSH_KEY string `json:"PUSH_KEY,omitempty"`

	// PushDeer
	DEER_KEY string `json:"DEER_KEY,omitempty"`
	DEER_URL string `json:"DEER_URL,omitempty"`

	// Synology Chat
	CHAT_URL   string `json:"CHAT_URL,omitempty"`
	CHAT_TOKEN string `json:"CHAT_TOKEN,omitempty"`

	// PushPlus
	PUSH_PLUS_TOKEN       string `json:"PUSH_PLUS_TOKEN,omitempty"`
	PUSH_PLUS_USER        string `json:"PUSH_PLUS_USER,omitempty"`
	PUSH_PLUS_TEMPLATE    string `json:"PUSH_PLUS_TEMPLATE,omitempty"`
	PUSH_PLUS_CHANNEL     string `json:"PUSH_PLUS_CHANNEL,omitempty"`
	PUSH_PLUS_WEBHOOK     string `json:"PUSH_PLUS_WEBHOOK,omitempty"`
	PUSH_PLUS_CALLBACKURL string `json:"PUSH_PLUS_CALLBACKURL,omitempty"`
	PUSH_PLUS_TO          string `json:"PUSH_PLUS_TO,omitempty"`

	// QMsg
	QMSG_KEY string `json:"QMSG_KEY,omitempty"`
	QMSG_QQ  string `json:"QMSG_QQ,omitempty"`

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
	WEBHOOK_URL          string `json:"WEBHOOK_URL,omitempty"`
	WEBHOOK_BODY         string `json:"WEBHOOK_BODY,omitempty"`
	WEBHOOK_HEADERS      string `json:"WEBHOOK_HEADERS,omitempty"`
	WEBHOOK_METHOD       string `json:"WEBHOOK_METHOD,omitempty"`
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
	QQ_APP_ID       string `json:"QQ_APP_ID,omitempty"`
	QQ_APP_SECRET   string `json:"QQ_APP_SECRET,omitempty"`
	QQ_OPENID       string `json:"QQ_OPENID,omitempty"`
	QQ_GROUP_OPENID string `json:"QQ_GROUP_OPENID,omitempty"`

	// WeChat ClawBot
	WECHAT_CLAWBOT_BOT_TOKEN  string `json:"WECHAT_CLAWBOT_BOT_TOKEN,omitempty"`
	WECHAT_CLAWBOT_BASE_URL   string `json:"WECHAT_CLAWBOT_BASE_URL,omitempty"`
	WECHAT_CLAWBOT_TO_USER    string `json:"WECHAT_CLAWBOT_TO_USER,omitempty"`
	WECHAT_CLAWBOT_ACCOUNT_ID string `json:"WECHAT_CLAWBOT_ACCOUNT_ID,omitempty"`
}
