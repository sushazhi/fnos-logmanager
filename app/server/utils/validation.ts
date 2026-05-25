import path from 'path';
import fs from 'fs';

export function safePath(inputPath: string): string | null {
    if (!inputPath || typeof inputPath !== 'string') return null;
    if (inputPath.length > 4096) return null;

    if (inputPath.includes('\0') ||
        inputPath.includes('\r') ||
        inputPath.includes('\n')) {
        return null;
    }

    const isWindows = process.platform === 'win32';

    let normalized = inputPath.replace(/\\/g, '/');

    normalized = path.normalize(normalized);

    if (normalized.includes('..')) return null;

    if (!normalized.startsWith('/') && !(isWindows && /^[A-Za-z]:/.test(normalized))) {
        return null;
    }

    try {
        normalized = path.resolve(normalized);
    } catch {
        return null;
    }

    return normalized;
}

/**
 * 检查路径是否为符号链接，或路径链中是否包含符号链接
 * 递归检查整个路径链，防止深层符号链接绕过
 */
export function isSymlinkPath(filePath: string): boolean {
    try {
        // 检查目标文件本身
        const stats = fs.lstatSync(filePath);
        if (stats.isSymbolicLink()) {
            return true;
        }

        // 递归检查路径中每个组件是否为符号链接
        const parts = filePath.split(/[/\\]/);
        let currentPath = filePath.startsWith('/') ? '/' : '';

        for (let i = 0; i < parts.length - 1; i++) {
            const part = parts[i];
            if (!part) continue;
            currentPath = path.join(currentPath, part);

            try {
                const partStats = fs.lstatSync(currentPath);
                if (partStats.isSymbolicLink()) {
                    return true;
                }
            } catch {
                // 路径组件不存在，继续检查
            }
        }

        return false;
    } catch {
        // 文件不存在或其他错误
        return false;
    }
}

export function isAllowedPath(inputPath: string, allowedDirs: string[]): boolean {
    if (!inputPath) return false;
    const normalized = safePath(inputPath);
    if (!normalized) return false;
    
    // 检查符号链接
    if (isSymlinkPath(normalized)) {
        return false;
    }

    for (const allowedDir of allowedDirs) {
        // 也检查允许目录本身是否为符号链接
        if (isSymlinkPath(allowedDir)) {
            continue;
        }
        if (normalized === allowedDir || normalized.startsWith(allowedDir + '/')) {
            return true;
        }
    }
    return false;
}

function isValidSize(size: string): boolean {
    if (!size || typeof size !== 'string') return false;
    return /^[0-9]+[KMGT]?$/i.test(size);
}

export function isValidContainerName(name: string): boolean {
    if (!name || typeof name !== 'string') return false;
    if (name.length > 128) return false;
    return /^[a-zA-Z0-9][a-zA-Z0-9_.-]*$/.test(name);
}

export function formatBytes(bytes: number): string {
    if (bytes === 0) return '0B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + sizes[i];
}

export function escapeRegExp(str: string): string {
    return str.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}

function isValidUrl(url: string): boolean {
    if (!url || typeof url !== 'string') return false;
    if (url.length > 2048) return false;
    try {
        const parsed = new URL(url);
        return ['http:', 'https:'].includes(parsed.protocol);
    } catch {
        return false;
    }
}

export function isValidGitHubUrl(url: string): boolean {
    if (!isValidUrl(url)) return false;
    try {
        const parsed = new URL(url);
        return parsed.hostname === 'github.com' ||
               parsed.hostname === 'api.github.com' ||
               parsed.hostname === 'objects.githubusercontent.com';
    } catch {
        return false;
    }
}

export function isValidAction(action: string): action is 'truncate' | 'delete' | 'deleteUninstalled' {
    return action === 'truncate' || action === 'delete' || action === 'deleteUninstalled';
}

export function isValidDays(days: number): boolean {
    return Number.isInteger(days) && days >= 1 && days <= 365;
}

export function isValidThreshold(threshold: string): boolean {
    if (!threshold || typeof threshold !== 'string') return false;
    return isValidSize(threshold);
}

export function clamp(value: number, min: number, max: number): number {
    return Math.min(Math.max(value, min), max);
}
