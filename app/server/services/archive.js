/**
 * @fileoverview 归档文件服务 - 使用 spawn 避免 shell 注入
 */

const { spawn } = require('child_process');
const fs = require('fs');
const { promisify } = require('util');
const config = require('../utils/config');
const { safePath, isAllowedPath } = require('../utils/validation');

const stat = promisify(fs.stat);

/**
 * 安全执行解压命令（使用 spawn 避免 shell 注入）
 * @param {string} cmd - 命令
 * @param {string[]} args - 参数数组
 * @param {number} lines - 行数限制
 * @returns {Promise<string>}
 */
function execDecompress(cmd, args, lines) {
    return new Promise((resolve, reject) => {
        const proc = spawn(cmd, args, {
            stdio: ['ignore', 'pipe', 'pipe']
        });
        
        let stdout = '';
        let stderr = '';
        let lineCount = 0;
        const maxLines = lines || 50;
        
        proc.stdout.on('data', (data) => {
            if (lineCount < maxLines * 2) {
                stdout += data.toString();
                lineCount += data.toString().split('\n').length - 1;
            }
        });
        
        proc.stderr.on('data', (data) => {
            stderr += data.toString();
        });
        
        proc.on('close', (code) => {
            if (code !== 0 && !stdout) {
                reject(new Error(`解压失败: ${stderr || `退出码 ${code}`}`));
            } else {
                const outputLines = stdout.split('\n').slice(0, maxLines);
                resolve(outputLines.join('\n'));
            }
        });
        
        proc.on('error', (err) => {
            reject(new Error(`执行命令失败: ${err.message}`));
        });
    });
}

/**
 * 读取归档文件内容
 * @param {string} archivePath - 归档文件路径
 * @param {number} [lines=50] - 行数限制
 * @returns {Promise<{content: string, truncated: boolean}>}
 */
async function readArchiveContent(archivePath, lines = 50) {
    const safeArchivePath = safePath(archivePath);
    
    if (!safeArchivePath) {
        throw new Error('无效的文件路径');
    }
    
    if (!isAllowedPath(safeArchivePath, config.logDirs)) {
        throw new Error('不允许访问此文件');
    }
    
    const stats = await stat(safeArchivePath);
    if (!stats.isFile()) {
        throw new Error('不是有效的文件');
    }
    
    const lowerPath = safeArchivePath.toLowerCase();
    let content;
    
    try {
        // 使用 spawn 传递参数数组，避免 shell 注入
        if (lowerPath.endsWith('.tar.gz') || lowerPath.endsWith('.tgz')) {
            content = await execDecompress('tar', ['-xzf', safeArchivePath, '-O'], lines);
        } else if (lowerPath.endsWith('.tar.bz2') || lowerPath.endsWith('.tbz2')) {
            content = await execDecompress('tar', ['-xjf', safeArchivePath, '-O'], lines);
        } else if (lowerPath.endsWith('.tar.xz') || lowerPath.endsWith('.txz')) {
            content = await execDecompress('tar', ['-xJf', safeArchivePath, '-O'], lines);
        } else if (lowerPath.endsWith('.tar')) {
            content = await execDecompress('tar', ['-xf', safeArchivePath, '-O'], lines);
        } else if (lowerPath.endsWith('.gz')) {
            content = await execDecompress('zcat', [safeArchivePath], lines);
        } else if (lowerPath.endsWith('.bz2')) {
            content = await execDecompress('bzcat', [safeArchivePath], lines);
        } else if (lowerPath.endsWith('.xz')) {
            content = await execDecompress('xzcat', [safeArchivePath], lines);
        } else if (lowerPath.endsWith('.zip')) {
            content = await execDecompress('unzip', ['-p', safeArchivePath], lines);
        } else if (lowerPath.endsWith('.7z')) {
            content = await execDecompress('7z', ['x', '-so', safeArchivePath], lines);
        } else if (lowerPath.endsWith('.rar')) {
            content = await execDecompress('unrar', ['p', '-inul', safeArchivePath], lines);
        } else {
            throw new Error('不支持的压缩格式');
        }
        
        const contentLines = content.split('\n');
        return {
            content: content,
            truncated: contentLines.length >= lines
        };
    } catch (e) {
        throw new Error(`读取归档文件失败: ${e.message}`);
    }
}

module.exports = {
    readArchiveContent
};
