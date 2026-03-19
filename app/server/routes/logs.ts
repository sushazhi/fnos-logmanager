import express, { Request, Response, NextFunction } from 'express';
import { query, body, ValidationChain } from 'express-validator';
import * as logFileService from '../services/logFile';
import * as auditService from '../services/audit';
import * as backupService from '../services/backup';
import { validateToken, validateCSRF, validate } from '../middleware/auth';
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
    clamp
} from '../utils/validation';
import config from '../utils/config';
import { ValidationError } from '../utils/errors';

const router = express.Router();

let FILTER_SENSITIVE = process.env.FILTER_SENSITIVE !== 'false';

const MAX_PATTERN_LENGTH = 100;
const MAX_PATH_LENGTH = 4096;

function filterSensitiveInfo(content: string): string {
    if (!FILTER_SENSITIVE) return content;
    if (!content || typeof content !== 'string') return content;
    let filtered = content;
    for (const pattern of config.sensitivePatterns) {
        filtered = filtered.replace(pattern, '[FILTERED]');
    }
    return filtered;
}

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
    res.json({ enabled: FILTER_SENSITIVE });
});

router.post('/settings/filter', validateToken, validateCSRF, (req: Request, res: Response) => {
    const { enabled } = req.body;
    if (typeof enabled === 'boolean') {
        process.env.FILTER_SENSITIVE = enabled ? 'true' : 'false';
        FILTER_SENSITIVE = enabled;
        res.json({ success: true, enabled: FILTER_SENSITIVE });
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

        let thresholdBytes: number;
        const match = thresholdStr.match(/^([0-9]+)([KMGT]?)$/i);
        if (match) {
            const num = parseInt(match[1]);
            const unit = (match[2] || '').toUpperCase();
            const multipliers: Record<string, number> = { '': 1, 'K': 1024, 'M': 1024 * 1024, 'G': 1024 * 1024 * 1024, 'T': 1024 * 1024 * 1024 * 1024 };
            thresholdBytes = num * (multipliers[unit] || 1);
        } else {
            thresholdBytes = 10 * 1024 * 1024;
        }

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

            let thresholdBytes: number;
            const match = sizeThreshold.match(/^([0-9]+)([KMGT]?)$/i);
            if (match) {
                const num = parseInt(match[1]);
                const unit = (match[2] || '').toUpperCase();
                const multipliers: Record<string, number> = { '': 1, 'K': 1024, 'M': 1024 * 1024, 'G': 1024 * 1024 * 1024, 'T': 1024 * 1024 * 1024 * 1024 };
                thresholdBytes = num * (multipliers[unit] || 1);
            } else {
                thresholdBytes = 10 * 1024 * 1024;
            }

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

router.get('/log/content', validateToken, [
    query('path').notEmpty().isString().isLength({ max: MAX_PATH_LENGTH }),
    query('maxLines').optional().isInt({ min: 100, max: 50000 }),
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
            maxLines: clamp(maxLinesStr ? parseInt(maxLinesStr) : 5000, 100, 50000),
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
            const match = threshold.match(/^([0-9]+)([KMGT]?)$/i);
            if (match) {
                const num = parseInt(match[1]);
                const unit = (match[2] || '').toUpperCase();
                const multipliers: Record<string, number> = { '': 1, 'K': 1024, 'M': 1024 * 1024, 'G': 1024 * 1024 * 1024, 'T': 1024 * 1024 * 1024 * 1024 };
                thresholdBytes = num * (multipliers[unit] || 1);
            }
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

router.post('/backups/delete', validateToken, validateCSRF, sensitiveActionRateLimit(5, 300000), [
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

        await backupService.deleteBackup(backupPath);
        auditService.addAuditLog('backup_delete', { path: backupPath }, req);
        res.json({ success: true, message: '备份已删除' });
    } catch (err) {
        next(err);
    }
});

router.post('/backups/clean', validateToken, validateCSRF, sensitiveActionRateLimit(3, 300000), [
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

export default router;
