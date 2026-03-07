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
// secure: 生产环境强制HTTPS
// sameSite: 'strict' 最严格的CSRF保护
const COOKIE_OPTIONS = {
    httpOnly: true,
    secure: process.env.NODE_ENV === 'production' || process.env.FORCE_HTTPS === 'true',
    sameSite: 'strict',
    maxAge: 24 * 60 * 60 * 1000,
    path: '/',
    domain: process.env.COOKIE_DOMAIN || undefined
};

/**
 * @route POST /api/auth/setup
 * @description 设置初始密码（仅限内网IP，系统启动30分钟内）
 */
router.post('/setup', validate([
    require('express-validator').body('password').isLength({ min: 8 }).withMessage('密码至少8位')
]), async (req, res, next) => {
    try {
        const { password } = req.body;
        const ip = getClientIP(req);
        req.clientIP = ip;
        
        // 安全检查1：只允许内网IP访问
        const isPrivateIP = (ipAddr) => {
            if (ipAddr.startsWith('10.')) return true;
            if (ipAddr.startsWith('172.')) {
                const second = parseInt(ipAddr.split('.')[1]);
                if (second >= 16 && second <= 31) return true;
            }
            if (ipAddr.startsWith('192.168.')) return true;
            if (ipAddr === '127.0.0.1' || ipAddr === '::1') return true;
            return false;
        };
        
        if (!isPrivateIP(ip)) {
            return res.status(403).json({
                success: false,
                error: '仅允许内网访问'
            });
        }
        
        // 安全检查2：系统启动后30分钟内允许设置
        const uptime = process.uptime();
        const maxSetupTime = 30 * 60;
        
        if (uptime > maxSetupTime) {
            return res.status(403).json({
                success: false,
                error: '初始设置时间已过'
            });
        }
        
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
        
        // 删除旧会话（防止会话固定攻击）
        const oldToken = getSessionToken(req);
        if (oldToken) {
            sessionService.deleteSession(oldToken);
        }
        
        // 创建新会话
        const token = sessionService.createSession('admin');
        const csrfToken = sessionService.getCSRFToken(token);
        auditService.addAuditLog('login_success', { ip }, req);
        
        res.cookie('session_token', token, COOKIE_OPTIONS);
        
        res.json({
            success: true,
            csrfToken: csrfToken,
            expiresIn: 24 * 60 * 60 * 1000
        });
    } catch (err) {
        next(err);
    }
});

/**
 * @route POST /api/auth/logout
 * @description 用户登出
 */
router.post('/logout', validateCSRF, (req, res) => {
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
            csrfToken: csrfToken
        });
    } else {
        res.json({ csrfToken: null });
    }
});

module.exports = router;
