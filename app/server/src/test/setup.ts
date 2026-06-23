// Jest 测试环境初始化
import { jest } from '@jest/globals';

// Mock Logger for tests
jest.mock('../../utils/logger', () => ({
    __esModule: true,
    default: {
        info: jest.fn(),
        warn: jest.fn(),
        error: jest.fn(),
        debug: jest.fn(),
        trace: jest.fn(),
        fatal: jest.fn(),
        child: jest.fn().mockReturnThis()
    },
    createLogger: jest.fn().mockReturnValue({
        info: jest.fn(),
        warn: jest.fn(),
        error: jest.fn(),
        debug: jest.fn(),
        trace: jest.fn(),
        fatal: jest.fn(),
        child: jest.fn().mockReturnThis()
    }),
    getLogLevel: jest.fn(() => 'info'),
    isDevelopmentMode: jest.fn(() => false)
}));

// Mock config for tests
jest.mock('../../utils/config', () => ({
    __esModule: true,
    default: {
        port: 8090,
        dataDir: '/tmp/test-logmanager',
        sessionExpiry: 24 * 60 * 60 * 1000,
        rateLimit: {
            windowMs: 60000,
            maxRequests: 100
        },
        auth: {
            allowQueryToken: false,
            queryTokenPaths: ['/api/auth/status', '/api/auth/csrf-token']
        },
        audit: {
            maxLogs: 1000
        },
        csrf: {
            expiry: 2 * 60 * 60 * 1000
        },
        logging: {
            redactQuery: true
        },
        docker: {
            listTimeoutMs: 10000,
            logsTimeoutMs: 120000,
            maxLogLines: 2000,
            maxOutputBytes: 2 * 1024 * 1024
        },
        logFile: {
            maxPreviewLines: 1000,
            maxPreviewBytes: 5242880
        },
        archive: {
            maxPreviewLines: 100,
            maxArchiveBytes: 1073741824
        },
        backup: {
            baseDir: '/tmp/test-logmanager/backup',
            maxFiles: 10,
            maxFileSizeBytes: 104857600,
            maxTotalBytes: 1073741824
        },
        eventLogger: {
            dbPath: '/tmp/test-logmanager/events.db',
            enabled: true,
            checkInterval: 5000,
            eventTypes: ['error', 'warning'],
            minSeverity: 'info',
            notificationChannels: []
        },
        update: {
            checkCacheMs: 3600000,
            downloadTimeoutMs: 30000,
            maxDownloadBytes: 52428800,
            maxAssetBytes: 104857600,
            maxRedirects: 5,
            allowedHosts: ['api.github.com'],
            allowedUpdateDirs: ['/tmp/test-logmanager/update']
        },
        logDirs: [
            '/tmp/test-logmanager/logs'
        ],
        sensitivePatterns: [],
        auditLogFile: '/tmp/test-logmanager/audit.log',
        initTimestampFile: '/tmp/test-logmanager/.init-time'
    }
}));

// 全局测试超时
jest.setTimeout(30000);
