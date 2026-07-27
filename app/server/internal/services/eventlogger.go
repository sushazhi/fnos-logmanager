package services

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
	"github.com/sushazhi/fnos-logmanager/internal/config"
	"github.com/sushazhi/fnos-logmanager/internal/types"
	"github.com/sushazhi/fnos-logmanager/internal/utils"
)

// EventLogEntry represents a system event log entry (backward compatible).
type EventLogEntry struct {
	ID        int64     `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	Type      string    `json:"type"`
	Severity  string    `json:"severity"`
	Message   string    `json:"message"`
	RawData   string    `json:"rawData,omitempty"`
}

// EventLoggerState holds the runtime state of the event logger.
type EventLoggerState struct {
	mu          sync.RWMutex
	db          *sql.DB
	running     bool
	stopCh      chan struct{}
	lastEventID int64
	dbPath      string

	// Status tracking
	totalProcessed int
	lastCheckTime  time.Time
	lastEventTime  time.Time
	lastError      string
}

var globalEventLogger *EventLoggerState

// commonEventLoggerDBs lists common fnOS event logger database paths.
var commonEventLoggerDBs = []string{
	"/usr/trim/var/eventlogger_service/logger_data.db3",
	"/usr/trim/var/event_logger/logger_data.db3",
	"/var/log/eventlogger/logger_data.db3",
}

// findEventLoggerDB checks if the configured path exists; if not, tries common alternatives.
func findEventLoggerDB(configured string) string {
	if _, err := os.Stat(configured); err == nil {
		return configured
	}
	for _, alt := range commonEventLoggerDBs {
		if alt == configured {
			continue
		}
		if _, err := os.Stat(alt); err == nil {
			slog.Warn("event logger DB not found at configured path, using auto-detected path",
				"configured", configured, "detected", alt)
			return alt
		}
	}
	return configured
}

// InitEventLogger initializes the event logger.
func InitEventLogger(dbPath string) error {
	// Auto-detect if the configured path doesn't exist
	dbPath = findEventLoggerDB(dbPath)

	// Check file exists
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return fmt.Errorf("数据库文件不存在: %s", dbPath)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return fmt.Errorf("failed to open event logger DB: %w", err)
	}

	if err := db.Ping(); err != nil {
		db.Close()
		return fmt.Errorf("failed to ping event logger DB: %w", err)
	}

	// Detect available tables
	rows, err := db.Query("SELECT name FROM sqlite_master WHERE type='table' ORDER BY name")
	if err != nil {
		db.Close()
		return fmt.Errorf("failed to query tables: %w", err)
	}

	var tableNames []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			continue
		}
		tableNames = append(tableNames, name)
	}
	rows.Close()

	slog.Info("event logger connected", "db", dbPath, "tables", strings.Join(tableNames, ","))

	if len(tableNames) == 0 {
		db.Close()
		return fmt.Errorf("no tables found in event logger DB")
	}

	// Only set globalEventLogger after all checks pass
	globalEventLogger = &EventLoggerState{
		db:          db,
		stopCh:      make(chan struct{}),
		lastEventID: getLatestEventID(db),
		dbPath:      dbPath,
	}

	return nil
}

// getLatestEventID gets the maximum row ID from the event table.
func getLatestEventID(db *sql.DB) int64 {
	tableName := getFirstTable(db)
	if tableName == "" {
		return 0
	}

	query := fmt.Sprintf(`SELECT MAX(rowid) FROM "%s"`, tableName)
	var id sql.NullInt64
	if err := db.QueryRow(query).Scan(&id); err != nil {
		return 0
	}
	if id.Valid {
		return id.Int64
	}
	return 0
}

// getFirstTable returns the first non-sqlite table name.
// isValidSQLIdentifier checks if a string is a safe SQL identifier (table/column name).
func isValidSQLIdentifier(name string) bool {
	if name == "" {
		return false
	}
	// Only allow alphanumeric, underscore, and ensure it doesn't start with a digit
	matched, _ := regexp.MatchString(`^[a-zA-Z_][a-zA-Z0-9_]*$`, name)
	return matched
}

func getFirstTable(db *sql.DB) string {
	row := db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY rowid LIMIT 1`)
	var name string
	if err := row.Scan(&name); err != nil {
		return ""
	}
	if !isValidSQLIdentifier(name) {
		slog.Error("invalid table name from database schema", "name", name)
		return ""
	}
	return name
}

// StartEventLogger starts the event polling loop.
func StartEventLogger() error {
	if globalEventLogger == nil {
		return fmt.Errorf("event logger not initialized")
	}

	globalEventLogger.mu.Lock()
	if globalEventLogger.running {
		globalEventLogger.mu.Unlock()
		return fmt.Errorf("事件日志已在运行")
	}
	globalEventLogger.running = true
	globalEventLogger.stopCh = make(chan struct{})
	globalEventLogger.mu.Unlock()

	go globalEventLogger.pollLoop()
	slog.Info("event logger started")
	return nil
}

// StopEventLogger stops the event polling loop.
func StopEventLogger() error {
	if globalEventLogger == nil {
		return fmt.Errorf("event logger not initialized")
	}

	globalEventLogger.mu.Lock()
	if !globalEventLogger.running {
		globalEventLogger.mu.Unlock()
		return fmt.Errorf("事件日志未在运行")
	}
	globalEventLogger.running = false
	close(globalEventLogger.stopCh)
	globalEventLogger.mu.Unlock()

	if globalEventLogger.db != nil {
		globalEventLogger.db.Close()
		globalEventLogger.db = nil
	}

	slog.Info("event logger stopped")
	return nil
}

// CloseEventLogger fully shuts down and resets the event logger state.
func CloseEventLogger() {
	if globalEventLogger == nil {
		return
	}
	globalEventLogger.mu.Lock()
	defer globalEventLogger.mu.Unlock()
	if globalEventLogger.running {
		globalEventLogger.running = false
		close(globalEventLogger.stopCh)
	}
	if globalEventLogger.db != nil {
		globalEventLogger.db.Close()
	}
	globalEventLogger = nil
	slog.Info("event logger closed")
}

func (el *EventLoggerState) pollLoop() {
	ticker := time.NewTicker(time.Duration(config.Get().EventLogger.CheckInterval) * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-el.stopCh:
			return
		case <-ticker.C:
			el.mu.Lock()
			el.lastCheckTime = time.Now()
			el.mu.Unlock()
			el.pollNewEvents()
		}
	}
}

func (el *EventLoggerState) pollNewEvents() {
	if el.db == nil {
		return
	}

	tableName := getFirstTable(el.db)
	if tableName == "" {
		return
	}

	query := fmt.Sprintf(`SELECT rowid, * FROM "%s" WHERE rowid > ? ORDER BY rowid ASC LIMIT 100`, tableName)

	eventRows, err := el.db.Query(query, el.lastEventID)
	if err != nil {
		slog.Debug("event poll query failed", "error", err)
		return
	}
	defer eventRows.Close()

	columns, err := eventRows.Columns()
	if err != nil {
		return
	}

	var lastID int64
	newEvents := 0

	for eventRows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := eventRows.Scan(valuePtrs...); err != nil {
			continue
		}

		// Extract rowid (first column)
		var rowID int64
		if id, ok := values[0].(int64); ok {
			rowID = id
			lastID = id
		}

		// Parse row into a map
		rowMap := make(map[string]interface{})
		for i, col := range columns {
			if values[i] != nil {
				rowMap[col] = values[i]
			}
		}

		// Parse enhanced event entry
		entry := parseEventRow(rowMap)
		entry.ID = rowID

		newEvents++

		// Fire notification for significant events
		cfg := config.Get()
		if cfg.EventLogger.Enabled && isSignificantEvent(entry) {
				matchedRuleIDs := matchEventNotificationRules(entry)
			for _, ruleID := range matchedRuleIDs {
				if ns := getNotifStore(); ns != nil {
					rule := ns.GetRule(ruleID)
					if rule == nil {
						continue
					}

				// Map numeric type to readable category name, fallback to raw EventType
				categoryName := entry.EventType
				// Prefer the Cat field (parsed from the JSON parameter's "cat" field)
				// which is more accurate than the DB type column (which may always be "1").
				if entry.Cat > 0 {
					if name, ok := categoryMessages[entry.Cat]; ok {
						categoryName = name
					}
				} else {
					var catNum int
					if _, err := fmt.Sscanf(entry.EventType, "%d", &catNum); err == nil {
						if name, ok := categoryMessages[catNum]; ok {
							categoryName = name
						}
					}
				}
				timeStr := entry.Timestamp
				cst := time.FixedZone("CST", 8*3600)
				if t, err := time.Parse(time.RFC3339, entry.Timestamp); err == nil {
					timeStr = t.In(cst).Format("2006-01-02 15:04:05")
				} else if t, err := time.Parse("2006-01-02 15:04:05", entry.Timestamp); err == nil {
					timeStr = t.In(cst).Format("2006-01-02 15:04:05")
				}
					title := "系统通知"
					message := fmt.Sprintf("[%s] %s\n%s", timeStr, categoryName, entry.Message)

					// Send to each channel specified in the rule and record history
					for _, chName := range rule.Channels {
						ch := ns.GetChannel(chName)
						if ch == nil || !ch.Enabled {
							continue
						}
						err := SendNotificationToChannel(title, message, ch.Channel, ch.Config)
						success := err == nil
						errMsg := ""
						if err != nil {
							errMsg = err.Error()
						}
						_ = ns.AddHistory(NotificationHistory{
							ID:        generateID("notif"),
							RuleID:    ruleID,
							Channel:   chName,
							Title:     title,
							Message:   message,
							Success:   success,
							Error:     errMsg,
							Timestamp: time.Now(),
						})
					}

					ns.UpdateRuleTrigger(ruleID)
				}
			}
		}
	}

	if lastID > 0 {
		el.mu.Lock()
		el.lastEventID = lastID
		el.totalProcessed += newEvents
		if newEvents > 0 {
			el.lastEventTime = time.Now()
		}
		el.lastError = ""
		el.mu.Unlock()
	}

	if newEvents > 0 {
		slog.Debug("polled new events", "count", newEvents, "lastID", lastID)
	}
}

func getNotifStore() *NotificationStore {
	// Use the global notification store if available (set via SetNotifyStore from main.go).
	// If nil, notification rule triggers are silently skipped.
	return notifStore
}

// ==================== Template Message Formatting ====================

// templateMessages maps event template names to Chinese message formatters.
var templateMessages = map[string]func(params map[string]interface{}) string{
	// User related
	"LoginSucc": func(p map[string]interface{}) string {
		return fmt.Sprintf("%s登录成功 IP:%s", getParam(p, "user", "用户"), getParam(p, "IP", "未知"))
	},
	"LoginFail": func(p map[string]interface{}) string {
		return fmt.Sprintf("%s登录失败 IP:%s", getParam(p, "user", "用户"), getParam(p, "IP", "未知"))
	},
	"Logout": func(p map[string]interface{}) string {
		return fmt.Sprintf("%s登出", getParam(p, "user", "用户"))
	},
	"UserAdd": func(p map[string]interface{}) string {
		return fmt.Sprintf("添加用户 %s", getParam(p, "user", "未知"))
	},
	"UserDel": func(p map[string]interface{}) string {
		return fmt.Sprintf("删除用户 %s", getParam(p, "user", "未知"))
	},
	"UserMod": func(p map[string]interface{}) string {
		return fmt.Sprintf("修改用户 %s", getParam(p, "user", "未知"))
	},
	"PassMod": func(p map[string]interface{}) string {
		return fmt.Sprintf("%s修改密码", getParam(p, "user", "用户"))
	},
	"FoundDisk": func(p map[string]interface{}) string {
		return fmt.Sprintf("发现硬盘%s 型号:%s 序列号:%s",
			getParam(p, "name", getParam(p, "disk", "未知")),
			getParam(p, "model", "未知"),
			getParam(p, "serial", "未知"))
	},
	"DiskWakeup": func(p map[string]interface{}) string {
		return fmt.Sprintf("硬盘唤醒 %s", getParam(p, "disk", ""))
	},
	"DiskSpindown": func(p map[string]interface{}) string {
		return fmt.Sprintf("硬盘休眠 %s", getParam(p, "disk", ""))
	},
	"DiskAdd": func(p map[string]interface{}) string {
		return fmt.Sprintf("添加硬盘 %s", getParam(p, "disk", ""))
	},
	"DiskRemove": func(p map[string]interface{}) string {
		return fmt.Sprintf("移除硬盘 %s", getParam(p, "disk", ""))
	},
	"DiskError": func(p map[string]interface{}) string {
		return fmt.Sprintf("硬盘错误 %s %s", getParam(p, "disk", "未知"), getParam(p, "error", ""))
	},
	"NetUp": func(p map[string]interface{}) string {
		return fmt.Sprintf("网络接口%s已启动", getParam(p, "iface", "未知"))
	},
	"NetDown": func(p map[string]interface{}) string {
		return fmt.Sprintf("网络接口%s已停止", getParam(p, "iface", "未知"))
	},
	"NetChange": func(p map[string]interface{}) string {
		return "网络配置已更改"
	},
	"Shutdown": func(p map[string]interface{}) string {
		return "系统关机"
	},
	"Reboot": func(p map[string]interface{}) string {
		return "系统重启"
	},
	"Startup": func(p map[string]interface{}) string {
		return "系统启动"
	},
	"ServiceStart": func(p map[string]interface{}) string {
		return fmt.Sprintf("服务%s已启动", getParam(p, "service", "未知"))
	},
	"ServiceStop": func(p map[string]interface{}) string {
		return fmt.Sprintf("服务%s已停止", getParam(p, "service", "未知"))
	},
	"ServiceRestart": func(p map[string]interface{}) string {
		return fmt.Sprintf("服务%s已重启", getParam(p, "service", "未知"))
	},
	"AppInstall": func(p map[string]interface{}) string {
		return fmt.Sprintf("安装应用 %s", getParam(p, "app", ""))
	},
	"AppUninstall": func(p map[string]interface{}) string {
		return fmt.Sprintf("卸载应用 %s", getParam(p, "app", ""))
	},
	"AppUpdate": func(p map[string]interface{}) string {
		return fmt.Sprintf("更新应用 %s", getParam(p, "app", ""))
	},
	"AppStart": func(p map[string]interface{}) string {
		return fmt.Sprintf("启动应用 %s", getParam(p, "app", ""))
	},
	"AppStop": func(p map[string]interface{}) string {
		return fmt.Sprintf("停止应用 %s", getParam(p, "app", ""))
	},
	"AppRestart": func(p map[string]interface{}) string {
		return fmt.Sprintf("重启应用 %s", getParam(p, "app", ""))
	},
	"DeleteFile": func(p map[string]interface{}) string {
		return fmt.Sprintf("删除文件 %s", getParam(p, "path", getParam(p, "FILE", getParam(p, "file", ""))))
	},
	"CreateFile": func(p map[string]interface{}) string {
		return fmt.Sprintf("创建文件 %s", getParam(p, "path", getParam(p, "FILE", getParam(p, "file", ""))))
	},
	"MoveFile": func(p map[string]interface{}) string {
		return fmt.Sprintf("移动文件 %s -> %s",
			getParam(p, "src", getParam(p, "SRC", "")),
			getParam(p, "dst", getParam(p, "DST", "")))
	},
	"CopyFile": func(p map[string]interface{}) string {
		return fmt.Sprintf("复制文件 %s -> %s",
			getParam(p, "src", getParam(p, "SRC", "")),
			getParam(p, "dst", getParam(p, "DST", "")))
	},
	"RenameFile": func(p map[string]interface{}) string {
		return fmt.Sprintf("重命名文件 %s -> %s",
			getParam(p, "old", getParam(p, "OLD", "")),
			getParam(p, "new", getParam(p, "NEW", "")))
	},
	"Mkdir": func(p map[string]interface{}) string {
		return fmt.Sprintf("创建目录 %s",
			getParam(p, "path", getParam(p, "FILE", getParam(p, "dir", ""))))
	},
	"UploadFile": func(p map[string]interface{}) string {
		return fmt.Sprintf("上传文件 %s", getParam(p, "FILE", getParam(p, "file", "")))
	},
	"DownloadFile": func(p map[string]interface{}) string {
		return fmt.Sprintf("下载文件 %s", getParam(p, "FILE", getParam(p, "file", "")))
	},
	"RaidCreate": func(p map[string]interface{}) string {
		return fmt.Sprintf("创建RAID %s", getParam(p, "raid", ""))
	},
	"RaidDelete": func(p map[string]interface{}) string {
		return fmt.Sprintf("删除RAID %s", getParam(p, "raid", ""))
	},
	"VolumeCreate": func(p map[string]interface{}) string {
		return fmt.Sprintf("创建存储卷 %s", getParam(p, "volume", ""))
	},
	"VolumeDelete": func(p map[string]interface{}) string {
		return fmt.Sprintf("删除存储卷 %s", getParam(p, "volume", ""))
	},
	"VolumeExpand": func(p map[string]interface{}) string {
		return fmt.Sprintf("扩展存储卷 %s", getParam(p, "volume", ""))
	},
	"ShareCreate": func(p map[string]interface{}) string {
		return fmt.Sprintf("创建共享 %s", getParam(p, "share", ""))
	},
	"ShareDelete": func(p map[string]interface{}) string {
		return fmt.Sprintf("删除共享 %s", getParam(p, "share", ""))
	},
	"ShareMod": func(p map[string]interface{}) string {
		return fmt.Sprintf("修改共享 %s", getParam(p, "share", ""))
	},
	"SystemStart": func(p map[string]interface{}) string {
		return "系统启动"
	},
	"SystemShutdown": func(p map[string]interface{}) string {
		return "系统关机"
	},
	"SystemReboot": func(p map[string]interface{}) string {
		return "系统重启"
	},
	"SystemUpdate": func(p map[string]interface{}) string {
		return "系统更新"
	},
	"NetworkChange": func(p map[string]interface{}) string {
		return fmt.Sprintf("网络配置变更 %s", getParam(p, "iface", ""))
	},
	"BackupCreate": func(p map[string]interface{}) string {
		return fmt.Sprintf("创建备份 %s", getParam(p, "name", ""))
	},
	"BackupRestore": func(p map[string]interface{}) string {
		return fmt.Sprintf("恢复备份 %s", getParam(p, "name", ""))
	},
	"BackupDelete": func(p map[string]interface{}) string {
		return fmt.Sprintf("删除备份 %s", getParam(p, "name", ""))
	},
	"ContainerCreate": func(p map[string]interface{}) string {
		return fmt.Sprintf("创建容器 %s", getParam(p, "container", ""))
	},
	"ContainerDelete": func(p map[string]interface{}) string {
		return fmt.Sprintf("删除容器 %s", getParam(p, "container", ""))
	},
	"ContainerStart": func(p map[string]interface{}) string {
		return fmt.Sprintf("启动容器 %s", getParam(p, "container", ""))
	},
	"ContainerStop": func(p map[string]interface{}) string {
		return fmt.Sprintf("停止容器 %s", getParam(p, "container", ""))
	},
	"PermissionChange": func(p map[string]interface{}) string {
		return fmt.Sprintf("权限变更 %s", getParam(p, "path", ""))
	},
	"AclChange": func(p map[string]interface{}) string {
		return fmt.Sprintf("ACL变更 %s", getParam(p, "path", ""))
	},
}

// categoryMessages maps category numbers to default messages.
var categoryMessages = map[int]string{
	1: "存储事件",
	2: "网络事件",
	3: "用户事件",
	4: "系统事件",
	5: "应用事件",
	6: "备份事件",
	7: "Docker事件",
}

// appActionMap maps application event IDs to Chinese descriptions.
var appActionMap = map[string]string{
	"APP_STARTED":                    "启用成功",
	"APP_AUTO_STARTED":               "自动启动成功",
	"APP_STOPPED":                    "停止成功",
	"APP_INSTALLED":                  "安装成功",
	"APP_UNINSTALLED":                "卸载成功",
	"APP_UPDATED":                    "更新成功",
	"APP_UPGRADED":                   "升级成功",
	"APP_CRASH":                      "异常退出",
	"APP_START_FAILED_LOCAL_APP_RUN_EXCEPTION": "启用失败。原因：执行应用启动脚本失败。",
}

func getParam(params map[string]interface{}, key string, defaultVal string) string {
	if v, ok := params[key]; ok && v != nil {
		if s, ok2 := v.(string); ok2 && s != "" {
			return s
		}
		return fmt.Sprintf("%v", v)
	}
	return defaultVal
}

// formatTemplateMessage formats a message using the template system.
func formatTemplateMessage(params map[string]interface{}) string {
	tmpl, _ := params["template"].(string)
	if tmpl == "" {
		return ""
	}

	if formatter, ok := templateMessages[tmpl]; ok {
		return formatter(params)
	}

	if cat, ok := params["cat"].(float64); ok {
		catInt := int(cat)
		if msg, ok := categoryMessages[catInt]; ok {
			return fmt.Sprintf("%s: %s", msg, toJSON(params))
		}
	}

	if msg, ok := params["message"].(string); ok {
		return msg
	}
	return toJSON(params)
}

// formatEventMessage formats an event log entry into a readable message.
func formatEventMessage(event types.EnhancedEventLogEntry) string {
	if event.Param != "" {
		var params map[string]interface{}
		if err := json.Unmarshal([]byte(event.Param), &params); err == nil {
			return formatTemplateMessage(params)
		}
	}

	if event.Template != "" {
		params := map[string]interface{}{"template": event.Template}
		return formatTemplateMessage(params)
	}

	if event.Message != "" {
		return event.Message
	}

	return toJSON(event)
}

// ==================== Event Row Parsing ====================

// parseEventRow parses a raw database row into an EnhancedEventLogEntry.
func parseEventRow(row map[string]interface{}) types.EnhancedEventLogEntry {
	entry := types.EnhancedEventLogEntry{}
	entry.Raw = row

	// Identify column names
	tsKey := findColumn(row, "logtime", "timestamp", "created_at", "createdat")
	srcKey := findColumn(row, "serviceid", "service", "source", "app", "appname")
	msgKey := findColumn(row, "message", "msg", "content", "description", "desc", "log")
	sevKey := findColumn(row, "loglevel", "level", "severity", "priority")
	typeKey := findColumn(row, "type", "category", "event_type", "eventtype")
	userKey := findColumn(row, "uid", "uname", "user", "username")

	// Process timestamp
	if tsKey != "" {
		rawTS := row[tsKey]
		entry.Timestamp = processTimestamp(rawTS)
	}

	// Process source
	if srcKey != "" {
		entry.Source = fmt.Sprintf("%v", row[srcKey])
		if entry.Source == "<nil>" || entry.Source == "null" {
			entry.Source = "unknown"
		}
	}

	// Process event type
	if typeKey != "" {
		entry.EventType = fmt.Sprintf("%v", row[typeKey])
		if entry.EventType == "<nil>" || entry.EventType == "null" {
			entry.EventType = "system"
		}
	} else {
		entry.EventType = "system"
	}

	// Process severity
	if sevKey != "" {
		entry.Severity = parseSeverity(row[sevKey])
	}

	// Process user
	if userKey != "" {
		val := fmt.Sprintf("%v", row[userKey])
		if val != "<nil>" && val != "null" {
			entry.User = val
		}
	}

	// Extract parameter (raw JSON with template info for formatted messages)
	if paramVal, ok := row["parameter"]; ok && paramVal != nil {
		if paramStr, ok2 := paramVal.(string); ok2 && paramStr != "" && paramStr != "null" {
			entry.Param = paramStr
		}
	}

	// Process message (uses param/parameter for template-formatted Chinese text)
	entry.Message = processMessage(row, msgKey)

	// Extract template from parameter JSON if not already set by processMessage
	if entry.Template == "" && entry.Param != "" {
		var params map[string]interface{}
		if err := json.Unmarshal([]byte(entry.Param), &params); err == nil {
			if tmpl, _ := params["template"].(string); tmpl != "" {
				entry.Template = tmpl
			}
			// Also extract category for category-based lookup
			if cat, ok := params["cat"].(float64); ok {
				entry.Cat = int(cat)
			}
		}
	}

	return entry
}

// findColumn finds the first matching column key in the row.
func findColumn(row map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if _, ok := row[key]; ok {
			return key
		}
		lowerKey := strings.ToLower(key)
		for k := range row {
			if strings.ToLower(k) == lowerKey {
				return k
			}
		}
	}
	return ""
}

// processTimestamp converts a timestamp value to ISO 8601 string.
func processTimestamp(val interface{}) string {
	if val == nil {
		return time.Now().Format(time.RFC3339)
	}

	var result string

	switch v := val.(type) {
	case int64:
		if v > 1000000000000 {
			// milliseconds
			result = time.UnixMilli(v).Format(time.RFC3339)
		} else if v > 1000000000 {
			// seconds — DB stores CST-local Unix timestamps
			_, offset := time.Now().Zone()
			v += int64(offset) // compensate for CST epoch storage
			utcTime := time.Unix(v, 0)
			result = utcTime.Format(time.RFC3339)
		} else {
			result = time.Unix(v, 0).Format(time.RFC3339)
		}
	case float64:
		i := int64(v)
		if i > 1000000000000 {
			result = time.UnixMilli(i).Format(time.RFC3339)
		} else if i > 1000000000 {
			// seconds — DB stores CST-local Unix timestamps
			_, offset := time.Now().Zone()
			i += int64(offset) // compensate for CST epoch storage
			utcTime := time.Unix(i, 0)
			result = utcTime.Format(time.RFC3339)
		} else {
			result = time.Unix(i, 0).Format(time.RFC3339)
		}
	case string:
		t, err := time.Parse(time.RFC3339, v)
		if err == nil {
			result = t.Format(time.RFC3339)
			return result
		}
		// Try parsing as local time (CST/Asia/Shanghai) — fnOS event logger
		// stores timestamps in local time without timezone info in some tables.
		t, err = time.ParseInLocation("2006-01-02 15:04:05", v, time.Local)
		if err == nil {
			result = t.Format(time.RFC3339)
			return result
		}
		return v
	}

	return result
}

// parseSeverity converts a severity value from DB to EventSeverity.
func parseSeverity(val interface{}) types.EventSeverity {
	if val == nil {
		return types.SeverityInfo
	}

	switch v := val.(type) {
	case float64:
		switch int(v) {
		case 0:
			return types.SeverityInfo
		case 1:
			return types.SeverityWarning
		case 2:
			return types.SeverityError
		case 3, 4:
			return types.SeverityCritical
		default:
			return types.SeverityInfo
		}
	case int64:
		switch v {
		case 0:
			return types.SeverityInfo
		case 1:
			return types.SeverityWarning
		case 2:
			return types.SeverityError
		case 3, 4:
			return types.SeverityCritical
		default:
			return types.SeverityInfo
		}
	case string:
		s := strings.ToLower(v)
		if strings.Contains(s, "crit") || strings.Contains(s, "emerg") || strings.Contains(s, "panic") {
			return types.SeverityCritical
		}
		if strings.Contains(s, "err") {
			return types.SeverityError
		}
		if strings.Contains(s, "warn") {
			return types.SeverityWarning
		}
		if strings.Contains(s, "debug") || strings.Contains(s, "trace") {
			return types.SeverityDebug
		}
		return types.SeverityInfo
	}
	return types.SeverityInfo
}

// processMessage extracts a readable message from a DB row.
func processMessage(row map[string]interface{}, msgKey string) string {
	// Try direct message field first
	if msgKey != "" {
		if msg, ok := row[msgKey].(string); ok && msg != "" && msg != "null" {
			// Check if it looks like a template format
			return msg
		}
	}

	// Try parameter field (JSON with template info)
	if paramStr, ok := row["parameter"].(string); ok && paramStr != "" && paramStr != "null" {
		var params map[string]interface{}
		if err := json.Unmarshal([]byte(paramStr), &params); err == nil {
			// Template format message
			if tmpl, _ := params["template"].(string); tmpl != "" {
				entry := types.EnhancedEventLogEntry{Param: paramStr, Template: tmpl}
				return formatEventMessage(entry)
			}
			// App event format
			if data, ok := params["data"].(map[string]interface{}); ok {
				appName, _ := data["DISPLAY_NAME"].(string)
				if appName == "" {
					appName, _ = data["APP_NAME"].(string)
				}
				eventID, _ := params["eventId"].(string)
				action := appActionMap[eventID]
				if action == "" {
					action = eventID
				}
				if appName != "" && action != "" {
					return fmt.Sprintf("应用 %s %s", appName, action)
				}
				if appName != "" {
					return appName
				}
				if eventID != "" {
					return eventID
				}
			}
			// Return raw parameter
			if len(paramStr) > 500 {
				return paramStr[:500]
			}
			return paramStr
		}
		if len(paramStr) > 500 {
			return paramStr[:500]
		}
		return paramStr
	}

	// Try to extract from other fields
	excluded := map[string]bool{
		"rowid": true, "id": true, "parameter": true, "metadata": true,
	}
	for k := range row {
		excluded[strings.ToLower(k)] = true
	}
	if msgKey != "" {
		excluded[strings.ToLower(msgKey)] = true
	}

	var parts []string
	for k, v := range row {
		if excluded[strings.ToLower(k)] {
			continue
		}
		if v == nil {
			continue
		}
		s := fmt.Sprintf("%v", v)
		if s == "" || s == "<nil>" || s == "null" {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s: %s", k, s))
	}
	if len(parts) > 0 {
		result := strings.Join(parts, ", ")
		if len(result) > 500 {
			result = result[:500]
		}
		return result
	}

	return "未知事件"
}

// isSignificantEvent checks if an event meets the notification criteria.
func isSignificantEvent(entry types.EnhancedEventLogEntry) bool {
	cfg := config.Get()
	if !cfg.EventLogger.Enabled {
		return false
	}

	// Check event types filter
	if len(cfg.EventLogger.EventTypes) > 0 && cfg.EventLogger.EventTypes[0] != "*" {
		match := false
		for _, et := range cfg.EventLogger.EventTypes {
			if strings.EqualFold(et, entry.EventType) {
				match = true
				break
			}
		}
		if !match {
			return false
		}
	}

	// Check severity filter
	minLevel, ok := types.SeverityOrder[types.EventSeverity(cfg.EventLogger.MinSeverity)]
	if !ok {
		minLevel = 1 // info
	}
	entryLevel, ok := types.SeverityOrder[entry.Severity]
	if !ok {
		entryLevel = 1
	}

	return entryLevel >= minLevel
}

// ruleLevelToSeverity maps a rule's LogLevel string to a numeric severity value.
func ruleLevelToSeverity(level string) int {
	switch strings.ToLower(level) {
	case "error":
		return 3
	case "warning", "warn":
		return 2
	case "info":
		return 1
	case "debug", "all", "":
		return 0
	default:
		return 0
	}
}

// matchesEventAppName checks if the rule's AppName targets eventlogger events.
func matchesEventAppName(appName string) bool {
	if appName == "" || appName == "*" {
		return true
	}
	parts := strings.Split(appName, ",")
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "eventlogger" || p == "*" {
			return true
		}
	}
	return false
}

// matchKeywords checks if any keyword in the list appears in the given text (case-insensitive).
func matchKeywords(keywords []string, text string) bool {
	lower := strings.ToLower(text)
	for _, kw := range keywords {
		if kw != "" && strings.Contains(lower, strings.ToLower(kw)) {
			return true
		}
	}
	return false
}

// extractEventIP extracts the source IP address from an event's parameter JSON.
// Login events (LoginSucc/LoginFail) store the source IP under the "IP" key
// in the parameter JSON blob. Returns empty string if no IP is found.
func extractEventIP(event types.EnhancedEventLogEntry) string {
	if event.Param == "" {
		return ""
	}
	var params map[string]interface{}
	if err := json.Unmarshal([]byte(event.Param), &params); err != nil {
		return ""
	}
	if ip, ok := params["IP"].(string); ok && ip != "" {
		return ip
	}
	return ""
}

// isEventIPWhitelisted checks whether the event's source IP falls within
// the rule's IPWhitelist. Returns false if the rule has no whitelist
// configured or if no IP can be extracted from the event.
func isEventIPWhitelisted(event types.EnhancedEventLogEntry, rule NotificationRule) bool {
	if len(rule.IPWhitelist) == 0 {
		return false
	}
	ip := extractEventIP(event)
	if ip == "" {
		return false
	}
	return utils.IsIPInAnyCIDR(ip, rule.IPWhitelist)
}

// matchEventNotificationRules finds notification rules that match this event.
func matchEventNotificationRules(event types.EnhancedEventLogEntry) []string {
	ns := getNotifStore()
	if ns == nil {
		return nil
	}

	rules := ns.GetEnabledRules()
	if len(rules) == 0 {
		return nil
	}

	// Use formatEventMessage to get the full formatted message (template-based Chinese text from Param)
	// instead of raw event.Message which may be English/non-descriptive.
	message := formatEventMessage(event)
	if message == "" {
		return nil
	}

	entryLevel := types.SeverityOrder[event.Severity]

	var matched []string
	for _, rule := range rules {
		// Check AppName targets eventlogger
		if !matchesEventAppName(rule.AppName) {
			continue
		}

		// Check severity threshold
		minLevel := ruleLevelToSeverity(rule.LogLevel)
		if entryLevel < minLevel {
			continue
		}

		// Check exclude keywords
		if len(rule.ExcludeKeywords) > 0 && matchKeywords(rule.ExcludeKeywords, message) {
			continue
		}

		// Check inclusion keywords
		if len(rule.Keywords) > 0 && !matchKeywords(rule.Keywords, message) {
			continue
		}

		// Check regex pattern
		if rule.Pattern != "" {
			re, err := regexp.Compile("(?i)" + rule.Pattern)
			if err != nil || !re.MatchString(message) {
				continue
			}
		}

		// Check IP whitelist: if the event has a source IP that matches the
		// rule's IPWhitelist CIDR list, skip notification for this event.
		// This is useful for login events from trusted/internal networks.
		if isEventIPWhitelisted(event, rule) {
			continue
		}

		matched = append(matched, rule.ID)
	}

	return matched
}

func mapSeverity(s string) string {
	switch strings.ToLower(s) {
	case "critical", "error":
		return "error"
	case "warning":
		return "warning"
	default:
		return "info"
	}
}

// toJSON converts a value to its JSON string representation.
func toJSON(v interface{}) string {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	return string(data)
}

// ==================== Public API ====================

// EventLogFilter defines filter parameters for querying event logs.
type EventLogFilter struct {
	Limit     int
	Offset    int
	Severity  string // debug, info, warning, error, critical
	Source    string
	EventType string
	Search    string // free-text search in message
	StartTime string // RFC 3339 or "2006-01-02 15:04:05"
	EndTime   string
}

// GetEventLogs retrieves event logs with optional filters.
// Returns the entries and total count (before limit/offset).
func GetEventLogs(filter EventLogFilter) ([]types.EnhancedEventLogEntry, int, error) {
	if globalEventLogger == nil || globalEventLogger.db == nil {
		return nil, 0, fmt.Errorf("event logger not initialized")
	}

	tableName := getFirstTable(globalEventLogger.db)
	if tableName == "" {
		return nil, 0, fmt.Errorf("no event table found")
	}

	// Determine the column names in this specific DB
	// We need to probe because different event logger DBs have different schemas.
	sampleRow, err := globalEventLogger.db.Query(fmt.Sprintf(`SELECT * FROM "%s" LIMIT 1`, tableName))
	if err != nil {
		return nil, 0, err
	}
	allColumns, err := sampleRow.Columns()
	sampleRow.Close()
	if err != nil {
		return nil, 0, err
	}

	// Build column-agnostic WHERE clauses using findColumn logic
	colTimestamp := resolveColumn("logtime", allColumns, "timestamp", "created_at", "createdat")
	colSource := resolveColumn("source", allColumns, "serviceid", "service", "app", "appname")
	colSeverity := resolveColumn("severity", allColumns, "loglevel", "level", "priority")
	colType := resolveColumn("type", allColumns, "category", "event_type", "eventtype")
	colMessage := resolveColumn("message", allColumns, "msg", "content", "description", "desc", "log")

	var whereClauses []string
	var args []interface{}

	if filter.Severity != "" {
		// Match against the known severity column
		if colSeverity != "" {
			whereClauses = append(whereClauses, fmt.Sprintf(`LOWER("%s") = ?`, colSeverity))
			args = append(args, strings.ToLower(filter.Severity))
		}
	}

	if filter.Source != "" {
		if colSource != "" {
			whereClauses = append(whereClauses, fmt.Sprintf(`LOWER("%s") LIKE ?`, colSource))
			args = append(args, "%"+strings.ToLower(filter.Source)+"%")
		}
	}

	if filter.EventType != "" {
		if colType != "" {
			whereClauses = append(whereClauses, fmt.Sprintf(`LOWER("%s") LIKE ?`, colType))
			args = append(args, "%"+strings.ToLower(filter.EventType)+"%")
		}
	}

	if filter.Search != "" {
		if colMessage != "" {
			whereClauses = append(whereClauses, fmt.Sprintf(`LOWER("%s") LIKE ?`, colMessage))
			args = append(args, "%"+strings.ToLower(filter.Search)+"%")
		}
	}

	if filter.StartTime != "" {
		if colTimestamp != "" {
			ts := convertToDBTime(filter.StartTime)
			if ts != "" {
				whereClauses = append(whereClauses, fmt.Sprintf(`"%s" >= ?`, colTimestamp))
				args = append(args, ts)
			}
		}
	}

	if filter.EndTime != "" {
		if colTimestamp != "" {
			ts := convertToDBTime(filter.EndTime)
			if ts != "" {
				whereClauses = append(whereClauses, fmt.Sprintf(`"%s" <= ?`, colTimestamp))
				args = append(args, ts)
			}
		}
	}

	// Build the WHERE clause
	whereSQL := ""
	if len(whereClauses) > 0 {
		whereSQL = " WHERE " + strings.Join(whereClauses, " AND ")
	}

	// Get total count
	countSQL := fmt.Sprintf(`SELECT COUNT(*) FROM "%s"%s`, tableName, whereSQL)
	countRow, err := globalEventLogger.db.Query(countSQL, args...)
	if err != nil {
		return nil, 0, err
	}
	total := 0
	if countRow.Next() {
		countRow.Scan(&total)
	}
	countRow.Close()

	effectiveLimit := filter.Limit
	if effectiveLimit <= 0 {
		effectiveLimit = 100
	}
	if effectiveLimit > 1000 {
		effectiveLimit = 1000
	}

	// Build the data query with limit/offset
	query := fmt.Sprintf(`SELECT rowid, * FROM "%s"%s ORDER BY rowid DESC LIMIT ? OFFSET ?`, tableName, whereSQL)
	queryArgs := append(args, effectiveLimit, filter.Offset)
	eventRows, err := globalEventLogger.db.Query(query, queryArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer eventRows.Close()

	columns, err := eventRows.Columns()
	if err != nil {
		return nil, 0, err
	}

	var entries []types.EnhancedEventLogEntry
	for eventRows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := eventRows.Scan(valuePtrs...); err != nil {
			continue
		}

		rowMap := make(map[string]interface{})
		var rowID int64
		for i, col := range columns {
			if values[i] != nil {
				rowMap[col] = values[i]
				if col == "rowid" {
					if id, ok := values[i].(int64); ok {
						rowID = id
					}
				}
			}
		}

		entry := parseEventRow(rowMap)
		entry.ID = rowID
		entries = append(entries, entry)
	}

	return entries, total, nil
}

// resolveColumn finds the first column from candidates that exists in the DB schema.
func resolveColumn(preferred string, existingColumns []string, alternatives ...string) string {
	allCandidates := append([]string{preferred}, alternatives...)
	colSet := make(map[string]bool, len(existingColumns))
	for _, c := range existingColumns {
		if !isValidSQLIdentifier(c) {
			slog.Error("invalid column name in database schema", "name", c)
			return ""
		}
		colSet[strings.ToLower(c)] = true
	}
	for _, cand := range allCandidates {
		if colSet[strings.ToLower(cand)] {
			return cand
		}
	}
	return ""
}

// convertToDBTime attempts to parse a time string and convert it to the format
// typically used in event logger DBs (RFC3339 or "2006-01-02 15:04:05").
func convertToDBTime(s string) string {
	if s == "" {
		return ""
	}
	formats := []string{
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02",
	}
	for _, f := range formats {
		// Try parsing as local time (CST/Asia/Shanghai) first, since filter times
		// from the frontend are in local timezone.
		t, err := time.ParseInLocation(f, s, time.Local)
		if err == nil {
			return t.Format("2006-01-02 15:04:05")
		}
	}
	return s
}

// GetEventSources returns a list of distinct event sources from the DB.
func GetEventSources() []string {
	if globalEventLogger == nil || globalEventLogger.db == nil {
		return nil
	}

	tableName := getFirstTable(globalEventLogger.db)
	if tableName == "" {
		return nil
	}

	// Probe columns to find the source column
	sampleRow, err := globalEventLogger.db.Query(fmt.Sprintf(`SELECT * FROM "%s" LIMIT 1`, tableName))
	if err != nil {
		return nil
	}
	allColumns, err := sampleRow.Columns()
	sampleRow.Close()
	if err != nil {
		return nil
	}

	sourceCol := resolveColumn("source", allColumns, "serviceid", "service", "app", "appname")
	if sourceCol == "" {
		return nil
	}

	query := fmt.Sprintf(`SELECT DISTINCT "%s" FROM "%s" WHERE "%s" IS NOT NULL AND "%s" != ''`, sourceCol, tableName, sourceCol, sourceCol)
	rows, err := globalEventLogger.db.Query(query)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var sources []string
	for rows.Next() {
		var source string
		if err := rows.Scan(&source); err != nil {
			continue
		}
		if source != "" {
			sources = append(sources, source)
		}
	}
	return sources
}

// ForceEventLoggerCheck triggers an immediate check for new events by running a poll cycle.
func ForceEventLoggerCheck() error {
	if globalEventLogger == nil {
		return fmt.Errorf("event logger not initialized")
	}
	globalEventLogger.mu.Lock()
	globalEventLogger.lastCheckTime = time.Now()
	globalEventLogger.mu.Unlock()
	globalEventLogger.pollNewEvents()
	return nil
}

// GetEventStats returns event logger statistics.
func GetEventStats() map[string]interface{} {
	return map[string]interface{}{
		"enabled": config.Get().EventLogger.Enabled,
		"running": globalEventLogger != nil && globalEventLogger.running,
	}
}

// GetEventLoggerStatus returns the current event logger status.
func GetEventLoggerStatus() types.EventLoggerStatus {
	if globalEventLogger == nil {
		return types.EventLoggerStatus{
			IsRunning:    false,
			DBAccessible: false,
		}
	}

	globalEventLogger.mu.RLock()
	defer globalEventLogger.mu.RUnlock()

	status := types.EventLoggerStatus{
		IsRunning:            globalEventLogger.running,
		TotalEventsProcessed: globalEventLogger.totalProcessed,
		LastError:            globalEventLogger.lastError,
		DBAccessible:         globalEventLogger.db != nil,
		DBPath:               globalEventLogger.dbPath,
	}

	if !globalEventLogger.lastCheckTime.IsZero() {
		s := globalEventLogger.lastCheckTime.Format(time.RFC3339)
		status.LastCheckTime = &s
	}
	if !globalEventLogger.lastEventTime.IsZero() {
		s := globalEventLogger.lastEventTime.Format(time.RFC3339)
		status.LastEventTime = &s
	}

	return status
}

// GetEventLoggerEvents queries events with filtering.
func GetEventLoggerEvents(req types.GetEventsRequest) types.GetEventsResponse {
	resp := types.GetEventsResponse{
		Events: []types.EnhancedEventLogEntry{},
	}

	if req.Limit <= 0 {
		req.Limit = 100
	}

	severityStr := ""
	if req.Severity != "" {
		severityStr = string(req.Severity)
	}

	startStr := ""
	if req.StartTime > 0 {
		startStr = time.UnixMilli(req.StartTime).Format("2006-01-02 15:04:05")
	}
	endStr := ""
	if req.EndTime > 0 {
		endStr = time.UnixMilli(req.EndTime).Format("2006-01-02 15:04:05")
	}

	events, total, err := GetEventLogs(EventLogFilter{
		Limit:     req.Limit,
		Offset:    req.Offset,
		Severity:  severityStr,
		Source:    req.Source,
		Search:    req.Search,
		StartTime: startStr,
		EndTime:   endStr,
	})
	if err != nil {
		return resp
	}

	resp.Events = events
	resp.Total = total
	resp.HasMore = (total > req.Limit+req.Offset)
	return resp
}

// ToRadians is a helper used in various calculations.
func ToRadians(deg float64) float64 {
	return deg * math.Pi / 180
}
