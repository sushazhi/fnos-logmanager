package services

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/sushazhi/fnos-logmanager/internal/config"
	"github.com/sushazhi/fnos-logmanager/internal/types"
	"github.com/sushazhi/fnos-logmanager/internal/utils"
)

// CleanRule represents a scheduled cleanup rule.
type CleanRule struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	Enabled         bool     `json:"enabled"`
	Schedule        string   `json:"schedule"`        // cron expression or seconds interval (e.g. "30s")
	LogDirs         []string `json:"logDirs"`          // empty = all dirs
	FilePattern     string   `json:"filePattern"`      // regex to match filenames
	MinSizeBytes    int64    `json:"minSizeBytes"`     // 0 = no min
	MaxSizeBytes    int64    `json:"maxSizeBytes"`     // 0 = no max
	RetentionDays   int      `json:"retentionDays"`    // 0 = no date limit
	Action          string   `json:"action"`           // delete, truncate
	MaxFilesToClean int      `json:"maxFilesToClean"`  // 0 = unlimited
	Description     string   `json:"description"`
	LastRun         *time.Time `json:"lastRun"`
	LastResult      string   `json:"lastResult"`
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`
}

// CleanRunHistory records a single execution of a clean rule.
type CleanRunHistory struct {
	RuleID     string    `json:"ruleId"`
	RuleName   string    `json:"ruleName"`
	StartedAt  time.Time `json:"startedAt"`
	FinishedAt time.Time `json:"finishedAt"`
	Cleaned    int       `json:"cleaned"`
	Errors     []string  `json:"errors"`
	Success    bool      `json:"success"`
}

type AutoCleanState struct {
	mu            sync.RWMutex
	rules         map[string]*CleanRule
	running       bool
	stopCh        chan struct{}
	cronScheduler *cron.Cron
	rulesFile     string
	historyFile   string
	history       []CleanRunHistory
	maxHistory    int
}

var globalAutoClean *AutoCleanState

func InitAutoClean(dataDir string) error {
	rulesDir := filepath.Join(dataDir, "config")
	if err := os.MkdirAll(rulesDir, 0755); err != nil {
		return err
	}

	globalAutoClean = &AutoCleanState{
		rules:       make(map[string]*CleanRule),
		stopCh:      make(chan struct{}),
		rulesFile:   filepath.Join(rulesDir, "autoclean_rules.json"),
		historyFile: filepath.Join(rulesDir, "autoclean_history.json"),
		maxHistory:  100,
	}

	if err := globalAutoClean.loadRules(); err != nil {
		slog.Warn("failed to load auto-clean rules", "error", err)
	}
	if err := globalAutoClean.loadHistory(); err != nil {
		slog.Warn("failed to load auto-clean history", "error", err)
	}

	return nil
}

func (ac *AutoCleanState) loadRules() error {
	data, err := os.ReadFile(ac.rulesFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var rules []CleanRule
	if err := json.Unmarshal(data, &rules); err != nil {
		return err
	}

	ac.mu.Lock()
	defer ac.mu.Unlock()
	for _, rule := range rules {
		ac.rules[rule.ID] = &rule
	}
	return nil
}

func (ac *AutoCleanState) saveRules() error {
	ac.mu.RLock()
	rules := make([]CleanRule, 0, len(ac.rules))
	for _, rule := range ac.rules {
		rules = append(rules, *rule)
	}
	ac.mu.RUnlock()

	data, err := json.MarshalIndent(rules, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(ac.rulesFile, data, 0644)
}

func (ac *AutoCleanState) loadHistory() error {
	data, err := os.ReadFile(ac.historyFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var history []CleanRunHistory
	if err := json.Unmarshal(data, &history); err != nil {
		return err
	}

	ac.mu.Lock()
	ac.history = history
	ac.mu.Unlock()
	return nil
}

func (ac *AutoCleanState) saveHistory() error {
	ac.mu.RLock()
	history := ac.history
	ac.mu.RUnlock()

	data, err := json.MarshalIndent(history, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(ac.historyFile, data, 0644)
}

// GetCleanRules returns all auto-clean rules.
func GetCleanRules() []CleanRule {
	if globalAutoClean == nil {
		return nil
	}
	globalAutoClean.mu.RLock()
	defer globalAutoClean.mu.RUnlock()

	rules := make([]CleanRule, 0, len(globalAutoClean.rules))
	for _, rule := range globalAutoClean.rules {
		rules = append(rules, *rule)
	}
	return rules
}

// GetCleanRule returns a single rule by ID.
func GetCleanRule(id string) (*CleanRule, error) {
	if globalAutoClean == nil {
		return nil, fmt.Errorf("auto-clean not initialized")
	}
	globalAutoClean.mu.RLock()
	defer globalAutoClean.mu.RUnlock()

	rule, ok := globalAutoClean.rules[id]
	if !ok {
		return nil, fmt.Errorf("规则不存在")
	}
	cp := *rule
	return &cp, nil
}

// CreateCleanRule creates a new auto-clean rule.
func CreateCleanRule(rule CleanRule) (*CleanRule, error) {
	if globalAutoClean == nil {
		return nil, fmt.Errorf("auto-clean not initialized")
	}

	if rule.Name == "" {
		return nil, fmt.Errorf("规则名称不能为空")
	}
	if rule.Schedule == "" {
		return nil, fmt.Errorf("调度表达式不能为空")
	}
	if !utils.IsValidAction(rule.Action) {
		return nil, fmt.Errorf("无效的操作类型: %s", rule.Action)
	}

	rule.ID = fmt.Sprintf("clean_%d", time.Now().UnixNano())
	rule.Enabled = true
	rule.CreatedAt = time.Now()
	rule.UpdatedAt = time.Now()

	globalAutoClean.mu.Lock()
	globalAutoClean.rules[rule.ID] = &rule
	globalAutoClean.mu.Unlock()

	if err := globalAutoClean.saveRules(); err != nil {
		slog.Warn("failed to save clean rules", "error", err)
	}

	globalAutoClean.reschedule()

	return &rule, nil
}

// UpdateCleanRule updates an existing auto-clean rule.
func UpdateCleanRule(id string, update CleanRule) (*CleanRule, error) {
	if globalAutoClean == nil {
		return nil, fmt.Errorf("auto-clean not initialized")
	}

	globalAutoClean.mu.Lock()
	rule, ok := globalAutoClean.rules[id]
	if !ok {
		globalAutoClean.mu.Unlock()
		return nil, fmt.Errorf("规则不存在")
	}

	if update.Name != "" {
		rule.Name = update.Name
	}
	if update.Schedule != "" {
		rule.Schedule = update.Schedule
	}
	if len(update.LogDirs) > 0 {
		rule.LogDirs = update.LogDirs
	}
	if update.FilePattern != "" {
		rule.FilePattern = update.FilePattern
	}
	if update.MinSizeBytes > 0 {
		rule.MinSizeBytes = update.MinSizeBytes
	}
	if update.MaxSizeBytes > 0 {
		rule.MaxSizeBytes = update.MaxSizeBytes
	}
	if update.RetentionDays > 0 {
		rule.RetentionDays = update.RetentionDays
	}
	if update.Action != "" {
		if !utils.IsValidAction(update.Action) {
			globalAutoClean.mu.Unlock()
			return nil, fmt.Errorf("无效的操作类型: %s", update.Action)
		}
		rule.Action = update.Action
	}
	if update.MaxFilesToClean > 0 {
		rule.MaxFilesToClean = update.MaxFilesToClean
	}
	if update.Description != "" {
		rule.Description = update.Description
	}
	rule.Enabled = update.Enabled
	rule.UpdatedAt = time.Now()

	cp := *rule
	globalAutoClean.mu.Unlock()

	if err := globalAutoClean.saveRules(); err != nil {
		slog.Warn("failed to save clean rules", "error", err)
	}

	globalAutoClean.reschedule()

	return &cp, nil
}

// DeleteCleanRule deletes an auto-clean rule.
func DeleteCleanRule(id string) error {
	if globalAutoClean == nil {
		return fmt.Errorf("auto-clean not initialized")
	}

	globalAutoClean.mu.Lock()
	if _, ok := globalAutoClean.rules[id]; !ok {
		globalAutoClean.mu.Unlock()
		return fmt.Errorf("规则不存在")
	}
	delete(globalAutoClean.rules, id)
	globalAutoClean.mu.Unlock()

	if err := globalAutoClean.saveRules(); err != nil {
		slog.Warn("failed to save clean rules", "error", err)
	}

	globalAutoClean.reschedule()

	return nil
}

// ToggleCleanRule enables or disables a rule.
func ToggleCleanRule(id string, enabled bool) error {
	if globalAutoClean == nil {
		return fmt.Errorf("auto-clean not initialized")
	}

	globalAutoClean.mu.Lock()
	rule, ok := globalAutoClean.rules[id]
	if !ok {
		globalAutoClean.mu.Unlock()
		return fmt.Errorf("规则不存在")
	}
	rule.Enabled = enabled
	rule.UpdatedAt = time.Now()
	globalAutoClean.mu.Unlock()

	if err := globalAutoClean.saveRules(); err != nil {
		slog.Warn("failed to save clean rules", "error", err)
	}

	globalAutoClean.reschedule()

	return nil
}

// GetCleanHistory returns clean run history.
func GetCleanHistory(limit int) []CleanRunHistory {
	if globalAutoClean == nil {
		return nil
	}
	globalAutoClean.mu.RLock()
	defer globalAutoClean.mu.RUnlock()

	if limit <= 0 || limit > len(globalAutoClean.history) {
		limit = len(globalAutoClean.history)
	}

	h := make([]CleanRunHistory, limit)
	copy(h, globalAutoClean.history[:limit])
	return h
}

// RunCleanRuleNow executes a clean rule immediately.
func RunCleanRuleNow(id string) (types.CleanLogResult, error) {
	if globalAutoClean == nil {
		return types.CleanLogResult{}, fmt.Errorf("auto-clean not initialized")
	}

	globalAutoClean.mu.RLock()
	rule, ok := globalAutoClean.rules[id]
	if !ok {
		globalAutoClean.mu.RUnlock()
		return types.CleanLogResult{}, fmt.Errorf("规则不存在")
	}
	ru := *rule // copy to release lock
	globalAutoClean.mu.RUnlock()

	return executeCleanRule(&ru)
}

func executeCleanRule(rule *CleanRule) (types.CleanLogResult, error) {
	result := types.CleanLogResult{}
	now := time.Now()

	searchDirs := rule.LogDirs
	if len(searchDirs) == 0 {
		searchDirs = config.Get().LogDirs
	}

	var nameRe *regexp.Regexp
	if rule.FilePattern != "" {
		var err error
		nameRe, err = regexp.Compile(rule.FilePattern)
		if err != nil {
			return result, fmt.Errorf("无效的文件匹配模式: %s", err.Error())
		}
	}

	var cutoffTime time.Time
	if rule.RetentionDays > 0 {
		cutoffTime = now.AddDate(0, 0, -rule.RetentionDays)
	}

	cleanedCount := 0
	var errors []string

	for _, dir := range searchDirs {
		normalizedDir := utils.SafePath(dir)
		if normalizedDir == "" {
			continue
		}
		if _, err := os.Stat(normalizedDir); os.IsNotExist(err) {
			continue
		}

		walkErr := filepath.Walk(normalizedDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				if os.IsPermission(err) {
					return nil
				}
				return err
			}
			if info.IsDir() {
				return nil
			}

			// Check file pattern
			if nameRe != nil && !nameRe.MatchString(info.Name()) {
				return nil
			}

			// Check size constraints
			if rule.MinSizeBytes > 0 && info.Size() < rule.MinSizeBytes {
				return nil
			}
			if rule.MaxSizeBytes > 0 && info.Size() > rule.MaxSizeBytes {
				return nil
			}

			// Check retention days
			if rule.RetentionDays > 0 && info.ModTime().After(cutoffTime) {
				return nil
			}

			// Perform action
			var actionErr error
			switch rule.Action {
			case "delete":
				actionErr = os.Remove(path)
			case "truncate":
				actionErr = os.WriteFile(path, []byte{}, 0644)
			default:
				actionErr = fmt.Errorf("unknown action: %s", rule.Action)
			}

			if actionErr != nil {
				errors = append(errors, fmt.Sprintf("%s: %s", path, actionErr.Error()))
			} else {
				cleanedCount++
			}

			// Limit check
			if rule.MaxFilesToClean > 0 && cleanedCount >= rule.MaxFilesToClean {
				return fmt.Errorf("reached max files limit")
			}

			return nil
		})

		if walkErr != nil && cleanedCount >= rule.MaxFilesToClean {
			break
		}
	}

	result.Cleaned = cleanedCount
	result.Errors = errors
	return result, nil
}

// StartAutoClean starts the auto-clean scheduler.
func StartAutoClean() error {
	if globalAutoClean == nil {
		return fmt.Errorf("auto-clean not initialized")
	}

	globalAutoClean.mu.Lock()
	if globalAutoClean.running {
		globalAutoClean.mu.Unlock()
		return fmt.Errorf("自动清理已在运行")
	}
	globalAutoClean.running = true
	globalAutoClean.mu.Unlock()

	globalAutoClean.reschedule()
	slog.Info("auto-clean scheduler started")
	return nil
}

// StopAutoClean stops the auto-clean scheduler.
func StopAutoClean() error {
	if globalAutoClean == nil {
		return fmt.Errorf("auto-clean not initialized")
	}

	globalAutoClean.mu.Lock()
	if !globalAutoClean.running {
		globalAutoClean.mu.Unlock()
		return fmt.Errorf("自动清理未在运行")
	}
	globalAutoClean.running = false
	if globalAutoClean.cronScheduler != nil {
		globalAutoClean.cronScheduler.Stop()
		globalAutoClean.cronScheduler = nil
	}
	globalAutoClean.mu.Unlock()

	slog.Info("auto-clean scheduler stopped")
	return nil
}

func (ac *AutoCleanState) reschedule() {
	ac.mu.Lock()
	defer ac.mu.Unlock()

	if !ac.running {
		return
	}

	// Stop existing scheduler
	if ac.cronScheduler != nil {
		ac.cronScheduler.Stop()
		ac.cronScheduler = nil
	}

	ac.cronScheduler = cron.New()

	for _, rule := range ac.rules {
		if !rule.Enabled {
			continue
		}

		rule := rule // capture for closure
		schedule := rule.Schedule

		// Check if it's a seconds-based schedule (e.g. "30s")
		if isSecondsSchedule(schedule) {
			interval := parseSecondsInterval(schedule)
			if interval > 0 {
				// Use cron's @every syntax
				schedule = fmt.Sprintf("@every %s", formatDuration(interval))
			}
		}

		_, err := ac.cronScheduler.AddFunc(schedule, func() {
			ac.executeRuleAndRecord(rule)
		})
		if err != nil {
			slog.Error("failed to schedule clean rule",
				"ruleId", rule.ID, "schedule", rule.Schedule, "error", err)
		}
	}

	ac.cronScheduler.Start()
}

func isSecondsSchedule(s string) bool {
	return len(s) > 2 && s[len(s)-1] == 's'
}

func parseSecondsInterval(s string) time.Duration {
	if len(s) < 2 {
		return 0
	}
	valStr := s[:len(s)-1]
	var secs int
	if _, err := fmt.Sscanf(valStr, "%d", &secs); err != nil {
		return 0
	}
	return time.Duration(secs) * time.Second
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	return d.String()
}

func (ac *AutoCleanState) executeRuleAndRecord(rule *CleanRule) {
	startTime := time.Now()
	slog.Info("executing clean rule", "ruleId", rule.ID, "name", rule.Name)

	result, err := executeCleanRule(rule)

	finishTime := time.Now()
	history := CleanRunHistory{
		RuleID:     rule.ID,
		RuleName:   rule.Name,
		StartedAt:  startTime,
		FinishedAt: finishTime,
		Cleaned:    result.Cleaned,
		Errors:     result.Errors,
		Success:    err == nil,
	}

	// Update rule's last run
	ac.mu.Lock()
	rule.LastRun = &startTime
	if err != nil {
		rule.LastResult = fmt.Sprintf("错误: %s", err.Error())
	} else {
		rule.LastResult = fmt.Sprintf("已清理 %d 个文件", result.Cleaned)
	}
	rule.UpdatedAt = startTime

	// Add to history
	ac.history = append([]CleanRunHistory{history}, ac.history...)
	if len(ac.history) > ac.maxHistory {
		ac.history = ac.history[:ac.maxHistory]
	}
	ac.mu.Unlock()

	if err := ac.saveRules(); err != nil {
		slog.Warn("failed to save rules after execution", "error", err)
	}
	if err := ac.saveHistory(); err != nil {
		slog.Warn("failed to save history after execution", "error", err)
	}

	slog.Info("clean rule executed",
		"ruleId", rule.ID, "name", rule.Name,
		"cleaned", result.Cleaned, "errors", len(result.Errors))
}

// RunAllCleanRulesNow executes all enabled clean rules immediately.
func RunAllCleanRulesNow() []CleanRunHistory {
	if globalAutoClean == nil {
		return nil
	}

	globalAutoClean.mu.RLock()
	rules := make([]*CleanRule, 0)
	for _, rule := range globalAutoClean.rules {
		if rule.Enabled {
			rules = append(rules, rule)
		}
	}
	globalAutoClean.mu.RUnlock()

	var results []CleanRunHistory
	for _, rule := range rules {
		globalAutoClean.executeRuleAndRecord(rule)
	}

	globalAutoClean.mu.RLock()
	results = make([]CleanRunHistory, len(globalAutoClean.history))
	copy(results, globalAutoClean.history)
	globalAutoClean.mu.RUnlock()

	return results
}
