import express, { Request, Response, NextFunction } from 'express';
import { body } from 'express-validator';
import * as passwordService from '../services/password';
import * as sessionService from '../services/session';
import * as auditService from '../services/audit';
import { validateToken, validateCSRF, validate, getSessionToken, loginValidationRules, changePasswordValidationRules } from '../middleware/auth';
import { getClientIP } from '../utils/ip';
import { isLockedOut, recordLoginAttempt, getRemainingAttempts } from '../middleware/rateLimit';
import { RateLimitError } from '../utils/errors';
import { AuthenticatedRequest } from '../types';

const router = express.Router();

const COOKIE_OPTIONS = {
    httpOnly: true,
    secure: process.env.NODE_ENV === 'production' || process.env.FORCE_HTTPS === 'true',
    sameSite: 'strict' as const,
    maxAge: 24 * 60 * 60 * 1000,
    path: '/',
    domain: process.env.COOKIE_DOMAIN || undefined
};

function isPrivateIP(ipAddr: string): boolean {
    if (ipAddr.startsWith('10.')) return true;
    if (ipAddr.startsWith('172.')) {
        const second = parseInt(ipAddr.split('.')[1]);
        if (second >= 16 && second <= 31) return true;
    }
    if (ipAddr.startsWith('192.168.')) return true;
    if (ipAddr === '127.0.0.1' || ipAddr === '::1') return true;
    return false;
}

router.post('/setup', validate([
    body('password').isLength({ min: 8 }).withMessage('密码至少8位')
]), async (req: Request, res: Response, next: NextFunction) => {
    try {
        const { password } = req.body;
        const ip = getClientIP(req);
        (req as AuthenticatedRequest).clientIP = ip;

        if (!isPrivateIP(ip)) {
            res.status(403).json({
                success: false,
                error: '仅允许内网访问'
            });
            return;
        }

        const uptime = process.uptime();
        const maxSetupTime = 30 * 60;

        if (uptime > maxSetupTime) {
            res.status(403).json({
                success: false,
                error: '初始设置时间已过'
            });
            return;
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

router.post('/login', validate(loginValidationRules), async (req: Request, res: Response, next: NextFunction) => {
    try {
        const { password } = req.body;
        const ip = getClientIP(req);
        (req as AuthenticatedRequest).clientIP = ip;

        if (isLockedOut(ip)) {
            auditService.addAuditLog('login_locked', { ip }, req);
            throw new RateLimitError('登录失败次数过多，请30分钟后再试');
        }

        const storedHash = await passwordService.getStoredPassword();
        if (!storedHash) {
            res.status(500).json({ error: '系统未初始化，请先设置密码' });
            return;
        }

        const isValid = await passwordService.verifyPassword(password, storedHash);

        if (!isValid) {
            recordLoginAttempt(ip, false);
            auditService.addAuditLog('login_failed', { ip }, req);
            const remaining = getRemainingAttempts(ip);
            res.status(401).json({
                error: '密码错误',
                remaining: Math.max(0, remaining)
            });
            return;
        }

        recordLoginAttempt(ip, true);

        const oldToken = getSessionToken(req);
        if (oldToken) {
            sessionService.deleteSession(oldToken);
        }

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

router.post('/logout', validateCSRF, (req: Request, res: Response) => {
    const token = getSessionToken(req);
    if (token) {
        sessionService.deleteSession(token);
        auditService.addAuditLog('logout', {}, req);
    }
    res.clearCookie('session_token', { path: '/' });
    res.json({ success: true });
});

router.post('/password', validateToken, validateCSRF, validate(changePasswordValidationRules), async (req: Request, res: Response, next: NextFunction) => {
    try {
        const { currentPassword, newPassword } = req.body;

        const result = await passwordService.changePassword(currentPassword, newPassword);

        if (!result.success) {
            auditService.addAuditLog('password_change_failed', {}, req);
            res.status(400).json(result);
            return;
        }

        auditService.addAuditLog('password_changed', {}, req);
        res.json(result);
    } catch (err) {
        next(err);
    }
});

router.get('/status', async (req: Request, res: Response) => {
    const hasPassword = await passwordService.isPasswordSet();
    const sessionToken = getSessionToken(req);
    const isLoggedIn = sessionToken && sessionService.validateSession(sessionToken);

    res.json({
        initialized: hasPassword,
        isLoggedIn: !!isLoggedIn,
        sessionExpiry: 24 * 60 * 60 * 1000
    });
});

router.get('/csrf-token', (req: Request, res: Response) => {
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

export default router;
