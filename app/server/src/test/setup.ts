// Jest 测试环境初始化

// Mock Logger for tests
jest.mock('../utils/logger', () => ({
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
    })
}));

// Mock config for tests
jest.mock('../utils/config', () => ({
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
        login: {
            maxAttempts: 5,
            lockoutTime: 30 * 60 * 1000
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
        logDirs: [
            '/tmp/test-logmanager/logs'
        ],
        sensitivePatterns: [],
        passwordFile: '/tmp/test-logmanager/.password',
        auditLogFile: '/tmp/test-logmanager/audit.log',
        initTimestampFile: '/tmp/test-logmanager/.init-time'
    }
}));

// 全局测试超时
jest.setTimeout(30000);
