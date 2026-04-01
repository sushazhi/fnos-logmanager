/**
 * 钉钉机器人推送渠道
 */
import crypto from 'crypto';
import Logger from '../../utils/logger';
import { NotifyChannel, NotifyResult } from '../types';
import { $ } from '../httpClient';
import { hasConfig, getConfig } from '../config';

const logger = Logger.child({ channel: 'dingtalk' });

export const dingtalkChannel: NotifyChannel = {
    name: 'dingtalk',
    enabled: false,

    async send(text: string, desp: string): Promise<NotifyResult> {
        return new Promise((resolve) => {
            if (!hasConfig('DD_BOT_TOKEN')) {
                resolve({ success: false, message: 'DD_BOT_TOKEN 未配置' });
                return;
            }

            const DD_BOT_TOKEN = getConfig('DD_BOT_TOKEN');
            const DD_BOT_SECRET = getConfig('DD_BOT_SECRET');

            let url = `https://oapi.dingtalk.com/robot/send?access_token=${DD_BOT_TOKEN}`;

            // 签名
            if (DD_BOT_SECRET) {
                const now = Date.now();
                const stringToSign = `${now}\n${DD_BOT_SECRET}`;
                const sign = crypto
                    .createHmac('sha256', DD_BOT_SECRET)
                    .update(stringToSign)
                    .digest('base64');
                url += `&timestamp=${now}&sign=${encodeURIComponent(sign)}`;
            }

            const title = text.substring(0, 64);
            const content = desp.substring(0, 15000);

            const body = {
                msgtype: 'markdown',
                markdown: {
                    title,
                    text: content
                }
            };

            $.post({ url, json: body }, (err, _res, resBody) => {
                if (err) {
                    logger.error({ err }, '钉钉发送失败');
                    resolve({ success: false, error: err as Error });
                    return;
                }

                const data = resBody as { errcode?: number; errmsg?: string };
                if (data?.errcode === 0) {
                    logger.info('钉钉发送成功');
                    resolve({ success: true, message: '发送成功' });
                } else {
                    logger.warn({ body: resBody }, '钉钉发送失败');
                    resolve({ success: false, message: data?.errmsg || '发送失败' });
                }
            });
        });
    }
};

// 检查是否启用
if (hasConfig('DD_BOT_TOKEN')) {
    dingtalkChannel.enabled = true;
}

export default dingtalkChannel;
