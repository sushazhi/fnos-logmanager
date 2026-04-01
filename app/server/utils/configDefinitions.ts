import configManager from './configManager';

// ============================================
// 服务器配置域
// ============================================
configManager.define({
    key: 'server.port',
    defaultValue: 8090,
    envKey: 'PORT',
    description: '服务监听端口'
});

configManager.define({
    key: 'server.dataDir',
    defaultValue: '/vol1/@appdata/logmanager',
    envKey: 'LOGMANAGER_DATA_DIR',
    description: '数据存储目录'
});

configManager.define({
    key: 'server.sessionExpiry',
    defaultValue: 24 * 60 * 60 * 1000,
    description: '会话过期时间（毫秒）'
});

// ============================================
// 限流配置域
// ============================================
configManager.define({
    key: 'rateLimit.windowMs',
    defaultValue: 60000,
    description: '限流时间窗口（毫秒）'
});

configManager.define({
    key: 'rateLimit.maxRequests',
    defaultValue: 100,
    description: '时间窗口内最大请求数'
});

// ============================================
// 认证配置域
// ============================================
configManager.define({
    key: 'auth.maxAttempts',
    defaultValue: 5,
    description: '最大登录尝试次数'
});

configManager.define({
    key: 'auth.lockoutTime',
    defaultValue: 30 * 60 * 1000,
    description: '登录锁定时间（毫秒）'
});

configManager.define({
    key: 'auth.allowQueryToken',
    defaultValue: false,
    envKey: 'ALLOW_QUERY_TOKEN',
    description: '是否允许 URL 查询参数传递令牌'
});

configManager.define({
    key: 'auth.csrfExpiry',
    defaultValue: 2 * 60 * 60 * 1000,
    description: 'CSRF 令牌过期时间（毫秒）'
});

// ============================================
// 审计配置域
// ============================================
configManager.define({
    key: 'audit.maxLogs',
    defaultValue: 1000,
    description: '最大审计日志数量'
});

// ============================================
// 通知配置域
// ============================================
configManager.define({
    key: 'notify.timeout',
    defaultValue: 15000,
    envKey: 'NOTIFY_TIMEOUT',
    description: '通知发送超时时间（毫秒）'
});

configManager.define({
    key: 'notify.retryCount',
    defaultValue: 3,
    envKey: 'NOTIFY_RETRY_COUNT',
    description: '通知发送重试次数'
});

configManager.define({
    key: 'notify.checkInterval',
    defaultValue: 30000,
    envKey: 'NOTIFY_CHECK_INTERVAL',
    description: '日志监控检查间隔（毫秒）'
});

// ============================================
// 日志配置域
// ============================================
configManager.define({
    key: 'log.level',
    defaultValue: 'info',
    envKey: 'LOG_LEVEL',
    description: '日志级别'
});

configManager.define({
    key: 'log.pretty',
    defaultValue: false,
    envKey: 'LOG_PRETTY',
    description: '是否使用美化输出'
});

configManager.define({
    key: 'log.redactQuery',
    defaultValue: true,
    envKey: 'LOG_REDACT_QUERY',
    description: '是否脱敏查询参数'
});

// ============================================
// Docker 配置域
// ============================================
configManager.define({
    key: 'docker.listTimeoutMs',
    defaultValue: 10000,
    envKey: 'DOCKER_LIST_TIMEOUT_MS',
    description: 'Docker 容器列表超时时间（毫秒）'
});

configManager.define({
    key: 'docker.logsTimeoutMs',
    defaultValue: 120000,
    envKey: 'DOCKER_LOGS_TIMEOUT_MS',
    description: 'Docker 日志获取超时时间（毫秒）'
});

configManager.define({
    key: 'docker.maxLogLines',
    defaultValue: 2000,
    envKey: 'DOCKER_LOGS_MAX_LINES',
    description: 'Docker 日志最大行数'
});

configManager.define({
    key: 'docker.maxOutputBytes',
    defaultValue: 2 * 1024 * 1024,
    envKey: 'DOCKER_LOGS_MAX_BYTES',
    description: 'Docker 日志最大输出字节数'
});

// ============================================
// 归档配置域
// ============================================
configManager.define({
    key: 'archive.maxPreviewLines',
    defaultValue: 200,
    envKey: 'ARCHIVE_MAX_PREVIEW_LINES',
    description: '归档文件预览最大行数'
});

configManager.define({
    key: 'archive.maxArchiveBytes',
    defaultValue: 200 * 1024 * 1024,
    envKey: 'ARCHIVE_MAX_BYTES',
    description: '归档文件最大字节数'
});

configManager.define({
    key: 'archive.maxOutputBytes',
    defaultValue: 1024 * 1024,
    envKey: 'ARCHIVE_MAX_OUTPUT_BYTES',
    description: '归档输出最大字节数'
});

// ============================================
// 日志文件配置域
// ============================================
configManager.define({
    key: 'logFile.maxPreviewLines',
    defaultValue: 5000,
    envKey: 'LOGFILE_MAX_PREVIEW_LINES',
    description: '日志文件预览最大行数'
});

configManager.define({
    key: 'logFile.maxPreviewBytes',
    defaultValue: 10 * 1024 * 1024,
    envKey: 'LOGFILE_MAX_PREVIEW_BYTES',
    description: '日志文件预览最大字节数'
});

// ============================================
// 备份配置域
// ============================================
configManager.define({
    key: 'backup.baseDir',
    defaultValue: '/vol1/@appshare/log-backup',
    envKey: 'BACKUP_BASE_DIR',
    description: '备份基础目录'
});

configManager.define({
    key: 'backup.maxFiles',
    defaultValue: 1000,
    envKey: 'BACKUP_MAX_FILES',
    description: '最大备份文件数'
});

configManager.define({
    key: 'backup.maxFileSizeBytes',
    defaultValue: 200 * 1024 * 1024,
    envKey: 'BACKUP_MAX_FILE_BYTES',
    description: '单个备份文件最大字节数'
});

configManager.define({
    key: 'backup.maxTotalBytes',
    defaultValue: 2 * 1024 * 1024 * 1024,
    envKey: 'BACKUP_MAX_TOTAL_BYTES',
    description: '备份总大小最大字节数'
});

// ============================================
// 更新配置域
// ============================================
configManager.define({
    key: 'update.checkCacheMs',
    defaultValue: 60000,
    envKey: 'UPDATE_CHECK_CACHE_MS',
    description: '更新检查缓存时间（毫秒）'
});

configManager.define({
    key: 'update.downloadTimeoutMs',
    defaultValue: 300000,
    envKey: 'UPDATE_DOWNLOAD_TIMEOUT_MS',
    description: '更新下载超时时间（毫秒）'
});

configManager.define({
    key: 'update.maxDownloadBytes',
    defaultValue: 200 * 1024 * 1024,
    envKey: 'UPDATE_MAX_DOWNLOAD_BYTES',
    description: '更新下载最大字节数'
});

configManager.define({
    key: 'update.maxAssetBytes',
    defaultValue: 500 * 1024 * 1024,
    envKey: 'UPDATE_MAX_ASSET_BYTES',
    description: '更新资源最大字节数'
});

configManager.define({
    key: 'update.maxRedirects',
    defaultValue: 3,
    envKey: 'UPDATE_MAX_REDIRECTS',
    description: '更新下载最大重定向次数'
});

// ============================================
// EventLogger 配置域
// ============================================
configManager.define({
    key: 'eventLogger.dbPath',
    defaultValue: '/usr/trim/var/eventlogger_service/logger_data.db3',
    envKey: 'EVENTLOGGER_DB_PATH',
    description: 'EventLogger 数据库路径'
});

configManager.define({
    key: 'eventLogger.enabled',
    defaultValue: false,
    envKey: 'EVENTLOGGER_ENABLED',
    description: '是否启用 EventLogger'
});

configManager.define({
    key: 'eventLogger.checkInterval',
    defaultValue: 30000,
    envKey: 'EVENTLOGGER_CHECK_INTERVAL',
    description: 'EventLogger 检查间隔（毫秒）'
});

// 从环境变量加载配置
configManager.loadFromEnv();

export default configManager;
