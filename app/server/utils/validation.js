/**
 * @fileoverview 输入验证工具函数
 */

const path = require('path');

/**
 * 安全化路径输入 - 增强版防止路径遍历
 * @param {string} inputPath - 输入路径
 * @returns {string|null}
 */
function safePath(inputPath) {
    if (!inputPath || typeof inputPath !== 'string') return null;
    if (inputPath.length > 4096) return null;
    
    // 检查危险字符
    if (inputPath.includes('\0') || 
        inputPath.includes('\\') ||
        inputPath.includes('\r') ||
        inputPath.includes('\n')) {
        return null;
    }
    
    // 规范化路径
    const normalized = path.normalize(inputPath);
    
    // 检查路径遍历尝试
    if (normalized.includes('..')) return null;
    
    // 确保是绝对路径
    if (!normalized.startsWith('/')) return null;
    
    return normalized;
}

/**
 * 检查路径是否在允许的目录内
 * @param {string} inputPath - 输入路径
 * @param {string[]} allowedDirs - 允许的目录列表
 * @returns {boolean}
 */
function isAllowedPath(inputPath, allowedDirs) {
    if (!inputPath) return false;
    const normalized = safePath(inputPath);
    if (!normalized) return false;
    
    for (const allowedDir of allowedDirs) {
        if (normalized === allowedDir || normalized.startsWith(allowedDir + '/')) {
            return true;
        }
    }
    return false;
}

/**
 * 验证大小格式
 * @param {string} size - 大小字符串
 * @returns {boolean}
 */
function isValidSize(size) {
    if (!size || typeof size !== 'string') return false;
    return /^[0-9]+[KMGT]?$/i.test(size);
}

/**
 * 验证数字范围
 * @param {string|number} num - 数字
 * @param {number} min - 最小值
 * @param {number} max - 最大值
 * @returns {boolean}
 */
function isValidNumber(num, min, max) {
    const n = parseInt(num, 10);
    return !isNaN(n) && n >= min && n <= max;
}

/**
 * 验证容器名称
 * @param {string} name - 容器名称
 * @returns {boolean}
 */
function isValidContainerName(name) {
    if (!name || typeof name !== 'string') return false;
    if (name.length > 128) return false;
    return /^[a-zA-Z0-9][a-zA-Z0-9_.-]*$/.test(name);
}

/**
 * 验证相对路径
 * @param {string} p - 路径
 * @returns {boolean}
 */
function isValidRelativePath(p) {
    if (!p || typeof p !== 'string') return false;
    if (p.includes('..') || p.includes('\0')) return false;
    if (p.startsWith('/')) return false;
    return true;
}

/**
 * 格式化字节数
 * @param {number} bytes - 字节数
 * @returns {string}
 */
function formatBytes(bytes) {
    if (bytes === 0) return '0B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + sizes[i];
}

module.exports = {
    safePath,
    isAllowedPath,
    isValidSize,
    isValidNumber,
    isValidContainerName,
    isValidRelativePath,
    formatBytes
};
