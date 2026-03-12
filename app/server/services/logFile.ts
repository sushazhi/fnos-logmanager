import fs from 'fs';
import path from 'path';
import { promisify } from 'util';
import readline from 'readline';
import { spawn } from 'child_process';
import config from '../utils/config';
import { safePath, isAllowedPath, formatBytes } from '../utils/validation';
import { LogFile, ArchiveFile, LogStats, DirInfo, ReadLogOptions, ReadLogResult, CleanLogOptions, CleanLogResult } from '../types';

const readdir = promisify(fs.readdir);
const stat = promisify(fs.stat);
const readFile = promisify(fs.readFile);
const writeFile = promisify(fs.writeFile);
const unlink = promisify(fs.unlink);

const LOG_EXTENSIONS = ['.log', '.log.'];
const ARCHIVE_EXTENSIONS = ['.gz', '.bz2', '.xz', '.zip', '.tar', '.tar.gz', '.tar.bz2', '.tar.xz', '.7z', '.rar'];
const MAX_PREVIEW_LINES = config.logFile.maxPreviewLines;
const MAX_PREVIEW_SIZE = config.logFile.maxPreviewBytes;

let cachedInstalledApps: Set<string> | null = null;
let installedAppsCacheTime = 0;
const INSTALLED_APPS_CACHE_TTL = 60000;

interface CommandResult {
    stdout: string;
    stderr: string;
}

function execCommand(cmd: string, args: string[] = [], timeout: number = 30000): Promise<CommandResult> {
    return new Promise((resolve) => {
        const proc = spawn(cmd, args, { timeout });
        let stdout = '';
        let stderr = '';

        proc.stdout.on('data', (data) => {
            stdout += data.toString();
        });

        proc.stderr.on('data', (data) => {
            stderr += data.toString();
        });

        proc.on('close', () => {
            resolve({ stdout, stderr });
        });

        proc.on('error', () => {
            resolve({ stdout, stderr });
        });
    });
}

async function getInstalledApps(): Promise<Set<string>> {
    const now = Date.now();
    if (cachedInstalledApps && (now - installedAppsCacheTime) < INSTALLED_APPS_CACHE_TTL) {
        return cachedInstalledApps;
    }

    const apps = new Set<string>();

    try {
        const { stdout } = await execCommand('appcenter-cli', ['list'], 10000);
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
    } catch {
        // 忽略错误
    }

    cachedInstalledApps = apps;
    installedAppsCacheTime = now;
    return apps;
}

export function extractAppNameFromPath(logPath: string): string | null {
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

export function isLogFile(filename: string): boolean {
    const lower = filename.toLowerCase();
    if (lower.endsWith('.log')) return true;
    if (lower.includes('.log.')) return true;
    if (lower.includes('log') && lower.endsWith('.txt')) return true;
    return false;
}

export function isArchiveFile(filename: string): boolean {
    const lower = filename.toLowerCase();
    return ARCHIVE_EXTENSIONS.some(ext => lower.endsWith(ext));
}

async function findFiles(dir: string, filterFn: (name: string) => boolean, limit: number = 100): Promise<string[]> {
    const results: string[] = [];

    async function walk(currentDir: string): Promise<void> {
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
        } catch {
            // 忽略权限错误等
        }
    }

    await walk(dir);
    return results;
}

export async function listLogFiles(dir?: string, limit: number = 100): Promise<LogFile[]> {
    const searchDirs = dir ? [dir] : config.logDirs;
    const results: LogFile[] = [];
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
            } catch {
                // 忽略无法访问的文件
            }
        }
    }

    return results.slice(0, limit);
}

export async function listArchiveFiles(limit: number = 50): Promise<ArchiveFile[]> {
    const results: ArchiveFile[] = [];

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
            } catch {
                // 忽略无法访问的文件
            }
        }
    }

    return results.slice(0, limit);
}

export async function listLargeLogFiles(thresholdBytes: number, limit: number = 50): Promise<LogFile[]> {
    const results: LogFile[] = [];

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
            } catch {
                // 忽略
            }
        }
    }

    results.sort((a, b) => b.size - a.size);
    return results.slice(0, limit);
}

export async function searchLogFilesByName(pattern: string, limit: number = 50): Promise<LogFile[]> {
    const results: LogFile[] = [];
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
            } catch {
                // 忽略
            }
        }
    }

    results.sort((a, b) => (b.modified?.getTime() || 0) - (a.modified?.getTime() || 0));
    return results.slice(0, limit);
}

export async function getLogStats(): Promise<LogStats> {
    let totalLogs = 0;
    let totalArchives = 0;
    let largeFiles = 0;
    let totalSize = 0;
    const largeThreshold = 10 * 1024 * 1024;

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
            } catch {
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

export async function readLogFile(filePath: string, options: ReadLogOptions = {}): Promise<ReadLogResult> {
    const { maxLines = MAX_PREVIEW_LINES, offset = 0, tail = false } = options;
    const normalizedPath = safePath(filePath);

    if (!normalizedPath || !isAllowedPath(filePath, config.logDirs)) {
        throw new Error('不允许访问此文件');
    }

    const stats = await stat(normalizedPath);

    if (!stats.isFile()) {
        throw new Error('不是有效的文件');
    }

    if (stats.size <= MAX_PREVIEW_SIZE) {
        const content = await readFile(normalizedPath, 'utf8');
        const allLines = content.split('\n');
        const totalLines = allLines.length;

        let selectedLines: string[];
        let truncated: boolean;

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

    return await readLargeFileStreaming(normalizedPath, { maxLines, offset, tail, size: stats.size });
}

interface StreamingOptions extends ReadLogOptions {
    size: number;
}

async function readLargeFileStreaming(filePath: string, options: StreamingOptions): Promise<ReadLogResult> {
    const { maxLines = MAX_PREVIEW_LINES, offset = 0, tail = false, size } = options;

    return new Promise((resolve, reject) => {
        const stream = fs.createReadStream(filePath, { encoding: 'utf8' });
        const rl = readline.createInterface({
            input: stream,
            crlfDelay: Infinity
        });

        let totalLines = 0;
        let collectedLines: string[] = [];
        let truncated = false;
        let lineBuffer: string[] = [];

        rl.on('line', (line) => {
            totalLines++;

            if (tail) {
                lineBuffer.push(line);
                if (lineBuffer.length > maxLines) {
                    lineBuffer.shift();
                }
            } else {
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

export async function getFileLineCount(filePath: string): Promise<number> {
    const normalizedPath = safePath(filePath);
    if (!normalizedPath || !isAllowedPath(filePath, config.logDirs)) {
        throw new Error('不允许访问此文件');
    }

    return new Promise((resolve, reject) => {
        let lineCount = 0;
        const stream = fs.createReadStream(normalizedPath);

        stream.on('data', (chunk) => {
            for (let i = 0; i < chunk.length; i++) {
                if ((chunk as Buffer)[i] === 10) {
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

export async function truncateLogFile(filePath: string): Promise<void> {
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
        if ((e as NodeJS.ErrnoException).code === 'ENOENT') {
            throw new Error('文件不存在');
        }
        throw e;
    }

    await writeFile(normalizedPath, '');
}

export async function deleteLogFile(filePath: string): Promise<void> {
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

export async function getDirsInfo(): Promise<DirInfo[]> {
    const results: DirInfo[] = [];

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
                } catch {
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
                        } catch {
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
            results.push({ path: dir, exists: false, logCount: 0, archiveCount: 0, totalSize: '0B', error: (e as Error).message });
        }
    }

    return results;
}

export async function cleanLogFiles(options: CleanLogOptions): Promise<CleanLogResult> {
    const { thresholdBytes, days, action } = options;
    const results: CleanLogResult = { cleaned: 0, errors: [] };
    const cutoffTime = days ? Date.now() - days * 24 * 60 * 60 * 1000 : 0;

    for (const dir of config.logDirs) {
        const normalizedDir = safePath(dir);
        if (!normalizedDir || !fs.existsSync(normalizedDir)) continue;

        try {
            const files = await findFiles(normalizedDir, (name) => {
                if (days) {
                    return isArchiveFile(name);
                } else {
                    return isLogFile(name);
                }
            }, 10000);

            for (const file of files) {
                try {
                    const stats = await stat(file);

                    if (thresholdBytes && stats.size < thresholdBytes) continue;
                    if (days && stats.mtime.getTime() > cutoffTime) continue;

                    if (action === 'delete') {
                        await unlink(file);
                    } else if (action === 'truncate') {
                        await writeFile(file, '');
                    }

                    results.cleaned++;
                } catch (e) {
                    results.errors.push(`${file}: ${(e as Error).message}`);
                }
            }
        } catch (e) {
            results.errors.push(`${dir}: ${(e as Error).message}`);
        }
    }

    return results;
}
