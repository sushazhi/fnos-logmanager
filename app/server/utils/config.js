/**
 * @fileoverview 应用配置模块
 */

const path = require('path');
const fs = require('fs');

const config = {
    port: parseInt(process.env.PORT || '8090', 10),
    dataDir: process.env.LOGMANAGER_DATA_DIR || '/vol1/@appdata/logmanager',
    sessionExpiry: 24 * 60 * 60 * 1000,
    rateLimit: {
        windowMs: 60000,
        maxRequests: 100
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
    logDirs: [
        '/vol1/@appdata',
        '/vol1/@appconf',
        '/vol1/@apphome',
        '/vol1/@apptemp',
        '/vol1/@appshare',
        '/var/log/apps'
    ],
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
    ]
};

config.passwordFile = path.join(config.dataDir, '.password');
config.auditLogFile = path.join(config.dataDir, 'audit.log');

const configFile = path.join(config.dataDir, 'config', 'config.json');
try {
    if (fs.existsSync(configFile)) {
        const fileContent = fs.readFileSync(configFile, 'utf8');
        const fileConfig = JSON.parse(fileContent);
        if (fileConfig.log_dirs && Array.isArray(fileConfig.log_dirs)) {
            config.logDirs = fileConfig.log_dirs;
        }
    }
} catch (e) {
    // 使用默认配置
}

module.exports = config;
