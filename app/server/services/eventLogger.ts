/**
 * Event Logger Service
 * Monitors eventlogger_service SQLite database and triggers notifications
 */

import initSqlJs, { Database as SqlJsDatabase } from 'sql.js';
import fs from 'fs';
import path from 'path';
import * as notificationStore from './notificationStore';
import * as notificationService from './notification';
import config from '../utils/config';
import {
    EventLoggerConfig,
    EventLoggerStatus,
    EventLogEntry,
    EventLoggerStats,
    EventLoggerNotificationRule,
    EventSeverity,
    GetEventsRequest,
    GetEventsResponse,
    EventNotificationRequest
} from '../types/eventLogger';
import { parseEventRow, formatTimestampToLocal, formatTemplateMessage, parseLoglevel, TIMEZONE_OFFSET_SECONDS } from './eventLogger/eventParser';

let SQL: any = null;
let db: SqlJsDatabase | null = null;
let isRunning = false;
let checkInterval: NodeJS.Timeout | null = null;
let lastEventId = 0;
let lastDbMtime = 0; // 数据库文件最后修改时间，用于增量检测
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

// Severity order for comparison
const SEVERITY_ORDER: Record<EventSeverity, number> = {
    debug: 0,
    info: 1,
    warning: 2,
    error: 3,
    critical: 4
};

/**
 * 计算本地时区与 UTC 的偏移量（秒）
 * 用于修正飞牛系统数据库中存储的本地时间戳
 */
function getLocalTimezoneOffsetSeconds(): number {
    const now = new Date();
    // getTimezoneOffset() 返回 UTC - 本地的差值（分钟）
    // 例如东八区返回 -480，我们需要 +28800 秒
    return -now.getTimezoneOffset() * 60;
}

// 缓存时区偏移，避免每次调用都计算
// TIMEZONE_OFFSET_SECONDS 已从 eventParser 导入

// 已发送通知的事件ID集合（去重）
const notifiedEvents = new Set<number>();

// 清理超过24小时的旧事件ID
setInterval(() => {
    // 简单处理：每24小时清空一次
    // 实际生产环境可能需要更复杂的持久化方案
}, 24 * 60 * 60 * 1000);

/**
 * Initialize the service with config
 */
export async function init(cfg?: Partial<EventLoggerConfig>): Promise<void> {
    configData = {
        dbPath: cfg?.dbPath || '/usr/trim/var/eventlogger_service/logger_data.db3',
        enabled: cfg?.enabled ?? false,
        checkInterval: cfg?.checkInterval ?? 30000,
        eventTypes: cfg?.eventTypes ?? ['*'],
        minSeverity: cfg?.minSeverity ?? 'info',
        notificationChannels: cfg?.notificationChannels ?? [],
        appFilter: cfg?.appFilter,
        excludeSources: cfg?.excludeSources
    };

    status.dbPath = configData.dbPath;

    // Initialize SQL.js
    try {
        SQL = await initSqlJs();
        console.log('[EventLogger] SQL.js initialized');
    } catch (err) {
        console.error('[EventLogger] Failed to initialize SQL.js:', err);
        status.lastError = 'Failed to initialize SQL.js';
        return;
    }

    // Try to connect to database
    await testConnection();

    // Load rules from notification store
    rules = loadRules();

    console.log('[EventLogger] Service initialized');

    if (configData.enabled && status.dbAccessible) {
        await start();
    }
}

/**
 * Load database from file
 */
function loadDatabase(): SqlJsDatabase | null {
    try {
        if (!fs.existsSync(configData.dbPath)) {
            status.lastError = `Database file not found: ${configData.dbPath}`;
            console.error('[EventLogger]', status.lastError);
            return null;
        }

        const fileBuffer = fs.readFileSync(configData.dbPath);
        // 使用 Uint8Array 确保内存对齐
        const uint8Array = new Uint8Array(fileBuffer);
        const database = new SQL.Database(uint8Array);
        
        // Test query
        const result = database.exec('SELECT 1 as test');
        
        if (result.length > 0) {
            status.dbAccessible = true;
            status.lastError = null;
            console.log('[EventLogger] Database connection OK');
            return database;
        }
        
        return null;
    } catch (err) {
        status.dbAccessible = false;
        status.lastError = `Database connection failed: ${(err as Error).message}`;
        console.error('[EventLogger]', status.lastError);
        return null;
    }
}

/**
 * Test database connection
 */
export async function testConnection(): Promise<boolean> {
    db = loadDatabase();
    return db !== null;
}

/**
 * Close database connection
 */
function closeDb(): void {
    if (db) {
        try {
            db.close();
        } catch {
            // Ignore close errors
        }
        db = null;
    }
}

/**
 * Reconnect to database
 * 使用 mtime 检测优化：仅在数据库文件变更时重新加载
 */
async function reconnect(force: boolean = false): Promise<void> {
    try {
        const stat = fs.statSync(configData.dbPath);
        const currentMtime = stat.mtimeMs;
        
        // 如果文件未修改且不强制刷新，跳过重新加载
        if (!force && currentMtime === lastDbMtime && db) {
            return;
        }
        
        lastDbMtime = currentMtime;
    } catch {
        // 文件不存在或无法访问，继续尝试重连
    }
    
    closeDb();
    db = loadDatabase();
}

/**
 * Load notification rules from store
 */
function loadRules(): EventLoggerNotificationRule[] {
    // Convert from notification store rules
    const storeRules = notificationStore.getRules();
    
    return storeRules
        .filter(r => r.appName === 'eventlogger' || r.appName === '*')
        .map(r => ({
            id: r.id,
            name: r.name,
            enabled: r.status === 'enabled',
            eventTypes: r.logPaths || ['*'],
            severity: mapLogLevelToSeverity(r.logLevel),
            sources: r.sources || (r.appName === '*' ? undefined : [r.appName]),
            excludeSources: r.excludeSources,
            keywords: r.keywords,
            excludeKeywords: r.excludeKeywords,
            channels: r.channels,
            cooldown: r.cooldown,
            maxNotificationsPerHour: r.maxNotifications,
            quietHoursStart: r.quietHoursStart,
            quietHoursEnd: r.quietHoursEnd,
            createdAt: r.createdAt,
            lastTriggeredAt: r.lastTriggeredAt,
            triggerCount: r.triggerCount
        }));
}

/**
 * Map log level to severity
 */
function mapLogLevelToSeverity(level: string): EventSeverity {
    const map: Record<string, EventSeverity> = {
        'error': 'error',
        'warn': 'warning',
        'warning': 'warning',
        'info': 'info',
        'debug': 'debug',
        'all': 'info'
    };
    return map[level] || 'info';
}

// parseSeverity 和 parseLoglevel 已从 eventParser 导入

/**
 * Try to find event table and get events
 */
function queryEvents(request: GetEventsRequest): EventLogEntry[] {
    if (!db) {
        reconnect();
        if (!db) return [];
    }

    if (!db) return [];

    try {
        // Get all tables
        const tablesResult = db.exec(
            "SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'"
        );
        
        if (tablesResult.length === 0) return [];
        
        const tableNames = tablesResult[0].values.map((row: any) => row[0]);
        let events: any[] = [];
        
        // 表名安全验证正则：只允许字母、数字、下划线
        const VALID_TABLE_PATTERN = /^[a-zA-Z_][a-zA-Z0-9_]*$/;
        
        for (const tableName of tableNames) {
            // 安全检查：验证表名格式，防止 SQL 注入
            if (!VALID_TABLE_PATTERN.test(tableName)) {
                console.warn(`[EventLogger] 跳过无效表名: ${tableName}`);
                continue;
            }
            
            try {
                // Get column info
                const columnsResult = db.exec(`PRAGMA table_info(${tableName})`);
                if (columnsResult.length === 0) continue;
                
                const columnNames = columnsResult[0].values.map((row: any) => row[1].toLowerCase());
                
                // Find relevant columns
                const timestampCol = columnNames.find(c => 
                    c.includes('time') || c.includes('date') || c === 'created' || c === 'timestamp'
                );
                const idCol = columnNames.find(c => c === 'id' || c === 'rowid');
                const sourceCol = columnNames.find(c => 
                    c.includes('source') || c.includes('app') || c.includes('name') || c === 'program'
                );
                const messageCol = columnNames.find(c => 
                    c.includes('message') || c.includes('msg') || c.includes('content') || c === 'description'
                );
                const severityCol = columnNames.find(c => 
                    c.includes('severity') || c.includes('level') || c.includes('priority')
                );
                const typeCol = columnNames.find(c => 
                    c.includes('type') || c.includes('category') || c.includes('event')
                );

                if (!idCol) continue;

                // Build query based on available columns
                let sql = `SELECT * FROM ${tableName}`;
                const conditions: string[] = [];
                const params: any[] = [];

                // Apply filters
                if (request.startTime && timestampCol) {
                    conditions.push(`${timestampCol} >= ?`);
                    params.push(request.startTime);
                }
                if (request.endTime && timestampCol) {
                    conditions.push(`${timestampCol} <= ?`);
                    params.push(request.endTime);
                }
                if (request.source && sourceCol) {
                    conditions.push(`${sourceCol} LIKE ?`);
                    params.push(`%${request.source}%`);
                }
                if (request.search && messageCol) {
                    conditions.push(`${messageCol} LIKE ?`);
                    params.push(`%${request.search}%`);
                }
                if (request.severity && severityCol) {
                    conditions.push(`${severityCol} = ?`);
                    params.push(request.severity);
                }
                if (request.eventType && typeCol) {
                    conditions.push(`${typeCol} = ?`);
                    params.push(request.eventType);
                }

                if (conditions.length > 0) {
                    sql += ' WHERE ' + conditions.join(' AND ');
                }

                // Order by timestamp descending
                if (timestampCol) {
                    sql += ` ORDER BY ${timestampCol} DESC`;
                } else if (idCol) {
                    sql += ` ORDER BY ${idCol} DESC`;
                }

                // Apply limit/offset
                const limit = request.limit || 100;
                sql += ` LIMIT ${limit}`;
                
                if (request.offset) {
                    sql += ` OFFSET ${request.offset}`;
                }

                const result = db.exec(sql, params);
                if (result.length > 0) {
                    const columns = result[0].columns;
                    events = result[0].values.map((row: any[]) => {
                        const obj: any = {};
                        columns.forEach((col: string, idx: number) => {
                            obj[col] = row[idx];
                        });
                        return obj;
                    });
                    break;
                }
            } catch {
                // Table doesn't exist or query failed, try next
                continue;
            }
        }

        // Map to EventLogEntry using shared parser
        return events.map((row: any) => parseEventRow(row));

    } catch (err) {
        console.error('[EventLogger] Query error:', err);
        status.lastError = (err as Error).message;
        return [];
    }
}

/**
 * Get latest event ID
 */
function getLatestEventId(): number {
    if (!db) return 0;
    
    // 表名安全验证正则
    const VALID_TABLE_PATTERN = /^[a-zA-Z_][a-zA-Z0-9_]*$/;
    
    try {
        const tablesResult = db.exec(
            "SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'"
        );
        
        if (tablesResult.length === 0) return 0;
        
        const tableNames = tablesResult[0].values.map((row: any) => row[0]);
        
        for (const tableName of tableNames) {
            // 安全检查：验证表名格式
            if (!VALID_TABLE_PATTERN.test(tableName)) {
                continue;
            }
            
            try {
                const columnsResult = db.exec(`PRAGMA table_info(${tableName})`);
                if (columnsResult.length === 0) continue;
                
                const columnNames = columnsResult[0].values.map((row: any) => row[1].toLowerCase());
                const idCol = columnNames.find(c => c === 'id' || c === 'rowid');
                
                if (idCol) {
                    const result = db.exec(`SELECT MAX(${idCol}) as maxId FROM ${tableName}`);
                    if (result.length > 0 && result[0].values.length > 0) {
                        const val = result[0].values[0][0];
                        return typeof val === 'number' ? val : 0;
                    }
                }
            } catch {
                continue;
            }
        }
    } catch {
        // Ignore
    }
    
    return 0;
}

/**
 * Get new events since last check
 */
function getNewEvents(): EventLogEntry[] {
    const latestId = getLatestEventId();
    
    if (latestId <= lastEventId) {
        return [];
    }

    // Query events with ID > lastEventId
    if (!db) return [];
    
    // 表名安全验证正则
    const VALID_TABLE_PATTERN = /^[a-zA-Z_][a-zA-Z0-9_]*$/;
    
    try {
        const tablesResult = db.exec(
            "SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'"
        );
        
        if (tablesResult.length === 0) return [];
        
        const tableNames = tablesResult[0].values.map((row: any) => row[0]);
        
        for (const tableName of tableNames) {
            // 安全检查：验证表名格式
            if (!VALID_TABLE_PATTERN.test(tableName)) {
                continue;
            }
            
            try {
                const columnsResult = db.exec(`PRAGMA table_info(${tableName})`);
                if (columnsResult.length === 0) continue;
                
                const columnNames = columnsResult[0].values.map((row: any) => row[1].toLowerCase());
                const idCol = columnNames.find(c => c === 'id' || c === 'rowid');
                
                if (idCol) {
                    const result = db.exec(
                        `SELECT * FROM ${tableName} WHERE ${idCol} > ? ORDER BY ${idCol} ASC`,
                        [lastEventId]
                    );
                    
                    if (result.length > 0) {
                        const columns = result[0].columns;
                        const events = result[0].values.map((row: any[]) => {
                            const obj: any = {};
                            columns.forEach((col: string, idx: number) => {
                                obj[col] = row[idx];
                            });
                            return obj;
                        });
                        
                        lastEventId = latestId;
                        
                        // Use shared parser instead of inline parsing
                        return events.map((row: any) => parseEventRow(row));
                    }
                }
            } catch {
                continue;
            }
        }
    } catch {
        // Ignore
    }
    
    return [];
}

/**
 * Check if event matches rule
 */
function eventMatchesRule(event: EventLogEntry, rule: EventLoggerNotificationRule): boolean {
    // Check severity
    if (SEVERITY_ORDER[event.severity] < SEVERITY_ORDER[rule.severity]) {
        return false;
    }

    // Check event types
    if (rule.eventTypes.length > 0 && !rule.eventTypes.includes('*')) {
        if (!rule.eventTypes.some(t => 
            event.eventType.toLowerCase().includes(t.toLowerCase()) || t === '*'
        )) {
            return false;
        }
    }

    // Check sources
    if (rule.sources && rule.sources.length > 0) {
        if (!rule.sources.some(s => 
            event.source.toLowerCase().includes(s.toLowerCase())
        )) {
            return false;
        }
    }

    // Check exclude sources
    if (rule.excludeSources && rule.excludeSources.length > 0) {
        if (rule.excludeSources.some(s => 
            event.source.toLowerCase().includes(s.toLowerCase())
        )) {
            return false;
        }
    }

    // Check keywords - 同时检查消息内容和 eventCode
    if (rule.keywords && rule.keywords.length > 0) {
        const searchText = (event.message + ' ' + (event.eventCode || '')).toLowerCase();
        if (!rule.keywords.some(k => searchText.includes(k.toLowerCase()))) {
            return false;
        }
    }

    // Check exclude keywords - 同时检查消息内容和 eventCode
    if (rule.excludeKeywords && rule.excludeKeywords.length > 0) {
        const searchText = (event.message + ' ' + (event.eventCode || '')).toLowerCase();
        if (rule.excludeKeywords.some(k => searchText.includes(k.toLowerCase()))) {
            return false;
        }
    }

    return true;
}

/**
 * Check if in quiet hours
 */
function isInQuietHours(rule: EventLoggerNotificationRule): boolean {
    if (!rule.quietHoursStart || !rule.quietHoursEnd) {
        return false;
    }

    const now = new Date();
    const currentTime = now.getHours() * 60 + now.getMinutes();

    const parseTime = (timeStr: string): number => {
        const [hours, minutes] = timeStr.split(':').map(Number);
        return hours * 60 + minutes;
    };

    const startTime = parseTime(rule.quietHoursStart);
    const endTime = parseTime(rule.quietHoursEnd);

    if (startTime > endTime) {
        return currentTime >= startTime || currentTime < endTime;
    } else {
        return currentTime >= startTime && currentTime < endTime;
    }
}

// Cooldown tracking
const cooldownTracker = new Map<string, number>();

/**
 * Check if can send notification (rate limiting)
 */
function canSendNotification(rule: EventLoggerNotificationRule): boolean {
    const now = Date.now();
    const lastSent = cooldownTracker.get(rule.id);
    
    if (lastSent && (now - lastSent) < rule.cooldown * 1000) {
        return false;
    }

    return true;
}

/**
 * Send notification for event
 */
async function sendEventNotification(
    event: EventLogEntry,
    rule: EventLoggerNotificationRule
): Promise<void> {
    // 从消息中提取应用名称，如 "应用 日志管理 启用成功" -> "日志管理"
    let appName = '';
    const appNameMatch = event.message.match(/应用\s+(\S+?)(?:\s+|$)/);
    if (appNameMatch) {
        appName = appNameMatch[1];
    } else if (event.source && event.source !== 'unknown') {
        // 尝试从来源提取，如 trim.app-center -> app-center
        appName = event.source.replace(/^.*\./, '');
    }
    
    const title = `飞牛系统通知`;
    const formattedTime = formatTimestampToLocal(event.timestamp);
    const content = `来源: ${event.source}\n类型: ${event.eventType}\n级别: ${event.severity.toUpperCase()}\n时间: ${formattedTime}\n\n消息:\n${event.message}`;

    // Get channel config
    const channels = notificationStore.getChannels();
    const enabledChannels = channels.filter(c => 
        rule.channels.includes(c.name) && c.enabled
    );

    for (const channel of enabledChannels) {
        try {
            const result = await notificationService.sendToChannel(channel, title, content);
            
            // 记录历史
            const historyRecord = {
                id: `${Date.now()}-${Math.random().toString(36).substr(2, 9)}`,
                ruleId: rule.id,
                ruleName: rule.name,
                channel: channel.channel,
                title,
                content,
                appName: appName || '系统事件',
                logPath: '',
                matchedLine: event.message,
                success: result.success,
                error: result.error,
                timestamp: new Date()
            };
            
            await notificationStore.addHistory(historyRecord);
            
            if (result.success) {
                console.log(`[EventLogger] Notification sent: ${channel.name}`);
            } else {
                console.error(`[EventLogger] Notification failed: ${result.error}`);
            }
        } catch (err) {
            console.error(`[EventLogger] Notification error:`, err);
        }
    }

    // 更新规则触发信息
    if (rule.id) {
        await notificationStore.updateRuleTrigger(rule.id);
        notificationStore.recordNotification(rule.id);
    }

    // Update cooldown
    cooldownTracker.set(rule.id, Date.now());
}

/**
 * Process new events
 */
async function processEvents(): Promise<void> {
    const checkStartTime = Date.now();
    
    try {
        if (!status.dbAccessible) {
            await reconnect();
            if (!status.dbAccessible) {
                console.warn('[EventLogger] Database not accessible, will retry next check');
                status.lastError = 'Database not accessible';
                return;
            }
        }

        // 每次检查前重新加载数据库文件，确保获取最新数据
        await reconnect();

        status.lastCheckTime = new Date();
        status.lastError = null;

        const newEvents = getNewEvents();
        
        for (const event of newEvents) {
            status.totalEventsProcessed++;
            status.lastEventTime = new Date(event.timestamp);

            // 去重：检查事件是否已通知过
            if (notifiedEvents.has(event.id)) {
                continue;
            }

            // Check each rule
            for (const rule of rules) {
                if (!rule.enabled) continue;
                if (!canSendNotification(rule)) continue;
                if (isInQuietHours(rule)) continue;

                if (eventMatchesRule(event, rule)) {
                    await sendEventNotification(event, rule);
                    // 标记事件已通知
                    notifiedEvents.add(event.id);
                    console.log(`[EventLogger] Event matched rule "${rule.name}":`, event.message.substring(0, 100));
                }
            }
        }

        // Reload rules in case they changed
        rules = loadRules();

        const checkDuration = Date.now() - checkStartTime;
        if (checkDuration > 1000) {
            console.log(`[EventLogger] Check completed in ${checkDuration}ms, processed ${newEvents.length} events`);
        }

    } catch (err) {
        status.lastError = (err as Error).message;
        console.error('[EventLogger] Process error:', err);
        // 数据库可能已损坏，标记为不可访问以便下次重连
        status.dbAccessible = false;
    }
}

/**
 * Start monitoring
 */
export async function start(): Promise<void> {
    if (isRunning) {
        console.log('[EventLogger] Already running');
        return;
    }

    if (!status.dbAccessible) {
        await testConnection();
        if (!status.dbAccessible) {
            console.error('[EventLogger] Cannot start: database not accessible');
            return;
        }
    }

    // 初始化 lastEventId 为当前数据库中的最大事件 ID
    // 这样可以避免处理所有历史事件，只处理启动后的新事件
    const currentMaxId = getLatestEventId();
    lastEventId = currentMaxId;
    console.log(`[EventLogger] Initialized lastEventId to current max: ${lastEventId}`);

    isRunning = true;
    status.isRunning = true;

    console.log(`[EventLogger] Starting monitor, interval: ${configData.checkInterval}ms`);

    // Set up interval with error handling
    checkInterval = setInterval(async () => {
        try {
            await processEvents();
        } catch (err) {
            console.error('[EventLogger] Interval check error:', err);
        }
    }, configData.checkInterval);
    
    console.log('[EventLogger] Monitor started successfully');
}

/**
 * Stop monitoring
 */
export function stop(): void {
    if (!isRunning) return;

    isRunning = false;
    status.isRunning = false;

    if (checkInterval) {
        clearInterval(checkInterval);
        checkInterval = null;
    }

    closeDb();
    console.log('[EventLogger] Stopped');
}

/**
 * Restart monitoring
 */
export async function restart(): Promise<void> {
    stop();
    await start();
}

/**
 * Get current status
 */
export function getStatus(): EventLoggerStatus {
    return { ...status };
}

/**
 * Get statistics
 */
export function getStats(): EventLoggerStats {
    if (!status.dbAccessible) {
        return {
            totalEvents: 0,
            eventsBySeverity: {} as Record<EventSeverity, number>,
            eventsBySource: {},
            eventsByType: {},
            timeRange: { earliest: null, latest: null }
        };
    }

    try {
        const events = queryEvents({ limit: 1000 });
        
        const stats: EventLoggerStats = {
            totalEvents: status.totalEventsProcessed || events.length,
            eventsBySeverity: {} as Record<EventSeverity, number>,
            eventsBySource: {},
            eventsByType: {},
            timeRange: {
                earliest: events.length > 0 ? events[events.length - 1].timestamp : null,
                latest: events.length > 0 ? events[0].timestamp : null
            }
        };

        for (const event of events) {
            // By severity
            stats.eventsBySeverity[event.severity] = 
                (stats.eventsBySeverity[event.severity] || 0) + 1;
            
            // By source
            stats.eventsBySource[event.source] = 
                (stats.eventsBySource[event.source] || 0) + 1;
            
            // By type
            stats.eventsByType[event.eventType] = 
                (stats.eventsByType[event.eventType] || 0) + 1;
        }

        return stats;
    } catch {
        return {
            totalEvents: 0,
            eventsBySeverity: {} as Record<EventSeverity, number>,
            eventsBySource: {},
            eventsByType: {},
            timeRange: { earliest: null, latest: null }
        };
    }
}

/**
 * Get events with filtering
 */
export function getEvents(request: GetEventsRequest): GetEventsResponse {
    if (!status.dbAccessible) {
        return { events: [], total: 0, hasMore: false };
    }

    try {
        const events = queryEvents(request);
        const limit = request.limit || 100;
        
        return {
            events,
            total: events.length,
            hasMore: events.length >= limit
        };
    } catch {
        return { events: [], total: 0, hasMore: false };
    }
}

/**
 * Update configuration
 */
export async function updateConfig(updates: Partial<EventLoggerConfig>): Promise<void> {
    configData = { ...configData, ...updates };
    status.dbPath = configData.dbPath;
    
    // If enabling, start monitoring
    if (configData.enabled && !isRunning) {
        await start();
    } else if (!configData.enabled && isRunning) {
        stop();
    } else if (isRunning) {
        // Restart with new config
        await restart();
    }
}

/**
 * Get current config
 */
export function getConfig(): EventLoggerConfig {
    return { ...configData };
}

/**
 * Force check (manual trigger)
 */
export async function forceCheck(): Promise<void> {
    await processEvents();
}

/**
 * Get available sources (app names) from database
 */
export function getSources(): string[] {
    if (!status.dbAccessible) {
        return [];
    }

    try {
        // 重新加载数据库以获取最新数据
        reconnect();
        
        if (!db) return [];

        const tablesResult = db.exec(
            "SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'"
        );
        
        if (tablesResult.length === 0) return [];

        const tableNames = tablesResult[0].values.map((row: any) => row[0]);
        const sources = new Set<string>();
        
        const VALID_TABLE_PATTERN = /^[a-zA-Z_][a-zA-Z0-9_]*$/;
        
        for (const tableName of tableNames) {
            if (!VALID_TABLE_PATTERN.test(tableName)) continue;
            
            try {
                const columnsResult = db.exec(`PRAGMA table_info(${tableName})`);
                if (columnsResult.length === 0) continue;
                
                const columnNames = columnsResult[0].values.map((row: any) => row[1].toLowerCase());
                const sourceCol = columnNames.find(c => 
                    c.includes('source') || c.includes('app') || c.includes('name') || 
                    c === 'serviceid' || c === 'service' || c === 'program'
                );
                
                if (sourceCol) {
                    const result = db.exec(`SELECT DISTINCT ${sourceCol} FROM ${tableName} LIMIT 100`);
                    if (result.length > 0) {
                        for (const row of result[0].values) {
                            if (row[0] && typeof row[0] === 'string' && row[0] !== 'null') {
                                sources.add(row[0]);
                            }
                        }
                    }
                }
            } catch {
                continue;
            }
        }
        
        return Array.from(sources).sort();
    } catch (err) {
        console.error('[EventLogger] Get sources error:', err);
        return [];
    }
}

/**
 * Dispose service
 */
export function dispose(): void {
    stop();
    cooldownTracker.clear();
}
