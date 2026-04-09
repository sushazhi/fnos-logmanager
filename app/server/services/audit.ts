import fs from 'fs';
import config from '../utils/config';
import { getClientIP } from '../utils/ip';
import { AuditLogEntry } from '../types';
import { Request } from 'express';
import Logger from '../utils/logger';

const logger = Logger.child({ module: 'Audit' });

interface ExtendedAuditDetails {
    [key: string]: unknown;
    duration?: number;
    success?: boolean;
    error?: string;
    userAgent?: string;
    method?: string;
    path?: string;
}

const ACTION_NAMES: Record<string, string> = {
    'SERVER_START': '服务器启动',
    'SERVER_SHUTDOWN': '服务器关闭',
    'login_success': '登录成功',
    'login_failed': '登录失败',
    'login_locked': '登录锁定',
    'logout': '登出',
    'password_setup': '密码设置',
    'password_changed': '密码修改',
    'password_change_failed': '密码修改失败',
    'auth_failed': '认证失败',
    'csrf_failed': 'CSRF验证失败',
    'log_truncate': '日志清空',
    'log_delete': '日志删除',
    'logs_clean': '日志批量清理',
    'logs_backup': '日志备份',
    'backup_delete': '备份删除',
    'backups_clean': '备份清理',
    'archive_delete': '归档删除',
    'app_updated': '应用更新',
    'update_failed': '更新失败',
    'SECURITY_UNCAUGHT_EXCEPTION': '安全异常-未捕获异常',
    'SECURITY_UNHANDLED_REJECTION': '安全异常-未处理Promise',
    'SECURITY_SENSITIVE_INFO_SCAN': '安全扫描-敏感信息',
    'SECURITY_APP_UPDATED': '安全事件-应用更新',
    'SECURITY_UPDATE_FAILED': '安全事件-更新失败'
};

function getActionName(action: string): string {
    return ACTION_NAMES[action] || action;
}

function redactSensitiveDetails(details: Record<string, unknown>): Record<string, unknown> {
    const result: Record<string, unknown> = {};

    for (const [key, value] of Object.entries(details || {})) {
        if (typeof value === 'string') {
            let sanitized = value;
            for (const pattern of config.sensitivePatterns) {
                sanitized = sanitized.replace(pattern, '[FILTERED]');
            }
            result[key] = sanitized;
            continue;
        }

        if (Array.isArray(value)) {
            result[key] = value.map((item) => {
                if (typeof item === 'string') {
                    let sanitized = item;
                    for (const pattern of config.sensitivePatterns) {
                        sanitized = sanitized.replace(pattern, '[FILTERED]');
                    }
                    return sanitized;
                }
                if (item && typeof item === 'object') {
                    return redactSensitiveDetails(item as Record<string, unknown>);
                }
                return item;
            });
            continue;
        }

        if (value && typeof value === 'object') {
            result[key] = redactSensitiveDetails(value as Record<string, unknown>);
            continue;
        }

        result[key] = value;
    }

    return result;
}

export async function addAuditLog(action: string, details: Record<string, unknown>, req?: Request): Promise<void> {
    let ip = '系统';
    let userAgent = '系统';
    let method = '系统';
    let requestPath = '';

    if (req) {
        ip = (req as { clientIP?: string }).clientIP || getClientIP(req);
        userAgent = req.headers['user-agent'] || '未知';
        method = req.method;
        requestPath = req.path;
    }

    const entry: AuditLogEntry = {
        timestamp: new Date().toISOString(),
        action: getActionName(action),
        details: {
            ...redactSensitiveDetails(details),
            method,
            path: requestPath,
            originalAction: action
        },
        ip: ip,
        userAgent: userAgent
    };

    try {
        try {
            await fs.promises.mkdir(config.dataDir, { recursive: true, mode: 0o700 });
        } catch (e) {
            if ((e as NodeJS.ErrnoException).code !== 'EEXIST') throw e;
        }

        const logLine = JSON.stringify(entry) + '\n';
        await fs.promises.appendFile(config.auditLogFile, logLine, { mode: 0o600 });
    } catch (e) {
        logger.error({ err: e }, '写入审计日志失败');
    }
}

export async function addSecurityAuditLog(
    action: string,
    details: ExtendedAuditDetails,
    req?: Request
): Promise<void> {
    const extendedDetails: ExtendedAuditDetails = {
        ...details,
        securityEvent: true,
        timestamp: Date.now()
    };

    await addAuditLog(`SECURITY_${action}`, extendedDetails, req);
}

export async function getAuditLogs(limit: number = 100): Promise<AuditLogEntry[]> {
    try {
        const content = await fs.promises.readFile(config.auditLogFile, 'utf8');
        const lines = content.trim().split('\n').filter(line => line);
        const logs = lines.slice(-limit * 10).map(line => {
            try {
                return JSON.parse(line) as AuditLogEntry;
            } catch {
                return null;
            }
        }).filter((log): log is AuditLogEntry => log !== null);
        return logs.reverse().slice(0, limit);
    } catch (e) {
        if ((e as NodeJS.ErrnoException).code === 'ENOENT') {
            return [];
        }
        logger.error({ err: e }, '读取审计日志失败');
        return [];
    }
}

export async function cleanOldAuditLogs(): Promise<void> {
    try {
        // 先检查文件大小，执行轮转
        await rotateAuditLog();

        const content = await fs.promises.readFile(config.auditLogFile, 'utf8');
        const lines = content.trim().split('\n').filter(line => line);

        if (lines.length > config.audit.maxLogs) {
            const keptLines = lines.slice(-config.audit.maxLogs);
            await fs.promises.writeFile(config.auditLogFile, keptLines.join('\n') + '\n', { mode: 0o600 });
        }
    } catch (e) {
        if ((e as NodeJS.ErrnoException).code === 'ENOENT') {
            return;
        }
        logger.error({ err: e }, '清理审计日志失败');
    }
}

/**
 * 审计日志轮转
 * 当文件超过 maxFileSize 时，将当前文件重命名为 .1，.1 重命名为 .2，以此类推
 * 最多保留 maxRotatedFiles 个历史文件
 */
async function rotateAuditLog(): Promise<void> {
    const MAX_FILE_SIZE = 10 * 1024 * 1024; // 10MB 触发轮转
    const MAX_ROTATED_FILES = 5; // 最多保留 5 个历史文件

    try {
        const stat = await fs.promises.stat(config.auditLogFile);
        if (stat.size < MAX_FILE_SIZE) return;

        // 轮转历史文件：.4 -> .5, .3 -> .4, .2 -> .3, .1 -> .2
        for (let i = MAX_ROTATED_FILES - 1; i >= 1; i--) {
            const oldPath = `${config.auditLogFile}.${i}`;
            const newPath = `${config.auditLogFile}.${i + 1}`;
            try {
                await fs.promises.access(oldPath);
                if (i === MAX_ROTATED_FILES - 1) {
                    // 最老的文件直接删除
                    await fs.promises.unlink(oldPath);
                } else {
                    await fs.promises.rename(oldPath, newPath);
                }
            } catch {
                // 文件不存在，跳过
            }
        }

        // 将当前文件重命名为 .1
        await fs.promises.rename(config.auditLogFile, `${config.auditLogFile}.1`);

        logger.info({ originalSize: stat.size }, 'Audit log rotated');
    } catch (e) {
        if ((e as NodeJS.ErrnoException).code === 'ENOENT') return;
        logger.error({ err: e }, '审计日志轮转失败');
    }
}

export async function scanForSensitiveInfo(): Promise<{ path: string; matches: string[] }[]> {
    const results: { path: string; matches: string[] }[] = [];

    for (const dir of config.logDirs) {
        try {
            if (!fs.existsSync(dir)) continue;

            const files = await fs.promises.readdir(dir, { withFileTypes: true });

            for (const file of files) {
                if (!file.isFile()) continue;

                const filePath = `${dir}/${file.name}`;
                try {
                    const content = await fs.promises.readFile(filePath, 'utf8');
                    const matches: string[] = [];

                    for (const pattern of config.sensitivePatterns) {
                        const found = content.match(pattern);
                        if (found) {
                            matches.push(...found.slice(0, 3));
                        }
                    }

                    if (matches.length > 0) {
                        results.push({ path: filePath, matches: matches.slice(0, 10) });
                    }
                } catch {
                    // 忽略无法读取的文件
                }
            }
        } catch {
            // 忽略无法访问的目录
        }
    }

    return results;
}

setInterval(cleanOldAuditLogs, 3600000);

setInterval(async () => {
    const results = await scanForSensitiveInfo();
    if (results.length > 0) {
        await addSecurityAuditLog('SENSITIVE_INFO_SCAN', {
            filesFound: results.length,
            timestamp: new Date().toISOString()
        });
    }
}, 24 * 60 * 60 * 1000);
