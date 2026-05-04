/**
 * QQ机器人推送渠道（基于QQ开放平台API）
 * 文档: https://bot.q.qq.com/wiki/develop/api/
 */
import Logger from '../../utils/logger';
import { NotifyChannel, NotifyResult } from '../types';
import { $ } from '../httpClient';
import { hasConfig, getConfig } from '../config';
import WebSocket from 'ws';

const logger = Logger.child({ channel: 'qqbot' });

const QQ_API_BASE = 'https://api.sgroup.qq.com';
const QQ_TOKEN_URL = 'https://bots.qq.com/app/getAppAccessToken';

let qqCachedToken: { token: string; expiresAt: number; appId: string } | null = null;

// 捕获的 openID
let capturedOpenId: string | null = null;
let capturedGroupOpenId: string | null = null;

// WebSocket 监听
let wsClient: WebSocket | null = null;
let wsHeartbeatTimer: ReturnType<typeof setInterval> | null = null;
let wsReconnectAttempts = 0;
const WS_MAX_RECONNECT = 5;

async function getQQAccessToken(appId: string, appSecret: string): Promise<string> {
    const now = Date.now();

    if (qqCachedToken && now < qqCachedToken.expiresAt - 5 * 60 * 1000 && qqCachedToken.appId === appId) {
        return qqCachedToken.token;
    }

    if (qqCachedToken && qqCachedToken.appId !== appId) {
        qqCachedToken = null;
    }

    return new Promise((resolve, reject) => {
        $.post({
            url: QQ_TOKEN_URL,
            json: { appId, clientSecret: appSecret }
        }, (err, _resp, data) => {
            if (err) {
                reject(err);
            } else {
                const result = data as { access_token?: string; expires_in?: number };
                const token = result.access_token;
                if (!token) {
                    reject(new Error('获取Token失败'));
                    return;
                }
                const expiresIn = parseInt(String(result.expires_in)) || 7200;
                qqCachedToken = {
                    token,
                    expiresAt: now + expiresIn * 1000,
                    appId,
                };
                resolve(token);
            }
        });
    });
}

async function sendQQMessage(accessToken: string, target: string, content: string, isGroup: boolean, useMarkdown = false): Promise<any> {
    const path = isGroup
        ? `/v2/groups/${target}/messages`
        : `/v2/users/${target}/messages`;

    const body = useMarkdown
        ? { markdown: { content }, msg_type: 2 }
        : { content, msg_type: 0 };

    return new Promise((resolve, reject) => {
        $.post({
            url: `${QQ_API_BASE}${path}`,
            json: body,
            headers: { 'Authorization': `QQBot ${accessToken}` }
        }, (err, _resp, data) => {
            if (err) { reject(err); } else { resolve(data); }
        });
    });
}

/**
 * 处理QQ事件回调（HTTP webhook 方式）
 */
export function handleQQBotEvent(event: any): void {
    if (!event) return;
    const d = event.d || {};
    const t = event.t || '';

    if (t === 'C2C_MESSAGE_CREATE') {
        const openid = d?.author?.user_openid || d?.author?.member_openid;
        if (openid) {
            capturedOpenId = openid;
            logger.info({ openid }, 'QQ机器人捕获到用户 openID');
        }
    }

    if (t === 'GROUP_AT_MESSAGE_CREATE') {
        const groupOpenid = d?.group_openid;
        if (groupOpenid) {
            capturedGroupOpenId = groupOpenid;
            logger.info({ groupOpenid }, 'QQ机器人捕获到群 openID');
        }
    }
}

export function getCapturedOpenIds(): { openId: string | null; groupOpenId: string | null } {
    return { openId: capturedOpenId, groupOpenId: capturedGroupOpenId };
}

/**
 * 启动 QQ 机器人 WebSocket 监听（主动连接 QQ 平台，无需公网 IP）
 * 监听用户消息事件，自动捕获 openID
 */
export async function startQQBotListener(): Promise<{ success: boolean; message: string }> {
    if (wsClient && wsClient.readyState === WebSocket.OPEN) {
        return { success: true, message: '已在监听中' };
    }

    if (!hasConfig('QQ_APP_ID', 'QQ_APP_SECRET')) {
        return { success: false, message: '请先配置 QQ_APP_ID 和 QQ_APP_SECRET' };
    }

    const appId = getConfig('QQ_APP_ID') || '';
    const appSecret = getConfig('QQ_APP_SECRET') || '';

    try {
        logger.info({ appId }, 'QQ机器人获取AccessToken...');
        const accessToken = await getQQAccessToken(appId, appSecret);
        logger.info('QQ机器人AccessToken获取成功');

        logger.info('QQ机器人获取WebSocket gateway URL...');
        const gatewayUrl = await new Promise<string>((resolve, reject) => {
            $.get({
                url: `${QQ_API_BASE}/gateway`,
                headers: { 'Authorization': `QQBot ${accessToken}` }
            }, (err: any, _resp: any, data: any) => {
                if (err) {
                    logger.warn({ err }, 'QQ机器人获取gateway失败');
                    reject(err);
                } else {
                    try {
                        const result = typeof data === 'string' ? JSON.parse(data) : data;
                        if (result?.url) {
                            logger.info({ url: result.url }, 'QQ机器人获取gateway成功');
                            resolve(result.url);
                        } else {
                            reject(new Error('获取gateway失败: ' + JSON.stringify(data).substring(0, 200)));
                        }
                    } catch (parseErr) {
                        reject(new Error('解析gateway响应失败'));
                    }
                }
            });
        });

        // 等待 WebSocket 实际连接成功
        const connected = await new Promise<boolean>((resolve) => {
            const socket = new WebSocket(gatewayUrl);
            const timeout = setTimeout(() => {
                logger.warn('QQ机器人WebSocket连接超时');
                socket.close();
                resolve(false);
            }, 10000);

            socket.on('open', () => {
                clearTimeout(timeout);
                logger.info('QQ机器人 WebSocket 已连接');
                wsClient = socket;
                wsReconnectAttempts = 0;

                socket.send(JSON.stringify({
                    op: 2,
                    d: {
                        token: `QQBot ${accessToken}`,
                        intents: (1 << 30) | (1 << 25),
                        shard: [0, 1]
                    }
                }));
                resolve(true);
            });

            socket.on('message', (data: WebSocket.Data) => {
                try {
                    const payload = JSON.parse(data.toString());
                    logger.info({ op: payload.op, t: payload.t, d: payload.d }, 'QQ机器人 WebSocket 收到消息');
                    if (payload.op === 10) {
                        const heartbeatInterval = payload.d?.heartbeat_interval || 30000;
                        if (wsHeartbeatTimer) clearInterval(wsHeartbeatTimer);
                        wsHeartbeatTimer = setInterval(() => {
                            if (wsClient && wsClient.readyState === WebSocket.OPEN) {
                                wsClient.send(JSON.stringify({ op: 1, d: null }));
                            }
                        }, heartbeatInterval);
                    }
                    if (payload.op === 0 && payload.t) {
                        handleQQBotEvent(payload);
                    }
                } catch (parseErr) {
                    logger.warn({ err: parseErr }, 'QQ机器人 WebSocket 消息解析失败');
                }
            });

            socket.on('close', () => {
                clearTimeout(timeout);
                logger.info('QQ机器人 WebSocket 已断开');
                if (wsHeartbeatTimer) { clearInterval(wsHeartbeatTimer); wsHeartbeatTimer = null; }
                wsClient = null;
            });

            socket.on('error', (err) => {
                clearTimeout(timeout);
                logger.warn({ err }, 'QQ机器人 WebSocket 错误');
                wsClient = null;
                resolve(false);
            });
        });

        if (connected) {
            return { success: true, message: 'WebSocket已连接，等待消息中' };
        } else {
            return { success: false, message: '无法连接QQ平台WebSocket，请检查网络和配置' };
        }
    } catch (err) {
        logger.error({ err }, 'QQ机器人启动监听失败');
        return { success: false, message: `启动失败: ${(err as Error)?.message || '未知错误'}` };
    }
}

export function stopQQBotListener(): void {
    if (wsHeartbeatTimer) { clearInterval(wsHeartbeatTimer); wsHeartbeatTimer = null; }
    if (wsClient) { wsClient.close(); wsClient = null; }
    logger.info('QQ机器人监听已停止');
}

export const qqbotChannel: NotifyChannel = {
    name: 'qqbot',
    enabled: false,

    async send(text: string, desp: string): Promise<NotifyResult> {
        return new Promise(async (resolve) => {
            if (!hasConfig('QQ_APP_ID', 'QQ_APP_SECRET')) {
                resolve({ success: false, message: 'QQ机器人配置不完整，需要 QQ_APP_ID 和 QQ_APP_SECRET' });
                return;
            }

            const QQ_APP_ID = getConfig('QQ_APP_ID') || '';
            const QQ_APP_SECRET = getConfig('QQ_APP_SECRET') || '';
            const QQ_OPENID = getConfig('QQ_OPENID');
            const QQ_GROUP_OPENID = getConfig('QQ_GROUP_OPENID');

            logger.info('QQ机器人 服务启动');

            const targets: Array<{ id: string; isGroup: boolean }> = [];
            if (QQ_GROUP_OPENID) { targets.push({ id: QQ_GROUP_OPENID, isGroup: true }); }
            if (QQ_OPENID) { targets.push({ id: QQ_OPENID, isGroup: false }); }

            if (targets.length === 0) {
                const captured = getCapturedOpenIds();
                let hint = '未配置 openID。请点击"获取openID"按钮，然后给机器人发一条消息';
                if (captured.openId) {
                    hint += `\n\n已捕获到用户 openID: ${captured.openId}，请填入 QQ_OPENID`;
                }
                if (captured.groupOpenId) {
                    hint += `\n已捕获到群 openID: ${captured.groupOpenId}，请填入 QQ_GROUP_OPENID`;
                }
                resolve({ success: false, message: hint });
                return;
            }

            const content = `**${text}**\n\n${desp}`;

            try {
                const accessToken = await getQQAccessToken(QQ_APP_ID, QQ_APP_SECRET);
                let successCount = 0;

                for (const target of targets) {
                    try {
                        await sendQQMessage(accessToken, target.id, content, target.isGroup, true);
                        successCount++;
                    } catch (err) {
                        const errMsg = (err as Error)?.message || String(err);
                        if (errMsg.includes('markdown') || errMsg.includes('11244') || errMsg.includes('权限')) {
                            try {
                                await sendQQMessage(accessToken, target.id, `【${text}】\n\n${desp}`, target.isGroup, false);
                                successCount++;
                            } catch (fallbackErr) {
                                logger.warn({ target: target.id, err: fallbackErr }, 'QQ机器人发送失败');
                            }
                        } else {
                            logger.warn({ target: target.id, err }, 'QQ机器人发送失败');
                        }
                    }
                }

                if (successCount > 0) {
                    logger.info('QQ机器人推送成功');
                    resolve({ success: true, message: '发送成功' });
                } else {
                    resolve({ success: false, message: '发送失败' });
                }
            } catch (err) {
                logger.error({ err }, 'QQ机器人推送失败');
                resolve({ success: false, message: (err as Error)?.message || '推送失败' });
            }
        });
    }
};

if (hasConfig('QQ_APP_ID', 'QQ_APP_SECRET')) {
    qqbotChannel.enabled = true;
}

export default qqbotChannel;
