import Logger, { createLogger, getLogLevel, isDevelopmentMode } from './logger';

describe('Logger', () => {
    describe('default export', () => {
        it('should be a pino logger instance', () => {
            expect(Logger).toBeDefined();
            expect(typeof Logger.info).toBe('function');
            expect(typeof Logger.error).toBe('function');
            expect(typeof Logger.warn).toBe('function');
            expect(typeof Logger.debug).toBe('function');
            expect(typeof Logger.trace).toBe('function');
            expect(typeof Logger.fatal).toBe('function');
        });

        it('should have child method', () => {
            expect(typeof Logger.child).toBe('function');
        });
    });

    describe('createLogger', () => {
        it('should return base logger when no context', () => {
            const logger = createLogger();
            expect(logger).toBeDefined();
            expect(typeof logger.info).toBe('function');
        });

        it('should return child logger with context', () => {
            const logger = createLogger({ module: 'TestModule' });
            expect(logger).toBeDefined();
            expect(typeof logger.info).toBe('function');
        });
    });

    describe('getLogLevel', () => {
        it('should return current log level', () => {
            const level = getLogLevel();
            expect(typeof level).toBe('string');
            expect(['trace', 'debug', 'info', 'warn', 'error', 'fatal']).toContain(level);
        });
    });

    describe('isDevelopmentMode', () => {
        it('should return boolean', () => {
            const isDev = isDevelopmentMode();
            expect(typeof isDev).toBe('boolean');
        });
    });

    describe('logging methods', () => {
        it('should not throw when calling info', () => {
            expect(() => Logger.info('test message')).not.toThrow();
        });

        it('should not throw when calling info with object', () => {
            expect(() => Logger.info({ key: 'value' }, 'test message')).not.toThrow();
        });

        it('should not throw when calling error', () => {
            expect(() => Logger.error('test error')).not.toThrow();
        });

        it('should not throw when calling error with Error object', () => {
            const err = new Error('test error');
            expect(() => Logger.error({ err }, 'error occurred')).not.toThrow();
        });

        it('should not throw when calling warn', () => {
            expect(() => Logger.warn('test warning')).not.toThrow();
        });

        it('should not throw when calling debug', () => {
            expect(() => Logger.debug('test debug')).not.toThrow();
        });
    });

    describe('child logger', () => {
        it('should create child logger with context', () => {
            const childLogger = Logger.child({ module: 'ChildModule' });
            expect(childLogger).toBeDefined();
            expect(typeof childLogger.info).toBe('function');
        });

        it('should not throw when child logger logs', () => {
            const childLogger = Logger.child({ module: 'ChildModule' });
            expect(() => childLogger.info('child log message')).not.toThrow();
        });
    });
});
