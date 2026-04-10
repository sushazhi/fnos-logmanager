import path from 'path';
import fs from 'fs';
import { AppConfig } from '../types';

const config: AppConfig = {
    port: parseInt(process.env.PORT || '8090', 10),
    dataDir: process.env.LOGMANAGER_DATA_DIR || '/vol1/@appdata/logmanager',
    sessionExpiry: 24 * 60 * 60 * 1000,
    rateLimit: {
        windowMs: 60000,
        maxRequests: 100
    },
    auth: {
        // 安全提示：allowQueryToken 允许通过 URL query 参数传递 session token
        // 这会使 token 出现在访问日志、浏览器历史记录和 Referer 头中
        // 仅在 WebSocket 连接等无法设置 cookie 的场景下启用，并限制到最小路径集合
        allowQueryToken: process.env.ALLOW_QUERY_TOKEN === 'true',
        queryTokenPaths: ['/api/auth/status', '/api/auth/csrf-token']
    },
    login: {
        maxAttempts: 5,
        lockoutTime: 30 * 60 * 1000
    },
    audit: {
        maxLogs: 1000
    },
    csrf: {
        expiry: 2 * 60 * 60 * 1000
    },
    logging: {
        redactQuery: process.env.LOG_REDACT_QUERY !== 'false'
    },
    update: {
        checkCacheMs: parseInt(process.env.UPDATE_CHECK_CACHE_MS || '60000', 10),
        checkRateLimit: {
            windowMs: parseInt(process.env.UPDATE_CHECK_WINDOW_MS || '60000', 10),
            maxRequests: parseInt(process.env.UPDATE_CHECK_MAX || '10', 10)
        },
        downloadTimeoutMs: parseInt(process.env.UPDATE_DOWNLOAD_TIMEOUT_MS || '300000', 10),
        maxDownloadBytes: parseInt(process.env.UPDATE_MAX_DOWNLOAD_BYTES || String(200 * 1024 * 1024), 10),
        maxAssetBytes: parseInt(process.env.UPDATE_MAX_ASSET_BYTES || String(500 * 1024 * 1024), 10),
        maxRedirects: parseInt(process.env.UPDATE_MAX_REDIRECTS || '3', 10),
        allowedHosts: ['github.com', 'api.github.com', 'objects.githubusercontent.com', 'ghfast.top'],
        allowedUpdateDirs: [
            '/vol1/@appshare',
            '/vol2/@appshare',
            '/vol3/@appshare',
            '/vol4/@appshare'
        ]
    },
    docker: {
        listTimeoutMs: parseInt(process.env.DOCKER_LIST_TIMEOUT_MS || '10000', 10),
        logsTimeoutMs: parseInt(process.env.DOCKER_LOGS_TIMEOUT_MS || '120000', 10),
        maxLogLines: parseInt(process.env.DOCKER_LOGS_MAX_LINES || '2000', 10),
        maxOutputBytes: parseInt(process.env.DOCKER_LOGS_MAX_BYTES || String(2 * 1024 * 1024), 10)
    },
    archive: {
        maxPreviewLines: parseInt(process.env.ARCHIVE_MAX_PREVIEW_LINES || '200', 10),
        maxArchiveBytes: parseInt(process.env.ARCHIVE_MAX_BYTES || String(200 * 1024 * 1024), 10),
        maxOutputBytes: parseInt(process.env.ARCHIVE_MAX_OUTPUT_BYTES || String(1024 * 1024), 10)
    },
    logFile: {
        maxPreviewLines: parseInt(process.env.LOGFILE_MAX_PREVIEW_LINES || '5000', 10),
        maxPreviewBytes: parseInt(process.env.LOGFILE_MAX_PREVIEW_BYTES || String(10 * 1024 * 1024), 10)
    },
    backup: {
        baseDir: process.env.BACKUP_BASE_DIR || '/vol1/@appshare/log-backup',
        maxFiles: parseInt(process.env.BACKUP_MAX_FILES || '1000', 10),
        maxFileSizeBytes: parseInt(process.env.BACKUP_MAX_FILE_BYTES || String(200 * 1024 * 1024), 10),
        maxTotalBytes: parseInt(process.env.BACKUP_MAX_TOTAL_BYTES || String(2 * 1024 * 1024 * 1024), 10)
    },
    logDirs: [
        '/vol1/@appdata',
        '/vol1/@appconf',
        '/vol1/@apphome',
        '/vol1/@apptemp',
        '/vol1/@appshare',
        '/var/log/apps'
    ],
    eventLogger: {
        dbPath: process.env.EVENTLOGGER_DB_PATH || '/usr/trim/var/eventlogger_service/logger_data.db3',
        enabled: process.env.EVENTLOGGER_ENABLED === 'true',
        checkInterval: parseInt(process.env.EVENTLOGGER_CHECK_INTERVAL || '30000', 10),
        eventTypes: (process.env.EVENTLOGGER_EVENT_TYPES || '*').split(','),
        minSeverity: (process.env.EVENTLOGGER_MIN_SEVERITY || 'info') as 'debug' | 'info' | 'warning' | 'error' | 'critical',
        notificationChannels: (process.env.EVENTLOGGER_CHANNELS || '').split(',').filter(Boolean)
    },
    sensitivePatterns: [
        /password\s*[=:]\s*\S+/gi,
        /passwd\s*[=:]\s*\S+/gi,
        /secret\s*[=:]\s*\S+/gi,
        /api[_-]?key\s*[=:]\s*\S+/gi,
        /token\s*[=:]\s*\S+/gi,
        /private[_-]?key\s*[=:]\s*\S+/gi,
        /access[_-]?key\s*[=:]\s*\S+/gi,
        /auth[_-]?key\s*[=:]\s*\S+/gi,
        /-----BEGIN\s+(RSA\s+)?PRIVATE\s+KEY-----/g,
        /-----BEGIN\s+OPENSSH\s+PRIVATE\s+KEY-----/g,
        /eyJ[A-Za-z0-9-_]+\.[A-Za-z0-9-_]+\.[A-Za-z0-9-_.+/]*/g
    ],
    passwordFile: '',
    auditLogFile: '',
    initTimestampFile: ''
};

config.passwordFile = path.join(config.dataDir, '.password');
config.auditLogFile = path.join(config.dataDir, 'audit.log');
config.initTimestampFile = path.join(config.dataDir, '.init-time');

const configFile = path.join(config.dataDir, 'config', 'config.json');
try {
    if (fs.existsSync(configFile)) {
        const fileContent = fs.readFileSync(configFile, 'utf8');
        const fileConfig = JSON.parse(fileContent) as { log_dirs?: string[] };
        if (fileConfig.log_dirs && Array.isArray(fileConfig.log_dirs)) {
            config.logDirs = fileConfig.log_dirs;
        }
    }
} catch {
    // 使用默认配置
}

export default config;
