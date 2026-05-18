import express, { Request, Response } from 'express';
import * as sessionService from '../services/session';
import * as auditService from '../services/audit';
import { validateToken, validateCSRF, getSessionToken } from '../middleware/auth';

const router = express.Router();

const COOKIE_OPTIONS = {
    httpOnly: true,
    secure: process.env.FORCE_HTTPS === 'true',
    sameSite: 'lax' as const,
    maxAge: 24 * 60 * 60 * 1000,
    path: '/',
    domain: process.env.COOKIE_DOMAIN || undefined
};

router.post('/logout', validateToken, validateCSRF, (req: Request, res: Response) => {
    const token = getSessionToken(req);
    if (token) {
        sessionService.deleteSession(token);
        auditService.addAuditLog('logout', {}, req);
    }
    res.clearCookie('session_token', { path: '/' });
    res.json({ success: true });
});

router.get('/status', async (req: Request, res: Response) => {
    const isGatewayMode = !!process.env.GATEWAY_SOCKET;

    if (isGatewayMode) {
        res.json({
            initialized: true,
            isLoggedIn: true,
            isAdmin: req.headers['x-trim-isadmin'] === 'true',
            username: (req.headers['x-trim-username'] as string) || ''
        });
        return;
    }

    const sessionToken = getSessionToken(req);
    const isLoggedIn = sessionToken && sessionService.validateSession(sessionToken);
    res.json({
        initialized: !!isLoggedIn,
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
