import { Request, Response, NextFunction } from 'express';
import { getClientIP } from '../utils/ip';
import { RateLimitError } from '../utils/errors';
import { RateLimitRecord } from '../types';
import config from '../utils/config';

const MAX_ENTRIES = 10000;

function boundedSet<K, V>(map: Map<K, V>, key: K, value: V): void {
    if (map.size >= MAX_ENTRIES && !map.has(key)) {
        const firstKey = map.keys().next().value;
        if (firstKey !== undefined) map.delete(firstKey);
    }
    map.set(key, value);
}

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

    boundedSet(rateLimitMap, ip, record);

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

export function apiRateLimit(maxRequests: number = 180, windowMs: number = 60000) {
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

        boundedSet(apiLimitMap, ip, record);

        if (record.count > maxRequests) {
            res.setHeader('Retry-After', String(Math.ceil((record.resetTime - now) / 1000)));
            next(new RateLimitError('API请求过于频繁，请稍后再试'));
            return;
        }

        next();
    };
}

export function sensitiveActionRateLimit(maxRequests: number = 30, windowMs: number = 300000) {
    return (req: Request, res: Response, next: NextFunction): void => {
        const ip = getClientIP(req);
        const now = Date.now();
        // 按 IP+路径独立计数（不同敏感操作互不影响）
        const key = ip + ':' + req.path;

        const record = sensitiveActionMap.get(key) || { count: 0, resetTime: now + windowMs };

        if (now > record.resetTime) {
            record.count = 1;
            record.resetTime = now + windowMs;
        } else {
            record.count++;
        }

        boundedSet(sensitiveActionMap, key, record);

        if (record.count > maxRequests) {
            res.setHeader('Retry-After', String(Math.ceil((record.resetTime - now) / 1000)));
            next(new RateLimitError('敏感操作过于频繁，请稍后再试'));
            return;
        }

        next();
    };
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

}

setInterval(cleanupRateLimits, 60000);
