import express, { Request, Response, NextFunction } from 'express';
import { spawn } from 'child_process';
import { query } from 'express-validator';
import { validateToken, validate, checkValidation } from '../middleware/auth';
import { isValidContainerName } from '../utils/validation';
import { filterSensitiveInfo } from '../utils/filter';
import { isFilterEnabled } from '../utils/filter';
import * as sessionService from '../services/session';
import * as auditService from '../services/audit';
import config from '../utils/config';
import { DockerContainer } from '../types';

const router = express.Router();

const dockerSseConnectionCounts = new Map<string, number>();

function execDocker(args: string[], timeout: number = 60000, maxBytes: number = config.docker.maxOutputBytes): Promise<string> {
    return new Promise((resolve, reject) => {
        const proc = spawn('docker', args, {
            timeout: timeout
        });

        let stdout = '';
        let stderr = '';
        let totalBytes = 0;

        const appendOutput = (buffer: Buffer, target: 'stdout' | 'stderr') => {
            totalBytes += buffer.length;
            if (totalBytes > maxBytes) {
                proc.kill();
                reject(new Error('Docker 输出过大'));
                return;
            }
            if (target === 'stdout') {
                stdout += buffer.toString();
            } else {
                stderr += buffer.toString();
            }
        };

        proc.stdout.on('data', (data) => {
            appendOutput(data as Buffer, 'stdout');
        });

        proc.stderr.on('data', (data) => {
            appendOutput(data as Buffer, 'stderr');
        });

        const timer = setTimeout(() => {
            proc.kill();
            reject(new Error('命令执行超时'));
        }, timeout);

        proc.on('close', (code) => {
            clearTimeout(timer);
            if (code === 0) {
                resolve(stdout + stderr);
            } else {
                reject(new Error(stderr || `Docker 命令退出码: ${code}`));
            }
        });

        proc.on('error', (err) => {
            clearTimeout(timer);
            reject(new Error(`Docker 命令执行失败: ${err.message}`));
        });
    });
}

router.get('/docker/containers', validateToken, async (_req: Request, res: Response, _next: NextFunction) => {
    try {
        const stdout = await execDocker(['ps', '--format', '{{.Names}}\t{{.Status}}\t{{.Image}}'], config.docker.listTimeoutMs, config.docker.maxOutputBytes);
        const containers: DockerContainer[] = stdout.trim().split('\n').filter(line => line).map(line => {
            const parts = line.split('\t');
            const name = parts[0] || '';
            if (!isValidContainerName(name)) return null;
            return {
                name,
                status: parts[1] || '',
                image: parts[2] || ''
            };
        }).filter((c): c is DockerContainer => c !== null);
        res.json({ containers });
    } catch {
        res.json({ containers: [], error: 'Docker未安装或未运行' });
    }
});

router.get('/docker/logs', validateToken, [
    query('container').notEmpty().isString(),
    query('lines').optional().isInt({ min: 1, max: config.docker.maxLogLines })
], async (req: Request, res: Response, _next: NextFunction) => {
    try {
        const { container, lines } = req.query;

        if (!container) {
            res.status(400).json({ error: '缺少容器名称' });
            return;
        }

        if (!isValidContainerName(container as string)) {
            res.status(400).json({ error: '无效的容器名称' });
            return;
        }

        let args: string[];
        if (lines && parseInt(lines as string) > 0) {
            const linesNum = Math.min(parseInt(lines as string), config.docker.maxLogLines);
            args = ['logs', container as string, '--tail', String(linesNum)];
        } else {
            args = ['logs', container as string];
        }

        const stdout = await execDocker(args, config.docker.logsTimeoutMs, config.docker.maxOutputBytes);
        res.json({ logs: filterSensitiveInfo(stdout) });
    } catch (e) {
        res.status(500).json({ error: '获取Docker日志失败' });
    }
});

router.get('/docker/tail', validateToken, [
    query('container').notEmpty().isString(),
    query('offset').optional().isInt()
], checkValidation, async (req: Request, res: Response, _next: NextFunction) => {
    const container = req.query.container as string;
    if (!container || !isValidContainerName(container)) {
        res.status(400).json({ error: '无效的容器名称' });
        return;
    }

    try {
        const offset = parseInt(req.query.offset as string, 10) || 0;
        const stdout = await execDocker(['logs', container, '--since', '1m'], config.docker.logsTimeoutMs, config.docker.maxOutputBytes);
        const lines = stdout.split('\n');
        const newLines = offset < lines.length ? lines.slice(offset).join('\n') : '';
        const content = isFilterEnabled() ? filterSensitiveInfo(newLines) : newLines;
        res.json({ content, offset: lines.length });
    } catch {
        res.status(500).json({ error: '获取Docker日志增量失败' });
    }
});

router.get('/docker/stream', [
    query('container').notEmpty().isString(),
    query('token').optional().isString()
], checkValidation, async (req: Request, res: Response) => {
    const clientIp = req.ip || req.socket.remoteAddress || 'unknown';
    const currentCount = dockerSseConnectionCounts.get(clientIp) || 0;
    if (currentCount >= 3) {
        res.status(429).json({ error: 'SSE 连接数超过限制' });
        return;
    }
    dockerSseConnectionCounts.set(clientIp, currentCount + 1);

    const sessionToken = (req.query.token as string) || req.cookies?.session_token;
    if (!sessionToken || !sessionService.validateSession(sessionToken)) {
        auditService.addAuditLog('auth_failed', { path: req.path, clientIP: clientIp, sseStream: true }, req);
        res.status(401).json({ error: '未认证' });
        return;
    }

    const container = req.query.container as string;
    if (!container || !isValidContainerName(container)) {
        res.status(400).json({ error: '无效的容器名称' });
        return;
    }

    res.setHeader('Content-Type', 'text/event-stream');
    res.setHeader('Cache-Control', 'no-cache');
    res.setHeader('Connection', 'keep-alive');
    res.setHeader('X-Accel-Buffering', 'no');
    res.flushHeaders();

    const sendEvent = (event: string, data: any): void => {
        try {
            res.write(`event: ${event}\ndata: ${JSON.stringify(data)}\n\n`);
        } catch { /* client disconnected */ }
    };

    sendEvent('connected', { message: 'Docker stream connected' });

    const dockerProcess = spawn('docker', ['logs', container, '-f'], {
        stdio: ['ignore', 'pipe', 'pipe']
    });

    let buffer = '';

    const flushBuffer = (): void => {
        if (buffer) {
            const filtered = isFilterEnabled() ? filterSensitiveInfo(buffer) : buffer;
            sendEvent('data', { content: filtered });
            buffer = '';
        }
    };

    const flushTimer = setInterval(flushBuffer, 500);

    dockerProcess.stdout?.on('data', (data: Buffer) => {
        buffer += data.toString();
    });

    dockerProcess.stderr?.on('data', (data: Buffer) => {
        buffer += data.toString();
    });

    dockerProcess.on('error', (err) => {
        sendEvent('error', { message: err.message });
        clearInterval(flushTimer);
        res.end();
    });

    dockerProcess.on('close', (code) => {
        flushBuffer();
        sendEvent('error', { message: `Docker process exited with code ${code}` });
        clearInterval(flushTimer);
        res.end();
    });

    req.on('close', () => {
        clearInterval(flushTimer);
        dockerProcess.kill();
        const count = dockerSseConnectionCounts.get(clientIp) || 1;
        if (count <= 1) dockerSseConnectionCounts.delete(clientIp);
        else dockerSseConnectionCounts.set(clientIp, count - 1);
    });
});

router.get('/docker/export', validateToken, [
    query('container').notEmpty().isString(),
    query('format').optional().isIn(['txt', 'json', 'csv'])
], checkValidation, async (req: Request, res: Response, _next: NextFunction) => {
    try {
        const { container, format } = req.query;

        if (!container) {
            res.status(400).json({ error: '缺少容器名称' });
            return;
        }

        if (!isValidContainerName(container as string)) {
            res.status(400).json({ error: '无效的容器名称' });
            return;
        }

        const exportFormat = (format as string) || 'txt';
        const stdout = await execDocker(['logs', container as string], config.docker.logsTimeoutMs, config.docker.maxOutputBytes);
        const content = filterSensitiveInfo(stdout);

        const safeName = (container as string).replace(/[^a-zA-Z0-9._-]/g, '_');
        const timestamp = new Date().toISOString().replace(/[:.]/g, '-');
        const exportName = `docker_${safeName}_${timestamp}`.replace(/"/g, '\\"');

        if (exportFormat === 'txt') {
            res.setHeader('Content-Type', 'text/plain; charset=utf-8');
            res.setHeader('Content-Disposition', `attachment; filename="${exportName}.txt"`);
            res.send(content);
        } else if (exportFormat === 'json') {
            const lines = content.split('\n');
            const data = lines.map((line: string, index: number) => ({
                line: index + 1,
                content: line
            }));
            res.setHeader('Content-Type', 'application/json; charset=utf-8');
            res.setHeader('Content-Disposition', `attachment; filename="${exportName}.json"`);
            res.json({
                source: `docker:${container}`,
                exportedAt: new Date().toISOString(),
                totalLines: lines.length,
                lines: data
            });
        } else if (exportFormat === 'csv') {
            const lines = content.split('\n');
            const csvLines = ['"line","content"'];
            for (let i = 0; i < lines.length; i++) {
                const escaped = lines[i].replace(/"/g, '""');
                csvLines.push(`"${i + 1}","${escaped}"`);
            }
            res.setHeader('Content-Type', 'text/csv; charset=utf-8');
            res.setHeader('Content-Disposition', `attachment; filename="${exportName}.csv"`);
            res.send(csvLines.join('\n'));
        }
    } catch (e) {
        res.status(500).json({ error: '导出Docker日志失败' });
    }
});

export default router;
