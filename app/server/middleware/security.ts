import crypto from 'crypto';
import os from 'os';
import { Request, Response, NextFunction } from 'express';
import config from '../utils/config';

export function securityHeaders(req: Request, res: Response, next: NextFunction): void {
    res.setHeader('X-Content-Type-Options', 'nosniff');
    res.setHeader('X-XSS-Protection', '1; mode=block');
    res.setHeader('X-Frame-Options', 'SAMEORIGIN');
    res.setHeader('Strict-Transport-Security', 'max-age=31536000; includeSubDomains');
    res.setHeader('X-Permitted-Cross-Domain-Policies', 'none');
    res.setHeader('X-Download-Options', 'noopen');
    res.setHeader('X-DNS-Prefetch-Control', 'off');
    res.removeHeader('X-Powered-By');
    res.removeHeader('Server');

    // 获取客户端可见的主机名，供 CSP 和 frame-ancestors 使用
    // 环境：可能经过 fnOS 网关（Unix socket）或反向代理，Host 头可能为内部地址
    function getClientHost(req: Request): string {
        // 1. X-Forwarded-Host（网关/反向代理透传的原始 Host）
        const forwardedHost = req.headers['x-forwarded-host'] as string | undefined;
        if (forwardedHost) {
            const fh = forwardedHost.split(':')[0];
            if (fh && fh !== '127.0.0.1' && fh !== 'localhost') return fh;
        }
        // 2. Origin 头（浏览器实际访问地址，但导航请求不携带）
        const origin = req.headers.origin || '';
        if (origin) {
            try {
                const oh = new URL(origin).hostname;
                if (oh && oh !== '127.0.0.1' && oh !== 'localhost') return oh;
            } catch { /* fall through */ }
        }
        // 3. Host 头（网关/反向代理可能改写为内部地址）
        const host = req.headers.host || '';
        const hostBase = host.split(':')[0];
        if (hostBase && hostBase !== '127.0.0.1' && hostBase !== 'localhost') {
            return hostBase;
        }
        // 4. 全失败时，尝试获取本机 LAN IP
        try {
            const interfaces = os.networkInterfaces();
            for (const name of Object.keys(interfaces)) {
                for (const iface of interfaces[name] || []) {
                    if (iface.family === 'IPv4' && !iface.internal) {
                        return iface.address;
                    }
                }
            }
        } catch { /* fall through */ }
        return hostBase || '127.0.0.1';
    }
    const hostBase = getClientHost(req);
    // 判断是否为 IP 地址（纯数字和点组成）
    const isIpAddress = /^\d{1,3}(\.\d{1,3}){3}$/.test(hostBase);
    
    // 构建 frame-ancestors
    let frameAncestors: string;
    if (isIpAddress) {
        frameAncestors = `'self' http://${hostBase}:* https://${hostBase}:*`;
    } else {
        const domainParts = hostBase.split('.');
        const mainDomain = domainParts.length >= 2 
            ? domainParts.slice(-2).join('.') 
            : hostBase;
        frameAncestors = `'self' https://*.${mainDomain} http://*.${mainDomain}`;
    }

    // WebSocket 直连本地端口（ARM CGI 代理 / x86 绕过网关），需加入 CSP
    const wsOrigin = ` ws://${hostBase}:${config.port}`;

    const cspNonce = crypto.randomBytes(16).toString('base64');
    res.locals.cspNonce = cspNonce;

    res.setHeader('Content-Security-Policy',
        `default-src 'self'; ` +
        `script-src 'self' 'nonce-${cspNonce}'; ` +
        `style-src 'self' 'unsafe-inline'; ` +
        `img-src 'self' data:; ` +
        `font-src 'self' data:; ` +
        `connect-src 'self' https://api.github.com${wsOrigin}; ` +
        `frame-ancestors ${frameAncestors}; ` +
        `base-uri 'self'; ` +
        `form-action 'self'; ` +
        `object-src 'none'`
    );

    res.setHeader('Referrer-Policy', 'strict-origin-when-cross-origin');
    res.setHeader('Permissions-Policy', 'geolocation=(), microphone=(), camera=()');

    next();
}

export function validateContentType(req: Request, res: Response, next: NextFunction): void {
    if (['POST', 'PUT', 'PATCH'].includes(req.method)) {
        const contentType = req.headers['content-type'];
        if (!contentType || !contentType.includes('application/json')) {
            res.status(415).json({ error: '不支持的媒体类型', code: 'UNSUPPORTED_MEDIA_TYPE' });
            return;
        }
    }
    next();
}

export function requestSizeLimit(maxSize: string = '10mb') {
    return (req: Request, res: Response, next: NextFunction): void => {
        const contentLength = parseInt(req.headers['content-length'] || '0', 10);
        const maxBytes = parseSize(maxSize);

        if (contentLength > maxBytes) {
            res.status(413).json({ error: '请求体过大', code: 'PAYLOAD_TOO_LARGE' });
            return;
        }
        next();
    };
}

function parseSize(size: string): number {
    const match = size.match(/^(\d+)(kb|mb|gb)?$/i);
    if (!match) return 10 * 1024 * 1024;

    const num = parseInt(match[1], 10);
    const unit = (match[2] || '').toLowerCase();

    const multipliers: Record<string, number> = {
        '': 1,
        'kb': 1024,
        'mb': 1024 * 1024,
        'gb': 1024 * 1024 * 1024
    };

    return num * (multipliers[unit] || 1);
}

export function sanitizeInput(req: Request, res: Response, next: NextFunction): void {
    if (req.body && typeof req.body === 'object') {
        req.body = sanitizeObject(req.body);
    }
    next();
}

function sanitizeObject(obj: Record<string, unknown>): Record<string, unknown> {
    const sanitized: Record<string, unknown> = {};

    for (const [key, value] of Object.entries(obj)) {
        if (typeof value === 'string') {
            sanitized[key] = sanitizeString(value);
        } else if (Array.isArray(value)) {
            // 处理数组：递归处理每个元素，保持数组结构
            sanitized[key] = value.map(item => {
                if (typeof item === 'string') {
                    return sanitizeString(item);
                } else if (typeof item === 'object' && item !== null) {
                    return sanitizeObject(item as Record<string, unknown>);
                }
                return item;
            });
        } else if (typeof value === 'object' && value !== null) {
            sanitized[key] = sanitizeObject(value as Record<string, unknown>);
        } else {
            sanitized[key] = value;
        }
    }

    return sanitized;
}

/**
 * 字符串净化：使用白名单方式防御 XSS
 * 1. HTML 实体编码 < 和 >，防止任何 HTML 标签注入
 * 2. 移除 javascript:/vbscript:/data: 协议
 * 3. 移除事件处理器属性 on*=
 */
function sanitizeString(value: string): string {
    return value
        // HTML 实体编码尖括号，阻止所有 HTML 标签注入
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;')
        // 移除危险协议
        .replace(/javascript\s*:/gi, '')
        .replace(/vbscript\s*:/gi, '')
        .replace(/data\s*:/gi, '')
        // 移除事件处理器（on*= 形式，允许空格）
        .replace(/\bon\w+\s*=\s*/gi, '');
}

export function validateEnv(): { valid: boolean; errors: string[] } {
    const errors: string[] = [];

    const port = process.env.PORT;
    if (port) {
        const portNum = parseInt(port, 10);
        if (isNaN(portNum) || portNum < 1 || portNum > 65535) {
            errors.push(`Invalid PORT: ${port}`);
        }
    }

    const dataDir = process.env.LOGMANAGER_DATA_DIR;
    if (dataDir) {
        if (dataDir.includes('..') || dataDir.includes('\0')) {
            errors.push('LOGMANAGER_DATA_DIR contains invalid characters');
        }
    }

    return {
        valid: errors.length === 0,
        errors
    };
}

export function checkDependencies(): { valid: boolean; missing: string[] } {
    const missing: string[] = [];

    const requiredDeps = ['express', '@node-rs/argon2', 'express-validator', 'morgan', 'cookie-parser'];

    for (const dep of requiredDeps) {
        try {
            require.resolve(dep);
        } catch {
            missing.push(dep);
        }
    }

    return {
        valid: missing.length === 0,
        missing
    };
}
