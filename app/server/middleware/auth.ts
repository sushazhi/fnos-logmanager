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

    const isSameOrigin =
        referer.includes('/app-center/') ||
        referer.includes('/desktop/') ||
        origin.includes(host) ||
        referer.startsWith(`http://${host}`) ||
        referer.startsWith(`https://${host}`) ||
        refererHost === hostBase ||
        originHost === hostBase;

    const token = getSessionToken(req);
    (req as AuthenticatedRequest).clientIP = clientIP;

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

export function validate(validators: ValidationChain[]): ValidatorFunction[] {
    return [
        ...validators,
        (req: Request, res: Response, next: NextFunction) => {
            const errors = validationResult(req);
            if (!errors.isEmpty()) {
                const errorMessages = errors.array().map(e => e.msg);
                next(new ValidationError(errorMessages.join(', '), errors.array()));
                return;
            }
            next();
        }
    ];
}

export const loginValidationRules: ValidationChain[] = [
    body('password')
        .notEmpty().withMessage('请输入密码')
        .isString().withMessage('密码必须是字符串')
];

export const changePasswordValidationRules: ValidationChain[] = [
    body('currentPassword')
        .notEmpty().withMessage('请输入当前密码'),
    body('newPassword')
        .notEmpty().withMessage('请输入新密码')
        .isLength({ min: 8 }).withMessage('新密码至少8位')
];

export const logPathValidationRules: ValidationChain[] = [
    body('path').optional()
        .isString().withMessage('路径必须是字符串')
        .custom((value) => {
            if (value && (value.includes('..') || value.includes('\0'))) {
                throw new Error('无效的路径');
            }
            return true;
        })
];

export { getSessionToken };
