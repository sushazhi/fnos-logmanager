/**
 * 企业微信机器人推送渠道
 */
import crypto from 'crypto';
import Logger from '../../utils/logger';
import { NotifyChannel, NotifyResult } from '../types';
import { $ } from '../httpClient';
import { hasConfig, getConfig } from '../config';

const logger = Logger.child({ channel: 'wechat-work' });

/**
 * 企业微信机器人渠道
 */
export const wechatWorkBotChannel: NotifyChannel = {
    name: 'wechat-work-bot',
    enabled: false,

    async send(text: string, desp: string): Promise<NotifyResult> {
        return new Promise((resolve) => {
            if (!hasConfig('QYWX_KEY')) {
                resolve({ success: false, message: 'QYWX_KEY 未配置' });
                return;
            }

            const QYWX_KEY = getConfig('QYWX_KEY');
            const QYWX_ORIGIN = getConfig('QYWX_ORIGIN') || 'https://qyapi.weixin.qq.com';
            const url = `${QYWX_ORIGIN}/cgi-bin/webhook/send?key=${QYWX_KEY}`;

            const body = {
                msgtype: 'text',
                text: {
                    content: `${text}\n\n${desp}`
                }
            };

            $.post({ url, json: body }, (err, _resp, data) => {
                if (err) {
                    logger.error({ err }, '企业微信机器人发送失败');
                    resolve({ success: false, error: err as Error });
                } else {
                    const result = data as { errcode?: number; errmsg?: string };
                    if (result.errcode === 0) {
                        logger.info('企业微信机器人发送成功');
                        resolve({ success: true, message: '发送成功' });
                    } else {
                        logger.warn({ msg: result.errmsg }, '企业微信机器人发送失败');
                        resolve({ success: false, message: result.errmsg || '发送失败' });
                    }
                }
            });
        });
    }
};

/**
 * 企业微信应用渠道
 */
export const wechatWorkAppChannel: NotifyChannel = {
    name: 'wechat-work-app',
    enabled: false,

    async send(text: string, desp: string): Promise<NotifyResult> {
        return new Promise((resolve) => {
            if (!hasConfig('QYWX_AM')) {
                resolve({ success: false, message: 'QYWX_AM 未配置' });
                return;
            }

            const QYWX_AM = getConfig('QYWX_AM') || '';
            const QYWX_ORIGIN = getConfig('QYWX_ORIGIN') || 'https://qyapi.weixin.qq.com';

            const QYWX_AM_AY = QYWX_AM.split(',');
            const [corpid, corpsecret, touser, agentid, msgtype] = QYWX_AM_AY;

            if (!corpid || !corpsecret || !agentid) {
                resolve({ success: false, message: 'QYWX_AM 格式错误' });
                return;
            }

            // 获取 access_token (POST请求)
            const tokenUrl = `${QYWX_ORIGIN}/cgi-bin/gettoken`;
            const tokenBody = {
                corpid: corpid,
                corpsecret: corpsecret
            };

            $.post({ url: tokenUrl, json: tokenBody }, (err, _resp, tokenData) => {
                if (err) {
                    logger.error({ err }, '企业微信获取token失败');
                    resolve({ success: false, error: err as Error });
                    return;
                }

                const tokenResult = tokenData as { access_token?: string; errcode?: number };
                if (!tokenResult.access_token) {
                    logger.warn('企业微信获取token失败');
                    resolve({ success: false, message: '获取token失败' });
                    return;
                }

                const accessToken = tokenResult.access_token;
                const sendUrl = `${QYWX_ORIGIN}/cgi-bin/message/send?access_token=${accessToken}`;

                // 确定用户ID
                const userId = touser || '@all';

                // 构建消息体
                let messageBody: any;
                switch (msgtype) {
                    case '0': // textcard
                        messageBody = {
                            msgtype: 'textcard',
                            textcard: {
                                title: text,
                                description: desp,
                                url: 'https://github.com/whyour/qinglong',
                                btntxt: '更多'
                            }
                        };
                        break;
                    case '1': // text
                        messageBody = {
                            msgtype: 'text',
                            text: {
                                content: `${text}\n\n${desp}`
                            }
                        };
                        break;
                    default: // 默认text
                        messageBody = {
                            msgtype: 'text',
                            text: {
                                content: `${text}\n\n${desp}`
                            }
                        };
                }

                const body = {
                    touser: userId,
                    agentid: parseInt(agentid),
                    safe: '0',
                    ...messageBody
                };

                $.post({ url: sendUrl, json: body }, (sendErr, _sendResp, sendData) => {
                    if (sendErr) {
                        logger.error({ err: sendErr }, '企业微信应用发送失败');
                        resolve({ success: false, error: sendErr as Error });
                    } else {
                        const sendResult = sendData as { errcode?: number; errmsg?: string };
                        if (sendResult.errcode === 0) {
                            logger.info('企业微信应用发送成功');
                            resolve({ success: true, message: '发送成功' });
                        } else {
                            logger.warn({ msg: sendResult.errmsg }, '企业微信应用发送失败');
                            resolve({ success: false, message: sendResult.errmsg || '发送失败' });
                        }
                    }
                });
            });
        });
    }
};

// 检查是否启用
if (hasConfig('QYWX_KEY')) {
    wechatWorkBotChannel.enabled = true;
}
if (hasConfig('QYWX_AM')) {
    wechatWorkAppChannel.enabled = true;
}

export default { wechatWorkBotChannel, wechatWorkAppChannel };
