/**
 * 通知实时推送 WebSocket 服务
 * 替代前端轮询，通过 WebSocket 推送监控状态变更和通知历史更新
 */

import { WebSocketServer, WebSocket } from 'ws';
import { Server } from 'http';
import Logger from '../utils/logger';
import * as sessionService from '../services/session';

const logger = Logger.child({ module: 'NotifyWS' });

interface NotifyClient {
  ws: WebSocket;
  subscriptions: Set<string>; // 'monitor' | 'history' | 'rules'
}

const clients: Map<WebSocket, NotifyClient> = new Map();
let wss: WebSocketServer | null = null;

/**
 * 从请求头中解析 cookie 获取 session token
 */
function getSessionTokenFromRequest(req: any): string {
    const cookieHeader = req.headers.cookie || '';
    const match = cookieHeader.match(/session_token=([^;]+)/);
    return match ? match[1] : '';
}

/**
 * 初始化通知 WebSocket 服务器
 */
export function initNotifyWebSocket(server: Server): void {
    wss = new WebSocketServer({
        server,
        path: '/api/notifications/ws',
        verifyClient: (info, callback) => {
            const origin = info.origin || '';
            const host = info.req.headers.host || '';
            if (!origin || origin.includes(host) || origin.includes('5ddd.com')) {
                // Origin 检查通过，继续验证 Session
            } else {
                logger.warn({ origin, host }, 'NotifyWS connection rejected: origin mismatch');
                callback(false, 403, 'Forbidden');
                return;
            }

            // 验证 Session Token
            const token = getSessionTokenFromRequest(info.req);
            if (!token || !sessionService.validateSession(token)) {
                logger.warn('NotifyWS connection rejected: invalid or missing session token');
                callback(false, 401, 'Unauthorized');
                return;
            }

            callback(true);
        }
    });

    wss.on('connection', (ws) => {
        const client: NotifyClient = {
            ws,
            subscriptions: new Set()
        };
        clients.set(ws, client);

        ws.on('message', (data) => {
            try {
                const message = JSON.parse(data.toString());
                if (message.type === 'subscribe' && message.channels) {
                    for (const ch of message.channels) {
                        if (['monitor', 'history', 'rules'].includes(ch)) {
                            client.subscriptions.add(ch);
                        }
                    }
                } else if (message.type === 'unsubscribe' && message.channels) {
                    for (const ch of message.channels) {
                        client.subscriptions.delete(ch);
                    }
                }
            } catch {
                // Ignore invalid messages
            }
        });

        ws.on('close', () => {
            clients.delete(ws);
        });

        ws.on('error', () => {
            clients.delete(ws);
        });

        ws.send(JSON.stringify({ type: 'connected' }));
    });

    logger.info('Notification WebSocket server initialized');
}

/**
 * 向订阅了指定频道的客户端广播消息
 */
export function broadcast(channel: string, data: any): void {
    const message = JSON.stringify({
        type: 'update',
        channel,
        data,
        timestamp: Date.now()
    });

    for (const [ws, client] of clients) {
        if (ws.readyState === WebSocket.OPEN && client.subscriptions.has(channel)) {
            try {
                ws.send(message);
            } catch {
                clients.delete(ws);
            }
        }
    }
}

/**
 * 关闭通知 WebSocket 服务器
 */
export function closeNotifyWebSocket(): void {
    for (const [ws] of clients) {
        if (ws.readyState === WebSocket.OPEN) {
            ws.close(1001, 'Server shutting down');
        }
    }
    clients.clear();

    if (wss) {
        wss.close();
        wss = null;
    }

    logger.info('Notification WebSocket server closed');
}

/**
 * 获取连接统计
 */
export function getNotifyWSStats(): { totalClients: number; subscriptions: Record<string, number> } {
    const subscriptions: Record<string, number> = { monitor: 0, history: 0, rules: 0 };
    for (const [, client] of clients) {
        for (const ch of client.subscriptions) {
            subscriptions[ch] = (subscriptions[ch] || 0) + 1;
        }
    }
    return { totalClients: clients.size, subscriptions };
}
