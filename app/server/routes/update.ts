import express, { Request, Response, NextFunction } from 'express';
import https from 'https';
import http from 'http';
import fs from 'fs';
import path from 'path';
import { spawn } from 'child_process';
import { validateToken, validateCSRF } from '../middleware/auth';
import { sensitiveActionRateLimit } from '../middleware/rateLimit';
import * as auditService from '../services/audit';
import config from '../utils/config';
import { isValidGitHubUrl } from '../utils/validation';
import { UpdateStatus, GitHubRelease, GitHubAsset } from '../types';

const router = express.Router();

const GITHUB_REPO = 'sushazhi/fnos-logmanager';
const GITHUB_API = 'https://api.github.com';
const GITHUB_HOSTS = ['github.com', 'api.github.com', 'objects.githubusercontent.com'];

const BINARY_PROXY = 'https://ghfast.top/';

const systemStatus: UpdateStatus = {
    ready: 'true',
    updating: false,
    updateProgress: 0,
    updateMessage: ''
};

function compareVersion(v1: string, v2: string): number {
    const parts1 = v1.toString().split('.').map(Number);
    const parts2 = v2.toString().split('.').map(Number);
    const maxLen = Math.max(parts1.length, parts2.length);

    for (let i = 0; i < maxLen; i++) {
        const num1 = parts1[i] || 0;
        const num2 = parts2[i] || 0;
        if (num1 > num2) return 1;
        if (num1 < num2) return -1;
    }
    return 0;
}

function isValidGitHubAsset(asset: GitHubAsset): boolean {
    if (!asset || !asset.name || !asset.browser_download_url) {
        return false;
    }

    try {
        const url = new URL(asset.browser_download_url);
        return GITHUB_HOSTS.includes(url.hostname);
    } catch {
        return false;
    }
}

router.get('/version', validateToken, (_req: Request, res: Response) => {
    try {
        const currentVersion = process.env.TRIM_APPVER || '0.0.0';
        res.json({
            success: true,
            version: currentVersion,
            appName: '飞牛日志管理',
            platform: process.arch
        });
    } catch {
        res.status(500).json({
            success: false,
            error: '获取版本信息失败'
        });
    }
});

router.get('/check', validateToken, async (_req: Request, res: Response) => {
    try {
        const currentVersion = process.env.TRIM_APPVER || '0.0.0';

        const latestRelease = await new Promise<GitHubRelease>((resolve, reject) => {
            const url = `${GITHUB_API}/repos/${GITHUB_REPO}/releases/latest`;

            const options = {
                headers: {
                    'User-Agent': 'fnos-logmanager-updater',
                    'Accept': 'application/vnd.github.v3+json'
                },
                timeout: 30000
            };

            const req = https.get(url, options, (response) => {
                let data = '';
                response.on('data', chunk => data += chunk);
                response.on('end', () => {
                    try {
                        if (response.statusCode === 200) {
                            const release = JSON.parse(data);
                            resolve({
                                version: release.tag_name.replace('v', ''),
                                changelog: release.body,
                                publishedAt: release.published_at,
                                assets: release.assets
                            });
                        } else {
                            reject(new Error(`GitHub API 返回 ${response.statusCode}`));
                        }
                    } catch (e) {
                        reject(e);
                    }
                });
            });

            req.on('error', reject);
            req.on('timeout', () => {
                req.destroy();
                reject(new Error('GitHub API 请求超时'));
            });
        });

        const hasUpdate = compareVersion(latestRelease.version, currentVersion) > 0;

        res.json({
            success: true,
            currentVersion,
            latestVersion: latestRelease.version,
            hasUpdate,
            changelog: latestRelease.changelog,
            publishedAt: latestRelease.publishedAt,
            message: hasUpdate ? '发现新版本' : '已是最新版本'
        });
    } catch (error) {
        console.error('检查更新失败:', error);
        res.status(500).json({
            success: false,
            error: '检查更新失败: ' + (error as Error).message
        });
    }
});

router.post('/install', validateToken, validateCSRF, sensitiveActionRateLimit(1, 600000), async (_req: Request, res: Response) => {
    try {
        if (systemStatus.updating) {
            res.status(409).json({
                success: false,
                error: '正在更新中，请稍候'
            });
            return;
        }

        systemStatus.updating = true;
        systemStatus.updateProgress = 0;
        systemStatus.updateMessage = '准备更新...';

        res.json({
            success: true,
            message: '开始下载更新，请稍候...'
        });

        performUpdate();

    } catch (error) {
        console.error('安装更新失败:', error);
        systemStatus.updating = false;
        systemStatus.updateMessage = '更新失败: ' + (error as Error).message;
    }
});

router.get('/status', validateToken, (_req: Request, res: Response) => {
    res.json({
        success: true,
        ...systemStatus
    });
});

async function performUpdate(): Promise<void> {
    try {
        systemStatus.updateMessage = '正在获取更新信息...';
        systemStatus.updateProgress = 5;

        const releaseInfo = await new Promise<{ tag_name: string; assets: GitHubAsset[] }>((resolve, reject) => {
            const url = `${GITHUB_API}/repos/${GITHUB_REPO}/releases/latest`;

            const req = https.get(url, {
                headers: {
                    'User-Agent': 'fnos-logmanager-updater',
                    'Accept': 'application/vnd.github.v3+json'
                },
                timeout: 30000
            }, (response) => {
                let data = '';
                response.on('data', chunk => data += chunk);
                response.on('end', () => {
                    try {
                        if (response.statusCode === 200) {
                            resolve(JSON.parse(data));
                        } else {
                            reject(new Error(`GitHub API 返回 ${response.statusCode}`));
                        }
                    } catch (e) {
                        reject(e);
                    }
                });
            });

            req.on('error', reject);
            req.on('timeout', () => {
                req.destroy();
                reject(new Error('GitHub API 请求超时'));
            });
        });

        systemStatus.updateMessage = '正在查找更新包...';
        systemStatus.updateProgress = 10;

        const asset = releaseInfo.assets.find(a =>
            a.name.endsWith('.fpk') && a.name.includes('logmanager') && isValidGitHubAsset(a)
        );

        if (!asset) {
            throw new Error('未找到有效的更新包');
        }

        if (!isValidGitHubUrl(asset.browser_download_url)) {
            throw new Error('更新包来源不安全');
        }

        systemStatus.updateMessage = '正在准备更新目录...';
        systemStatus.updateProgress = 15;

        const updateDir = path.join(
            process.env.TRIM_APPDEST_VOL || '/vol1',
            '@appshare',
            process.env.TRIM_APPNAME || 'logmanager',
            'update'
        );

        if (!fs.existsSync(updateDir)) {
            fs.mkdirSync(updateDir, { recursive: true });
        }

        const fpkFile = path.join(updateDir, 'logmanager.tar');
        const configFile = path.join(updateDir, 'config.env');

        systemStatus.updateMessage = '正在下载更新包...';
        systemStatus.updateProgress = 20;

        const downloadUrl = `${BINARY_PROXY}${asset.browser_download_url}`;

        await downloadFileWithProgress(downloadUrl, fpkFile, (progress) => {
            systemStatus.updateProgress = 20 + Math.floor(progress * 0.4);
        });

        const stats = fs.statSync(fpkFile);
        if (stats.size === 0) {
            fs.unlinkSync(fpkFile);
            throw new Error('下载的文件为空');
        }
        if (asset.size && Math.abs(stats.size - asset.size) > 1024) {
            fs.unlinkSync(fpkFile);
            throw new Error(`文件大小不匹配：期望 ${asset.size} 字节，实际 ${stats.size} 字节`);
        }

        systemStatus.updateMessage = '正在创建配置文件...';
        systemStatus.updateProgress = 65;

        const configContent = `# 更新配置文件
wizard_app_port=${process.env.LOGMANAGER_PORT || config.port}
wizard_data_action=keep
`;
        fs.writeFileSync(configFile, configContent, 'utf8');

        systemStatus.updateMessage = '正在解压更新包...';
        systemStatus.updateProgress = 70;

        await new Promise<void>((resolve, reject) => {
            const proc = spawn('tar', ['-xf', fpkFile, '-C', updateDir], {
                timeout: 120000
            });

            proc.on('close', (code) => {
                if (code === 0) {
                    resolve();
                } else {
                    reject(new Error('解压失败'));
                }
            });

            proc.on('error', (err) => {
                reject(new Error('解压失败: ' + err.message));
            });
        });

        if (fs.existsSync(fpkFile)) {
            fs.unlinkSync(fpkFile);
        }

        systemStatus.updateMessage = '正在安装更新...';
        systemStatus.updateProgress = 80;

        const volume = updateDir.match(/\/vol(\d+)\//);
        const volumeNum = volume ? volume[1] : '1';

        await new Promise<void>((resolve, reject) => {
            const proc = spawn('appcenter-cli', ['default-volume', volumeNum], {
                cwd: updateDir,
                shell: true,
                timeout: 60000
            });

            proc.on('close', (code) => {
                if (code === 0) {
                    resolve();
                } else {
                    reject(new Error('设置默认卷失败'));
                }
            });

            proc.on('error', reject);
        });

        const installProc = spawn('appcenter-cli', ['install-local', '--env', 'config.env'], {
            cwd: updateDir,
            detached: true,
            stdio: 'ignore',
            shell: true
        });

        installProc.unref();

        systemStatus.updateMessage = '更新完成，应用将重启...';
        systemStatus.updateProgress = 100;
        systemStatus.updating = false;

        auditService.addSecurityAuditLog('APP_UPDATED', {
            fromVersion: process.env.TRIM_APPVER,
            toVersion: releaseInfo.tag_name.replace('v', ''),
            timestamp: new Date().toISOString()
        });

    } catch (error) {
        console.error('更新过程出错:', error);
        systemStatus.updating = false;
        systemStatus.updateMessage = '更新失败: ' + (error as Error).message;
        systemStatus.updateProgress = 0;

        auditService.addSecurityAuditLog('UPDATE_FAILED', {
            error: (error as Error).message,
            timestamp: new Date().toISOString()
        });
    }
}

function downloadFileWithProgress(url: string, dest: string, onProgress: (progress: number) => void): Promise<void> {
    return new Promise((resolve, reject) => {
        const protocol = url.startsWith('https') ? https : http;
        const file = fs.createWriteStream(dest);
        const timeout = 300000;

        const request = (requestUrl: string, redirectCount: number = 0) => {
            if (redirectCount > 5) {
                reject(new Error('重定向次数过多'));
                return;
            }

            const req = protocol.get(requestUrl, {
                timeout: timeout
            }, (response) => {
                if (response.statusCode === 301 || response.statusCode === 302) {
                    const redirectUrl = response.headers.location;
                    if (redirectUrl) {
                        request(redirectUrl, redirectCount + 1);
                        return;
                    }
                }

                if (response.statusCode !== 200) {
                    reject(new Error(`下载失败: HTTP ${response.statusCode}`));
                    return;
                }

                const totalSize = parseInt(response.headers['content-length'] || '0', 10);
                let downloadedSize = 0;

                response.on('data', (chunk) => {
                    downloadedSize += chunk.length;
                    if (totalSize && onProgress) {
                        onProgress(downloadedSize / totalSize);
                    }
                });

                response.pipe(file);

                file.on('finish', () => {
                    file.close();
                    resolve();
                });

                file.on('error', (err) => {
                    fs.unlink(dest, () => {});
                    reject(err);
                });
            });

            req.on('error', (err) => {
                fs.unlink(dest, () => {});
                reject(err);
            });

            req.on('timeout', () => {
                req.destroy();
                fs.unlink(dest, () => {});
                reject(new Error('下载超时'));
            });
        };

        request(url);
    });
}

export default router;
