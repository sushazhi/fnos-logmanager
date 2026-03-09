import path from 'path';

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

export function isAllowedPath(inputPath: string, allowedDirs: string[]): boolean {
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

export function isValidSize(size: string): boolean {
    if (!size || typeof size !== 'string') return false;
    return /^[0-9]+[KMGT]?$/i.test(size);
}

export function isValidNumber(num: string | number, min: number, max: number): boolean {
    const n = parseInt(String(num), 10);
    return !isNaN(n) && n >= min && n <= max;
}

export function isValidContainerName(name: string): boolean {
    if (!name || typeof name !== 'string') return false;
    if (name.length > 128) return false;
    return /^[a-zA-Z0-9][a-zA-Z0-9_.-]*$/.test(name);
}

export function isValidRelativePath(p: string): boolean {
    if (!p || typeof p !== 'string') return false;
    if (p.includes('..') || p.includes('\0')) return false;
    if (p.startsWith('/')) return false;
    return true;
}

export function formatBytes(bytes: number): string {
    if (bytes === 0) return '0B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + sizes[i];
}

export function isValidPattern(pattern: string): boolean {
    if (!pattern || typeof pattern !== 'string') return false;
    if (pattern.length > 100) return false;

    const dangerousPatterns = [
        /[\*\+]{2,}/,
        /\(\?\<[=!]/,
        /\(\?\=/,
        /\(\?\!/,
        /\(\?\<=/,
        /\(\?\<!/,
        /\{[0-9]+,[0-9]*\}/
    ];

    for (const dangerous of dangerousPatterns) {
        if (dangerous.test(pattern)) {
            return false;
        }
    }

    try {
        new RegExp(pattern);
        return true;
    } catch {
        return false;
    }
}

export function escapeRegExp(str: string): string {
    return str.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}

export function isValidEmail(email: string): boolean {
    if (!email || typeof email !== 'string') return false;
    if (email.length > 254) return false;
    return /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email);
}

export function isValidUrl(url: string): boolean {
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

export function sanitizeFilename(filename: string): string | null {
    if (!filename || typeof filename !== 'string') return null;
    if (filename.length > 255) return null;

    const sanitized = filename
        .replace(/\.\./g, '')
        .replace(/[<>:"|?*\x00-\x1f]/g, '')
        .replace(/^\.+/, '')
        .replace(/\.+$/, '');

    if (sanitized.length === 0) return null;
    return sanitized;
}

export function isValidAction(action: string): action is 'truncate' | 'delete' {
    return action === 'truncate' || action === 'delete';
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
