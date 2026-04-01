/**
 * 飞书机器人推送渠道
 */
import crypto from 'crypto';
import Logger from '../../utils/logger';
import { NotifyChannel, NotifyResult } from '../types';
import { $ } from '../httpClient';
import { hasConfig, getConfig } from '../config';

const logger = Logger.child({ channel: 'feishu' });

// Token 缓存
let feishuTokenCache = { token: null as string | null, expiresAt: 0 };

/**
 * 获取飞书应用 Token
 */
async function getFeishuAppToken(): Promise<string> {
    const FEISHU_APP_ID = getConfig('FEISHU_APP_ID');
    const FEISHU_APP_SECRET = getConfig('FEISHU_APP_SECRET');

    // 检查缓存的 token 是否有效
    if (feishuTokenCache.token && Date.now() < feishuTokenCache.expiresAt) {
        return feishuTokenCache.token;
    }

    return new Promise((resolve, reject) => {
        const options = {
            url: 'https://open.feishu.cn/open-apis/auth/v3/tenant_access_token/internal',
            json: {
                app_id: FEISHU_APP_ID,
                app_secret: FEISHU_APP_SECRET
            },
            headers: {
                'Content-Type': 'application/json',
            }
        };

        $.post(options, (err, _resp, data) => {
            if (err) {
                logger.error({ err }, '飞书获取token失败');
                reject(err);
            } else {
                const result = data as { code?: number; tenant_access_token?: string; expire?: number; msg?: string };
                if (result.code === 0) {
                    // 缓存 token，过期时间提前 5 分钟
                    feishuTokenCache.token = result.tenant_access_token!;
                    feishuTokenCache.expiresAt = Date.now() + (result.expire! - 300) * 1000;
                    resolve(result.tenant_access_token!);
                } else {
                    logger.error({ msg: result.msg }, '飞书获取token异常');
                    reject(new Error(result.msg));
                }
            }
        });
    });
}

/**
 * 飞书自定义机器人渠道
 */
export const feishuBotChannel: NotifyChannel = {
    name: 'feishu-bot',
    enabled: false,

    async send(text: string, desp: string): Promise<NotifyResult> {
        return new Promise((resolve) => {
            if (!hasConfig('FSKEY')) {
                resolve({ success: false, message: 'FSKEY 未配置' });
                return;
            }

            const FSKEY = getConfig('FSKEY');
            const FSSECRET = getConfig('FSSECRET');

            const messageContent = `${text}\n\n${desp}`;
            let body: Record<string, unknown>;

            if (FSSECRET) {
                // 带签名验证的消息
                const timestamp = Math.floor(Date.now() / 1000).toString();
                const stringToSign = `${timestamp}\n${FSSECRET}`;
                const hmac = crypto.createHmac('sha256', FSSECRET);
                hmac.update(stringToSign);
                const sign = hmac.digest('base64');

                body = {
                    msg_type: 'text',
                    content: { text: messageContent },
                    timestamp: timestamp,
                    sign: sign
                };
            } else {
                // 不带签名 - 使用富文本消息
                body = {
                    msg_type: 'post',
                    content: {
                        post: {
                            zh_cn: {
                                title: text,
                                content: [[{ tag: 'text', text: desp }]]
                            }
                        }
                    }
                };
            }

            const options = {
                url: `https://open.feishu.cn/open-apis/bot/v2/hook/${FSKEY}`,
                json: body,
                headers: {
                    'Content-Type': 'application/json',
                }
            };

            $.post(options, (err, _resp, data) => {
                if (err) {
                    logger.error({ err }, '飞书发送通知调用API失败');
                    resolve({ success: false, error: err as Error });
                } else {
                    const result = data as { StatusCode?: number; code?: number; msg?: string };
                    if (result.StatusCode === 0 || result.code === 0) {
                        logger.info('飞书发送通知消息成功');
                        resolve({ success: true, message: '发送成功' });
                    } else {
                        logger.warn({ msg: result.msg }, '飞书发送通知消息异常');
                        resolve({ success: false, message: result.msg || '发送失败' });
                    }
                }
            });
        });
    }
};

/**
 * 飞书企业应用渠道
 */
export const feishuAppChannel: NotifyChannel = {
    name: 'feishu-app',
    enabled: false,

    async send(text: string, desp: string): Promise<NotifyResult> {
        if (!hasConfig('FEISHU_APP_ID', 'FEISHU_APP_SECRET')) {
            return { success: false, message: 'FEISHU_APP_ID 或 FEISHU_APP_SECRET 未配置' };
        }

        const FEISHU_USER_ID = getConfig('FEISHU_USER_ID');

        try {
            const token = await getFeishuAppToken();
            const messageContent = `${text}\n\n${desp}`;

            const body: Record<string, unknown> = {
                receive_id: FEISHU_USER_ID || '',
                msg_type: 'text',
                content: JSON.stringify({ text: messageContent })
            };

            const options = {
                url: 'https://open.feishu.cn/open-apis/im/v1/messages?receive_id_type=union_id',
                json: body,
                headers: {
                    'Content-Type': 'application/json',
                    'Authorization': `Bearer ${token}`
                }
            };

            return new Promise((resolve) => {
                $.post(options, (err, _resp, data) => {
                    if (err) {
                        logger.error({ err }, '飞书企业应用发送失败');
                        resolve({ success: false, error: err as Error });
                    } else {
                        const result = data as { code?: number; msg?: string };
                        if (result.code === 0) {
                            logger.info('飞书企业应用发送成功');
                            resolve({ success: true, message: '发送成功' });
                        } else {
                            logger.warn({ msg: result.msg }, '飞书企业应用发送失败');
                            resolve({ success: false, message: result.msg || '发送失败' });
                        }
                    }
                });
            });
        } catch (err) {
            logger.error({ err }, '飞书企业应用发送异常');
            return { success: false, error: err as Error };
        }
    }
};

// 检查是否启用
if (hasConfig('FSKEY')) {
    feishuBotChannel.enabled = true;
}
if (hasConfig('FEISHU_APP_ID', 'FEISHU_APP_SECRET')) {
    feishuAppChannel.enabled = true;
}

export default { feishuBotChannel, feishuAppChannel };
