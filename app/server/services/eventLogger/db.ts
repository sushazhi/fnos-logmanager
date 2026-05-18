/**
 * EventLogger 数据库操作
 * 动态发现表名和列名，适配飞牛 eventlogger 数据库
 */
import initSqlJs, { Database as SqlJsDatabase } from 'sql.js';
import fs from 'fs';
import Logger from '../../utils/logger';
import { EventLogEntry, GetEventsRequest, EventSeverity, SEVERITY_ORDER } from './types';
import { parseEventRow } from './eventParser';

const logger = Logger.child({ module: 'EventLoggerDb' });

let SQL: any = null;
let db: SqlJsDatabase | null = null;
let dbPath: string = '';

const VALID_TABLE_PATTERN = /^[a-zA-Z_][a-zA-Z0-9_]*$/;

interface TableSchema {
    tableName: string;
    idCol: string | undefined;
    timestampCol: string | undefined;
    sourceCol: string | undefined;
    messageCol: string | undefined;
    severityCol: string | undefined;
    typeCol: string | undefined;
}

async function initSql(): Promise<void> {
    if (!SQL) {
        SQL = await initSqlJs();
    }
}

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

export async function reconnect(): Promise<void> {
    closeDb();
    await initSql();
    if (dbPath) {
        loadDatabase(dbPath);
    }
}

export function getDb(): SqlJsDatabase | null {
    return db;
}

export function isDbAccessible(): boolean {
    return db !== null;
}

export function mapLogLevelToSeverity(level: string): EventSeverity {
    const levelLower = level.toLowerCase();
    if (levelLower.includes('debug') || levelLower.includes('trace')) return 'debug';
    if (levelLower.includes('info')) return 'info';
    if (levelLower.includes('warn')) return 'warning';
    if (levelLower.includes('error')) return 'error';
    if (levelLower.includes('fatal') || levelLower.includes('critical')) return 'critical';
    return 'info';
}

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
 * 动态发现数据库中的表和列
 */
function discoverSchema(): TableSchema | null {
    if (!db) return null;

    try {
        const tablesResult = db.exec(
            "SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'"
        );
        if (tablesResult.length === 0) return null;

        const tableNames = tablesResult[0].values.map((row: any) => row[0]);

        for (const tableName of tableNames) {
            if (!VALID_TABLE_PATTERN.test(tableName)) continue;

            try {
                const columnsResult = db.exec(`PRAGMA table_info(${tableName})`);
                if (columnsResult.length === 0) continue;

                const columnNames = columnsResult[0].values.map((row: any) => row[1].toLowerCase());

                const idCol = columnNames.find(c => c === 'id' || c === 'rowid');
                const timestampCol = columnNames.find(c =>
                    c.includes('time') || c.includes('date') || c === 'created' || c === 'timestamp'
                );
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
                    c.includes('type') || c.includes('category') || c.includes('event') || c === 'template'
                );

                if (idCol) {
                    return { tableName, idCol, timestampCol, sourceCol, messageCol, severityCol, typeCol };
                }
            } catch {
                continue;
            }
        }
    } catch (err) {
        logger.error({ err }, 'Schema discovery failed');
    }

    return null;
}

export function queryEvents(request: GetEventsRequest): EventLogEntry[] {
    if (!db) return [];

    const schema = discoverSchema();
    if (!schema) return [];

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
        let sql = `SELECT * FROM ${schema.tableName}`;
        const conditions: string[] = [];
        const params: any[] = [];

        if (startTime && schema.timestampCol) {
            conditions.push(`${schema.timestampCol} >= ?`);
            params.push(startTime);
        }
        if (endTime && schema.timestampCol) {
            conditions.push(`${schema.timestampCol} <= ?`);
            params.push(endTime);
        }
        if (source && schema.sourceCol) {
            conditions.push(`${schema.sourceCol} LIKE ?`);
            params.push(`%${source}%`);
        }
        if (search && schema.messageCol) {
            conditions.push(`${schema.messageCol} LIKE ?`);
            params.push(`%${search}%`);
        }
        if (severity && schema.severityCol) {
            conditions.push(`${schema.severityCol} = ?`);
            params.push(severity);
        }
        if (template && schema.typeCol) {
            conditions.push(`${schema.typeCol} = ?`);
            params.push(template);
        }

        if (conditions.length > 0) {
            sql += ' WHERE ' + conditions.join(' AND ');
        }

        if (schema.timestampCol) {
            sql += ` ORDER BY ${schema.timestampCol} ${sortDirection === 'asc' ? 'ASC' : 'DESC'}`;
        } else if (schema.idCol) {
            sql += ` ORDER BY ${schema.idCol} ${sortDirection === 'asc' ? 'ASC' : 'DESC'}`;
        }

        sql += ' LIMIT ? OFFSET ?';
        params.push(limit, offset);

        const results = db.exec(sql, params);
        if (results.length === 0) return [];

        const columns = results[0].columns;
        return results[0].values.map(row => {
            const obj: any = {};
            columns.forEach((col, idx) => { obj[col] = row[idx]; });
            return parseEventRow(obj);
        });
    } catch (err) {
        logger.error({ err }, 'Query events failed');
        return [];
    }
}

export function getLatestEventId(): number {
    if (!db) return 0;

    const schema = discoverSchema();
    if (!schema || !schema.idCol) return 0;

    try {
        const result = db.exec(`SELECT MAX(${schema.idCol}) as maxId FROM ${schema.tableName}`);
        if (result.length > 0 && result[0].values.length > 0) {
            return (result[0].values[0][0] as number) || 0;
        }
        return 0;
    } catch (err) {
        logger.error({ err }, 'Get latest event ID failed');
        return 0;
    }
}

export function getNewEvents(sinceId: number): EventLogEntry[] {
    if (!db) return [];

    const schema = discoverSchema();
    if (!schema || !schema.idCol) return [];

    try {
        const sql = `SELECT * FROM ${schema.tableName} WHERE ${schema.idCol} > ? ORDER BY ${schema.idCol} ASC LIMIT 100`;
        const result = db.exec(sql, [sinceId]);
        if (result.length === 0) return [];

        const columns = result[0].columns;
        return result[0].values.map(row => {
            const obj: any = {};
            columns.forEach((col, idx) => { obj[col] = row[idx]; });
            return parseEventRow(obj);
        });
    } catch (err) {
        logger.error({ err }, 'Get new events failed');
        return [];
    }
}

export function getTotalEvents(): number {
    if (!db) return 0;

    const schema = discoverSchema();
    if (!schema) return 0;

    try {
        const result = db.exec(`SELECT COUNT(*) as count FROM ${schema.tableName}`);
        if (result.length > 0 && result[0].values.length > 0) {
            return (result[0].values[0][0] as number) || 0;
        }
        return 0;
    } catch (err) {
        logger.error({ err }, 'Get total events failed');
        return 0;
    }
}

export function getSources(): string[] {
    if (!db) return [];

    const schema = discoverSchema();
    if (!schema || !schema.sourceCol) return [];

    try {
        const result = db.exec(`SELECT DISTINCT ${schema.sourceCol} FROM ${schema.tableName} WHERE ${schema.sourceCol} IS NOT NULL`);
        if (result.length > 0) {
            return result[0].values.map(row => row[0] as string);
        }
        return [];
    } catch (err) {
        logger.error({ err }, 'Get sources failed');
        return [];
    }
}

export { initSql };
