/**
 * 日志监控服务
 * 监测三方应用日志，根据规则触发通知
 */

import fs from 'fs';
import path from 'path';
import { promisify } from 'util';
import * as notificationStore from './notificationStore';
import * as notificationService from './notification';
import * as logFileService from './logFile';
import Logger from '../utils/logger';

const logger = Logger.child({ module: 'LogMonitor' });

const stat = promisify(fs.stat);
const open = promisify(fs.open);
const read = promisify(fs.read);
const close = promisify(fs.close);

// 监控状态
let isRunning = false;
let checkInterval: NodeJS.Timeout | null = null;
let watchedFiles = new Map<string, { lastSize: number; lastModified: number }>();
let activeRules: any[] = [];
let errors: string[] = [];
let lastCheckTime: Date = new Date();

// 日志级别匹配模式
const LOG_LEVEL_PATTERNS: Record<string, RegExp[]> = {
    error: [
        /\b(error|err|fatal|critical|crit|emergency|emerg|panic)\b/i,
        /\b(exception|except)\b/i,
        /\b(failed|failure)\b/i
    ],
    warn: [
        /\b(warn|warning)\b/i
    ],
    info: [
        /\b(info|information)\b/i
    ],
    debug: [
        /\b(debug|dbg|trace|verbose)\b/i
    ],
    all: [/.*/]
};

// 生成唯一ID
function generateId(): string {
    return `${Date.now()}-${Math.random().toString(36).substr(2, 9)}`;
}

// 检查日志行是否匹配级别
function matchesLogLevel(line: string, level: string): boolean {
    if (level === 'all') return true;
    const patterns = LOG_LEVEL_PATTERNS[level];
    if (!patterns) return true;
    return patterns.some(pattern => pattern.test(line));
}

// 匹配关键词（支持正则表达式）
function matchKeyword(line: string, keyword: string): boolean {
    if (!keyword) return false;
    
    // 检查是否是 /pattern/flags 格式
    const slashMatch = keyword.match(/^\/(.+)\/([gimsuvy]*)$/);
    if (slashMatch) {
        try {
            const regex = new RegExp(slashMatch[1], slashMatch[2] || 'i');
            return regex.test(line);
        } catch {
            return line.toLowerCase().includes(keyword.toLowerCase());
        }
    }
    
    // 检查是否是 regex: 前缀格式
    if (keyword.startsWith('regex:')) {
        const pattern = keyword.substring(6);
        try {
            const regex = new RegExp(pattern, 'i');
            return regex.test(line);
        } catch {
            return line.toLowerCase().includes(keyword.toLowerCase());
        }
    }
    
    // 普通文本匹配
    return line.toLowerCase().includes(keyword.toLowerCase());
}

// 检查日志行是否匹配规则
function matchesRule(line: string, rule: any): boolean {
    // 检查日志级别
    if (!matchesLogLevel(line, rule.logLevel)) {
        return false;
    }
    
    // 检查排除关键词
    if (rule.excludeKeywords && rule.excludeKeywords.length > 0) {
        for (const keyword of rule.excludeKeywords) {
            if (matchKeyword(line, keyword)) {
                return false;
            }
        }
    }
    
    // 检查关键词匹配
    if (rule.keywords && rule.keywords.length > 0) {
        let matched = false;
        for (const keyword of rule.keywords) {
            if (matchKeyword(line, keyword)) {
                matched = true;
                break;
            }
        }
        if (!matched) return false;
    }
    
    // 检查正则表达式
    if (rule.pattern) {
        try {
            const regex = new RegExp(rule.pattern, 'i');
            if (!regex.test(line)) {
                return false;
            }
        } catch {
            // 正则无效，忽略
        }
    }
    
    return true;
}

// 检查应用名称是否匹配
function matchesAppName(appName: string, pattern: string): boolean {
    if (!appName) return false;
    if (pattern === '*') return true;
    
    // 支持 /pattern/flags 格式的正则表达式
    const slashMatch = pattern.match(/^\/(.+)\/([gimsuvy]*)$/);
    if (slashMatch) {
        try {
            const regex = new RegExp(slashMatch[1], slashMatch[2] || 'i');
            return regex.test(appName);
        } catch {
            // 正则无效，回退到普通匹配
        }
    }
    
    // 支持 regex: 前缀格式
    if (pattern.startsWith('regex:')) {
        const regexPattern = pattern.substring(6);
        try {
            const regex = new RegExp(regexPattern, 'i');
            return regex.test(appName);
        } catch {
            // 正则无效，回退到普通匹配
        }
    }
    
    // 支持简单的通配符 *
    if (pattern.includes('*')) {
        const regexPattern = pattern
            .replace(/\./g, '\\.')
            .replace(/\*/g, '.*');
        return new RegExp(`^${regexPattern}$`, 'i').test(appName);
    }
    
    // 精确匹配（不区分大小写）
    return appName.toLowerCase() === pattern.toLowerCase();
}

// 获取日志文件的新增内容
async function getNewContent(filePath: string, lastSize: number): Promise<{ content: string; newSize: number } | null> {
    try {
        const stats = await stat(filePath);
        const newSize = stats.size;
        
        // 文件变小了（可能被清空或轮转），重新开始
        if (newSize < lastSize) {
            return { content: '', newSize };
        }
        
        // 没有新内容
        if (newSize === lastSize) {
            return null;
        }
        
        // 读取新增内容
        const fd = await open(filePath, 'r');
        const buffer = Buffer.alloc(newSize - lastSize);
        
        try {
            await read(fd, buffer, 0, buffer.length, lastSize);
            await close(fd);
            return { content: buffer.toString('utf8'), newSize };
        } catch {
            await close(fd);
            return null;
        }
    } catch {
        return null;
    }
}

// 处理日志文件
async function processLogFile(filePath: string, appName: string | null): Promise<void> {
    const fileInfo = watchedFiles.get(filePath);
    if (!fileInfo) return;
    
    // 获取新增内容
    const result = await getNewContent(filePath, fileInfo.lastSize);
    if (!result || !result.content) {
        if (result) {
            watchedFiles.set(filePath, {
                lastSize: result.newSize,
                lastModified: Date.now()
            });
        }
        return;
    }
    
    // 更新文件信息
    watchedFiles.set(filePath, {
        lastSize: result.newSize,
        lastModified: Date.now()
    });
    
    // 按行处理
    const lines = result.content.split('\n');
    for (const line of lines) {
        if (!line.trim()) continue;
        
        // 检查每条规则
        for (const rule of activeRules) {
            // 检查应用名称匹配
            if (!matchesAppName(appName || '', rule.appName)) continue;
            
            // 检查日志路径（如果规则指定了路径）
            if (rule.logPaths && rule.logPaths.length > 0) {
                const pathMatch = rule.logPaths.some((p: string) => filePath === p || filePath.startsWith(p));
                if (!pathMatch) continue;
            }
            
            // 检查是否匹配规则
            if (!matchesRule(line, rule)) continue;
            
            // 检查频率控制
            if (!notificationStore.canSendNotification(rule)) continue;
            
            // 检查静默时段
            if (notificationStore.isInQuietHours(rule)) continue;
            
            // 发送通知
            try {
                await notificationService.sendNotification({
                    title: `[日志告警] ${appName || '未知应用'}`,
                    content: line,
                    appName: appName || '未知应用',
                    logPath: filePath,
                    matchedLine: line,
                    rule
                });
                logger.info({ rule: rule.name, app: appName, file: filePath }, '触发通知');
            } catch (err) {
                logger.error({ err }, '发送通知失败');
            }
        }
    }
}

// 执行一次检查
async function performCheck(): Promise<void> {
    const checkStartTime = Date.now();
    lastCheckTime = new Date();
    errors = [];
    
    try {
        // 获取启用的规则
        activeRules = notificationStore.getEnabledRules();
        
        if (activeRules.length === 0) {
            return;
        }
        
        // 获取所有日志文件
        const logFiles = await logFileService.listLogFiles(undefined, 500);
        
        // 更新监控文件列表
        for (const logFile of logFiles) {
            if (!watchedFiles.has(logFile.path)) {
                watchedFiles.set(logFile.path, {
                    lastSize: logFile.size,
                    lastModified: logFile.modified.getTime()
                });
            }
        }
        
        // 移除不存在的文件
        for (const [filePath] of watchedFiles) {
            if (!logFiles.some(f => f.path === filePath)) {
                watchedFiles.delete(filePath);
            }
        }
        
        // 处理每个文件
        for (const logFile of logFiles) {
            // 排除日志管理器的运行日志文件，避免循环通知
            // 当 EventLogger 发送通知后，会写入 info.log/error.log，LogMonitor 不应再次触发通知
            // 但保留 audit.log 的监控，因为审计日志是用户操作记录，需要监控
            if (logFile.appName === 'logmanager') {
                const fileName = logFile.path.split('/').pop() || '';
                // 只排除运行日志，保留审计日志
                if (fileName === 'info.log' || fileName === 'error.log' || 
                    fileName.startsWith('info.log.') || fileName.startsWith('error.log.')) {
                    continue;
                }
            }
            
            try {
                await processLogFile(logFile.path, logFile.appName ?? null);
            } catch (err) {
                const error = err as Error;
                errors.push(`${logFile.path}: ${error.message}`);
                logger.error({ err: error, file: logFile.path }, '处理文件失败');
            }
        }
        
        // 清理频率控制缓存
        notificationStore.cleanupFrequencyCache();
        
        const checkDuration = Date.now() - checkStartTime;
        if (checkDuration > 1000 || errors.length > 0) {
            logger.info({ duration: checkDuration, files: logFiles.length, errors: errors.length }, '检查完成');
        }

    } catch (err) {
        const error = err as Error;
        errors.push(`检查失败: ${error.message}`);
        logger.error({ err: error }, '检查失败');
    }
}

// 初始化
export async function init(): Promise<void> {
    // 初始化存储
    await notificationStore.init();

    // 如果通知功能已启用，自动启动监控
    const settings = notificationStore.getSettings();
    logger.info({ enabled: settings.enabled, checkInterval: settings.checkInterval }, '通知功能状态');

    if (settings.enabled) {
        await start();
    } else {
        logger.info('通知功能未启用，监控服务未启动。可通过API /api/notifications/settings 启用');
    }

    logger.info('监控服务初始化完成');
}

// 启动
export async function start(): Promise<void> {
    if (isRunning) {
        logger.info('监控已在运行中');
        return;
    }

    const settings = notificationStore.getSettings();

    // 即使配置未启用，也允许启动监控（用于手动启动）
    // 但会记录警告日志
    if (!settings.enabled) {
        logger.warn('通知功能未启用，但监控服务将启动。建议在设置中启用通知功能');
    }

    isRunning = true;
    watchedFiles.clear();
    activeRules = [];
    errors = [];

    logger.info({ checkInterval: settings.checkInterval }, '启动监控');

    // 立即执行一次检查
    await performCheck();

    // 设置定时检查，添加错误处理
    checkInterval = setInterval(async () => {
        try {
            await performCheck();
        } catch (err) {
            logger.error({ err }, '定时检查错误');
        }
    }, settings.checkInterval);

    logger.info('监控启动成功');
}

// 停止
export function stop(): void {
    if (!isRunning) {
        return;
    }

    isRunning = false;
    if (checkInterval) {
        clearInterval(checkInterval);
        checkInterval = null;
    }

    watchedFiles.clear();
    activeRules = [];

    logger.info('监控已停止');
}

// 重启
export async function restart(): Promise<void> {
    stop();
    await start();
}

// 获取状态
export function getStatus() {
    return {
        running: isRunning,
        watchedFiles: watchedFiles.size,
        activeRules: activeRules.length,
        lastCheckTime,
        errors: errors.slice(0, 10)
    };
}

// 触发检查（供外部调用）
export async function triggerCheck(): Promise<void> {
    await performCheck();
}

// 检查特定应用的日志
export async function checkAppLogs(appName: string): Promise<void> {
    const rules = notificationStore.getEnabledRules().filter((r: any) => matchesAppName(appName, r.appName));
    if (rules.length === 0) {
        return;
    }
    
    // 获取该应用的日志文件
    const logFiles = await logFileService.listLogFiles(undefined, 500);
    const appLogFiles = logFiles.filter((f: any) => f.appName && matchesAppName(f.appName, appName));
    
    for (const logFile of appLogFiles) {
        try {
            // 读取最新的日志内容
            const result = await logFileService.readLogFile(logFile.path, {
                maxLines: 100,
                tail: true
            });
            
            const lines = result.content.split('\n');
            for (const line of lines) {
                if (!line.trim()) continue;
                
                for (const rule of rules) {
                    if (!matchesRule(line, rule)) continue;
                    if (!notificationStore.canSendNotification(rule)) continue;
                    if (notificationStore.isInQuietHours(rule)) continue;
                    
                    try {
                        await notificationService.sendNotification({
                            title: `[日志告警] ${appName}`,
                            content: line,
                            appName,
                            logPath: logFile.path,
                            matchedLine: line,
                            rule
                        });
                    } catch (err) {
                        logger.error({ err }, '发送通知失败');
                    }
                }
            }
        } catch (err) {
            logger.error({ err, file: logFile.path }, '检查日志失败');
        }
    }
}

// 测试规则匹配
export async function testRuleMatch(ruleId: string, testContent: string): Promise<{ matched: boolean; reason: string }> {
    const rule = notificationStore.getRule(ruleId);
    if (!rule) {
        return { matched: false, reason: '规则不存在' };
    }
    
    const lines = testContent.split('\n');
    for (const line of lines) {
        if (!line.trim()) continue;
        
        // 检查日志级别
        if (!matchesLogLevel(line, rule.logLevel)) {
            continue;
        }
        
        // 检查排除关键词
        if (rule.excludeKeywords && rule.excludeKeywords.length > 0) {
            let excluded = false;
            for (const keyword of rule.excludeKeywords) {
                if (matchKeyword(line, keyword)) {
                    excluded = true;
                    break;
                }
            }
            if (excluded) continue;
        }
        
        // 检查关键词匹配
        if (rule.keywords && rule.keywords.length > 0) {
            let matched = false;
            for (const keyword of rule.keywords) {
                if (matchKeyword(line, keyword)) {
                    matched = true;
                    break;
                }
            }
            if (!matched) continue;
        }
        
        // 检查正则表达式
        if (rule.pattern) {
            try {
                const regex = new RegExp(rule.pattern, 'i');
                if (!regex.test(line)) {
                    continue;
                }
            } catch {
                continue;
            }
        }
        
        return {
            matched: true,
            reason: `匹配行: "${line.substring(0, 100)}${line.length > 100 ? '...' : ''}"`
        };
    }
    
    return { matched: false, reason: '没有匹配的日志行' };
}
