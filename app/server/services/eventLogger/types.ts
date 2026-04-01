/**
 * EventLogger 类型定义
 */
export type EventSeverity = 'debug' | 'info' | 'warning' | 'error' | 'critical';

export interface EventLoggerConfig {
    dbPath: string;
    enabled: boolean;
    checkInterval: number;
    eventTypes: string[];
    minSeverity: EventSeverity;
    notificationChannels: string[];
}

export interface EventLoggerStatus {
    isRunning: boolean;
    lastCheckTime: Date | null;
    lastEventTime: Date | null;
    totalEventsProcessed: number;
    lastError: string | null;
    dbAccessible: boolean;
    dbPath: string;
}

export interface EventLogEntry {
    id: number;
    timestamp: number | string;
    template?: string;
    param?: string;
    severity?: string | number;
    level?: string | number;
    source?: string;
    message?: string;
    cat?: number;
    [key: string]: unknown;
}

export interface EventLoggerStats {
    totalEvents: number;
    eventsBySeverity: Record<EventSeverity, number>;
    eventsBySource: Record<string, number>;
    eventsByTemplate: Record<string, number>;
    recentEvents: EventLogEntry[];
}

export interface EventLoggerNotificationRule {
    id: string;
    name: string;
    enabled: boolean;
    eventTypes?: string[];
    templates?: string[];
    sources?: string[];
    minSeverity?: EventSeverity;
    keywords?: string[];
    excludeKeywords?: string[];
    cooldown?: number;
    quietHoursStart?: string;
    quietHoursEnd?: string;
    channels?: string[];
    createdAt: Date;
    updatedAt: Date;
}

export interface GetEventsRequest {
    limit?: number;
    offset?: number;
    severity?: EventSeverity;
    source?: string;
    template?: string;
    search?: string;
    startTime?: number;
    endTime?: number;
    sortDirection?: 'asc' | 'desc';
}

export interface GetEventsResponse {
    events: EventLogEntry[];
    total: number;
    hasMore: boolean;
}

export interface EventNotificationRequest {
    rule: EventLoggerNotificationRule;
}

// Severity order for comparison
export const SEVERITY_ORDER: Record<EventSeverity, number> = {
    debug: 0,
    info: 1,
    warning: 2,
    error: 3,
    critical: 4
};
