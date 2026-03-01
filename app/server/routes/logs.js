/**
 * @fileoverview 日志路由
 */

const express = require('express');
const router = express.Router();
const { query, body } = require('express-validator');
const logFileService = require('../services/logFile');
const auditService = require('../services/audit');
const backupService = require('../services/backup');
const { validateToken, validateCSRF, validate } = require('../middleware/auth');
const { safePath, isAllowedPath, isValidSize, isValidNumber } = require('../utils/validation');
const config = require('../utils/config');
const { NotFoundError, ValidationError } = require('../utils/errors');

let FILTER_SENSITIVE = process.env.FILTER_SENSITIVE !== 'false';

/**
 * 过滤敏感信息
 * @param {string} content - 内容
 * @returns {string}
 */
function filterSensitiveInfo(content) {
    if (!FILTER_SENSITIVE) return content;
    if (!content || typeof content !== 'string') return content;
    let filtered = content;
    for (const pattern of config.sensitivePatterns) {
        filtered = filtered.replace(pattern, '[FILTERED]');
    }
    return filtered;
}

/**
 * @route GET /api/health
 * @description 健康检查
 */
router.get('/health', (req, res) => {
    res.json({ status: 'ok', version: '1.0.0' });
});

/**
 * @route GET /api/settings/filter
 * @description 获取敏感信息过滤设置
 */
router.get('/settings/filter', validateToken, (req, res) => {
    res.json({ enabled: FILTER_SENSITIVE });
});

/**
 * @route POST /api/settings/filter
 * @description 设置敏感信息过滤
 */
router.post('/settings/filter', validateToken, validateCSRF, (req, res) => {
    const { enabled } = req.body;
    if (typeof enabled === 'boolean') {
        process.env.FILTER_SENSITIVE = enabled ? 'true' : 'false';
        FILTER_SENSITIVE = enabled;
        res.json({ success: true, enabled: FILTER_SENSITIVE });
    } else {
        res.status(400).json({ error: '无效的参数' });
    }
});

/**
 * @route GET /api/dirs
 * @description 获取日志目录列表
 */
router.get('/dirs', validateToken, async (req, res, next) => {
    try {
        const dirs = await logFileService.getDirsInfo();
        res.json({ dirs });
    } catch (err) {
        next(err);
    }
});

/**
 * @route GET /api/logs/list
 * @description 获取日志文件列表
 */
router.get('/logs/list', validateToken, [
    query('dir').optional().isString(),
    query('limit').optional().isInt({ min: 1, max: 500 })
], async (req, res, next) => {
    try {
        const { dir, limit = 100 } = req.query;
        const limitNum = isValidNumber(limit, 1, 500) ? parseInt(limit) : 100;
        
        if (dir && !isAllowedPath(dir, config.logDirs)) {
            return res.status(403).json({ error: '不允许访问此目录' });
        }
        
        const logs = await logFileService.listLogFiles(dir, limitNum);
        res.json({ logs: logs, total: logs.length });
    } catch (err) {
        next(err);
    }
});

/**
 * @route GET /api/logs/large
 * @description 获取大日志文件列表
 */
router.get('/logs/large', validateToken, [
    query('threshold').optional().matches(/^[0-9]+[KMGT]?$/i),
    query('limit').optional().isInt({ min: 1, max: 200 })
], async (req, res, next) => {
    try {
        const { threshold = '10M', limit = 50 } = req.query;
        const limitNum = isValidNumber(limit, 1, 200) ? parseInt(limit) : 50;
        
        let thresholdBytes;
        const match = threshold.match(/^([0-9]+)([KMGT]?)$/i);
        if (match) {
            const num = parseInt(match[1]);
            const unit = (match[2] || '').toUpperCase();
            const multipliers = { '': 1, 'K': 1024, 'M': 1024 * 1024, 'G': 1024 * 1024 * 1024, 'T': 1024 * 1024 * 1024 * 1024 };
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

/**
 * @route GET /api/logs/search
 * @description 搜索日志文件
 */
router.get('/logs/search', validateToken, [
    query('type').isIn(['size', 'name']),
    query('threshold').optional().matches(/^[0-9]+[KMGT]?$/i),
    query('pattern').optional().isString(),
    query('limit').optional().isInt({ min: 1, max: 200 })
], async (req, res, next) => {
    try {
        const { type, threshold, pattern, limit = 50 } = req.query;
        const limitNum = isValidNumber(limit, 1, 200) ? parseInt(limit) : 50;
        
        let logs = [];
        
        if (type === 'size') {
            const sizeThreshold = threshold || '10M';
            if (!isValidSize(sizeThreshold)) {
                return res.status(400).json({ error: '无效的大小阈值' });
            }
            
            let thresholdBytes;
            const match = sizeThreshold.match(/^([0-9]+)([KMGT]?)$/i);
            if (match) {
                const num = parseInt(match[1]);
                const unit = (match[2] || '').toUpperCase();
                const multipliers = { '': 1, 'K': 1024, 'M': 1024 * 1024, 'G': 1024 * 1024 * 1024, 'T': 1024 * 1024 * 1024 * 1024 };
                thresholdBytes = num * (multipliers[unit] || 1);
            } else {
                thresholdBytes = 10 * 1024 * 1024;
            }
            
            logs = await logFileService.listLargeLogFiles(thresholdBytes, limitNum);
        } else if (type === 'name') {
            if (!pattern || typeof pattern !== 'string') {
                return res.status(400).json({ error: '请输入文件名模式' });
            }
            logs = await logFileService.searchLogFilesByName(pattern, limitNum);
        }
        
        res.json({ logs: logs.slice(0, limitNum), total: logs.length });
    } catch (err) {
        next(err);
    }
});

/**
 * @route GET /api/logs/stats
 * @description 获取日志统计信息
 */
router.get('/logs/stats', validateToken, async (req, res, next) => {
    try {
        const stats = await logFileService.getLogStats();
        res.json(stats);
    } catch (err) {
        next(err);
    }
});

/**
 * @route GET /api/log/content
 * @description 获取日志文件内容（支持流式读取大文件）
 */
router.get('/log/content', validateToken, [
    query('path').notEmpty().isString(),
    query('maxLines').optional().isInt({ min: 100, max: 50000 }),
    query('offset').optional().isInt({ min: 0 }),
    query('tail').optional().isBoolean()
], async (req, res, next) => {
    try {
        const { path: logPath, maxLines, offset, tail } = req.query;
        
        if (!logPath) {
            return res.status(400).json({ error: '缺少文件路径' });
        }
        
        if (!isAllowedPath(logPath, config.logDirs)) {
            return res.status(403).json({ error: '不允许访问此文件' });
        }
        
        const options = {
            maxLines: maxLines ? parseInt(maxLines) : 5000,
            offset: offset ? parseInt(offset) : 0,
            tail: tail === 'true' || tail === true
        };
        
        const result = await logFileService.readLogFile(logPath, options);
        res.json({
            content: filterSensitiveInfo(result.content),
            totalLines: result.totalLines,
            size: result.size,
            sizeFormatted: result.sizeFormatted,
            truncated: result.truncated,
            hasMore: result.hasMore
        });
    } catch (err) {
        if (err.size) {
            return res.status(400).json({
                error: err.message,
                size: err.size,
                sizeFormatted: err.sizeFormatted
            });
        }
        next(err);
    }
});

/**
 * @route POST /api/log/truncate
 * @description 清空日志文件
 */
router.post('/log/truncate', validateToken, validateCSRF, [
    body('path').notEmpty().isString()
], async (req, res, next) => {
    try {
        const { path: logPath } = req.body;
        
        if (!logPath) {
            return res.status(400).json({ error: '缺少文件路径' });
        }
        
        if (!isAllowedPath(logPath, config.logDirs)) {
            return res.status(403).json({ error: '不允许访问此文件' });
        }
        
        await logFileService.truncateLogFile(logPath);
        auditService.addAuditLog('log_truncate', { path: safePath(logPath) }, req);
        res.json({ success: true, message: '日志已清空' });
    } catch (err) {
        next(err);
    }
});

/**
 * @route POST /api/log/delete
 * @description 删除日志文件
 */
router.post('/log/delete', validateToken, validateCSRF, [
    body('path').notEmpty().isString()
], async (req, res, next) => {
    try {
        const { path: logPath } = req.body;
        
        if (!logPath) {
            return res.status(400).json({ error: '缺少文件路径' });
        }
        
        if (!isAllowedPath(logPath, config.logDirs)) {
            return res.status(403).json({ error: '不允许访问此文件' });
        }
        
        await logFileService.deleteLogFile(logPath);
        auditService.addAuditLog('log_delete', { path: safePath(logPath) }, req);
        res.json({ success: true, message: '日志文件已删除' });
    } catch (err) {
        next(err);
    }
});

/**
 * @route POST /api/logs/clean
 * @description 批量清理日志
 */
router.post('/logs/clean', validateToken, validateCSRF, async (req, res, next) => {
    try {
        const { threshold, days, action = 'truncate' } = req.body;
        
        if (action !== 'truncate' && action !== 'delete') {
            return res.status(400).json({ error: '无效的操作类型' });
        }
        
        if (!threshold && !days) {
            return res.status(400).json({ error: '请指定清理条件' });
        }
        
        // 解析大小阈值
        let thresholdBytes = null;
        if (threshold && isValidSize(threshold)) {
            const match = threshold.match(/^([0-9]+)([KMGT]?)$/i);
            if (match) {
                const num = parseInt(match[1]);
                const unit = (match[2] || '').toUpperCase();
                const multipliers = { '': 1, 'K': 1024, 'M': 1024 * 1024, 'G': 1024 * 1024 * 1024, 'T': 1024 * 1024 * 1024 * 1024 };
                thresholdBytes = num * (multipliers[unit] || 1);
            }
        }
        
        const results = await logFileService.cleanLogFiles({
            thresholdBytes,
            days: days ? parseInt(days) : null,
            action
        });
        
        auditService.addAuditLog('logs_clean', { action, threshold, days, cleaned: results.cleaned }, req);
        res.json(results);
    } catch (err) {
        next(err);
    }
});

/**
 * @route POST /api/logs/backup
 * @description 备份日志
 */
router.post('/logs/backup', validateToken, validateCSRF, async (req, res, next) => {
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

/**
 * @route GET /api/backups/list
 * @description 获取备份列表
 */
router.get('/backups/list', validateToken, async (req, res, next) => {
    try {
        const backups = await backupService.listBackups();
        res.json({ backups, total: backups.length });
    } catch (err) {
        next(err);
    }
});

/**
 * @route POST /api/backups/delete
 * @description 删除备份
 */
router.post('/backups/delete', validateToken, validateCSRF, [
    body('path').notEmpty().isString()
], async (req, res, next) => {
    try {
        const { path: backupPath } = req.body;
        
        if (!backupPath) {
            return res.status(400).json({ error: '缺少备份文件路径' });
        }
        
        await backupService.deleteBackup(backupPath);
        auditService.addAuditLog('backup_delete', { path: backupPath }, req);
        res.json({ success: true, message: '备份已删除' });
    } catch (err) {
        next(err);
    }
});

/**
 * @route POST /api/backups/clean
 * @description 清理旧备份
 */
router.post('/backups/clean', validateToken, validateCSRF, [
    body('days').optional().isInt({ min: 1, max: 365 })
], async (req, res, next) => {
    try {
        const { days = 30 } = req.body;
        const deleted = await backupService.cleanOldBackups(days);
        auditService.addAuditLog('backups_clean', { days, deleted }, req);
        res.json({ success: true, deleted, message: `已删除 ${deleted} 个旧备份` });
    } catch (err) {
        next(err);
    }
});

/**
 * @route GET /api/archives/list
 * @description 获取归档文件列表
 */
router.get('/archives/list', validateToken, [
    query('limit').optional().isInt({ min: 1, max: 200 })
], async (req, res, next) => {
    try {
        const { limit = 50 } = req.query;
        const limitNum = isValidNumber(limit, 1, 200) ? parseInt(limit) : 50;
        
        const archives = await logFileService.listArchiveFiles(limitNum);
        res.json({ archives, total: archives.length });
    } catch (err) {
        next(err);
    }
});

/**
 * @route GET /api/archive/content
 * @description 获取归档文件内容
 */
router.get('/archive/content', validateToken, [
    query('path').notEmpty().isString(),
    query('lines').optional().isInt({ min: 1, max: 500 })
], async (req, res, next) => {
    try {
        const { path: archivePath, lines = 50 } = req.query;
        
        if (!archivePath) {
            return res.status(400).json({ error: '缺少文件路径' });
        }
        
        const linesNum = isValidNumber(lines, 1, 500) ? parseInt(lines) : 50;
        
        const archiveService = require('../services/archive');
        const result = await archiveService.readArchiveContent(archivePath, linesNum);
        
        res.json({
            content: result.content,
            truncated: result.truncated
        });
    } catch (err) {
        next(err);
    }
});

/**
 * @route POST /api/archives/delete
 * @description 删除归档文件
 */
router.post('/archives/delete', validateToken, validateCSRF, [
    body('path').notEmpty().isString()
], async (req, res, next) => {
    try {
        const { path: archivePath } = req.body;
        
        if (!archivePath) {
            return res.status(400).json({ error: '缺少文件路径' });
        }
        
        if (!isAllowedPath(archivePath, config.logDirs)) {
            return res.status(403).json({ error: '不允许访问此文件' });
        }
        
        await logFileService.deleteLogFile(archivePath);
        auditService.addAuditLog('archive_delete', { path: safePath(archivePath) }, req);
        res.json({ success: true, message: '归档文件已删除' });
    } catch (err) {
        next(err);
    }
});

/**
 * @route GET /api/audit/log
 * @description 获取审计日志
 */
router.get('/audit/log', validateToken, async (req, res) => {
    await auditService.cleanOldAuditLogs();
    const logs = await auditService.getAuditLogs(100);
    res.json({ logs });
});

module.exports = router;
