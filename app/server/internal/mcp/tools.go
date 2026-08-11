package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/sushazhi/fnos-logmanager/internal/config"
	"github.com/sushazhi/fnos-logmanager/internal/services"
	"github.com/sushazhi/fnos-logmanager/internal/types"
	"github.com/sushazhi/fnos-logmanager/internal/utils"
)

// ---------------------------------------------------------------------------
// Tool definition
// ---------------------------------------------------------------------------

// tool defines an MCP tool with its input schema.
type tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

// toolHandler executes a tool with the given JSON arguments.
type toolHandler func(args json.RawMessage) (string, error)

var toolRegistry = map[string]toolHandler{}

func registerTool(name, desc string, schema map[string]interface{}, h toolHandler) {
	toolRegistry[name] = h
	toolsList = append(toolsList, tool{
		Name:        name,
		Description: desc,
		InputSchema: schema,
	})
}

var toolsList []tool

// allTools returns a copy of the registered tools.
func allTools() []tool {
	out := make([]tool, len(toolsList))
	copy(out, toolsList)
	return out
}

// invokeTool dispatches a tools/call by name.
func invokeTool(name string, args json.RawMessage) (string, error) {
	h, ok := toolRegistry[name]
	if !ok {
		return "", fmt.Errorf("unknown tool: %s", name)
	}
	return h(args)
}

// ---------------------------------------------------------------------------
// Helpers for argument binding and validation
// ---------------------------------------------------------------------------

// jsonStr returns a pretty-printed JSON string of v.
func jsonStr(v interface{}) string {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	return string(b)
}

// bindArgs parses JSON arguments into dest, returning a tool-friendly error.
func bindArgs(args json.RawMessage, dest interface{}) error {
	if len(args) == 0 || string(args) == "null" {
		return nil
	}
	if err := json.Unmarshal(args, dest); err != nil {
		return fmt.Errorf("参数解析失败: %v", err)
	}
	return nil
}

// objectSchema builds an MCP object input schema from property definitions.
func objectSchema(required []string, properties map[string]map[string]interface{}) map[string]interface{} {
	schema := map[string]interface{}{
		"type":       "object",
		"properties": properties,
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

// strProp builds a string property schema entry.
func strProp(desc string) map[string]interface{} {
	return map[string]interface{}{"type": "string", "description": desc}
}

func intProp(desc string) map[string]interface{} {
	return map[string]interface{}{"type": "integer", "description": desc}
}

func boolProp(desc string) map[string]interface{} {
	return map[string]interface{}{"type": "boolean", "description": desc}
}

func strArrProp(desc string) map[string]interface{} {
	return map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}, "description": desc}
}

// safeLogPath validates that a path is within the allowed log dirs and is safe.
// Returns the normalized path or an error.
func safeLogPath(p string) (string, error) {
	if p == "" {
		return "", fmt.Errorf("缺少路径参数")
	}
	if len(p) > 4096 {
		return "", fmt.Errorf("路径过长")
	}
	if !utils.IsAllowedPath(p, config.Get().LogDirs) {
		return "", fmt.Errorf("路径不在允许的目录范围内: %s", p)
	}
	return p, nil
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// ---------------------------------------------------------------------------
// Tool implementations
// ---------------------------------------------------------------------------

func init() {
	registerTools()
}

func registerTools() {
	// ---- 日志目录与列表 ----
	registerTool("list_dirs", "列出所有已配置/已授权的日志目录及其统计信息",
		objectSchema(nil, map[string]map[string]interface{}{}),
		func(args json.RawMessage) (string, error) {
			dirs, err := services.GetDirsInfo()
			if err != nil {
				return "", err
			}
			return jsonStr(dirs), nil
		})

	registerTool("get_app_names", "列出系统中检测到的应用名称列表（用于日志分类）",
		objectSchema(nil, map[string]map[string]interface{}{}),
		func(args json.RawMessage) (string, error) {
			names, err := services.GetAppNames()
			if err != nil {
				return "", err
			}
			return jsonStr(names), nil
		})

	registerTool("list_logs", "列出指定日志目录（或全部）中的日志文件",
		objectSchema(nil, map[string]map[string]interface{}{
			"dir":   strProp("日志目录路径，留空表示所有配置目录"),
			"limit": intProp("返回条数上限，默认100，最大500"),
		}),
		func(args json.RawMessage) (string, error) {
			var p struct {
				Dir   string `json:"dir"`
				Limit int    `json:"limit"`
			}
			if err := bindArgs(args, &p); err != nil {
				return "", err
			}
			if p.Dir != "" {
				if _, err := safeLogPath(p.Dir); err != nil {
					return "", err
				}
			}
			limit := 100
			if p.Limit > 0 {
				limit = clamp(p.Limit, 1, 500)
			}
			logs, err := services.ListLogFiles(p.Dir, limit)
			if err != nil {
				return "", err
			}
			return jsonStr(map[string]interface{}{"total": len(logs), "logs": logs}), nil
		})

	registerTool("search_logs", "按文件名模式或大小阈值搜索日志文件",
		objectSchema([]string{"type"}, map[string]map[string]interface{}{
			"type":      strProp("搜索类型：name=按文件名模式，size=按大小"),
			"pattern":   strProp("文件名模式（type=name时必填），如 *.log"),
			"threshold": strProp("大小阈值（type=size时可选），如 10M、500M、1G"),
			"limit":     intProp("返回条数上限，默认50，最大200"),
		}),
		func(args json.RawMessage) (string, error) {
			var p struct {
				Type      string `json:"type"`
				Pattern   string `json:"pattern"`
				Threshold string `json:"threshold"`
				Limit     int    `json:"limit"`
			}
			if err := bindArgs(args, &p); err != nil {
				return "", err
			}
			limit := 50
			if p.Limit > 0 {
				limit = clamp(p.Limit, 1, 200)
			}
			var logs []types.LogFile
			var err error
			switch p.Type {
			case "name":
				if p.Pattern == "" || len(p.Pattern) > 100 {
					return "", fmt.Errorf("请输入有效的文件名模式")
				}
				logs, err = services.SearchLogFilesByName(utils.EscapeRegExp(p.Pattern), limit)
			case "size":
				thr := p.Threshold
				if thr == "" {
					thr = "10M"
				}
				if !utils.IsValidThreshold(thr) {
					return "", fmt.Errorf("无效的大小阈值")
				}
				logs, err = services.ListLargeLogFiles(utils.ParseSizeThreshold(thr), limit)
			default:
				return "", fmt.Errorf("无效的搜索类型，应为 name 或 size")
			}
			if err != nil {
				return "", err
			}
			return jsonStr(map[string]interface{}{"total": len(logs), "logs": logs}), nil
		})

	registerTool("get_log_stats", "获取日志系统整体统计（数量、归档、大小）",
		objectSchema(nil, map[string]map[string]interface{}{}),
		func(args json.RawMessage) (string, error) {
			stats, err := services.GetLogStats()
			if err != nil {
				return "", err
			}
			return jsonStr(stats), nil
		})

	// ---- 日志读取 ----
	registerTool("read_log", "读取日志文件内容",
		objectSchema([]string{"path"}, map[string]map[string]interface{}{
			"path":     strProp("日志文件完整路径"),
			"maxLines": intProp("读取行数上限，默认5000，最大200000"),
			"offset":   intProp("起始偏移（行），默认0"),
			"tail":     boolProp("是否从文件尾部读取"),
		}),
		func(args json.RawMessage) (string, error) {
			var p struct {
				Path     string `json:"path"`
				MaxLines int    `json:"maxLines"`
				Offset   int    `json:"offset"`
				Tail     bool   `json:"tail"`
			}
			if err := bindArgs(args, &p); err != nil {
				return "", err
			}
			if _, err := safeLogPath(p.Path); err != nil {
				return "", err
			}
			maxLines := 5000
			if p.MaxLines > 0 {
				maxLines = clamp(p.MaxLines, 100, 200000)
			}
			result, err := services.ReadLogFile(p.Path, types.ReadLogOptions{
				MaxLines: maxLines,
				Offset:   p.Offset,
				Tail:     p.Tail,
			})
			if err != nil {
				return "", err
			}
			return jsonStr(map[string]interface{}{
				"content":       utils.FilterSensitiveInfo(result.Content),
				"totalLines":    result.TotalLines,
				"size":          result.Size,
				"sizeFormatted": result.SizeFormatted,
				"truncated":     result.Truncated,
				"hasMore":       result.HasMore,
			}), nil
		})

	registerTool("tail_log", "从指定字节偏移处读取日志文件增量（用于追踪文件增长）",
		objectSchema([]string{"path"}, map[string]map[string]interface{}{
			"path":   strProp("日志文件完整路径"),
			"offset": intProp("字节偏移量，-1表示从末尾开始，默认-1"),
		}),
		func(args json.RawMessage) (string, error) {
			var p struct {
				Path   string `json:"path"`
				Offset int64  `json:"offset"`
			}
			if err := bindArgs(args, &p); err != nil {
				return "", err
			}
			if _, err := safeLogPath(p.Path); err != nil {
				return "", err
			}
			if p.Offset < 0 {
				p.Offset = -1
			}
			np := utils.SafePath(p.Path)
			info, err := os.Stat(np)
			if err != nil {
				if os.IsNotExist(err) {
					return jsonStr(map[string]interface{}{"content": "", "deleted": true}), nil
				}
				return "", err
			}
			fileSize := info.Size()
			if p.Offset < 0 || p.Offset >= fileSize {
				return jsonStr(map[string]interface{}{"content": "", "offset": fileSize, "totalSize": fileSize}), nil
			}
			const maxBytes = 512 * 1024
			end := p.Offset + maxBytes
			if end > fileSize {
				end = fileSize
			}
			f, err := os.Open(np)
			if err != nil {
				return "", err
			}
			defer f.Close()
			buf := make([]byte, end-p.Offset)
			n, _ := f.ReadAt(buf, p.Offset)
			content := utils.FilterSensitiveInfo(string(buf[:n]))
			return jsonStr(map[string]interface{}{"content": content, "offset": end, "totalSize": fileSize}), nil
		})

	registerTool("read_archive", "读取压缩归档日志文件内容（支持 .gz/.bz2/.xz/.zip/.tar/.7z/.rar）",
		objectSchema([]string{"path"}, map[string]map[string]interface{}{
			"path":  strProp("归档文件完整路径"),
			"lines": intProp("读取行数上限，默认50，最大500"),
		}),
		func(args json.RawMessage) (string, error) {
			var p struct {
				Path  string `json:"path"`
				Lines int    `json:"lines"`
			}
			if err := bindArgs(args, &p); err != nil {
				return "", err
			}
			if _, err := safeLogPath(p.Path); err != nil {
				return "", err
			}
			maxLines := 50
			if p.Lines > 0 {
				maxLines = clamp(p.Lines, 1, 500)
			}
			content, truncated, err := readArchiveContent(p.Path, maxLines)
			if err != nil {
				return "", err
			}
			return jsonStr(map[string]interface{}{
				"content":   utils.FilterSensitiveInfo(content),
				"truncated": truncated,
			}), nil
		})

	// ---- 日志管理 ----
	registerTool("truncate_log", "清空（截断）日志文件内容",
		objectSchema([]string{"path"}, map[string]map[string]interface{}{
			"path": strProp("要清空的日志文件完整路径"),
		}),
		func(args json.RawMessage) (string, error) {
			var p struct {
				Path string `json:"path"`
			}
			if err := bindArgs(args, &p); err != nil {
				return "", err
			}
			if _, err := safeLogPath(p.Path); err != nil {
				return "", err
			}
			if err := services.TruncateLogFile(p.Path); err != nil {
				return "", err
			}
			services.AddSecurityAuditLog("MCP_LOG_TRUNCATE", map[string]interface{}{"path": p.Path}, nil)
			return "日志已清空: " + p.Path, nil
		})

	registerTool("delete_log", "删除日志文件",
		objectSchema([]string{"path"}, map[string]map[string]interface{}{
			"path": strProp("要删除的日志文件完整路径"),
		}),
		func(args json.RawMessage) (string, error) {
			var p struct {
				Path string `json:"path"`
			}
			if err := bindArgs(args, &p); err != nil {
				return "", err
			}
			if _, err := safeLogPath(p.Path); err != nil {
				return "", err
			}
			if err := services.DeleteLogFile(p.Path); err != nil {
				return "", err
			}
			services.AddSecurityAuditLog("MCP_LOG_DELETE", map[string]interface{}{"path": p.Path}, nil)
			return "日志文件已删除: " + p.Path, nil
		})

	registerTool("clean_logs", "按大小或天数批量清理日志文件",
		objectSchema(nil, map[string]map[string]interface{}{
			"threshold": strProp("大小阈值，如 10M、500M、1G；与 days 二选一"),
			"days":      intProp("保留天数，超过该天数的日志将被清理；与 threshold 二选一"),
			"action":    strProp("操作类型：truncate=清空、delete=删除、deleteUninstalled=删除已卸载应用的日志，默认truncate"),
		}),
		func(args json.RawMessage) (string, error) {
			var p struct {
				Threshold string `json:"threshold"`
				Days      *int   `json:"days"`
				Action    string `json:"action"`
			}
			if err := bindArgs(args, &p); err != nil {
				return "", err
			}
			action := p.Action
			if action == "" {
				action = "truncate"
			}
			if !utils.IsValidAction(action) {
				return "", fmt.Errorf("无效的操作类型")
			}
			if action == "deleteUninstalled" {
				res, err := services.CleanUninstalledLogs()
				if err != nil {
					return "", err
				}
				services.AddSecurityAuditLog("MCP_LOGS_CLEAN", map[string]interface{}{"action": action, "cleaned": res.Cleaned}, nil)
				return jsonStr(res), nil
			}
			if p.Threshold == "" && p.Days == nil {
				return "", fmt.Errorf("请指定清理条件（threshold 或 days）")
			}
			var thresholdBytes *int64
			if p.Threshold != "" {
				if !utils.IsValidThreshold(p.Threshold) {
					return "", fmt.Errorf("无效的大小阈值")
				}
				tb := utils.ParseSizeThreshold(p.Threshold)
				thresholdBytes = &tb
			}
			var days *int
			if p.Days != nil {
				d := clamp(*p.Days, 1, 365)
				days = &d
			}
			res, err := services.CleanLogFiles(types.CleanLogOptions{
				ThresholdBytes: thresholdBytes,
				Days:           days,
				Action:         action,
			})
			if err != nil {
				return "", err
			}
			services.AddSecurityAuditLog("MCP_LOGS_CLEAN", map[string]interface{}{"action": action, "cleaned": res.Cleaned}, nil)
			return jsonStr(res), nil
		})

	registerTool("clean_empty_dirs", "清理日志目录下的所有空文件夹",
		objectSchema(nil, map[string]map[string]interface{}{}),
		func(args json.RawMessage) (string, error) {
			result, err := services.CleanEmptyAppDirs()
			if err != nil {
				return "", err
			}
			services.AddSecurityAuditLog("MCP_DIRS_CLEAN_EMPTY", result, nil)
			return jsonStr(result), nil
		})

	// ---- 备份 / 归档 ----
	registerTool("backup_logs", "创建所有日志文件的 tar.gz 备份",
		objectSchema(nil, map[string]map[string]interface{}{}),
		func(args json.RawMessage) (string, error) {
			result := services.PerformBackup()
			if result.Files == 0 {
				msg := "备份失败: 没有找到日志文件"
				if len(result.Errors) > 0 {
					msg = "备份失败: " + result.Errors[len(result.Errors)-1]
				}
				return "", fmt.Errorf("%s", msg)
			}
			services.AddSecurityAuditLog("MCP_LOGS_BACKUP", map[string]interface{}{
				"backupPath": result.BackupPath, "files": result.Files, "size": result.BackupSize,
			}, nil)
			return jsonStr(result), nil
		})

	registerTool("list_backups", "列出所有日志备份",
		objectSchema(nil, map[string]map[string]interface{}{}),
		func(args json.RawMessage) (string, error) {
			backups := services.ListBackups()
			return jsonStr(map[string]interface{}{"total": len(backups), "backups": backups}), nil
		})

	registerTool("delete_backup", "删除一个日志备份文件",
		objectSchema([]string{"path"}, map[string]map[string]interface{}{
			"path": strProp("备份文件完整路径（必须以 .tar.gz 结尾且在备份目录下）"),
		}),
		func(args json.RawMessage) (string, error) {
			var p struct {
				Path string `json:"path"`
			}
			if err := bindArgs(args, &p); err != nil {
				return "", err
			}
			safePath := utils.SafePath(p.Path)
			cfg := config.Get()
			if safePath == "" || !strings.HasPrefix(safePath, cfg.Backup.BaseDir) || !strings.HasSuffix(safePath, ".tar.gz") {
				return "", fmt.Errorf("只能删除备份目录下的 .tar.gz 文件")
			}
			if err := services.DeleteBackup(p.Path); err != nil {
				return "", err
			}
			services.AddSecurityAuditLog("MCP_BACKUP_DELETE", map[string]interface{}{"path": p.Path}, nil)
			return "备份已删除: " + p.Path, nil
		})

	registerTool("clean_backups", "清理超过指定天数的旧备份",
		objectSchema(nil, map[string]map[string]interface{}{
			"days": intProp("保留天数，默认30，最大365"),
		}),
		func(args json.RawMessage) (string, error) {
			var p struct {
				Days int `json:"days"`
			}
			if err := bindArgs(args, &p); err != nil {
				return "", err
			}
			days := 30
			if p.Days > 0 {
				days = clamp(p.Days, 1, 365)
			}
			deleted, err := services.CleanOldBackups(days)
			if err != nil {
				return "", err
			}
			services.AddSecurityAuditLog("MCP_BACKUPS_CLEAN", map[string]interface{}{"days": days, "deleted": deleted}, nil)
			return fmt.Sprintf("已删除 %d 个超过 %d 天的旧备份", deleted, days), nil
		})

	registerTool("list_archives", "列出归档日志文件",
		objectSchema(nil, map[string]map[string]interface{}{
			"limit": intProp("返回条数上限，默认50，最大200"),
		}),
		func(args json.RawMessage) (string, error) {
			var p struct {
				Limit int `json:"limit"`
			}
			if err := bindArgs(args, &p); err != nil {
				return "", err
			}
			limit := 50
			if p.Limit > 0 {
				limit = clamp(p.Limit, 1, 200)
			}
			archives, err := services.ListArchiveFiles(limit)
			if err != nil {
				return "", err
			}
			return jsonStr(map[string]interface{}{"total": len(archives), "archives": archives}), nil
		})

	// ---- Docker ----
	registerTool("list_docker_containers", "列出所有 Docker 容器",
		objectSchema(nil, map[string]map[string]interface{}{}),
		func(args json.RawMessage) (string, error) {
			containers, err := listDockerContainers()
			if err != nil {
				return "", err
			}
			return jsonStr(containers), nil
		})

	registerTool("get_docker_logs", "获取指定 Docker 容器的日志",
		objectSchema([]string{"container"}, map[string]map[string]interface{}{
			"container": strProp("Docker 容器名称"),
			"lines":     intProp("返回最后 N 行，可选"),
		}),
		func(args json.RawMessage) (string, error) {
			var p struct {
				Container string `json:"container"`
				Lines     int    `json:"lines"`
			}
			if err := bindArgs(args, &p); err != nil {
				return "", err
			}
			if !utils.IsValidContainerName(p.Container) {
				return "", fmt.Errorf("无效的容器名称")
			}
			cfg := config.Get().Docker
			var cmdArgs []string
			if p.Lines > 0 {
				if p.Lines > cfg.MaxLogLines {
					p.Lines = cfg.MaxLogLines
				}
				cmdArgs = []string{"logs", p.Container, "--tail", fmt.Sprintf("%d", p.Lines)}
			} else {
				cmdArgs = []string{"logs", p.Container}
			}
			out, err := execDocker(cmdArgs, cfg.LogsTimeoutMs, cfg.MaxOutputBytes)
			if err != nil {
				return "", err
			}
			return utils.FilterSensitiveInfo(out), nil
		})

	// ---- 事件日志 ----
	registerTool("get_event_logs", "查询系统事件日志（支持按严重级别、来源、关键字、时间过滤）",
		objectSchema(nil, map[string]map[string]interface{}{
			"limit":     intProp("返回条数上限，默认100，最大1000"),
			"offset":    intProp("起始偏移，默认0"),
			"severity":  strProp("严重级别过滤：debug/info/warning/error/critical"),
			"source":    strProp("来源过滤"),
			"eventType": strProp("事件类型过滤"),
			"search":    strProp("消息关键字搜索"),
			"startTime": strProp("开始时间（RFC3339）"),
			"endTime":   strProp("结束时间（RFC3339）"),
		}),
		func(args json.RawMessage) (string, error) {
			var p struct {
				Limit     int    `json:"limit"`
				Offset    int    `json:"offset"`
				Severity  string `json:"severity"`
				Source    string `json:"source"`
				EventType string `json:"eventType"`
				Search    string `json:"search"`
				StartTime string `json:"startTime"`
				EndTime   string `json:"endTime"`
			}
			if err := bindArgs(args, &p); err != nil {
				return "", err
			}
			if p.Limit <= 0 {
				p.Limit = 100
			}
			p.Limit = clamp(p.Limit, 1, 1000)
			entries, total, err := services.GetEventLogs(services.EventLogFilter{
				Limit:     p.Limit,
				Offset:    p.Offset,
				Severity:  p.Severity,
				Source:    p.Source,
				EventType: p.EventType,
				Search:    p.Search,
				StartTime: p.StartTime,
				EndTime:   p.EndTime,
			})
			if err != nil {
				return "", err
			}
			if entries == nil {
				entries = []types.EnhancedEventLogEntry{}
			}
			return jsonStr(map[string]interface{}{"total": total, "events": entries}), nil
		})

	registerTool("get_event_sources", "列出事件日志的所有来源",
		objectSchema(nil, map[string]map[string]interface{}{}),
		func(args json.RawMessage) (string, error) {
			sources := services.GetEventSources()
			if sources == nil {
				sources = []string{}
			}
			return jsonStr(sources), nil
		})

	registerTool("event_logger_status", "获取事件日志服务运行状态",
		objectSchema(nil, map[string]map[string]interface{}{}),
		func(args json.RawMessage) (string, error) {
			status := services.GetEventLoggerStatus()
			return jsonStr(status), nil
		})

	// ---- 内核管理 ----
	registerTool("list_kernels", "列出系统已安装的内核版本及占用空间",
		objectSchema(nil, map[string]map[string]interface{}{}),
		func(args json.RawMessage) (string, error) {
			return listKernels(), nil
		})

	registerTool("remove_kernel", "删除指定版本的内核（当前内核不可删除）",
		objectSchema([]string{"version"}, map[string]map[string]interface{}{
			"version": strProp("要删除的内核版本号"),
		}),
		func(args json.RawMessage) (string, error) {
			var p struct {
				Version string `json:"version"`
			}
			if err := bindArgs(args, &p); err != nil {
				return "", err
			}
			current, err := getCurrentKernel()
			if err == nil && p.Version == current {
				return "", fmt.Errorf("不能卸载当前正在使用的内核")
			}
			if !isValidKernelVersion(p.Version) {
				return "", fmt.Errorf("无效的内核版本号")
			}
			if err := removeKernelVersion(p.Version); err != nil {
				return "", fmt.Errorf("删除内核 %s 失败: %v", p.Version, err)
			}
			services.AddSecurityAuditLog("MCP_KERNEL_REMOVE", map[string]interface{}{"version": p.Version}, nil)
			return "内核 " + p.Version + " 已删除", nil
		})

	registerTool("cleanup_kernels", "删除所有非当前内核版本以释放空间",
		objectSchema(nil, map[string]map[string]interface{}{}),
		func(args json.RawMessage) (string, error) {
			current, err := getCurrentKernel()
			if err != nil {
				return "", err
			}
			versions, err := getInstalledKernels(current)
			if err != nil {
				return "", err
			}
			removed := 0
			var errors []string
			var freed int64
			for _, v := range versions {
				if v.IsCurrent {
					continue
				}
				if err := removeKernelVersion(v.Version); err != nil {
					errors = append(errors, fmt.Sprintf("%s: %v", v.Version, err))
					continue
				}
				freed += v.TotalSize
				removed++
			}
			services.AddSecurityAuditLog("MCP_KERNEL_CLEANUP", map[string]interface{}{"removed": removed, "freedSize": freed}, nil)
			return jsonStr(map[string]interface{}{
				"removed": removed, "errors": errors,
				"freedSize": freed, "freedSizeFormatted": formatBytes(freed),
			}), nil
		})

	// ---- 系统信息 / 工具 ----
	// 说明：fnOS 系统语言/版本/主题由前端 JS SDK getPlatformConfig() 获取（无需 scope），
	// 后端 trim.system.getPlatformConfig 在部分安装下 403 且非核心功能所需，故不再提供该 MCP 工具。

	registerTool("convert_path", "将内部路径转换为语义化展示路径（如 /vol1/@appdata/xxx 转换为 存储空间1/xxx）。trim API 不可用时返回原始路径。",
		objectSchema([]string{"path"}, map[string]map[string]interface{}{
			"path": strProp("内部路径，如 /vol1/@appdata/xxx"),
		}),
		func(args json.RawMessage) (string, error) {
			var p struct {
				Path string `json:"path"`
			}
			if err := bindArgs(args, &p); err != nil {
				return "", err
			}
			if p.Path == "" {
				return "", fmt.Errorf("缺少路径参数")
			}
			client := services.GetTrimClient()
			semanticPath, err := client.ConvertPath(p.Path, "")
			if err != nil || semanticPath == "" {
				semanticPath = p.Path
			}
			return jsonStr(map[string]interface{}{"path": p.Path, "semanticPath": semanticPath}), nil
		})

	registerTool("get_audit_logs", "获取应用操作审计日志",
		objectSchema(nil, map[string]map[string]interface{}{
			"limit": intProp("返回条数上限，默认100"),
		}),
		func(args json.RawMessage) (string, error) {
			var p struct {
				Limit int `json:"limit"`
			}
			if err := bindArgs(args, &p); err != nil {
				return "", err
			}
			limit := 100
			if p.Limit > 0 {
				limit = clamp(p.Limit, 1, 1000)
			}
			services.CleanOldAuditLogs()
			logs, err := services.GetAuditLogs(limit)
			if err != nil {
				return "", err
			}
			return jsonStr(logs), nil
		})

	registerTool("get_app_version", "获取应用当前版本信息",
		objectSchema(nil, map[string]map[string]interface{}{}),
		func(args json.RawMessage) (string, error) {
			version := os.Getenv("TRIM_APPVER")
			if version == "" {
				version = "0.0.0"
			}
			return jsonStr(map[string]interface{}{
				"version": version,
				"appName": "飞牛日志管理",
				"arch":    os.Getenv("TRIM_ARCH"),
			}), nil
		})
}

// ---------------------------------------------------------------------------
// Docker helpers
// ---------------------------------------------------------------------------

type dockerContainer struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Image  string `json:"image"`
}

func execDocker(args []string, timeoutMs int64, maxBytes int64) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutMs)*time.Millisecond)
	defer cancel()
	dockerPath, err := exec.LookPath("docker")
	if err != nil {
		return "", fmt.Errorf("Docker 命令未找到: %v", err)
	}
	cmd := exec.CommandContext(ctx, dockerPath, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("Docker 命令执行超时")
		}
		return "", fmt.Errorf("Docker 命令执行失败: %s", strings.TrimSpace(stderr.String()))
	}
	if int64(stdout.Len()+stderr.Len()) > maxBytes {
		return "", fmt.Errorf("Docker 输出过大")
	}
	return stdout.String() + stderr.String(), nil
}

func listDockerContainers() ([]dockerContainer, error) {
	cfg := config.Get().Docker
	out, err := execDocker([]string{"ps", "--format", "{{.Names}}\t{{.Status}}\t{{.Image}}"}, cfg.ListTimeoutMs, cfg.MaxOutputBytes)
	if err != nil {
		return nil, fmt.Errorf("Docker 未安装或未运行: %v", err)
	}
	var containers []dockerContainer
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 3)
		name := ""
		if len(parts) > 0 {
			name = parts[0]
		}
		if !utils.IsValidContainerName(name) {
			continue
		}
		c := dockerContainer{Name: name}
		if len(parts) > 1 {
			c.Status = parts[1]
		}
		if len(parts) > 2 {
			c.Image = parts[2]
		}
		containers = append(containers, c)
	}
	return containers, nil
}

// ---------------------------------------------------------------------------
// Archive read helper (mirrors routes.readArchiveContent)
// ---------------------------------------------------------------------------

func readArchiveContent(path string, maxLines int) (string, bool, error) {
	np := utils.SafePath(path)
	if np == "" {
		return "", false, fmt.Errorf("无效的文件路径")
	}
	lower := strings.ToLower(np)
	var cmd *exec.Cmd
	switch {
	case strings.HasSuffix(lower, ".gz") || strings.HasSuffix(lower, ".tgz"):
		cmd = exec.Command("zcat", np)
	case strings.HasSuffix(lower, ".bz2"):
		cmd = exec.Command("bzcat", np)
	case strings.HasSuffix(lower, ".xz"):
		cmd = exec.Command("xzcat", np)
	case strings.HasSuffix(lower, ".zip"):
		cmd = exec.Command("unzip", "-p", np)
	case strings.HasSuffix(lower, ".tar") || strings.HasSuffix(lower, ".tar.gz") || strings.HasSuffix(lower, ".tar.bz2") || strings.HasSuffix(lower, ".tar.xz"):
		cmd = exec.Command("tar", "-xOf", np)
	case strings.HasSuffix(lower, ".7z"):
		cmd = exec.Command("7z", "x", "-so", np)
	case strings.HasSuffix(lower, ".rar"):
		cmd = exec.Command("unrar", "p", np)
	default:
		f, err := os.Open(np)
		if err != nil {
			return "", false, err
		}
		defer f.Close()
		scanner := bufio.NewScanner(f)
		scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
		var lines []string
		for scanner.Scan() && len(lines) < maxLines {
			lines = append(lines, scanner.Text())
		}
		truncated := false
		if scanner.Scan() {
			truncated = true
		}
		return strings.Join(lines, "\n"), truncated, nil
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", false, err
	}
	if err := cmd.Start(); err != nil {
		return "", false, err
	}
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	var lines []string
	for scanner.Scan() && len(lines) < maxLines {
		lines = append(lines, scanner.Text())
	}
	truncated := false
	if scanner.Scan() {
		truncated = true
	}
	// Drain
	for scanner.Scan() {
	}
	cmd.Wait()
	return strings.Join(lines, "\n"), truncated, nil
}

// ---------------------------------------------------------------------------
// Kernel helpers (mirror routes/kernel.go logic)
// ---------------------------------------------------------------------------

type kernelInfo struct {
	Version        string `json:"version"`
	IsCurrent      bool   `json:"isCurrent"`
	TotalSize      int64  `json:"totalSize"`
	TotalSizeFormatted string `json:"totalSizeFormatted"`
}

func listKernels() string {
	current, err := getCurrentKernel()
	if err != nil {
		current = ""
	}
	versions, err := getInstalledKernels(current)
	if err != nil {
		return jsonStr(map[string]interface{}{"error": "无法读取内核版本信息", "versions": []interface{}{}})
	}
	total := int64(0)
	for _, v := range versions {
		total += v.TotalSize
	}
	return jsonStr(map[string]interface{}{
		"versions": versions, "total": len(versions), "current": current,
		"totalSize": total, "totalSizeFormatted": formatBytes(total),
	})
}

func getCurrentKernel() (string, error) {
	cmd := exec.Command("uname", "-r")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		data, err := os.ReadFile("/proc/sys/kernel/osrelease")
		if err != nil {
			return "", fmt.Errorf("无法获取当前内核版本: %v", err)
		}
		return strings.TrimSpace(string(data)), nil
	}
	return strings.TrimSpace(out.String()), nil
}

func getInstalledKernels(current string) ([]kernelInfo, error) {
	versionSet := make(map[string]bool)
	if entries, err := os.ReadDir("/boot"); err == nil {
		for _, e := range entries {
			name := e.Name()
			if strings.HasPrefix(name, "vmlinuz-") {
				versionSet[strings.TrimPrefix(name, "vmlinuz-")] = true
			} else if strings.HasPrefix(name, "initrd.img-") {
				versionSet[strings.TrimPrefix(name, "initrd.img-")] = true
			}
		}
	}
	if entries, err := os.ReadDir("/lib/modules"); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				versionSet[e.Name()] = true
			}
		}
	}
	if entries, err := os.ReadDir("/usr/src"); err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			name := e.Name()
			for _, prefix := range []string{"linux-headers-", "linux-kbuild-", "linux-hwe-"} {
				if rest, ok := strings.CutPrefix(name, prefix); ok {
					versionSet[rest] = true
					break
				}
			}
			if name != "." && name != ".." && !strings.HasPrefix(name, "linux-headers-") {
				versionSet[name] = true
			}
		}
	}
	if len(versionSet) == 0 {
		return nil, fmt.Errorf("未发现已安装的内核")
	}
	var versions []kernelInfo
	for ver := range versionSet {
		size := calcDirSize(filepath.Join("/lib/modules", ver)) + calcSrcSize(ver) + calcBootSize(ver)
		versions = append(versions, kernelInfo{
			Version: ver, IsCurrent: ver == current,
			TotalSize: size, TotalSizeFormatted: formatBytes(size),
		})
	}
	sort.Slice(versions, func(i, j int) bool { return kernelCompare(versions[i].Version, versions[j].Version) > 0 })
	return versions, nil
}

func calcBootSize(version string) int64 {
	var total int64
	prefixes := []string{"vmlinuz-", "initrd.img-", "config-", "System.map-"}
	if entries, err := os.ReadDir("/boot"); err == nil {
		for _, e := range entries {
			name := e.Name()
			for _, p := range prefixes {
				if strings.HasPrefix(name, p+version) || name == p+version {
					if fi, err := e.Info(); err == nil {
						total += fi.Size()
					}
					break
				}
			}
		}
	}
	return total
}

func calcSrcSize(version string) int64 {
	var total int64
	if entries, err := os.ReadDir("/usr/src"); err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			name := e.Name()
			if name == version || strings.HasSuffix(name, "-"+version) ||
				strings.HasSuffix(name, "-"+version+"-common") ||
				strings.HasPrefix(name, "linux-headers-"+version) ||
				strings.HasPrefix(name, "linux-kbuild-"+version) {
				total += calcDirSize(filepath.Join("/usr/src", name))
			}
		}
	}
	return total
}

func calcDirSize(path string) int64 {
	var total int64
	filepath.Walk(path, func(_ string, fi os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !fi.IsDir() {
			total += fi.Size()
		}
		return nil
	})
	return total
}

func isValidKernelVersion(version string) bool {
	if version == "" || version == "." || version == ".." {
		return false
	}
	for _, r := range version {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' || r == '-' || r == '_') {
			return false
		}
	}
	if filepath.Base(version) != version || strings.ContainsAny(version, `/\\`) {
		return false
	}
	return strings.ContainsAny(version, "0123456789")
}

func removeKernelVersion(version string) error {
	if !isValidKernelVersion(version) {
		return fmt.Errorf("无效的内核版本号")
	}
	var errs []string
	if entries, err := os.ReadDir("/boot"); err == nil {
		for _, e := range entries {
			name := e.Name()
			if strings.HasSuffix(name, "-"+version) || strings.Contains(name, "-"+version+"-") {
				if err := os.Remove(filepath.Join("/boot", name)); err != nil {
					errs = append(errs, err.Error())
				}
			}
		}
	}
	if fi, err := os.Lstat(filepath.Join("/lib/modules", version)); err == nil && fi.IsDir() {
		if err := os.RemoveAll(filepath.Join("/lib/modules", version)); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if entries, err := os.ReadDir("/usr/src"); err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			name := e.Name()
			match := name == version || strings.HasSuffix(name, "-"+version) ||
				strings.HasSuffix(name, "-"+version+"-common") ||
				strings.HasPrefix(name, "linux-headers-"+version) ||
				strings.HasPrefix(name, "linux-kbuild-"+version)
			if match {
				if err := os.RemoveAll(filepath.Join("/usr/src", name)); err != nil {
					errs = append(errs, err.Error())
				}
			}
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	return nil
}

func kernelCompare(a, b string) int {
	clean := func(v string) string {
		parts := strings.Split(v, "-")
		if len(parts) == 0 {
			return v
		}
		return parts[0]
	}
	pa, pb := clean(a), clean(b)
	sa, sb := strings.Split(pa, "."), strings.Split(pb, ".")
	ml := len(sa)
	if len(sb) > ml {
		ml = len(sb)
	}
	for i := 0; i < ml; i++ {
		var na, nb int
		if i < len(sa) {
			fmt.Sscanf(sa[i], "%d", &na)
		}
		if i < len(sb) {
			fmt.Sscanf(sb[i], "%d", &nb)
		}
		if na != nb {
			return na - nb
		}
	}
	return 0
}

func formatBytes(bytes int64) string {
	if bytes == 0 {
		return "0 B"
	}
	units := []string{"B", "KB", "MB", "GB"}
	i := 0
	size := float64(bytes)
	for size >= 1024 && i < len(units)-1 {
		size /= 1024
		i++
	}
	if i == 0 {
		return fmt.Sprintf("%.0f %s", size, units[i])
	}
	return fmt.Sprintf("%.1f %s", size, units[i])
}
