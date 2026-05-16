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
    const sessionToken = getSessionToken(req);
    const isLoggedIn = sessionToken && sessionService.validateSession(sessionToken);
    const isGatewayMode = !!process.env.GATEWAY_SOCKET;

    if (isGatewayMode && !isLoggedIn) {
        const origin = req.headers.origin || '';
        const referer = req.headers.referer || '';
        const host = req.headers.host || '';
        const isSameOrigin = origin === `http://${host}` || origin === `https://${host}` ||
            referer.startsWith(`http://${host}/`) || referer.startsWith(`https://${host}/`);
        if (isSameOrigin) {
            const uid = (req.headers['x-trim-uid'] as string) || 'gateway';
            const token = sessionService.createSession(uid);
            const csrfToken = sessionService.getCSRFToken(token);
            res.cookie('session_token', token, COOKIE_OPTIONS);
            res.json({
                initialized: true,
                isLoggedIn: true,
                csrfToken,
                sessionToken: token,
                sessionExpiry: 24 * 60 * 60 * 1000
            });
            return;
        }
    }

    res.json({
        initialized: isGatewayMode ? true : !!isLoggedIn,
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
