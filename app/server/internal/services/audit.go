package services

import (
	"bufio"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sushazhi/fnos-logmanager/internal/config"
	"github.com/sushazhi/fnos-logmanager/internal/types"
	"github.com/sushazhi/fnos-logmanager/internal/utils"
)

var auditMu sync.Mutex

var actionNames = map[string]string{
	"SERVER_START":                  "服务器启动",
	"SERVER_SHUTDOWN":               "服务器关闭",
	"logout":                        "登出",
	"auth_failed":                   "认证失败",
	"csrf_failed":                   "CSRF验证失败",
	"log_truncate":                  "日志清空",
	"log_delete":                    "日志删除",
	"logs_clean":                    "日志批量清理",
	"logs_backup":                   "日志备份",
	"backup_delete":                 "备份删除",
	"backups_clean":                 "备份清理",
	"archive_delete":                "归档删除",
	"auto_clean":                    "自动清理执行",
	"auto_clean_rule_add":           "添加自动清理规则",
	"auto_clean_rule_update":        "更新自动清理规则",
	"auto_clean_rule_delete":        "删除自动清理规则",
	"auto_clean_rule_toggle":        "切换自动清理规则",
	"auto_clean_manual":             "手动执行自动清理",
	"log_export":                    "日志导出",
	"dirs_clean_empty":              "清理空目录",
	"dirs_clean_uninstalled_trash":  "清理已卸载应用残留目录",
	"dirs_clean_uninstalled_restore": "还原已卸载应用残留目录",
	"bookmark_add":                  "添加书签",
	"bookmark_delete":               "删除书签",
	"bookmark_update":               "更新书签",
	"process_kill":                  "结束进程",
	"PROCESS_KILL":                  "结束进程",
	"MCP_PROCESS_KILL":              "结束进程(MCP)",
	"PROCESS_LOG_READ":              "查看进程日志",
	"MCP_PROCESS_LOG_READ":          "查看进程日志(MCP)",
	"SECURITY_UNCAUGHT_EXCEPTION":   "安全异常-未捕获异常",
	"SECURITY_UNHANDLED_REJECTION":  "安全异常-未处理Promise",
	"SECURITY_SENSITIVE_INFO_SCAN":  "安全扫描-敏感信息",
	"SECURITY_APP_UPDATED":          "安全事件-应用更新",
	"SECURITY_UPDATE_FAILED":        "安全事件-更新失败",
}

func getActionName(action string) string {
	if name, exists := actionNames[action]; exists {
		return name
	}
	return action
}

func redactSensitiveDetails(details map[string]interface{}) map[string]interface{} {
	cfg := config.Get()
	patterns := make([]*regexp.Regexp, 0, len(cfg.SensitivePatterns))
	for _, p := range cfg.SensitivePatterns {
		if re, err := regexp.Compile(p); err == nil {
			patterns = append(patterns, re)
		}
	}

	result := make(map[string]interface{}, len(details))
	for key, value := range details {
		switch v := value.(type) {
		case string:
			sanitized := v
			for _, re := range patterns {
				sanitized = re.ReplaceAllString(sanitized, "[FILTERED]")
			}
			result[key] = sanitized
		case []interface{}:
			arr := make([]interface{}, len(v))
			for i, item := range v {
				switch s := item.(type) {
				case string:
					sanitized := s
					for _, re := range patterns {
						sanitized = re.ReplaceAllString(sanitized, "[FILTERED]")
					}
					arr[i] = sanitized
				case map[string]interface{}:
					arr[i] = redactSensitiveDetails(s)
				default:
					arr[i] = item
				}
			}
			result[key] = arr
		case map[string]interface{}:
			result[key] = redactSensitiveDetails(v)
		default:
			result[key] = value
		}
	}
	return result
}

// AddAuditLog adds an entry to the audit log.
func AddAuditLog(action string, details map[string]interface{}, c *gin.Context) {
	auditMu.Lock()
	defer auditMu.Unlock()

	ip := "系统"
	userAgent := "系统"
	method := "系统"
	requestPath := ""

	if c != nil {
		ip = utils.GetClientIP(c.Request)
		userAgent = c.Request.UserAgent()
		if userAgent == "" {
			userAgent = "未知"
		}
		method = c.Request.Method
		requestPath = c.Request.URL.Path

		if v, exists := c.Get("clientIP"); exists {
			if s, ok := v.(string); ok {
				ip = s
			}
		}
	}

	entry := types.AuditLogEntry{
		Timestamp: time.Now().Format(time.RFC3339),
		Action:    getActionName(action),
		Details:   mergeDetails(redactSensitiveDetails(details), method, requestPath, action),
		IP:        ip,
		UserAgent: userAgent,
	}

	cfg := config.Get()

	// Ensure data directory exists
	os.MkdirAll(cfg.DataDir, 0700)

	line, err := json.Marshal(entry)
	if err != nil {
		slog.Error("failed to marshal audit entry", "error", err)
		return
	}

	f, err := os.OpenFile(cfg.AuditLogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		slog.Error("failed to open audit log file", "error", err)
		return
	}
	defer f.Close()

	f.WriteString(string(line) + "\n")
}

func mergeDetails(details map[string]interface{}, method, path, action string) map[string]interface{} {
	if details == nil {
		details = make(map[string]interface{})
	}
	details["method"] = method
	details["path"] = path
	details["originalAction"] = action
	return details
}

// AddSecurityAuditLog adds a security-related audit entry.
func AddSecurityAuditLog(action string, details map[string]interface{}, c *gin.Context) {
	if details == nil {
		details = make(map[string]interface{})
	}
	details["securityEvent"] = true
	details["timestamp"] = time.Now().UnixMilli()
	AddAuditLog("SECURITY_"+action, details, c)
}

// GetAuditLogs retrieves recent audit log entries.
func GetAuditLogs(limit int) ([]types.AuditLogEntry, error) {
	auditLogFile := config.Get().AuditLogFile

	f, err := os.Open(auditLogFile)
	if err != nil {
		if os.IsNotExist(err) {
			return []types.AuditLogEntry{}, nil
		}
		return nil, err
	}
	defer f.Close()

	var entries []types.AuditLogEntry
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var entry types.AuditLogEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}
		entries = append(entries, entry)
	}

	// Reverse and limit
	total := len(entries)
	start := 0
	if total > limit {
		start = total - limit
	}
	result := make([]types.AuditLogEntry, 0, limit)
	for i := total - 1; i >= start && i >= 0; i-- {
		result = append(result, entries[i])
	}
	if len(result) > limit {
		result = result[:limit]
	}

	return result, nil
}

// CleanOldAuditLogs rotates and trims audit logs.
func CleanOldAuditLogs() {
	auditMu.Lock()
	defer auditMu.Unlock()

	rotateAuditLog()

	auditLogFile := config.Get().AuditLogFile
	maxLogs := config.Get().Audit.MaxLogs

	data, err := os.ReadFile(auditLogFile)
	if err != nil {
		if os.IsNotExist(err) {
			return
		}
		slog.Error("failed to read audit log", "error", err)
		return
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) > maxLogs {
		kept := lines[len(lines)-maxLogs:]
		if err := os.WriteFile(auditLogFile, []byte(strings.Join(kept, "\n")+"\n"), 0600); err != nil {
			slog.Error("failed to trim audit log", "error", err)
		}
	}
}

func rotateAuditLog() {
	const maxFileSize = 10 * 1024 * 1024
	const maxRotatedFiles = 5

	auditLogFile := config.Get().AuditLogFile

	info, err := os.Stat(auditLogFile)
	if err != nil {
		if os.IsNotExist(err) {
			return
		}
		return
	}

	if info.Size() < maxFileSize {
		return
	}

	// Rotate: .4 -> .5, .3 -> .4, etc.
	for i := maxRotatedFiles - 1; i >= 1; i-- {
		oldPath := auditLogFile + "." + string(rune('0'+i))
		newPath := auditLogFile + "." + string(rune('0'+i+1))
		if _, err := os.Stat(oldPath); err == nil {
			if i == maxRotatedFiles-1 {
				os.Remove(oldPath)
			} else {
				os.Rename(oldPath, newPath)
			}
		}
	}

	// Rotate current to .1
	os.Rename(auditLogFile, auditLogFile+".1")

	slog.Info("audit log rotated", "originalSize", info.Size())
}

func init() {
	// Periodic cleanup
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		for range ticker.C {
			CleanOldAuditLogs()
		}
	}()

	// Daily sensitive info scan
	go func() {
		ticker := time.NewTicker(24 * time.Hour)
		for range ticker.C {
			scanForSensitiveInfo()
		}
	}()
}

func scanForSensitiveInfo() {
	cfg := config.Get()
	var results []map[string]interface{}

	for _, dir := range cfg.LogDirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}

			filePath := filepath.Join(dir, entry.Name())
			data, err := os.ReadFile(filePath)
			if err != nil {
				continue
			}

			patterns := make([]*regexp.Regexp, 0, len(cfg.SensitivePatterns))
			for _, p := range cfg.SensitivePatterns {
				if re, err := regexp.Compile(p); err == nil {
					patterns = append(patterns, re)
				}
			}

			var matches []string
			content := string(data)
			for _, re := range patterns {
				found := re.FindAllString(content, -1)
				for _, m := range found {
					if len(matches) < 3 {
						matches = append(matches, m[:min(len(m), 50)])
					}
				}
			}

			if len(matches) > 0 {
				results = append(results, map[string]interface{}{
					"path":    filePath,
					"matches": matches[:min(len(matches), 10)],
				})
			}
		}
	}

	if len(results) > 0 {
		AddSecurityAuditLog("SENSITIVE_INFO_SCAN", map[string]interface{}{
			"filesFound": len(results),
		}, nil)
	}
}
