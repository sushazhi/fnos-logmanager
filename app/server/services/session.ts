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
const KEY_FILE = path.join(config.dataDir, '.sessions.key');

// AES-256-GCM 常量
const ALGORITHM = 'aes-256-gcm';
const KEY_LENGTH = 32; // 256 bits
const IV_LENGTH = 16;  // GCM 标准 IV 长度
const AUTH_TAG_LENGTH = 16;

/**
 * 获取或生成加密密钥
 * 首次运行时随机生成 32 字节密钥，持久化到独立的 `.sessions.key` 文件（权限 0o600）
 */
function getEncryptionKey(): Buffer {
    try {
        if (fs.existsSync(KEY_FILE)) {
            const keyHex = fs.readFileSync(KEY_FILE, 'utf8').trim();
            return Buffer.from(keyHex, 'hex');
        }
    } catch {
        // 读取失败时重新生成
    }

    // 生成新密钥
    const key = crypto.randomBytes(KEY_LENGTH);
    try {
        if (!fs.existsSync(config.dataDir)) {
            fs.mkdirSync(config.dataDir, { recursive: true, mode: 0o700 });
        }
        fs.writeFileSync(KEY_FILE, key.toString('hex'), { mode: 0o600 });
        logger.info('Session encryption key generated');
    } catch (err) {
        logger.error({ err }, 'Failed to save encryption key');
    }
    return key;
}

/**
 * 加密数据（AES-256-GCM）
 * 返回格式: hex(iv):hex(authTag):hex(ciphertext)
 */
function encrypt(plaintext: string, key: Buffer): string {
    const iv = crypto.randomBytes(IV_LENGTH);
    const cipher = crypto.createCipheriv(ALGORITHM, key, iv);
    const encrypted = Buffer.concat([cipher.update(plaintext, 'utf8'), cipher.final()]);
    const authTag = cipher.getAuthTag();
    return iv.toString('hex') + ':' + authTag.toString('hex') + ':' + encrypted.toString('hex');
}

/**
 * 解密数据
 */
function decrypt(encoded: string, key: Buffer): string | null {
    try {
        const parts = encoded.split(':');
        if (parts.length !== 3) return null;
        const iv = Buffer.from(parts[0], 'hex');
        const authTag = Buffer.from(parts[1], 'hex');
        const encrypted = Buffer.from(parts[2], 'hex');

        if (iv.length !== IV_LENGTH || authTag.length !== AUTH_TAG_LENGTH) return null;

        const decipher = crypto.createDecipheriv(ALGORITHM, key, iv);
        decipher.setAuthTag(authTag);
        const decrypted = Buffer.concat([decipher.update(encrypted), decipher.final()]);
        return decrypted.toString('utf8');
    } catch {
        return null;
    }
}

// 加密密钥（懒加载）
let encryptionKey: Buffer | null = null;

function ensureKey(): Buffer {
    if (!encryptionKey) {
        encryptionKey = getEncryptionKey();
    }
    return encryptionKey;
}

// 脏标记，用于延迟写入
let dirty = false;
let saveTimer: NodeJS.Timeout | null = null;

/**
 * 从加密文件加载 session 数据
 */
function loadSessionsFromDisk(): void {
    try {
        if (!fs.existsSync(SESSION_FILE)) return;

        const stored = fs.readFileSync(SESSION_FILE, 'utf8').trim();
        if (!stored) return;

        const key = ensureKey();

        // 尝试解密，若失败则回退到明文读取（兼容旧格式升级）
        let plaintext: string | null = decrypt(stored, key);
        if (!plaintext) {
            // 旧格式明文兼容
            logger.warn('Failed to decrypt session file, trying plaintext fallback');
            plaintext = stored;
        }

        const data = JSON.parse(plaintext) as {
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

        // 如果加载的是旧格式明文，立即以加密格式重写
        if (plaintext === stored) {
            logger.info('Migrating session file to encrypted format');
            scheduleSaveToDisk();
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
 * 实际写入磁盘（加密后原子写入）
 */
function saveToDisk(): void {
    if (!dirty) return;
    dirty = false;

    try {
        // 确保数据目录存在
        if (!fs.existsSync(config.dataDir)) {
            fs.mkdirSync(config.dataDir, { recursive: true, mode: 0o700 });
        }

        const key = ensureKey();

        const data = {
            sessions: Array.from(sessions.entries()),
            csrfTokens: Array.from(csrfTokens.entries())
        };

        // AES-256-GCM 加密后再写入
        const encrypted = encrypt(JSON.stringify(data), key);

        // 原子写入：先写临时文件，再重命名
        const tmpFile = SESSION_FILE + '.tmp';
        fs.writeFileSync(tmpFile, encrypted, { mode: 0o600 });
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

function generateToken(): string {
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

function getSession(token: string): Session | null {
    return sessions.get(token) || null;
}

export function deleteSession(token: string): void {
    sessions.delete(token);
    csrfTokens.delete(token);
    scheduleSaveToDisk();
}

function cleanExpiredSessions(): void {
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
    const a = Buffer.from(stored.token, 'utf8');
    const b = Buffer.from(csrfToken, 'utf8');
    if (a.length !== b.length) return false;
    return crypto.timingSafeEqual(a, b);
}

function cleanExpiredCSRFTokens(): void {
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
