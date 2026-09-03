package services

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/sushazhi/fnos-logmanager/internal/config"
	"github.com/sushazhi/fnos-logmanager/internal/types"
	"github.com/sushazhi/fnos-logmanager/internal/utils"
)

// CleanRule represents a scheduled cleanup rule.
type CleanRule struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// Enabled is a *bool so creation can distinguish "not sent" (nil, default
	// to enabled) from an explicit "false" (create the rule disabled).
	Enabled         *bool      `json:"enabled"`
	Schedule        string     `json:"schedule"`        // cron expression or seconds interval (e.g. "30s")
	LogDirs         []string   `json:"logDirs"`         // empty = all dirs
	FilePattern     string     `json:"filePattern"`     // regex to match filenames; empty = logs/archives only (see cleanNameMatcher)
	MinSizeBytes    int64      `json:"minSizeBytes"`    // 0 = no min
	MaxSizeBytes    int64      `json:"maxSizeBytes"`    // 0 = no max
	RetentionDays   int        `json:"retentionDays"`   // 0 = no date limit
	Action          string     `json:"action"`          // delete, truncate
	MaxFilesToClean int        `json:"maxFilesToClean"` // 0 = unlimited
	Description     string     `json:"description"`
	LastRun         *time.Time `json:"lastRun"`
	LastResult      string     `json:"lastResult"`
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

// cleanMatch is one candidate file collected during the scan phase of a clean
// rule run. It carries everything the execute phase needs; nothing is modified
// during the scan.
type cleanMatch struct {
	Path    string
	Size    int64
	ModTime time.Time
}

// cleanNameMatcher builds the filename matcher for a clean rule. An explicit
// filePattern regex is the user's own scope and is honored as-is. Without one,
// a rule would match EVERY file under the scan dirs — user data included
// (e.g. qBittorrent's BT_backup .torrent/.fastresume files under
// /vol1/@appdata) — so fall back to the same log/archive scope as
// CleanLogFiles: delete targets logs and archives, truncate only live logs
// (truncating an archive corrupts it while it stays removable via delete).
func cleanNameMatcher(filePattern, action string) (func(string) bool, error) {
	if filePattern != "" {
		re, err := regexp.Compile(filePattern)
		if err != nil {
			return nil, fmt.Errorf("无效的文件匹配模式: %s", err.Error())
		}
		return re.MatchString, nil
	}
	return func(name string) bool {
		if action == "truncate" {
			return isLogFile(name) && !isArchiveFile(name)
		}
		return isLogFile(name) || isArchiveFile(name)
	}, nil
}

// matchCleanFiles walks searchDirs (already normalized absolute paths) and
// collects files passing every filter WITHOUT touching them:
//   - matchName: filename matcher (see cleanNameMatcher)
//   - minSize / maxSize: byte bounds; 0 = unbounded on that side
//   - retentionDays: only files older than N days; 0 = no date filter
//
// Scan-only, so the collection phase and the destructive phase stay separable
// (and testable). Symlinks are never followed or collected, and known
// non-log directories (venv, node_modules, ...) are skipped like findFiles.
func matchCleanFiles(searchDirs []string, matchName func(string) bool, minSize, maxSize int64, retentionDays int) []cleanMatch {
	var cutoffTime time.Time
	if retentionDays > 0 {
		cutoffTime = time.Now().AddDate(0, 0, -retentionDays)
	}

	var matches []cleanMatch
	for _, dir := range searchDirs {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			continue
		}

		walkErr := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				if os.IsPermission(err) {
					return nil
				}
				return err
			}
			if info.IsDir() {
				if isIgnoredLogDir(info.Name()) {
					return filepath.SkipDir
				}
				return nil
			}
			// Never follow/act on symlinks: truncating or removing a link would
			// write/delete the link TARGET, which may live outside the scanned
			// directory (and outside the log dirs entirely).
			if info.Mode()&os.ModeSymlink != 0 {
				return nil
			}

			if !matchName(info.Name()) {
				return nil
			}
			if minSize > 0 && info.Size() < minSize {
				return nil
			}
			if maxSize > 0 && info.Size() > maxSize {
				return nil
			}
			if retentionDays > 0 && info.ModTime().After(cutoffTime) {
				return nil
			}

			matches = append(matches, cleanMatch{Path: path, Size: info.Size(), ModTime: info.ModTime()})
			if len(matches) >= findFilesHardCap {
				return filepath.SkipDir
			}
			return nil
		})
		if walkErr != nil {
			slog.Warn("clean rule scan aborted for dir", "dir", dir, "error", walkErr)
		}
	}

	return matches
}

// executeCleanMatches applies the rule action to collected matches. Reaching
// MaxFilesToClean is a successful bounded run (FIX bug 33): the cap only stops
// further processing and is never surfaced as an error.
func executeCleanMatches(matches []cleanMatch, action string, maxFiles int) types.CleanLogResult {
	result := types.CleanLogResult{}

	for _, m := range matches {
		var actionErr error
		switch action {
		case "delete":
			actionErr = os.Remove(m.Path)
		case "truncate":
			actionErr = os.WriteFile(m.Path, []byte{}, 0644)
		default:
			actionErr = fmt.Errorf("unknown action: %s", action)
		}

		if actionErr != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %s", m.Path, actionErr.Error()))
		} else {
			result.Cleaned++
		}

		if maxFiles > 0 && result.Cleaned >= maxFiles {
			break
		}
	}

	return result
}

func executeCleanRule(rule *CleanRule) (types.CleanLogResult, error) {
	// Scheduled uninstalled-app cleanup delegates to the one-shot
	// implementation: it needs the installed-apps check and has no
	// pattern/size/date dimensions of its own. Without this branch the rule
	// validated at creation time then failed every run with "unknown action".
	if rule.Action == "deleteUninstalled" {
		return CleanUninstalledLogs()
	}

	// Validate the pattern up-front so an invalid pattern aborts the run with
	// an error instead of silently matching nothing.
	matchName, err := cleanNameMatcher(rule.FilePattern, rule.Action)
	if err != nil {
		return types.CleanLogResult{}, err
	}

	searchDirs := rule.LogDirs
	if len(searchDirs) == 0 {
		searchDirs = config.Get().LogDirs
	}

	normalizedDirs := make([]string, 0, len(searchDirs))
	for _, dir := range searchDirs {
		normalizedDir := utils.SafePath(dir)
		if normalizedDir == "" {
			continue
		}
		normalizedDirs = append(normalizedDirs, normalizedDir)
	}

	if rule.Action == "cleanEmpty" {
		return cleanEmptyItems(normalizedDirs, matchName, rule.MaxFilesToClean), nil
	}

	matches := matchCleanFiles(normalizedDirs, matchName, rule.MinSizeBytes, rule.MaxSizeBytes, rule.RetentionDays)
	return executeCleanMatches(matches, rule.Action, rule.MaxFilesToClean), nil
}

// cleanEmptyItems implements the "cleanEmpty" action: it removes 0-byte files
// matching the rule's name matcher, then removes directories left empty that
// belong to uninstalled apps (deepest-first, so emptied parents cascade).
// Installed apps are never touched: their 0-byte files may be markers or locks
// of a running app and hold no space anyway. When the installed-app list
// cannot be determined, app-belonging files and all directories are skipped
// and the skip is reported; nameless paths (non-app dirs) are still swept.
func cleanEmptyItems(dirs []string, matchName func(string) bool, maxFiles int) types.CleanLogResult {
	result := types.CleanLogResult{}

	installedSet := make(map[string]bool)
	haveInstalled := true
	installedApps, err := getInstalledApps()
	if err != nil {
		haveInstalled = false
		result.Errors = append(result.Errors, "无法获取已安装应用列表，应用目录相关的空文件和空目录已跳过")
	} else {
		for _, app := range installedApps {
			installedSet[strings.ToLower(app)] = true
		}
	}

	for _, dir := range dirs {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			continue
		}
		walkErr := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				if os.IsPermission(err) {
					return nil
				}
				return err
			}
			if info.IsDir() {
				if isIgnoredLogDir(info.Name()) {
					return filepath.SkipDir
				}
				return nil
			}
			// Never follow/act on symlinks (see matchCleanFiles).
			if info.Mode()&os.ModeSymlink != 0 {
				return nil
			}
			if info.Size() != 0 || !matchName(info.Name()) {
				return nil
			}
			if appName := extractAppNameFromPath(path); appName != "" {
				if !haveInstalled || installedSet[strings.ToLower(appName)] {
					return nil
				}
			}
			if maxFiles > 0 && result.Cleaned >= maxFiles {
				return filepath.SkipAll
			}
			if err := os.Remove(path); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("%s: %s", path, err.Error()))
			} else {
				result.Cleaned++
			}
			return nil
		})
		if walkErr != nil {
			slog.Warn("clean-empty rule scan aborted for dir", "dir", dir, "error", walkErr)
		}
	}

	if maxFiles > 0 && result.Cleaned >= maxFiles {
		return result
	}
	if !haveInstalled {
		return result
	}

	removed, _ := removeEmptyUninstalledDirs(dirs, installedSet)
	result.Cleaned += removed

	return result
}

// CleanEmptyLogItems is the one-shot entry point for the cleanEmpty action
// (manual clean API and MCP): it removes 0-byte log/archive files under the
// configured log dirs (skipping installed apps' files) plus empty dirs of
// uninstalled apps.
func CleanEmptyLogItems() (types.CleanLogResult, error) {
	matchName, err := cleanNameMatcher("", "cleanEmpty")
	if err != nil {
		return types.CleanLogResult{}, err
	}
	dirs := make([]string, 0, len(config.Get().LogDirs))
	for _, dir := range config.Get().LogDirs {
		if normalized := utils.SafePath(dir); normalized != "" {
			dirs = append(dirs, normalized)
		}
	}
	return cleanEmptyItems(dirs, matchName, 0), nil
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
			live.LastResult = fmt.Sprintf("已清理 %d 项", result.Cleaned)
			if len(result.Reminded) > 0 {
				live.LastResult += fmt.Sprintf("，%d 个卸载残留目录待人工处理（已通知）", len(result.Reminded))
			}
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
