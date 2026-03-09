import express, { Request, Response, NextFunction } from 'express';
import { spawn } from 'child_process';
import { query } from 'express-validator';
import { validateToken } from '../middleware/auth';
import { isValidNumber, isValidContainerName } from '../utils/validation';
import config from '../utils/config';
import { DockerContainer } from '../types';

const router = express.Router();

let FILTER_SENSITIVE = process.env.FILTER_SENSITIVE !== 'false';

function filterSensitiveInfo(content: string): string {
    if (!FILTER_SENSITIVE) return content;
    if (!content || typeof content !== 'string') return content;
    let filtered = content;
    for (const pattern of config.sensitivePatterns) {
        filtered = filtered.replace(pattern, '[FILTERED]');
    }
    return filtered;
}

function execDocker(args: string[], timeout: number = 60000): Promise<string> {
    return new Promise((resolve, reject) => {
        const proc = spawn('docker', args, {
            timeout: timeout
        });

        let stdout = '';
        let stderr = '';

        proc.stdout.on('data', (data) => {
            stdout += data.toString();
        });

        proc.stderr.on('data', (data) => {
            stderr += data.toString();
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
        const stdout = await execDocker(['ps', '--format', '{{.Names}}\t{{.Status}}\t{{.Image}}'], 10000);
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
    query('lines').optional().isInt({ min: 1, max: 50000 })
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
            const linesNum = Math.min(parseInt(lines as string), 50000);
            args = ['logs', container as string, '--tail', String(linesNum)];
        } else {
            args = ['logs', container as string];
        }

        const stdout = await execDocker(args, 120000);
        res.json({ logs: filterSensitiveInfo(stdout) });
    } catch (e) {
        res.status(500).json({ error: (e as Error).message });
    }
});

export default router;
