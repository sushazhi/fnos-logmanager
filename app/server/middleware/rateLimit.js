/**
 * @fileoverview 速率限制中间件
 */

const { getClientIP } = require('../utils/ip');
const { RateLimitError } = require('../utils/errors');
const config = require('../utils/config');

/** @type {Map<string, {count: number, resetTime: number}>} */
const rateLimitMap = new Map();

/**
 * 速率限制中间件
 * @param {import('express').Request} req - Express请求对象
 * @param {import('express').Response} res - Express响应对象
 * @param {import('express').NextFunction} next - 下一个中间件
 */
function rateLimit(req, res, next) {
    const ip = getClientIP(req);
    const now = Date.now();
    
    const record = rateLimitMap.get(ip) || { count: 0, resetTime: now + config.rateLimit.windowMs };
    
    if (now > record.resetTime) {
        record.count = 1;
        record.resetTime = now + config.rateLimit.windowMs;
    } else {
        record.count++;
    }
    
    rateLimitMap.set(ip, record);
    
    if (record.count > config.rateLimit.maxRequests) {
        return next(new RateLimitError('请求过于频繁，请稍后再试'));
    }
    
    next();
}

/**
 * 登录速率限制
 * @type {Map<string, {count: number, lockoutUntil: number}>}
 */
const loginAttemptsMap = new Map();

/**
 * 获取登录尝试记录
 * @param {string} ip - IP地址
 * @returns {{count: number, lockoutUntil: number}}
 */
function getLoginAttempts(ip) {
    const attempts = loginAttemptsMap.get(ip) || { count: 0, lockoutUntil: 0 };
    if (attempts.lockoutUntil && Date.now() > attempts.lockoutUntil) {
        loginAttemptsMap.delete(ip);
        return { count: 0, lockoutUntil: 0 };
    }
    return attempts;
}

/**
 * 记录登录尝试
 * @param {string} ip - IP地址
 * @param {boolean} success - 是否成功
 */
function recordLoginAttempt(ip, success) {
    if (success) {
        loginAttemptsMap.delete(ip);
        return;
    }
    
    const attempts = getLoginAttempts(ip);
    attempts.count++;
    if (attempts.count >= config.login.maxAttempts) {
        attempts.lockoutUntil = Date.now() + config.login.lockoutTime;
    }
    loginAttemptsMap.set(ip, attempts);
}

/**
 * 检查是否被锁定
 * @param {string} ip - IP地址
 * @returns {boolean}
 */
function isLockedOut(ip) {
    const attempts = getLoginAttempts(ip);
    return attempts.lockoutUntil > 0;
}

/**
 * 获取剩余尝试次数
 * @param {string} ip - IP地址
 * @returns {number}
 */
function getRemainingAttempts(ip) {
    const attempts = getLoginAttempts(ip);
    return Math.max(0, config.login.maxAttempts - attempts.count);
}

module.exports = {
    rateLimit,
    getLoginAttempts,
    recordLoginAttempt,
    isLockedOut,
    getRemainingAttempts
};
