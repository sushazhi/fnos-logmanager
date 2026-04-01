/**
 * PushMe 推送渠道
 */
import Logger from '../../utils/logger';
import { NotifyChannel, NotifyResult } from '../types';
import { $ } from '../httpClient';
import { hasConfig, getConfig } from '../config';

const logger = Logger.child({ channel: 'pushme' });

export const pushmeChannel: NotifyChannel = {
    name: 'pushme',
    enabled: false,

    async send(text: string, desp: string): Promise<NotifyResult> {
        return new Promise((resolve) => {
            if (!hasConfig('PUSHME_KEY')) {
                resolve({ success: false, message: 'PUSHME_KEY 未配置' });
                return;
            }

            const PUSHME_KEY = getConfig('PUSHME_KEY');
            const url = `https://push.i-i.me?pushkey=${PUSHME_KEY}`;

            const body = {
                title: text,
                content: desp
            };

            $.post({ url, json: body }, (err, _resp, data) => {
                if (err) {
                    logger.error({ err }, 'PushMe 发送失败');
                    resolve({ success: false, error: err as Error });
                } else {
                    const result = data as { code?: number; message?: string };
                    if (result.code === 200 || result.code === 0) {
                        logger.info('PushMe 发送成功');
                        resolve({ success: true, message: '发送成功' });
                    } else {
                        logger.warn({ body: data }, 'PushMe 发送失败');
                        resolve({ success: false, message: result.message || '发送失败' });
                    }
                }
            });
        });
    }
};

if (hasConfig('PUSHME_KEY')) {
    pushmeChannel.enabled = true;
}

export default pushmeChannel;
