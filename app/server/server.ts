import express from 'express';
import path from 'path';
import morgan from 'morgan';
import cookieParser from 'cookie-parser';
import Logger from './utils/logger';

// 设置时区为东八区（中国标准时间）
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

const app = express();

const getLogUrl = (req: express.Request) => {
    if (!config.logging.redactQuery) return req.originalUrl;
    return req.path;
};

app.set('trust proxy', true);

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
app.use(express.urlencoded({ extended: true, limit: '10mb' }));
app.use(requestSizeLimit('10mb'));
app.use(validateContentType);
app.use(sanitizeInput);
app.use(rateLimit);

app.use(express.static(path.join(__dirname, '../ui')));

app.use('/api/auth', authRoutes);
app.use('/api', logRoutes);
app.use('/api', dockerRoutes);
app.use('/api/update', updateRoutes);
app.use('/api/notifications', notificationRoutes);
app.use('/api/eventlogger', eventLoggerRoutes);

app.use(notFoundHandler);
app.use(errorHandler);

const server = app.listen(config.port, '0.0.0.0', async () => {
    logger.info({ port: config.port }, '飞牛日志管理服务已启动');
    auditService.addAuditLog('SERVER_START', { port: config.port }).catch(() => {});

    // 初始化日志监控服务
    try {
        await logMonitor.init();
    } catch (err) {
        logger.error({ err }, '日志监控服务初始化失败');
    }

    // 初始化事件日志监控服务
    try {
        await eventLoggerService.init(config.eventLogger);
    } catch (err) {
        logger.error({ err }, '事件日志监控服务初始化失败');
    }
});

server.setTimeout(120000);
server.keepAliveTimeout = 65000;
server.headersTimeout = 66000;

process.on('SIGTERM', () => {
    logger.info('收到SIGTERM信号，正在关闭...');
    auditService.addAuditLog('SERVER_SHUTDOWN', { reason: 'SIGTERM' }).catch(() => {});
    server.close(() => {
        logger.info('服务已关闭');
        process.exit(0);
    });
});

process.on('SIGINT', () => {
    logger.info('收到SIGINT信号，正在关闭...');
    auditService.addAuditLog('SERVER_SHUTDOWN', { reason: 'SIGINT' }).catch(() => {});
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
