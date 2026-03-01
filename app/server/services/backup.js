/**
 * @fileoverview 备份服务 - 使用 spawn 避免 shell 注入
 */

const fs = require('fs');
const path = require('path');
const { promisify } = require('util');
const { spawn } = require('child_process');
const config = require('../utils/config');
const { safePath, formatBytes } = require('../utils/validation');

const readdir = promisify(fs.readdir);
const stat = promisify(fs.stat);
const mkdir = promisify(fs.mkdir);
const copyFile = promisify(fs.copyFile);
const unlink = promisify(fs.unlink);
const rmdir = promisify(fs.rmdir);

const BACKUP_BASE_DIR = '/vol1/@appshare/log-backup';

/**
 * 确保目录存在
 * @param {string} dir - 目录路径
 */
async function ensureDir(dir) {
    try {
        await mkdir(dir, { recursive: true });
    } catch (e) {
        if (e.code !== 'EEXIST') throw e;
    }
}

/**
 * 检查是否为日志文件
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
 * 递归收集日志文件
 * @param {string} dir - 目录路径
 * @param {string} baseDir - 基础目录
 * @param {number} maxFiles - 最大文件数
 * @returns {Promise<Array<{fullPath: string, relativePath: string}>>}
 */
async function collectLogFiles(dir, baseDir, maxFiles = 500) {
    const files = [];
    
    async function walk(currentDir) {
        if (files.length >= maxFiles) return;
        
        try {
            const entries = await readdir(currentDir, { withFileTypes: true });
            for (const entry of entries) {
                if (files.length >= maxFiles) break;
                
                const fullPath = path.join(currentDir, entry.name);
                
                if (entry.isDirectory()) {
                    await walk(fullPath);
                } else if (entry.isFile() && isLogFile(entry.name)) {
                    const relativePath = path.relative(baseDir, fullPath);
                    files.push({ fullPath, relativePath });
                }
            }
        } catch (e) {
            // 忽略权限错误
        }
    }
    
    await walk(dir);
    return files;
}

/**
 * 复制文件到备份目录
 * @param {string} src - 源文件路径
 * @param {string} dest - 目标文件路径
 */
async function copyFileToBackup(src, dest) {
    const destDir = path.dirname(dest);
    await ensureDir(destDir);
    await copyFile(src, dest);
}

/**
 * 创建 tar.gz 压缩包（使用 spawn 避免 shell 注入）
 * @param {string} sourceDir - 源目录
 * @param {string} outputFile - 输出文件路径
 */
async function createTarGz(sourceDir, outputFile) {
    const safeSourceDir = safePath(sourceDir);
    const safeOutputFile = safePath(outputFile);
    
    if (!safeSourceDir || !safeOutputFile) {
        throw new Error('无效的路径');
    }
    
    const baseName = path.basename(safeSourceDir);
    const parentDir = path.dirname(safeSourceDir);
    
    // 验证路径不包含危险字符
    if (baseName.includes('\0') || parentDir.includes('\0')) {
        throw new Error('路径包含非法字符');
    }
    
    return new Promise((resolve, reject) => {
        // 使用 spawn 传递参数数组，避免 shell 注入
        const proc = spawn('tar', [
            '-czf',
            safeOutputFile,
            '-C', parentDir,
            baseName
        ], {
            timeout: 300000 // 5分钟超时
        });
        
        let stderr = '';
        
        proc.stderr.on('data', (data) => {
            stderr += data.toString();
        });
        
        proc.on('close', (code) => {
            if (code === 0) {
                resolve();
            } else {
                reject(new Error(`tar 命令失败: ${stderr || `退出码 ${code}`}`));
            }
        });
        
        proc.on('error', (err) => {
            reject(new Error(`tar 命令执行失败: ${err.message}`));
        });
    });
}

/**
 * 执行备份
 * @returns {Promise<{backupPath: string, files: number, backupSize: string, errors: Array<string>}>}
 */
async function performBackup() {
    const timestamp = new Date().toISOString()
        .replace(/[:.]/g, '-')
        .slice(0, 19)
        .replace(/[^0-9T\-]/g, '');
    
    const backupDir = path.join(BACKUP_BASE_DIR, `backup-${timestamp}`);
    const backupFile = `${backupDir}.tar.gz`;
    
    const results = {
        backupPath: backupFile,
        files: 0,
        backupSize: '0B',
        errors: []
    };
    
    try {
        // 确保备份基础目录存在
        await ensureDir(BACKUP_BASE_DIR);
        await ensureDir(backupDir);
        
        // 遍历所有日志目录
        for (const logDir of config.logDirs) {
            const normalizedDir = safePath(logDir);
            if (!normalizedDir) continue;
            
            try {
                const stats = await stat(normalizedDir).catch(() => null);
                if (!stats || !stats.isDirectory()) continue;
                
                const dirName = path.basename(normalizedDir);
                const targetDir = path.join(backupDir, dirName);
                
                // 收集日志文件
                const files = await collectLogFiles(normalizedDir, normalizedDir, 500);
                
                // 复制文件
                for (const file of files) {
                    try {
                        const targetPath = path.join(targetDir, file.relativePath);
                        await copyFileToBackup(file.fullPath, targetPath);
                        results.files++;
                    } catch (e) {
                        results.errors.push(`复制失败: ${file.fullPath} - ${e.message}`);
                    }
                }
            } catch (e) {
                results.errors.push(`处理目录失败: ${logDir} - ${e.message}`);
            }
        }
        
        // 创建压缩包
        if (results.files > 0) {
            await createTarGz(backupDir, backupFile);
            
            // 删除临时目录
            await removeDir(backupDir);
            
            // 获取压缩包大小
            const stats = await stat(backupFile);
            results.backupSize = formatBytes(stats.size);
        } else {
            // 没有文件，删除空目录
            await removeDir(backupDir);
            results.errors.push('没有找到日志文件');
        }
    } catch (e) {
        results.errors.push(`备份失败: ${e.message}`);
    }
    
    return results;
}

/**
 * 递归删除目录
 * @param {string} dir - 目录路径
 */
async function removeDir(dir) {
    const safeDir = safePath(dir);
    if (!safeDir) return;
    
    // 安全检查：不允许删除系统目录
    const dangerousPaths = ['/', '/var', '/etc', '/usr', '/bin', '/sbin', '/lib', '/boot'];
    if (dangerousPaths.includes(safeDir)) {
        throw new Error('不允许删除系统目录');
    }
    
    try {
        const stats = await stat(safeDir);
        if (!stats.isDirectory()) return;
    } catch (e) {
        return; // 目录不存在
    }
    
    async function rmRecursive(currentDir) {
        const entries = await readdir(currentDir, { withFileTypes: true });
        
        for (const entry of entries) {
            const fullPath = path.join(currentDir, entry.name);
            if (entry.isDirectory()) {
                await rmRecursive(fullPath);
            } else {
                await unlink(fullPath);
            }
        }
        
        await rmdir(currentDir);
    }
    
    await rmRecursive(safeDir);
}

/**
 * 获取备份列表
 * @returns {Promise<Array<{name: string, path: string, size: string, created: Date}>>}
 */
async function listBackups() {
    const backups = [];
    
    try {
        await stat(BACKUP_BASE_DIR).catch(() => null);
        const entries = await readdir(BACKUP_BASE_DIR, { withFileTypes: true }).catch(() => []);
        
        for (const entry of entries) {
            if (!entry.isFile() || !entry.name.endsWith('.tar.gz')) continue;
            
            const fullPath = path.join(BACKUP_BASE_DIR, entry.name);
            try {
                const stats = await stat(fullPath);
                backups.push({
                    name: entry.name,
                    path: fullPath,
                    size: formatBytes(stats.size),
                    created: stats.mtime
                });
            } catch (e) {
                // 忽略无法访问的文件
            }
        }
    } catch (e) {
        // 忽略错误
    }
    
    // 按创建时间降序排列
    backups.sort((a, b) => b.created - a.created);
    return backups;
}

/**
 * 删除备份
 * @param {string} backupPath - 备份文件路径
 */
async function deleteBackup(backupPath) {
    const safeBackupPath = safePath(backupPath);
    
    if (!safeBackupPath) {
        throw new Error('无效的路径');
    }
    
    // 安全检查：必须是备份目录下的文件
    if (!safeBackupPath.startsWith(BACKUP_BASE_DIR)) {
        throw new Error('只能删除备份目录下的文件');
    }
    
    // 安全检查：必须是 .tar.gz 文件
    if (!safeBackupPath.endsWith('.tar.gz')) {
        throw new Error('只能删除备份文件');
    }
    
    await unlink(safeBackupPath);
}

/**
 * 清理旧备份
 * @param {number} keepDays - 保留天数
 * @returns {Promise<number>} 删除的文件数量
 */
async function cleanOldBackups(keepDays = 30) {
    const backups = await listBackups();
    const cutoff = Date.now() - keepDays * 24 * 60 * 60 * 1000;
    let deleted = 0;
    
    for (const backup of backups) {
        if (backup.created.getTime() < cutoff) {
            try {
                await deleteBackup(backup.path);
                deleted++;
            } catch (e) {
                // 忽略错误
            }
        }
    }
    
    return deleted;
}

module.exports = {
    performBackup,
    listBackups,
    deleteBackup,
    cleanOldBackups,
    BACKUP_BASE_DIR
};
