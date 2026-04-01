/**
 * PushPlus 推送渠道
 */
import Logger from '../../utils/logger';
import { NotifyChannel, NotifyResult } from '../types';
import { $ } from '../httpClient';
import { hasConfig, getConfig } from '../config';

const logger = Logger.child({ channel: 'pushplus' });

export const pushplusChannel: NotifyChannel = {
    name: 'pushplus',
    enabled: false,

    async send(text: string, desp: string): Promise<NotifyResult> {
        return new Promise((resolve) => {
            if (!hasConfig('PUSH_PLUS_TOKEN')) {
                resolve({ success: false, message: 'PUSH_PLUS_TOKEN 未配置' });
                return;
            }

            const PUSH_PLUS_TOKEN = getConfig('PUSH_PLUS_TOKEN');
            const PUSH_PLUS_USER = getConfig('PUSH_PLUS_USER');
            const PUSH_PLUS_TEMPLATE = getConfig('PUSH_PLUS_TEMPLATE') || 'html';
            const PUSH_PLUS_CHANNEL = getConfig('PUSH_PLUS_CHANNEL') || 'wechat';
            const PUSH_PLUS_WEBHOOK = getConfig('PUSH_PLUS_WEBHOOK');
            const PUSH_PLUS_CALLBACKURL = getConfig('PUSH_PLUS_CALLBACKURL');
            const PUSH_PLUS_TO = getConfig('PUSH_PLUS_TO');

            // 默认为html, 不支持plaintext
            const formattedDesp = desp.replace(/[\n\r]/g, '<br>');

            const url = 'https://www.pushplus.plus/send';
            const body = {
                token: PUSH_PLUS_TOKEN,
                title: text,
                content: formattedDesp,
                topic: PUSH_PLUS_USER,
                template: PUSH_PLUS_TEMPLATE,
                channel: PUSH_PLUS_CHANNEL,
                webhook: PUSH_PLUS_WEBHOOK,
                callbackUrl: PUSH_PLUS_CALLBACKURL,
                to: PUSH_PLUS_TO,
            };

            $.post({ url, json: body }, (err, _resp, data) => {
                if (err) {
                    logger.error({ err }, 'PushPlus 发送失败');
                    resolve({ success: false, error: err as Error });
                } else {
                    const result = data as { code?: number; msg?: string; data?: string };
                    if (result?.code === 200) {
                        logger.info({ flowId: result.data }, 'PushPlus 发送成功');
                        resolve({ success: true, message: '发送成功' });
                    } else {
                        logger.warn({ body: data }, 'PushPlus 发送失败');
                        resolve({ success: false, message: result?.msg || '发送失败' });
                    }
                }
            });
        });
    }
};

if (hasConfig('PUSH_PLUS_TOKEN')) {
    pushplusChannel.enabled = true;
}

export default pushplusChannel;
