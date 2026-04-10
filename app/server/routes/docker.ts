import express, { Request, Response, NextFunction } from 'express';
import { spawn } from 'child_process';
import { query } from 'express-validator';
import { validateToken } from '../middleware/auth';
import { isValidContainerName } from '../utils/validation';
import { filterSensitiveInfo } from '../utils/filter';
import config from '../utils/config';
import { DockerContainer } from '../types';

const router = express.Router();

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

export default router;
