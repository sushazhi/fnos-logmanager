import express from 'express';
import path from 'path';
import morgan from 'morgan';
import cookieParser from 'cookie-parser';
import http from 'http';
import stream from 'stream';
import Logger from './utils/logger';

process.env.TZ = 'Asia/Shanghai';

const logger = Logger.child({ module: 'Server' });

import config from './utils/config';
import { notFoundHandler, errorHandler } from './middleware/errorHandler';
import { 
    securityHeaders, 
    validateContentType, 
    requestSizeLimit, 
    sanitizeInput,
    validateEnv,
    checkDependencies 
} from './middleware/security';
import { rateLimit } from './middleware/rateLimit';
import * as auditService from './services/audit';

import authRoutes from './routes/auth';
import logRoutes from './routes/logs';
import dockerRoutes from './routes/docker';
import updateRoutes from './routes/update';
import notificationRoutes from './routes/notifications';
import eventLoggerRoutes from './routes/eventLogger';
import * as logMonitor from './services/logMonitor';
import * as eventLoggerService from './services/eventLogger';
import { initLogStream, closeLogStream } from './services/logStream';
import { initDockerLogStream, closeDockerLogStream } from './services/dockerLogStream';
import { initNotifyWebSocket, closeNotifyWebSocket } from './services/notifyWebSocket';
import * as autoCleanService from './services/autoClean';

const envValidation = validateEnv();
if (!envValidation.valid) {
    logger.error({ errors: envValidation.errors }, '环境变量验证失败');
    process.exit(1);
}

const depsCheck = checkDependencies();
if (!depsCheck.valid) {
    logger.error({ missing: depsCheck.missing }, '缺少依赖');
    process.exit(1);
}

const GATEWAY_SOCKET = process.env.GATEWAY_SOCKET || '';
const SOCKET_PATH = GATEWAY_SOCKET ? path.join(GATEWAY_SOCKET) : '';
const GATEWAY_PREFIX = '/app/logmanager';

const app = express();

const getLogUrl = (req: express.Request) => {
    if (!config.logging.redactQuery) return req.originalUrl;
    return req.path;
};

app.set('trust proxy', process.env.TRUSTED_PROXIES ? process.env.TRUSTED_PROXIES.split(',').map(ip => ip.trim()) : ['127.0.0.1', '::1']);

app.use(morgan((tokens, req, res) => {
    const status = tokens.status(req, res);
    const length = tokens.res(req, res, 'content-length') || '-';
    const referrer = tokens.referrer(req, res) || '-';
    const userAgent = tokens['user-agent'](req, res) || '-';
    const url = getLogUrl(req);
    return `${tokens['remote-addr'](req, res)} - ${tokens['remote-user'](req, res) || '-'} ${tokens.date(req, res, 'clf')} "${tokens.method(req, res)} ${url} HTTP/${tokens['http-version'](req, res)}" ${status} ${length} "${referrer}" "${userAgent}"`;
}));
app.use(securityHeaders);
app.use(cookieParser());
app.use(express.json({ limit: '10mb' }));
app.use(express.urlencoded({ extended: false, limit: '10mb' }));
app.use(requestSizeLimit('10mb'));
app.use(validateContentType);
app.use(sanitizeInput);
app.use(rateLimit);

app.use(express.static(path.join(__dirname, '../ui')));

app.use((req: express.Request, res: express.Response, next: express.NextFunction) => {
    if (req.method === 'GET' && !req.path.startsWith('/api') && !req.path.includes('.')) {
        res.sendFile(path.join(__dirname, '../ui/index.html'), (err) => {
            if (err) next(err);
        });
        return;
    }
    next();
});

app.use('/api/auth', authRoutes);
app.use('/api', logRoutes);
app.use('/api', dockerRoutes);
app.use('/api/update', updateRoutes);
app.use('/api/notifications', notificationRoutes);
app.use('/api/eventlogger', eventLoggerRoutes);

app.use(notFoundHandler);
app.use(errorHandler);

const server = http.createServer((req: http.IncomingMessage, res: http.ServerResponse) => {
    const originalUrl = req.url || '/';
    if (GATEWAY_SOCKET) {
        if (req.url === GATEWAY_PREFIX) {
            res.writeHead(302, { 'Location': GATEWAY_PREFIX + '/' });
            res.end();
            return;
        }
        if (req.url && req.url.startsWith(GATEWAY_PREFIX + '/')) {
            req.url = req.url.slice(GATEWAY_PREFIX.length);
        } else if (req.url && req.url.startsWith(GATEWAY_PREFIX)) {
            req.url = req.url.slice(GATEWAY_PREFIX.length) || '/';
        }
    }
    app(req, res);
});

server.on('upgrade', (req: http.IncomingMessage, socket: stream.Duplex, head: Buffer) => {
    if (GATEWAY_SOCKET && req.url) {
        if (req.url.startsWith(GATEWAY_PREFIX + '/')) {
            req.url = req.url.slice(GATEWAY_PREFIX.length);
        } else if (req.url.startsWith(GATEWAY_PREFIX)) {
            req.url = req.url.slice(GATEWAY_PREFIX.length) || '/';
        }
    }
});

async function startServer() {
    initLogStream(server);
    initDockerLogStream(server);
    initNotifyWebSocket(server);

    try {
        await logMonitor.init();
    } catch (err) {
        logger.error({ err }, '日志监控服务初始化失败');
    }

    try {
        await eventLoggerService.init(config.eventLogger);
    } catch (err) {
        logger.error({ err }, '事件日志监控服务初始化失败');
    }

    try {
        await autoCleanService.init();
    } catch (err) {
        logger.error({ err }, '自动清理服务初始化失败');
    }
}

function listenServer(): void {
    if (SOCKET_PATH) {
        try {
            const fs = require('fs');
            if (fs.existsSync(SOCKET_PATH)) {
                fs.unlinkSync(SOCKET_PATH);
            }
        } catch { /* ignore */ }
        
        server.listen(SOCKET_PATH, async () => {
            try {
                const fs = require('fs');
                fs.chmodSync(SOCKET_PATH, 0o777);
            } catch { /* ignore */ }
            logger.info({ socket: SOCKET_PATH }, '飞牛日志管理服务已启动（网关模式）');
            auditService.addAuditLog('SERVER_START', { mode: 'gateway', socket: SOCKET_PATH }).catch(() => {});
            await startServer();
        });
    } else {
        server.listen(config.port, '0.0.0.0', async () => {
            logger.info({ port: config.port }, '飞牛日志管理服务已启动');
            auditService.addAuditLog('SERVER_START', { mode: 'direct', port: config.port }).catch(() => {});
            await startServer();
        });
    }
}

listenServer();

server.setTimeout(120000);
server.keepAliveTimeout = 65000;
server.headersTimeout = 66000;

process.on('SIGTERM', () => {
    logger.info('收到SIGTERM信号，正在关闭...');
    auditService.addAuditLog('SERVER_SHUTDOWN', { reason: 'SIGTERM' }).catch(() => {});
    closeLogStream();
    closeDockerLogStream();
    closeNotifyWebSocket();
    autoCleanService.shutdown().catch(() => {});
    server.close(() => {
        logger.info('服务已关闭');
        process.exit(0);
    });
});

process.on('SIGINT', () => {
    logger.info('收到SIGINT信号，正在关闭...');
    auditService.addAuditLog('SERVER_SHUTDOWN', { reason: 'SIGINT' }).catch(() => {});
    closeLogStream();
    closeDockerLogStream();
    closeNotifyWebSocket();
    autoCleanService.shutdown().catch(() => {});
    server.close(() => {
        logger.info('服务已关闭');
        process.exit(0);
    });
});

process.on('uncaughtException', (error) => {
    logger.error({ err: error }, '未捕获的异常');
    auditService.addSecurityAuditLog('UNCAUGHT_EXCEPTION', {
        error: error.message,
        stack: error.stack
    }).catch(() => {});
});

process.on('unhandledRejection', (reason) => {
    logger.error({ reason: String(reason) }, '未处理的Promise拒绝');
    auditService.addSecurityAuditLog('UNHANDLED_REJECTION', {
        reason: String(reason)
    }).catch(() => {});
});

export default app;
