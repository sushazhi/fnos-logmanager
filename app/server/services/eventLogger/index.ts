import fs from 'fs';
import path from 'path';
import { promisify } from 'util';
import Logger from '../../utils/logger';
import config from '../../utils/config';
import * as notificationStore from '../notificationStore';
import * as notificationService from '../notification';
import {
    EventLoggerConfig, EventLoggerStatus, EventLogEntry, EventLoggerStats,
    EventLoggerNotificationRule, GetEventsRequest, GetEventsResponse,
    EventSeverity, SEVERITY_ORDER
} from './types';
import {
    initSql, loadDatabase, closeDb, reconnect, isDbAccessible,
    isDbFileChanged, reloadDatabase, queryEvents, getLatestEventId,
    getNewEvents, getTotalEvents, getSources, parseSeverity
} from './db';
import { formatTimestampToLocal, formatEventMessage } from './templateHandler';

const logger = Logger.child({ module: 'EventLogger' });
const readFileAsync = promisify(fs.readFile);
const writeFileAsync = promisify(fs.writeFile);
const mkdirAsync = promisify(fs.mkdir);

const DEFAULT_CONFIG: EventLoggerConfig = {
    dbPath: '/usr/trim/var/eventlogger_service/logger_data.db3',
    enabled: false,
    checkInterval: 30000,
    eventTypes: ['*'],
    minSeverity: 'info',
    notificationChannels: []
};

let configFilePath: string | null = null;
let isRunning = false;
let checkInterval: NodeJS.Timeout | null = null;
let lastEventId = 0;
let configData: EventLoggerConfig = { ...DEFAULT_CONFIG };
let status: EventLoggerStatus = {
    isRunning: false, lastCheckTime: null, lastEventTime: null,
    totalEventsProcessed: 0, lastError: null, dbAccessible: false, dbPath: ''
};
const cooldownCache = new Map<string, number>();

export async function init(cfg?: Partial<EventLoggerConfig>): Promise<void> {
    configFilePath = path.join(config.dataDir, 'eventlogger-config.json');
    try { await mkdirAsync(config.dataDir, { recursive: true }); } catch { /* ok */ }

    const defaults: EventLoggerConfig = {
        dbPath: config.eventLogger.dbPath || DEFAULT_CONFIG.dbPath,
        enabled: config.eventLogger.enabled,
        checkInterval: config.eventLogger.checkInterval || DEFAULT_CONFIG.checkInterval,
        eventTypes: config.eventLogger.eventTypes?.length ? config.eventLogger.eventTypes : DEFAULT_CONFIG.eventTypes,
        minSeverity: config.eventLogger.minSeverity || DEFAULT_CONFIG.minSeverity,
        notificationChannels: config.eventLogger.notificationChannels?.length ? config.eventLogger.notificationChannels : DEFAULT_CONFIG.notificationChannels
    };

    try {
        const data = await readFileAsync(configFilePath, 'utf8');
        const saved = JSON.parse(data) as Partial<EventLoggerConfig>;
        configData = {
            dbPath: saved.dbPath ?? defaults.dbPath,
            enabled: saved.enabled ?? defaults.enabled,
            checkInterval: saved.checkInterval ?? defaults.checkInterval,
            eventTypes: saved.eventTypes ?? defaults.eventTypes,
            minSeverity: saved.minSeverity ?? defaults.minSeverity,
            notificationChannels: saved.notificationChannels ?? defaults.notificationChannels
        };
    } catch (e) {
        configData = { ...defaults };
        await saveElConfig();
    }

    logger.info({ config: configData, path: configFilePath }, 'EventLogger config loaded');
    await initSql();
    status.dbPath = configData.dbPath;
    const database = loadDatabase(configData.dbPath);
    status.dbAccessible = database !== null;

    // 只处理启动后的新事件，跳过历史事件
    if (status.dbAccessible) {
        lastEventId = getLatestEventId();
        logger.info({ lastEventId }, 'EventLogger initialized, skipping history');
    }

    if (configData.enabled) await start();
}

async function saveElConfig(): Promise<void> {
    if (!configFilePath) throw new Error('配置文件路径未设置');
    try {
        const dir = path.dirname(configFilePath);
        await mkdirAsync(dir, { recursive: true });
        const toSave = {
            enabled: configData.enabled, dbPath: configData.dbPath,
            checkInterval: configData.checkInterval, eventTypes: configData.eventTypes,
            minSeverity: configData.minSeverity, notificationChannels: configData.notificationChannels
        };
        await writeFileAsync(configFilePath, JSON.stringify(toSave, null, 2), 'utf8');
    } catch (e) {
        logger.error({ err: e, path: configFilePath }, 'Failed to save eventlogger config');
        throw e;
    }
}

function logLevelToSeverity(level: string): EventSeverity {
    switch (level) {
        case 'error': return 'error';
        case 'warn': return 'warning';
        case 'info': return 'info';
        case 'debug': case 'all': return 'debug';
        default: return 'debug';
    }
}

function eventMatchesNotifRule(event: EventLogEntry, rule: any): boolean {
    if (rule.appName !== 'eventlogger' && rule.appName !== '*') {
        const apps = (rule.appName || '').split(',').map((s: string) => s.trim());
        if (!apps.includes('eventlogger')) return false;
    }
    if (rule.sources && rule.sources.length > 0) {
        const src = event.source || '';
        if (!rule.sources.some((s: string) => src.toLowerCase().includes(s.toLowerCase()))) return false;
    }
    if (rule.excludeSources && rule.excludeSources.length > 0) {
        const src = event.source || '';
        if (rule.excludeSources.some((s: string) => src.toLowerCase().includes(s.toLowerCase()))) return false;
    }
    const minSeverity = logLevelToSeverity(rule.logLevel || 'all');
    const eventSeverity = parseSeverity(event.severity as string || event.level as string || 'info');
    if (SEVERITY_ORDER[eventSeverity] < SEVERITY_ORDER[minSeverity]) return false;
    const message = formatEventMessage(event);
    if (rule.excludeKeywords && rule.excludeKeywords.length > 0) {
        if (rule.excludeKeywords.some((kw: string) => message.toLowerCase().includes(kw.toLowerCase()))) return false;
    }
    if (rule.keywords && rule.keywords.length > 0) {
        if (!rule.keywords.some((kw: string) => message.toLowerCase().includes(kw.toLowerCase()))) return false;
    }
    if (rule.pattern) {
        try { if (!new RegExp(rule.pattern, 'i').test(message)) return false; } catch { /* skip */ }
    }
    return true;
}

async function sendEventNotification(event: EventLogEntry, rule: any): Promise<void> {
    const timestamp = formatTimestampToLocal(event.timestamp);
    const message = formatEventMessage(event);
    const title = `飞牛系统通知`;
    const content = `来源: ${event.source || '未知'}\n类型: ${event.eventType || event.template || ''}\n级别: ${(event.severity || 'info').toString().toUpperCase()}\n时间: ${timestamp}\n\n消息:\n${message}`;
    const channels = notificationStore.getChannels();
    const enabledChannels = channels.filter(c => rule.channels?.includes(c.name) && c.enabled);
    for (const channel of enabledChannels) {
        try {
            const result = await notificationService.sendToChannel(channel, title, content);
            await notificationStore.addHistory({
                id: `el-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`,
                ruleId: rule.id,
                ruleName: rule.name,
                channel: channel.channel,
                title, content,
                appName: event.source || '系统事件',
                logPath: '',
                matchedLine: message,
                success: result.success,
                error: result.error,
                timestamp: new Date()
            });
            if (result.success) {
                logger.info({ channel: channel.name, rule: rule.name }, 'Notification sent');
            } else {
                logger.error({ channel: channel.name, error: result.error }, 'Notification failed');
            }
        } catch (err) {
            logger.error({ err, channel: channel.name }, 'Notification error');
        }
    }
    cooldownCache.set(rule.id, Date.now());
}

async function processEvents(): Promise<void> {
    if (isDbFileChanged() || !isDbAccessible()) {
        if (reloadDatabase()) {
            const newLastId = getLatestEventId();
            // DB 轮转：重置为当前最大 ID，避免重复处理历史事件
            if (newLastId < lastEventId) {
                lastEventId = newLastId;
            }
        } else {
            status.lastError = `数据库不可访问: ${configData.dbPath}`;
            return;
        }
    }
    if (!isDbAccessible()) {
        await reconnect();
        if (!isDbAccessible()) {
            status.lastError = `数据库不可访问: ${configData.dbPath}`;
            return;
        }
    }
    status.lastError = null;
    const events = getNewEvents(lastEventId);
    if (events.length === 0) return;

    const notifRules = notificationStore.getEnabledRules();
    const elRules = notifRules.filter((r: any) => {
        if (r.appName === 'eventlogger' || r.appName === '*') return true;
        return (r.appName || '').split(',').map((s: string) => s.trim()).includes('eventlogger');
    });
    if (elRules.length === 0) return;

    for (const event of events) {
        lastEventId = Math.max(lastEventId, event.id);
        for (const rule of elRules) {
            if (!eventMatchesNotifRule(event, rule)) continue;
            if (notificationStore.isInQuietHours(rule)) continue;
            const lastSent = cooldownCache.get(rule.id);
            if (lastSent && (Date.now() - lastSent) < (rule.cooldown || 60) * 1000) continue;
            await sendEventNotification(event, rule);
        }
        status.totalEventsProcessed++;
        status.lastEventTime = new Date();
    }
    const now = Date.now();
    for (const [key, lastSent] of cooldownCache) {
        if (now - lastSent > 3600000) cooldownCache.delete(key);
    }
}

export async function start(): Promise<void> {
    if (isRunning) return;
    if (!isDbAccessible()) {
        await reconnect();
        if (!isDbAccessible()) {
            status.lastError = `数据库不可访问: ${configData.dbPath}`;
            logger.warn({ dbPath: configData.dbPath }, 'EventLogger start: DB not accessible');
        }
    }
    // 启动时跳过历史事件，只处理之后的新事件
    lastEventId = getLatestEventId();
    isRunning = true;
    status.isRunning = true;
    configData.enabled = true;
    await saveElConfig();
    logger.info({ checkInterval: configData.checkInterval }, 'EventLogger started');
    await processEvents();
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

export async function stop(): Promise<void> {
    if (!isRunning) return;
    isRunning = false;
    status.isRunning = false;
    configData.enabled = false;
    await saveElConfig();
    if (checkInterval) { clearInterval(checkInterval); checkInterval = null; }
    closeDb();
    logger.info('EventLogger stopped');
}

export async function restart(): Promise<void> { await stop(); await start(); }
export function getStatus(): EventLoggerStatus { return { ...status, dbAccessible: isDbAccessible() }; }
export function getStats(): EventLoggerStats {
    return { totalEvents: getTotalEvents(), recentEvents: queryEvents({ limit: 10, sortDirection: 'desc' }) };
}
export function getEvents(request: GetEventsRequest): GetEventsResponse {
    const events = queryEvents(request);
    return { events, total: getTotalEvents(), hasMore: request.offset! + events.length < getTotalEvents() };
}
export async function updateConfig(updates: Partial<EventLoggerConfig>): Promise<void> {
    const wasEnabled = configData.enabled;
    configData = { ...configData, ...updates };
    await saveElConfig();
    Object.assign(config.eventLogger, configData);
    if (updates.enabled !== undefined && updates.enabled !== wasEnabled) {
        if (updates.enabled && !isRunning) await start();
        else if (!updates.enabled && isRunning) await stop();
    }
    if (updates.checkInterval && isRunning) await restart();
}
export function getConfig(): EventLoggerConfig { return { ...configData }; }
export async function forceCheck(): Promise<{ newEvents: number }> {
    const beforeId = lastEventId;
    await processEvents();
    return { newEvents: lastEventId - beforeId };
}
export function getSourcesList(): string[] { return getSources(); }


export {
    EventLoggerConfig, EventLoggerStatus, EventLogEntry, EventLoggerStats,
    EventLoggerNotificationRule, GetEventsRequest, GetEventsResponse, EventSeverity
} from './types';
