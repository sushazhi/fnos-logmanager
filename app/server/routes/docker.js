/**
 * @fileoverview Docker 路由 - 使用 spawn 避免命令注入
 */

const express = require('express');
const router = express.Router();
const { spawn } = require('child_process');
const { query } = require('express-validator');
const { validateToken } = require('../middleware/auth');
const { isValidNumber, isValidContainerName } = require('../utils/validation');
const config = require('../utils/config');

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
 * 安全执行 Docker 命令（使用 spawn 避免 shell 注入）
 * @param {string[]} args - 命令参数
 * @param {number} [timeout=60000] - 超时时间
 * @returns {Promise<string>}
 */
function execDocker(args, timeout = 60000) {
    return new Promise((resolve, reject) => {
        const proc = spawn('docker', args, {
            timeout: timeout
        });
        
        let stdout = '';
        let stderr = '';
        
        proc.stdout.on('data', (data) => {
            stdout += data.toString();
        });
        
        proc.stderr.on('data', (data) => {
            stderr += data.toString();
        });
        
        const timer = setTimeout(() => {
            proc.kill();
            reject(new Error('命令执行超时'));
        }, timeout);
        
        proc.on('close', (code) => {
            clearTimeout(timer);
            if (code === 0) {
                // 合并 stdout 和 stderr（Docker 日志可能输出到两者）
                resolve(stdout + stderr);
            } else {
                reject(new Error(stderr || `Docker 命令退出码: ${code}`));
            }
        });
        
        proc.on('error', (err) => {
            clearTimeout(timer);
            reject(new Error(`Docker 命令执行失败: ${err.message}`));
        });
    });
}

/**
 * @route GET /api/docker/containers
 * @description 获取Docker容器列表
 */
router.get('/docker/containers', validateToken, async (req, res, next) => {
    try {
        const stdout = await execDocker(['ps', '--format', '{{.Names}}\t{{.Status}}\t{{.Image}}'], 10000);
        const containers = stdout.trim().split('\n').filter(line => line).map(line => {
            const parts = line.split('\t');
            const name = parts[0] || '';
            if (!isValidContainerName(name)) return null;
            return {
                name,
                status: parts[1] || '',
                image: parts[2] || ''
            };
        }).filter(c => c);
        res.json({ containers });
    } catch (e) {
        res.json({ containers: [], error: 'Docker未安装或未运行' });
    }
});

/**
 * @route GET /api/docker/logs
 * @description 获取Docker容器日志
 */
router.get('/docker/logs', validateToken, [
    query('container').notEmpty().isString(),
    query('lines').optional().isInt({ min: 1, max: 50000 })
], async (req, res, next) => {
    try {
        const { container, lines } = req.query;
        
        if (!container) {
            return res.status(400).json({ error: '缺少容器名称' });
        }
        
        if (!isValidContainerName(container)) {
            return res.status(400).json({ error: '无效的容器名称' });
        }
        
        // 如果不指定行数或为0，获取全部日志
        let args;
        if (lines && parseInt(lines) > 0) {
            const linesNum = Math.min(parseInt(lines), 50000);
            args = ['logs', container, '--tail', String(linesNum)];
        } else {
            // 获取全部日志
            args = ['logs', container];
        }
        
        // 使用 spawn 传递参数，避免 shell 注入
        const stdout = await execDocker(args, 120000);
        res.json({ logs: filterSensitiveInfo(stdout) });
    } catch (e) {
        res.status(500).json({ error: e.message });
    }
});

module.exports = router;
