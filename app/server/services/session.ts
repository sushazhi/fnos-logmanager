import crypto from 'crypto';
import fs from 'fs';
import path from 'path';
import config from '../utils/config';
import { Session, CSRFToken } from '../types';
import Logger from '../utils/logger';

const logger = Logger.child({ module: 'Session' });

const sessions = new Map<string, Session>();
const csrfTokens = new Map<string, CSRFToken>();

// 持久化文件路径
const SESSION_FILE = path.join(config.dataDir, '.sessions.json');

// 脏标记，用于延迟写入
let dirty = false;
let saveTimer: NodeJS.Timeout | null = null;

/**
 * 从文件加载 session 数据
 */
function loadSessionsFromDisk(): void {
    try {
        if (!fs.existsSync(SESSION_FILE)) return;
        const content = fs.readFileSync(SESSION_FILE, 'utf8');
        const data = JSON.parse(content) as {
            sessions: Array<[string, Session]>;
            csrfTokens: Array<[string, CSRFToken]>;
        };
        const now = Date.now();

        // 只加载未过期的 session
        if (data.sessions && Array.isArray(data.sessions)) {
            for (const [token, session] of data.sessions) {
                if (now - session.lastAccess <= config.sessionExpiry) {
                    sessions.set(token, session);
                }
            }
        }

        // 只加载关联了有效 session 的 CSRF token
        if (data.csrfTokens && Array.isArray(data.csrfTokens)) {
            for (const [sessionToken, csrfData] of data.csrfTokens) {
                if (sessions.has(sessionToken) && now - csrfData.createdAt <= config.csrf.expiry) {
                    csrfTokens.set(sessionToken, csrfData);
                }
            }
        }

        logger.info({ sessionCount: sessions.size }, 'Sessions loaded from disk');
    } catch (err) {
        logger.warn({ err }, 'Failed to load sessions from disk, starting fresh');
    }
}

/**
 * 将 session 数据保存到文件（延迟写入）
 */
function scheduleSaveToDisk(): void {
    dirty = true;
    if (saveTimer) return;
    saveTimer = setTimeout(() => {
        saveToDisk();
        saveTimer = null;
    }, 5000); // 5秒延迟，合并多次变更
}

/**
 * 实际写入磁盘
 */
function saveToDisk(): void {
    if (!dirty) return;
    dirty = false;

    try {
        // 确保数据目录存在
        if (!fs.existsSync(config.dataDir)) {
            fs.mkdirSync(config.dataDir, { recursive: true, mode: 0o700 });
        }

        const data = {
            sessions: Array.from(sessions.entries()),
            csrfTokens: Array.from(csrfTokens.entries())
        };
        // 原子写入：先写临时文件，再重命名
        const tmpFile = SESSION_FILE + '.tmp';
        fs.writeFileSync(tmpFile, JSON.stringify(data), { mode: 0o600 });
        fs.renameSync(tmpFile, SESSION_FILE);
    } catch (err) {
        logger.error({ err }, 'Failed to save sessions to disk');
    }
}

// 启动时加载
try {
    loadSessionsFromDisk();
} catch {
    // 忽略加载失败
}

export function generateToken(): string {
    return crypto.randomBytes(32).toString('hex');
}

export function createSession(username: string): string {
    const token = generateToken();
    sessions.set(token, {
        username: username,
        createdAt: Date.now(),
        lastAccess: Date.now()
    });
    scheduleSaveToDisk();
    return token;
}

export function validateSession(token: string | undefined): boolean {
    if (!token) return false;
    const session = sessions.get(token);
    if (!session) return false;

    if (Date.now() - session.lastAccess > config.sessionExpiry) {
        sessions.delete(token);
        csrfTokens.delete(token);
        scheduleSaveToDisk();
        return false;
    }

    session.lastAccess = Date.now();
    // 不在每次验证时保存，由定时清理触发保存
    return true;
}

export function getSession(token: string): Session | null {
    return sessions.get(token) || null;
}

export function deleteSession(token: string): void {
    sessions.delete(token);
    csrfTokens.delete(token);
    scheduleSaveToDisk();
}

export function cleanExpiredSessions(): void {
    const now = Date.now();
    let changed = false;
    for (const [token, session] of sessions.entries()) {
        if (now - session.lastAccess > config.sessionExpiry) {
            sessions.delete(token);
            csrfTokens.delete(token);
            changed = true;
        }
    }
    if (changed) {
        scheduleSaveToDisk();
    }
}

export function getCSRFToken(sessionToken: string): string | null {
    if (sessionToken && sessions.has(sessionToken)) {
        if (csrfTokens.has(sessionToken)) {
            const stored = csrfTokens.get(sessionToken);
            if (stored && Date.now() - stored.createdAt < config.csrf.expiry) {
                return stored.token;
            }
        }
        const csrfToken = generateToken();
        csrfTokens.set(sessionToken, { token: csrfToken, createdAt: Date.now() });
        scheduleSaveToDisk();
        return csrfToken;
    }
    return null;
}

export function validateCSRFToken(sessionToken: string, csrfToken: string): boolean {
    if (!sessionToken || !csrfToken) return false;
    const stored = csrfTokens.get(sessionToken);
    if (!stored) return false;
    if (Date.now() - stored.createdAt > config.csrf.expiry) {
        csrfTokens.delete(sessionToken);
        scheduleSaveToDisk();
        return false;
    }
    return stored.token === csrfToken;
}

export function cleanExpiredCSRFTokens(): void {
    const now = Date.now();
    let changed = false;
    for (const [sessionToken, data] of csrfTokens.entries()) {
        if (now - data.createdAt > config.csrf.expiry) {
            csrfTokens.delete(sessionToken);
            changed = true;
        }
    }
    if (changed) {
        scheduleSaveToDisk();
    }
}

setInterval(cleanExpiredSessions, 3600000);
setInterval(cleanExpiredCSRFTokens, 3600000);

// 进程退出时保存
process.on('SIGTERM', () => saveToDisk());
process.on('SIGINT', () => saveToDisk());
