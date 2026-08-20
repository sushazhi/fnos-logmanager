package services

import (
	"encoding/json"
	"errors"
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

// errMaxFilesReached is a sentinel used to stop directory traversal once a rule
// hits MaxFilesToClean. Reaching the cap is a successful (bounded) run, so the
// sentinel is never surfaced to callers as an error. FIX(bug 33).
var errMaxFilesReached = errors.New("reached max files limit")

// CleanRule represents a scheduled cleanup rule.
type CleanRule struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	// Enabled is a *bool so creation can distinguish "not sent" (nil, default
	// to enabled) from an explicit "false" (create the rule disabled).
	Enabled         *bool    `json:"enabled"`
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

// isEnabled reports whether the rule is enabled, treating a nil Enabled
// (field not sent / legacy data without the field) as enabled.
func (r *CleanRule) isEnabled() bool {
	return r.Enabled == nil || *r.Enabled
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
	if !validCleanSchedule(rule.Schedule) {
		return nil, fmt.Errorf("无效的调度表达式: %s", rule.Schedule)
	}
	if !utils.IsValidAction(rule.Action) {
		return nil, fmt.Errorf("无效的操作类型: %s", rule.Action)
	}

	rule.ID = fmt.Sprintf("clean_%d", time.Now().UnixNano())
	// Default a missing Enabled (clients that don't send the field) to true,
	// but honor an explicit "false" so a rule can be created disabled.
	if rule.Enabled == nil {
		v := true
		rule.Enabled = &v
	}
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
	// Numeric limits are synced unconditionally: the route layer builds the
	// update from the existing rule (so an absent field carries over the old
	// value) and uses pointer fields to distinguish "not sent" from an explicit
	// zero. Unconditional assignment lets a client clear a limit back to 0.
	rule.MinSizeBytes = update.MinSizeBytes
	rule.MaxSizeBytes = update.MaxSizeBytes
	rule.RetentionDays = update.RetentionDays
	if update.Action != "" {
		if !utils.IsValidAction(update.Action) {
			globalAutoClean.mu.Unlock()
			return nil, fmt.Errorf("无效的操作类型: %s", update.Action)
		}
		rule.Action = update.Action
	}
	rule.MaxFilesToClean = update.MaxFilesToClean
	if update.Description != "" {
		rule.Description = update.Description
	}
	// Only override Enabled when the client explicitly sent the field;
	// a nil Enabled (field omitted in the update) leaves the current state.
	if update.Enabled != nil {
		rule.Enabled = update.Enabled
	}
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
	rule.Enabled = &enabled
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
	var errList []string

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
			// Never follow/act on symlinks: truncating or removing a link would
			// write/delete the link TARGET, which may live outside the scanned
			// directory (and outside the log dirs entirely).
			if info.Mode()&os.ModeSymlink != 0 {
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
				errList = append(errList, fmt.Sprintf("%s: %s", path, actionErr.Error()))
			} else {
				cleanedCount++
			}

			// Limit check. FIX(bug 33): reaching the cap is a SUCCESSFUL run
			// (the rule did its job and stopped), not a failure. Use a sentinel
			// error only to stop the walk, and swallow it below so the run is
			// not recorded as "错误: reached max files limit".
			if rule.MaxFilesToClean > 0 && cleanedCount >= rule.MaxFilesToClean {
				return errMaxFilesReached
			}

			return nil
		})

		if walkErr != nil && errors.Is(walkErr, errMaxFilesReached) {
			// Stop scanning further dirs; this is not an error.
			break
		}
	}

	result.Cleaned = cleanedCount
	result.Errors = errList
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
		if !rule.isEnabled() {
			continue
		}

		// FIX(bug 18): capture a VALUE COPY (not the map-element pointer). The
		// old `rule := rule` kept pointing at the rule stored in ac.rules, which
		// UpdateCleanRule/ToggleCleanRule rewrite under ac.mu while the cron job
		// reads the same fields unlocked -> race detector report and possible
		// mixed old/new conditions. A copy read once here is immutable afterwards.
		// executeRuleAndRecord still persists LastRun/LastResult to the LIVE
		// rule in ac.rules (looked up by ID under the lock), so scheduled runs
		// update the persisted state instead of only a discarded copy.
		ruleCopy := *rule
		schedule := ruleCopy.Schedule

		// Check if it's a seconds-based schedule (e.g. "30s")
		if isSecondsSchedule(schedule) {
			interval := parseSecondsInterval(schedule)
			if interval > 0 {
				// Use cron's @every syntax
				schedule = fmt.Sprintf("@every %s", formatDuration(interval))
			}
		}

		_, err := ac.cronScheduler.AddFunc(schedule, func() {
			ac.executeRuleAndRecord(&ruleCopy)
		})
		if err != nil {
			slog.Error("failed to schedule clean rule",
				"ruleId", ruleCopy.ID, "schedule", ruleCopy.Schedule, "error", err)
		}
	}

	ac.cronScheduler.Start()
}

func isSecondsSchedule(s string) bool {
	return len(s) > 2 && s[len(s)-1] == 's'
}

// validCleanSchedule reports whether a rule schedule expression is valid.
// Accepts a cron expression (standard 5-field) or a seconds interval like
// "30s". Used to reject malformed expressions at rule creation time so a bad
// schedule is never silently accepted and never executed.
func validCleanSchedule(s string) bool {
	if s == "" {
		return false
	}
	if isSecondsSchedule(s) {
		return parseSecondsInterval(s) > 0
	}
	// robfig/cron v3 standard parser: minute hour dom month dow, plus
	// descriptors (@every/@hourly/...) which reschedule also accepts.
	_, err := cron.ParseStandard(s)
	return err == nil
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

	// Update the LIVE rule's last run (looked up by ID in the map). The rule
	// parameter may be a value copy captured by a cron closure, so writing to
	// it directly would never reach the persisted rule. Persist the result to
	// the rule stored in ac.rules under the lock so LastRun/LastResult survive
	// saveRules() and are visible to the frontend. If the rule was deleted in
	// the meantime, skip (history is still recorded).
	ac.mu.Lock()
	if live, ok := ac.rules[rule.ID]; ok {
		live.LastRun = &startTime
		if err != nil {
			live.LastResult = fmt.Sprintf("错误: %s", err.Error())
		} else {
			live.LastResult = fmt.Sprintf("已清理 %d 个文件", result.Cleaned)
		}
		live.UpdatedAt = startTime
	}

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
		if rule.isEnabled() {
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
