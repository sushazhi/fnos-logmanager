/**
 * @fileoverview 安全头中间件
 */

const crypto = require('crypto');

/**
 * 安全头中间件
 */
function securityHeaders(req, res, next) {
    res.setHeader('X-Content-Type-Options', 'nosniff');
    res.setHeader('X-XSS-Protection', '1; mode=block');
    res.setHeader('Strict-Transport-Security', 'max-age=31536000; includeSubDomains');
    
    const host = req.headers.host || '';
    const hostBase = host.split(':')[0];
    const frameAncestors = `'self' http://${hostBase}:* https://${hostBase}:*`;
    
    const cspNonce = crypto.randomBytes(16).toString('base64');
    res.locals.cspNonce = cspNonce;
    
    res.setHeader('Content-Security-Policy', 
        `default-src 'self'; ` +
        `script-src 'self' 'nonce-${cspNonce}'; ` +
        `style-src 'self' 'unsafe-inline'; ` +
        `img-src 'self' data:; ` +
        `font-src 'self' data:; ` +
        `connect-src 'self' https://api.github.com; ` +
        `frame-ancestors ${frameAncestors}`
    );
    
    next();
}

module.exports = { securityHeaders };
