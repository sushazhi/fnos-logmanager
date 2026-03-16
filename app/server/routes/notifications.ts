/**
 * 通知配置路由
 */

import express, { Request, Response, NextFunction } from 'express';
import { query, body, param } from 'express-validator';
import * as notificationStore from '../services/notificationStore';
import * as notificationService from '../services/notification';
import * as logMonitor from '../services/logMonitor';
import { validateToken, validateCSRF } from '../middleware/auth';
import {
    NotificationChannelConfig,
    NotificationRule,
    NotificationChannel,
    CreateNotificationRuleRequest,
    CreateChannelRequest
} from '../types/notification';

const router = express.Router();

/**
 * 生成唯一ID
 */
function generateId(): string {
    return `${Date.now()}-${Math.random().toString(36).substr(2, 9)}`;
}

// ==================== 设置管理 ====================

/**
 * 获取通知设置
 */
router.get('/settings', validateToken, (_req: Request, res: Response) => {
    const settings = notificationStore.getSettings();
    res.json({ settings });
});

/**
 * 更新通知设置
 */
router.post('/settings', validateToken, validateCSRF, async (req: Request, res: Response, next: NextFunction) => {
    try {
        const { enabled, checkInterval, maxHistoryDays, maxHistoryCount } = req.body;

        const updates: Record<string, unknown> = {};

        if (typeof enabled === 'boolean') {
            updates.enabled = enabled;
        }

        if (typeof checkInterval === 'number' && checkInterval >= 5000 && checkInterval <= 300000) {
            updates.checkInterval = checkInterval;
        }

        if (typeof maxHistoryDays === 'number' && maxHistoryDays >= 1 && maxHistoryDays <= 30) {
            updates.maxHistoryDays = maxHistoryDays;
        }

        if (typeof maxHistoryCount === 'number' && maxHistoryCount >= 100 && maxHistoryCount <= 10000) {
            updates.maxHistoryCount = maxHistoryCount;
        }

        await notificationStore.updateSettings(updates);

        // 如果启用状态改变，重启监控
        if (typeof enabled === 'boolean') {
            if (enabled) {
                await logMonitor.start();
            } else {
                logMonitor.stop();
            }
        } else if (updates.checkInterval) {
            // 如果检查间隔改变，重启监控
            await logMonitor.restart();
        }

        res.json({ success: true, settings: notificationStore.getSettings() });
    } catch (err) {
        next(err);
    }
});

// ==================== 渠道管理 ====================

/**
 * 获取所有渠道
 */
router.get('/channels', validateToken, (_req: Request, res: Response) => {
    const channels = notificationStore.getChannels();
    res.json({ channels });
});

/**
 * 获取支持的渠道类型
 */
router.get('/channels/types', validateToken, (_req: Request, res: Response) => {
    const types: NotificationChannel[] = [
        'bark', 'dingtalk', 'feishu', 'feishu_app', 'wecom', 'wecom_app', 'wechat_bot',
        'telegram', 'serverchan', 'pushplus',
        'webhook', 'ntfy', 'gotify', 'pushdeer', 'qqbot'
    ];

    const typeInfo = types.map(type => ({
        type,
        name: notificationService.getChannelTypeName(type),
        fields: notificationService.getChannelConfigFields(type)
    }));

    res.json({ types: typeInfo });
});

/**
 * 添加渠道
 */
router.post('/channels', validateToken, validateCSRF, async (req: Request, res: Response, next: NextFunction) => {
    try {
        const { channel, name, config } = req.body as CreateChannelRequest;

        if (!channel || !name) {
            res.status(400).json({ error: '缺少必要参数' });
            return;
        }

        const validChannels: NotificationChannel[] = [
            'bark', 'dingtalk', 'feishu', 'feishu_app', 'wecom', 'wecom_app', 'wechat_bot',
            'telegram', 'serverchan', 'pushplus',
            'webhook', 'ntfy', 'gotify', 'pushdeer', 'qqbot'
        ];

        if (!validChannels.includes(channel)) {
            res.status(400).json({ error: '不支持的渠道类型' });
            return;
        }

        const channelConfig: NotificationChannelConfig = {
            channel,
            name,
            enabled: true,
            ...config
        };

        await notificationStore.addChannel(channelConfig);
        res.json({ success: true, channel: channelConfig });
    } catch (err) {
        next(err);
    }
});

/**
 * 更新渠道
 */
router.put('/channels/:name', validateToken, validateCSRF, async (req: Request, res: Response, next: NextFunction) => {
    try {
        const channelName = req.params.name;
        const updates = req.body;

        await notificationStore.updateChannel(channelName, updates);
        const channel = notificationStore.getChannel(channelName);
        res.json({ success: true, channel });
    } catch (err) {
        next(err);
    }
});

/**
 * 删除渠道
 */
router.delete('/channels/:name', validateToken, validateCSRF, async (req: Request, res: Response, next: NextFunction) => {
    try {
        const channelName = req.params.name;
        await notificationStore.deleteChannel(channelName);
        res.json({ success: true });
    } catch (err) {
        next(err);
    }
});

/**
 * 测试渠道
 */
router.post('/channels/:name/test', validateToken, validateCSRF, async (req: Request, res: Response, next: NextFunction) => {
    try {
        const channelName = req.params.name;
        const channel = notificationStore.getChannel(channelName);

        if (!channel) {
            res.status(404).json({ error: '渠道不存在' });
            return;
        }

        const result = await notificationService.testChannel(channel);
        res.json({ result });
    } catch (err) {
        next(err);
    }
});

// ==================== 规则管理 ====================

/**
 * 获取所有规则
 */
router.get('/rules', validateToken, (_req: Request, res: Response) => {
    const rules = notificationStore.getRules();
    res.json({ rules });
});

/**
 * 获取单个规则
 */
router.get('/rules/:id', validateToken, async (req: Request, res: Response, next: NextFunction) => {
    try {
        const ruleId = req.params.id;
        const rule = notificationStore.getRule(ruleId);

        if (!rule) {
            res.status(404).json({ error: '规则不存在' });
            return;
        }

        res.json({ rule });
    } catch (err) {
        next(err);
    }
});

/**
 * 添加规则
 */
router.post('/rules', validateToken, validateCSRF, async (req: Request, res: Response, next: NextFunction) => {
    try {
        const ruleData = req.body as CreateNotificationRuleRequest;

        // 验证必要字段
        if (!ruleData.name || !ruleData.appName || !ruleData.logLevel) {
            res.status(400).json({ error: '缺少必要参数' });
            return;
        }

        // 确保 channels 是数组
        if (!ruleData.channels || !Array.isArray(ruleData.channels) || ruleData.channels.length === 0) {
            res.status(400).json({ error: '至少需要指定一个通知渠道' });
            return;
        }

        // 验证渠道是否存在
        const availableChannels = notificationStore.getChannels();
        for (const channelName of ruleData.channels) {
            if (!availableChannels.some(c => c.name === channelName)) {
                res.status(400).json({ error: `渠道 "${channelName}" 不存在` });
                return;
            }
        }

        const rule: NotificationRule = {
            id: generateId(),
            name: ruleData.name,
            status: 'enabled',
            appName: ruleData.appName,
            logPaths: ruleData.logPaths,
            logLevel: ruleData.logLevel,
            keywords: ruleData.keywords,
            excludeKeywords: ruleData.excludeKeywords,
            pattern: ruleData.pattern,
            channels: ruleData.channels,
            cooldown: ruleData.cooldown || 60,
            maxNotifications: ruleData.maxNotifications || 10,
            quietHoursStart: ruleData.quietHoursStart,
            quietHoursEnd: ruleData.quietHoursEnd,
            createdAt: new Date(),
            updatedAt: new Date(),
            triggerCount: 0
        };

        console.log('[Notifications] 创建规则对象:', JSON.stringify(rule, null, 2));
        await notificationStore.addRule(rule);
        res.json({ success: true, rule });
    } catch (err) {
        console.error('[Notifications] 添加规则失败:', err);
        next(err);
    }
});

/**
 * 更新规则
 */
router.put('/rules/:id', validateToken, validateCSRF, async (req: Request, res: Response, next: NextFunction) => {
    try {
        const ruleId = req.params.id;
        const updates = req.body;

        // 验证渠道是否存在
        if (updates.channels) {
            if (!Array.isArray(updates.channels)) {
                res.status(400).json({ error: 'channels 必须是数组' });
                return;
            }
            const channels = notificationStore.getChannels();
            for (const channelName of updates.channels) {
                if (!channels.some(c => c.name === channelName)) {
                    res.status(400).json({ error: `渠道 "${channelName}" 不存在` });
                    return;
                }
            }
        }

        await notificationStore.updateRule(ruleId, updates);
        const rule = notificationStore.getRule(ruleId);
        res.json({ success: true, rule });
    } catch (err) {
        next(err);
    }
});

/**
 * 删除规则
 */
router.delete('/rules/:id', validateToken, validateCSRF, async (req: Request, res: Response, next: NextFunction) => {
    try {
        const ruleId = req.params.id;
        await notificationStore.deleteRule(ruleId);
        res.json({ success: true });
    } catch (err) {
        next(err);
    }
});

/**
 * 启用/禁用规则
 */
router.post('/rules/:id/toggle', validateToken, validateCSRF, async (req: Request, res: Response, next: NextFunction) => {
    try {
        const ruleId = req.params.id;
        const rule = notificationStore.getRule(ruleId);

        if (!rule) {
            res.status(404).json({ error: '规则不存在' });
            return;
        }

        const newStatus = rule.status === 'enabled' ? 'disabled' : 'enabled';
        await notificationStore.updateRule(ruleId, { status: newStatus });

        res.json({ success: true, status: newStatus });
    } catch (err) {
        next(err);
    }
});

/**
 * 测试规则匹配
 */
router.post('/rules/:id/test', validateToken, validateCSRF, async (req: Request, res: Response, next: NextFunction) => {
    try {
        const ruleId = req.params.id;
        const { content } = req.body;

        if (!content) {
            res.status(400).json({ error: '缺少测试内容' });
            return;
        }

        const result = await logMonitor.testRuleMatch(ruleId, content);
        res.json({ result });
    } catch (err) {
        next(err);
    }
});

// ==================== 历史记录 ====================

/**
 * 获取通知历史
 */
router.get('/history', validateToken, [
    query('limit').optional().isInt({ min: 1, max: 500 })
], async (req: Request, res: Response, next: NextFunction) => {
    try {
        const limit = parseInt(req.query.limit as string) || 100;
        const history = notificationStore.getHistory(limit);
        res.json({ history });
    } catch (err) {
        next(err);
    }
});

/**
 * 清理历史记录
 */
router.post('/history/clean', validateToken, validateCSRF, async (_req: Request, res: Response, next: NextFunction) => {
    try {
        await notificationStore.clearHistory();
        res.json({ success: true });
    } catch (err) {
        next(err);
    }
});

// ==================== 统计信息 ====================

/**
 * 获取统计信息
 */
router.get('/stats', validateToken, (_req: Request, res: Response) => {
    const stats = notificationStore.getStats();
    res.json({ stats });
});

// ==================== 监控状态 ====================

/**
 * 获取监控状态
 */
router.get('/monitor/status', validateToken, (_req: Request, res: Response) => {
    const status = logMonitor.getStatus();
    res.json({ status });
});

/**
 * 启动监控
 */
router.post('/monitor/start', validateToken, validateCSRF, async (_req: Request, res: Response, next: NextFunction) => {
    try {
        await logMonitor.start();
        res.json({ success: true, status: logMonitor.getStatus() });
    } catch (err) {
        next(err);
    }
});

/**
 * 停止监控
 */
router.post('/monitor/stop', validateToken, validateCSRF, (_req: Request, res: Response) => {
    logMonitor.stop();
    res.json({ success: true, status: logMonitor.getStatus() });
});

/**
 * 手动触发检查
 */
router.post('/monitor/check', validateToken, validateCSRF, async (_req: Request, res: Response, next: NextFunction) => {
    try {
        await logMonitor.triggerCheck();
        res.json({ success: true, status: logMonitor.getStatus() });
    } catch (err) {
        next(err);
    }
});

export default router;
