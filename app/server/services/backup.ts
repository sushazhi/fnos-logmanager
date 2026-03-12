import fs from 'fs';
import path from 'path';
import { promisify } from 'util';
import { spawn } from 'child_process';
import config from '../utils/config';
import { safePath, formatBytes, isAllowedPath } from '../utils/validation';
import { BackupInfo, BackupResult } from '../types';

const readdir = promisify(fs.readdir);
const stat = promisify(fs.stat);
const mkdir = promisify(fs.mkdir);
const copyFile = promisify(fs.copyFile);
const unlink = promisify(fs.unlink);
const rmdir = promisify(fs.rmdir);

export const BACKUP_BASE_DIR = config.backup.baseDir;

async function ensureDir(dir: string): Promise<void> {
    try {
        await mkdir(dir, { recursive: true });
    } catch (e) {
        if ((e as NodeJS.ErrnoException).code !== 'EEXIST') throw e;
    }
}

function isSafeBackupBase(dir: string): boolean {
    const normalized = safePath(dir);
    if (!normalized) return false;
    return normalized.startsWith('/vol') && normalized.includes('/@appshare');
}

function isLogFile(filename: string): boolean {
    const lower = filename.toLowerCase();
    if (lower.endsWith('.log')) return true;
    if (lower.includes('.log.')) return true;
    if (lower.includes('log') && lower.endsWith('.txt')) return true;
    return false;
}

interface CollectedFile {
    fullPath: string;
    relativePath: string;
}

async function collectLogFiles(dir: string, baseDir: string, maxFiles: number = 500, maxFileSize: number = config.backup.maxFileSizeBytes): Promise<CollectedFile[]> {
    const files: CollectedFile[] = [];

    async function walk(currentDir: string): Promise<void> {
        if (files.length >= maxFiles) return;

        try {
            const entries = await readdir(currentDir, { withFileTypes: true });
            for (const entry of entries) {
                if (files.length >= maxFiles) break;

                const fullPath = path.join(currentDir, entry.name);

                if (entry.isDirectory()) {
                    await walk(fullPath);
                } else if (entry.isFile() && isLogFile(entry.name)) {
                    try {
                        const stats = await stat(fullPath);
                        if (stats.size > maxFileSize) continue;
                    } catch {
                        continue;
                    }
                    const relativePath = path.relative(baseDir, fullPath);
                    files.push({ fullPath, relativePath });
                }
            }
        } catch {
            // 忽略权限错误
        }
    }

    await walk(dir);
    return files;
}

async function copyFileToBackup(src: string, dest: string): Promise<void> {
    const destDir = path.dirname(dest);
    await ensureDir(destDir);
    await copyFile(src, dest);
}

async function createTarGz(sourceDir: string, outputFile: string): Promise<void> {
    const safeSourceDir = safePath(sourceDir);
    const safeOutputFile = safePath(outputFile);

    if (!safeSourceDir || !safeOutputFile) {
        throw new Error('无效的路径');
    }

    const baseName = path.basename(safeSourceDir);
    const parentDir = path.dirname(safeSourceDir);

    if (baseName.includes('\0') || parentDir.includes('\0')) {
        throw new Error('路径包含非法字符');
    }

    return new Promise((resolve, reject) => {
        const proc = spawn('tar', [
            '-czf',
            safeOutputFile,
            '-C', parentDir,
            baseName
        ], {
            timeout: 300000
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

async function removeDir(dir: string): Promise<void> {
    const safeDir = safePath(dir);
    if (!safeDir) return;

    const dangerousPaths = ['/', '/var', '/etc', '/usr', '/bin', '/sbin', '/lib', '/boot'];
    if (dangerousPaths.includes(safeDir)) {
        throw new Error('不允许删除系统目录');
    }

    try {
        const stats = await stat(safeDir);
        if (!stats.isDirectory()) return;
    } catch {
        return;
    }

    async function rmRecursive(currentDir: string): Promise<void> {
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

export async function performBackup(): Promise<BackupResult> {
    if (!isSafeBackupBase(BACKUP_BASE_DIR)) {
        return {
            backupPath: '',
            files: 0,
            backupSize: '0B',
            errors: ['备份目录不安全']
        };
    }
    const timestamp = new Date().toISOString()
        .replace(/[:.]/g, '-')
        .slice(0, 19)
        .replace(/[^0-9T\-]/g, '');

    const backupDir = path.join(BACKUP_BASE_DIR, `backup-${timestamp}`);
    const backupFile = `${backupDir}.tar.gz`;

    const results: BackupResult = {
        backupPath: backupFile,
        files: 0,
        backupSize: '0B',
        errors: []
    };

    try {
        await ensureDir(BACKUP_BASE_DIR);
        await ensureDir(backupDir);

        let totalBytes = 0;

        for (const logDir of config.logDirs) {
            const normalizedDir = safePath(logDir);
            if (!normalizedDir) continue;
            if (!isAllowedPath(normalizedDir, config.logDirs)) continue;

            try {
                const stats = await stat(normalizedDir).catch(() => null);
                if (!stats || !stats.isDirectory()) continue;

                const dirName = path.basename(normalizedDir);
                const targetDir = path.join(backupDir, dirName);

                const files = await collectLogFiles(normalizedDir, normalizedDir, config.backup.maxFiles, config.backup.maxFileSizeBytes);

                for (const file of files) {
                    try {
                        const srcStats = await stat(file.fullPath);
                        if (totalBytes + srcStats.size > config.backup.maxTotalBytes) {
                            results.errors.push('备份内容超过大小限制');
                            break;
                        }
                        const targetPath = path.join(targetDir, file.relativePath);
                        await copyFileToBackup(file.fullPath, targetPath);
                        results.files++;
                        totalBytes += srcStats.size;
                    } catch (e) {
                        results.errors.push(`复制失败: ${file.fullPath} - ${(e as Error).message}`);
                    }
                }

                if (totalBytes >= config.backup.maxTotalBytes) {
                    break;
                }
            } catch (e) {
                results.errors.push(`处理目录失败: ${logDir} - ${(e as Error).message}`);
            }
        }

        if (results.files > 0) {
            await createTarGz(backupDir, backupFile);

            await removeDir(backupDir);

            const stats = await stat(backupFile);
            if (stats.size > config.backup.maxTotalBytes) {
                await unlink(backupFile);
                throw new Error('备份文件超过大小限制');
            }
            results.backupSize = formatBytes(stats.size);
        } else {
            await removeDir(backupDir);
            results.errors.push('没有找到日志文件');
        }
    } catch (e) {
        results.errors.push(`备份失败: ${(e as Error).message}`);
    }

    return results;
}

export async function listBackups(): Promise<BackupInfo[]> {
    const backups: BackupInfo[] = [];

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
            } catch {
                // 忽略无法访问的文件
            }
        }
    } catch {
        // 忽略错误
    }

    backups.sort((a, b) => b.created.getTime() - a.created.getTime());
    return backups;
}

export async function deleteBackup(backupPath: string): Promise<void> {
    const safeBackupPath = safePath(backupPath);

    if (!safeBackupPath) {
        throw new Error('无效的路径');
    }

    if (!safeBackupPath.startsWith(BACKUP_BASE_DIR)) {
        throw new Error('只能删除备份目录下的文件');
    }

    if (!safeBackupPath.endsWith('.tar.gz')) {
        throw new Error('只能删除备份文件');
    }

    await unlink(safeBackupPath);
}

export async function cleanOldBackups(keepDays: number = 30): Promise<number> {
    const backups = await listBackups();
    const cutoff = Date.now() - keepDays * 24 * 60 * 60 * 1000;
    let deleted = 0;

    for (const backup of backups) {
        if (backup.created.getTime() < cutoff) {
            try {
                await deleteBackup(backup.path);
                deleted++;
            } catch {
                // 忽略错误
            }
        }
    }

    return deleted;
}
