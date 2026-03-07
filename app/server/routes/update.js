/**
 * @fileoverview 更新路由
 */

const express = require('express');
const router = express.Router();
const https = require('https');
const http = require('http');
const fs = require('fs');
const path = require('path');
const { spawn } = require('child_process');
const { validateToken, validateCSRF } = require('../middleware/auth');
const auditService = require('../services/audit');
const config = require('../utils/config');

// GitHub 配置
const GITHUB_REPO = 'sushazhi/fnos-logmanager';
const GITHUB_API = 'https://api.github.com';

// 代理配置
const MAIN_PROXY = 'https://hk.gh-proxy.org/';
const BINARY_PROXY = 'https://ghfast.top/';

// 系统状态
let systemStatus = {
    ready: 'true',
    updating: false,
    updateProgress: 0,
    updateMessage: ''
};

/**
 * 比较版本号
 * @param {string} v1 - 版本1
 * @param {string} v2 - 版本2
 * @returns {number} - 1: v1>v2, -1: v1<v2, 0: 相等
 */
function compareVersion(v1, v2) {
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

/**
 * 获取当前版本信息
 * @route GET /api/update/version
 */
router.get('/version', validateToken, (req, res) => {
    try {
        const currentVersion = process.env.TRIM_APPVER || '0.0.0';
        res.json({
            success: true,
            version: currentVersion,
            appName: '飞牛日志管理',
            platform: process.arch
        });
    } catch (error) {
        res.status(500).json({
            success: false,
            error: '获取版本信息失败'
        });
    }
});

/**
 * 检查最新版本
 * @route GET /api/update/check
 */
router.get('/check', validateToken, async (req, res) => {
    try {
        const currentVersion = process.env.TRIM_APPVER || '0.0.0';
        
        // 从 GitHub Releases 获取最新版本
        const latestRelease = await new Promise((resolve, reject) => {
            const url = `${GITHUB_API}/repos/${GITHUB_REPO}/releases/latest`;
            
            const options = {
                headers: {
                    'User-Agent': 'fnos-logmanager-updater',
                    'Accept': 'application/vnd.github.v3+json'
                }
            };
            
            https.get(url, options, (response) => {
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
            }).on('error', reject);
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
            error: '检查更新失败: ' + error.message
        });
    }
});

/**
 * 下载并安装更新
 * @route POST /api/update/install
 */
router.post('/install', validateToken, validateCSRF, async (req, res) => {
    try {
        if (systemStatus.updating) {
            return res.status(409).json({
                success: false,
                error: '正在更新中，请稍候'
            });
        }
        
        systemStatus.updating = true;
        systemStatus.updateProgress = 0;
        systemStatus.updateMessage = '准备更新...';
        
        // 立即返回响应，更新在后台进行
        res.json({
            success: true,
            message: '开始下载更新，请稍候...'
        });
        
        // 后台执行更新
        await performUpdate();
        
    } catch (error) {
        console.error('安装更新失败:', error);
        systemStatus.updating = false;
        systemStatus.updateMessage = '更新失败: ' + error.message;
    }
});

/**
 * 获取更新状态
 * @route GET /api/update/status
 */
router.get('/status', validateToken, (req, res) => {
    res.json({
        success: true,
        ...systemStatus
    });
});

/**
 * 执行更新
 */
async function performUpdate() {
    try {
        systemStatus.updateMessage = '正在获取更新信息...';
        systemStatus.updateProgress = 5;
        
        // 1. 从 GitHub 获取最新 Release 信息
        const releaseInfo = await new Promise((resolve, reject) => {
            const url = `${GITHUB_API}/repos/${GITHUB_REPO}/releases/latest`;
            
            https.get(url, {
                headers: {
                    'User-Agent': 'fnos-logmanager-updater',
                    'Accept': 'application/vnd.github.v3+json'
                }
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
            }).on('error', reject);
        });
        
        systemStatus.updateMessage = '正在查找更新包...';
        systemStatus.updateProgress = 10;
        
        // 2. 查找 FPK 文件（不区分架构）
        const asset = releaseInfo.assets.find(a => 
            a.name.endsWith('.fpk') && a.name.includes('logmanager')
        );
        
        if (!asset) {
            throw new Error('未找到更新包');
        }
        
        // 验证文件来源：必须是 GitHub 官方域名
        if (!asset.browser_download_url.startsWith('https://github.com/') && 
            !asset.browser_download_url.startsWith('https://objects.githubusercontent.com/')) {
            throw new Error('更新包来源不安全');
        }
        
        systemStatus.updateMessage = '正在准备更新目录...';
        systemStatus.updateProgress = 15;
        
        // 3. 准备更新目录
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
        
        // 4. 使用代理下载 FPK 文件
        const proxyUrl = `${BINARY_PROXY}${asset.browser_download_url}`;
        
        await downloadFileWithProgress(proxyUrl, fpkFile, (progress) => {
            systemStatus.updateProgress = 20 + Math.floor(progress * 0.4); // 20-60%
        });
        
        // 验证下载文件大小（防止下载不完整）
        const stats = fs.statSync(fpkFile);
        if (stats.size === 0) {
            throw new Error('下载的文件为空');
        }
        if (asset.size && Math.abs(stats.size - asset.size) > 1024) {
            // 允许 1KB 的误差
            throw new Error(`文件大小不匹配：期望 ${asset.size} 字节，实际 ${stats.size} 字节`);
        }
        
        systemStatus.updateMessage = '正在创建配置文件...';
        systemStatus.updateProgress = 65;
        
        // 5. 创建配置文件（保留用户数据）
        const configContent = `# 更新配置文件
wizard_app_port=${process.env.LOGMANAGER_PORT || config.port}
wizard_data_action=keep
`;
        fs.writeFileSync(configFile, configContent, 'utf8');
        
        systemStatus.updateMessage = '正在解压更新包...';
        systemStatus.updateProgress = 70;
        
        // 6. 解压 FPK 文件
        await new Promise((resolve, reject) => {
            const proc = spawn('tar', ['-xf', fpkFile, '-C', updateDir]);
            
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
        
        // 7. 删除 FPK 文件
        if (fs.existsSync(fpkFile)) {
            fs.unlinkSync(fpkFile);
        }
        
        systemStatus.updateMessage = '正在安装更新...';
        systemStatus.updateProgress = 80;
        
        // 8. 执行安装命令
        const volume = updateDir.match(/\/vol(\d+)\//);
        const volumeNum = volume ? volume[1] : '1';
        
        // 设置默认卷
        await new Promise((resolve, reject) => {
            const proc = spawn('appcenter-cli', ['default-volume', volumeNum], {
                cwd: updateDir,
                shell: true
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
        
        // 安装应用
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
        
        // 记录审计日志
        auditService.addAuditLog('app_updated', {
            fromVersion: process.env.TRIM_APPVER,
            toVersion: releaseInfo.tag_name.replace('v', ''),
            timestamp: new Date().toISOString()
        });
        
    } catch (error) {
        console.error('更新过程出错:', error);
        systemStatus.updating = false;
        systemStatus.updateMessage = '更新失败: ' + error.message;
        systemStatus.updateProgress = 0;
    }
}

/**
 * 带进度的文件下载
 * @param {string} url - 下载地址
 * @param {string} dest - 目标路径
 * @param {Function} onProgress - 进度回调 (0-1)
 */
function downloadFileWithProgress(url, dest, onProgress) {
    return new Promise((resolve, reject) => {
        const protocol = url.startsWith('https') ? https : http;
        const file = fs.createWriteStream(dest);
        
        const request = (url) => {
            protocol.get(url, (response) => {
                // 处理重定向
                if (response.statusCode === 301 || response.statusCode === 302) {
                    const redirectUrl = response.headers.location;
                    request(redirectUrl);
                    return;
                }
                
                if (response.statusCode !== 200) {
                    reject(new Error(`下载失败: HTTP ${response.statusCode}`));
                    return;
                }
                
                const totalSize = parseInt(response.headers['content-length'], 10);
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
            }).on('error', (err) => {
                fs.unlink(dest, () => {});
                reject(err);
            });
        };
        
        request(url);
    });
}

module.exports = router;
