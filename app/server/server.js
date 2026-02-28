const express = require('express');
const { exec, execSync } = require('child_process');
const path = require('path');
const fs = require('fs');
const crypto = require('crypto');

const app = express();
const PORT = process.env.PORT || 8090;

const DATA_DIR = process.env.LOGMANAGER_DATA_DIR || '/vol1/@appdata/logmanager';
const PASSWORD_FILE = path.join(DATA_DIR, '.password');
const AUDIT_LOG_FILE = path.join(DATA_DIR, 'audit.log');

let FILTER_SENSITIVE = process.env.FILTER_SENSITIVE !== 'false';

const rateLimitMap = new Map();
const RATE_LIMIT_WINDOW = 60000;
const RATE_LIMIT_MAX = 100;

const SESSION_EXPIRY = 24 * 60 * 60 * 1000;
const sessions = new Map();

const loginAttempts = new Map();
const LOGIN_LOCKOUT_TIME = 30 * 60 * 1000;
const MAX_LOGIN_ATTEMPTS = 5;

const MAX_AUDIT_LOG = 1000;

const SENSITIVE_PATTERNS = [
    /password\s*[=:]\s*\S+/gi,
    /passwd\s*[=:]\s*\S+/gi,
    /secret\s*[=:]\s*\S+/gi,
    /api[_-]?key\s*[=:]\s*\S+/gi,
    /token\s*[=:]\s*\S+/gi,
    /private[_-]?key\s*[=:]\s*\S+/gi,
    /access[_-]?key\s*[=:]\s*\S+/gi,
    /auth[_-]?key\s*[=:]\s*\S+/gi,
    /-----BEGIN\s+(RSA\s+)?PRIVATE\s+KEY-----/g,
    /-----BEGIN\s+OPENSSH\s+PRIVATE\s+KEY-----/g,
    /eyJ[A-Za-z0-9-_]+\.[A-Za-z0-9-_]+\.[A-Za-z0-9-_.+/]*/g
];

function generateToken() {
    return crypto.randomBytes(32).toString('hex');
}

function hashPassword(password) {
    return crypto.createHash('sha256').update(password).digest('hex');
}

function ensureDataDir() {
    try {
        if (!fs.existsSync(DATA_DIR)) {
            fs.mkdirSync(DATA_DIR, { recursive: true, mode: 0o700 });
        }
    } catch (e) {}
}

function getStoredPassword() {
    try {
        if (fs.existsSync(PASSWORD_FILE)) {
            return fs.readFileSync(PASSWORD_FILE, 'utf8').trim();
        }
    } catch (e) {}
    return null;
}

function setStoredPassword(hashedPassword) {
    ensureDataDir();
    fs.writeFileSync(PASSWORD_FILE, hashedPassword, { mode: 0o600 });
}

function initPassword() {
    const stored = getStoredPassword();
    if (stored) return null;
    
    const defaultPassword = generateToken().substring(0, 12);
    setStoredPassword(hashPassword(defaultPassword));
    return defaultPassword;
}

function getLoginAttempts(ip) {
    const attempts = loginAttempts.get(ip) || { count: 0, lockoutUntil: 0 };
    if (attempts.lockoutUntil && Date.now() > attempts.lockoutUntil) {
        loginAttempts.delete(ip);
        return { count: 0, lockoutUntil: 0 };
    }
    return attempts;
}

function recordLoginAttempt(ip, success) {
    if (success) {
        loginAttempts.delete(ip);
        return;
    }
    
    const attempts = getLoginAttempts(ip);
    attempts.count++;
    if (attempts.count >= MAX_LOGIN_ATTEMPTS) {
        attempts.lockoutUntil = Date.now() + LOGIN_LOCKOUT_TIME;
    }
    loginAttempts.set(ip, attempts);
}

function isLockedOut(ip) {
    const attempts = getLoginAttempts(ip);
    return attempts.lockoutUntil > 0;
}

function addAuditLog(action, details, req) {
    const entry = {
        timestamp: new Date().toISOString(),
        action: action,
        details: details,
        ip: req ? (req.ip || req.connection.remoteAddress || 'unknown') : 'system',
        userAgent: req ? (req.headers['user-agent'] || 'unknown') : 'system'
    };
    
    try {
        ensureDataDir();
        const logLine = JSON.stringify(entry) + '\n';
        fs.appendFileSync(AUDIT_LOG_FILE, logLine, { mode: 0o600 });
    } catch (e) {
        console.error('[LogManager] 写入审计日志失败:', e.message);
    }
}

function getAuditLogs(limit = 100) {
    try {
        if (!fs.existsSync(AUDIT_LOG_FILE)) {
            return [];
        }
        const content = fs.readFileSync(AUDIT_LOG_FILE, 'utf8');
        const lines = content.trim().split('\n').filter(line => line);
        const logs = lines.slice(-limit * 10).map(line => {
            try {
                return JSON.parse(line);
            } catch (e) {
                return null;
            }
        }).filter(log => log);
        return logs.reverse().slice(0, limit);
    } catch (e) {
        console.error('[LogManager] 读取审计日志失败:', e.message);
        return [];
    }
}

function cleanOldAuditLogs() {
    try {
        if (!fs.existsSync(AUDIT_LOG_FILE)) return;
        
        const content = fs.readFileSync(AUDIT_LOG_FILE, 'utf8');
        const lines = content.trim().split('\n').filter(line => line);
        
        if (lines.length > MAX_AUDIT_LOG) {
            const keptLines = lines.slice(-MAX_AUDIT_LOG);
            fs.writeFileSync(AUDIT_LOG_FILE, keptLines.join('\n') + '\n', { mode: 0o600 });
        }
    } catch (e) {
        console.error('[LogManager] 清理审计日志失败:', e.message);
    }
}

function isPrivateIP(ip) {
    if (!ip) return false;
    const cleanIP = ip.replace(/^::ffff:/, '').replace(/:.*$/, '');
    return cleanIP === '127.0.0.1' ||
           cleanIP.startsWith('10.') ||
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

function validateToken(req) {
    const clientIP = req.ip || req.connection.remoteAddress || req.socket.remoteAddress || '';
    if (isPrivateIP(clientIP)) return true;
    
    const referer = req.headers.referer || '';
    const host = req.headers.host || '';
    const origin = req.headers.origin || '';
    
    if (referer.includes('/app-center/') || 
        referer.includes('/desktop/') ||
        origin.includes(host) ||
        referer.startsWith(`http://${host}`) ||
        referer.startsWith(`https://${host}`)) {
        return true;
    }
    
    const authHeader = req.headers.authorization || '';
    const queryToken = req.query.token || '';
    const token = authHeader.startsWith('Bearer ') ? authHeader.slice(7) : queryToken;
    
    if (!token) return false;
    
    const session = sessions.get(token);
    if (session) {
        if (Date.now() - session.lastAccess > SESSION_EXPIRY) {
            sessions.delete(token);
            return false;
        }
        session.lastAccess = Date.now();
        return true;
    }
    
    return false;
}

function createSession(username) {
    const token = generateToken();
    sessions.set(token, {
        username: username,
        createdAt: Date.now(),
        lastAccess: Date.now()
    });
    return token;
}

function cleanExpiredSessions() {
    const now = Date.now();
    for (const [token, session] of sessions.entries()) {
        if (now - session.lastAccess > SESSION_EXPIRY) {
            sessions.delete(token);
        }
    }
}

setInterval(cleanExpiredSessions, 3600000);

const defaultPassword = initPassword();
if (defaultPassword) {
    console.log(`[LogManager] 初始密码已生成: ${defaultPassword}`);
    console.log(`[LogManager] 请登录后立即修改密码`);
}

app.use(express.json());

app.post('/api/auth/login', (req, res) => {
    const { password } = req.body;
    const ip = req.ip || req.connection.remoteAddress || 'unknown';
    
    if (!password) {
        return res.status(400).json({ error: '请输入密码' });
    }
    
    if (isLockedOut(ip)) {
        addAuditLog('login_locked', { ip }, req);
        return res.status(429).json({ error: '登录失败次数过多，请30分钟后再试' });
    }
    
    const storedPassword = getStoredPassword();
    if (!storedPassword) {
        return res.status(500).json({ error: '系统未初始化' });
    }
    
    if (hashPassword(password) !== storedPassword) {
        recordLoginAttempt(ip, false);
        addAuditLog('login_failed', { ip }, req);
        const attempts = getLoginAttempts(ip);
        const remaining = MAX_LOGIN_ATTEMPTS - attempts.count;
        return res.status(401).json({ 
            error: '密码错误', 
            remaining: Math.max(0, remaining)
        });
    }
    
    recordLoginAttempt(ip, true);
    const token = createSession('admin');
    addAuditLog('login_success', { ip }, req);
    
    res.json({ 
        success: true, 
        token: token,
        expiresIn: SESSION_EXPIRY
    });
});

app.post('/api/auth/logout', (req, res) => {
    const authHeader = req.headers.authorization || '';
    const token = authHeader.startsWith('Bearer ') ? authHeader.slice(7) : '';
    
    if (token && sessions.has(token)) {
        sessions.delete(token);
        addAuditLog('logout', {}, req);
    }
    
    res.json({ success: true });
});

app.post('/api/auth/password', (req, res) => {
    const { currentPassword, newPassword } = req.body;
    
    if (!currentPassword || !newPassword) {
        return res.status(400).json({ error: '请输入当前密码和新密码' });
    }
    
    if (newPassword.length < 8) {
        return res.status(400).json({ error: '新密码至少8位' });
    }
    
    const storedPassword = getStoredPassword();
    if (hashPassword(currentPassword) !== storedPassword) {
        addAuditLog('password_change_failed', {}, req);
        return res.status(401).json({ error: '当前密码错误' });
    }
    
    setStoredPassword(hashPassword(newPassword));
    addAuditLog('password_changed', {}, req);
    
    res.json({ success: true, message: '密码已修改' });
});

app.get('/api/auth/status', (req, res) => {
    const hasPassword = !!getStoredPassword();
    res.json({ 
        initialized: hasPassword,
        sessionExpiry: SESSION_EXPIRY
    });
});

function rateLimit(req, res, next) {
    const ip = req.ip || req.connection.remoteAddress || 'unknown';
    const key = ip;
    const now = Date.now();
    
    const record = rateLimitMap.get(key) || { count: 0, resetTime: now + RATE_LIMIT_WINDOW };
    
    if (now > record.resetTime) {
        record.count = 1;
        record.resetTime = now + RATE_LIMIT_WINDOW;
    } else {
        record.count++;
    }
    
    rateLimitMap.set(key, record);
    
    if (record.count > RATE_LIMIT_MAX) {
        return res.status(429).json({ error: '请求过于频繁，请稍后再试' });
    }
    
    next();
}

function filterSensitiveInfo(content) {
    if (!FILTER_SENSITIVE) return content;
    if (!content || typeof content !== 'string') return content;
    let filtered = content;
    for (const pattern of SENSITIVE_PATTERNS) {
        filtered = filtered.replace(pattern, '[FILTERED]');
    }
    return filtered;
}

app.use((req, res, next) => {
    res.setHeader('X-Content-Type-Options', 'nosniff');
    res.setHeader('X-XSS-Protection', '1; mode=block');
    res.setHeader('Strict-Transport-Security', 'max-age=31536000; includeSubDomains');
    next();
});

app.use(rateLimit);

app.use(express.static(path.join(__dirname, '../ui')));

app.use((req, res, next) => {
    if (req.path.startsWith('/api/')) {
        if (req.path === '/api/auth/login' || req.path === '/api/auth/status') {
            return next();
        }
        if (!validateToken(req)) {
            addAuditLog('auth_failed', { path: req.path }, req);
            return res.status(401).json({ error: '需要认证' });
        }
    }
    next();
});

const LOG_DIRS = [
    '/vol1/@appdata',
    '/vol1/@appconf',
    '/vol1/@apphome',
    '/vol1/@apptemp',
    '/vol1/@appshare',
    '/var/log/apps'
];

let cachedInstalledApps = null;
let installedAppsCacheTime = 0;
const INSTALLED_APPS_CACHE_TTL = 60000;

async function getInstalledApps() {
    const now = Date.now();
    if (cachedInstalledApps && (now - installedAppsCacheTime) < INSTALLED_APPS_CACHE_TTL) {
        return cachedInstalledApps;
    }
    
    const apps = new Set();
    
    try {
        const { stdout } = await execCommand('appcenter-cli list 2>/dev/null', { ignoreError: true });
        if (stdout && stdout.trim()) {
            const lines = stdout.trim().split('\n');
            for (const line of lines) {
                if (line.includes('│') && !line.includes('APP NAME') && !line.includes('────')) {
                    const parts = line.split('│');
                    if (parts.length >= 2) {
                        const appName = parts[1].trim();
                        if (appName && appName.length > 0) {
                            apps.add(appName);
                        }
                    }
                }
            }
        }
    } catch (e) {
        console.error('[LogManager] getInstalledApps error:', e.message);
    }
    
    try {
        const packagesDir = '/var/packages';
        if (fs.existsSync(packagesDir)) {
            const entries = fs.readdirSync(packagesDir, { withFileTypes: true });
            for (const entry of entries) {
                if (entry.isDirectory()) {
                    apps.add(entry.name);
                }
            }
        }
    } catch (e) {}
    
    cachedInstalledApps = apps;
    installedAppsCacheTime = now;
    return apps;
}

function extractAppNameFromPath(logPath) {
    const parts = logPath.split('/');
    for (let i = 0; i < parts.length; i++) {
        if (parts[i].startsWith('@')) {
            if (parts[i + 1]) {
                return parts[i + 1];
            }
        }
    }
    
    if (logPath.startsWith('/var/log/apps/')) {
        const rest = logPath.slice('/var/log/apps/'.length);
        let appName = rest.split('/')[0];
        if (appName) {
            appName = appName.replace(/\.log(-\d{8})?(\.\d+)?\.(gz|bz2|xz|zip|tar(\.gz|\.bz2|\.xz)?|7z|rar)$/i, '');
            appName = appName.replace(/\.log$/i, '');
            return appName || null;
        }
    }
    
    const appdataMatch = logPath.match(/\/vol\d+\/@appdata\/([^\/]+)/);
    if (appdataMatch) {
        return appdataMatch[1];
    }
    
    const appshareMatch = logPath.match(/\/vol\d+\/@appshare\/([^\/]+)/);
    if (appshareMatch) {
        return appshareMatch[1];
    }
    
    return null;
}

function safePath(inputPath) {
    if (!inputPath || typeof inputPath !== 'string') return null;
    if (inputPath.length > 4096) return null;
    const normalized = path.normalize(inputPath);
    if (normalized.includes('..') || normalized.includes('\0')) return null;
    if (!normalized.startsWith('/')) return null;
    return normalized;
}

function isAllowedPath(inputPath) {
    if (!inputPath) return false;
    const normalized = safePath(inputPath);
    if (!normalized) return false;
    
    for (const allowedDir of LOG_DIRS) {
        if (normalized === allowedDir || normalized.startsWith(allowedDir + '/')) {
            return true;
        }
    }
    return false;
}

function escapeShellArg(arg) {
    if (typeof arg !== 'string') return '';
    return arg.replace(/'/g, "'\\''");
}

function isValidSize(size) {
    if (!size || typeof size !== 'string') return false;
    return /^[0-9]+[KMGT]?$/i.test(size);
}

function isValidNumber(num, min, max) {
    const n = parseInt(num, 10);
    return !isNaN(n) && n >= min && n <= max;
}

function isValidPath(p) {
    if (!p || typeof p !== 'string') return false;
    return /^[\w\-\.\/]+$/.test(p);
}

function isValidContainerName(name) {
    if (!name || typeof name !== 'string') return false;
    if (name.length > 128) return false;
    return /^[a-zA-Z0-9][a-zA-Z0-9_.-]*$/.test(name);
}

function isValidRelativePath(p) {
    if (!p || typeof p !== 'string') return false;
    if (p.includes('..') || p.includes('\0')) return false;
    if (p.startsWith('/')) return false;
    return true;
}

function execCommand(cmd, options = {}) {
    return new Promise((resolve, reject) => {
        exec(cmd, { 
            timeout: 30000, 
            maxBuffer: 10 * 1024 * 1024,
            ...options 
        }, (error, stdout, stderr) => {
            if (error && !options.ignoreError) {
                reject({ error, stderr: stderr.toString(), stdout: stdout.toString() });
            } else {
                resolve({ stdout: stdout.toString(), stderr: stderr.toString() });
            }
        });
    });
}

app.get('/api/health', (req, res) => {
    res.json({ status: 'ok', version: '1.0.0' });
});

app.get('/api/settings/filter', (req, res) => {
    res.json({ enabled: FILTER_SENSITIVE });
});

app.post('/api/settings/filter', (req, res) => {
    const { enabled } = req.body;
    if (typeof enabled === 'boolean') {
        process.env.FILTER_SENSITIVE = enabled ? 'true' : 'false';
        FILTER_SENSITIVE = enabled;
        res.json({ success: true, enabled: FILTER_SENSITIVE });
    } else {
        res.status(400).json({ error: '无效的参数' });
    }
});

app.get('/api/dirs', (req, res) => {
    const dirs = LOG_DIRS.map(dir => {
        try {
            const exists = fs.existsSync(dir);
            let logCount = 0;
            let archiveCount = 0;
            let totalSize = 0;
            
            if (exists) {
                try {
                    const result = execSync(
                        `find '${escapeShellArg(dir)}' -type f \\( -name "*.log" -o -name "*.log.*" \\) 2>/dev/null | wc -l`,
                        { encoding: 'utf8', timeout: 10000 }
                    );
                    logCount = parseInt(result.trim()) || 0;
                } catch (e) {}
                
                try {
                    const result = execSync(
                        `find '${escapeShellArg(dir)}' -type f \\( -name "*.gz" -o -name "*.bz2" -o -name "*.xz" -o -name "*.zip" -o -name "*.tar" -o -name "*.tar.gz" -o -name "*.tar.bz2" -o -name "*.tar.xz" -o -name "*.7z" -o -name "*.rar" \\) 2>/dev/null | wc -l`,
                        { encoding: 'utf8', timeout: 10000 }
                    );
                    archiveCount = parseInt(result.trim()) || 0;
                } catch (e) {}
                
                try {
                    const result = execSync(
                        `find '${escapeShellArg(dir)}' -type f \\( -name "*.log" -o -name "*.log.*" -o -name "*.gz" -o -name "*.bz2" -o -name "*.xz" -o -name "*.zip" -o -name "*.tar" -o -name "*.tar.gz" -o -name "*.tar.bz2" -o -name "*.tar.xz" -o -name "*.7z" -o -name "*.rar" \\) -printf '%s\\n' 2>/dev/null | awk '{sum+=$1} END {print sum}'`,
                        { encoding: 'utf8', timeout: 15000 }
                    );
                    totalSize = parseInt(result.trim()) || 0;
                } catch (e) {}
            }
            
            return {
                path: dir,
                exists,
                logCount,
                archiveCount,
                totalSize: formatBytes(totalSize)
            };
        } catch (e) {
            return { path: dir, exists: false, error: e.message };
        }
    });
    
    res.json({ dirs });
});

app.get('/api/logs/list', async (req, res) => {
    const { dir, limit = 100 } = req.query;
    const limitNum = isValidNumber(limit, 1, 500) ? parseInt(limit) : 100;
    
    let searchDirs = LOG_DIRS;
    if (dir) {
        if (!isAllowedPath(dir)) {
            return res.status(403).json({ error: '不允许访问此目录' });
        }
        searchDirs = [safePath(dir)];
    }
    
    const results = [];
    const installedApps = await getInstalledApps();
    
    for (const searchDir of searchDirs) {
        try {
            if (!fs.existsSync(searchDir)) continue;
            
            const cmd = `find '${escapeShellArg(searchDir)}' -type f \\( -name "*.log" -o -name "*.log.*" -o -name "*log*.txt" \\) 2>/dev/null | head -${limitNum}`;
            const { stdout } = await execCommand(cmd, { ignoreError: true });
            
            const files = stdout.trim().split('\n').filter(f => f);
            for (const file of files) {
                try {
                    const stats = fs.statSync(file);
                    const appName = extractAppNameFromPath(file);
                    const canDelete = appName ? !installedApps.has(appName) : false;
                    results.push({
                        path: file,
                        size: stats.size,
                        sizeFormatted: formatBytes(stats.size),
                        modified: stats.mtime,
                        appName: appName,
                        canDelete: canDelete
                    });
                } catch (e) {}
            }
        } catch (e) {}
    }
    
    res.json({ logs: results.slice(0, limitNum), total: results.length });
});

app.get('/api/logs/large', async (req, res) => {
    const { threshold = '10M', limit = 50 } = req.query;
    
    if (!isValidSize(threshold)) {
        return res.status(400).json({ error: '无效的大小阈值' });
    }
    
    const limitNum = isValidNumber(limit, 1, 200) ? parseInt(limit) : 50;
    const results = [];
    
    for (const dir of LOG_DIRS) {
        try {
            if (!fs.existsSync(dir)) continue;
            
            const cmd = `find '${escapeShellArg(dir)}' -type f \\( -name "*.log" -o -name "*.log.*" \\) -size +${threshold} 2>/dev/null | head -${limitNum}`;
            const { stdout } = await execCommand(cmd, { ignoreError: true });
            
            const files = stdout.trim().split('\n').filter(f => f);
            for (const file of files) {
                try {
                    const stats = fs.statSync(file);
                    results.push({
                        path: file,
                        size: stats.size,
                        sizeFormatted: formatBytes(stats.size),
                        modified: stats.mtime
                    });
                } catch (e) {}
            }
        } catch (e) {}
    }
    
    results.sort((a, b) => b.size - a.size);
    res.json({ logs: results.slice(0, limitNum) });
});

app.get('/api/logs/stats', async (req, res) => {
    let totalLogs = 0;
    let totalArchives = 0;
    let totalSize = 0;
    let largeFiles = 0;
    
    for (const dir of LOG_DIRS) {
        try {
            if (!fs.existsSync(dir)) continue;
            
            const countCmd = `find '${escapeShellArg(dir)}' -type f \\( -name "*.log" -o -name "*.log.*" \\) 2>/dev/null | wc -l`;
            const countResult = await execCommand(countCmd, { ignoreError: true });
            totalLogs += parseInt(countResult.stdout.trim()) || 0;
            
            const archiveCmd = `find '${escapeShellArg(dir)}' -type f \\( -name "*.gz" -o -name "*.bz2" -o -name "*.xz" -o -name "*.zip" -o -name "*.tar" -o -name "*.tar.gz" -o -name "*.tar.bz2" -o -name "*.tar.xz" -o -name "*.7z" -o -name "*.rar" \\) 2>/dev/null | wc -l`;
            const archiveResult = await execCommand(archiveCmd, { ignoreError: true });
            totalArchives += parseInt(archiveResult.stdout.trim()) || 0;
            
            const largeCmd = `find '${escapeShellArg(dir)}' -type f \\( -name "*.log" -o -name "*.log.*" \\) -size +10M 2>/dev/null | wc -l`;
            const largeResult = await execCommand(largeCmd, { ignoreError: true });
            largeFiles += parseInt(largeResult.stdout.trim()) || 0;
            
            const sizeCmd = `find '${escapeShellArg(dir)}' -type f \\( -name "*.log" -o -name "*.log.*" \\) -exec du -b {} + 2>/dev/null | awk '{sum+=$1} END {print sum}'`;
            const sizeResult = await execCommand(sizeCmd, { ignoreError: true });
            totalSize += parseInt(sizeResult.stdout.trim()) || 0;
        } catch (e) {}
    }
    
    res.json({
        totalLogs,
        totalArchives,
        largeFiles,
        totalSize,
        totalSizeFormatted: formatBytes(totalSize)
    });
});

app.get('/api/log/content', async (req, res) => {
    const { path: logPath } = req.query;
    
    if (!logPath) {
        return res.status(400).json({ error: '缺少文件路径' });
    }
    
    if (!isAllowedPath(logPath)) {
        return res.status(403).json({ error: '不允许访问此文件' });
    }
    
    const safeLogPath = safePath(logPath);
    
    if (!fs.existsSync(safeLogPath)) {
        return res.status(404).json({ error: '文件不存在' });
    }
    
    try {
        const stats = fs.statSync(safeLogPath);
        
        if (stats.size > 100 * 1024 * 1024) {
            return res.status(400).json({ 
                error: '文件过大，请使用命令行查看',
                size: stats.size,
                sizeFormatted: formatBytes(stats.size)
            });
        }
        
        const content = fs.readFileSync(safeLogPath, 'utf8');
        
        res.json({ 
            content: filterSensitiveInfo(content), 
            totalLines: content.split('\n').length,
            size: stats.size,
            sizeFormatted: formatBytes(stats.size)
        });
    } catch (e) {
        res.status(500).json({ error: e.message });
    }
});

app.post('/api/log/truncate', async (req, res) => {
    const { path: logPath } = req.body;
    
    if (!logPath) {
        return res.status(400).json({ error: '缺少文件路径' });
    }
    
    if (!isAllowedPath(logPath)) {
        return res.status(403).json({ error: '不允许访问此文件' });
    }
    
    const safeLogPath = safePath(logPath);
    
    if (!fs.existsSync(safeLogPath)) {
        return res.status(404).json({ error: '文件不存在' });
    }
    
    try {
        fs.writeFileSync(safeLogPath, '');
        addAuditLog('log_truncate', { path: safeLogPath }, req);
        res.json({ success: true, message: '日志已清空' });
    } catch (e) {
        res.status(500).json({ error: e.message });
    }
});

app.post('/api/log/delete', async (req, res) => {
    const { path: logPath } = req.body;
    
    if (!logPath) {
        return res.status(400).json({ error: '缺少文件路径' });
    }
    
    if (!isAllowedPath(logPath)) {
        return res.status(403).json({ error: '不允许访问此文件' });
    }
    
    const safeLogPath = safePath(logPath);
    
    if (!fs.existsSync(safeLogPath)) {
        return res.status(404).json({ error: '文件不存在' });
    }
    
    try {
        const stats = fs.statSync(safeLogPath);
        if (stats.isDirectory()) {
            return res.status(400).json({ error: '不能删除目录' });
        }
        
        fs.unlinkSync(safeLogPath);
        addAuditLog('log_delete', { path: safeLogPath }, req);
        res.json({ success: true, message: '日志文件已删除' });
    } catch (e) {
        res.status(500).json({ error: e.message });
    }
});

app.post('/api/logs/clean', async (req, res) => {
    const { threshold, days, action = 'truncate' } = req.body;
    
    if (action !== 'truncate' && action !== 'delete') {
        return res.status(400).json({ error: '无效的操作类型' });
    }
    
    if (!threshold && !days) {
        return res.status(400).json({ error: '请指定清理条件' });
    }
    
    if (threshold && !isValidSize(threshold)) {
        return res.status(400).json({ error: '无效的大小阈值' });
    }
    
    if (days && !isValidNumber(days, 1, 365)) {
        return res.status(400).json({ error: '无效的天数' });
    }
    
    const results = { cleaned: 0, errors: [] };
    
    for (const dir of LOG_DIRS) {
        try {
            if (!fs.existsSync(dir)) continue;
            
            let findCmd;
            if (days) {
                findCmd = `find '${escapeShellArg(dir)}' -type f \\( -name "*.gz" -o -name "*.bz2" -o -name "*.xz" -o -name "*.zip" -o -name "*.tar" -o -name "*.tar.gz" -o -name "*.tar.bz2" -o -name "*.tar.xz" -o -name "*.7z" -o -name "*.rar" \\) -mtime +${parseInt(days)} 2>/dev/null`;
            } else {
                findCmd = `find '${escapeShellArg(dir)}' -type f -name "*.log" -size +${threshold} 2>/dev/null`;
            }
            
            const { stdout } = await execCommand(findCmd, { ignoreError: true });
            const files = stdout.trim().split('\n').filter(f => f);
            
            for (const file of files) {
                try {
                    if (action === 'delete') {
                        fs.unlinkSync(file);
                    } else {
                        fs.writeFileSync(file, '');
                    }
                    results.cleaned++;
                } catch (e) {
                    results.errors.push({ file, error: e.message });
                }
            }
        } catch (e) {
            results.errors.push({ dir, error: e.message });
        }
    }
    
    addAuditLog('logs_clean', { action, threshold, days, cleaned: results.cleaned }, req);
    res.json(results);
});

app.post('/api/logs/compress', async (req, res) => {
    const { threshold = '10M' } = req.body;
    
    if (!isValidSize(threshold)) {
        return res.status(400).json({ error: '无效的大小阈值' });
    }
    
    const results = { compressed: 0, errors: [] };
    
    for (const dir of LOG_DIRS) {
        try {
            if (!fs.existsSync(dir)) continue;
            
            const findCmd = `find '${escapeShellArg(dir)}' -type f -name "*.log" -size +${threshold} ! -name "*.gz" 2>/dev/null`;
            const { stdout } = await execCommand(findCmd, { ignoreError: true });
            const files = stdout.trim().split('\n').filter(f => f);
            
            for (const file of files) {
                try {
                    await execCommand(`gzip '${escapeShellArg(file)}'`, { ignoreError: true });
                    results.compressed++;
                } catch (e) {
                    results.errors.push({ file, error: e.message });
                }
            }
        } catch (e) {
            results.errors.push({ dir, error: e.message });
        }
    }
    
    res.json(results);
});

app.post('/api/logs/backup', async (req, res) => {
    const timestamp = new Date().toISOString().replace(/[:.]/g, '-').slice(0, 19).replace(/[^0-9T\-]/g, '');
    const backupDir = `/vol1/@appshare/log-backup/backup-${timestamp}`;
    
    const results = { backupPath: backupDir, files: 0, errors: [] };
    
    try {
        await execCommand(`mkdir -p '${escapeShellArg(backupDir)}'`);
        
        for (const dir of LOG_DIRS) {
            if (!fs.existsSync(dir)) continue;
            
            const dirName = path.basename(dir);
            await execCommand(`mkdir -p '${escapeShellArg(backupDir)}/${escapeShellArg(dirName)}'`);
            
            try {
                const findCmd = `find '${escapeShellArg(dir)}' -type f \\( -name "*.log" -o -name "*.log.*" \\) 2>/dev/null | head -100`;
                const { stdout } = await execCommand(findCmd, { ignoreError: true });
                const files = stdout.trim().split('\n').filter(f => f);
                
                for (const file of files) {
                    try {
                        const relativePath = path.relative(dir, file);
                        if (!isValidRelativePath(relativePath)) {
                            results.errors.push({ file, error: '无效的相对路径' });
                            continue;
                        }
                        const destDir = path.dirname(`${backupDir}/${dirName}/${relativePath}`);
                        await execCommand(`mkdir -p '${escapeShellArg(destDir)}'`);
                        await execCommand(`cp '${escapeShellArg(file)}' '${escapeShellArg(destDir)}/'`);
                        results.files++;
                    } catch (e) {
                        results.errors.push({ file, error: e.message });
                    }
                }
            } catch (e) {
                results.errors.push({ dir, error: e.message });
            }
        }
        
        await execCommand(`tar -czf '${escapeShellArg(backupDir)}.tar.gz' -C '${escapeShellArg(path.dirname(backupDir))}' '${escapeShellArg(path.basename(backupDir))}' && rm -rf '${escapeShellArg(backupDir)}'`);
        results.backupPath = `${backupDir}.tar.gz`;
        
        const stats = fs.statSync(results.backupPath);
        results.backupSize = formatBytes(stats.size);
    } catch (e) {
        results.errors.push({ error: e.message });
    }
    
    res.json(results);
});

app.get('/api/archives/list', async (req, res) => {
    const { limit = 50 } = req.query;
    const limitNum = isValidNumber(limit, 1, 200) ? parseInt(limit) : 50;
    
    const results = [];
    
    for (const dir of LOG_DIRS) {
        try {
            if (!fs.existsSync(dir)) continue;
            
            const cmd = `find '${escapeShellArg(dir)}' -type f \\( -name "*.gz" -o -name "*.bz2" -o -name "*.xz" -o -name "*.zip" -o -name "*.tar" -o -name "*.tar.gz" -o -name "*.tar.bz2" -o -name "*.tar.xz" -o -name "*.7z" -o -name "*.rar" \\) 2>/dev/null | head -${limitNum}`;
            const { stdout } = await execCommand(cmd, { ignoreError: true });
            
            const files = stdout.trim().split('\n').filter(f => f);
            for (const file of files) {
                try {
                    const stats = fs.statSync(file);
                    results.push({
                        path: file,
                        size: stats.size,
                        sizeFormatted: formatBytes(stats.size),
                        modified: stats.mtime,
                        type: path.extname(file)
                    });
                } catch (e) {}
            }
        } catch (e) {}
    }
    
    res.json({ archives: results.slice(0, limitNum), total: results.length });
});

app.get('/api/archive/content', async (req, res) => {
    const { path: archivePath, lines = 50 } = req.query;
    
    if (!archivePath) {
        return res.status(400).json({ error: '缺少文件路径' });
    }
    
    if (!isAllowedPath(archivePath)) {
        return res.status(403).json({ error: '不允许访问此文件' });
    }
    
    const safeArchivePath = safePath(archivePath);
    
    const linesNum = isValidNumber(lines, 1, 500) ? parseInt(lines) : 50;
    
    if (!fs.existsSync(safeArchivePath)) {
        return res.status(404).json({ error: '文件不存在' });
    }
    
    try {
        const filePath = safeArchivePath.toLowerCase();
        let cmd;
        
        if (filePath.endsWith('.tar.gz') || filePath.endsWith('.tgz')) {
            cmd = `tar -xzf '${escapeShellArg(safeArchivePath)}' -O 2>/dev/null | head -n ${linesNum}`;
        } else if (filePath.endsWith('.tar.bz2') || filePath.endsWith('.tbz2')) {
            cmd = `tar -xjf '${escapeShellArg(safeArchivePath)}' -O 2>/dev/null | head -n ${linesNum}`;
        } else if (filePath.endsWith('.tar.xz') || filePath.endsWith('.txz')) {
            cmd = `tar -xJf '${escapeShellArg(safeArchivePath)}' -O 2>/dev/null | head -n ${linesNum}`;
        } else if (filePath.endsWith('.tar')) {
            cmd = `tar -xf '${escapeShellArg(safeArchivePath)}' -O 2>/dev/null | head -n ${linesNum}`;
        } else if (filePath.endsWith('.gz')) {
            cmd = `zcat '${escapeShellArg(safeArchivePath)}' | head -n ${linesNum}`;
        } else if (filePath.endsWith('.bz2')) {
            cmd = `bzcat '${escapeShellArg(safeArchivePath)}' | head -n ${linesNum}`;
        } else if (filePath.endsWith('.xz')) {
            cmd = `xzcat '${escapeShellArg(safeArchivePath)}' | head -n ${linesNum}`;
        } else if (filePath.endsWith('.zip')) {
            cmd = `unzip -p '${escapeShellArg(safeArchivePath)}' 2>/dev/null | head -n ${linesNum}`;
        } else if (filePath.endsWith('.7z')) {
            cmd = `7z x -so '${escapeShellArg(safeArchivePath)}' 2>/dev/null | head -n ${linesNum}`;
        } else if (filePath.endsWith('.rar')) {
            cmd = `unrar p -inul '${escapeShellArg(safeArchivePath)}' 2>/dev/null | head -n ${linesNum}`;
        } else {
            return res.status(400).json({ error: '不支持的压缩格式' });
        }
        
        const { stdout } = await execCommand(cmd, { ignoreError: true });
        res.json({ content: filterSensitiveInfo(stdout), truncated: stdout.split('\n').length >= linesNum });
    } catch (e) {
        res.status(500).json({ error: e.message });
    }
});

app.post('/api/archives/delete', async (req, res) => {
    const { path: archivePath } = req.body;
    
    if (!archivePath) {
        return res.status(400).json({ error: '缺少文件路径' });
    }
    
    if (!isAllowedPath(archivePath)) {
        return res.status(403).json({ error: '不允许访问此文件' });
    }
    
    const safeArchivePath = safePath(archivePath);
    
    if (!fs.existsSync(safeArchivePath)) {
        return res.status(404).json({ error: '文件不存在' });
    }
    
    try {
        fs.unlinkSync(safeArchivePath);
        addAuditLog('archive_delete', { path: safeArchivePath }, req);
        res.json({ success: true, message: '归档文件已删除' });
    } catch (e) {
        res.status(500).json({ error: e.message });
    }
});

app.get('/api/audit/log', (req, res) => {
    cleanOldAuditLogs();
    res.json({ logs: getAuditLogs(100) });
});

app.get('/api/docker/containers', async (req, res) => {
    try {
        const { stdout } = await execCommand('docker ps --format "{{.Names}}\\t{{.Status}}\\t{{.Image}}"', { ignoreError: true });
        const containers = stdout.trim().split('\n').filter(line => line).map(line => {
            const parts = line.split('\t');
            const name = parts[0] || '';
            if (!isValidContainerName(name)) return null;
            return {
                name,
                status: parts[1] || '',
                image: parts[2] || ''
            };
        }).filter(c => c);
        res.json({ containers });
    } catch (e) {
        res.json({ containers: [], error: 'Docker未安装或未运行' });
    }
});

app.get('/api/docker/logs', async (req, res) => {
    const { container, lines = 100 } = req.query;
    
    if (!container) {
        return res.status(400).json({ error: '缺少容器名称' });
    }
    
    if (!isValidContainerName(container)) {
        return res.status(400).json({ error: '无效的容器名称' });
    }
    
    const linesNum = isValidNumber(lines, 1, 1000) ? parseInt(lines) : 100;
    
    try {
        const { stdout } = await execCommand(`docker logs '${escapeShellArg(container)}' --tail ${linesNum} 2>&1`, { ignoreError: true });
        res.json({ logs: filterSensitiveInfo(stdout) });
    } catch (e) {
        res.status(500).json({ error: e.message });
    }
});

function formatBytes(bytes) {
    if (bytes === 0) return '0B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + sizes[i];
}

app.listen(PORT, '0.0.0.0', () => {
    console.log(`飞牛日志管理服务已启动，端口: ${PORT}`);
});
