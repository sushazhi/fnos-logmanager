/**
 * EventLogger 入口模块
 * 导出所有公共接口，保持与原有模块兼容
 */
import fs from 'fs';
import path from 'path';
import Logger from '../../utils/logger';
import config from '../../utils/config';
import * as notificationStore from '../notificationStore';
import * as notificationService from '../notification';

import {
    EventLoggerConfig,
    EventLoggerStatus,
    EventLogEntry,
    EventLoggerStats,
    EventLoggerNotificationRule,
    GetEventsRequest,
    GetEventsResponse,
    EventSeverity,
    SEVERITY_ORDER
} from './types';

import {
    initSql,
    loadDatabase,
    closeDb,
    reconnect,
    getDb,
    isDbAccessible,
    queryEvents,
    getLatestEventId,
    getNewEvents,
    getTotalEvents,
    getSources,
    parseSeverity
} from './db';

import {
    formatTimestampToLocal,
    formatTemplateMessage,
    formatEventMessage
} from './templateHandler';

const logger = Logger.child({ module: 'EventLogger' });

// 状态变量
let isRunning = false;
let checkInterval: NodeJS.Timeout | null = null;
let lastEventId = 0;
let configData: EventLoggerConfig;
let rules: EventLoggerNotificationRule[] = [];
let status: EventLoggerStatus = {
    isRunning: false,
    lastCheckTime: null,
    lastEventTime: null,
    totalEventsProcessed: 0,
    lastError: null,
    dbAccessible: false,
    dbPath: ''
};

// 频率控制缓存
const notificationCache = new Map<string, { lastSent: number; count: number }>();

/**
 * 初始化服务
 */
export async function init(cfg?: Partial<EventLoggerConfig>): Promise<void> {
    await initSql();

    configData = {
        dbPath: cfg?.dbPath || config.eventLogger.dbPath,
        enabled: cfg?.enabled ?? config.eventLogger.enabled,
        checkInterval: cfg?.checkInterval || config.eventLogger.checkInterval,
        eventTypes: cfg?.eventTypes || config.eventLogger.eventTypes,
        minSeverity: cfg?.minSeverity || config.eventLogger.minSeverity,
        notificationChannels: cfg?.notificationChannels || config.eventLogger.notificationChannels
    };

    status.dbPath = configData.dbPath;

    // 加载数据库
    const database = loadDatabase(configData.dbPath);
    status.dbAccessible = database !== null;

    if (status.dbAccessible) {
        lastEventId = getLatestEventId();
        logger.info({ lastEventId }, 'EventLogger initialized');
    }

    // 加载规则
    rules = loadRules();

    // 如果启用，自动启动
    if (configData.enabled) {
        await start();
    }
}

/**
 * 加载通知规则
 */
function loadRules(): EventLoggerNotificationRule[] {
    const rulesPath = path.join(config.dataDir, 'eventlogger-rules.json');
    try {
        if (fs.existsSync(rulesPath)) {
            const content = fs.readFileSync(rulesPath, 'utf8');
            return JSON.parse(content);
        }
    } catch (err) {
        logger.error({ err }, 'Failed to load rules');
    }
    return [];
}

/**
 * 测试数据库连接
 */
export async function testConnection(): Promise<boolean> {
    await initSql();
    const database = loadDatabase(configData.dbPath);
    return database !== null;
}

/**
 * 检查事件是否匹配规则
 */
function eventMatchesRule(event: EventLogEntry, rule: EventLoggerNotificationRule): boolean {
    // 检查事件类型
    if (rule.eventTypes && rule.eventTypes.length > 0) {
        const template = event.template;
        if (!template || !rule.eventTypes.includes(template)) {
            return false;
        }
    }

    // 检查来源
    if (rule.sources && rule.sources.length > 0) {
        const source = event.source;
        if (!source || !rule.sources.includes(source)) {
            return false;
        }
    }

    // 检查严重程度
    if (rule.minSeverity) {
        const eventSeverity = parseSeverity(event.severity as string || event.level as string || 'info');
        if (SEVERITY_ORDER[eventSeverity] < SEVERITY_ORDER[rule.minSeverity]) {
            return false;
        }
    }

    // 检查关键词
    const message = formatEventMessage(event);
    if (rule.keywords && rule.keywords.length > 0) {
        const hasKeyword = rule.keywords.some(kw => message.toLowerCase().includes(kw.toLowerCase()));
        if (!hasKeyword) {
            return false;
        }
    }

    // 检查排除关键词
    if (rule.excludeKeywords && rule.excludeKeywords.length > 0) {
        const hasExclude = rule.excludeKeywords.some(kw => message.toLowerCase().includes(kw.toLowerCase()));
        if (hasExclude) {
            return false;
        }
    }

    return true;
}

/**
 * 检查是否在静默时段
 */
function isInQuietHours(rule: EventLoggerNotificationRule): boolean {
    if (!rule.quietHoursStart || !rule.quietHoursEnd) {
        return false;
    }

    const now = new Date();
    const currentMinutes = now.getHours() * 60 + now.getMinutes();

    const [startH, startM] = rule.quietHoursStart.split(':').map(Number);
    const [endH, endM] = rule.quietHoursEnd.split(':').map(Number);
    const startMinutes = startH * 60 + startM;
    const endMinutes = endH * 60 + endM;

    if (startMinutes <= endMinutes) {
        return currentMinutes >= startMinutes && currentMinutes <= endMinutes;
    } else {
        return currentMinutes >= startMinutes || currentMinutes <= endMinutes;
    }
}

/**
 * 检查是否可以发送通知
 */
function canSendNotification(rule: EventLoggerNotificationRule): boolean {
    if (!rule.cooldown) {
        return true;
    }

    const cacheKey = rule.id;
    const cached = notificationCache.get(cacheKey);

    if (cached) {
        const elapsed = Date.now() - cached.lastSent;
        if (elapsed < rule.cooldown * 1000) {
            return false;
        }
    }

    return true;
}

/**
 * 发送事件通知
 */
async function sendEventNotification(event: EventLogEntry, rule: EventLoggerNotificationRule): Promise<void> {
    const message = formatEventMessage(event);
    const timestamp = formatTimestampToLocal(event.timestamp);

    const title = `[系统事件] ${rule.name}`;
    const content = `时间: ${timestamp}\n来源: ${event.source || '未知'}\n事件: ${message}`;

    try {
        await notificationService.sendNotification({
            title,
            content,
            appName: event.source || '系统事件',
            logPath: '',
            matchedLine: message,
            rule
        });

        // 更新缓存
        notificationCache.set(rule.id, { lastSent: Date.now(), count: 1 });
        logger.info({ rule: rule.name, eventId: event.id }, 'Event notification sent');
    } catch (err) {
        logger.error({ err, rule: rule.name }, 'Failed to send event notification');
    }
}

/**
 * 处理事件
 */
async function processEvents(): Promise<void> {
    if (!isDbAccessible()) {
        // 尝试重新连接
        await reconnect();
        if (!isDbAccessible()) {
            return;
        }
    }

    const events = getNewEvents(lastEventId);

    if (events.length === 0) {
        return;
    }

    logger.debug({ count: events.length }, 'Processing new events');

    for (const event of events) {
        // 更新最后事件 ID
        lastEventId = Math.max(lastEventId, event.id);

        // 检查每个规则
        for (const rule of rules) {
            if (!rule.enabled) continue;

            if (!eventMatchesRule(event, rule)) continue;
            if (isInQuietHours(rule)) continue;
            if (!canSendNotification(rule)) continue;

            await sendEventNotification(event, rule);
        }

        status.totalEventsProcessed++;
        status.lastEventTime = new Date();
    }

    // 清理缓存
    const now = Date.now();
    for (const [key, value] of notificationCache) {
        if (now - value.lastSent > 3600000) { // 1小时
            notificationCache.delete(key);
        }
    }
}

/**
 * 启动服务
 */
export async function start(): Promise<void> {
    if (isRunning) {
        logger.warn('EventLogger already running');
        return;
    }

    isRunning = true;
    status.isRunning = true;

    logger.info({ checkInterval: configData.checkInterval }, 'EventLogger started');

    // 立即执行一次检查
    await processEvents();

    // 设置定时检查
    checkInterval = setInterval(async () => {
        try {
            status.lastCheckTime = new Date();
            await processEvents();
        } catch (err) {
            logger.error({ err }, 'EventLogger check failed');
            status.lastError = (err as Error).message;
        }
    }, configData.checkInterval);
}

/**
 * 停止服务
 */
export function stop(): void {
    if (!isRunning) {
        return;
    }

    isRunning = false;
    status.isRunning = false;

    if (checkInterval) {
        clearInterval(checkInterval);
        checkInterval = null;
    }

    closeDb();
    logger.info('EventLogger stopped');
}

/**
 * 重启服务
 */
export async function restart(): Promise<void> {
    stop();
    await start();
}

/**
 * 获取状态
 */
export function getStatus(): EventLoggerStatus {
    return { ...status, dbAccessible: isDbAccessible() };
}

/**
 * 获取统计信息
 */
export function getStats(): EventLoggerStats {
    const stats: EventLoggerStats = {
        totalEvents: getTotalEvents(),
        eventsBySeverity: { debug: 0, info: 0, warning: 0, error: 0, critical: 0 },
        eventsBySource: {},
        eventsByTemplate: {},
        recentEvents: []
    };

    // 获取最近事件
    stats.recentEvents = queryEvents({ limit: 10, sortDirection: 'desc' });

    return stats;
}

/**
 * 获取事件
 */
export function getEvents(request: GetEventsRequest): GetEventsResponse {
    const events = queryEvents(request);
    const total = getTotalEvents();

    return {
        events,
        total,
        hasMore: request.offset! + events.length < total
    };
}

/**
 * 更新配置
 */
export async function updateConfig(updates: Partial<EventLoggerConfig>): Promise<void> {
    configData = { ...configData, ...updates };

    // 保存配置
    const configPath = path.join(config.dataDir, 'eventlogger-config.json');
    fs.writeFileSync(configPath, JSON.stringify(configData, null, 2));

    // 如果检查间隔改变，重启
    if (updates.checkInterval && isRunning) {
        await restart();
    }
}

/**
 * 获取配置
 */
export function getConfig(): EventLoggerConfig {
    return { ...configData };
}

/**
 * 强制检查
 */
export async function forceCheck(): Promise<void> {
    await processEvents();
}

/**
 * 获取来源列表
 */
export function getSourcesList(): string[] {
    return getSources();
}

/**
 * 清理资源
 */
export function dispose(): void {
    stop();
    notificationCache.clear();
    rules = [];
}

// 导出类型
export {
    EventLoggerConfig,
    EventLoggerStatus,
    EventLogEntry,
    EventLoggerStats,
    EventLoggerNotificationRule,
    GetEventsRequest,
    GetEventsResponse,
    EventSeverity
};
