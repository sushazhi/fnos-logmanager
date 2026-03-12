import { Request } from 'express';

export interface LogFile {
    path: string;
    size: number;
    sizeFormatted: string;
    modified: Date;
    appName?: string | null;
    canDelete?: boolean;
}

export interface ArchiveFile {
    path: string;
    size: number;
    sizeFormatted: string;
    modified: Date;
    type: string;
}

export interface LogStats {
    totalLogs: number;
    totalArchives: number;
    largeFiles: number;
    totalSize: number;
    totalSizeFormatted: string;
}

export interface DirInfo {
    path: string;
    exists: boolean;
    logCount?: number;
    archiveCount?: number;
    totalSize?: string;
    error?: string;
}

export interface BackupInfo {
    name: string;
    path: string;
    size: string;
    created: Date;
}

export interface BackupResult {
    backupPath: string;
    files: number;
    backupSize: string;
    errors: string[];
}

export interface AuditLogEntry {
    timestamp: string;
    action: string;
    details: Record<string, unknown>;
    ip: string;
    userAgent: string;
}

export interface Session {
    username: string;
    createdAt: number;
    lastAccess: number;
}

export interface CSRFToken {
    token: string;
    createdAt: number;
}

export interface RateLimitRecord {
    count: number;
    resetTime: number;
}

export interface LoginAttempt {
    count: number;
    lockoutUntil: number;
}

export interface ReadLogOptions {
    maxLines?: number;
    offset?: number;
    tail?: boolean;
}

export interface ReadLogResult {
    content: string;
    totalLines: number;
    size: number;
    sizeFormatted: string;
    truncated: boolean;
    hasMore: boolean;
}

export interface CleanLogOptions {
    thresholdBytes?: number | null;
    days?: number | null;
    action: 'truncate' | 'delete';
}

export interface CleanLogResult {
    cleaned: number;
    errors: string[];
}

export interface AppConfig {
    port: number;
    dataDir: string;
    sessionExpiry: number;
    rateLimit: {
        windowMs: number;
        maxRequests: number;
    };
    auth: {
        allowQueryToken: boolean;
        queryTokenPaths: string[];
    };
    login: {
        maxAttempts: number;
        lockoutTime: number;
    };
    audit: {
        maxLogs: number;
    };
    csrf: {
        expiry: number;
    };
    logging: {
        redactQuery: boolean;
    };
    update: {
        checkCacheMs: number;
        checkRateLimit: {
            windowMs: number;
            maxRequests: number;
        };
        downloadTimeoutMs: number;
        maxDownloadBytes: number;
        maxAssetBytes: number;
        maxRedirects: number;
        allowedHosts: string[];
        allowedUpdateDirs: string[];
    };
    docker: {
        listTimeoutMs: number;
        logsTimeoutMs: number;
        maxLogLines: number;
        maxOutputBytes: number;
    };
    archive: {
        maxPreviewLines: number;
        maxArchiveBytes: number;
        maxOutputBytes: number;
    };
    logFile: {
        maxPreviewLines: number;
        maxPreviewBytes: number;
    };
    backup: {
        baseDir: string;
        maxFiles: number;
        maxFileSizeBytes: number;
        maxTotalBytes: number;
    };
    logDirs: string[];
    sensitivePatterns: RegExp[];
    passwordFile: string;
    auditLogFile: string;
}

export interface PasswordChangeResult {
    success: boolean;
    message: string;
}

export interface DockerContainer {
    name: string;
    status: string;
    image: string;
}

export interface UpdateStatus {
    ready: string;
    updating: boolean;
    updateProgress: number;
    updateMessage: string;
}

export interface GitHubRelease {
    version: string;
    changelog: string;
    publishedAt: string;
    assets: GitHubAsset[];
}

export interface GitHubAsset {
    name: string;
    browser_download_url: string;
    size?: number;
}

export interface AuthenticatedRequest extends Request {
    clientIP?: string;
    sessionToken?: string;
}
