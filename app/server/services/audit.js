/**
 * @fileoverview 审计日志服务 - 异步文件操作
 */

const fs = require('fs').promises; // ✅ 使用 Promise 版本
const config = require('../utils/config');
const { getClientIP } = require('../utils/ip');

/**
 * 添加审计日志（异步）
 * @param {string} action - 操作类型
 * @param {object} details - 详情
 * @param {import('express').Request} [req] - Express请求对象
 * @returns {Promise<void>}
 */
async function addAuditLog(action, details, req) {
    let ip = 'system';
    let userAgent = 'system';
    
    if (req) {
        ip = req.clientIP || getClientIP(req);
        userAgent = req.headers['user-agent'] || 'unknown';
    }
    
    const entry = {
        timestamp: new Date().toISOString(),
        action: action,
        details: details,
        ip: ip,
        userAgent: userAgent
    };
    
    try {
        // 确保目录存在
        try {
            await fs.mkdir(config.dataDir, { recursive: true, mode: 0o700 });
        } catch (e) {
            if (e.code !== 'EEXIST') throw e;
        }
        
        const logLine = JSON.stringify(entry) + '\n';
        await fs.appendFile(config.auditLogFile, logLine, { mode: 0o600 });
    } catch (e) {
        console.error('[LogManager] 写入审计日志失败:', e.message);
    }
}

/**
 * 获取审计日志（异步）
 * @param {number} limit - 限制数量
 * @returns {Promise<Array>}
 */
async function getAuditLogs(limit = 100) {
    try {
        const content = await fs.readFile(config.auditLogFile, 'utf8');
        const lines = content.trim().split('\n').filter(line => line);
        const logs = lines.slice(-limit * 10).map(line => {
            try {
                return JSON.parse(line);
            } catch (e) {
                return null;
            }
        }).filter(log => log);
        return logs.reverse().slice(0, limit);
    } catch (e) {
        if (e.code === 'ENOENT') {
            return [];
        }
        console.error('[LogManager] 读取审计日志失败:', e.message);
        return [];
    }
}

/**
 * 清理旧审计日志（异步）
 * @returns {Promise<void>}
 */
async function cleanOldAuditLogs() {
    try {
        const content = await fs.readFile(config.auditLogFile, 'utf8');
        const lines = content.trim().split('\n').filter(line => line);
        
        if (lines.length > config.audit.maxLogs) {
            const keptLines = lines.slice(-config.audit.maxLogs);
            await fs.writeFile(config.auditLogFile, keptLines.join('\n') + '\n', { mode: 0o600 });
        }
    } catch (e) {
        if (e.code === 'ENOENT') {
            return;
        }
        console.error('[LogManager] 清理审计日志失败:', e.message);
    }
}

setInterval(cleanOldAuditLogs, 3600000);

module.exports = {
    addAuditLog,
    getAuditLogs,
    cleanOldAuditLogs
};
