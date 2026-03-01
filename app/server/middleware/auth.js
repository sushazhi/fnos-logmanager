/**
 * @fileoverview 认证中间件
 */

const { body, validationResult } = require('express-validator');
const sessionService = require('../services/session');
const { getClientIP, isPrivateIP, isDirectAccess } = require('../utils/ip');
const { AuthenticationError, CSRFError, ValidationError } = require('../utils/errors');
const auditService = require('../services/audit');

/**
 * 从请求中获取会话令牌（支持 cookie 和 header）
 * @param {import('express').Request} req - Express请求对象
 * @returns {string}
 */
function getSessionToken(req) {
    // 优先从 cookie 获取（更安全，httpOnly）
    const cookieToken = req.cookies?.session_token;
    if (cookieToken) return cookieToken;
    
    // 其次从 Authorization header 获取
    const authHeader = req.headers.authorization || '';
    if (authHeader.startsWith('Bearer ')) {
        return authHeader.slice(7);
    }
    
    // 最后从 query 获取（兼容旧方式）
    return req.query.token || '';
}

/**
 * 验证令牌中间件
 * @param {import('express').Request} req - Express请求对象
 * @param {import('express').Response} res - Express响应对象
 * @param {import('express').NextFunction} next - 下一个中间件
 */
function validateToken(req, res, next) {
    const socketIP = req.ip || req.connection?.remoteAddress || req.socket?.remoteAddress || '';
    const clientIP = getClientIP(req);
    
    // 改进的同源判断：支持同服务器不同端口的iframe访问
    // 提取主机名（不带端口）进行比较
    const referer = req.headers.referer || '';
    const host = req.headers.host || '';
    const origin = req.headers.origin || '';
    const hostBase = host.split(':')[0];
    const refererHost = referer ? new URL(referer).hostname : '';
    const originHost = origin ? new URL(origin).hostname : '';
    
    const isSameOrigin = 
        referer.includes('/app-center/') || 
        referer.includes('/desktop/') ||
        origin.includes(host) ||
        referer.startsWith(`http://${host}`) ||
        referer.startsWith(`https://${host}`) ||
        refererHost === hostBase ||  // 支持同IP不同端口（如 /app-center/ 嵌入 :8090）
        originHost === hostBase;     // 支持同IP不同端口
    
    // 安全移除：私网认证绕过逻辑已被删除
    // 原因：造成严重安全漏洞，私网攻击者可绕过所有认证
    // iframe支持已通过CSP frame-ancestors和sameSite: 'lax'解决，不再需要绕过
    
    const token = getSessionToken(req);
    req.clientIP = clientIP;
    
    // 始终要求有效的会话令牌
    if (!token) {
        auditService.addAuditLog('auth_failed', { path: req.path, clientIP, isSameOrigin }, req);
        return next(new AuthenticationError());
    }
    
    if (!sessionService.validateSession(token)) {
        auditService.addAuditLog('auth_failed', { path: req.path, clientIP, isSameOrigin }, req);
        return next(new AuthenticationError());
    }
    
    req.sessionToken = token;
    next();
}

/**
 * CSRF 验证中间件
 * @param {import('express').Request} req - Express请求对象
 * @param {import('express').Response} res - Express响应对象
 * @param {import('express').NextFunction} next - 下一个中间件
 */
function validateCSRF(req, res, next) {
    // GET/HEAD请求不需要CSRF验证
    if (req.method === 'GET' || req.method === 'HEAD') {
        return next();
    }
    
    const clientIP = req.clientIP || getClientIP(req);
    
    // 安全移除：私网CSRF绕过逻辑已被删除
    // 原因：造成严重安全漏洞，私网攻击者可绕过CSRF保护
    // iframe支持已通过CSP frame-ancestors和sameSite: 'lax'解决，不再需要绕过
    
    const sessionToken = req.sessionToken;
    const csrfToken = req.headers['x-csrf-token'] || req.body?._csrf;
    
    if (!sessionService.validateCSRFToken(sessionToken, csrfToken)) {
        auditService.addAuditLog('csrf_failed', { path: req.path, clientIP }, req);
        return next(new CSRFError());
    }
    
    next();
}

/**
 * 输入验证中间件
 * @param {Array} validators - express-validator 验证器数组
 * @returns {Function}
 */
function validate(validators) {
    return [
        ...validators,
        (req, res, next) => {
            const errors = validationResult(req);
            if (!errors.isEmpty()) {
                const errorMessages = errors.array().map(e => e.msg);
                return next(new ValidationError(errorMessages.join(', '), errors.array()));
            }
            next();
        }
    ];
}

/**
 * 登录验证规则
 */
const loginValidationRules = [
    body('password')
        .notEmpty().withMessage('请输入密码')
        .isString().withMessage('密码必须是字符串')
];

/**
 * 修改密码验证规则
 */
const changePasswordValidationRules = [
    body('currentPassword')
        .notEmpty().withMessage('请输入当前密码'),
    body('newPassword')
        .notEmpty().withMessage('请输入新密码')
        .isLength({ min: 8 }).withMessage('新密码至少8位')
];

/**
 * 日志路径验证规则
 */
const logPathValidationRules = [
    body('path').optional()
        .isString().withMessage('路径必须是字符串')
        .custom((value) => {
            if (value && (value.includes('..') || value.includes('\0'))) {
                throw new Error('无效的路径');
            }
            return true;
        })
];

module.exports = {
    validateToken,
    validateCSRF,
    validate,
    getSessionToken,
    loginValidationRules,
    changePasswordValidationRules,
    logPathValidationRules
};
