package types

import "time"

// LogFile represents a log file entry.
type LogFile struct {
	Path          string    `json:"path"`
	Size          int64     `json:"size"`
	SizeFormatted string    `json:"sizeFormatted"`
	Modified      time.Time `json:"modified"`
	AppName       *string   `json:"appName,omitempty"`
	CanDelete     bool      `json:"canDelete,omitempty"`
	IsArchive     bool      `json:"isArchive,omitempty"`
}

// ArchiveFile represents an archived log file.
type ArchiveFile struct {
	Path          string    `json:"path"`
	Size          int64     `json:"size"`
	SizeFormatted string    `json:"sizeFormatted"`
	Modified      time.Time `json:"modified"`
	Type          string    `json:"type"`
}

// LogStats represents aggregated log statistics.
type LogStats struct {
	TotalLogs        int    `json:"totalLogs"`
	TotalArchives    int    `json:"totalArchives"`
	LargeFiles       int    `json:"largeFiles"`
	TotalSize        int64  `json:"totalSize"`
	TotalSizeFormatted string `json:"totalSizeFormatted"`
}

// DirInfo represents information about a log directory.
type DirInfo struct {
	Path         string `json:"path"`
	Exists       bool   `json:"exists"`
	LogCount     int    `json:"logCount,omitempty"`
	ArchiveCount int    `json:"archiveCount,omitempty"`
	TotalSize    string `json:"totalSize,omitempty"`
	Error        string `json:"error,omitempty"`
}

// BackupInfo represents a backup entry.
type BackupInfo struct {
	Name    string    `json:"name"`
	Path    string    `json:"path"`
	Size    string    `json:"size"`
	Created time.Time `json:"created"`
}

// BackupResult represents the result of a backup operation.
type BackupResult struct {
	BackupPath string   `json:"backupPath"`
	Files      int      `json:"files"`
	BackupSize string   `json:"backupSize"`
	Errors     []string `json:"errors,omitempty"`
}

// AuditLogEntry represents an audit log entry.
type AuditLogEntry struct {
	Timestamp string                 `json:"timestamp"`
	Action    string                 `json:"action"`
	Details   map[string]interface{} `json:"details"`
	IP        string                 `json:"ip"`
	UserAgent string                 `json:"userAgent"`
}

// Session represents a user session.
type Session struct {
	Username   string `json:"username"`
	CreatedAt  int64  `json:"createdAt"`
	LastAccess int64  `json:"lastAccess"`
}

// CSRFToken represents a CSRF token.
type CSRFToken struct {
	Token     string `json:"token"`
	CreatedAt int64  `json:"createdAt"`
}

// RateLimitRecord represents a rate limit entry.
type RateLimitRecord struct {
	Count     int   `json:"count"`
	ResetTime int64 `json:"resetTime"`
}

// ReadLogOptions represents options for reading a log file.
type ReadLogOptions struct {
	MaxLines int  `json:"maxLines,omitempty"`
	Offset   int  `json:"offset,omitempty"`
	Tail     bool `json:"tail,omitempty"`
}

// ReadLogResult represents the result of reading a log file.
type ReadLogResult struct {
	Content        string `json:"content"`
	TotalLines     int    `json:"totalLines"`
	Size           int64  `json:"size"`
	SizeFormatted  string `json:"sizeFormatted"`
	Truncated      bool   `json:"truncated"`
	HasMore        bool   `json:"hasMore"`
}

// CleanLogOptions represents options for cleaning log files.
type CleanLogOptions struct {
	ThresholdBytes *int64
	Days           *int
	Action         string // "truncate", "delete", "deleteUninstalled"
}

// CleanLogResult represents the result of a log cleaning operation.
type CleanLogResult struct {
	Cleaned int      `json:"cleaned"`
	Errors  []string `json:"errors,omitempty"`
}

// DockerContainer represents a Docker container.
type DockerContainer struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Image  string `json:"image"`
}

// UpdateStatus represents the current update status.
type UpdateStatus struct {
	Ready          string `json:"ready"`
	Updating       bool   `json:"updating"`
	UpdateProgress int    `json:"updateProgress"`
	UpdateMessage  string `json:"updateMessage"`
}

// GitHubRelease represents a GitHub release.
type GitHubRelease struct {
	Version     string        `json:"version"`
	Changelog   string        `json:"changelog"`
	PublishedAt string        `json:"publishedAt"`
	Assets      []GitHubAsset `json:"assets"`
}

// GitHubAsset represents a GitHub release asset.
type GitHubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               *int64 `json:"size,omitempty"`
}

// EventLoggerConfig holds configuration for the event logger.
type EventLoggerConfig struct {
	DBPath              string   `json:"dbPath"`
	Enabled             bool     `json:"enabled"`
	CheckInterval       int      `json:"checkInterval"`
	EventTypes          []string `json:"eventTypes"`
	MinSeverity         string   `json:"minSeverity"`
	NotificationChannels []string `json:"notificationChannels"`
}

// EventSeverity represents the severity level of a system event.
type EventSeverity string

const (
	SeverityDebug    EventSeverity = "debug"
	SeverityInfo     EventSeverity = "info"
	SeverityWarning  EventSeverity = "warning"
	SeverityError    EventSeverity = "error"
	SeverityCritical EventSeverity = "critical"
)

// SeverityOrder maps severity levels to numeric values for comparison.
var SeverityOrder = map[EventSeverity]int{
	SeverityDebug:    0,
	SeverityInfo:     1,
	SeverityWarning:  2,
	SeverityError:    3,
	SeverityCritical: 4,
}

// EnhancedEventLogEntry represents a detailed system event log entry.
type EnhancedEventLogEntry struct {
	ID        int64          `json:"id"`
	Timestamp string         `json:"timestamp"`
	Source    string         `json:"source,omitempty"`
	EventType string         `json:"eventType,omitempty"`
	Severity  EventSeverity  `json:"severity,omitempty"`
	Message   string         `json:"message,omitempty"`
	Template  string         `json:"template,omitempty"`
	Param     string         `json:"param,omitempty"`
	Metadata  interface{}    `json:"metadata,omitempty"`
	User      string         `json:"user,omitempty"`
	EventCode string         `json:"eventCode,omitempty"`
	Cat       int            `json:"cat,omitempty"`
	Raw       map[string]interface{} `json:"-"`
}

// EventLoggerStatus represents the current state of the event logger.
type EventLoggerStatus struct {
	IsRunning            bool    `json:"isRunning"`
	LastCheckTime        *string `json:"lastCheckTime,omitempty"`
	LastEventTime        *string `json:"lastEventTime,omitempty"`
	TotalEventsProcessed int     `json:"totalEventsProcessed"`
	LastError            string  `json:"lastError,omitempty"`
	DBAccessible         bool    `json:"dbAccessible"`
	DBPath               string  `json:"dbPath"`
}

// EventLoggerStats represents event logger statistics.
type EventLoggerStats struct {
	TotalEvents  int                   `json:"totalEvents"`
	RecentEvents []EnhancedEventLogEntry `json:"recentEvents"`
}

// GetEventsRequest represents a request to query event logs.
type GetEventsRequest struct {
	Limit         int             `json:"limit,omitempty"`
	Offset        int             `json:"offset,omitempty"`
	Severity      EventSeverity   `json:"severity,omitempty"`
	Source        string          `json:"source,omitempty"`
	Template      string          `json:"template,omitempty"`
	Search        string          `json:"search,omitempty"`
	StartTime     int64           `json:"startTime,omitempty"`
	EndTime       int64           `json:"endTime,omitempty"`
	SortDirection string          `json:"sortDirection,omitempty"` // "asc" or "desc"
}

// GetEventsResponse represents the response to a GetEventsRequest.
type GetEventsResponse struct {
	Events  []EnhancedEventLogEntry `json:"events"`
	Total   int                     `json:"total"`
	HasMore bool                    `json:"hasMore"`
}

// AutoCleanRule represents an auto-clean rule.
type AutoCleanRule struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Enabled  bool   `json:"enabled"`
	Type     string `json:"type"` // "truncateLarge", "deleteOld", "deleteUninstalled"
	Threshold string `json:"threshold,omitempty"`
	Days     *int   `json:"days,omitempty"`
	Schedule string `json:"schedule"`
	LastRun  string `json:"lastRun,omitempty"`
}
