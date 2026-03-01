/**
 * @fileoverview 日志文件服务 - 使用原生 Node.js API
 */

const fs = require('fs');
const path = require('path');
const { promisify } = require('util');
const { pipeline } = require('stream/promises');
const readline = require('readline');
const config = require('../utils/config');
const { safePath, isAllowedPath, formatBytes } = require('../utils/validation');

const readdir = promisify(fs.readdir);
const stat = promisify(fs.stat);
const readFile = promisify(fs.readFile);
const writeFile = promisify(fs.writeFile);
const unlink = promisify(fs.unlink);

const LOG_EXTENSIONS = ['.log', '.log.'];
const ARCHIVE_EXTENSIONS = ['.gz', '.bz2', '.xz', '.zip', '.tar', '.tar.gz', '.tar.bz2', '.tar.xz', '.7z', '.rar'];
const MAX_PREVIEW_LINES = 5000;
const MAX_PREVIEW_SIZE = 10 * 1024 * 1024; // 10MB

let cachedInstalledApps = null;
let installedAppsCacheTime = 0;
const INSTALLED_APPS_CACHE_TTL = 60000; // 缓存60秒

/**
 * 执行命令
 * @param {string} cmd - 命令
 * @param {number} [timeout=30000] - 超时时间
 * @returns {Promise<{stdout: string, stderr: string}>}
 */
function execCommand(cmd, timeout = 30000) {
    return new Promise((resolve) => {
        const { exec } = require('child_process');
        exec(cmd, { timeout, maxBuffer: 10 * 1024 * 1024 }, (error, stdout, stderr) => {
            resolve({ stdout: stdout.toString(), stderr: stderr.toString() });
        });
    });
}

/**
 * 获取已安装应用列表（通过 appcenter-cli）
 * @returns {Promise<Set<string>>}
 */
async function getInstalledApps() {
    const now = Date.now();
    if (cachedInstalledApps && (now - installedAppsCacheTime) < INSTALLED_APPS_CACHE_TTL) {
        return cachedInstalledApps;
    }
    
    const apps = new Set();
    
    try {
        const { stdout } = await execCommand('appcenter-cli list 2>/dev/null', 10000);
        if (stdout && stdout.trim()) {
            const lines = stdout.trim().split('\n');
            for (const line of lines) {
                if (line.includes('│') && !line.includes('APP NAME') && !line.includes('────')) {
                    const parts = line.split('│');
                    if (parts.length >= 2) {
                        const appName = parts[1].trim();
                        if (appName && appName.length > 0) {
                            apps.add(appName);
                        }
                    }
                }
            }
        }
    } catch (e) {
        // 忽略错误
    }
    
    cachedInstalledApps = apps;
    installedAppsCacheTime = now;
    return apps;
}

/**
 * @typedef {Object} LogFile
 * @property {string} path - 文件路径
 * @property {number} size - 文件大小
 * @property {string} sizeFormatted - 格式化后的大小
 * @property {Date} modified - 修改时间
 * @property {string} [appName] - 应用名称
 * @property {boolean} [canDelete] - 是否可删除
 */

/**
 * 从路径提取应用名称
 * @param {string} logPath - 日志路径
 * @returns {string|null}
 */
function extractAppNameFromPath(logPath) {
    const parts = logPath.split('/');
    for (let i = 0; i < parts.length; i++) {
        if (parts[i].startsWith('@')) {
            if (parts[i + 1]) {
                return parts[i + 1];
            }
        }
    }
    
    if (logPath.startsWith('/var/log/apps/')) {
        const rest = logPath.slice('/var/log/apps/'.length);
        let appName = rest.split('/')[0];
        if (appName) {
            appName = appName.replace(/\.log(-\d{8})?(\.\d+)?\.(gz|bz2|xz|zip|tar(\.gz|\.bz2|\.xz)?|7z|rar)$/i, '');
            appName = appName.replace(/\.log$/i, '');
            return appName || null;
        }
    }
    
    const appdataMatch = logPath.match(/\/vol\d+\/@appdata\/([^\/]+)/);
    if (appdataMatch) return appdataMatch[1];
    
    const appshareMatch = logPath.match(/\/vol\d+\/@appshare\/([^\/]+)/);
    if (appshareMatch) return appshareMatch[1];
    
    return null;
}

/**
 * 检查文件是否为日志文件
 * @param {string} filename - 文件名
 * @returns {boolean}
 */
function isLogFile(filename) {
    const lower = filename.toLowerCase();
    if (lower.endsWith('.log')) return true;
    if (lower.includes('.log.')) return true;
    if (lower.includes('log') && lower.endsWith('.txt')) return true;
    return false;
}

/**
 * 检查文件是否为归档文件
 * @param {string} filename - 文件名
 * @returns {boolean}
 */
function isArchiveFile(filename) {
    const lower = filename.toLowerCase();
    return ARCHIVE_EXTENSIONS.some(ext => lower.endsWith(ext));
}

/**
 * 递归遍历目录查找文件
 * @param {string} dir - 目录路径
 * @param {Function} filterFn - 过滤函数
 * @param {number} limit - 限制数量
 * @returns {Promise<string[]>}
 */
async function findFiles(dir, filterFn, limit = 100) {
    const results = [];
    
    async function walk(currentDir) {
        if (results.length >= limit) return;
        
        try {
            const entries = await readdir(currentDir, { withFileTypes: true });
            for (const entry of entries) {
                if (results.length >= limit) break;
                
                const fullPath = path.join(currentDir, entry.name);
                
                if (entry.isDirectory()) {
                    await walk(fullPath);
                } else if (entry.isFile() && filterFn(entry.name)) {
                    results.push(fullPath);
                }
            }
        } catch (e) {
            // 忽略权限错误等
        }
    }
    
    await walk(dir);
    return results;
}

/**
 * 获取日志文件列表
 * @param {string} [dir] - 指定目录
 * @param {number} [limit=100] - 限制数量
 * @returns {Promise<LogFile[]>}
 */
async function listLogFiles(dir, limit = 100) {
    const searchDirs = dir ? [dir] : config.logDirs;
    const results = [];
    const installedApps = await getInstalledApps();
    
    for (const searchDir of searchDirs) {
        const normalizedDir = safePath(searchDir);
        if (!normalizedDir || !fs.existsSync(normalizedDir)) continue;
        if (dir && !isAllowedPath(dir, config.logDirs)) continue;
        
        const files = await findFiles(normalizedDir, isLogFile, limit);
        
        for (const file of files) {
            try {
                const stats = await stat(file);
                const appName = extractAppNameFromPath(file);
                const canDelete = appName ? !installedApps.has(appName) : false;
                
                results.push({
                    path: file,
                    size: stats.size,
                    sizeFormatted: formatBytes(stats.size),
                    modified: stats.mtime,
                    appName: appName,
                    canDelete: canDelete
                });
            } catch (e) {
                // 忽略无法访问的文件
            }
        }
    }
    
    return results.slice(0, limit);
}

/**
 * 获取归档文件列表
 * @param {number} [limit=50] - 限制数量
 * @returns {Promise<Array>}
 */
async function listArchiveFiles(limit = 50) {
    const results = [];
    
    for (const dir of config.logDirs) {
        const normalizedDir = safePath(dir);
        if (!normalizedDir || !fs.existsSync(normalizedDir)) continue;
        
        const files = await findFiles(normalizedDir, isArchiveFile, limit);
        
        for (const file of files) {
            try {
                const stats = await stat(file);
                results.push({
                    path: file,
                    size: stats.size,
                    sizeFormatted: formatBytes(stats.size),
                    modified: stats.mtime,
                    type: path.extname(file)
                });
            } catch (e) {
                // 忽略无法访问的文件
            }
        }
    }
    
    return results.slice(0, limit);
}

/**
 * 获取大日志文件
 * @param {number} thresholdBytes - 大小阈值（字节）
 * @param {number} [limit=50] - 限制数量
 * @returns {Promise<LogFile[]>}
 */
async function listLargeLogFiles(thresholdBytes, limit = 50) {
    const results = [];
    
    for (const dir of config.logDirs) {
        const normalizedDir = safePath(dir);
        if (!normalizedDir || !fs.existsSync(normalizedDir)) continue;
        
        const files = await findFiles(normalizedDir, isLogFile, limit * 2);
        
        for (const file of files) {
            try {
                const stats = await stat(file);
                if (stats.size >= thresholdBytes) {
                    results.push({
                        path: file,
                        size: stats.size,
                        sizeFormatted: formatBytes(stats.size),
                        modified: stats.mtime
                    });
                }
            } catch (e) {
                // 忽略
            }
        }
    }
    
    results.sort((a, b) => b.size - a.size);
    return results.slice(0, limit);
}

/**
 * 按名称搜索日志文件
 * @param {string} pattern - 文件名模式
 * @param {number} [limit=50] - 限制数量
 * @returns {Promise<LogFile[]>}
 */
async function searchLogFilesByName(pattern, limit = 50) {
    const results = [];
    const safePattern = pattern.replace(/[^a-zA-Z0-9_\-\.\*\?\[\]\{\}]/g, '');
    if (!safePattern) return results;
    
    const regex = new RegExp(safePattern, 'i');
    
    for (const dir of config.logDirs) {
        const normalizedDir = safePath(dir);
        if (!normalizedDir || !fs.existsSync(normalizedDir)) continue;
        
        const files = await findFiles(normalizedDir, (name) => {
            return isLogFile(name) && regex.test(name);
        }, limit);
        
        for (const file of files) {
            try {
                const stats = await stat(file);
                results.push({
                    path: file,
                    size: stats.size,
                    sizeFormatted: formatBytes(stats.size),
                    modified: stats.mtime,
                    appName: extractAppNameFromPath(file)
                });
            } catch (e) {
                // 忽略
            }
        }
    }
    
    results.sort((a, b) => (b.modified || 0) - (a.modified || 0));
    return results.slice(0, limit);
}

/**
 * 获取日志统计信息
 * @returns {Promise<{totalLogs: number, totalArchives: number, largeFiles: number, totalSize: number, totalSizeFormatted: string}>}
 */
async function getLogStats() {
    let totalLogs = 0;
    let totalArchives = 0;
    let largeFiles = 0;
    let totalSize = 0;
    const largeThreshold = 10 * 1024 * 1024; // 10MB
    
    for (const dir of config.logDirs) {
        const normalizedDir = safePath(dir);
        if (!normalizedDir || !fs.existsSync(normalizedDir)) continue;
        
        const logFiles = await findFiles(normalizedDir, isLogFile, 10000);
        totalLogs += logFiles.length;
        
        for (const file of logFiles) {
            try {
                const stats = await stat(file);
                totalSize += stats.size;
                if (stats.size >= largeThreshold) {
                    largeFiles++;
                }
            } catch (e) {
                // 忽略
            }
        }
        
        const archiveFiles = await findFiles(normalizedDir, isArchiveFile, 10000);
        totalArchives += archiveFiles.length;
    }
    
    return {
        totalLogs,
        totalArchives,
        largeFiles,
        totalSize,
        totalSizeFormatted: formatBytes(totalSize)
    };
}

/**
 * 读取日志文件内容（支持大文件流式读取）
 * @param {string} filePath - 文件路径
 * @param {Object} options - 选项
 * @param {number} [options.maxLines=5000] - 最大行数
 * @param {number} [options.offset=0] - 起始行偏移
 * @param {string} [options.tail=false] - 是否从尾部读取
 * @returns {Promise<{content: string, totalLines: number, size: number, sizeFormatted: string, truncated: boolean}>}
 */
async function readLogFile(filePath, options = {}) {
    const { maxLines = MAX_PREVIEW_LINES, offset = 0, tail = false } = options;
    const normalizedPath = safePath(filePath);
    
    if (!normalizedPath || !isAllowedPath(filePath, config.logDirs)) {
        throw new Error('不允许访问此文件');
    }
    
    const stats = await stat(normalizedPath);
    
    if (!stats.isFile()) {
        throw new Error('不是有效的文件');
    }
    
    // 对于小文件，直接读取
    if (stats.size <= MAX_PREVIEW_SIZE) {
        const content = await readFile(normalizedPath, 'utf8');
        const allLines = content.split('\n');
        const totalLines = allLines.length;
        
        let selectedLines;
        let truncated = false;
        
        if (tail) {
            const start = Math.max(0, totalLines - maxLines);
            selectedLines = allLines.slice(start);
            truncated = start > 0;
        } else {
            const end = Math.min(totalLines, offset + maxLines);
            selectedLines = allLines.slice(offset, end);
            truncated = end < totalLines;
        }
        
        return {
            content: selectedLines.join('\n'),
            totalLines,
            size: stats.size,
            sizeFormatted: formatBytes(stats.size),
            truncated,
            hasMore: truncated
        };
    }
    
    // 大文件使用流式读取
    return await readLargeFileStreaming(normalizedPath, { maxLines, offset, tail, size: stats.size });
}

/**
 * 流式读取大文件
 * @param {string} filePath - 文件路径
 * @param {Object} options - 选项
 * @returns {Promise<{content: string, totalLines: number, size: number, sizeFormatted: string, truncated: boolean}>}
 */
async function readLargeFileStreaming(filePath, options) {
    const { maxLines = MAX_PREVIEW_LINES, offset = 0, tail = false, size } = options;
    
    return new Promise((resolve, reject) => {
        const stream = fs.createReadStream(filePath, { encoding: 'utf8' });
        const rl = readline.createInterface({
            input: stream,
            crlfDelay: Infinity
        });
        
        let totalLines = 0;
        let collectedLines = [];
        let truncated = false;
        let lineBuffer = [];
        
        rl.on('line', (line) => {
            totalLines++;
            
            if (tail) {
                // 尾部模式：保持缓冲区大小
                lineBuffer.push(line);
                if (lineBuffer.length > maxLines) {
                    lineBuffer.shift();
                }
            } else {
                // 头部模式：从偏移开始收集
                if (totalLines > offset && collectedLines.length < maxLines) {
                    collectedLines.push(line);
                }
            }
        });
        
        rl.on('close', () => {
            const content = tail ? lineBuffer.join('\n') : collectedLines.join('\n');
            truncated = tail ? lineBuffer.length < totalLines : (offset + maxLines) < totalLines;
            
            resolve({
                content,
                totalLines,
                size,
                sizeFormatted: formatBytes(size),
                truncated,
                hasMore: truncated
            });
        });
        
        rl.on('error', (err) => {
            reject(new Error(`读取文件失败: ${err.message}`));
        });
        
        stream.on('error', (err) => {
            reject(new Error(`文件流错误: ${err.message}`));
        });
    });
}

/**
 * 获取文件总行数（快速）
 * @param {string} filePath - 文件路径
 * @returns {Promise<number>}
 */
async function getFileLineCount(filePath) {
    const normalizedPath = safePath(filePath);
    if (!normalizedPath || !isAllowedPath(filePath, config.logDirs)) {
        throw new Error('不允许访问此文件');
    }
    
    return new Promise((resolve, reject) => {
        let lineCount = 0;
        const stream = fs.createReadStream(normalizedPath);
        
        stream.on('data', (chunk) => {
            for (let i = 0; i < chunk.length; i++) {
                if (chunk[i] === 10) { // \n
                    lineCount++;
                }
            }
        });
        
        stream.on('end', () => {
            resolve(lineCount);
        });
        
        stream.on('error', (err) => {
            reject(new Error(`读取文件失败: ${err.message}`));
        });
    });
}

/**
 * 清空日志文件
 * @param {string} filePath - 文件路径
 * @returns {Promise<void>}
 */
async function truncateLogFile(filePath) {
    const normalizedPath = safePath(filePath);
    if (!normalizedPath || !isAllowedPath(filePath, config.logDirs)) {
        throw new Error('不允许访问此文件');
    }
    
    try {
        const stats = await stat(normalizedPath);
        if (stats.isDirectory()) {
            throw new Error('不能清空目录');
        }
    } catch (e) {
        if (e.code === 'ENOENT') {
            throw new Error('文件不存在');
        }
        throw e;
    }
    
    await writeFile(normalizedPath, '');
}

/**
 * 删除日志文件
 * @param {string} filePath - 文件路径
 * @returns {Promise<void>}
 */
async function deleteLogFile(filePath) {
    const normalizedPath = safePath(filePath);
    if (!normalizedPath || !isAllowedPath(filePath, config.logDirs)) {
        throw new Error('不允许访问此文件');
    }
    
    const stats = await stat(normalizedPath);
    if (stats.isDirectory()) {
        throw new Error('不能删除目录');
    }
    
    await unlink(normalizedPath);
}

/**
 * 获取目录信息
 * @returns {Promise<Array>}
 */
async function getDirsInfo() {
    const results = [];
    
    for (const dir of config.logDirs) {
        try {
            const normalizedDir = safePath(dir);
            let exists = false;
            let logCount = 0;
            let archiveCount = 0;
            let totalSize = 0;
            
            if (normalizedDir) {
                try {
                    const stats = await stat(normalizedDir);
                    exists = stats.isDirectory();
                } catch (e) {
                    exists = false;
                }
                
                if (exists) {
                    const logFiles = await findFiles(normalizedDir, isLogFile, 10000);
                    logCount = logFiles.length;
                    
                    const archiveFiles = await findFiles(normalizedDir, isArchiveFile, 10000);
                    archiveCount = archiveFiles.length;
                    
                    for (const file of [...logFiles, ...archiveFiles]) {
                        try {
                            const stats = await stat(file);
                            totalSize += stats.size;
                        } catch (e) {
                            // 忽略
                        }
                    }
                }
            }
            
            results.push({
                path: dir,
                exists,
                logCount,
                archiveCount,
                totalSize: formatBytes(totalSize)
            });
        } catch (e) {
            results.push({ path: dir, exists: false, error: e.message });
        }
    }
    
    return results;
}

/**
 * 批量清理日志文件
 * @param {Object} options - 清理选项
 * @param {number} [options.thresholdBytes] - 大小阈值（字节）
 * @param {number} [options.days] - 天数阈值
 * @param {string} options.action - 操作类型 ('truncate' | 'delete')
 * @returns {Promise<{cleaned: number, errors: Array<string>}>}
 */
async function cleanLogFiles(options) {
    const { thresholdBytes, days, action } = options;
    const results = { cleaned: 0, errors: [] };
    const cutoffTime = days ? Date.now() - days * 24 * 60 * 60 * 1000 : 0;
    
    for (const dir of config.logDirs) {
        const normalizedDir = safePath(dir);
        if (!normalizedDir || !fs.existsSync(normalizedDir)) continue;
        
        try {
            // 根据条件收集文件
            const files = await findFiles(normalizedDir, (name) => {
                if (days) {
                    // 按天数清理归档文件
                    return isArchiveFile(name);
                } else {
                    // 按大小清理日志文件
                    return isLogFile(name);
                }
            }, 10000);
            
            for (const file of files) {
                try {
                    const stats = await stat(file);
                    
                    // 检查条件
                    if (thresholdBytes && stats.size < thresholdBytes) continue;
                    if (days && stats.mtime.getTime() > cutoffTime) continue;
                    
                    // 执行操作
                    if (action === 'delete') {
                        await unlink(file);
                    } else if (action === 'truncate') {
                        await writeFile(file, '');
                    }
                    
                    results.cleaned++;
                } catch (e) {
                    results.errors.push(`${file}: ${e.message}`);
                }
            }
        } catch (e) {
            results.errors.push(`${dir}: ${e.message}`);
        }
    }
    
    return results;
}

module.exports = {
    listLogFiles,
    listArchiveFiles,
    listLargeLogFiles,
    searchLogFilesByName,
    getLogStats,
    readLogFile,
    truncateLogFile,
    deleteLogFile,
    getDirsInfo,
    cleanLogFiles,
    getFileLineCount,
    extractAppNameFromPath,
    isLogFile,
    isArchiveFile,
    getInstalledApps
};
