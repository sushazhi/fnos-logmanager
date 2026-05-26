/**
 * 通知实时推送 WebSocket 服务
 * 替代前端轮询，通过 WebSocket 推送监控状态变更和通知历史更新
 */

import { WebSocketServer, WebSocket } from 'ws';
import { Server } from 'http';
import Logger from '../utils/logger';
import * as sessionService from '../services/session';
import * as notificationStore from '../services/notificationStore';
import * as logMonitor from '../services/logMonitor';

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
    if (match) return match[1];
    const url = new URL(req.url || '', 'http://localhost');
    return url.searchParams.get('token') || '';
}

/**
 * 初始化通知 WebSocket 服务器
 */
export function initNotifyWebSocket(_server: Server): void {
    wss = new WebSocketServer({
        noServer: true,
        verifyClient: (info, callback) => {
            // 通过 Unix Socket 来的连接（网关代理）：由网关处理认证
            const isUnixSocket = info.req.socket.remoteFamily === 'AF_UNIX' || !info.req.socket.localAddress;
            if (isUnixSocket && process.env.GATEWAY_SOCKET) {
                callback(true);
                return;
            }
            // 直连 TCP 连接：验证 session token
            const token = getSessionTokenFromRequest(info.req);
            if (!token || !sessionService.validateSession(token)) {
                logger.warn('NotifyWS connection rejected: invalid or missing session token');
                callback(false, 401, 'Unauthorized');
                return;
            }

            const origin = info.req.headers.origin || '';
            if (origin) {
                const host = info.req.headers.host || '';
                try {
                    const originHost = new URL(origin).hostname;
                    const requestHost = host.split(':')[0];
                    if (originHost !== requestHost) {
                        logger.warn({ origin, host }, 'NotifyWS connection rejected: origin mismatch');
                        callback(false, 403, 'Forbidden');
                        return;
                    }
                } catch {
                    callback(false, 403, 'Forbidden');
                    return;
                }
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

        // 连接后自动推送当前状态
        try {
            const status = logMonitor.getStatus();
            ws.send(JSON.stringify({ type: 'status', data: status }));
            const rules = notificationStore.getRules();
            ws.send(JSON.stringify({ type: 'rules', data: rules }));
            const history = notificationStore.getHistory(5);
            ws.send(JSON.stringify({ type: 'history', data: history }));
        } catch (err) {
            logger.warn({ err }, 'NotifyWS: 初始状态推送失败');
        }
    });

    logger.info('Notification WebSocket server initialized');
}

/**
 * 在 server.on('upgrade') 中调用，路由 WebSocket 升级请求
 */
export function handleNotifyWSUpgrade(req: any, socket: any, head: any): boolean {
    if (!wss) return false;
    const url = req.url || '';
    if (!url.startsWith('/api/notifications/ws')) return false;
    wss.handleUpgrade(req, socket, head, (ws) => {
        wss!.emit('connection', ws, req);
    });
    return true;
}

/**
 * 向订阅了指定频道的客户端广播消息
 */
function broadcast(channel: string, data: any): void {
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
function getNotifyWSStats(): { totalClients: number; subscriptions: Record<string, number> } {
    const subscriptions: Record<string, number> = { monitor: 0, history: 0, rules: 0 };
    for (const [, client] of clients) {
        for (const ch of client.subscriptions) {
            subscriptions[ch] = (subscriptions[ch] || 0) + 1;
        }
    }
    return { totalClients: clients.size, subscriptions };
}
