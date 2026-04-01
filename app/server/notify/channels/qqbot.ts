/**
 * QQ机器人推送渠道（基于QQ开放平台API）
 * 文档: https://bot.q.qq.com/wiki/develop/api/
 */
import Logger from '../../utils/logger';
import { NotifyChannel, NotifyResult } from '../types';
import { $ } from '../httpClient';
import { hasConfig, getConfig } from '../config';

const logger = Logger.child({ channel: 'qqbot' });

// QQ开放平台API地址
const QQ_API_BASE = 'https://api.sgroup.qq.com';
const QQ_TOKEN_URL = 'https://bots.qq.com/app/getAppAccessToken';

// Token缓存
let qqCachedToken: { token: string; expiresAt: number; appId: string } | null = null;

/**
 * 获取QQ AccessToken
 */
async function getQQAccessToken(appId: string, appSecret: string): Promise<string> {
    const now = Date.now();
    
    // 检查缓存
    if (qqCachedToken && now < qqCachedToken.expiresAt - 5 * 60 * 1000 && qqCachedToken.appId === appId) {
        return qqCachedToken.token;
    }

    if (qqCachedToken && qqCachedToken.appId !== appId) {
        qqCachedToken = null;
    }

    return new Promise((resolve, reject) => {
        $.post({
            url: QQ_TOKEN_URL,
            json: { appId: appId, clientSecret: appSecret }
        }, (err, _resp, data) => {
            if (err) {
                reject(err);
            } else {
                const result = data as { access_token?: string; expires_in?: number };
                const token = result.access_token;
                if (!token) {
                    reject(new Error('获取Token失败'));
                    return;
                }
                const expiresIn = parseInt(String(result.expires_in)) || 7200;
                qqCachedToken = {
                    token: token,
                    expiresAt: now + expiresIn * 1000,
                    appId: appId,
                };
                resolve(token);
            }
        });
    });
}

/**
 * 发送QQ消息
 */
async function sendQQMessage(accessToken: string, target: string, content: string, isGroup: boolean, useMarkdown = false): Promise<any> {
    const path = isGroup
        ? `/v2/groups/${target}/messages`
        : `/v2/users/${target}/messages`;

    const body = useMarkdown
        ? { markdown: { content: content }, msg_type: 2 }
        : { content: content, msg_type: 0 };

    return new Promise((resolve, reject) => {
        $.post({
            url: `${QQ_API_BASE}${path}`,
            json: body,
            headers: {
                'Authorization': `QQBot ${accessToken}`
            }
        }, (err, _resp, data) => {
            if (err) {
                reject(err);
            } else {
                resolve(data);
            }
        });
    });
}

export const qqbotChannel: NotifyChannel = {
    name: 'qqbot',
    enabled: false,

    async send(text: string, desp: string): Promise<NotifyResult> {
        return new Promise(async (resolve) => {
            if (!hasConfig('QQ_APP_ID', 'QQ_APP_SECRET')) {
                resolve({ success: false, message: 'QQ机器人配置不完整' });
                return;
            }

            const QQ_APP_ID = getConfig('QQ_APP_ID') || '';
            const QQ_APP_SECRET = getConfig('QQ_APP_SECRET') || '';
            const QQ_OPENID = getConfig('QQ_OPENID');
            const QQ_GROUP_OPENID = getConfig('QQ_GROUP_OPENID');

            logger.info('QQ机器人 服务启动');

            // 确定发送目标
            const targets: Array<{ id: string; isGroup: boolean }> = [];
            if (QQ_GROUP_OPENID) {
                targets.push({ id: QQ_GROUP_OPENID, isGroup: true });
            }
            if (QQ_OPENID) {
                targets.push({ id: QQ_OPENID, isGroup: false });
            }

            if (targets.length === 0) {
                resolve({ success: false, message: '未配置接收者' });
                return;
            }

            // 格式化消息内容（Markdown格式）
            const content = `**${text}**\n\n${desp}`;

            try {
                const accessToken = await getQQAccessToken(QQ_APP_ID, QQ_APP_SECRET);
                let successCount = 0;

                for (const target of targets) {
                    try {
                        await sendQQMessage(accessToken, target.id, content, target.isGroup, true);
                        successCount++;
                    } catch (err) {
                        const error = err as Error;
                        const errMsg = error.message || String(err);
                        // 如果Markdown不可用，回退到纯文本
                        if (errMsg.includes('markdown') || errMsg.includes('11244') || errMsg.includes('权限')) {
                            try {
                                const plainContent = `【${text}】\n\n${desp}`;
                                await sendQQMessage(accessToken, target.id, plainContent, target.isGroup, false);
                                successCount++;
                            } catch (fallbackErr) {
                                logger.warn({ target: target.id, err: fallbackErr }, 'QQ机器人发送失败');
                            }
                        } else {
                            logger.warn({ target: target.id, err }, 'QQ机器人发送失败');
                        }
                    }
                }

                if (successCount > 0) {
                    logger.info('QQ机器人推送成功');
                    resolve({ success: true, message: '发送成功' });
                } else {
                    resolve({ success: false, message: '发送失败' });
                }
            } catch (err) {
                logger.error({ err }, 'QQ机器人推送失败');
                resolve({ success: false, error: err as Error });
            }
        });
    }
};

if (hasConfig('QQ_APP_ID', 'QQ_APP_SECRET')) {
    qqbotChannel.enabled = true;
}

export default qqbotChannel;
