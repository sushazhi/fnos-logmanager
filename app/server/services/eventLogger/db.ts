/**
 * EventLogger 数据库操作
 */
import initSqlJs, { Database as SqlJsDatabase } from 'sql.js';
import fs from 'fs';
import Logger from '../../utils/logger';
import { EventLogEntry, GetEventsRequest, EventSeverity, SEVERITY_ORDER } from './types';

const logger = Logger.child({ module: 'EventLoggerDb' });

let SQL: any = null;
let db: SqlJsDatabase | null = null;
let dbPath: string = '';

/**
 * 初始化 SQL.js
 */
async function initSql(): Promise<void> {
    if (!SQL) {
        SQL = await initSqlJs();
    }
}

/**
 * 加载数据库
 */
export function loadDatabase(path: string): SqlJsDatabase | null {
    dbPath = path;

    try {
        if (!fs.existsSync(path)) {
            logger.warn({ path }, 'Database file not found');
            return null;
        }

        const fileBuffer = fs.readFileSync(path);
        if (!SQL) {
            logger.error('SQL.js not initialized');
            return null;
        }

        db = new SQL.Database(fileBuffer);
        logger.info({ path }, 'Database loaded');
        return db;
    } catch (err) {
        logger.error({ err, path }, 'Failed to load database');
        return null;
    }
}

/**
 * 关闭数据库
 */
export function closeDb(): void {
    if (db) {
        try {
            db.close();
            db = null;
            logger.info('Database closed');
        } catch (err) {
            logger.error({ err }, 'Failed to close database');
        }
    }
}

/**
 * 重新连接数据库
 */
export async function reconnect(): Promise<void> {
    closeDb();
    await initSql();
    if (dbPath) {
        loadDatabase(dbPath);
    }
}

/**
 * 获取数据库实例
 */
export function getDb(): SqlJsDatabase | null {
    return db;
}

/**
 * 检查数据库是否可用
 */
export function isDbAccessible(): boolean {
    return db !== null;
}

/**
 * 映射日志级别到严重程度
 */
export function mapLogLevelToSeverity(level: string): EventSeverity {
    const levelLower = level.toLowerCase();
    if (levelLower.includes('debug') || levelLower.includes('trace')) return 'debug';
    if (levelLower.includes('info')) return 'info';
    if (levelLower.includes('warn')) return 'warning';
    if (levelLower.includes('error')) return 'error';
    if (levelLower.includes('fatal') || levelLower.includes('critical')) return 'critical';
    return 'info';
}

/**
 * 解析严重程度
 */
export function parseSeverity(severity: string | number): EventSeverity {
    if (typeof severity === 'number') {
        if (severity <= 0) return 'debug';
        if (severity === 1) return 'info';
        if (severity === 2) return 'warning';
        if (severity === 3) return 'error';
        return 'critical';
    }

    const sev = severity.toLowerCase();
    if (sev in SEVERITY_ORDER) {
        return sev as EventSeverity;
    }
    return 'info';
}

/**
 * 查询事件
 */
export function queryEvents(request: GetEventsRequest): EventLogEntry[] {
    if (!db) {
        return [];
    }

    const {
        limit = 100,
        offset = 0,
        severity,
        source,
        template,
        search,
        startTime,
        endTime,
        sortDirection = 'desc'
    } = request;

    try {
        let query = 'SELECT * FROM logs WHERE 1=1';
        const params: (string | number)[] = [];

        // 严重程度过滤
        if (severity) {
            query += ' AND (severity = ? OR level = ?)';
            params.push(severity, severity);
        }

        // 来源过滤
        if (source) {
            query += ' AND source = ?';
            params.push(source);
        }

        // 模板过滤
        if (template) {
            query += ' AND template = ?';
            params.push(template);
        }

        // 时间范围
        if (startTime) {
            query += ' AND timestamp >= ?';
            params.push(startTime);
        }
        if (endTime) {
            query += ' AND timestamp <= ?';
            params.push(endTime);
        }

        // 搜索
        if (search) {
            query += ' AND (message LIKE ? OR param LIKE ? OR template LIKE ?)';
            const searchPattern = `%${search}%`;
            params.push(searchPattern, searchPattern, searchPattern);
        }

        // 排序
        query += ` ORDER BY id ${sortDirection.toUpperCase()}`;

        // 分页
        query += ' LIMIT ? OFFSET ?';
        params.push(limit, offset);

        const results = db.exec(query, params);

        if (results.length === 0) {
            return [];
        }

        const columns = results[0].columns;
        const rows = results[0].values;

        return rows.map(row => {
            const entry: EventLogEntry = { id: 0, timestamp: 0 };
            columns.forEach((col, idx) => {
                entry[col] = row[idx];
            });
            return entry;
        });
    } catch (err) {
        logger.error({ err }, 'Query events failed');
        return [];
    }
}

/**
 * 获取最新事件 ID
 */
export function getLatestEventId(): number {
    if (!db) {
        return 0;
    }

    try {
        const results = db.exec('SELECT MAX(id) as maxId FROM logs');
        if (results.length > 0 && results[0].values.length > 0) {
            return (results[0].values[0][0] as number) || 0;
        }
        return 0;
    } catch (err) {
        logger.error({ err }, 'Get latest event ID failed');
        return 0;
    }
}

/**
 * 获取新事件
 */
export function getNewEvents(sinceId: number): EventLogEntry[] {
    if (!db) {
        return [];
    }

    try {
        const query = 'SELECT * FROM logs WHERE id > ? ORDER BY id ASC LIMIT 100';
        const results = db.exec(query, [sinceId]);

        if (results.length === 0) {
            return [];
        }

        const columns = results[0].columns;
        const rows = results[0].values;

        return rows.map(row => {
            const entry: EventLogEntry = { id: 0, timestamp: 0 };
            columns.forEach((col, idx) => {
                entry[col] = row[idx];
            });
            return entry;
        });
    } catch (err) {
        logger.error({ err }, 'Get new events failed');
        return [];
    }
}

/**
 * 获取事件总数
 */
export function getTotalEvents(): number {
    if (!db) {
        return 0;
    }

    try {
        const results = db.exec('SELECT COUNT(*) as count FROM logs');
        if (results.length > 0 && results[0].values.length > 0) {
            return (results[0].values[0][0] as number) || 0;
        }
        return 0;
    } catch (err) {
        logger.error({ err }, 'Get total events failed');
        return 0;
    }
}

/**
 * 获取所有来源
 */
export function getSources(): string[] {
    if (!db) {
        return [];
    }

    try {
        const results = db.exec('SELECT DISTINCT source FROM logs WHERE source IS NOT NULL');
        if (results.length > 0) {
            return results[0].values.map(row => row[0] as string);
        }
        return [];
    } catch (err) {
        logger.error({ err }, 'Get sources failed');
        return [];
    }
}

// 导出初始化函数
export { initSql };
