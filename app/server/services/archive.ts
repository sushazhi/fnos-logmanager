import { spawn } from 'child_process';
import fs from 'fs';
import { promisify } from 'util';
import config from '../utils/config';
import { safePath, isAllowedPath } from '../utils/validation';

const stat = promisify(fs.stat);

function execDecompress(cmd: string, args: string[], lines: number, maxBytes: number): Promise<{ content: string; truncated: boolean }> {
    return new Promise((resolve, reject) => {
        const proc = spawn(cmd, args, {
            stdio: ['ignore', 'pipe', 'pipe']
        });

        let stdout = '';
        let stderr = '';
        let lineCount = 0;
        const maxLines = lines || 50;
        let truncated = false;
        let totalBytes = 0;

        proc.stdout.on('data', (data) => {
            if (truncated) return;
            totalBytes += data.length;
            if (totalBytes > maxBytes) {
                truncated = true;
                proc.kill();
                return;
            }
            if (lineCount < maxLines * 2) {
                const chunk = data.toString();
                stdout += chunk;
                lineCount += chunk.split('\n').length - 1;
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
                const content = outputLines.join('\n');
                resolve({ content, truncated: truncated || outputLines.length >= maxLines });
            }
        });

        proc.on('error', (err) => {
            reject(new Error(`执行命令失败: ${err.message}`));
        });
    });
}

interface ArchiveContentResult {
    content: string;
    truncated: boolean;
}

export async function readArchiveContent(archivePath: string, lines: number = 50): Promise<ArchiveContentResult> {
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

    if (stats.size > config.archive.maxArchiveBytes) {
        throw new Error('归档文件过大');
    }

    const maxLines = Math.max(1, Math.min(lines, config.archive.maxPreviewLines));

    const lowerPath = safeArchivePath.toLowerCase();
    let content: string;

    try {
        let result: { content: string; truncated: boolean };
        if (lowerPath.endsWith('.tar.gz') || lowerPath.endsWith('.tgz')) {
            result = await execDecompress('tar', ['-xzf', safeArchivePath, '-O'], maxLines, config.archive.maxOutputBytes);
        } else if (lowerPath.endsWith('.tar.bz2') || lowerPath.endsWith('.tbz2')) {
            result = await execDecompress('tar', ['-xjf', safeArchivePath, '-O'], maxLines, config.archive.maxOutputBytes);
        } else if (lowerPath.endsWith('.tar.xz') || lowerPath.endsWith('.txz')) {
            result = await execDecompress('tar', ['-xJf', safeArchivePath, '-O'], maxLines, config.archive.maxOutputBytes);
        } else if (lowerPath.endsWith('.tar')) {
            result = await execDecompress('tar', ['-xf', safeArchivePath, '-O'], maxLines, config.archive.maxOutputBytes);
        } else if (lowerPath.endsWith('.gz')) {
            result = await execDecompress('zcat', [safeArchivePath], maxLines, config.archive.maxOutputBytes);
        } else if (lowerPath.endsWith('.bz2')) {
            result = await execDecompress('bzcat', [safeArchivePath], maxLines, config.archive.maxOutputBytes);
        } else if (lowerPath.endsWith('.xz')) {
            result = await execDecompress('xzcat', [safeArchivePath], maxLines, config.archive.maxOutputBytes);
        } else if (lowerPath.endsWith('.zip')) {
            result = await execDecompress('unzip', ['-p', safeArchivePath], maxLines, config.archive.maxOutputBytes);
        } else if (lowerPath.endsWith('.7z')) {
            result = await execDecompress('7z', ['x', '-so', safeArchivePath], maxLines, config.archive.maxOutputBytes);
        } else if (lowerPath.endsWith('.rar')) {
            result = await execDecompress('unrar', ['p', '-inul', safeArchivePath], maxLines, config.archive.maxOutputBytes);
        } else {
            throw new Error('不支持的压缩格式');
        }

        content = result.content;
        const contentLines = content.split('\n');
        return {
            content: content,
            truncated: result.truncated || contentLines.length >= maxLines
        };
    } catch (e) {
        throw new Error(`读取归档文件失败: ${(e as Error).message}`);
    }
}
