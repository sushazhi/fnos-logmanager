/**
 * 企业微信智能机器人推送渠道（WebSocket 长连接模式）
 */
import crypto from 'crypto';
import WebSocket from 'ws';
import Logger from '../../utils/logger';
import { NotifyChannel, NotifyResult } from '../types';
import { hasConfig, getConfig } from '../config';

const logger = Logger.child({ channel: 'wechat-bot' });

// 单例客户端
let wechatBotClient: WechatBotClient | null = null;

class WechatBotClient {
    private botId: string;
    private botSecret: string;
    private wsUrl: string;
    private ws: WebSocket | null = null;
    private authenticated = false;
    private pendingAcks: Map<string, { resolve: (data: any) => void }> = new Map();
    private knownTargets: Set<string> = new Set();
    private heartbeatInterval: NodeJS.Timeout | null = null;
    private reconnectTimer: NodeJS.Timeout | null = null;

    constructor(botId: string, botSecret: string, wsUrl: string) {
        this.botId = botId;
        this.botSecret = botSecret;
        this.wsUrl = wsUrl;
        this.connect();
    }

    private buildReqId(prefix: string): string {
        return `${prefix}_${crypto.randomUUID().replace(/-/g, '')}`;
    }

    private connect(): void {
        if (this.ws) return;

        try {
            this.ws = new WebSocket(this.wsUrl);

            this.ws.on('open', () => {
                logger.info('企业微信智能机器人连接成功，开始订阅...');
                this.sendRaw({
                    cmd: 'aibot_subscribe',
                    headers: { req_id: this.buildReqId('aibot_subscribe') },
                    body: {
                        bot_id: this.botId,
                        secret: this.botSecret,
                    },
                });
            });

            this.ws.on('message', (data: WebSocket.RawData) => {
                try {
                    const payload = JSON.parse(data.toString());
                    this.handleMessage(payload);
                } catch (e) {
                    logger.error({ err: e }, '解析企业微信智能机器人消息失败');
                }
            });

            this.ws.on('error', (err: Error) => {
                logger.error({ err: err.message }, '企业微信智能机器人 WebSocket 错误');
                this.authenticated = false;
            });

            this.ws.on('close', () => {
                logger.info('企业微信智能机器人连接关闭');
                this.authenticated = false;
                this.ws = null;
                this.scheduleReconnect();
            });

            this.startHeartbeat();
        } catch (e) {
            const err = e as Error;
            logger.error({ err: err.message }, '企业微信智能机器人连接失败');
            this.scheduleReconnect();
        }
    }

    private scheduleReconnect(): void {
        if (this.reconnectTimer) return;
        this.reconnectTimer = setTimeout(() => {
            this.reconnectTimer = null;
            this.connect();
        }, 10000);
    }

    private startHeartbeat(): void {
        if (this.heartbeatInterval) {
            clearInterval(this.heartbeatInterval);
        }
        this.heartbeatInterval = setInterval(() => {
            if (this.authenticated && this.ws && this.ws.readyState === WebSocket.OPEN) {
                this.sendRaw({
                    cmd: 'ping',
                    headers: { req_id: this.buildReqId('ping') },
                });
            }
        }, 30000);
    }

    private sendRaw(payload: any): void {
        if (this.ws && this.ws.readyState === WebSocket.OPEN) {
            this.ws.send(JSON.stringify(payload));
        }
    }

    private handleMessage(payload: any): void {
        const reqId = (payload.headers || {}).req_id;

        // 处理 ACK
        if (reqId && this.pendingAcks.has(reqId)) {
            const pending = this.pendingAcks.get(reqId);
            this.pendingAcks.delete(reqId);
            pending?.resolve(payload);
        }

        // 处理订阅响应
        if (String(reqId).startsWith('aibot_subscribe')) {
            if (payload.errcode === 0) {
                this.authenticated = true;
                logger.info('企业微信智能机器人订阅成功');
            } else {
                logger.error({ msg: payload.errmsg }, '企业微信智能机器人订阅失败');
            }
            return;
        }

        // 处理消息回调
        const cmd = payload.cmd;
        if (cmd === 'aibot_msg_callback') {
            this.handleCallbackMessage(payload);
        }
    }

    private handleCallbackMessage(payload: any): void {
        const body = payload.body || {};
        const sender = (body.from || {}).userid || '';
        if (!sender) return;

        if (!this.knownTargets.has(sender)) {
            this.knownTargets.add(sender);
            logger.info({ sender }, '企业微信智能机器人记录用户');
        }
    }

    private async sendWithAck(payload: any, timeout = 10000): Promise<any> {
        return new Promise((resolve) => {
            const reqId = (payload.headers || {}).req_id;
            if (!reqId) {
                resolve({ errcode: -1, errmsg: '缺少 req_id' });
                return;
            }

            const timer = setTimeout(() => {
                this.pendingAcks.delete(reqId);
                resolve({ errcode: -1, errmsg: '发送超时' });
            }, timeout);

            this.pendingAcks.set(reqId, {
                resolve: (data) => {
                    clearTimeout(timer);
                    resolve(data);
                },
            });

            this.sendRaw(payload);
        });
    }

    private normalizeTarget(chatId: string): { target: string | null; chatType: number } {
        if (!chatId) return { target: null, chatType: 1 };
        const lowered = chatId.toLowerCase();
        if (lowered.startsWith('group:')) {
            return { target: chatId.substring(6).trim(), chatType: 2 };
        }
        if (lowered.startsWith('user:')) {
            return { target: chatId.substring(5).trim(), chatType: 1 };
        }
        return { target: chatId.trim(), chatType: 1 };
    }

    private getTargets(chatId: string): Array<{ target: string; chatType: number }> {
        const { target, chatType } = this.normalizeTarget(chatId);
        if (target) {
            return [{ target, chatType }];
        }
        // 如果没有指定目标，发送给所有已互动用户
        return Array.from(this.knownTargets).map((userid) => ({
            target: userid,
            chatType: 1,
        }));
    }

    private splitContent(content: string, maxBytes = 4000): string[] {
        if (!content) return [];
        const chunks: string[] = [];
        let current = '';
        for (const line of content.split('\n')) {
            const newContent = current + (current ? '\n' : '') + line;
            if (Buffer.byteLength(newContent, 'utf8') > maxBytes) {
                if (current) chunks.push(current);
                current = line;
            } else {
                current = newContent;
            }
        }
        if (current) chunks.push(current);
        return chunks;
    }

    async sendMarkdown(content: string, chatId: string): Promise<boolean> {
        if (!this.authenticated) {
            logger.info('企业微信智能机器人未认证，尝试连接...');
            return false;
        }

        const targets = this.getTargets(chatId);
        if (targets.length === 0) {
            logger.info('企业微信智能机器人没有可发送的目标');
            return false;
        }

        let success = false;
        for (const { target, chatType } of targets) {
            for (const chunk of this.splitContent(content)) {
                const payload = {
                    cmd: 'aibot_send_msg',
                    headers: { req_id: this.buildReqId('aibot_send_msg') },
                    body: {
                        chatid: target,
                        chat_type: chatType,
                        msgtype: 'markdown',
                        markdown: { content: chunk },
                    },
                };
                const ack = await this.sendWithAck(payload);
                if (ack.errcode === 0) {
                    success = true;
                } else {
                    logger.error({ msg: ack.errmsg }, '企业微信智能机器人发送失败');
                }
            }
        }
        return success;
    }

    async sendMsg(title: string, text: string, chatId: string): Promise<boolean> {
        const content = `**${title}**\n\n${text || ''}`.trim();
        return this.sendMarkdown(content, chatId);
    }

    isAuthenticated(): boolean {
        return this.authenticated;
    }

    async waitForAuth(maxWaitTime = 10000): Promise<boolean> {
        const checkInterval = 500;
        let waitedTime = 0;

        while (!this.authenticated && waitedTime < maxWaitTime) {
            logger.info(`企业微信智能机器人等待认证... (${waitedTime / 1000}s)`);
            await new Promise(resolve => setTimeout(resolve, checkInterval));
            waitedTime += checkInterval;
        }

        return this.authenticated;
    }
}

function getWechatBotClient(): WechatBotClient | null {
    if (wechatBotClient) {
        return wechatBotClient;
    }

    if (!hasConfig('WECHAT_BOT_ID', 'WECHAT_BOT_SECRET')) {
        return null;
    }

    const WECHAT_BOT_ID = getConfig('WECHAT_BOT_ID') || '';
    const WECHAT_BOT_SECRET = getConfig('WECHAT_BOT_SECRET') || '';
    const WECHAT_BOT_WS_URL = getConfig('WECHAT_BOT_WS_URL') || 'wss://openws.work.weixin.qq.com';

    wechatBotClient = new WechatBotClient(WECHAT_BOT_ID, WECHAT_BOT_SECRET, WECHAT_BOT_WS_URL);
    return wechatBotClient;
}

export const wechatBotChannel: NotifyChannel = {
    name: 'wechat-bot',
    enabled: false,

    async send(text: string, desp: string): Promise<NotifyResult> {
        const client = getWechatBotClient();
        if (!client) {
            return { success: false, message: '企业微信智能机器人配置不完整' };
        }

        const WECHAT_BOT_CHAT_ID = getConfig('WECHAT_BOT_CHAT_ID') || '';

        // 等待认证完成
        const authenticated = await client.waitForAuth(10000);
        if (!authenticated) {
            return { success: false, message: '企业微信智能机器人认证超时' };
        }

        try {
            const success = await client.sendMsg(text, desp, WECHAT_BOT_CHAT_ID);
            if (success) {
                logger.info('企业微信智能机器人发送成功');
                return { success: true, message: '发送成功' };
            } else {
                return { success: false, message: '发送失败' };
            }
        } catch (err) {
            const error = err as Error;
            logger.error({ err: error.message }, '企业微信智能机器人发送失败');
            return { success: false, error };
        }
    }
};

if (hasConfig('WECHAT_BOT_ID', 'WECHAT_BOT_SECRET')) {
    wechatBotChannel.enabled = true;
}

export default wechatBotChannel;
