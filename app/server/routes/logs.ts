import express, { Request, Response, NextFunction } from 'express';
import fs from 'fs';
import { query, body, ValidationChain } from 'express-validator';
import * as logFileService from '../services/logFile';
import * as auditService from '../services/audit';
import * as backupService from '../services/backup';
import * as autoCleanService from '../services/autoClean';
import * as bookmarkService from '../services/bookmark';
import * as sessionService from '../services/session';
import { validateToken, validateCSRF, validate, checkValidation, requireAdmin } from '../middleware/auth';
import { sensitiveActionRateLimit } from '../middleware/rateLimit';
import { 
    safePath, 
    isAllowedPath, 
    isSymlinkPath,
    isValidSize, 
    isValidNumber, 
    isValidPattern,
    escapeRegExp,
    isValidAction,
    isValidDays,
    isValidThreshold,
    isValidContainerName,
    clamp
} from '../utils/validation';
import config from '../utils/config';
import { filterSensitiveInfo, setFilterEnabled, isFilterEnabled } from '../utils/filter';
import { parseSizeThreshold } from '../utils/sizeParser';
import { ValidationError } from '../utils/errors';

const router = express.Router();

const MAX_PATTERN_LENGTH = 100;
const MAX_PATH_LENGTH = 4096;

function getQueryParam(value: unknown): string | undefined {
    if (!value) return undefined;
    if (Array.isArray(value)) return value[0] as string;
    return value as string;
}

function sanitizePattern(pattern: string): string {
    if (!pattern || pattern.length > MAX_PATTERN_LENGTH) {
        return '';
    }
    return escapeRegExp(pattern.replace(/[^a-zA-Z0-9_\-\.\*\?\[\]\{\}]/g, ''));
}

router.get('/health', (_req: Request, res: Response) => {
    res.json({ status: 'ok', version: '1.0.0' });
});

router.get('/settings/filter', validateToken, (_req: Request, res: Response) => {
    res.json({ enabled: isFilterEnabled() });
});

router.post('/settings/filter', validateToken, validateCSRF, (req: Request, res: Response) => {
    const { enabled } = req.body;
    if (typeof enabled === 'boolean') {
        setFilterEnabled(enabled);
        res.json({ success: true, enabled: isFilterEnabled() });
    } else {
        res.status(400).json({ error: '无效的参数' });
    }
});

router.get('/dirs', validateToken, async (_req: Request, res: Response, next: NextFunction) => {
    try {
        const dirs = await logFileService.getDirsInfo();
        res.json({ dirs });
    } catch (err) {
        next(err);
    }
});

router.get('/appnames', validateToken, async (_req: Request, res: Response, next: NextFunction) => {
    try {
        const appNames = await logFileService.getAppNames();
        res.json(appNames);
    } catch (err) {
        next(err);
    }
});

router.get('/logs/list', validateToken, [
    query('dir').optional().isString().isLength({ max: MAX_PATH_LENGTH }),
    query('limit').optional().isInt({ min: 1, max: 500 })
], async (req: Request, res: Response, next: NextFunction) => {
    try {
        const limitStr = getQueryParam(req.query.limit) || '100';
        const limitNum = clamp(parseInt(limitStr) || 100, 1, 500);

        const dirStr = getQueryParam(req.query.dir);
        if (dirStr) {
            if (dirStr.length > MAX_PATH_LENGTH) {
                res.status(400).json({ error: '路径过长' });
                return;
            }
            if (!isAllowedPath(dirStr, config.logDirs)) {
                res.status(403).json({ error: '不允许访问此目录' });
                return;
            }
        }

        const logs = await logFileService.listLogFiles(dirStr, limitNum);
        res.json({ logs: logs, total: logs.length });
    } catch (err) {
        next(err);
    }
});

router.get('/logs/large', validateToken, [
    query('threshold').optional().matches(/^[0-9]+[KMGT]?$/i),
    query('limit').optional().isInt({ min: 1, max: 200 })
], async (req: Request, res: Response, next: NextFunction) => {
    try {
        const limitStr = getQueryParam(req.query.limit) || '50';
        const limitNum = clamp(parseInt(limitStr) || 50, 1, 200);

        const thresholdStr = getQueryParam(req.query.threshold) || '10M';
        if (!isValidThreshold(thresholdStr)) {
            res.status(400).json({ error: '无效的大小阈值' });
            return;
        }

        const thresholdBytes = parseSizeThreshold(thresholdStr);

        const logs = await logFileService.listLargeLogFiles(thresholdBytes, limitNum);
        res.json({ logs });
    } catch (err) {
        next(err);
    }
});

router.get('/logs/search', validateToken, [
    query('type').isIn(['size', 'name']),
    query('threshold').optional().matches(/^[0-9]+[KMGT]?$/i),
    query('pattern').optional().isString().isLength({ max: MAX_PATTERN_LENGTH }),
    query('limit').optional().isInt({ min: 1, max: 200 })
], async (req: Request, res: Response, next: NextFunction) => {
    try {
        const limitStr = getQueryParam(req.query.limit) || '50';
        const limitNum = clamp(parseInt(limitStr) || 50, 1, 200);

        let logs: Awaited<ReturnType<typeof logFileService.listLargeLogFiles>> = [];

        const typeStr = getQueryParam(req.query.type);
        const thresholdStr = getQueryParam(req.query.threshold);
        const patternStr = getQueryParam(req.query.pattern);

        if (typeStr === 'size') {
            const sizeThreshold = thresholdStr || '10M';
            if (!isValidThreshold(sizeThreshold)) {
                res.status(400).json({ error: '无效的大小阈值' });
                return;
            }

            const thresholdBytes = parseSizeThreshold(sizeThreshold);

            logs = await logFileService.listLargeLogFiles(thresholdBytes, limitNum);
        } else if (typeStr === 'name') {
            if (!patternStr) {
                res.status(400).json({ error: '请输入文件名模式' });
                return;
            }

            if (patternStr.length > MAX_PATTERN_LENGTH) {
                res.status(400).json({ error: '搜索模式过长' });
                return;
            }

            const safePattern = sanitizePattern(patternStr);
            if (!safePattern) {
                res.status(400).json({ error: '无效的搜索模式' });
                return;
            }

            logs = await logFileService.searchLogFilesByName(safePattern, limitNum);
        }

        res.json({ logs: logs.slice(0, limitNum), total: logs.length });
    } catch (err) {
        next(err);
    }
});

router.get('/logs/stats', validateToken, async (_req: Request, res: Response, next: NextFunction) => {
    try {
        const stats = await logFileService.getLogStats();
        res.json(stats);
    } catch (err) {
        next(err);
    }
});

router.get('/log/tail', validateToken, [
    query('path').notEmpty().isString().isLength({ max: MAX_PATH_LENGTH }),
    query('offset').optional().isInt()
], checkValidation, async (req: Request, res: Response, next: NextFunction) => {
    try {
        const logPathStr = getQueryParam(req.query.path);
        if (!logPathStr || !isAllowedPath(logPathStr, config.logDirs)) {
            res.status(403).json({ error: '不允许访问此文件' });
            return;
        }
        if (await isSymlinkPath(logPathStr)) {
            res.status(403).json({ error: '不允许访问符号链接' });
            return;
        }

        let offset = parseInt(req.query.offset as string, 10);
        if (isNaN(offset) || offset < 0) offset = -1;

        try {
            const stat = await fs.promises.stat(logPathStr);
            const fileSize = stat.size;

            if (offset < 0) {
                res.json({ content: '', offset: fileSize, totalSize: fileSize });
                return;
            }

            if (offset >= fileSize) {
                res.json({ content: '', offset: fileSize, totalSize: fileSize });
                return;
            }

            const maxBytes = 512 * 1024;
            const end = Math.min(offset + maxBytes, fileSize);
            const stream = fs.createReadStream(logPathStr, { start: offset, end: end - 1, encoding: 'utf8' });
            let content = '';
            for await (const chunk of stream) {
                content += chunk;
            }

            const filtered = isFilterEnabled() ? filterSensitiveInfo(content) : content;
            res.json({ content: filtered, offset: end, totalSize: fileSize });
        } catch (err) {
            if ((err as NodeJS.ErrnoException).code === 'ENOENT') {
                res.json({ content: '', offset: 0, totalSize: 0, deleted: true });
            } else {
                throw err;
            }
        }
    } catch (err) {
        next(err);
    }
});

router.get('/log/stream', [
    query('path').notEmpty().isString().isLength({ max: MAX_PATH_LENGTH }),
    query('token').optional().isString()
], checkValidation, async (req: Request, res: Response) => {
    const clientIp = req.ip || req.socket.remoteAddress || 'unknown';
    const sseConnectionCounts: Map<string, number> = (req.app.locals.sseLogConnectionCounts as Map<string, number>) || new Map();
    const currentCount = sseConnectionCounts.get(clientIp) || 0;
    if (currentCount >= 5) {
        res.status(429).json({ error: 'SSE 连接数超过限制' });
        return;
    }
    sseConnectionCounts.set(clientIp, currentCount + 1);
    req.app.locals.sseLogConnectionCounts = sseConnectionCounts;

    const sessionToken = (req.query.token as string) || req.cookies?.session_token;
    if (!sessionToken || !sessionService.validateSession(sessionToken)) {
        auditService.addAuditLog('auth_failed', { path: req.path, clientIP: clientIp, sseStream: true }, req);
        res.status(401).json({ error: '未认证' });
        return;
    }

    const logPathStr = getQueryParam(req.query.path);
    if (!logPathStr || !isAllowedPath(logPathStr, config.logDirs)) {
        res.status(403).json({ error: '不允许访问此文件' });
        return;
    }
    if (await isSymlinkPath(logPathStr)) {
        res.status(403).json({ error: '不允许访问符号链接' });
        return;
    }

    res.setHeader('Content-Type', 'text/event-stream');
    res.setHeader('Cache-Control', 'no-cache');
    res.setHeader('Connection', 'keep-alive');
    res.setHeader('X-Accel-Buffering', 'no');
    res.flushHeaders();

    const sendEvent = (event: string, data: any): void => {
        try {
            res.write(`event: ${event}\ndata: ${JSON.stringify(data)}\n\n`);
        } catch { /* client disconnected */ }
    };

    sendEvent('connected', { message: 'Stream connected' });

    let lastSize = 0;
    let lastPos = 0;

    try {
        const stat = await fs.promises.stat(logPathStr);
        lastSize = stat.size;
        lastPos = stat.size;
    } catch { /* file may not exist yet */ }

    const POLL_INTERVAL = 1000;
    const timer = setInterval(async () => {
        try {
            const stat = await fs.promises.stat(logPathStr);
            const currentSize = stat.size;

            if (currentSize < lastSize) {
                lastPos = 0;
                lastSize = currentSize;
                sendEvent('file_rotated', { path: safePath(logPathStr) });
            }

            if (currentSize > lastPos) {
                const stream = fs.createReadStream(logPathStr, {
                    start: lastPos,
                    encoding: 'utf8'
                });
                let content = '';
                for await (const chunk of stream) {
                    content += chunk;
                }
                if (content) {
                    const filtered = isFilterEnabled() ? filterSensitiveInfo(content) : content;
                    sendEvent('data', { content: filtered, offset: lastPos, totalSize: currentSize });
                }
                lastPos = currentSize;
                lastSize = currentSize;
            }
        } catch (err) {
            if ((err as NodeJS.ErrnoException).code === 'ENOENT') {
                sendEvent('file_deleted', { path: safePath(logPathStr) });
            }
        }
    }, POLL_INTERVAL);

    req.on('close', () => {
        clearInterval(timer);
        const counts = req.app.locals.sseLogConnectionCounts as Map<string, number> | undefined;
        if (counts) {
            const count = counts.get(clientIp) || 1;
            if (count <= 1) counts.delete(clientIp);
            else counts.set(clientIp, count - 1);
        }
    });
});

router.get('/log/export', validateToken, [
    query('path').notEmpty().isString().isLength({ max: MAX_PATH_LENGTH }),
    query('format').optional().isIn(['txt', 'json', 'csv'])
], checkValidation, async (req: Request, res: Response, next: NextFunction) => {
    try {
        const logPathStr = getQueryParam(req.query.path);
        if (!logPathStr) {
            res.status(400).json({ error: '缺少文件路径' });
            return;
        }

        if (logPathStr.length > MAX_PATH_LENGTH) {
            res.status(400).json({ error: '路径过长' });
            return;
        }

        if (!isAllowedPath(logPathStr, config.logDirs)) {
            res.status(403).json({ error: '不允许访问此文件' });
            return;
        }

        const format = getQueryParam(req.query.format) || 'txt';
        const result = await logFileService.readLogFile(logPathStr, { maxLines: 200000 });

        const basename = logPathStr.split('/').pop() || 'log';
        const safeName = basename.replace(/[^a-zA-Z0-9._-]/g, '_');
        const timestamp = new Date().toISOString().replace(/[:.]/g, '-');
        const exportName = `${safeName}_${timestamp}`.replace(/"/g, '\\"');

        auditService.addAuditLog('log_export', { path: safePath(logPathStr), format }, req);

        if (format === 'txt') {
            res.setHeader('Content-Type', 'text/plain; charset=utf-8');
            res.setHeader('Content-Disposition', `attachment; filename="${exportName}.txt"`);
            res.send(filterSensitiveInfo(result.content));
        } else if (format === 'json') {
            const lines = result.content.split('\n');
            const data = lines.map((line: string, index: number) => ({
                line: index + 1,
                content: line
            }));
            res.setHeader('Content-Type', 'application/json; charset=utf-8');
            res.setHeader('Content-Disposition', `attachment; filename="${exportName}.json"`);
            res.json({
                source: safePath(logPathStr),
                exportedAt: new Date().toISOString(),
                totalLines: result.totalLines,
                lines: data
            });
        } else if (format === 'csv') {
            const lines = result.content.split('\n');
            const csvLines = ['"line","content"'];
            for (let i = 0; i < lines.length; i++) {
                const escaped = lines[i].replace(/"/g, '""');
                csvLines.push(`"${i + 1}","${escaped}"`);
            }
            res.setHeader('Content-Type', 'text/csv; charset=utf-8');
            res.setHeader('Content-Disposition', `attachment; filename="${exportName}.csv"`);
            res.send(csvLines.join('\n'));
        }
    } catch (err) {
        next(err);
    }
});

router.get('/log/content', validateToken, [
    query('path').notEmpty().isString().isLength({ max: MAX_PATH_LENGTH }),
    query('maxLines').optional().isInt({ min: 100, max: 200000 }),
    query('offset').optional().isInt({ min: 0 }),
    query('tail').optional().isBoolean()
], async (req: Request, res: Response, next: NextFunction) => {
    try {
        const logPathStr = getQueryParam(req.query.path);
        if (!logPathStr) {
            res.status(400).json({ error: '缺少文件路径' });
            return;
        }

        if (logPathStr.length > MAX_PATH_LENGTH) {
            res.status(400).json({ error: '路径过长' });
            return;
        }

        if (!isAllowedPath(logPathStr, config.logDirs)) {
            res.status(403).json({ error: '不允许访问此文件' });
            return;
        }

        const maxLinesStr = getQueryParam(req.query.maxLines);
        const offsetStr = getQueryParam(req.query.offset);
        const tailStr = getQueryParam(req.query.tail);

        const options = {
            maxLines: clamp(maxLinesStr ? parseInt(maxLinesStr) : 5000, 100, 200000),
            offset: clamp(offsetStr ? parseInt(offsetStr) : 0, 0, Number.MAX_SAFE_INTEGER),
            tail: tailStr === 'true'
        };

        const result = await logFileService.readLogFile(logPathStr, options);
        res.json({
            content: filterSensitiveInfo(result.content),
            totalLines: result.totalLines,
            size: result.size,
            sizeFormatted: result.sizeFormatted,
            truncated: result.truncated,
            hasMore: result.hasMore
        });
    } catch (err) {
        const error = err as Error & { size?: number; sizeFormatted?: string };
        if (error.size) {
            res.status(400).json({
                error: error.message,
                size: error.size,
                sizeFormatted: error.sizeFormatted
            });
            return;
        }
        next(err);
    }
});

router.post('/log/truncate', validateToken, validateCSRF, sensitiveActionRateLimit(10, 300000), [
    body('path').notEmpty().isString().isLength({ max: MAX_PATH_LENGTH })
], async (req: Request, res: Response, next: NextFunction) => {
    try {
        const { path: logPath } = req.body;

        if (!logPath) {
            res.status(400).json({ error: '缺少文件路径' });
            return;
        }

        if (logPath.length > MAX_PATH_LENGTH) {
            res.status(400).json({ error: '路径过长' });
            return;
        }

        if (!isAllowedPath(logPath, config.logDirs)) {
            res.status(403).json({ error: '不允许访问此文件' });
            return;
        }

        await logFileService.truncateLogFile(logPath);
        auditService.addAuditLog('log_truncate', { path: safePath(logPath) }, req);
        res.json({ success: true, message: '日志已清空' });
    } catch (err) {
        next(err);
    }
});

router.post('/log/delete', validateToken, validateCSRF, sensitiveActionRateLimit(5, 300000), [
    body('path').notEmpty().isString().isLength({ max: MAX_PATH_LENGTH })
], async (req: Request, res: Response, next: NextFunction) => {
    try {
        const { path: logPath } = req.body;

        if (!logPath) {
            res.status(400).json({ error: '缺少文件路径' });
            return;
        }

        if (logPath.length > MAX_PATH_LENGTH) {
            res.status(400).json({ error: '路径过长' });
            return;
        }

        if (!isAllowedPath(logPath, config.logDirs)) {
            res.status(403).json({ error: '不允许访问此文件' });
            return;
        }

        await logFileService.deleteLogFile(logPath);
        auditService.addAuditLog('log_delete', { path: safePath(logPath) }, req);
        res.json({ success: true, message: '日志文件已删除' });
    } catch (err) {
        next(err);
    }
});

router.post('/logs/clean', validateToken, validateCSRF, sensitiveActionRateLimit(3, 300000), async (req: Request, res: Response, next: NextFunction) => {
    try {
        const { threshold, days, action = 'truncate' } = req.body;

        if (!isValidAction(action)) {
            res.status(400).json({ error: '无效的操作类型' });
            return;
        }

        if (!threshold && !days) {
            res.status(400).json({ error: '请指定清理条件' });
            return;
        }

        let thresholdBytes: number | null = null;
        if (threshold && isValidThreshold(threshold)) {
            thresholdBytes = parseSizeThreshold(threshold);
        }

        const daysNum = days ? clamp(parseInt(days), 1, 365) : null;

        const results = await logFileService.cleanLogFiles({
            thresholdBytes,
            days: daysNum,
            action
        });

        auditService.addAuditLog('logs_clean', { action, threshold, days, cleaned: results.cleaned }, req);
        res.json(results);
    } catch (err) {
        next(err);
    }
});

router.post('/logs/backup', validateToken, validateCSRF, sensitiveActionRateLimit(3, 600000), async (req: Request, res: Response, next: NextFunction) => {
    try {
        const results = await backupService.performBackup();
        auditService.addAuditLog('logs_backup', {
            backupPath: results.backupPath,
            files: results.files,
            backupSize: results.backupSize
        }, req);
        res.json(results);
    } catch (err) {
        next(err);
    }
});

router.get('/backups/list', validateToken, async (_req: Request, res: Response, next: NextFunction) => {
    try {
        const backups = await backupService.listBackups();
        res.json({ backups, total: backups.length });
    } catch (err) {
        next(err);
    }
});

router.post('/backups/delete', validateToken, requireAdmin, validateCSRF, sensitiveActionRateLimit(5, 300000), [
    body('path').notEmpty().isString().isLength({ max: MAX_PATH_LENGTH })
], async (req: Request, res: Response, next: NextFunction) => {
    try {
        const { path: backupPath } = req.body;

        if (!backupPath) {
            res.status(400).json({ error: '缺少备份文件路径' });
            return;
        }

        if (backupPath.length > MAX_PATH_LENGTH) {
            res.status(400).json({ error: '路径过长' });
            return;
        }

        // 验证备份文件路径在允许的备份目录内
        const backupBaseDir = config.backup.baseDir;
        const normalizedBackupPath = safePath(backupPath);
        if (!normalizedBackupPath || !normalizedBackupPath.startsWith(backupBaseDir + '/')) {
            res.status(403).json({ error: '不允许访问此备份文件' });
            return;
        }

        await backupService.deleteBackup(backupPath);
        auditService.addAuditLog('backup_delete', { path: backupPath }, req);
        res.json({ success: true, message: '备份已删除' });
    } catch (err) {
        next(err);
    }
});

router.post('/backups/clean', validateToken, requireAdmin, validateCSRF, sensitiveActionRateLimit(3, 300000), [
    body('days').optional().isInt({ min: 1, max: 365 })
], async (req: Request, res: Response, next: NextFunction) => {
    try {
        const { days = 30 } = req.body;
        const daysNum = clamp(parseInt(days) || 30, 1, 365);
        const deleted = await backupService.cleanOldBackups(daysNum);
        auditService.addAuditLog('backups_clean', { days: daysNum, deleted }, req);
        res.json({ success: true, deleted, message: `已删除 ${deleted} 个旧备份` });
    } catch (err) {
        next(err);
    }
});

router.get('/archives/list', validateToken, [
    query('limit').optional().isInt({ min: 1, max: 200 })
], async (req: Request, res: Response, next: NextFunction) => {
    try {
        const limitStr = getQueryParam(req.query.limit) || '50';
        const limitNum = clamp(parseInt(limitStr) || 50, 1, 200);

        const archives = await logFileService.listArchiveFiles(limitNum);
        res.json({ archives, total: archives.length });
    } catch (err) {
        next(err);
    }
});

router.get('/archive/content', validateToken, [
    query('path').notEmpty().isString().isLength({ max: MAX_PATH_LENGTH }),
    query('lines').optional().isInt({ min: 1, max: 500 })
], async (req: Request, res: Response, next: NextFunction) => {
    try {
        const archivePathStr = getQueryParam(req.query.path);
        if (!archivePathStr) {
            res.status(400).json({ error: '缺少文件路径' });
            return;
        }

        if (archivePathStr.length > MAX_PATH_LENGTH) {
            res.status(400).json({ error: '路径过长' });
            return;
        }

        if (!isAllowedPath(archivePathStr, config.logDirs)) {
            res.status(403).json({ error: '不允许访问此文件' });
            return;
        }

        const linesStr = getQueryParam(req.query.lines) || '50';
        const linesNum = clamp(parseInt(linesStr) || 50, 1, 500);

        const archiveService = await import('../services/archive');
        const result = await archiveService.readArchiveContent(archivePathStr, linesNum);

        res.json({
            content: filterSensitiveInfo(result.content),
            truncated: result.truncated
        });
    } catch (err) {
        next(err);
    }
});

router.post('/archives/delete', validateToken, validateCSRF, sensitiveActionRateLimit(5, 300000), [
    body('path').notEmpty().isString().isLength({ max: MAX_PATH_LENGTH })
], async (req: Request, res: Response, next: NextFunction) => {
    try {
        const { path: archivePath } = req.body;

        if (!archivePath) {
            res.status(400).json({ error: '缺少文件路径' });
            return;
        }

        if (archivePath.length > MAX_PATH_LENGTH) {
            res.status(400).json({ error: '路径过长' });
            return;
        }

        if (!isAllowedPath(archivePath, config.logDirs)) {
            res.status(403).json({ error: '不允许访问此文件' });
            return;
        }

        await logFileService.deleteLogFile(archivePath);
        auditService.addAuditLog('archive_delete', { path: safePath(archivePath) }, req);
        res.json({ success: true, message: '归档文件已删除' });
    } catch (err) {
        next(err);
    }
});

router.get('/audit/log', validateToken, async (_req: Request, res: Response) => {
    await auditService.cleanOldAuditLogs();
    const logs = await auditService.getAuditLogs(100);
    res.json({ logs });
});

router.post('/dirs/clean-empty', validateToken, requireAdmin, validateCSRF, sensitiveActionRateLimit(3, 300000), async (req: Request, res: Response, next: NextFunction) => {
    try {
        const result = await logFileService.cleanEmptyAppDirs();
        auditService.addAuditLog('dirs_clean_empty', {
            cleaned: result.cleaned,
            dirs: result.dirs
        }, req);
        res.json({
            success: true,
            cleaned: result.cleaned,
            dirs: result.dirs,
            errors: result.errors,
            message: `已删除 ${result.cleaned} 个空文件夹`
        });
    } catch (err) {
        next(err);
    }
});

router.get('/auto-clean/rules', validateToken, async (_req: Request, res: Response, next: NextFunction) => {
    try {
        const rules = await autoCleanService.getRules();
        res.json({ rules });
    } catch (err) {
        next(err);
    }
});

router.post('/auto-clean/rules', validateToken, validateCSRF, sensitiveActionRateLimit(10, 300000), async (req: Request, res: Response, next: NextFunction) => {
    try {
        const { name, enabled, type, threshold, days, schedule } = req.body;

        if (!name || typeof name !== 'string') {
            res.status(400).json({ error: '规则名称不能为空' });
            return;
        }

        if (!['truncateLarge', 'deleteOld', 'deleteUninstalled'].includes(type)) {
            res.status(400).json({ error: '无效的清理类型' });
            return;
        }

        if (!['hourly', 'daily', 'weekly'].includes(schedule) && isNaN(parseInt(schedule, 10))) {
            res.status(400).json({ error: '无效的调度计划' });
            return;
        }

        if (schedule && !isNaN(parseInt(schedule, 10)) && parseInt(schedule, 10) < 60000) {
            res.status(400).json({ error: '自定义间隔最小为60000毫秒(1分钟)' });
            return;
        }

        if (type === 'truncateLarge' && threshold && !isValidThreshold(threshold)) {
            res.status(400).json({ error: '无效的大小阈值' });
            return;
        }

        if (type === 'deleteOld' && days && !isValidDays(days)) {
            res.status(400).json({ error: '天数必须在1-365之间' });
            return;
        }

        const rule = await autoCleanService.addRule({
            name,
            enabled: enabled !== false,
            type,
            threshold: type === 'truncateLarge' ? threshold : undefined,
            days: type === 'deleteOld' ? days : undefined,
            schedule
        });

        auditService.addAuditLog('auto_clean_rule_add', { ruleId: rule.id, name: rule.name, type: rule.type }, req);
        res.json({ rule });
    } catch (err) {
        next(err);
    }
});

router.put('/auto-clean/rules/:id', validateToken, validateCSRF, sensitiveActionRateLimit(10, 300000), async (req: Request, res: Response, next: NextFunction) => {
    try {
        const { id } = req.params;
        const updates = req.body;

        if (updates.type && !['truncateLarge', 'deleteOld', 'deleteUninstalled'].includes(updates.type)) {
            res.status(400).json({ error: '无效的清理类型' });
            return;
        }

        if (updates.schedule && !['hourly', 'daily', 'weekly'].includes(updates.schedule) && isNaN(parseInt(updates.schedule, 10))) {
            res.status(400).json({ error: '无效的调度计划' });
            return;
        }

        if (updates.threshold && !isValidThreshold(updates.threshold)) {
            res.status(400).json({ error: '无效的大小阈值' });
            return;
        }

        if (updates.days && !isValidDays(updates.days)) {
            res.status(400).json({ error: '天数必须在1-365之间' });
            return;
        }

        const rule = await autoCleanService.updateRule(id, updates);
        if (!rule) {
            res.status(404).json({ error: '规则不存在' });
            return;
        }

        auditService.addAuditLog('auto_clean_rule_update', { ruleId: id, updates }, req);
        res.json({ rule });
    } catch (err) {
        next(err);
    }
});

router.delete('/auto-clean/rules/:id', validateToken, validateCSRF, sensitiveActionRateLimit(10, 300000), async (req: Request, res: Response, next: NextFunction) => {
    try {
        const { id } = req.params;
        const deleted = await autoCleanService.deleteRule(id);
        if (!deleted) {
            res.status(404).json({ error: '规则不存在' });
            return;
        }

        auditService.addAuditLog('auto_clean_rule_delete', { ruleId: id }, req);
        res.json({ success: true });
    } catch (err) {
        next(err);
    }
});

router.post('/auto-clean/rules/:id/toggle', validateToken, validateCSRF, sensitiveActionRateLimit(10, 300000), async (req: Request, res: Response, next: NextFunction) => {
    try {
        const { id } = req.params;
        const rule = await autoCleanService.toggleRule(id);
        if (!rule) {
            res.status(404).json({ error: '规则不存在' });
            return;
        }

        auditService.addAuditLog('auto_clean_rule_toggle', { ruleId: id, enabled: rule.enabled }, req);
        res.json({ rule });
    } catch (err) {
        next(err);
    }
});

router.post('/auto-clean/rules/:id/execute', validateToken, validateCSRF, sensitiveActionRateLimit(5, 300000), async (req: Request, res: Response, next: NextFunction) => {
    try {
        const { id } = req.params;
        const result = await autoCleanService.executeRuleNow(id);
        if (!result) {
            res.status(404).json({ error: '规则不存在' });
            return;
        }

        auditService.addAuditLog('auto_clean_manual', { ruleId: id, cleaned: result.cleaned }, req);
        res.json(result);
    } catch (err) {
        next(err);
    }
});

router.get('/bookmarks', validateToken, async (_req: Request, res: Response, next: NextFunction) => {
    try {
        const bookmarks = await bookmarkService.loadBookmarks();
        res.json({ bookmarks });
    } catch (err) {
        next(err);
    }
});

router.post('/bookmarks', validateToken, validateCSRF, async (req: Request, res: Response, next: NextFunction) => {
    try {
        const { name, path: bookmarkPath, isDocker } = req.body;

        if (!bookmarkPath || typeof bookmarkPath !== 'string') {
            res.status(400).json({ error: '缺少路径' });
            return;
        }

        if (bookmarkPath.length > MAX_PATH_LENGTH) {
            res.status(400).json({ error: '路径过长' });
            return;
        }

        if (isDocker) {
            if (!isValidContainerName(bookmarkPath)) {
                res.status(400).json({ error: '无效的容器名称' });
                return;
            }
        } else {
            if (!isAllowedPath(bookmarkPath, config.logDirs)) {
                res.status(403).json({ error: '不允许访问此路径' });
                return;
            }
        }

        const displayName = (name && typeof name === 'string') ? name : bookmarkPath.split('/').pop() || bookmarkPath;
        const bookmark = await bookmarkService.addBookmark(displayName, bookmarkPath, !!isDocker);
        auditService.addAuditLog('bookmark_add', { id: bookmark.id, path: safePath(bookmarkPath), isDocker: !!isDocker }, req);
        res.json({ bookmark });
    } catch (err) {
        next(err);
    }
});

router.delete('/bookmarks/:id', validateToken, validateCSRF, async (req: Request, res: Response, next: NextFunction) => {
    try {
        const { id } = req.params;
        const deleted = await bookmarkService.deleteBookmark(id);
        if (!deleted) {
            res.status(404).json({ error: '书签不存在' });
            return;
        }
        auditService.addAuditLog('bookmark_delete', { id }, req);
        res.json({ success: true });
    } catch (err) {
        next(err);
    }
});

router.put('/bookmarks/:id', validateToken, validateCSRF, async (req: Request, res: Response, next: NextFunction) => {
    try {
        const { id } = req.params;
        const { name } = req.body;

        if (!name || typeof name !== 'string') {
            res.status(400).json({ error: '名称不能为空' });
            return;
        }

        const bookmark = await bookmarkService.updateBookmark(id, name);
        if (!bookmark) {
            res.status(404).json({ error: '书签不存在' });
            return;
        }
        auditService.addAuditLog('bookmark_update', { id, name }, req);
        res.json({ bookmark });
    } catch (err) {
        next(err);
    }
});

export default router;
