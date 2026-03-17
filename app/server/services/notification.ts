/**
 * 通知发送服务
 * 封装 sendNotify.js 的功能，提供统一的通知发送接口
 */

import { spawn } from 'child_process';
import path from 'path';
import fs from 'fs';
import * as notificationStore from './notificationStore';

/**
 * 获取 sendNotify.js 的路径
 */
function getSendNotifyPath(): string {
    const possiblePaths = [
        // 开发环境：项目根目录（优先检查）
        path.join(__dirname, '../../../sendNotify.js'),
        // 生产环境：服务端目录同级
        path.join(__dirname, '../sendNotify.js'),
        // 备用：当前目录
        path.join(process.cwd(), 'sendNotify.js')
    ];

    for (const p of possiblePaths) {
        if (fs.existsSync(p)) {
            console.log(`[Notification] 找到 sendNotify.js: ${p}`);
            return p;
        }
    }

    // 默认返回第一个路径
    const defaultPath = possiblePaths[0];
    console.error(`[Notification] 未找到 sendNotify.js，尝试路径: ${possiblePaths.join(', ')}`);
    return defaultPath;
}

const SEND_NOTIFY_PATH = getSendNotifyPath();
const SEND_NOTIFY_DIR = path.dirname(SEND_NOTIFY_PATH);

/**
 * 生成唯一ID
 */
function generateId(): string {
    return `${Date.now()}-${Math.random().toString(36).substr(2, 9)}`;
}

/**
 * 构建环境变量配置
 */
function buildEnvConfig(channel: any): Record<string, string> {
    const env: Record<string, string> = {};

    switch (channel.channel) {
        case 'bark':
            if (channel.barkPush) env.BARK_PUSH = channel.barkPush;
            if (channel.barkSound) env.BARK_SOUND = channel.barkSound;
            if (channel.barkGroup) env.BARK_GROUP = channel.barkGroup;
            if (channel.barkIcon) env.BARK_ICON = channel.barkIcon;
            if (channel.barkLevel) env.BARK_LEVEL = channel.barkLevel;
            if (channel.barkArchive) env.BARK_ARCHIVE = channel.barkArchive;
            if (channel.barkUrl) env.BARK_URL = channel.barkUrl;
            break;
        case 'dingtalk':
            if (channel.ddBotToken) env.DD_BOT_TOKEN = channel.ddBotToken;
            if (channel.ddBotSecret) env.DD_BOT_SECRET = channel.ddBotSecret;
            break;
        case 'feishu':
            // 自定义机器人
            if (channel.fsKey) env.FSKEY = channel.fsKey;
            if (channel.fsSecret) env.FSSECRET = channel.fsSecret;
            break;
        case 'feishu_app':
            // 企业自建应用
            if (channel.feishuAppId) env.FEISHU_APP_ID = channel.feishuAppId;
            if (channel.feishuAppSecret) env.FEISHU_APP_SECRET = channel.feishuAppSecret;
            if (channel.feishuUserId) env.FEISHU_USER_ID = channel.feishuUserId;
            break;
        case 'wecom':
            if (channel.qywxKey) env.QYWX_KEY = channel.qywxKey;
            if (channel.qywxOrigin) env.QYWX_ORIGIN = channel.qywxOrigin;
            break;
        case 'wecom_app':
            if (channel.qywxAm) env.QYWX_AM = channel.qywxAm;
            if (channel.qywxOrigin) env.QYWX_ORIGIN = channel.qywxOrigin;
            break;
        case 'telegram':
            if (channel.tgBotToken) env.TG_BOT_TOKEN = channel.tgBotToken;
            if (channel.tgUserId) env.TG_USER_ID = channel.tgUserId;
            if (channel.tgApiHost) env.TG_API_HOST = channel.tgApiHost;
            if (channel.tgProxyHost) env.TG_PROXY_HOST = channel.tgProxyHost;
            if (channel.tgProxyPort) env.TG_PROXY_PORT = String(channel.tgProxyPort);
            if (channel.tgProxyAuth) env.TG_PROXY_AUTH = channel.tgProxyAuth;
            break;
        case 'serverchan':
            if (channel.pushKey) env.PUSH_KEY = channel.pushKey;
            break;
        case 'pushplus':
            if (channel.pushPlusToken) env.PUSH_PLUS_TOKEN = channel.pushPlusToken;
            if (channel.pushPlusUser) env.PUSH_PLUS_USER = channel.pushPlusUser;
            if (channel.pushPlusTemplate) env.PUSH_PLUS_TEMPLATE = channel.pushPlusTemplate;
            if (channel.pushPlusChannel) env.PUSH_PLUS_CHANNEL = channel.pushPlusChannel;
            if (channel.pushPlusWebhook) env.PUSH_PLUS_WEBHOOK = channel.pushPlusWebhook;
            if (channel.pushPlusCallbackUrl) env.PUSH_PLUS_CALLBACKURL = channel.pushPlusCallbackUrl;
            if (channel.pushPlusTo) env.PUSH_PLUS_TO = channel.pushPlusTo;
            break;
        case 'wechat_bot':
            if (channel.wechatBotId) env.WECHAT_BOT_ID = channel.wechatBotId;
            if (channel.wechatBotSecret) env.WECHAT_BOT_SECRET = channel.wechatBotSecret;
            if (channel.wechatBotChatId) env.WECHAT_BOT_CHAT_ID = channel.wechatBotChatId;
            // 默认使用企业微信开放平台 WebSocket 地址
            env.WECHAT_BOT_WS_URL = channel.wechatBotWsUrl || 'wss://openws.work.weixin.qq.com';
            break;
        case 'webhook':
            if (channel.webhookUrl) env.WEBHOOK_URL = channel.webhookUrl;
            if (channel.webhookMethod) env.WEBHOOK_METHOD = channel.webhookMethod;
            if (channel.webhookHeaders) env.WEBHOOK_HEADERS = channel.webhookHeaders;
            if (channel.webhookBody) env.WEBHOOK_BODY = channel.webhookBody;
            if (channel.webhookContentType) env.WEBHOOK_CONTENT_TYPE = channel.webhookContentType;
            break;
        case 'ntfy':
            if (channel.ntfyUrl) env.NTFY_URL = channel.ntfyUrl;
            if (channel.ntfyTopic) env.NTFY_TOPIC = channel.ntfyTopic;
            if (channel.ntfyPriority) env.NTFY_PRIORITY = String(channel.ntfyPriority);
            if (channel.ntfyToken) env.NTFY_TOKEN = channel.ntfyToken;
            if (channel.ntfyUsername) env.NTFY_USERNAME = channel.ntfyUsername;
            if (channel.ntfyPassword) env.NTFY_PASSWORD = channel.ntfyPassword;
            break;
        case 'gotify':
            if (channel.gotifyUrl) env.GOTIFY_URL = channel.gotifyUrl;
            if (channel.gotifyToken) env.GOTIFY_TOKEN = channel.gotifyToken;
            if (channel.gotifyPriority !== undefined) env.GOTIFY_PRIORITY = String(channel.gotifyPriority);
            break;
        case 'pushdeer':
            if (channel.deerKey) env.DEER_KEY = channel.deerKey;
            if (channel.deerUrl) env.DEER_URL = channel.deerUrl;
            break;
        case 'qqbot':
            if (channel.qqAppId) env.QQ_APP_ID = channel.qqAppId;
            if (channel.qqAppSecret) env.QQ_APP_SECRET = channel.qqAppSecret;
            if (channel.qqOpenId) env.QQ_OPENID = channel.qqOpenId;
            if (channel.qqGroupOpenId) env.QQ_GROUP_OPENID = channel.qqGroupOpenId;
            break;
    }

    return env;
}

/**
 * 调用 sendNotify.js 发送通知
 */
async function callSendNotify(title: string, content: string, envConfig: Record<string, string>): Promise<{ success: boolean; error?: string; openid?: string; groupOpenid?: string }> {
    return new Promise((resolve) => {
        const env = { ...process.env, ...envConfig };

        // 创建一个简单的脚本来调用 sendNotify
        const script = `
            const { sendNotify } = require('${SEND_NOTIFY_PATH.replace(/\\/g, '\\\\')}');
            sendNotify(${JSON.stringify(title)}, ${JSON.stringify(content)})
                .then(() => process.exit(0))
                .catch((err) => {
                    console.error(err);
                    process.exit(1);
                });
        `;

        console.log(`[Notification] ====== 开始发送通知 ======`);
        console.log(`[Notification] 标题: ${title}`);
        console.log(`[Notification] 内容: ${content.substring(0, 100)}...`);
        console.log(`[Notification] sendNotify路径: ${SEND_NOTIFY_PATH}`);
        console.log(`[Notification] 环境变量: ${Object.keys(envConfig).join(', ')}`);

        const proc = spawn('node', ['-e', script], {
            env,
            cwd: SEND_NOTIFY_DIR,
            stdio: ['ignore', 'pipe', 'pipe']
        });

        let stdout = '';
        let stderr = '';

        proc.stdout?.on('data', (data) => {
            const str = data.toString();
            stdout += str;
            console.log(`[Notification] stdout: ${str}`);
        });

        proc.stderr?.on('data', (data) => {
            const str = data.toString();
            stderr += str;
            console.log(`[Notification] stderr: ${str}`);
        });

        proc.on('close', (code) => {
            console.log(`[Notification] 进程退出码: ${code}`);
            console.log(`[Notification] ====== 发送完成 ======`);

            // 从输出中提取 openid
            const openidMatch = stdout.match(/用户 openid:\s*([a-zA-Z0-9_-]+)/);
            const groupOpenidMatch = stdout.match(/群 openid:\s*([a-zA-Z0-9_-]+)/);

            // 检查是否有错误标志
            const hasError = stdout.includes('会话无效') || 
                            stdout.includes('监听结束（60秒超时）') ||
                            stdout.includes('WebSocket错误') ||
                            stdout.includes('连接已关闭') ||
                            code !== 0;

            const result: { success: boolean; openid?: string; groupOpenid?: string; error?: string } = {
                success: !hasError && (openidMatch !== null || groupOpenidMatch !== null || code === 0),
                openid: openidMatch ? openidMatch[1] : undefined,
                groupOpenid: groupOpenidMatch ? groupOpenidMatch[1] : undefined
            };

            if (hasError) {
                // 提取错误信息
                if (stdout.includes('会话无效')) {
                    result.error = 'QQ机器人: 会话无效，请检查 intents 权限配置';
                } else if (stdout.includes('监听结束（60秒超时）')) {
                    result.error = 'QQ机器人: 60秒内未收到消息，请确保已给机器人发送消息';
                } else if (stdout.includes('WebSocket错误')) {
                    result.error = 'QQ机器人: WebSocket 连接错误';
                } else if (code !== 0) {
                    result.error = stderr || stdout || `进程退出码: ${code}`;
                }
            }

            resolve(result);
        });

        proc.on('error', (err) => {
            console.log(`[Notification] spawn错误: ${err.message}`);
            resolve({
                success: false,
                error: err.message
            });
        });
    });
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
export async function sendToChannel(channel: any, title: string, content: string): Promise<{ success: boolean; channel: string; error?: string; openid?: string; groupOpenid?: string }> {
    if (!channel.enabled) {
        return {
            success: false,
            channel: channel.channel,
            error: '渠道已禁用'
        };
    }

    console.log(`[Notification] 渠道类型: ${channel.channel}, 渠道名称: ${channel.name}`);
    console.log(`[Notification] 渠道配置:`, JSON.stringify(channel).replace(/"[^"]*secret[^"]*":\s*"[^"]*"/g, '"****"').replace(/"[^"]*key[^"]*":\s*"[^"]*"/g, '"****"'));

    const envConfig = buildEnvConfig(channel);
    console.log(`[Notification] 构建的环境变量:`, Object.keys(envConfig));
    const result = await callSendNotify(title, content, envConfig);

    return {
        success: result.success,
        channel: channel.channel,
        error: result.error,
        openid: result.openid,
        groupOpenid: result.groupOpenid
    };
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
    const title = `[应用通知] ${request.appName}`;
    const content = `应用: ${request.appName}\n日志文件: ${request.logPath || 'N/A'}\n\n匹配内容:\n${request.matchedLine || request.content}\n\n规则: ${rule?.name || 'N/A'}`;

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
        qqbot: 'QQ机器人'
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
        qqbot: ['qqAppId', 'qqAppSecret', 'qqOpenId', 'qqGroupOpenId']
    };

    return fields[channel] || [];
}
