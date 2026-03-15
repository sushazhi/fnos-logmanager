/**
 * Event Logger Service Types
 * Types for eventlogger_service database monitoring
 */

export interface EventLoggerConfig {
    /** Database file path */
    dbPath: string;
    /** Enable/disable monitoring */
    enabled: boolean;
    /** Check interval in milliseconds */
    checkInterval: number;
    /** Event types to monitor */
    eventTypes: string[];
    /** Minimum severity level to notify */
    minSeverity: EventSeverity;
    /** Custom notification channels */
    notificationChannels: string[];
    /** Filter by source app names */
    appFilter?: string[];
    /** Exclude sources */
    excludeSources?: string[];
}

export type EventSeverity = 'debug' | 'info' | 'warning' | 'error' | 'critical';

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
    /** Unique event ID */
    id: number;
    /** Event timestamp */
    timestamp: string;
    /** Event source/app name */
    source: string;
    /** Event type/category */
    eventType: string;
    /** Severity level */
    severity: EventSeverity;
    /** Event message */
    message: string;
    /** Additional metadata (JSON string) */
    metadata?: string;
    /** User associated with event */
    user?: string;
    /** Event code */
    eventCode?: string;
}

export interface EventLoggerStats {
    totalEvents: number;
    eventsBySeverity: Record<EventSeverity, number>;
    eventsBySource: Record<string, number>;
    eventsByType: Record<string, number>;
    timeRange: {
        earliest: string | null;
        latest: string | null;
    };
}

export interface EventLoggerNotificationRule {
    id: string;
    name: string;
    enabled: boolean;
    eventTypes: string[];
    severity: EventSeverity;
    sources?: string[];
    excludeSources?: string[];
    keywords?: string[];
    excludeKeywords?: string[];
    channels: string[];
    cooldown: number;
    maxNotificationsPerHour: number;
    quietHoursStart?: string;
    quietHoursEnd?: string;
    createdAt: Date;
    lastTriggeredAt?: Date;
    triggerCount: number;
}

// Database schema info (for reference)
export interface EventLoggerSchema {
    tables: {
        name: string;
        columns: string[];
    }[];
}

// API Request/Response types
export interface UpdateEventLoggerConfigRequest {
    dbPath?: string;
    enabled?: boolean;
    checkInterval?: number;
    eventTypes?: string[];
    minSeverity?: EventSeverity;
    notificationChannels?: string[];
    appFilter?: string[];
    excludeSources?: string[];
}

export interface GetEventsRequest {
    limit?: number;
    offset?: number;
    startTime?: string;
    endTime?: string;
    severity?: EventSeverity;
    source?: string;
    eventType?: string;
    search?: string;
}

export interface GetEventsResponse {
    events: EventLogEntry[];
    total: number;
    hasMore: boolean;
}

export interface CreateEventNotificationRuleRequest {
    name: string;
    eventTypes: string[];
    severity: EventSeverity;
    sources?: string[];
    excludeSources?: string[];
    keywords?: string[];
    excludeKeywords?: string[];
    channels: string[];
    cooldown: number;
    maxNotificationsPerHour: number;
    quietHoursStart?: string;
    quietHoursEnd?: string;
}

export interface EventNotificationRequest {
    title: string;
    content: string;
    event: EventLogEntry;
    rule: EventLoggerNotificationRule;
}
