import fs from 'fs';
import path from 'path';
import config from '../utils/config';
import Logger from '../utils/logger';
import * as logFileService from './logFile';
import * as auditService from './audit';
import { isValidThreshold, isValidDays } from '../utils/validation';
import { parseSizeThreshold } from '../utils/sizeParser';

const logger = Logger.child({ module: 'AutoClean' });

export interface AutoCleanRule {
    id: string;
    name: string;
    enabled: boolean;
    type: 'truncateLarge' | 'deleteOld' | 'deleteUninstalled';
    threshold?: string;
    days?: number;
    schedule: string;
    lastRun?: string;
}

interface AutoCleanConfig {
    rules: AutoCleanRule[];
}

const CONFIG_FILENAME = 'auto-clean.json';
const runningTimers = new Map<string, NodeJS.Timeout>();
const runningLocks = new Set<string>();

function isCronExpression(schedule: string): boolean {
    if (['hourly', 'daily', 'weekly'].includes(schedule)) return false;
    if (/^\d+$/.test(schedule)) return false;
    const parts = schedule.trim().split(/\s+/);
    return parts.length === 5;
}

function parseCronNextTime(expression: string): number | null {
    const parts = expression.trim().split(/\s+/);
    if (parts.length !== 5) return null;

    const now = new Date();
    const fieldRanges = [
        { min: 0, max: 59 },
        { min: 0, max: 23 },
        { min: 1, max: 31 },
        { min: 1, max: 12 },
        { min: 0, max: 6 }
    ];

    function parseField(field: string, range: { min: number; max: number }): number[] | null {
        if (field === '*') {
            const values: number[] = [];
            for (let i = range.min; i <= range.max; i++) values.push(i);
            return values;
        }
        if (/^\d+$/.test(field)) {
            const v = parseInt(field, 10);
            if (v < range.min || v > range.max) return null;
            return [v];
        }
        if (field.includes('/')) {
            const [base, stepStr] = field.split('/');
            const step = parseInt(stepStr, 10);
            if (isNaN(step) || step <= 0) return null;
            const start = base === '*' ? range.min : parseInt(base, 10);
            if (isNaN(start)) return null;
            const values: number[] = [];
            for (let i = start; i <= range.max; i += step) values.push(i);
            return values;
        }
        if (field.includes(',')) {
            const values: number[] = [];
            for (const part of field.split(',')) {
                const sub = parseField(part, range);
                if (!sub) return null;
                values.push(...sub);
            }
            return [...new Set(values)].sort((a, b) => a - b);
        }
        if (field.includes('-')) {
            const [fromStr, toStr] = field.split('-');
            const from = parseInt(fromStr, 10);
            const to = parseInt(toStr, 10);
            if (isNaN(from) || isNaN(to)) return null;
            const values: number[] = [];
            for (let i = from; i <= to; i++) values.push(i);
            return values;
        }
        return null;
    }

    const fieldValues: number[][] = [];
    for (let i = 0; i < 5; i++) {
        const values = parseField(parts[i], fieldRanges[i]);
        if (!values) return null;
        fieldValues.push(values);
    }

    const [minutes, hours, daysOfMonth, months, daysOfWeek] = fieldValues;

    for (let offset = 1; offset <= 366 * 24 * 60; offset++) {
        const candidate = new Date(now.getTime() + offset * 60 * 1000);
        if (!months.includes(candidate.getMonth() + 1)) continue;
        if (!daysOfMonth.includes(candidate.getDate())) continue;
        if (!daysOfWeek.includes(candidate.getDay())) continue;
        if (!hours.includes(candidate.getHours())) continue;
        if (!minutes.includes(candidate.getMinutes())) continue;
        return offset * 60 * 1000;
    }
    return null;
}

function getConfigPath(): string {
    return path.join(config.dataDir, 'config', CONFIG_FILENAME);
}

async function ensureDataDir(): Promise<void> {
    try {
        await fs.promises.mkdir(path.join(config.dataDir, 'config'), { recursive: true, mode: 0o700 });
    } catch (e) {
        if ((e as NodeJS.ErrnoException).code !== 'EEXIST') throw e;
    }
}

export async function loadConfig(): Promise<AutoCleanConfig> {
    const configPath = getConfigPath();
    try {
        const content = await fs.promises.readFile(configPath, 'utf8');
        const data = JSON.parse(content) as AutoCleanConfig;
        return { rules: Array.isArray(data.rules) ? data.rules : [] };
    } catch (e) {
        if ((e as NodeJS.ErrnoException).code === 'ENOENT') {
            return { rules: [] };
        }
        logger.error({ err: e }, '加载自动清理配置失败');
        return { rules: [] };
    }
}

async function saveConfig(configData: AutoCleanConfig): Promise<void> {
    await ensureDataDir();
    const configPath = getConfigPath();
    const tmpPath = configPath + '.tmp';
    const content = JSON.stringify(configData, null, 2);
    await fs.promises.writeFile(tmpPath, content, { mode: 0o600 });
    await fs.promises.rename(tmpPath, configPath);
}

function parseScheduleToMs(schedule: string): number | null {
    switch (schedule) {
        case 'hourly':
            return 60 * 60 * 1000;
        case 'daily':
            return 24 * 60 * 60 * 1000;
        case 'weekly':
            return 7 * 24 * 60 * 60 * 1000;
        default: {
            if (isCronExpression(schedule)) {
                return 60 * 1000;
            }
            const seconds = parseInt(schedule, 10);
            if (!isNaN(seconds) && seconds >= 60) {
                return seconds * 1000;
            }
            return null;
        }
    }
}

function getNextScheduleDelay(schedule: string): number | null {
    const now = new Date();
    switch (schedule) {
        case 'daily': {
            const next = new Date(now);
            next.setHours(3, 0, 0, 0);
            if (next.getTime() <= now.getTime()) {
                next.setDate(next.getDate() + 1);
            }
            return next.getTime() - now.getTime();
        }
        case 'weekly': {
            const next = new Date(now);
            next.setHours(3, 0, 0, 0);
            const dayOfWeek = next.getDay();
            const daysUntilSunday = dayOfWeek === 0 ? 0 : 7 - dayOfWeek;
            next.setDate(next.getDate() + daysUntilSunday);
            if (next.getTime() <= now.getTime()) {
                next.setDate(next.getDate() + 7);
            }
            return next.getTime() - now.getTime();
        }
        case 'hourly': {
            const next = new Date(now);
            next.setMinutes(0, 0, 0);
            next.setHours(next.getHours() + 1);
            return next.getTime() - now.getTime();
        }
        default: {
            if (isCronExpression(schedule)) {
                return parseCronNextTime(schedule);
            }
            const ms = parseScheduleToMs(schedule);
            return ms;
        }
    }
}

async function executeRule(rule: AutoCleanRule): Promise<{ cleaned: number; errors: string[] }> {
    if (runningLocks.has(rule.id)) {
        logger.warn({ ruleId: rule.id }, '规则正在执行中，跳过');
        return { cleaned: 0, errors: ['规则正在执行中'] };
    }

    runningLocks.add(rule.id);
    try {
        let result: { cleaned: number; errors: string[] };

        switch (rule.type) {
            case 'truncateLarge': {
                const threshold = rule.threshold || '100M';
                if (!isValidThreshold(threshold)) {
                    return { cleaned: 0, errors: ['无效的大小阈值'] };
                }
                const thresholdBytes = parseSizeThreshold(threshold);
                result = await logFileService.cleanLogFiles({
                    thresholdBytes,
                    days: null,
                    action: 'truncate'
                });
                break;
            }
            case 'deleteOld': {
                const days = rule.days || 7;
                if (!isValidDays(days)) {
                    return { cleaned: 0, errors: ['无效的天数'] };
                }
                result = await logFileService.cleanLogFiles({
                    thresholdBytes: null,
                    days,
                    action: 'delete'
                });
                break;
            }
            case 'deleteUninstalled': {
                result = await logFileService.cleanLogFiles({
                    thresholdBytes: null,
                    days: null,
                    action: 'delete'
                });
                break;
            }
            default:
                return { cleaned: 0, errors: ['未知的清理类型'] };
        }

        await auditService.addAuditLog('auto_clean', {
            ruleId: rule.id,
            ruleName: rule.name,
            type: rule.type,
            cleaned: result.cleaned,
            errors: result.errors.length
        });

        return result;
    } catch (e) {
        logger.error({ err: e, ruleId: rule.id }, '自动清理规则执行失败');
        await auditService.addAuditLog('auto_clean', {
            ruleId: rule.id,
            ruleName: rule.name,
            type: rule.type,
            error: (e as Error).message
        });
        return { cleaned: 0, errors: [(e as Error).message] };
    } finally {
        runningLocks.delete(rule.id);
    }
}

function scheduleRule(rule: AutoCleanRule): void {
    clearTimer(rule.id);

    if (!rule.enabled) return;

    const initialDelay = getNextScheduleDelay(rule.schedule);
    if (initialDelay === null) {
        logger.warn({ ruleId: rule.id, schedule: rule.schedule }, '无效的调度计划');
        return;
    }

    logger.info({ ruleId: rule.id, schedule: rule.schedule, delayMs: initialDelay }, '调度自动清理规则');

    const timer = setTimeout(async () => {
        await executeRule(rule);

        const currentConfig = await loadConfig();
        const currentRule = currentConfig.rules.find(r => r.id === rule.id);
        if (currentRule) {
            currentRule.lastRun = new Date().toISOString();
            await saveConfig(currentConfig);
        }

        if (isCronExpression(rule.schedule)) {
            const scheduleCron = () => {
                const nextDelay = parseCronNextTime(rule.schedule);
                if (nextDelay) {
                    const t = setTimeout(async () => {
                        await executeRule(rule);
                        const cfg = await loadConfig();
                        const r = cfg.rules.find(x => x.id === rule.id);
                        if (r) {
                            r.lastRun = new Date().toISOString();
                            await saveConfig(cfg);
                        }
                        scheduleCron();
                    }, nextDelay);
                    runningTimers.set(rule.id, t);
                }
            };
            scheduleCron();
        } else {
            const interval = parseScheduleToMs(rule.schedule);
            if (interval) {
                const repeatTimer = setInterval(async () => {
                    await executeRule(rule);

                    const cfg = await loadConfig();
                    const r = cfg.rules.find(x => x.id === rule.id);
                    if (r) {
                        r.lastRun = new Date().toISOString();
                        await saveConfig(cfg);
                    }
                }, interval);
                runningTimers.set(rule.id, repeatTimer as unknown as NodeJS.Timeout);
            }
        }
    }, initialDelay);

    runningTimers.set(rule.id, timer);
}

function clearTimer(ruleId: string): void {
    const timer = runningTimers.get(ruleId);
    if (timer) {
        clearTimeout(timer);
        clearInterval(timer);
        runningTimers.delete(ruleId);
    }
}

export async function init(): Promise<void> {
    const configData = await loadConfig();
    logger.info({ ruleCount: configData.rules.length }, '初始化自动清理服务');

    for (const rule of configData.rules) {
        if (rule.enabled) {
            scheduleRule(rule);
        }
    }
}

export async function shutdown(): Promise<void> {
    logger.info('关闭自动清理服务');
    for (const [ruleId, timer] of runningTimers) {
        clearTimeout(timer);
        clearInterval(timer);
        runningTimers.delete(ruleId);
    }
}

export async function getRules(): Promise<AutoCleanRule[]> {
    const configData = await loadConfig();
    return configData.rules;
}

export async function addRule(rule: Omit<AutoCleanRule, 'id' | 'lastRun'>): Promise<AutoCleanRule> {
    const configData = await loadConfig();
    const newRule: AutoCleanRule = {
        ...rule,
        id: crypto.randomUUID ? crypto.randomUUID() : `rule_${Date.now()}_${Math.random().toString(36).slice(2, 8)}`,
    };
    configData.rules.push(newRule);
    await saveConfig(configData);

    if (newRule.enabled) {
        scheduleRule(newRule);
    }

    logger.info({ ruleId: newRule.id, name: newRule.name }, '添加自动清理规则');
    return newRule;
}

export async function updateRule(id: string, updates: Partial<Omit<AutoCleanRule, 'id'>>): Promise<AutoCleanRule | null> {
    const configData = await loadConfig();
    const index = configData.rules.findIndex(r => r.id === id);
    if (index === -1) return null;

    const oldEnabled = configData.rules[index].enabled;
    configData.rules[index] = { ...configData.rules[index], ...updates };
    await saveConfig(configData);

    const rule = configData.rules[index];
    if (oldEnabled && !rule.enabled) {
        clearTimer(rule.id);
    } else if (!oldEnabled && rule.enabled) {
        scheduleRule(rule);
    } else if (rule.enabled) {
        scheduleRule(rule);
    }

    logger.info({ ruleId: id }, '更新自动清理规则');
    return rule;
}

export async function deleteRule(id: string): Promise<boolean> {
    const configData = await loadConfig();
    const index = configData.rules.findIndex(r => r.id === id);
    if (index === -1) return false;

    clearTimer(id);
    configData.rules.splice(index, 1);
    await saveConfig(configData);

    logger.info({ ruleId: id }, '删除自动清理规则');
    return true;
}

export async function toggleRule(id: string): Promise<AutoCleanRule | null> {
    const configData = await loadConfig();
    const rule = configData.rules.find(r => r.id === id);
    if (!rule) return null;

    rule.enabled = !rule.enabled;
    await saveConfig(configData);

    if (rule.enabled) {
        scheduleRule(rule);
    } else {
        clearTimer(rule.id);
    }

    logger.info({ ruleId: id, enabled: rule.enabled }, '切换自动清理规则');
    return rule;
}

export async function executeRuleNow(id: string): Promise<{ cleaned: number; errors: string[] } | null> {
    const configData = await loadConfig();
    const rule = configData.rules.find(r => r.id === id);
    if (!rule) return null;

    const result = await executeRule(rule);

    rule.lastRun = new Date().toISOString();
    await saveConfig(configData);

    return result;
}
