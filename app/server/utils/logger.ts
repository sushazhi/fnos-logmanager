import pino, { Logger as PinoLogger } from 'pino';

// 获取日志配置
const logLevel = process.env.LOG_LEVEL || 'info';
const isDevelopment = process.env.NODE_ENV !== 'production';
const isPretty = process.env.LOG_PRETTY === 'true' || isDevelopment;

// 创建基础 Logger
const baseLogger = pino({
    level: logLevel,
    timestamp: pino.stdTimeFunctions.isoTime,
    formatters: {
        level: (label) => ({ level: label }),
        bindings: (bindings) => {
            // 移除默认的 pid 和 hostname
            return { ...bindings, pid: undefined, hostname: undefined };
        }
    },
    transport: isPretty ? {
        target: 'pino-pretty',
        options: {
            colorize: true,
            translateTime: 'SYS:standard',
            ignore: 'pid,hostname',
            messageFormat: '{msg}'
        }
    } : undefined
});

/**
 * 创建带上下文的子 Logger
 * @param context 上下文信息，如 { module: 'ModuleName' }
 * @returns 子 Logger 实例
 */
export function createLogger(context?: Record<string, unknown>): PinoLogger {
    if (context) {
        return baseLogger.child(context);
    }
    return baseLogger;
}

/**
 * 获取当前日志级别
 */
export function getLogLevel(): string {
    return logLevel;
}

/**
 * 检查是否为开发模式
 */
export function isDevelopmentMode(): boolean {
    return isDevelopment;
}

// 默认导出基础 Logger
const Logger = baseLogger;
export default Logger;
