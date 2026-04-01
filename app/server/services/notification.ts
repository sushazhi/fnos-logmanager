/**
 * 通知发送服务
 * 使用模块化通知系统，提供统一的通知发送接口
 */

import * as notificationStore from './notificationStore';
import { setConfig } from '../notify';
import { registry } from '../notify/registry';
import type { ChannelConfig } from '../notify/types';

/**
 * 渠道类型到渠道名称的映射
 */
const channelNameMap: Record<string, string> = {
    bark: 'bark',
    dingtalk: 'dingtalk',
    feishu: 'feishu-bot',
    feishu_app: 'feishu-app',
    wecom: 'wechat-work-bot',
    wecom_app: 'wechat-work-app',
    wechat_bot: 'wechat-bot',
    telegram: 'telegram',
    serverchan: 'serverChan',
    pushplus: 'pushplus',
    webhook: 'webhook',
    ntfy: 'ntfy',
    gotify: 'gotify',
    pushdeer: 'pushdeer',
    qqbot: 'qqbot',
    igot: 'igot',
    chat: 'synology-chat',
    qmsg: 'qmsg',
    pushme: 'pushme',
    wxpusher: 'wxpusher',
    aibotk: 'aibotk',
    weplusbot: 'weplusbot'
};

/**
 * 生成唯一ID
 */
function generateId(): string {
    return `${Date.now()}-${Math.random().toString(36).substr(2, 9)}`;
}

/**
 * 构建渠道配置
 */
function buildChannelConfig(channel: any): Partial<ChannelConfig> {
    const config: Partial<ChannelConfig> = {};

    switch (channel.channel) {
        case 'bark':
            if (channel.barkPush) config.BARK_PUSH = channel.barkPush;
            if (channel.barkSound) config.BARK_SOUND = channel.barkSound;
            if (channel.barkGroup) config.BARK_GROUP = channel.barkGroup;
            if (channel.barkIcon) config.BARK_ICON = channel.barkIcon;
            if (channel.barkLevel) config.BARK_LEVEL = channel.barkLevel;
            if (channel.barkArchive) config.BARK_ARCHIVE = channel.barkArchive;
            if (channel.barkUrl) config.BARK_URL = channel.barkUrl;
            break;
        case 'dingtalk':
            if (channel.ddBotToken) config.DD_BOT_TOKEN = channel.ddBotToken;
            if (channel.ddBotSecret) config.DD_BOT_SECRET = channel.ddBotSecret;
            break;
        case 'feishu':
            // 自定义机器人
            if (channel.fsKey) config.FSKEY = channel.fsKey;
            if (channel.fsSecret) config.FSSECRET = channel.fsSecret;
            break;
        case 'feishu_app':
            // 企业自建应用
            if (channel.feishuAppId) config.FEISHU_APP_ID = channel.feishuAppId;
            if (channel.feishuAppSecret) config.FEISHU_APP_SECRET = channel.feishuAppSecret;
            if (channel.feishuUserId) config.FEISHU_USER_ID = channel.feishuUserId;
            break;
        case 'wecom':
            if (channel.qywxKey) config.QYWX_KEY = channel.qywxKey;
            if (channel.qywxOrigin) config.QYWX_ORIGIN = channel.qywxOrigin;
            break;
        case 'wecom_app':
            if (channel.qywxAm) config.QYWX_AM = channel.qywxAm;
            if (channel.qywxOrigin) config.QYWX_ORIGIN = channel.qywxOrigin;
            break;
        case 'telegram':
            if (channel.tgBotToken) config.TG_BOT_TOKEN = channel.tgBotToken;
            if (channel.tgUserId) config.TG_USER_ID = channel.tgUserId;
            if (channel.tgApiHost) config.TG_API_HOST = channel.tgApiHost;
            if (channel.tgProxyHost) config.TG_PROXY_HOST = channel.tgProxyHost;
            if (channel.tgProxyPort) config.TG_PROXY_PORT = String(channel.tgProxyPort);
            if (channel.tgProxyAuth) config.TG_PROXY_AUTH = channel.tgProxyAuth;
            break;
        case 'serverchan':
            if (channel.pushKey) config.PUSH_KEY = channel.pushKey;
            break;
        case 'pushplus':
            if (channel.pushPlusToken) config.PUSH_PLUS_TOKEN = channel.pushPlusToken;
            if (channel.pushPlusUser) config.PUSH_PLUS_USER = channel.pushPlusUser;
            if (channel.pushPlusTemplate) config.PUSH_PLUS_TEMPLATE = channel.pushPlusTemplate;
            if (channel.pushPlusChannel) config.PUSH_PLUS_CHANNEL = channel.pushPlusChannel;
            if (channel.pushPlusWebhook) config.PUSH_PLUS_WEBHOOK = channel.pushPlusWebhook;
            if (channel.pushPlusCallbackUrl) config.PUSH_PLUS_CALLBACKURL = channel.pushPlusCallbackUrl;
            if (channel.pushPlusTo) config.PUSH_PLUS_TO = channel.pushPlusTo;
            break;
        case 'wechat_bot':
            if (channel.wechatBotId) config.WECHAT_BOT_ID = channel.wechatBotId;
            if (channel.wechatBotSecret) config.WECHAT_BOT_SECRET = channel.wechatBotSecret;
            if (channel.wechatBotChatId) config.WECHAT_BOT_CHAT_ID = channel.wechatBotChatId;
            // 默认使用企业微信开放平台 WebSocket 地址
            config.WECHAT_BOT_WS_URL = channel.wechatBotWsUrl || 'wss://openws.work.weixin.qq.com';
            break;
        case 'webhook':
            if (channel.webhookUrl) config.WEBHOOK_URL = channel.webhookUrl;
            if (channel.webhookMethod) config.WEBHOOK_METHOD = channel.webhookMethod;
            if (channel.webhookHeaders) config.WEBHOOK_HEADERS = channel.webhookHeaders;
            if (channel.webhookBody) config.WEBHOOK_BODY = channel.webhookBody;
            if (channel.webhookContentType) config.WEBHOOK_CONTENT_TYPE = channel.webhookContentType;
            break;
        case 'ntfy':
            if (channel.ntfyUrl) config.NTFY_URL = channel.ntfyUrl;
            if (channel.ntfyTopic) config.NTFY_TOPIC = channel.ntfyTopic;
            if (channel.ntfyPriority) config.NTFY_PRIORITY = String(channel.ntfyPriority);
            if (channel.ntfyToken) config.NTFY_TOKEN = channel.ntfyToken;
            if (channel.ntfyUsername) config.NTFY_USERNAME = channel.ntfyUsername;
            if (channel.ntfyPassword) config.NTFY_PASSWORD = channel.ntfyPassword;
            break;
        case 'gotify':
            if (channel.gotifyUrl) config.GOTIFY_URL = channel.gotifyUrl;
            if (channel.gotifyToken) config.GOTIFY_TOKEN = channel.gotifyToken;
            if (channel.gotifyPriority !== undefined) config.GOTIFY_PRIORITY = channel.gotifyPriority;
            break;
        case 'pushdeer':
            if (channel.deerKey) config.DEER_KEY = channel.deerKey;
            if (channel.deerUrl) config.DEER_URL = channel.deerUrl;
            break;
        case 'qqbot':
            if (channel.qqAppId) config.QQ_APP_ID = channel.qqAppId;
            if (channel.qqAppSecret) config.QQ_APP_SECRET = channel.qqAppSecret;
            if (channel.qqOpenId) config.QQ_OPENID = channel.qqOpenId;
            if (channel.qqGroupOpenId) config.QQ_GROUP_OPENID = channel.qqGroupOpenId;
            break;
        case 'igot':
            if (channel.igotPushKey) config.IGOT_PUSH_KEY = channel.igotPushKey;
            break;
        case 'chat':
            if (channel.chatUrl) config.CHAT_URL = channel.chatUrl;
            if (channel.chatToken) config.CHAT_TOKEN = channel.chatToken;
            break;
        case 'qmsg':
            if (channel.qmsgKey) config.QMSG_KEY = channel.qmsgKey;
            if (channel.qmsgType) config.QMSG_TYPE = channel.qmsgType;
            break;
        case 'pushme':
            if (channel.pushmeKey) config.PUSHME_KEY = channel.pushmeKey;
            break;
        case 'wxpusher':
            if (channel.wxpusherAppToken) config.WXPUSHER_APP_TOKEN = channel.wxpusherAppToken;
            if (channel.wxpusherTopicIds) config.WXPUSHER_TOPIC_IDS = channel.wxpusherTopicIds;
            if (channel.wxpusherUids) config.WXPUSHER_UIDS = channel.wxpusherUids;
            break;
        case 'aibotk':
            if (channel.aibotkKey) config.AIBOTK_KEY = channel.aibotkKey;
            if (channel.aibotkType) config.AIBOTK_TYPE = channel.aibotkType;
            if (channel.aibotkName) config.AIBOTK_NAME = channel.aibotkName;
            break;
        case 'weplusbot':
            if (channel.wePlusBotToken) config.WE_PLUS_BOT_TOKEN = channel.wePlusBotToken;
            if (channel.wePlusBotReceiver) config.WE_PLUS_BOT_RECEIVER = channel.wePlusBotReceiver;
            if (channel.wePlusBotVersion) config.WE_PLUS_BOT_VERSION = channel.wePlusBotVersion;
            break;
    }

    return config;
}

// 发送通知请求接口
export interface SendNotificationRequest {
    title: string;
    content: string;
    appName: string;
    logPath?: string;
    matchedLine?: string;
    rule?: any;
}

/**
 * 发送通知到指定渠道
 */
export async function sendToChannel(channel: any, title: string, content: string): Promise<{ success: boolean; channel: string; error?: string }> {
    if (!channel.enabled) {
        return {
            success: false,
            channel: channel.channel,
            error: '渠道已禁用'
        };
    }

    console.log(`[Notification] 渠道类型: ${channel.channel}, 渠道名称: ${channel.name}`);

    // 设置渠道配置
    const config = buildChannelConfig(channel);
    for (const [key, value] of Object.entries(config)) {
        setConfig(key as keyof ChannelConfig, value);
    }

    // 获取渠道名称并查找已注册的渠道
    const channelName = channelNameMap[channel.channel];
    const registeredChannel = registry.get(channelName);

    if (!registeredChannel) {
        return {
            success: false,
            channel: channel.channel,
            error: `渠道 ${channel.channel} 未注册`
        };
    }

    try {
        // 临时启用渠道并发送
        const wasEnabled = registeredChannel.enabled;
        registeredChannel.enabled = true;
        const result = await registeredChannel.send(title, content);
        registeredChannel.enabled = wasEnabled; // 恢复原状态

        return {
            success: result.success,
            channel: channel.channel,
            error: result.success ? undefined : (result.message || '发送失败')
        };
    } catch (err) {
        const error = err as Error;
        return {
            success: false,
            channel: channel.channel,
            error: error.message
        };
    }
}

/**
 * 发送通知
 */
export async function sendNotification(request: SendNotificationRequest): Promise<any[]> {
    const results: any[] = [];
    const { rule } = request;

    // 获取所有配置的渠道
    const channels = notificationStore.getChannels();
    const enabledChannels = channels.filter(c => rule?.channels?.includes(c.name) && c.enabled);

    if (enabledChannels.length === 0) {
        console.log(`[Notification] 规则 "${rule?.name}" 没有可用的通知渠道`);
        return results;
    }

    // 构建通知内容
    const now = new Date();
    const formattedTime = now.toLocaleString('zh-CN', {
        year: 'numeric',
        month: '2-digit',
        day: '2-digit',
        hour: '2-digit',
        minute: '2-digit',
        second: '2-digit',
        hour12: false
    });
    const title = `[应用通知] ${request.appName}`;
    const content = `时间: ${formattedTime}\n应用: ${request.appName}\n日志文件: ${request.logPath || 'N/A'}\n\n匹配内容:\n${request.matchedLine || request.content}\n\n规则: ${rule?.name || 'N/A'}`;

    // 发送到各个渠道
    for (const channel of enabledChannels) {
        try {
            const result = await sendToChannel(channel, title, content);
            results.push(result);

            // 记录历史
            const historyRecord = {
                id: generateId(),
                ruleId: rule?.id,
                ruleName: rule?.name,
                channel: channel.channel,
                title,
                content,
                appName: request.appName,
                logPath: request.logPath || '',
                matchedLine: request.matchedLine || '',
                success: result.success,
                error: result.error,
                timestamp: new Date()
            };

            await notificationStore.addHistory(historyRecord);

            if (result.success) {
                console.log(`[Notification] 发送成功: ${channel.name} (${channel.channel})`);
            } else {
                console.error(`[Notification] 发送失败: ${channel.name} (${channel.channel}) - ${result.error}`);
            }
        } catch (err) {
            const error = err as Error;
            console.error(`[Notification] 发送异常: ${channel.name} - ${error.message}`);
            results.push({
                success: false,
                channel: channel.channel,
                error: error.message
            });
        }
    }

    // 更新规则触发信息
    if (rule?.id) {
        await notificationStore.updateRuleTrigger(rule.id);
        notificationStore.recordNotification(rule.id);
    }

    return results;
}

/**
 * 测试通知渠道
 */
export async function testChannel(channel: any): Promise<{ success: boolean; error?: string }> {
    const title = '日志管理器 - 测试通知';
    const content = `这是一条测试通知消息。\n\n发送时间: ${new Date().toLocaleString('zh-CN')}\n渠道: ${channel.channel}`;
    return await sendToChannel(channel, title, content);
}

/**
 * 批量测试通知渠道
 */
export async function testChannels(channelNames: string[]): Promise<Record<string, any>> {
    const results: Record<string, any> = {};
    const channels = notificationStore.getChannels();

    for (const name of channelNames) {
        const channel = channels.find(c => c.name === name);
        if (channel) {
            results[name] = await testChannel(channel);
        } else {
            results[name] = {
                success: false,
                channel: 'unknown',
                error: '渠道不存在'
            };
        }
    }

    return results;
}

/**
 * 获取渠道类型的中文名称
 */
export function getChannelTypeName(channel: string): string {
    const names: Record<string, string> = {
        bark: 'Bark (iOS)',
        dingtalk: '钉钉机器人',
        feishu: '飞书机器人',
        feishu_app: '飞书企业应用',
        wecom: '企业微信机器人',
        wecom_app: '企业微信应用',
        wechat_bot: '企业微信智能机器人',
        telegram: 'Telegram',
        serverchan: 'Server酱',
        pushplus: 'PushPlus',
        webhook: '自定义Webhook',
        ntfy: 'Ntfy',
        gotify: 'Gotify',
        pushdeer: 'PushDeer',
        qqbot: 'QQ机器人',
        igot: 'iGot',
        chat: 'Synology Chat',
        qmsg: 'Qmsg',
        pushme: 'PushMe',
        wxpusher: 'WxPusher',
        aibotk: '智能微秘书',
        weplusbot: '微加机器人',
        chronocat: 'Chronocat QQ'
    };

    return names[channel] || channel;
}

/**
 * 获取渠道类型的配置字段
 */
export function getChannelConfigFields(channel: string): string[] {
    const fields: Record<string, string[]> = {
        bark: ['barkPush', 'barkSound', 'barkGroup', 'barkIcon', 'barkLevel', 'barkArchive', 'barkUrl'],
        dingtalk: ['ddBotToken', 'ddBotSecret'],
        feishu: ['fsKey', 'fsSecret'],
        feishu_app: ['feishuAppId', 'feishuAppSecret', 'feishuUserId'],
        wecom: ['qywxKey', 'qywxOrigin'],
        wecom_app: ['qywxAm', 'qywxOrigin'],
        wechat_bot: ['wechatBotId', 'wechatBotSecret', 'wechatBotChatId', 'wechatBotWsUrl'],
        telegram: ['tgBotToken', 'tgUserId', 'tgApiHost', 'tgProxyHost', 'tgProxyPort', 'tgProxyAuth'],
        serverchan: ['pushKey'],
        pushplus: ['pushPlusToken', 'pushPlusUser', 'pushPlusTemplate', 'pushPlusChannel', 'pushPlusWebhook', 'pushPlusCallbackUrl', 'pushPlusTo'],
        webhook: ['webhookUrl', 'webhookMethod', 'webhookHeaders', 'webhookBody', 'webhookContentType'],
        ntfy: ['ntfyUrl', 'ntfyTopic', 'ntfyPriority', 'ntfyToken', 'ntfyUsername', 'ntfyPassword'],
        gotify: ['gotifyUrl', 'gotifyToken', 'gotifyPriority'],
        pushdeer: ['deerKey', 'deerUrl'],
        qqbot: ['qqAppId', 'qqAppSecret', 'qqOpenId', 'qqGroupOpenId'],
        igot: ['igotPushKey'],
        chat: ['chatUrl', 'chatToken'],
        qmsg: ['qmsgKey', 'qmsgType'],
        pushme: ['pushmeKey'],
        wxpusher: ['wxpusherAppToken', 'wxpusherTopicIds', 'wxpusherUids'],
        aibotk: ['aibotkKey', 'aibotkType', 'aibotkName'],
        weplusbot: ['wePlusBotToken', 'wePlusBotReceiver', 'wePlusBotVersion'],
        chronocat: ['chronocatQq', 'chronocatToken', 'chronocatUrl']
    };

    return fields[channel] || [];
}
