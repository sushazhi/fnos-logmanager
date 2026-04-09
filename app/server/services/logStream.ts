/**
 * 实时日志流服务
 * 使用 WebSocket 实现日志实时推送（类似 tail -f）
 */

import { WebSocketServer, WebSocket } from 'ws';
import { Server } from 'http';
import fs from 'fs';
import path from 'path';
import Logger from '../utils/logger';
import { isAllowedPath } from '../utils/validation';
import config from '../utils/config';
import { filterSensitiveInfo } from '../utils/filter';

const logger = Logger.child({ module: 'LogStream' });

interface StreamClient {
    ws: WebSocket;
    filePath: string;
    lastSize: number;
    pollingTimer: NodeJS.Timeout | null;
}

// 活跃的流客户端
const clients: Map<WebSocket, StreamClient> = new Map();

let wss: WebSocketServer | null = null;

/**
 * 初始化 WebSocket 服务器
 * 挂载到现有 HTTP 服务器上
 */
export function initLogStream(server: Server): void {
    wss = new WebSocketServer({ 
        server, 
        path: '/api/logs/stream',
        // 验证 Origin 头
        verifyClient: (info, callback) => {
            // 允许同源和飞牛桌面端
            const origin = info.origin || '';
            const host = info.req.headers.host || '';
            if (!origin || origin.includes(host) || origin.includes('5ddd.com')) {
                callback(true);
            } else {
                logger.warn({ origin, host }, 'WebSocket connection rejected: origin mismatch');
                callback(false, 403, 'Forbidden');
            }
        }
    });

    wss.on('connection', (ws, req) => {
        const clientIp = req.socket.remoteAddress;
        logger.info({ clientIp }, 'WebSocket client connected');

        ws.on('message', (data) => {
            try {
                const message = JSON.parse(data.toString());
                handleMessage(ws, message);
            } catch (err) {
                logger.warn({ err }, 'Invalid WebSocket message');
                ws.send(JSON.stringify({ type: 'error', message: 'Invalid message format' }));
            }
        });

        ws.on('close', () => {
            logger.info({ clientIp }, 'WebSocket client disconnected');
            cleanupClient(ws);
        });

        ws.on('error', (err) => {
            logger.error({ err }, 'WebSocket error');
            cleanupClient(ws);
        });

        // 发送连接确认
        ws.send(JSON.stringify({ type: 'connected' }));
    });

    logger.info('Log stream WebSocket server initialized');
}

/**
 * 处理客户端消息
 */
function handleMessage(ws: WebSocket, message: any): void {
    switch (message.type) {
        case 'subscribe': {
            const { filePath } = message;
            if (!filePath || typeof filePath !== 'string') {
                ws.send(JSON.stringify({ type: 'error', message: 'Missing filePath' }));
                return;
            }

            // 安全检查
            if (!isAllowedPath(filePath, config.logDirs)) {
                ws.send(JSON.stringify({ type: 'error', message: 'Path not allowed' }));
                return;
            }

            if (!fs.existsSync(filePath)) {
                ws.send(JSON.stringify({ type: 'error', message: 'File not found' }));
                return;
            }

            // 清理旧的订阅
            cleanupClient(ws);

            const initialSize = fs.statSync(filePath).size;
            const client: StreamClient = {
                ws,
                filePath,
                lastSize: initialSize,
                pollingTimer: null
            };

            // 设置轮询检测文件变化
            const pollInterval = message.pollInterval || 1000; // 默认 1 秒
            client.pollingTimer = setInterval(() => pollFileChanges(client), Math.max(pollInterval, 500));

            clients.set(ws, client);

            ws.send(JSON.stringify({ 
                type: 'subscribed', 
                filePath,
                initialSize 
            }));

            logger.info({ filePath }, 'Client subscribed to log stream');
            break;
        }

        case 'unsubscribe': {
            cleanupClient(ws);
            ws.send(JSON.stringify({ type: 'unsubscribed' }));
            break;
        }

        default:
            ws.send(JSON.stringify({ type: 'error', message: `Unknown message type: ${message.type}` }));
    }
}

/**
 * 轮询检测文件变化并推送新内容
 */
function pollFileChanges(client: StreamClient): void {
    if (client.ws.readyState !== WebSocket.OPEN) {
        cleanupClient(client.ws);
        return;
    }

    try {
        if (!fs.existsSync(client.filePath)) {
            client.ws.send(JSON.stringify({ type: 'file_deleted', filePath: client.filePath }));
            cleanupClient(client.ws);
            return;
        }

        const currentSize = fs.statSync(client.filePath).size;

        // 文件被清空或轮转
        if (currentSize < client.lastSize) {
            client.lastSize = 0;
            client.ws.send(JSON.stringify({ 
                type: 'file_rotated', 
                filePath: client.filePath 
            }));
        }

        // 有新内容
        if (currentSize > client.lastSize) {
            const readSize = Math.min(currentSize - client.lastSize, 1024 * 1024); // 单次最多 1MB
            const buffer = Buffer.alloc(readSize);
            const fd = fs.openSync(client.filePath, 'r');
            try {
                fs.readSync(fd, buffer, 0, readSize, client.lastSize);
            } finally {
                fs.closeSync(fd);
            }

            const content = filterSensitiveInfo(buffer.toString('utf-8'));
            client.lastSize = currentSize;

            client.ws.send(JSON.stringify({ 
                type: 'data', 
                content,
                filePath: client.filePath,
                offset: client.lastSize - readSize,
                totalSize: currentSize
            }));
        }
    } catch (err) {
        logger.error({ err, filePath: client.filePath }, 'Error polling file changes');
        client.ws.send(JSON.stringify({ 
            type: 'error', 
            message: 'Error reading file' 
        }));
    }
}

/**
 * 清理客户端资源
 */
function cleanupClient(ws: WebSocket): void {
    const client = clients.get(ws);
    if (client) {
        if (client.pollingTimer) {
            clearInterval(client.pollingTimer);
        }
        clients.delete(ws);
    }
}

/**
 * 关闭 WebSocket 服务器
 */
export function closeLogStream(): void {
    // 清理所有客户端
    for (const [ws, client] of clients) {
        cleanupClient(ws);
        if (ws.readyState === WebSocket.OPEN) {
            ws.close(1001, 'Server shutting down');
        }
    }

    if (wss) {
        wss.close();
        wss = null;
    }

    logger.info('Log stream WebSocket server closed');
}

/**
 * 获取当前连接统计
 */
export function getStreamStats(): { totalClients: number; subscriptions: Array<{ filePath: string; clientIp: string }> } {
    const subscriptions: Array<{ filePath: string; clientIp: string }> = [];
    for (const [ws, client] of clients) {
        subscriptions.push({
            filePath: client.filePath,
            clientIp: ws.readyState === WebSocket.OPEN ? 'connected' : 'disconnected'
        });
    }
    return {
        totalClients: clients.size,
        subscriptions
    };
}
