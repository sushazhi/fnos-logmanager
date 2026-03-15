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

let SQL: any = null;
let db: SqlJsDatabase | null = null;
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

// Severity order for comparison
const SEVERITY_ORDER: Record<EventSeverity, number> = {
    debug: 0,
    info: 1,
    warning: 2,
    error: 3,
    critical: 4
};

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
 */
async function reconnect(): Promise<void> {
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
            sources: r.appName === '*' ? undefined : [r.appName],
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

/**
 * Map severity string to EventSeverity
 */
function parseSeverity(severity: string | number): EventSeverity {
    const s = String(severity).toLowerCase();
    if (s.includes('crit') || s.includes('emerg') || s.includes('panic')) return 'critical';
    if (s.includes('err')) return 'error';
    if (s.includes('warn')) return 'warning';
    if (s.includes('debug') || s.includes('trace')) return 'debug';
    return 'info';
}

/**
 * Parse numeric loglevel to severity
 * Based on common logging levels:
 * 0 = debug/trace
 * 1 = info
 * 2 = warning
 * 3 = error
 * 4+ = critical
 */
function parseLoglevel(level: string | number): EventSeverity {
    if (typeof level === 'number') {
        // loglevel: 0 = 普通(info), 1 = 警告, 2 = 错误, 3 = 严重
        switch (level) {
            case 0: return 'info';   // 普通
            case 1: return 'warning'; // 警告
            case 2: return 'error';  // 错误
            case 3: return 'critical'; // 严重
            default: return level >= 4 ? 'critical' : 'info';
        }
    }
    return parseSeverity(level);
}

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
        
        for (const tableName of tableNames) {
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

        // Map to EventLogEntry
        return events.map((row: any) => {
            const keys = Object.keys(row);
            
            // 尝试匹配实际字段名
            const timestampKey = keys.find(k => 
                k.toLowerCase() === 'logtime' || k.toLowerCase() === 'timestamp' || 
                k.toLowerCase().includes('time') || k.toLowerCase().includes('date')
            );
            const sourceKey = keys.find(k => 
                k.toLowerCase() === 'serviceid' || k.toLowerCase() === 'service' ||
                k.toLowerCase().includes('source') || k.toLowerCase().includes('app')
            );
            const messageKey = keys.find(k => 
                k.toLowerCase() === 'message' || k.toLowerCase() === 'msg' || 
                k.toLowerCase() === 'content' || k.toLowerCase().includes('description') ||
                k.toLowerCase() === 'log'
            );
            const severityKey = keys.find(k => 
                k.toLowerCase() === 'loglevel' || k.toLowerCase() === 'level' || 
                k.toLowerCase().includes('severity') || k.toLowerCase().includes('priority')
            );
            const typeKey = keys.find(k => 
                k.toLowerCase() === 'type' || k.toLowerCase() === 'category' ||
                k.toLowerCase().includes('event')
            );
            const idKey = keys.find(k => k.toLowerCase() === 'id');
            const metadataKey = keys.find(k => 
                k.toLowerCase().includes('metadata') || k.toLowerCase().includes('extra')
            );
            const userKey = keys.find(k => 
                k.toLowerCase() === 'uid' || k.toLowerCase() === 'uname' ||
                k.toLowerCase().includes('user')
            );

            // 处理时间戳 - 可能是Unix时间戳
            let timestampValue = row[timestampKey || 'timestamp'];
            if (typeof timestampValue === 'number' && timestampValue > 1000000000) {
                // Unix时间戳（秒）- 转换为本地时间字符串
                const date = new Date(timestampValue * 1000);
                timestampValue = date.getFullYear() + '-' + 
                    String(date.getMonth() + 1).padStart(2, '0') + '-' + 
                    String(date.getDate()).padStart(2, '0') + ' ' + 
                    String(date.getHours()).padStart(2, '0') + ':' + 
                    String(date.getMinutes()).padStart(2, '0') + ':' + 
                    String(date.getSeconds()).padStart(2, '0');
            } else if (typeof timestampValue === 'number' && timestampValue > 1000000000000) {
                // Unix时间戳（毫秒）
                const date = new Date(timestampValue);
                timestampValue = date.getFullYear() + '-' + 
                    String(date.getMonth() + 1).padStart(2, '0') + '-' + 
                    String(date.getDate()).padStart(2, '0') + ' ' + 
                    String(date.getHours()).padStart(2, '0') + ':' + 
                    String(date.getMinutes()).padStart(2, '0') + ':' + 
                    String(date.getSeconds()).padStart(2, '0');
            }

            // 处理消息 - 可能是剩余的所有字段
            let message = row[messageKey || 'message'];
            if (!message || message === 'null') {
                // 尝试从 parameter 字段提取有用信息
                const parameterStr = row['parameter'];
                if (parameterStr && parameterStr !== 'null') {
                    try {
                        const param = typeof parameterStr === 'string' ? JSON.parse(parameterStr) : parameterStr;
                        if (param.data) {
                            const appName = param.data.APP_NAME;
                            const displayName = param.data.DISPLAY_NAME;
                            const eventId = param.eventId;
                            
                            // 生成中文消息
                            const actionMap: Record<string, string> = {
                                'APP_STARTED': '启用成功',
                                'APP_STOPPED': '停止成功',
                                'APP_INSTALLED': '安装成功',
                                'APP_UNINSTALLED': '卸载成功',
                                'APP_UPDATED': '更新成功',
                                'APP_UPGRADED': '升级成功'
                            };
                            
                            const appNameCn = displayName || appName || '';
                            const action = eventId ? (actionMap[eventId] || eventId) : '';
                            
                            if (appNameCn && action) {
                                message = `应用 ${appNameCn} ${action}`;
                            } else if (appNameCn) {
                                message = appNameCn;
                            } else if (eventId) {
                                message = eventId;
                            }
                        }
                        // 如果解析失败，使用原始字符串
                        if (!message) {
                            message = parameterStr.substring(0, 500);
                        }
                    } catch {
                        // JSON解析失败，使用原始值
                        message = String(parameterStr).substring(0, 500);
                    }
                }
            }
            
            // 如果还是没有消息，收集其他字段
            if (!message || message === 'null') {
                const excludedKeys = [idKey, timestampKey, sourceKey, severityKey, typeKey, metadataKey, userKey, 'parameter'];
                const otherFields = keys.filter(k => !excludedKeys.includes(k));
                message = otherFields.map(k => `${k}: ${row[k]}`).join(', ').substring(0, 500);
            }

            // 处理用户
            let userValue = row[userKey || 'user'];
            if (userValue === null || userValue === 'null') {
                userValue = undefined;
            }

            return {
                id: row[idKey || 'id'] || 0,
                timestamp: timestampValue || new Date().toISOString(),
                source: row[sourceKey || 'source'] || 'unknown',
                eventType: row[typeKey || 'event_type'] || 'system',
                severity: parseLoglevel(row[severityKey || 'level']),
                message: message,
                metadata: row[metadataKey || 'metadata'],
                user: userValue
            } as EventLogEntry;
        });

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
    
    try {
        const tablesResult = db.exec(
            "SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'"
        );
        
        if (tablesResult.length === 0) return 0;
        
        const tableNames = tablesResult[0].values.map((row: any) => row[0]);
        
        for (const tableName of tableNames) {
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
    
    try {
        const tablesResult = db.exec(
            "SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'"
        );
        
        if (tablesResult.length === 0) return [];
        
        const tableNames = tablesResult[0].values.map((row: any) => row[0]);
        
        for (const tableName of tableNames) {
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
                        
                        return events.map((row: any) => {
                            const keys = Object.keys(row);
                            
                            // 尝试匹配实际字段名
                            const timestampKey = keys.find(k => 
                                k.toLowerCase() === 'logtime' || k.toLowerCase() === 'timestamp' || 
                                k.toLowerCase().includes('time') || k.toLowerCase().includes('date')
                            );
                            const sourceKey = keys.find(k => 
                                k.toLowerCase() === 'serviceid' || k.toLowerCase() === 'service' ||
                                k.toLowerCase().includes('source') || k.toLowerCase().includes('app')
                            );
                            const messageKey = keys.find(k => 
                                k.toLowerCase() === 'message' || k.toLowerCase() === 'msg' || 
                                k.toLowerCase() === 'content' || k.toLowerCase().includes('description') ||
                                k.toLowerCase() === 'log'
                            );
                            const severityKey = keys.find(k => 
                                k.toLowerCase() === 'loglevel' || k.toLowerCase() === 'level' || 
                                k.toLowerCase().includes('severity') || k.toLowerCase().includes('priority')
                            );
                            const typeKey = keys.find(k => 
                                k.toLowerCase() === 'type' || k.toLowerCase() === 'category' ||
                                k.toLowerCase().includes('event')
                            );
                            const idKey = keys.find(k => k.toLowerCase() === 'id');
                            const metadataKey = keys.find(k => 
                                k.toLowerCase().includes('metadata') || k.toLowerCase().includes('extra')
                            );
                            const userKey = keys.find(k => 
                                k.toLowerCase() === 'uid' || k.toLowerCase() === 'uname' ||
                                k.toLowerCase().includes('user')
                            );

                            // 处理时间戳
                            let timestampValue = row[timestampKey || 'timestamp'];
                            if (typeof timestampValue === 'number' && timestampValue > 1000000000) {
                                // Unix时间戳（秒）- 转换为本地时间字符串
                                const date = new Date(timestampValue * 1000);
                                timestampValue = date.getFullYear() + '-' + 
                                    String(date.getMonth() + 1).padStart(2, '0') + '-' + 
                                    String(date.getDate()).padStart(2, '0') + ' ' + 
                                    String(date.getHours()).padStart(2, '0') + ':' + 
                                    String(date.getMinutes()).padStart(2, '0') + ':' + 
                                    String(date.getSeconds()).padStart(2, '0');
                            } else if (typeof timestampValue === 'number' && timestampValue > 1000000000000) {
                                // Unix时间戳（毫秒）
                                const date = new Date(timestampValue);
                                timestampValue = date.getFullYear() + '-' + 
                                    String(date.getMonth() + 1).padStart(2, '0') + '-' + 
                                    String(date.getDate()).padStart(2, '0') + ' ' + 
                                    String(date.getHours()).padStart(2, '0') + ':' + 
                                    String(date.getMinutes()).padStart(2, '0') + ':' + 
                                    String(date.getSeconds()).padStart(2, '0');
                            }

            // 处理消息
            let message = row[messageKey || 'message'];
            let eventIdValue = '';
            if (!message || message === 'null') {
                // 尝试从 parameter 字段提取有用信息
                const parameterStr = row['parameter'];
                if (parameterStr && parameterStr !== 'null') {
                    try {
                        const param = typeof parameterStr === 'string' ? JSON.parse(parameterStr) : parameterStr;
                        if (param.data) {
                            const appName = param.data.APP_NAME;
                            const displayName = param.data.DISPLAY_NAME;
                            const eventId = param.eventId;
                            eventIdValue = eventId || '';
                            
                            // 生成中文消息
                            const actionMap: Record<string, string> = {
                                'APP_STARTED': '启用成功',
                                'APP_STOPPED': '停止成功',
                                'APP_INSTALLED': '安装成功',
                                'APP_UNINSTALLED': '卸载成功',
                                'APP_UPDATED': '更新成功',
                                'APP_UPGRADED': '升级成功'
                            };
                            
                            const appNameCn = displayName || appName || '';
                            const action = eventId ? (actionMap[eventId] || eventId) : '';
                            
                            if (appNameCn && action) {
                                message = `应用 ${appNameCn} ${action}`;
                            } else if (appNameCn) {
                                message = appNameCn;
                            } else if (eventId) {
                                message = eventId;
                            }
                        }
                        // 如果解析失败，使用原始字符串
                        if (!message) {
                            message = parameterStr.substring(0, 500);
                        }
                    } catch {
                        // JSON解析失败，使用原始值
                        message = String(parameterStr).substring(0, 500);
                    }
                }
            }
            
            // 如果还是没有消息，收集其他字段
            if (!message || message === 'null') {
                const excludedKeys = [idKey, timestampKey, sourceKey, severityKey, typeKey, metadataKey, userKey, 'parameter'];
                const otherFields = keys.filter(k => !excludedKeys.includes(k));
                message = otherFields.map(k => `${k}: ${row[k]}`).join(', ').substring(0, 500);
            }

            // 处理用户
            let userValue = row[userKey || 'user'];
            if (userValue === null || userValue === 'null') {
                userValue = undefined;
            }

            return {
                id: row[idKey || 'id'] || 0,
                timestamp: timestampValue || new Date().toISOString(),
                source: row[sourceKey || 'source'] || 'unknown',
                eventType: row[typeKey || 'event_type'] || 'system',
                severity: parseLoglevel(row[severityKey || 'level']),
                message: message,
                metadata: row[metadataKey || 'metadata'],
                user: userValue,
                eventCode: eventIdValue
            } as EventLogEntry;
                        });
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
    const content = `来源: ${event.source}\n类型: ${event.eventType}\n级别: ${event.severity.toUpperCase()}\n时间: ${event.timestamp}\n\n消息:\n${event.message}`;

    // Get channel config
    const channels = notificationStore.getChannels();
    const enabledChannels = channels.filter(c => 
        rule.channels.includes(c.name) && c.enabled
    );

    for (const channel of enabledChannels) {
        try {
            const result = await notificationService.sendToChannel(channel, title, content);
            
            if (result.success) {
                console.log(`[EventLogger] Notification sent: ${channel.name}`);
            } else {
                console.error(`[EventLogger] Notification failed: ${result.error}`);
            }
        } catch (err) {
            console.error(`[EventLogger] Notification error:`, err);
        }
    }

    // Update cooldown
    cooldownTracker.set(rule.id, Date.now());
}

/**
 * Process new events
 */
async function processEvents(): Promise<void> {
    if (!status.dbAccessible) {
        await reconnect();
        if (!status.dbAccessible) return;
    }

    status.lastCheckTime = new Date();
    status.lastError = null;

    try {
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

    } catch (err) {
        status.lastError = (err as Error).message;
        console.error('[EventLogger] Process error:', err);
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

    isRunning = true;
    status.isRunning = true;

    console.log(`[EventLogger] Starting monitor, interval: ${configData.checkInterval}ms`);

    // Initial check
    await processEvents();

    // Set up interval
    checkInterval = setInterval(processEvents, configData.checkInterval);
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
 * Dispose service
 */
export function dispose(): void {
    stop();
    cooldownTracker.clear();
}
