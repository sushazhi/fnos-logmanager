import { body, validationResult, ValidationChain } from 'express-validator';
import { Request, Response, NextFunction } from 'express';
import * as sessionService from '../services/session';
import { getClientIP } from '../utils/ip';
import { AuthenticationError, CSRFError, ValidationError } from '../utils/errors';
import * as auditService from '../services/audit';
import { AuthenticatedRequest } from '../types';
import config from '../utils/config';

function getSessionToken(req: Request): string {
    const cookieToken = req.cookies?.session_token;
    if (cookieToken) return cookieToken;

    const authHeader = req.headers.authorization || '';
    if (authHeader.startsWith('Bearer ')) {
        return authHeader.slice(7);
    }

    const queryToken = (req.query.token as string) || '';
    if (!queryToken) return '';

    const allowedPath = config.auth.queryTokenPaths.includes(req.path);
    if (config.auth.allowQueryToken && allowedPath) {
        return queryToken;
    }

    return '';
}

export function validateToken(req: Request, res: Response, next: NextFunction): void {
    const clientIP = getClientIP(req);

    const referer = req.headers.referer || '';
    const host = req.headers.host || '';
    const origin = req.headers.origin || '';
    const hostBase = host.split(':')[0];
    let refererHost = '';
    let originHost = '';

    try {
        refererHost = referer ? new URL(referer).hostname : '';
        originHost = origin ? new URL(origin).hostname : '';
    } catch {
        // URL解析失败，继续处理
    }

    let isSameOrigin = false;
    if (origin && host) {
        isSameOrigin = originHost === hostBase || origin === `http://${host}` || origin === `https://${host}`;
    }
    if (!isSameOrigin && referer && host) {
        isSameOrigin = refererHost === hostBase || referer.startsWith(`http://${host}/`) || referer.startsWith(`https://${host}/`);
    }

    const token = getSessionToken(req);
    (req as AuthenticatedRequest).clientIP = clientIP;

    const isGatewayMode = !!process.env.GATEWAY_SOCKET;
    if (isGatewayMode) {
        const uid = (req.headers['x-trim-uid'] as string) || 'gateway';
        (req as AuthenticatedRequest).gatewayUid = uid;
        next();
        return;
    }

    if (!token) {
        auditService.addAuditLog('auth_failed', { path: req.path, clientIP, isSameOrigin }, req);
        next(new AuthenticationError());
        return;
    }

    if (!sessionService.validateSession(token)) {
        auditService.addAuditLog('auth_failed', { path: req.path, clientIP, isSameOrigin }, req);
        next(new AuthenticationError());
        return;
    }

    (req as AuthenticatedRequest).sessionToken = token;
    next();
}

export function validateCSRF(req: Request, res: Response, next: NextFunction): void {
    if (req.method === 'GET' || req.method === 'HEAD') {
        next();
        return;
    }

    const isGatewayMode = !!process.env.GATEWAY_SOCKET;
    if (isGatewayMode) {
        next();
        return;
    }

    const clientIP = (req as AuthenticatedRequest).clientIP || getClientIP(req);

    const sessionToken = (req as AuthenticatedRequest).sessionToken;
    const csrfToken = req.headers['x-csrf-token'] as string || req.body?._csrf;

    if (!sessionService.validateCSRFToken(sessionToken || '', csrfToken || '')) {
        auditService.addAuditLog('csrf_failed', { path: req.path, clientIP }, req);
        next(new CSRFError());
        return;
    }

    next();
}

type ValidatorFunction = (req: Request, res: Response, next: NextFunction) => void;

export function checkValidation(req: Request, _res: Response, next: NextFunction): void {
    const errors = validationResult(req);
    if (!errors.isEmpty()) {
        const errorMessages = errors.array().map(e => e.msg);
        next(new ValidationError(errorMessages.join(', '), errors.array()));
        return;
    }
    next();
}

export { getSessionToken };
