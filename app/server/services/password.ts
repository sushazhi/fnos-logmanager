import fs from 'fs';
import { promisify } from 'util';
import config from '../utils/config';
import { PasswordChangeResult } from '../types';

const mkdir = promisify(fs.mkdir);
const readFile = promisify(fs.readFile);
const writeFile = promisify(fs.writeFile);
const stat = promisify(fs.stat);

interface Argon2Module {
    hash: (password: string | Buffer, options?: {
        memoryCost?: number;
        timeCost?: number;
        parallelism?: number;
        outputLength?: number;
    }) => Promise<string>;
    verify: (hash: string, password: string | Buffer) => Promise<boolean>;
}

let argon2: Argon2Module | null = null;

async function initArgon2(): Promise<Argon2Module> {
    if (argon2) return argon2;
    try {
        // 使用 @node-rs/argon2 (Rust 实现，无原生依赖问题)
        const module = require('@node-rs/argon2');
        argon2 = module;
        return argon2!;
    } catch (e) {
        throw new Error('Argon2 模块加载失败，请确保已安装 @node-rs/argon2 依赖: ' + (e as Error).message);
    }
}

async function hashPassword(password: string): Promise<string> {
    const a2 = await initArgon2();
    const hash = await a2.hash(password, {
        memoryCost: 65536,
        timeCost: 3,
        parallelism: 4,
        outputLength: 64
    });
    return `argon2$$${hash}`;
}

async function verifyPassword(password: string, storedHash: string): Promise<boolean> {
    if (!storedHash) return false;

    const parts = storedHash.split('$$');
    if (parts[0] !== 'argon2') return false;

    const a2 = await initArgon2();
    const hashPart = parts.slice(1).join('$$');
    try {
        return await a2.verify(hashPart, password);
    } catch {
        return false;
    }
}

async function ensureDataDir(): Promise<void> {
    try {
        await mkdir(config.dataDir, { recursive: true, mode: 0o700 });
    } catch (e) {
        if ((e as NodeJS.ErrnoException).code !== 'EEXIST') {
            throw e;
        }
    }
}

export async function getStoredPassword(): Promise<string | null> {
    try {
        await stat(config.passwordFile);
        const content = await readFile(config.passwordFile, 'utf8');
        return content.trim();
    } catch {
        return null;
    }
}

async function setStoredPassword(hashedPassword: string): Promise<void> {
    await ensureDataDir();
    await writeFile(config.passwordFile, hashedPassword, { mode: 0o600 });
}

/**
 * 获取初始化时间戳（首次设置密码的时间）
 */
export async function getInitTimestamp(): Promise<number | null> {
    try {
        await stat(config.initTimestampFile);
        const content = await readFile(config.initTimestampFile, 'utf8');
        const timestamp = parseInt(content.trim(), 10);
        return isNaN(timestamp) ? null : timestamp;
    } catch {
        return null;
    }
}

/**
 * 设置初始化时间戳
 */
async function setInitTimestamp(timestamp: number): Promise<void> {
    await ensureDataDir();
    await writeFile(config.initTimestampFile, String(timestamp), { mode: 0o600 });
}

/**
 * 检查设置密码是否在允许的时间窗口内
 */
export async function isSetupAllowed(): Promise<boolean> {
    const timestamp = await getInitTimestamp();
    if (!timestamp) {
        // 没有时间戳，说明还没设置过密码，允许设置
        return true;
    }
    
    const MAX_SETUP_TIME = 30 * 60 * 1000; // 30分钟
    return (Date.now() - timestamp) < MAX_SETUP_TIME;
}

export async function isPasswordSet(): Promise<boolean> {
    const hash = await getStoredPassword();
    return !!hash;
}

export async function setupPassword(password: string): Promise<PasswordChangeResult> {
    const existing = await getStoredPassword();
    if (existing) {
        return { success: false, message: '密码已设置，请使用修改密码功能' };
    }

    if (!password || password.length < 8) {
        return { success: false, message: '密码至少8位' };
    }

    // 检查是否在允许的时间窗口内
    if (!(await isSetupAllowed())) {
        return { success: false, message: '初始设置时间已过，请联系管理员' };
    }

    const hashedPassword = await hashPassword(password);
    await setStoredPassword(hashedPassword);
    
    // 首次设置密码时保存时间戳
    await setInitTimestamp(Date.now());
    
    return { success: true, message: '密码设置成功' };
}

export async function changePassword(currentPassword: string, newPassword: string): Promise<PasswordChangeResult> {
    const storedHash = await getStoredPassword();
    if (!storedHash) {
        return { success: false, message: '系统未初始化' };
    }

    const isValid = await verifyPassword(currentPassword, storedHash);
    if (!isValid) {
        return { success: false, message: '当前密码错误' };
    }

    if (!newPassword || newPassword.length < 8) {
        return { success: false, message: '新密码至少8位' };
    }

    const newHash = await hashPassword(newPassword);
    await setStoredPassword(newHash);
    return { success: true, message: '密码已修改' };
}

export { hashPassword, verifyPassword };
