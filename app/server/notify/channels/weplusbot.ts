/**
 * 微加机器人推送渠道
 */
import Logger from '../../utils/logger';
import { NotifyChannel, NotifyResult } from '../types';
import { $ } from '../httpClient';
import { hasConfig, getConfig } from '../config';

const logger = Logger.child({ channel: 'weplusbot' });

export const weplusbotChannel: NotifyChannel = {
    name: 'weplusbot',
    enabled: false,

    async send(text: string, desp: string): Promise<NotifyResult> {
        return new Promise((resolve) => {
            if (!hasConfig('WE_PLUS_BOT_TOKEN')) {
                resolve({ success: false, message: 'WE_PLUS_BOT_TOKEN 未配置' });
                return;
            }

            const WE_PLUS_BOT_TOKEN = getConfig('WE_PLUS_BOT_TOKEN');
            const WE_PLUS_BOT_RECEIVER = getConfig('WE_PLUS_BOT_RECEIVER');
            const WE_PLUS_BOT_VERSION = getConfig('WE_PLUS_BOT_VERSION') || 'pro';

            const url = WE_PLUS_BOT_VERSION === 'personal'
                ? 'https://personal.weplusbot.com/send'
                : 'https://pro.weplusbot.com/send';

            const body = {
                token: WE_PLUS_BOT_TOKEN,
                receiver: WE_PLUS_BOT_RECEIVER,
                title: text,
                content: desp
            };

            $.post({ url, json: body }, (err, _resp, data) => {
                if (err) {
                    logger.error({ err }, '微加机器人发送失败');
                    resolve({ success: false, error: err as Error });
                } else {
                    const result = data as { code?: number; message?: string };
                    if (result.code === 0 || result.code === 200) {
                        logger.info('微加机器人发送成功');
                        resolve({ success: true, message: '发送成功' });
                    } else {
                        logger.warn({ body: data }, '微加机器人发送失败');
                        resolve({ success: false, message: result.message || '发送失败' });
                    }
                }
            });
        });
    }
};

if (hasConfig('WE_PLUS_BOT_TOKEN')) {
    weplusbotChannel.enabled = true;
}

export default weplusbotChannel;
