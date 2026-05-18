/**
 * Docker 日志实时流服务
 * 使用 WebSocket + docker logs -f 实现容器日志实时推送
 */

import { WebSocketServer, WebSocket } from 'ws';
import { Server } from 'http';
import { spawn, ChildProcess } from 'child_process';
import Logger from '../utils/logger';
import { isValidContainerName } from '../utils/validation';
import { filterSensitiveInfo } from '../utils/filter';
import * as sessionService from '../services/session';

const logger = Logger.child({ module: 'DockerLogStream' });

interface DockerStreamClient {
    ws: WebSocket;
    container: string;
    dockerProcess: ChildProcess | null;
}

const MAX_DOCKER_WS_CONNECTIONS = 10;

const clients: Map<WebSocket, DockerStreamClient> = new Map();

let wss: WebSocketServer | null = null;

function getSessionTokenFromRequest(req: any): string {
    const cookieHeader = req.headers.cookie || '';
    const match = cookieHeader.match(/session_token=([^;]+)/);
    if (match) return match[1];
    const url = new URL(req.url || '', 'http://localhost');
    return url.searchParams.get('token') || '';
}

export function initDockerLogStream(server: Server): void {
    wss = new WebSocketServer({
        server,
        path: '/api/docker/stream',
        verifyClient: (info, callback) => {
            if (process.env.GATEWAY_SOCKET) {
                callback(true);
                return;
            }
            const token = getSessionTokenFromRequest(info.req);
            if (!token || !sessionService.validateSession(token)) {
                logger.warn('WebSocket connection rejected: invalid or missing session token');
                callback(false, 401, 'Unauthorized');
                return;
            }

            const origin = info.req.headers.origin || '';
            if (origin && !process.env.GATEWAY_SOCKET) {
                const host = info.req.headers.host || '';
                try {
                    const originHost = new URL(origin).host;
                    if (originHost !== host) {
                        logger.warn({ origin, host }, 'Docker WS connection rejected: origin mismatch');
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

    wss.on('connection', (ws, req) => {
        if (clients.size >= MAX_DOCKER_WS_CONNECTIONS) {
            logger.warn({ currentCount: clients.size }, 'Docker WS connection rejected: max connections reached');
            ws.close(1013, 'Max connections reached');
            return;
        }

        const clientIp = req.socket.remoteAddress;
        logger.info({ clientIp }, 'Docker log WebSocket client connected');

        ws.on('message', (data) => {
            try {
                const message = JSON.parse(data.toString());
                handleMessage(ws, message);
            } catch (err) {
                if (data.toString() === 'ping') {
                    ws.send('pong');
                    return;
                }
                logger.warn({ err }, 'Invalid WebSocket message');
                ws.send(JSON.stringify({ type: 'error', message: 'Invalid message format' }));
            }
        });

        let isAlive = true;
        ws.on('pong', () => { isAlive = true; });

        const heartbeat = setInterval(() => {
            if (!isAlive) {
                logger.info('Docker WS heartbeat failed, terminating connection');
                clearInterval(heartbeat);
                return ws.terminate();
            }
            isAlive = false;
            ws.ping();
        }, 30000);

        ws.on('close', () => {
            logger.info({ clientIp }, 'Docker log WebSocket client disconnected');
            clearInterval(heartbeat);
            cleanupClient(ws);
        });

        ws.on('error', (err) => {
            logger.error({ err }, 'Docker log WebSocket error');
            cleanupClient(ws);
        });

        ws.send(JSON.stringify({ type: 'connected' }));
    });

    logger.info('Docker log stream WebSocket server initialized');
}

function handleMessage(ws: WebSocket, message: any): void {
    switch (message.type) {
        case 'ping': {
            ws.send(JSON.stringify({ type: 'pong' }));
            break;
        }

        case 'subscribe': {
            const { container } = message;
            if (!container || typeof container !== 'string') {
                ws.send(JSON.stringify({ type: 'error', message: 'Missing container name' }));
                return;
            }

            if (!isValidContainerName(container)) {
                ws.send(JSON.stringify({ type: 'error', message: 'Invalid container name' }));
                return;
            }

            cleanupClient(ws);

            try {
                const dockerProcess = spawn('docker', ['logs', container, '-f'], {
                    stdio: ['ignore', 'pipe', 'pipe']
                });

                const client: DockerStreamClient = {
                    ws,
                    container,
                    dockerProcess
                };

                dockerProcess.stdout?.on('data', (data: Buffer) => {
                    if (ws.readyState === WebSocket.OPEN) {
                        const content = filterSensitiveInfo(data.toString('utf-8'));
                        ws.send(JSON.stringify({ type: 'data', content }));
                    }
                });

                dockerProcess.stderr?.on('data', (data: Buffer) => {
                    if (ws.readyState === WebSocket.OPEN) {
                        const content = filterSensitiveInfo(data.toString('utf-8'));
                        ws.send(JSON.stringify({ type: 'data', content }));
                    }
                });

                dockerProcess.on('error', (err) => {
                    logger.error({ err, container }, 'Docker logs process error');
                    if (ws.readyState === WebSocket.OPEN) {
                        ws.send(JSON.stringify({ type: 'error', message: `Failed to start docker logs: ${err.message}` }));
                    }
                    cleanupClient(ws);
                });

                dockerProcess.on('close', (code) => {
                    logger.info({ container, code }, 'Docker logs process exited');
                    if (ws.readyState === WebSocket.OPEN) {
                        ws.send(JSON.stringify({ type: 'error', message: `Docker logs process exited with code ${code}` }));
                    }
                    cleanupClient(ws);
                });

                clients.set(ws, client);

                ws.send(JSON.stringify({
                    type: 'subscribed',
                    container
                }));

                logger.info({ container }, 'Client subscribed to docker log stream');
            } catch (err) {
                logger.error({ err, container }, 'Failed to spawn docker logs process');
                ws.send(JSON.stringify({ type: 'error', message: 'Failed to start docker logs' }));
            }
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

function cleanupClient(ws: WebSocket): void {
    const client = clients.get(ws);
    if (client) {
        if (client.dockerProcess && !client.dockerProcess.killed) {
            client.dockerProcess.kill('SIGTERM');
            setTimeout(() => {
                if (client.dockerProcess && !client.dockerProcess.killed) {
                    client.dockerProcess.kill('SIGKILL');
                }
            }, 5000);
        }
        clients.delete(ws);
    }
}

export function closeDockerLogStream(): void {
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

    logger.info('Docker log stream WebSocket server closed');
}

export function getDockerStreamStats(): { totalClients: number; subscriptions: Array<{ container: string; clientIp: string }> } {
    const subscriptions: Array<{ container: string; clientIp: string }> = [];
    for (const [ws, client] of clients) {
        subscriptions.push({
            container: client.container,
            clientIp: ws.readyState === WebSocket.OPEN ? 'connected' : 'disconnected'
        });
    }
    return {
        totalClients: clients.size,
        subscriptions
    };
}
