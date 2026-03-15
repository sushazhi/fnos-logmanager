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

    const hashedPassword = await hashPassword(password);
    await setStoredPassword(hashedPassword);
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
