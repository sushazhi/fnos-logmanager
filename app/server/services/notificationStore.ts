/**
 * 通知数据存储模块
 * 负责持久化存储通知配置、规则和历史记录
 */

import fs from 'fs';
import path from 'path';
import { promisify } from 'util';
import config from '../utils/config';
import {
    NotificationChannelConfig,
    NotificationRule,
    NotificationHistory,
    NotificationConfigFile,
    NotificationStats,
    NotificationChannel
} from '../types/notification';

const readFile = promisify(fs.readFile);
const writeFile = promisify(fs.writeFile);
const mkdir = promisify(fs.mkdir);

const CONFIG_VERSION = '1.0.0';
const DEFAULT_CONFIG_FILE = 'notification-config.json';
const HISTORY_FILE = 'notification-history.json';

// 默认配置
const DEFAULT_CONFIG: NotificationConfigFile = {
    version: CONFIG_VERSION,
    channels: [],
    rules: [],
    settings: {
        enabled: false,
        checkInterval: 30000,      // 30秒
        maxHistoryDays: 7,
        maxHistoryCount: 1000
    }
};

// 内存缓存
let cachedConfig: NotificationConfigFile | null = null;
let cachedHistory: NotificationHistory[] | null = null;
let configFilePath: string | null = null;
let historyFilePath: string | null = null;

/**
 * 初始化存储模块
 */
export async function init(): Promise<void> {
    // 使用配置目录下的通知配置文件
    const configDir = path.dirname(config.auditLogFile);
    configFilePath = path.join(configDir, DEFAULT_CONFIG_FILE);
    historyFilePath = path.join(configDir, HISTORY_FILE);

    // 确保目录存在
    try {
        await mkdir(configDir, { recursive: true });
    } catch {
        // 目录已存在
    }

    // 加载配置
    await loadConfig();
    await loadHistory();

    console.log('[NotificationStore] 初始化完成');
}

/**
 * 加载配置文件
 */
async function loadConfig(): Promise<void> {
    if (!configFilePath) return;

    try {
        const data = await readFile(configFilePath, 'utf8');
        const parsed = JSON.parse(data);

        // 版本兼容性检查
        if (parsed.version !== CONFIG_VERSION) {
            console.log('[NotificationStore] 配置版本不匹配，使用默认配置');
            cachedConfig = { ...DEFAULT_CONFIG };
        } else {
            cachedConfig = parsed;
        }

        // 转换日期字段
        if (cachedConfig) {
            cachedConfig.rules = cachedConfig.rules.map(rule => ({
                ...rule,
                createdAt: new Date(rule.createdAt),
                updatedAt: new Date(rule.updatedAt),
                lastTriggeredAt: rule.lastTriggeredAt ? new Date(rule.lastTriggeredAt) : undefined
            }));
        }
    } catch (e) {
        if ((e as NodeJS.ErrnoException).code === 'ENOENT') {
            // 文件不存在，使用默认配置
            cachedConfig = { ...DEFAULT_CONFIG };
            await saveConfig();
        } else {
            console.error('[NotificationStore] 加载配置失败:', e);
            cachedConfig = { ...DEFAULT_CONFIG };
        }
    }
}

/**
 * 保存配置文件
 */
async function saveConfig(): Promise<void> {
    if (!configFilePath) {
        console.error('[NotificationStore] 保存配置失败: 配置文件路径未设置');
        throw new Error('配置文件路径未设置');
    }
    if (!cachedConfig) {
        console.error('[NotificationStore] 保存配置失败: 配置未加载');
        throw new Error('配置未加载');
    }

    try {
        console.log('[NotificationStore] 保存配置到:', configFilePath);
        await writeFile(configFilePath, JSON.stringify(cachedConfig, null, 2), 'utf8');
        console.log('[NotificationStore] 配置保存成功');
    } catch (e) {
        console.error('[NotificationStore] 保存配置失败:', e);
        throw e;
    }
}

/**
 * 加载历史记录
 */
async function loadHistory(): Promise<void> {
    if (!historyFilePath) return;

    try {
        const data = await readFile(historyFilePath, 'utf8');
        const parsed = JSON.parse(data);

        cachedHistory = parsed.map((item: NotificationHistory) => ({
            ...item,
            timestamp: new Date(item.timestamp)
        }));
    } catch (e) {
        if ((e as NodeJS.ErrnoException).code === 'ENOENT') {
            cachedHistory = [];
        } else {
            console.error('[NotificationStore] 加载历史记录失败:', e);
            cachedHistory = [];
        }
    }
}

/**
 * 保存历史记录
 */
async function saveHistory(): Promise<void> {
    if (!historyFilePath || !cachedHistory) return;

    try {
        await writeFile(historyFilePath, JSON.stringify(cachedHistory, null, 2), 'utf8');
    } catch (e) {
        console.error('[NotificationStore] 保存历史记录失败:', e);
    }
}

/**
 * 获取完整配置
 */
export function getConfig(): NotificationConfigFile {
    if (!cachedConfig) {
        console.warn('[NotificationStore] 配置未初始化，使用默认配置');
        cachedConfig = { ...DEFAULT_CONFIG };
    }
    return cachedConfig;
}

/**
 * 确保存储已初始化
 */
async function ensureInitialized(): Promise<void> {
    if (!configFilePath || !cachedConfig) {
        await init();
    }
}

/**
 * 获取设置
 */
export function getSettings() {
    return getConfig().settings;
}

/**
 * 更新设置
 */
export async function updateSettings(settings: Partial<NotificationConfigFile['settings']>): Promise<void> {
    await ensureInitialized();
    if (!cachedConfig) {
        throw new Error('存储未初始化');
    }

    cachedConfig.settings = {
        ...cachedConfig.settings,
        ...settings
    };
    await saveConfig();
}

// ==================== 渠道管理 ====================

/**
 * 获取所有渠道配置
 */
export function getChannels(): NotificationChannelConfig[] {
    return getConfig().channels;
}

/**
 * 获取渠道配置
 */
export function getChannel(channelId: string): NotificationChannelConfig | undefined {
    return getConfig().channels.find(c => c.name === channelId);
}

/**
 * 添加渠道配置
 */
export async function addChannel(channel: NotificationChannelConfig): Promise<void> {
    await ensureInitialized();
    if (!cachedConfig) {
        throw new Error('存储未初始化');
    }

    // 检查名称是否重复
    if (cachedConfig.channels.some(c => c.name === channel.name)) {
        throw new Error('渠道名称已存在');
    }

    cachedConfig.channels.push(channel);
    await saveConfig();
}

/**
 * 更新渠道配置
 */
export async function updateChannel(channelName: string, updates: Partial<NotificationChannelConfig>): Promise<void> {
    await ensureInitialized();
    if (!cachedConfig) {
        throw new Error('存储未初始化');
    }

    const index = cachedConfig.channels.findIndex(c => c.name === channelName);
    if (index === -1) {
        throw new Error('渠道不存在');
    }

    cachedConfig.channels[index] = {
        ...cachedConfig.channels[index],
        ...updates
    };
    await saveConfig();
}

/**
 * 删除渠道配置
 */
export async function deleteChannel(channelName: string): Promise<void> {
    await ensureInitialized();
    if (!cachedConfig) {
        throw new Error('存储未初始化');
    }

    const index = cachedConfig.channels.findIndex(c => c.name === channelName);
    if (index === -1) {
        throw new Error('渠道不存在');
    }

    cachedConfig.channels.splice(index, 1);
    await saveConfig();
}

// ==================== 规则管理 ====================

/**
 * 获取所有规则
 */
export function getRules(): NotificationRule[] {
    return getConfig().rules;
}

/**
 * 获取启用的规则
 */
export function getEnabledRules(): NotificationRule[] {
    return getConfig().rules.filter(r => r.status === 'enabled');
}

/**
 * 获取规则
 */
export function getRule(ruleId: string): NotificationRule | undefined {
    return getConfig().rules.find(r => r.id === ruleId);
}

/**
 * 添加规则
 */
export async function addRule(rule: NotificationRule): Promise<void> {
    await ensureInitialized();
    if (!cachedConfig) {
        throw new Error('存储未初始化');
    }

    // 检查ID是否重复
    if (cachedConfig.rules.some(r => r.id === rule.id)) {
        throw new Error('规则ID已存在');
    }

    cachedConfig.rules.push(rule);
    await saveConfig();
}

/**
 * 更新规则
 */
export async function updateRule(ruleId: string, updates: Partial<NotificationRule>): Promise<void> {
    await ensureInitialized();
    if (!cachedConfig) {
        throw new Error('存储未初始化');
    }

    const index = cachedConfig.rules.findIndex(r => r.id === ruleId);
    if (index === -1) {
        throw new Error('规则不存在');
    }

    cachedConfig.rules[index] = {
        ...cachedConfig.rules[index],
        ...updates,
        updatedAt: new Date()
    };
    await saveConfig();
}

/**
 * 删除规则
 */
export async function deleteRule(ruleId: string): Promise<void> {
    await ensureInitialized();
    if (!cachedConfig) {
        throw new Error('存储未初始化');
    }

    const index = cachedConfig.rules.findIndex(r => r.id === ruleId);
    if (index === -1) {
        throw new Error('规则不存在');
    }

    cachedConfig.rules.splice(index, 1);
    await saveConfig();
}

/**
 * 更新规则触发信息
 */
export async function updateRuleTrigger(ruleId: string): Promise<void> {
    await ensureInitialized();
    if (!cachedConfig) return;

    const rule = cachedConfig.rules.find(r => r.id === ruleId);
    if (rule) {
        rule.lastTriggeredAt = new Date();
        rule.triggerCount++;
        await saveConfig();
    }
}

// ==================== 历史记录管理 ====================

/**
 * 添加历史记录
 */
export async function addHistory(record: NotificationHistory): Promise<void> {
    await ensureInitialized();
    if (!cachedHistory) {
        cachedHistory = [];
    }

    cachedHistory.unshift(record);

    // 清理旧记录
    const settings = getSettings();
    if (cachedHistory.length > settings.maxHistoryCount) {
        cachedHistory = cachedHistory.slice(0, settings.maxHistoryCount);
    }

    await saveHistory();
}

/**
 * 获取历史记录
 */
export function getHistory(limit: number = 100): NotificationHistory[] {
    if (!cachedHistory) return [];
    return cachedHistory.slice(0, limit);
}

/**
 * 清理旧历史记录
 */
export async function cleanOldHistory(): Promise<void> {
    await ensureInitialized();
    if (!cachedHistory) return;

    const settings = getSettings();
    const cutoffTime = Date.now() - settings.maxHistoryDays * 24 * 60 * 60 * 1000;

    cachedHistory = cachedHistory.filter(h => new Date(h.timestamp).getTime() > cutoffTime);
    await saveHistory();
}

/**
 * 清空所有历史记录
 */
export async function clearHistory(): Promise<void> {
    await ensureInitialized();
    cachedHistory = [];
    await saveHistory();
}

/**
 * 获取统计信息
 */
export function getStats(): NotificationStats {
    const now = Date.now();
    const last24Hours = now - 24 * 60 * 60 * 1000;

    if (!cachedHistory) {
        return {
            totalSent: 0,
            successCount: 0,
            failCount: 0,
            last24Hours: 0,
            byChannel: {} as Record<NotificationChannel, { sent: number; success: number; failed: number }>,
            byRule: {}
        };
    }

    const stats: NotificationStats = {
        totalSent: cachedHistory.length,
        successCount: cachedHistory.filter(h => h.success).length,
        failCount: cachedHistory.filter(h => !h.success).length,
        last24Hours: cachedHistory.filter(h => new Date(h.timestamp).getTime() > last24Hours).length,
        byChannel: {} as Record<NotificationChannel, { sent: number; success: number; failed: number }>,
        byRule: {}
    };

    // 按渠道统计
    for (const record of cachedHistory) {
        if (!stats.byChannel[record.channel]) {
            stats.byChannel[record.channel] = { sent: 0, success: 0, failed: 0 };
        }
        stats.byChannel[record.channel].sent++;
        if (record.success) {
            stats.byChannel[record.channel].success++;
        } else {
            stats.byChannel[record.channel].failed++;
        }
    }

    // 按规则统计
    for (const record of cachedHistory) {
        if (!stats.byRule[record.ruleId]) {
            stats.byRule[record.ruleId] = { sent: 0, success: 0, failed: 0 };
        }
        stats.byRule[record.ruleId].sent++;
        if (record.success) {
            stats.byRule[record.ruleId].success++;
        } else {
            stats.byRule[record.ruleId].failed++;
        }
    }

    return stats;
}

// ==================== 频率控制 ====================

// 通知频率控制缓存
const notificationCooldowns = new Map<string, number>();
const notificationCounts = new Map<string, { count: number; resetTime: number }>();

/**
 * 检查是否可以发送通知（频率控制）
 */
export function canSendNotification(rule: NotificationRule): boolean {
    const now = Date.now();
    const ruleId = rule.id;

    // 检查冷却时间
    const lastSent = notificationCooldowns.get(ruleId);
    if (lastSent && (now - lastSent) < rule.cooldown * 1000) {
        return false;
    }

    // 检查每小时最大次数
    let countInfo = notificationCounts.get(ruleId);
    if (!countInfo || now > countInfo.resetTime) {
        countInfo = { count: 0, resetTime: now + 3600000 };
        notificationCounts.set(ruleId, countInfo);
    }

    if (countInfo.count >= rule.maxNotifications) {
        return false;
    }

    return true;
}

/**
 * 记录通知发送
 */
export function recordNotification(ruleId: string): void {
    const now = Date.now();
    notificationCooldowns.set(ruleId, now);

    let countInfo = notificationCounts.get(ruleId);
    if (!countInfo || now > countInfo.resetTime) {
        countInfo = { count: 1, resetTime: now + 3600000 };
    } else {
        countInfo.count++;
    }
    notificationCounts.set(ruleId, countInfo);
}

/**
 * 检查是否在静默时段
 */
export function isInQuietHours(rule: NotificationRule): boolean {
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

    // 处理跨午夜的情况
    if (startTime > endTime) {
        return currentTime >= startTime || currentTime < endTime;
    } else {
        return currentTime >= startTime && currentTime < endTime;
    }
}

/**
 * 清理频率控制缓存
 */
export function cleanupFrequencyCache(): void {
    const now = Date.now();

    // 清理过期的计数器
    for (const [ruleId, countInfo] of notificationCounts.entries()) {
        if (now > countInfo.resetTime) {
            notificationCounts.delete(ruleId);
        }
    }
}
