import { Request } from 'express';

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
    const xForwardedFor = getFirstHeader(req.headers['x-forwarded-for']);
    if (xForwardedFor) {
        const ips = xForwardedFor.split(',').map((ip: string) => ip.trim());
        for (const ip of ips) {
            if (ip && !isLocalhost(ip) && !isPrivateIP(ip)) {
                return ip;
            }
        }
        if (ips.length > 0 && ips[0]) {
            return ips[0];
        }
    }

    const xRealIP = getFirstHeader(req.headers['x-real-ip']);
    if (xRealIP && !isLocalhost(xRealIP)) {
        return xRealIP;
    }

    const cfConnectingIP = getFirstHeader(req.headers['cf-connecting-ip']);
    if (cfConnectingIP && !isLocalhost(cfConnectingIP)) {
        return cfConnectingIP;
    }

    return req.ip || (req.connection?.remoteAddress) || (req.socket?.remoteAddress) || 'unknown';
}

export function isDirectAccess(req: Request, clientIP: string): boolean {
    const socketIP = req.ip || req.connection?.remoteAddress || req.socket?.remoteAddress || '';
    return socketIP === clientIP ||
           (isLocalhost(socketIP) && !req.headers['x-forwarded-for'] && !req.headers['x-real-ip']);
}
