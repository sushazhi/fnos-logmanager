package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

// Config holds all application configuration.
type Config struct {
	Port int `json:"port"`
	DataDir string `json:"dataDir"`
	SessionExpiry int64 `json:"sessionExpiry"` // milliseconds
	RateLimit RateLimitConfig `json:"rateLimit"`
	Auth AuthConfig `json:"auth"`
	Audit AuditConfig `json:"audit"`
	CSRF CSRFConfig `json:"csrf"`
	Logging LoggingConfig `json:"logging"`
	Update UpdateConfig `json:"update"`
	Docker DockerConfig `json:"docker"`
	Archive ArchiveConfig `json:"archive"`
	LogFile LogFileConfig `json:"logFile"`
	Backup BackupConfig `json:"backup"`
	LogDirs []string `json:"logDirs"`
	Notify NotifyConfig `json:"notify"`
	EventLogger EventLoggerConfig `json:"eventLogger"`
	SensitivePatterns []string `json:"sensitivePatterns"`
	GatewaySocket string `json:"-"`
	GatewayPrefix string `json:"-"`
	AuditLogFile string `json:"auditLogFile"`
}

type RateLimitConfig struct {
	WindowMs    int64 `json:"windowMs"`
	MaxRequests int   `json:"maxRequests"`
}

type AuthConfig struct {
	AllowQueryToken bool     `json:"allowQueryToken"`
	QueryTokenPaths []string `json:"queryTokenPaths"`
}

type AuditConfig struct {
	MaxLogs int `json:"maxLogs"`
}

type CSRFConfig struct {
	Expiry int64 `json:"expiry"` // milliseconds
}

type LoggingConfig struct {
	RedactQuery bool `json:"redactQuery"`
}

type UpdateConfig struct {
	CheckCacheMs     int64    `json:"checkCacheMs"`
	DownloadTimeoutMs int64   `json:"downloadTimeoutMs"`
	MaxDownloadBytes  int64   `json:"maxDownloadBytes"`
	MaxAssetBytes     int64   `json:"maxAssetBytes"`
	MaxRedirects      int     `json:"maxRedirects"`
	AllowedHosts      []string `json:"allowedHosts"`
	AllowedUpdateDirs []string `json:"allowedUpdateDirs"`
}

type DockerConfig struct {
	ListTimeoutMs  int64 `json:"listTimeoutMs"`
	LogsTimeoutMs  int64 `json:"logsTimeoutMs"`
	MaxLogLines    int   `json:"maxLogLines"`
	MaxOutputBytes int64 `json:"maxOutputBytes"`
}

type ArchiveConfig struct {
	MaxPreviewLines int   `json:"maxPreviewLines"`
	MaxArchiveBytes int64 `json:"maxArchiveBytes"`
	MaxOutputBytes  int64 `json:"maxOutputBytes"`
}

type LogFileConfig struct {
	MaxPreviewLines int   `json:"maxPreviewLines"`
	MaxPreviewBytes int64 `json:"maxPreviewBytes"`
}

type BackupConfig struct {
	BaseDir         string `json:"baseDir"`
	MaxFiles        int    `json:"maxFiles"`
	MaxFileSizeBytes int64 `json:"maxFileSizeBytes"`
	MaxTotalBytes   int64  `json:"maxTotalBytes"`
}

type NotifyConfig struct {
	Enabled    bool   `json:"enabled"`
	WebhookURL string `json:"webhookUrl"`
	SMTPHost   string `json:"smtpHost"`
	SMTPPort   int    `json:"smtpPort"`
	SMTPUser   string `json:"smtpUser"`
	SMTPPass   string `json:"smtpPass"`
	SMTPFrom   string `json:"smtpFrom"`
	SMTPTo     string `json:"smtpTo"`
}

type EventLoggerConfig struct {
	DBPath              string   `json:"dbPath"`
	Enabled             bool     `json:"enabled"`
	CheckInterval       int      `json:"checkInterval"`
	EventTypes          []string `json:"eventTypes"`
	MinSeverity         string   `json:"minSeverity"`
	NotificationChannels []string `json:"notificationChannels"`
}

var (
	cfg  *Config
	once sync.Once
)

// Get returns the singleton configuration.
func Get() *Config {
	once.Do(func() {
		cfg = loadConfig()
	})
	return cfg
}

func getEnvOrDefault(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return defaultVal
}

func getEnvInt64(key string, defaultVal int64) int64 {
	if val := os.Getenv(key); val != "" {
		if i, err := strconv.ParseInt(val, 10, 64); err == nil {
			return i
		}
	}
	return defaultVal
}

func getEnvBool(key string, defaultVal bool) bool {
	if val := os.Getenv(key); val != "" {
		return val == "true"
	}
	return defaultVal
}

func loadConfig() *Config {
	dataDir := getEnvOrDefault("LOGMANAGER_DATA_DIR", "/vol1/@appdata/logmanager")
	gatewaySocket := os.Getenv("GATEWAY_SOCKET")

	// Load config from JSON file for overrides
	var fileConfig struct {
		LogDirs     []string `json:"log_dirs"`
		EventLogger *struct {
			Enabled             *bool    `json:"enabled"`
			DBPath              *string  `json:"dbPath"`
			CheckInterval       *int     `json:"checkInterval"`
			EventTypes          []string `json:"eventTypes"`
			MinSeverity         *string  `json:"minSeverity"`
			NotificationChannels []string `json:"notificationChannels"`
		} `json:"event_logger"`
	}

	configFilePath := filepath.Join(dataDir, "config", "config.json")
	if data, err := os.ReadFile(configFilePath); err == nil {
		json.Unmarshal(data, &fileConfig)
	}

	// Default backup dir
	backupDir := getEnvOrDefault("BACKUP_BASE_DIR", "/vol1/@appshare/logmanager/backup")
	if fileConfig.EventLogger != nil {
		// Could read backup_dir from config file too
	}

	cfg := &Config{
		Port:           getEnvInt("PORT", 8090),
		DataDir:        dataDir,
		SessionExpiry:  int64(getEnvInt("SESSION_EXPIRY", 86400000)),
		GatewaySocket:  gatewaySocket,
		GatewayPrefix:  "/app/logmanager",
		AuditLogFile:   filepath.Join(dataDir, "audit.log"),
		RateLimit: RateLimitConfig{
			WindowMs:    60000,
			MaxRequests: 300,
		},
		Auth: AuthConfig{
			AllowQueryToken: getEnvBool("ALLOW_QUERY_TOKEN", false),
			QueryTokenPaths: []string{"/api/auth/status", "/api/auth/csrf-token"},
		},
		Audit: AuditConfig{
			MaxLogs: 1000,
		},
		CSRF: CSRFConfig{
			Expiry: 2 * 60 * 60 * 1000, // 2 hours
		},
		Logging: LoggingConfig{
			RedactQuery: getEnvBool("LOG_REDACT_QUERY", true),
		},
		Update: UpdateConfig{
			CheckCacheMs:      getEnvInt64("UPDATE_CHECK_CACHE_MS", 60000),
			DownloadTimeoutMs: getEnvInt64("UPDATE_DOWNLOAD_TIMEOUT_MS", 300000),
			MaxDownloadBytes:  getEnvInt64("UPDATE_MAX_DOWNLOAD_BYTES", 200*1024*1024),
			MaxAssetBytes:     getEnvInt64("UPDATE_MAX_ASSET_BYTES", 500*1024*1024),
			MaxRedirects:      getEnvInt("UPDATE_MAX_REDIRECTS", 3),
			AllowedHosts:      []string{"github.com", "api.github.com", "objects.githubusercontent.com", "ghfast.top"},
			AllowedUpdateDirs: []string{
				"/vol1/@appshare",
				"/vol2/@appshare",
				"/vol3/@appshare",
				"/vol4/@appshare",
			},
		},
		Docker: DockerConfig{
			ListTimeoutMs:  getEnvInt64("DOCKER_LIST_TIMEOUT_MS", 10000),
			LogsTimeoutMs:  getEnvInt64("DOCKER_LOGS_TIMEOUT_MS", 120000),
			MaxLogLines:    getEnvInt("DOCKER_LOGS_MAX_LINES", 2000),
			MaxOutputBytes: getEnvInt64("DOCKER_LOGS_MAX_BYTES", 2*1024*1024),
		},
		Archive: ArchiveConfig{
			MaxPreviewLines: getEnvInt("ARCHIVE_MAX_PREVIEW_LINES", 200),
			MaxArchiveBytes: getEnvInt64("ARCHIVE_MAX_BYTES", 200*1024*1024),
			MaxOutputBytes:  getEnvInt64("ARCHIVE_MAX_OUTPUT_BYTES", 1024*1024),
		},
		LogFile: LogFileConfig{
			MaxPreviewLines: getEnvInt("LOGFILE_MAX_PREVIEW_LINES", 5000),
			MaxPreviewBytes: getEnvInt64("LOGFILE_MAX_PREVIEW_BYTES", 10*1024*1024),
		},
		Backup: BackupConfig{
			BaseDir:          backupDir,
			MaxFiles:         getEnvInt("BACKUP_MAX_FILES", 1000),
			MaxFileSizeBytes: getEnvInt64("BACKUP_MAX_FILE_BYTES", 200*1024*1024),
			MaxTotalBytes:    getEnvInt64("BACKUP_MAX_TOTAL_BYTES", 2*1024*1024*1024),
		},
		Notify: NotifyConfig{
			Enabled:    getEnvBool("NOTIFY_ENABLED", false),
			WebhookURL: getEnvOrDefault("NOTIFY_WEBHOOK_URL", ""),
			SMTPHost:   getEnvOrDefault("NOTIFY_SMTP_HOST", ""),
			SMTPPort:   getEnvInt("NOTIFY_SMTP_PORT", 587),
			SMTPUser:   getEnvOrDefault("NOTIFY_SMTP_USER", ""),
			SMTPPass:   getEnvOrDefault("NOTIFY_SMTP_PASS", ""),
			SMTPFrom:   getEnvOrDefault("NOTIFY_SMTP_FROM", "logmanager@fnos.local"),
			SMTPTo:     getEnvOrDefault("NOTIFY_SMTP_TO", ""),
		},
		LogDirs: []string{
			"/vol1/@appdata",
			"/vol1/@appconf",
			"/vol1/@apphome",
			"/vol1/@apptemp",
			"/vol1/@appshare",
			"/var/log/apps",
		},
		EventLogger: EventLoggerConfig{
			DBPath:        getEnvOrDefault("EVENTLOGGER_DB_PATH", "/usr/trim/var/eventlogger_service/logger_data.db3"),
			Enabled:       getEnvBool("EVENTLOGGER_ENABLED", false),
			CheckInterval: getEnvInt("EVENTLOGGER_CHECK_INTERVAL", 30000),
			EventTypes:    strings.Split(getEnvOrDefault("EVENTLOGGER_EVENT_TYPES", "*"), ","),
			MinSeverity:   getEnvOrDefault("EVENTLOGGER_MIN_SEVERITY", "info"),
		},
		SensitivePatterns: []string{
			`password\s*[=:]\s*\S+`,
			`passwd\s*[=:]\s*\S+`,
			`secret\s*[=:]\s*\S+`,
			`api[_-]?key\s*[=:]\s*\S+`,
			`token\s*[=:]\s*\S+`,
			`private[_-]?key\s*[=:]\s*\S+`,
			`access[_-]?key\s*[=:]\s*\S+`,
			`auth[_-]?key\s*[=:]\s*\S+`,
			`-----BEGIN\s+(RSA\s+)?PRIVATE\s+KEY-----`,
			`-----BEGIN\s+OPENSSH\s+PRIVATE\s+KEY-----`,
			`eyJ[A-Za-z0-9-_]+\.[A-Za-z0-9-_]+\.[A-Za-z0-9-_.+/]*`,
		},
	}

	// Apply JSON config file overrides
	if len(fileConfig.LogDirs) > 0 {
		cfg.LogDirs = fileConfig.LogDirs
	}
	if fileConfig.EventLogger != nil {
		if fileConfig.EventLogger.Enabled != nil {
			cfg.EventLogger.Enabled = *fileConfig.EventLogger.Enabled
		}
		if fileConfig.EventLogger.DBPath != nil {
			cfg.EventLogger.DBPath = *fileConfig.EventLogger.DBPath
		}
		if fileConfig.EventLogger.CheckInterval != nil {
			cfg.EventLogger.CheckInterval = *fileConfig.EventLogger.CheckInterval
		}
		if len(fileConfig.EventLogger.EventTypes) > 0 {
			cfg.EventLogger.EventTypes = fileConfig.EventLogger.EventTypes
		}
		if fileConfig.EventLogger.MinSeverity != nil {
			cfg.EventLogger.MinSeverity = *fileConfig.EventLogger.MinSeverity
		}
		if len(fileConfig.EventLogger.NotificationChannels) > 0 {
			cfg.EventLogger.NotificationChannels = fileConfig.EventLogger.NotificationChannels
		}
	}

	// Load event logger config from dedicated file (saved by user via UI)
	elConfigPath := filepath.Join(dataDir, "eventlogger-config.json")
	if elData, err := os.ReadFile(elConfigPath); err == nil {
		var elConfig struct {
			DBPath              *string  `json:"dbPath"`
			Enabled             *bool    `json:"enabled"`
			CheckInterval       *int     `json:"checkInterval"`
			EventTypes          []string `json:"eventTypes"`
			MinSeverity         *string  `json:"minSeverity"`
			NotificationChannels []string `json:"notificationChannels"`
		}
		if json.Unmarshal(elData, &elConfig) == nil {
			if elConfig.Enabled != nil {
				cfg.EventLogger.Enabled = *elConfig.Enabled
			}
			if elConfig.DBPath != nil {
				cfg.EventLogger.DBPath = *elConfig.DBPath
			}
			if elConfig.CheckInterval != nil {
				cfg.EventLogger.CheckInterval = *elConfig.CheckInterval
			}
			if len(elConfig.EventTypes) > 0 {
				cfg.EventLogger.EventTypes = elConfig.EventTypes
			}
			if elConfig.MinSeverity != nil {
				cfg.EventLogger.MinSeverity = *elConfig.MinSeverity
			}
			if len(elConfig.NotificationChannels) > 0 {
				cfg.EventLogger.NotificationChannels = elConfig.NotificationChannels
			}
		}
	}

	return cfg
}

// UpdateEventLoggerConfig updates the event logger configuration in memory and persists to disk.
func UpdateEventLoggerConfig(el EventLoggerConfig) error {
	Get().EventLogger = el
	return saveEventLoggerConfig(el)
}

func saveEventLoggerConfig(el EventLoggerConfig) error {
	dataDir := Get().DataDir
	configPath := filepath.Join(dataDir, "eventlogger-config.json")
	// Ensure directory exists
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(el, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configPath, data, 0644)
}

// GetGatewayPrefix returns the gateway URL prefix.
func GetGatewayPrefix() string {
	return Get().GatewayPrefix
}

// IsGatewayMode returns true if running behind the fnOS gateway.
func IsGatewayMode() bool {
	return Get().GatewaySocket != ""
}
