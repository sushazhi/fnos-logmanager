import fs from 'fs';
import config from '../utils/config';
import { getClientIP } from '../utils/ip';
import { AuditLogEntry } from '../types';
import { Request } from 'express';

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
            ...details,
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
        console.error('[LogManager] 写入审计日志失败:', (e as Error).message);
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
        console.error('[LogManager] 读取审计日志失败:', (e as Error).message);
        return [];
    }
}

export async function cleanOldAuditLogs(): Promise<void> {
    try {
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
        console.error('[LogManager] 清理审计日志失败:', (e as Error).message);
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
