/**
 * @fileoverview 会话管理服务
 */

const crypto = require('crypto');
const config = require('../utils/config');

/** @type {Map<string, {username: string, createdAt: number, lastAccess: number}>} */
const sessions = new Map();

/** @type {Map<string, {token: string, createdAt: number}>} */
const csrfTokens = new Map();

/**
 * 生成随机令牌
 * @returns {string}
 */
function generateToken() {
    return crypto.randomBytes(32).toString('hex');
}

/**
 * 创建会话
 * @param {string} username - 用户名
 * @returns {string} 会话令牌
 */
function createSession(username) {
    const token = generateToken();
    sessions.set(token, {
        username: username,
        createdAt: Date.now(),
        lastAccess: Date.now()
    });
    return token;
}

/**
 * 验证会话
 * @param {string} token - 会话令牌
 * @returns {boolean}
 */
function validateSession(token) {
    if (!token) return false;
    const session = sessions.get(token);
    if (!session) return false;
    
    if (Date.now() - session.lastAccess > config.sessionExpiry) {
        sessions.delete(token);
        csrfTokens.delete(token);
        return false;
    }
    
    session.lastAccess = Date.now();
    return true;
}

/**
 * 获取会话
 * @param {string} token - 会话令牌
 * @returns {object|null}
 */
function getSession(token) {
    return sessions.get(token) || null;
}

/**
 * 删除会话
 * @param {string} token - 会话令牌
 */
function deleteSession(token) {
    sessions.delete(token);
    csrfTokens.delete(token);
}

/**
 * 清理过期会话
 */
function cleanExpiredSessions() {
    const now = Date.now();
    for (const [token, session] of sessions.entries()) {
        if (now - session.lastAccess > config.sessionExpiry) {
            sessions.delete(token);
            csrfTokens.delete(token);
        }
    }
}

/**
 * 获取或创建 CSRF 令牌
 * @param {string} sessionToken - 会话令牌
 * @returns {string|null}
 */
function getCSRFToken(sessionToken) {
    if (sessionToken && sessions.has(sessionToken)) {
        if (csrfTokens.has(sessionToken)) {
            const stored = csrfTokens.get(sessionToken);
            if (Date.now() - stored.createdAt < config.csrf.expiry) {
                return stored.token;
            }
        }
        const csrfToken = generateToken();
        csrfTokens.set(sessionToken, { token: csrfToken, createdAt: Date.now() });
        return csrfToken;
    }
    return null;
}

/**
 * 验证 CSRF 令牌
 * @param {string} sessionToken - 会话令牌
 * @param {string} csrfToken - CSRF令牌
 * @returns {boolean}
 */
function validateCSRFToken(sessionToken, csrfToken) {
    if (!sessionToken || !csrfToken) return false;
    const stored = csrfTokens.get(sessionToken);
    if (!stored) return false;
    if (Date.now() - stored.createdAt > config.csrf.expiry) {
        csrfTokens.delete(sessionToken);
        return false;
    }
    return stored.token === csrfToken;
}

/**
 * 清理过期 CSRF 令牌
 */
function cleanExpiredCSRFTokens() {
    const now = Date.now();
    for (const [sessionToken, data] of csrfTokens.entries()) {
        if (now - data.createdAt > config.csrf.expiry) {
            csrfTokens.delete(sessionToken);
        }
    }
}

setInterval(cleanExpiredSessions, 3600000);
setInterval(cleanExpiredCSRFTokens, 3600000);

module.exports = {
    generateToken,
    createSession,
    validateSession,
    getSession,
    deleteSession,
    cleanExpiredSessions,
    getCSRFToken,
    validateCSRFToken
};
