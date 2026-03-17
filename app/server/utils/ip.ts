import { Request } from 'express';

const TRUSTED_PROXIES = process.env.TRUSTED_PROXIES 
    ? process.env.TRUSTED_PROXIES.split(',').map(ip => ip.trim())
    : ['127.0.0.1', '::1', 'localhost'];

function isTrustedProxy(ip: string): boolean {
    if (!ip) return false;
    return TRUSTED_PROXIES.includes(ip);
}

export function isPrivateIP(ip: string): boolean {
    if (!ip) return false;
    const cleanIP = ip.replace(/^::ffff:/, '').replace(/:.*$/, '');
    return cleanIP.startsWith('10.') ||
           cleanIP.startsWith('192.168.') ||
           cleanIP.startsWith('172.16.') ||
           cleanIP.startsWith('172.17.') ||
           cleanIP.startsWith('172.18.') ||
           cleanIP.startsWith('172.19.') ||
           cleanIP.startsWith('172.20.') ||
           cleanIP.startsWith('172.21.') ||
           cleanIP.startsWith('172.22.') ||
           cleanIP.startsWith('172.23.') ||
           cleanIP.startsWith('172.24.') ||
           cleanIP.startsWith('172.25.') ||
           cleanIP.startsWith('172.26.') ||
           cleanIP.startsWith('172.27.') ||
           cleanIP.startsWith('172.28.') ||
           cleanIP.startsWith('172.29.') ||
           cleanIP.startsWith('172.30.') ||
           cleanIP.startsWith('172.31.');
}

export function isLocalhost(ip: string): boolean {
    if (!ip) return false;
    const cleanIP = ip.replace(/^::ffff:/, '').replace(/:.*$/, '');
    return cleanIP === '127.0.0.1' || cleanIP === '::1' || cleanIP === 'localhost';
}

function getFirstHeader(value: string | string[] | undefined): string | undefined {
    if (!value) return undefined;
    if (Array.isArray(value)) return value[0];
    return value;
}

export function getClientIP(req: Request): string {
    const socketIP = req.ip || req.connection?.remoteAddress || req.socket?.remoteAddress || '';
    
    // 检查是否为直接访问（非代理）
    const isDirect = !req.headers['x-forwarded-for'] && !req.headers['x-real-ip'];
    
    // 如果是直接访问，直接返回socket IP
    if (isDirect) {
        return socketIP || 'unknown';
    }
    
    // 如果有代理头，只从可信代理获取真实IP
    const xRealIP = getFirstHeader(req.headers['x-real-ip']);
    if (xRealIP && isTrustedProxy(socketIP)) {
        const cleanIP = xRealIP.replace(/^::ffff:/, '').replace(/:.*$/, '');
        if (cleanIP && !isLocalhost(cleanIP)) {
            return cleanIP;
        }
    }
    
    const cfConnectingIP = getFirstHeader(req.headers['cf-connecting-ip']);
    if (cfConnectingIP && isTrustedProxy(socketIP)) {
        const cleanIP = cfConnectingIP.replace(/^::ffff:/, '').replace(/:.*$/, '');
        if (cleanIP && !isLocalhost(cleanIP)) {
            return cleanIP;
        }
    }
    
    // 如果socketIP本身是可信的代理，从X-Forwarded-For获取
    if (isTrustedProxy(socketIP)) {
        const xForwardedFor = getFirstHeader(req.headers['x-forwarded-for']);
        if (xForwardedFor) {
            const ips = xForwardedFor.split(',').map((ip: string) => ip.trim());
            // 返回第一个非本地、非私有的IP（即客户端真实IP）
            for (const ip of ips) {
                if (ip && !isLocalhost(ip) && !isPrivateIP(ip)) {
                    return ip;
                }
            }
            // 如果都是私有IP，返回第一个
            if (ips.length > 0 && ips[0]) {
                return ips[0];
            }
        }
    }
    
    // 降级：返回socket IP
    return socketIP || 'unknown';
}

export function isDirectAccess(req: Request, clientIP: string): boolean {
    const socketIP = req.ip || req.connection?.remoteAddress || req.socket?.remoteAddress || '';
    return socketIP === clientIP ||
           (isLocalhost(socketIP) && !req.headers['x-forwarded-for'] && !req.headers['x-real-ip']);
}
