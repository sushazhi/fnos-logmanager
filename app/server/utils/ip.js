/**
 * @fileoverview IP 相关工具函数
 */

/**
 * 判断是否为私有IP地址
 * @param {string} ip - IP地址
 * @returns {boolean}
 */
function isPrivateIP(ip) {
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

/**
 * 判断是否为本机地址
 * @param {string} ip - IP地址
 * @returns {boolean}
 */
function isLocalhost(ip) {
    if (!ip) return false;
    const cleanIP = ip.replace(/^::ffff:/, '').replace(/:.*$/, '');
    return cleanIP === '127.0.0.1' || cleanIP === '::1' || cleanIP === 'localhost';
}

/**
 * 从请求中获取真实客户端IP
 * @param {import('express').Request} req - Express请求对象
 * @returns {string}
 */
function getClientIP(req) {
    const xForwardedFor = req.headers['x-forwarded-for'];
    if (xForwardedFor) {
        const ips = xForwardedFor.split(',').map(ip => ip.trim());
        // 优先返回第一个非私有、非本地的IP
        for (const ip of ips) {
            if (ip && !isLocalhost(ip) && !isPrivateIP(ip)) {
                return ip;
            }
        }
        // 如果都是私有IP，返回第一个（可能是真实客户端IP）
        if (ips.length > 0 && ips[0]) {
            return ips[0];
        }
    }
    
    const xRealIP = req.headers['x-real-ip'];
    if (xRealIP && !isLocalhost(xRealIP)) {
        return xRealIP;
    }
    
    const cfConnectingIP = req.headers['cf-connecting-ip'];
    if (cfConnectingIP && !isLocalhost(cfConnectingIP)) {
        return cfConnectingIP;
    }
    
    return req.ip || req.connection?.remoteAddress || req.socket?.remoteAddress || 'unknown';
}

/**
 * 判断请求是否为直接访问（非代理）
 * @param {import('express').Request} req - Express请求对象
 * @param {string} clientIP - 客户端IP
 * @returns {boolean}
 */
function isDirectAccess(req, clientIP) {
    const socketIP = req.ip || req.connection?.remoteAddress || req.socket?.remoteAddress || '';
    return socketIP === clientIP || 
           (isLocalhost(socketIP) && !req.headers['x-forwarded-for'] && !req.headers['x-real-ip']);
}

module.exports = {
    isPrivateIP,
    isLocalhost,
    getClientIP,
    isDirectAccess
};
