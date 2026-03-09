import { Request, Response, NextFunction } from 'express';
import { getClientIP } from '../utils/ip';
import { RateLimitError } from '../utils/errors';
import { RateLimitRecord, LoginAttempt } from '../types';
import config from '../utils/config';

const rateLimitMap = new Map<string, RateLimitRecord>();
const apiLimitMap = new Map<string, RateLimitRecord>();
const sensitiveActionMap = new Map<string, RateLimitRecord>();

export function rateLimit(req: Request, res: Response, next: NextFunction): void {
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

    res.setHeader('X-RateLimit-Limit', String(config.rateLimit.maxRequests));
    res.setHeader('X-RateLimit-Remaining', String(Math.max(0, config.rateLimit.maxRequests - record.count)));
    res.setHeader('X-RateLimit-Reset', String(record.resetTime));

    if (record.count > config.rateLimit.maxRequests) {
        res.setHeader('Retry-After', String(Math.ceil((record.resetTime - now) / 1000)));
        next(new RateLimitError('请求过于频繁，请稍后再试'));
        return;
    }

    next();
}

export function apiRateLimit(maxRequests: number = 60, windowMs: number = 60000) {
    return (req: Request, res: Response, next: NextFunction): void => {
        const ip = getClientIP(req);
        const now = Date.now();

        const record = apiLimitMap.get(ip) || { count: 0, resetTime: now + windowMs };

        if (now > record.resetTime) {
            record.count = 1;
            record.resetTime = now + windowMs;
        } else {
            record.count++;
        }

        apiLimitMap.set(ip, record);

        if (record.count > maxRequests) {
            res.setHeader('Retry-After', String(Math.ceil((record.resetTime - now) / 1000)));
            next(new RateLimitError('API请求过于频繁，请稍后再试'));
            return;
        }

        next();
    };
}

export function sensitiveActionRateLimit(maxRequests: number = 10, windowMs: number = 300000) {
    return (req: Request, res: Response, next: NextFunction): void => {
        const ip = getClientIP(req);
        const now = Date.now();

        const record = sensitiveActionMap.get(ip) || { count: 0, resetTime: now + windowMs };

        if (now > record.resetTime) {
            record.count = 1;
            record.resetTime = now + windowMs;
        } else {
            record.count++;
        }

        sensitiveActionMap.set(ip, record);

        if (record.count > maxRequests) {
            res.setHeader('Retry-After', String(Math.ceil((record.resetTime - now) / 1000)));
            next(new RateLimitError('敏感操作过于频繁，请稍后再试'));
            return;
        }

        next();
    };
}

const loginAttemptsMap = new Map<string, LoginAttempt>();

export function getLoginAttempts(ip: string): LoginAttempt {
    const attempts = loginAttemptsMap.get(ip) || { count: 0, lockoutUntil: 0 };
    if (attempts.lockoutUntil && Date.now() > attempts.lockoutUntil) {
        loginAttemptsMap.delete(ip);
        return { count: 0, lockoutUntil: 0 };
    }
    return attempts;
}

export function recordLoginAttempt(ip: string, success: boolean): void {
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

export function isLockedOut(ip: string): boolean {
    const attempts = getLoginAttempts(ip);
    return attempts.lockoutUntil > 0;
}

export function getRemainingAttempts(ip: string): number {
    const attempts = getLoginAttempts(ip);
    return Math.max(0, config.login.maxAttempts - attempts.count);
}

export function getLockoutRemainingTime(ip: string): number {
    const attempts = getLoginAttempts(ip);
    if (attempts.lockoutUntil > 0) {
        return Math.max(0, attempts.lockoutUntil - Date.now());
    }
    return 0;
}

function cleanupRateLimits(): void {
    const now = Date.now();

    for (const [ip, record] of rateLimitMap.entries()) {
        if (now > record.resetTime) {
            rateLimitMap.delete(ip);
        }
    }

    for (const [ip, record] of apiLimitMap.entries()) {
        if (now > record.resetTime) {
            apiLimitMap.delete(ip);
        }
    }

    for (const [ip, record] of sensitiveActionMap.entries()) {
        if (now > record.resetTime) {
            sensitiveActionMap.delete(ip);
        }
    }

    for (const [ip, attempts] of loginAttemptsMap.entries()) {
        if (attempts.lockoutUntil && now > attempts.lockoutUntil) {
            loginAttemptsMap.delete(ip);
        }
    }
}

setInterval(cleanupRateLimits, 60000);
