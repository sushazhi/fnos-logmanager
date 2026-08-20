package services

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/sushazhi/fnos-logmanager/internal/config"
	"github.com/sushazhi/fnos-logmanager/internal/utils"
)

// MonitorRule represents a log monitoring rule.
type MonitorRule struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Pattern     string   `json:"pattern"`
	LogPaths    []string `json:"logPaths"`
	NotifyType  string   `json:"notifyType"` // webhook, email, syslog
	NotifyURL   string   `json:"notifyUrl"`
	Enabled     bool     `json:"enabled"`
	CooldownSec int      `json:"cooldownSec"` // minimum seconds between notifications
	Description string   `json:"description"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// AlertEvent represents a triggered alert.
type AlertEvent struct {
	ID        string    `json:"id"`
	RuleID    string    `json:"ruleId"`
	RuleName  string    `json:"ruleName"`
	LogPath   string    `json:"logPath"`
	Line      int       `json:"line"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
	Notified  bool      `json:"notified"`
}

type MonitorState struct {
	mu            sync.RWMutex
	rules         map[string]*MonitorRule
	lastAlerted   map[string]time.Time // ruleID+logPath -> last alert time
	lastOffset    map[string]int64     // ruleID+logPath -> last read byte offset
	alertHistory  []AlertEvent
	running       bool
	stopCh        chan struct{}
	monitorFile   string
	rulesFile     string
	maxHistory    int
}

var globalMonitor *MonitorState

func InitMonitor(rulesDir string) error {
	if err := os.MkdirAll(rulesDir, 0755); err != nil {
		return err
	}

	globalMonitor = &MonitorState{
		rules:        make(map[string]*MonitorRule),
		lastAlerted:  make(map[string]time.Time),
		lastOffset:   make(map[string]int64),
		alertHistory: make([]AlertEvent, 0),
		stopCh:       make(chan struct{}),
		rulesFile:    filepath.Join(rulesDir, "monitor_rules.json"),
		monitorFile:  filepath.Join(rulesDir, "monitor_state.json"),
		maxHistory:   1000,
	}

	if err := globalMonitor.loadRules(); err != nil {
		slog.Warn("failed to load monitor rules", "error", err)
	}
	if err := globalMonitor.loadState(); err != nil {
		slog.Warn("failed to load monitor state", "error", err)
	}

	return nil
}

func (m *MonitorState) loadRules() error {
	data, err := os.ReadFile(m.rulesFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var rules []MonitorRule
	if err := json.Unmarshal(data, &rules); err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	for _, rule := range rules {
		m.rules[rule.ID] = &rule
	}
	return nil
}

func (m *MonitorState) saveRules() error {
	m.mu.RLock()
	rules := make([]MonitorRule, 0, len(m.rules))
	for _, rule := range m.rules {
		rules = append(rules, *rule)
	}
	m.mu.RUnlock()

	data, err := json.MarshalIndent(rules, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.rulesFile, data, 0644)
}

func (m *MonitorState) loadState() error {
	data, err := os.ReadFile(m.monitorFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var state struct {
		LastAlerted  map[string]time.Time `json:"lastAlerted"`
		LastOffset   map[string]int64     `json:"lastOffset"`
		AlertHistory []AlertEvent         `json:"alertHistory"`
	}
	if err := json.Unmarshal(data, &state); err != nil {
		return err
	}

	m.mu.Lock()
	m.lastAlerted = state.LastAlerted
	if state.LastOffset != nil {
		m.lastOffset = state.LastOffset
	}
	m.alertHistory = state.AlertHistory
	m.mu.Unlock()
	return nil
}

func (m *MonitorState) saveState() error {
	m.mu.RLock()
	state := struct {
		LastAlerted  map[string]time.Time `json:"lastAlerted"`
		LastOffset   map[string]int64     `json:"lastOffset"`
		AlertHistory []AlertEvent         `json:"alertHistory"`
	}{
		LastAlerted:  m.lastAlerted,
		LastOffset:   m.lastOffset,
		AlertHistory: m.alertHistory,
	}
	m.mu.RUnlock()

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.monitorFile, data, 0644)
}

// GetRules returns all monitor rules.
func GetMonitorRules() []MonitorRule {
	if globalMonitor == nil {
		return nil
	}
	globalMonitor.mu.RLock()
	defer globalMonitor.mu.RUnlock()

	rules := make([]MonitorRule, 0, len(globalMonitor.rules))
	for _, rule := range globalMonitor.rules {
		rules = append(rules, *rule)
	}
	return rules
}

// GetRule returns a single monitor rule by ID.
func GetMonitorRule(id string) (*MonitorRule, error) {
	if globalMonitor == nil {
		return nil, fmt.Errorf("monitor not initialized")
	}
	globalMonitor.mu.RLock()
	defer globalMonitor.mu.RUnlock()

	rule, ok := globalMonitor.rules[id]
	if !ok {
		return nil, fmt.Errorf("规则不存在")
	}
	cp := *rule
	return &cp, nil
}

// CreateRule creates a new monitor rule.
func CreateMonitorRule(rule MonitorRule) (*MonitorRule, error) {
	if globalMonitor == nil {
		return nil, fmt.Errorf("monitor not initialized")
	}

	if rule.Name == "" {
		return nil, fmt.Errorf("规则名称不能为空")
	}
	if rule.Pattern == "" {
		return nil, fmt.Errorf("匹配模式不能为空")
	}
	if _, err := regexp.Compile(rule.Pattern); err != nil {
		return nil, fmt.Errorf("无效的正则表达式: %s", err.Error())
	}
	if len(rule.LogPaths) == 0 {
		return nil, fmt.Errorf("至少需要指定一个日志路径")
	}
	if rule.CooldownSec <= 0 {
		rule.CooldownSec = 60
	}

	rule.ID = fmt.Sprintf("rule_%d", time.Now().UnixNano())
	rule.Enabled = true
	rule.CreatedAt = time.Now()
	rule.UpdatedAt = time.Now()

	globalMonitor.mu.Lock()
	globalMonitor.rules[rule.ID] = &rule
	globalMonitor.mu.Unlock()

	if err := globalMonitor.saveRules(); err != nil {
		slog.Warn("failed to save monitor rules", "error", err)
	}

	return &rule, nil
}

// UpdateRule updates an existing monitor rule.
func UpdateMonitorRule(id string, update MonitorRule) (*MonitorRule, error) {
	if globalMonitor == nil {
		return nil, fmt.Errorf("monitor not initialized")
	}

	globalMonitor.mu.Lock()
	rule, ok := globalMonitor.rules[id]
	if !ok {
		globalMonitor.mu.Unlock()
		return nil, fmt.Errorf("规则不存在")
	}

	if update.Name != "" {
		rule.Name = update.Name
	}
	if update.Pattern != "" {
		if _, err := regexp.Compile(update.Pattern); err != nil {
			globalMonitor.mu.Unlock()
			return nil, fmt.Errorf("无效的正则表达式: %s", err.Error())
		}
		rule.Pattern = update.Pattern
	}
	if len(update.LogPaths) > 0 {
		rule.LogPaths = update.LogPaths
	}
	if update.NotifyType != "" {
		rule.NotifyType = update.NotifyType
	}
	if update.NotifyURL != "" {
		rule.NotifyURL = update.NotifyURL
	}
	if update.CooldownSec > 0 {
		rule.CooldownSec = update.CooldownSec
	}
	if update.Description != "" {
		rule.Description = update.Description
	}
	rule.Enabled = update.Enabled
	rule.UpdatedAt = time.Now()

	cp := *rule
	globalMonitor.mu.Unlock()

	if err := globalMonitor.saveRules(); err != nil {
		slog.Warn("failed to save monitor rules", "error", err)
	}

	return &cp, nil
}

// DeleteRule deletes a monitor rule.
func DeleteMonitorRule(id string) error {
	if globalMonitor == nil {
		return fmt.Errorf("monitor not initialized")
	}

	globalMonitor.mu.Lock()
	if _, ok := globalMonitor.rules[id]; !ok {
		globalMonitor.mu.Unlock()
		return fmt.Errorf("规则不存在")
	}
	delete(globalMonitor.rules, id)

	// Clean up lastAlerted and lastOffset entries for this rule
	for key := range globalMonitor.lastAlerted {
		if strings.HasPrefix(key, id+":") {
			delete(globalMonitor.lastAlerted, key)
		}
	}
	for key := range globalMonitor.lastOffset {
		if strings.HasPrefix(key, id+":") {
			delete(globalMonitor.lastOffset, key)
		}
	}
	globalMonitor.mu.Unlock()

	return globalMonitor.saveRules()
}

// ToggleRule enables or disables a rule.
func ToggleMonitorRule(id string, enabled bool) error {
	if globalMonitor == nil {
		return fmt.Errorf("monitor not initialized")
	}

	globalMonitor.mu.Lock()
	rule, ok := globalMonitor.rules[id]
	if !ok {
		globalMonitor.mu.Unlock()
		return fmt.Errorf("规则不存在")
	}
	rule.Enabled = enabled
	rule.UpdatedAt = time.Now()
	globalMonitor.mu.Unlock()

	return globalMonitor.saveRules()
}

// GetAlertHistory returns alert history.
func GetAlertHistory(limit int) []AlertEvent {
	if globalMonitor == nil {
		return nil
	}
	globalMonitor.mu.RLock()
	defer globalMonitor.mu.RUnlock()

	if limit <= 0 || limit > len(globalMonitor.alertHistory) {
		limit = len(globalMonitor.alertHistory)
	}

	events := make([]AlertEvent, limit)
	copy(events, globalMonitor.alertHistory[:limit])
	return events
}

// GetMonitorStatus returns the current monitor status.
func GetMonitorStatus() map[string]interface{} {
	if globalMonitor == nil {
		return map[string]interface{}{
			"initialized": false,
			"running":     false,
			"ruleCount":   0,
		}
	}

	globalMonitor.mu.RLock()
	defer globalMonitor.mu.RUnlock()

	return map[string]interface{}{
		"initialized": true,
		"running":     globalMonitor.running,
		"ruleCount":   len(globalMonitor.rules),
		"alertCount":  len(globalMonitor.alertHistory),
	}
}

// TriggerMonitorCheck triggers an immediate log monitoring scan.
func TriggerMonitorCheck() error {
	if globalMonitor == nil {
		return fmt.Errorf("monitor not initialized")
	}
	globalMonitor.mu.RLock()
	running := globalMonitor.running
	globalMonitor.mu.RUnlock()

	if !running {
		return fmt.Errorf("monitor not running")
	}

	go globalMonitor.scanFiles()
	slog.Debug("monitor check triggered")
	return nil
}

// StartMonitor starts the log monitoring loop.
func StartMonitor() error {
	if globalMonitor == nil {
		return fmt.Errorf("monitor not initialized")
	}

	globalMonitor.mu.Lock()
	if globalMonitor.running {
		globalMonitor.mu.Unlock()
		return fmt.Errorf("监控已经在运行")
	}
	globalMonitor.running = true
	globalMonitor.stopCh = make(chan struct{})
	globalMonitor.mu.Unlock()

	go globalMonitor.monitorLoop()
	slog.Info("log monitor started")
	return nil
}

// StopMonitor stops the log monitoring loop.
func StopMonitor() error {
	if globalMonitor == nil {
		return fmt.Errorf("monitor not initialized")
	}

	globalMonitor.mu.Lock()
	if !globalMonitor.running {
		globalMonitor.mu.Unlock()
		return fmt.Errorf("监控未在运行")
	}
	globalMonitor.running = false
	close(globalMonitor.stopCh)
	globalMonitor.mu.Unlock()

	return nil
}

func (m *MonitorState) monitorLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopCh:
			slog.Info("log monitor stopped")
			return
		case <-ticker.C:
			m.scanFiles()
		}
	}
}

func (m *MonitorState) scanFiles() {
	m.mu.RLock()
	rules := make([]*MonitorRule, 0, len(m.rules))
	for _, rule := range m.rules {
		if rule.Enabled {
			rules = append(rules, rule)
		}
	}
	m.mu.RUnlock()

	if len(rules) == 0 {
		return
	}

	for _, rule := range rules {
		m.scanRule(rule)
	}
}

func (m *MonitorState) scanRule(rule *MonitorRule) {
	re, err := regexp.Compile(rule.Pattern)
	if err != nil {
		slog.Error("invalid regex in rule", "ruleId", rule.ID, "pattern", rule.Pattern)
		return
	}

	for _, logPath := range rule.LogPaths {
		m.scanLogFile(rule, re, logPath)
	}
}

func (m *MonitorState) scanLogFile(rule *MonitorRule, re *regexp.Regexp, logPath string) {
	// Defense in depth: only monitor regular files inside the configured log
	// dirs, and never follow symlinks (a symlinked logPath could point at an
	// arbitrary file outside the allowed tree).
	normalized := utils.SafePath(logPath)
	if normalized == "" || !utils.IsAllowedPath(logPath, config.Get().LogDirs) {
		slog.Warn("log monitor rejected out-of-tree path", "path", logPath)
		return
	}
	if utils.IsSymlinkPath(normalized) {
		slog.Warn("log monitor rejected symlink path", "path", logPath)
		return
	}

	f, err := os.Open(normalized)
	if err != nil {
		slog.Warn("cannot open log file for monitoring", "path", logPath, "error", err)
		return
	}
	defer f.Close()

	key := rule.ID + ":" + logPath

	// Read the last known offset so we only scan NEW data, not the whole file
	// every 30s (which re-triggered every matching line each cycle). Reset to 0
	// when the file shrank (rotated/truncated) so a fresh rotation is scanned
	// from the top instead of seeking past EOF.
	info, err := f.Stat()
	if err != nil {
		slog.Warn("cannot stat log file for monitoring", "path", logPath, "error", err)
		return
	}
	m.mu.RLock()
	offset := m.lastOffset[key]
	m.mu.RUnlock()
	if offset > info.Size() {
		offset = 0
	}
	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		slog.Warn("cannot seek log file for monitoring", "path", logPath, "error", err)
		return
	}

	// Use bufio.Reader (not Scanner) so arbitrarily long lines are not capped
	// at 1 MiB and silently skipped.
	reader := bufio.NewReader(f)
	lineNum := 0
	for {
		line, err := reader.ReadString('\n')
		if len(line) > 0 {
			lineNum++
			// Strip trailing newline/CR to match the raw log line.
			trimmed := strings.TrimRight(line, "\n")
			trimmed = strings.TrimSuffix(trimmed, "\r")

			if re.MatchString(trimmed) {
				// Check cooldown
				m.mu.RLock()
				lastAlert, exists := m.lastAlerted[key]
				m.mu.RUnlock()

				if !(exists && time.Since(lastAlert).Seconds() < float64(rule.CooldownSec)) {
					event := AlertEvent{
						ID:        fmt.Sprintf("alert_%d_%d", time.Now().UnixNano(), lineNum),
						RuleID:    rule.ID,
						RuleName:  rule.Name,
						LogPath:   logPath,
						Line:      lineNum,
						Content:   trimmed,
						Timestamp: time.Now(),
						Notified:  false,
					}

					m.mu.Lock()
					m.lastAlerted[key] = time.Now()
					m.alertHistory = append([]AlertEvent{event}, m.alertHistory...)
					if len(m.alertHistory) > m.maxHistory {
						m.alertHistory = m.alertHistory[:m.maxHistory]
					}
					m.mu.Unlock()

					// Send notification
					if rule.NotifyType != "" && rule.NotifyURL != "" {
						go sendMonitorNotification(rule, event)
					}

					slog.Warn("log monitor alert triggered",
						"ruleId", rule.ID, "ruleName", rule.Name,
						"path", logPath, "line", lineNum,
					)
				}
			}
		}
		if err != nil {
			break
		}
	}

	// Persist the new read offset so the next scan continues from here.
	if newOffset, err := f.Seek(0, io.SeekCurrent); err == nil {
		m.mu.Lock()
		m.lastOffset[key] = newOffset
		m.mu.Unlock()
	}
}

func sendMonitorNotification(rule *MonitorRule, event AlertEvent) {
	payload := map[string]interface{}{
		"type":    "log_alert",
		"rule":    rule.Name,
		"path":    event.LogPath,
		"line":    event.Line,
		"content": event.Content,
		"time":    event.Timestamp.Format(time.RFC3339),
	}

	payloadBytes, _ := json.Marshal(payload)

	switch rule.NotifyType {
	case "webhook":
		notifyWebhook(string(payloadBytes), rule.NotifyURL)
	case "email":
		notifyEmail(string(payloadBytes))
	case "syslog":
		notifySyslog(string(payloadBytes))
	}

	// Mark notified
	globalMonitor.mu.Lock()
	for i := range globalMonitor.alertHistory {
		if globalMonitor.alertHistory[i].ID == event.ID {
			globalMonitor.alertHistory[i].Notified = true
			break
		}
	}
	globalMonitor.mu.Unlock()

	_ = globalMonitor.saveState()
}

// sendMonitorTest sends a test notification for a monitor rule.
func SendMonitorTest(ruleID string) error {
	if globalMonitor == nil {
		return fmt.Errorf("monitor not initialized")
	}

	globalMonitor.mu.RLock()
	rule, ok := globalMonitor.rules[ruleID]
	if !ok {
		globalMonitor.mu.RUnlock()
		return fmt.Errorf("规则不存在")
	}
	globalMonitor.mu.RUnlock()

	event := AlertEvent{
		ID:        fmt.Sprintf("test_%d", time.Now().UnixNano()),
		RuleID:    rule.ID,
		RuleName:  rule.Name,
		LogPath:   "(test)",
		Line:      0,
		Content:   "这是来自 fnos-logmanager 的测试通知消息",
		Timestamp: time.Now(),
	}

	go sendMonitorNotification(rule, event)
	return nil
}

// ClearAlertHistory clears all alert history.
func ClearAlertHistory() error {
	if globalMonitor == nil {
		return fmt.Errorf("monitor not initialized")
	}

	globalMonitor.mu.Lock()
	globalMonitor.alertHistory = make([]AlertEvent, 0)
	globalMonitor.mu.Unlock()

	return globalMonitor.saveState()
}

// notifyWebhook sends a payload to a webhook URL.
func notifyWebhook(payload, url string) {
	if url == "" {
		slog.Warn("webhook URL is empty, skipping notification")
		return
	}
	if utils.IsPrivateURL(url) {
		slog.Warn("webhook URL points to a private address, skipping notification", "url", url)
		return
	}
	resp, err := http.Post(url, "application/json", bytes.NewReader([]byte(payload)))
	if err != nil {
		slog.Warn("webhook notification failed", "error", err)
		return
	}
	defer resp.Body.Close()
	slog.Debug("webhook notification sent", "status", resp.StatusCode)
}


