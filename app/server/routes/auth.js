/**
 * @fileoverview 认证路由
 */

const express = require('express');
const router = express.Router();
const passwordService = require('../services/password');
const sessionService = require('../services/session');
const auditService = require('../services/audit');
const { validateToken, validateCSRF, validate, getSessionToken, loginValidationRules, changePasswordValidationRules } = require('../middleware/auth');
const { getClientIP } = require('../utils/ip');
const { isLockedOut, recordLoginAttempt, getRemainingAttempts } = require('../middleware/rateLimit');
const { RateLimitError } = require('../utils/errors');

// Cookie配置 - 安全最佳实践
// httpOnly: 防止XSS访问cookie
// secure: 所有环境强制HTTPS（如果可用）
// sameSite: 'lax' 平衡CSRF保护和iframe支持
const COOKIE_OPTIONS = {
    httpOnly: true,
    secure: process.env.NODE_ENV === 'production' || process.env.FORCE_HTTPS === 'true',
    sameSite: 'lax',
    maxAge: 24 * 60 * 60 * 1000,
    path: '/',
    domain: process.env.COOKIE_DOMAIN || undefined
};

/**
 * @route POST /api/auth/setup
 * @description 设置初始密码（仅安装时调用，无认证要求）
 */
router.post('/setup', validate([
    require('express-validator').body('password').isLength({ min: 8 }).withMessage('密码至少8位')
]), async (req, res, next) => {
    try {
        const { password } = req.body;
        const ip = getClientIP(req);
        req.clientIP = ip;
        
        const result = await passwordService.setupPassword(password);
        
        if (result.success) {
            auditService.addAuditLog('password_setup', { ip }, req);
            res.json(result);
        } else {
            res.status(400).json(result);
        }
    } catch (err) {
        next(err);
    }
});

/**
 * @route POST /api/auth/login
 * @description 用户登录（无认证要求）
 */
router.post('/login', validate(loginValidationRules), async (req, res, next) => {
    try {
        const { password } = req.body;
        const ip = getClientIP(req);
        req.clientIP = ip;
        
        if (isLockedOut(ip)) {
            auditService.addAuditLog('login_locked', { ip }, req);
            throw new RateLimitError('登录失败次数过多，请30分钟后再试');
        }
        
        const storedHash = await passwordService.getStoredPassword();
        if (!storedHash) {
            return res.status(500).json({ error: '系统未初始化，请先设置密码' });
        }
        
        const isValid = await passwordService.verifyPassword(password, storedHash);
        
        if (!isValid) {
            recordLoginAttempt(ip, false);
            auditService.addAuditLog('login_failed', { ip }, req);
            const remaining = getRemainingAttempts(ip);
            return res.status(401).json({
                error: '密码错误',
                remaining: Math.max(0, remaining)
            });
        }
        
        recordLoginAttempt(ip, true);
        const token = sessionService.createSession('admin');
        const csrfToken = sessionService.getCSRFToken(token);
        auditService.addAuditLog('login_success', { ip }, req);
        
        res.cookie('session_token', token, COOKIE_OPTIONS);
        
        res.json({
            success: true,
            token: token,
            csrfToken: csrfToken,
            expiresIn: 24 * 60 * 60 * 1000
        });
    } catch (err) {
        next(err);
    }
});

/**
 * @route POST /api/auth/logout
 * @description 用户登出（无认证要求，允许清理状态）
 */
router.post('/logout', (req, res) => {
    const token = getSessionToken(req);
    if (token) {
        sessionService.deleteSession(token);
        auditService.addAuditLog('logout', {}, req);
    }
    res.clearCookie('session_token', { path: '/' });
    res.json({ success: true });
});

/**
 * @route POST /api/auth/password
 * @description 修改密码
 */
router.post('/password', validateToken, validateCSRF, validate(changePasswordValidationRules), async (req, res, next) => {
    try {
        const { currentPassword, newPassword } = req.body;
        
        const result = await passwordService.changePassword(currentPassword, newPassword);
        
        if (!result.success) {
            auditService.addAuditLog('password_change_failed', {}, req);
            return res.status(400).json(result);
        }
        
        auditService.addAuditLog('password_changed', {}, req);
        res.json(result);
    } catch (err) {
        next(err);
    }
});

/**
 * @route GET /api/auth/status
 * @description 获取认证状态
 */
router.get('/status', async (req, res) => {
    const hasPassword = await passwordService.isPasswordSet();
    const sessionToken = getSessionToken(req);
    const isLoggedIn = sessionToken && sessionService.validateSession(sessionToken);
    
    res.json({
        initialized: hasPassword,
        isLoggedIn: !!isLoggedIn,
        sessionExpiry: 24 * 60 * 60 * 1000
    });
});

/**
 * @route GET /api/csrf-token
 * @description 获取CSRF令牌
 */
router.get('/csrf-token', (req, res) => {
    const sessionToken = getSessionToken(req);
    if (sessionToken && sessionService.validateSession(sessionToken)) {
        const csrfToken = sessionService.getCSRFToken(sessionToken);
        res.json({ 
            csrfToken: csrfToken,
            token: sessionToken  // 也返回token供前端使用
        });
    } else {
        res.json({ csrfToken: null, token: null });
    }
});

module.exports = router;
