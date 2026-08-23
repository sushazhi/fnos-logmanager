package services

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	notifConfigVersion = "1.0.0"
	notifConfigFile    = "notification-config.json"
	notifHistoryFile   = "notification-history.json"
)

// NotificationChannelConfig represents a notification channel configuration.
type NotificationChannelConfig struct {
	ID      string                 `json:"id"`
	Name    string                 `json:"name"`
	Channel string                 `json:"channel"`
	Enabled bool                   `json:"enabled"`
	Config  map[string]interface{} `json:"config,omitempty"`
}

// NotificationRule represents a notification rule.
type NotificationRule struct {
	ID               string    `json:"id"`
	Name             string    `json:"name"`
	Status           string    `json:"status"` // "enabled" or "disabled"
	AppName          string    `json:"appName"`
	LogLevel         string    `json:"logLevel"`
	LogPaths         []string  `json:"logPaths,omitempty"`
	Keywords         []string  `json:"keywords,omitempty"`
	ExcludeKeywords  []string  `json:"excludeKeywords,omitempty"`
	Pattern          string    `json:"pattern,omitempty"`
	Channels         []string  `json:"channels"`
	Cooldown         int       `json:"cooldown"`         // seconds
	MaxNotifications int       `json:"maxNotifications"` // per hour
	QuietHoursStart  string    `json:"quietHoursStart,omitempty"`
	QuietHoursEnd    string    `json:"quietHoursEnd,omitempty"`
	IPWhitelist      []string  `json:"ipWhitelist,omitempty"`      // CIDR/IP whitelist: login events from these IPs skip notification
	TriggerCount     int       `json:"triggerCount"`
	LastTriggeredAt  *time.Time `json:"lastTriggeredAt,omitempty"`
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
}

// NotificationHistory represents a notification history entry.
type NotificationHistory struct {
	ID        string    `json:"id"`
	RuleID    string    `json:"ruleId"`
	Channel   string    `json:"channel"`
	Title     string    `json:"title"`
	Message   string    `json:"message"`
	Success   bool      `json:"success"`
	Error     string    `json:"error,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// NotificationSettings represents notification system settings.
type NotificationSettings struct {
	Enabled         bool `json:"enabled"`
	CheckInterval   int  `json:"checkInterval"`
	MaxHistoryDays  int  `json:"maxHistoryDays"`
	MaxHistoryCount int  `json:"maxHistoryCount"`
}

// NotificationConfigFile is the top-level structure of the config file.
type NotificationConfigFile struct {
	Version  string                    `json:"version"`
	Channels []NotificationChannelConfig `json:"channels"`
	Rules    []NotificationRule        `json:"rules"`
	Settings NotificationSettings      `json:"settings"`
}

// NotificationStats holds aggregated notification statistics.
type NotificationStats struct {
	TotalSent      int                                         `json:"totalSent"`
	SuccessCount   int                                         `json:"successCount"`
	FailCount      int                                         `json:"failCount"`
	Last24Hours    int                                         `json:"last24Hours"`
	ByChannel      map[string]ChannelStat                     `json:"byChannel"`
	ByRule         map[string]ChannelStat                     `json:"byRule"`
}

// ChannelStat holds per-channel or per-rule statistics.
type ChannelStat struct {
	Sent    int `json:"sent"`
	Success int `json:"success"`
	Failed  int `json:"failed"`
}

// defaultNotificationConfig returns the default configuration.
func defaultNotificationConfig() NotificationConfigFile {
	return NotificationConfigFile{
		Version:  notifConfigVersion,
		Channels: []NotificationChannelConfig{},
		Rules:    []NotificationRule{},
		Settings: NotificationSettings{
			Enabled:         false,
			CheckInterval:   30000,
			MaxHistoryDays:  7,
			MaxHistoryCount: 1000,
		},
	}
}

// NotificationStore provides persistent storage for notification configuration.
type NotificationStore struct {
	mu             sync.RWMutex
	dataDir        string
	configFilePath string
	historyPath    string
	config         *NotificationConfigFile
	history        []NotificationHistory

	// Frequency control (in-memory)
	cooldowns   map[string]int64
	counters    map[string]*notifCounter
	countersMu  sync.Mutex
}

type notifCounter struct {
	Count     int
	ResetTime int64
}

// NewNotificationStore creates and initializes a notification store.
func NewNotificationStore(dataDir string) *NotificationStore {
	ns := &NotificationStore{
		dataDir:        dataDir,
		configFilePath: filepath.Join(dataDir, notifConfigFile),
		historyPath:    filepath.Join(dataDir, notifHistoryFile),
		cooldowns:      make(map[string]int64),
		counters:       make(map[string]*notifCounter),
	}
	ns.load()
	return ns
}

// knownConfigFields are the struct fields of NotificationChannelConfig,
// used to detect Node.js flat-format fields that should be migrated into Config.
var knownStructFields = map[string]bool{
	"id": true, "name": true, "channel": true, "enabled": true, "config": true,
}

// frontendToConfigKey maps Node.js camelCase flat field names to UPPER_CASE config keys.
// This is used to migrate Node.js-format JSON to Go format.
var migrateFrontendKey = map[string]string{
	"qqAppId":            "QQ_APP_ID",
	"qqAppSecret":        "QQ_APP_SECRET",
	"qqOpenId":           "QQ_OPENID",
	"qqGroupOpenId":      "QQ_GROUP_OPENID",
	"wechatClawBotToken": "WECHAT_CLAWBOT_BOT_TOKEN",
	"wechatClawBaseUrl":  "WECHAT_CLAWBOT_BASE_URL",
	"wechatClawToUser":   "WECHAT_CLAWBOT_TO_USER",
	"wechatClawAccountId":"WECHAT_CLAWBOT_ACCOUNT_ID",
	"barkPush":           "BARK_PUSH",
	"barkSound":          "BARK_SOUND",
	"barkGroup":          "BARK_GROUP",
	"barkIcon":           "BARK_ICON",
	"barkLevel":          "BARK_LEVEL",
	"barkArchive":        "BARK_ARCHIVE",
	"barkUrl":            "BARK_URL",
	"ddBotToken":         "DD_BOT_TOKEN",
	"ddBotSecret":        "DD_BOT_SECRET",
	"fskey":              "FSKEY",
	"fssecret":           "FSSECRET",
	"feishuAppId":        "FEISHU_APP_ID",
	"feishuAppSecret":    "FEISHU_APP_SECRET",
	"feishuUserId":       "FEISHU_USER_ID",
	"gotifyUrl":          "GOTIFY_URL",
	"gotifyToken":        "GOTIFY_TOKEN",
	"igotPushKey":        "IGOT_PUSH_KEY",
	"pushKey":            "PUSH_KEY",
	"deerKey":            "DEER_KEY",
	"deerUrl":            "DEER_URL",
	"chatUrl":            "CHAT_URL",
	"chatToken":          "CHAT_TOKEN",
	"pushPlusToken":      "PUSH_PLUS_TOKEN",
	"pushPlusUser":       "PUSH_PLUS_USER",
	"tgBotToken":         "TG_BOT_TOKEN",
	"tgUserId":           "TG_USER_ID",
	"tgApiHost":          "TG_API_HOST",
	"aibotkKey":          "AIBOTK_KEY",
	"aibotkType":         "AIBOTK_TYPE",
	"aibotkName":         "AIBOTK_NAME",
	"pushmeKey":          "PUSHME_KEY",
	"webhookUrl":         "WEBHOOK_URL",
	"webhookBody":        "WEBHOOK_BODY",
	"webhookHeaders":     "WEBHOOK_HEADERS",
	"webhookMethod":      "WEBHOOK_METHOD",
	"ntfyUrl":            "NTFY_URL",
	"ntfyTopic":          "NTFY_TOPIC",
	"ntfyToken":          "NTFY_TOKEN",
	"wxpusherAppToken":   "WXPUSHER_APP_TOKEN",
	"wxpusherTopicIds":   "WXPUSHER_TOPIC_IDS",
	"wxpusherUids":       "WXPUSHER_UIDS",
	"weplusBotToken":     "WE_PLUS_BOT_TOKEN",
	"weplusBotReceiver":  "WE_PLUS_BOT_RECEIVER",
	"qmsgKey":            "QMSG_KEY",
	"qmsgType":           "QMSG_TYPE",
	"qywxAm":             "QYWX_AM",
	"qywxKey":            "QYWX_KEY",
	"wechatBotId":        "WECHAT_BOT_ID",
	"wechatBotSecret":    "WECHAT_BOT_SECRET",
	"wechatBotChatId":    "WECHAT_BOT_CHAT_ID",
}

// load reads configuration and history from disk.
func (ns *NotificationStore) load() {
	// Ensure data directory exists
	os.MkdirAll(ns.dataDir, 0755)

	// Load config
	cfg := defaultNotificationConfig()
	if data, err := os.ReadFile(ns.configFilePath); err == nil {
		// Notification secrets are encrypted at rest. Try decrypt first; fall back
		// to plaintext so existing (unencrypted) deployments keep working.
		plaintext := data
		if dec, derr := decrypt(string(data)); derr == nil && dec != "" {
			plaintext = []byte(dec)
		}
		if err := json.Unmarshal(plaintext, &cfg); err != nil {
			slog.Warn("notification config parse error, using defaults", "error", err)
			cfg = defaultNotificationConfig()
		} else {
			// Migrate Node.js flat-format fields into Config map.
			// Node.js stores channel fields flat (e.g. "qqAppId": "xxx")
			// while Go nests them under "config": {"QQ_APP_ID": "xxx"}.
			var rawRoot map[string]interface{}
			if err := json.Unmarshal(data, &rawRoot); err == nil {
				if rawChannels, ok := rawRoot["channels"].([]interface{}); ok {
					for i, rawItem := range rawChannels {
						if i >= len(cfg.Channels) {
							break
						}
						rawCh, ok := rawItem.(map[string]interface{})
						if !ok {
							continue
						}
						hasFlatFields := false
						for key := range rawCh {
							if !knownStructFields[key] {
								hasFlatFields = true
								break
							}
						}
						if !hasFlatFields {
							continue
						}
						// Initialize Config map if needed
						if cfg.Channels[i].Config == nil {
							cfg.Channels[i].Config = make(map[string]interface{})
						}
						// Migrate flat fields
						for key, val := range rawCh {
							if knownStructFields[key] {
								continue
							}
							strVal, ok := val.(string)
							if !ok || strVal == "" {
								continue
							}
							// Map camelCase to UPPER_CASE
							configKey := key
							if mapped, ok := migrateFrontendKey[key]; ok {
								configKey = mapped
							}
							cfg.Channels[i].Config[configKey] = strVal
						}
						slog.Info("migrated Node.js flat fields to Config map",
							"channel", cfg.Channels[i].Name,
							"fields", len(cfg.Channels[i].Config))
					}
				}
			}
		}
	}
	ns.config = &cfg

	// Log qqbot channel credentials after load (debug)
	for _, ch := range cfg.Channels {
		if ch.Name == "qqbot" || ch.Channel == "qqbot" {
			if ch.Config != nil {
				hasId := ch.Config["QQ_APP_ID"] != nil && ch.Config["QQ_APP_ID"].(string) != ""
				hasSecret := ch.Config["QQ_APP_SECRET"] != nil && ch.Config["QQ_APP_SECRET"].(string) != ""
				slog.Info("QQ Bot通道加载完成", "hasAppId", hasId, "hasSecret", hasSecret, "configKeys", fmtKeys(ch.Config))
			} else {
				slog.Warn("QQ Bot通道加载后 Config 为空")
			}
		}
	}

	// Load history
	if data, err := os.ReadFile(ns.historyPath); err == nil {
		var hist []NotificationHistory
		if err := json.Unmarshal(data, &hist); err == nil {
			ns.history = hist
			return
		}
	}
	ns.history = []NotificationHistory{}
}

// saveConfig writes the current configuration to disk.
func (ns *NotificationStore) saveConfig() error {
	if ns.config == nil {
		return fmt.Errorf("config not loaded")
	}
	data, err := json.MarshalIndent(ns.config, "", "  ")
	if err != nil {
		return err
	}
	// Encrypt notification secrets at rest (AES-GCM via the shared session key).
	// If encryption is unavailable, fall back to plaintext rather than failing.
	if enc, eerr := encrypt(string(data)); eerr == nil && enc != "" {
		data = []byte(enc)
	}
	return os.WriteFile(ns.configFilePath, data, 0600)
}

// saveHistory writes the current history to disk. History may embed excerpts
// of log content (which can contain sensitive data), so it is written with
// owner-only permissions rather than world-readable 0644.
func (ns *NotificationStore) saveHistory() error {
	data, err := json.MarshalIndent(ns.history, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(ns.historyPath, data, 0600)
}

// ==================== Config Accessors ====================

// GetConfig returns the full notification configuration.
func (ns *NotificationStore) GetConfig() NotificationConfigFile {
	ns.mu.RLock()
	defer ns.mu.RUnlock()
	if ns.config == nil {
		return defaultNotificationConfig()
	}
	return *ns.config
}

// GetSettings returns the notification settings.
func (ns *NotificationStore) GetSettings() NotificationSettings {
	ns.mu.RLock()
	defer ns.mu.RUnlock()
	if ns.config == nil {
		return defaultNotificationConfig().Settings
	}
	return ns.config.Settings
}

// UpdateSettings updates notification settings.
func (ns *NotificationStore) UpdateSettings(s NotificationSettings) error {
	ns.mu.Lock()
	defer ns.mu.Unlock()
	if ns.config == nil {
		ns.config = new(NotificationConfigFile)
		*ns.config = defaultNotificationConfig()
	}
	ns.config.Settings = s
	return ns.saveConfig()
}

// ==================== Channel Management ====================

// GetChannels returns all notification channels.
func (ns *NotificationStore) GetChannels() []NotificationChannelConfig {
	ns.mu.RLock()
	defer ns.mu.RUnlock()
	if ns.config == nil {
		return []NotificationChannelConfig{}
	}
	result := make([]NotificationChannelConfig, len(ns.config.Channels))
	copy(result, ns.config.Channels)
	return result
}

// GetChannel returns a channel by name or channel type (Node.js compatible).
// NOTE: the returned pointer is a detached copy — mutating it does NOT persist.
// Callers must use UpdateChannel to apply changes.
func (ns *NotificationStore) GetChannel(name string) *NotificationChannelConfig {
	ns.mu.RLock()
	defer ns.mu.RUnlock()
	if ns.config == nil {
		return nil
	}
	for _, ch := range ns.config.Channels {
		if ch.Name == name || ch.Channel == name {
			cp := ch
			return &cp
		}
	}
	return nil
}

// AddChannel adds a new notification channel.
func (ns *NotificationStore) AddChannel(ch NotificationChannelConfig) error {
	ns.mu.Lock()
	defer ns.mu.Unlock()
	if ns.config == nil {
		ns.config = new(NotificationConfigFile)
		*ns.config = defaultNotificationConfig()
	}
	for _, c := range ns.config.Channels {
		if c.Name == ch.Name {
			return fmt.Errorf("渠道名称已存在")
		}
	}
	ns.config.Channels = append(ns.config.Channels, ch)
	return ns.saveConfig()
}

// UpdateChannel updates an existing notification channel.
func (ns *NotificationStore) UpdateChannel(name string, updates map[string]interface{}) error {
	ns.mu.Lock()
	defer ns.mu.Unlock()
	if ns.config == nil {
		return fmt.Errorf("存储未初始化")
	}
	for i, ch := range ns.config.Channels {
		if ch.Name == name {
			// Apply updates
			if v, ok := updates["name"]; ok {
				if s, ok2 := v.(string); ok2 {
					ns.config.Channels[i].Name = s
				}
			}
			if v, ok := updates["enabled"]; ok {
				if b, ok2 := v.(bool); ok2 {
					ns.config.Channels[i].Enabled = b
				}
			}
			if v, ok := updates["config"]; ok {
				if m, ok2 := v.(map[string]interface{}); ok2 {
					if ns.config.Channels[i].Config == nil {
						ns.config.Channels[i].Config = make(map[string]interface{})
					}
					const redactedSentinel = "••••••••"
					for k, v := range m {
						// Skip redacted sentinel values (frontend sends back "••••••••" for sensitive fields after redaction).
						// Otherwise every channel update would overwrite real secrets with the placeholder.
						if s, isStr := v.(string); isStr && s == redactedSentinel {
							continue
						}
						ns.config.Channels[i].Config[k] = v
					}
				}
			}
			return ns.saveConfig()
		}
	}
	return fmt.Errorf("渠道不存在")
}

// DeleteChannel removes a notification channel by name.
func (ns *NotificationStore) DeleteChannel(name string) error {
	ns.mu.Lock()
	defer ns.mu.Unlock()
	if ns.config == nil {
		return fmt.Errorf("存储未初始化")
	}
	for i, ch := range ns.config.Channels {
		if ch.Name == name {
			ns.config.Channels = append(ns.config.Channels[:i], ns.config.Channels[i+1:]...)
			return ns.saveConfig()
		}
	}
	return fmt.Errorf("渠道不存在")
}

// ==================== Rule Management ====================

// GetRules returns all notification rules.
func (ns *NotificationStore) GetRules() []NotificationRule {
	ns.mu.RLock()
	defer ns.mu.RUnlock()
	if ns.config == nil {
		return []NotificationRule{}
	}
	result := make([]NotificationRule, len(ns.config.Rules))
	copy(result, ns.config.Rules)
	return result
}

// GetEnabledRules returns all enabled rules.
func (ns *NotificationStore) GetEnabledRules() []NotificationRule {
	ns.mu.RLock()
	defer ns.mu.RUnlock()
	if ns.config == nil {
		return []NotificationRule{}
	}
	var result []NotificationRule
	for _, r := range ns.config.Rules {
		if r.Status == "enabled" {
			result = append(result, r)
		}
	}
	return result
}

// GetRule returns a rule by ID.
// NOTE: the returned pointer is a detached copy — mutating it does NOT persist.
// Callers must use UpdateRule to apply changes.
func (ns *NotificationStore) GetRule(id string) *NotificationRule {
	ns.mu.RLock()
	defer ns.mu.RUnlock()
	if ns.config == nil {
		return nil
	}
	for _, r := range ns.config.Rules {
		if r.ID == id {
			cp := r
			return &cp
		}
	}
	return nil
}

// AddRule adds a new notification rule.
func (ns *NotificationStore) AddRule(rule NotificationRule) error {
	ns.mu.Lock()
	defer ns.mu.Unlock()
	if ns.config == nil {
		ns.config = new(NotificationConfigFile)
		*ns.config = defaultNotificationConfig()
	}
	for _, r := range ns.config.Rules {
		if r.ID == rule.ID {
			return fmt.Errorf("规则ID已存在")
		}
	}
	ns.config.Rules = append(ns.config.Rules, rule)
	return ns.saveConfig()
}

// UpdateRule updates an existing notification rule.
func (ns *NotificationStore) UpdateRule(id string, rule NotificationRule) error {
	ns.mu.Lock()
	defer ns.mu.Unlock()
	if ns.config == nil {
		return fmt.Errorf("存储未初始化")
	}
	for i, r := range ns.config.Rules {
		if r.ID == id {
			rule.UpdatedAt = time.Now()
			ns.config.Rules[i] = rule
			return ns.saveConfig()
		}
	}
	return fmt.Errorf("规则不存在")
}

// DeleteRule removes a notification rule by ID.
func (ns *NotificationStore) DeleteRule(id string) error {
	ns.mu.Lock()
	defer ns.mu.Unlock()
	if ns.config == nil {
		return fmt.Errorf("存储未初始化")
	}
	for i, r := range ns.config.Rules {
		if r.ID == id {
			ns.config.Rules = append(ns.config.Rules[:i], ns.config.Rules[i+1:]...)
			return ns.saveConfig()
		}
	}
	return fmt.Errorf("规则不存在")
}

// UpdateRuleTrigger increments a rule's trigger count.
func (ns *NotificationStore) UpdateRuleTrigger(id string) error {
	ns.mu.Lock()
	defer ns.mu.Unlock()
	if ns.config == nil {
		return fmt.Errorf("存储未初始化")
	}
	for i, r := range ns.config.Rules {
		if r.ID == id {
			now := time.Now()
			ns.config.Rules[i].LastTriggeredAt = &now
			ns.config.Rules[i].TriggerCount++
			return ns.saveConfig()
		}
	}
	return fmt.Errorf("规则不存在")
}

// ==================== History Management ====================

// AddHistory adds a notification history record.
func (ns *NotificationStore) AddHistory(record NotificationHistory) error {
	ns.mu.Lock()
	defer ns.mu.Unlock()

	ns.history = append([]NotificationHistory{record}, ns.history...)

	// Trim to max count
	settings := defaultNotificationConfig().Settings
	if ns.config != nil {
		settings = ns.config.Settings
	}
	if len(ns.history) > settings.MaxHistoryCount {
		ns.history = ns.history[:settings.MaxHistoryCount]
	}

	return ns.saveHistory()
}

// GetHistory returns notification history with an optional limit.
func (ns *NotificationStore) GetHistory(limit int) []NotificationHistory {
	ns.mu.RLock()
	defer ns.mu.RUnlock()
	if limit <= 0 || limit > len(ns.history) {
		limit = len(ns.history)
	}
	result := make([]NotificationHistory, limit)
	copy(result, ns.history[:limit])
	return result
}

// ClearHistory clears all notification history.
func (ns *NotificationStore) ClearHistory() error {
	ns.mu.Lock()
	defer ns.mu.Unlock()
	ns.history = []NotificationHistory{}
	return ns.saveHistory()
}

// ==================== Statistics ====================

// GetStats returns aggregate notification statistics.
func (ns *NotificationStore) GetStats() NotificationStats {
	ns.mu.RLock()
	defer ns.mu.RUnlock()

	stats := NotificationStats{
		TotalSent:    len(ns.history),
		ByChannel:    make(map[string]ChannelStat),
		ByRule:       make(map[string]ChannelStat),
	}

	cutoff := time.Now().Add(-24 * time.Hour)

	for _, h := range ns.history {
		if h.Success {
			stats.SuccessCount++
		} else {
			stats.FailCount++
		}
		if h.Timestamp.After(cutoff) {
			stats.Last24Hours++
		}

		// By channel
		cs := stats.ByChannel[h.Channel]
		cs.Sent++
		if h.Success {
			cs.Success++
		} else {
			cs.Failed++
		}
		stats.ByChannel[h.Channel] = cs

		// By rule
		rs := stats.ByRule[h.RuleID]
		rs.Sent++
		if h.Success {
			rs.Success++
		} else {
			rs.Failed++
		}
		stats.ByRule[h.RuleID] = rs
	}

	return stats
}

// ==================== Frequency Control ====================

// CanSendNotification checks rate limits for a rule.
func (ns *NotificationStore) CanSendNotification(rule NotificationRule) bool {
	now := time.Now().UnixMilli()

	// Check cooldown
	ns.countersMu.Lock()
	defer ns.countersMu.Unlock()

	lastSent, exists := ns.cooldowns[rule.ID]
	if exists && (now-lastSent) < int64(rule.Cooldown)*1000 {
		return false
	}

	// Check hourly max
	counter, exists := ns.counters[rule.ID]
	if !exists || now > counter.ResetTime {
		ns.counters[rule.ID] = &notifCounter{
			Count:     0,
			ResetTime: now + 3600000,
		}
		counter = ns.counters[rule.ID]
	}
	if counter.Count >= rule.MaxNotifications {
		return false
	}

	return true
}

// RecordNotification records that a notification was sent.
func (ns *NotificationStore) RecordNotification(ruleID string) {
	now := time.Now().UnixMilli()

	ns.countersMu.Lock()
	defer ns.countersMu.Unlock()

	ns.cooldowns[ruleID] = now

	counter, exists := ns.counters[ruleID]
	if !exists || now > counter.ResetTime {
		ns.counters[ruleID] = &notifCounter{
			Count:     1,
			ResetTime: now + 3600000,
		}
		return
	}
	counter.Count++
}

// IsInQuietHours checks if the current time falls within a rule's quiet hours.
func IsInQuietHours(rule NotificationRule) bool {
	if rule.QuietHoursStart == "" || rule.QuietHoursEnd == "" {
		return false
	}

	now := time.Now()
	currentMinutes := now.Hour()*60 + now.Minute()

	parseTime := func(s string) (int, error) {
		var h, m int
		if _, err := fmt.Sscanf(s, "%d:%d", &h, &m); err != nil {
			return 0, err
		}
		return h*60 + m, nil
	}

	startMinutes, err := parseTime(rule.QuietHoursStart)
	if err != nil {
		return false
	}
	endMinutes, err := parseTime(rule.QuietHoursEnd)
	if err != nil {
		return false
	}

	if startMinutes > endMinutes {
		return currentMinutes >= startMinutes || currentMinutes < endMinutes
	}
	return currentMinutes >= startMinutes && currentMinutes < endMinutes
}

// fmtKeys returns comma-separated keys from a map for logging.
func fmtKeys(m map[string]interface{}) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return strings.Join(keys, ",")
}

// CleanupFrequencyCache removes expired rate limit entries.
func (ns *NotificationStore) CleanupFrequencyCache() {
	now := time.Now().UnixMilli()

	ns.countersMu.Lock()
	defer ns.countersMu.Unlock()

	for id, counter := range ns.counters {
		if now > counter.ResetTime {
			delete(ns.counters, id)
			delete(ns.cooldowns, id)
		}
	}
}
