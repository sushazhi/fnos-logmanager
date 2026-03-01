/**
 * @fileoverview 密码服务 - 使用 Argon2id 安全哈希
 */

const fs = require('fs');
const { promisify } = require('util');
const config = require('../utils/config');

const mkdir = promisify(fs.mkdir);
const readFile = promisify(fs.readFile);
const writeFile = promisify(fs.writeFile);
const stat = promisify(fs.stat);

let argon2 = null;

/**
 * 初始化 Argon2 模块
 */
async function initArgon2() {
    if (argon2) return argon2;
    try {
        argon2 = require('argon2');
        return argon2;
    } catch (e) {
        throw new Error('Argon2 模块加载失败，请确保已安装 argon2 依赖: ' + e.message);
    }
}

/**
 * 哈希密码
 * @param {string} password - 明文密码
 * @returns {Promise<string>}
 */
async function hashPassword(password) {
    const a2 = await initArgon2();
    const hash = await a2.hash(password, {
        type: a2.argon2id,
        memoryCost: 65536,
        timeCost: 3,
        parallelism: 4,
        hashLength: 64
    });
    return `argon2$$${hash}`;
}

/**
 * 验证密码
 * @param {string} password - 明文密码
 * @param {string} storedHash - 存储的哈希值
 * @returns {Promise<boolean>}
 */
async function verifyPassword(password, storedHash) {
    if (!storedHash) return false;
    
    const parts = storedHash.split('$$');
    if (parts[0] !== 'argon2') return false;
    
    const a2 = await initArgon2();
    const hashPart = parts.slice(1).join('$$');
    try {
        return await a2.verify(hashPart, password);
    } catch (e) {
        return false;
    }
}

/**
 * 确保数据目录存在
 */
async function ensureDataDir() {
    try {
        await mkdir(config.dataDir, { recursive: true, mode: 0o700 });
    } catch (e) {
        if (e.code !== 'EEXIST') {
            throw e;
        }
    }
}

/**
 * 获取存储的密码哈希
 * @returns {Promise<string|null>}
 */
async function getStoredPassword() {
    try {
        await stat(config.passwordFile);
        const content = await readFile(config.passwordFile, 'utf8');
        return content.trim();
    } catch (e) {
        return null;
    }
}

/**
 * 存储密码哈希
 * @param {string} hashedPassword - 哈希后的密码
 */
async function setStoredPassword(hashedPassword) {
    await ensureDataDir();
    await writeFile(config.passwordFile, hashedPassword, { mode: 0o600 });
}

/**
 * 检查密码是否已设置
 * @returns {Promise<boolean>}
 */
async function isPasswordSet() {
    const hash = await getStoredPassword();
    return !!hash;
}

/**
 * 设置初始密码（仅安装时调用）
 * @param {string} password - 明文密码
 * @returns {Promise<{success: boolean, message: string}>}
 */
async function setupPassword(password) {
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

/**
 * 修改密码
 * @param {string} currentPassword - 当前密码
 * @param {string} newPassword - 新密码
 * @returns {Promise<{success: boolean, message: string}>}
 */
async function changePassword(currentPassword, newPassword) {
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

module.exports = {
    hashPassword,
    verifyPassword,
    setupPassword,
    changePassword,
    getStoredPassword,
    isPasswordSet
};
