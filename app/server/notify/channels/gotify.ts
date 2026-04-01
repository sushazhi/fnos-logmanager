/**
 * Gotify 推送渠道
 */
import Logger from '../../utils/logger';
import { NotifyChannel, NotifyResult } from '../types';
import { $ } from '../httpClient';
import { hasConfig, getConfig } from '../config';

const logger = Logger.child({ channel: 'gotify' });

export const gotifyChannel: NotifyChannel = {
    name: 'gotify',
    enabled: false,

    async send(text: string, desp: string): Promise<NotifyResult> {
        return new Promise((resolve) => {
            if (!hasConfig('GOTIFY_URL', 'GOTIFY_TOKEN')) {
                resolve({ success: false, message: 'Gotify 配置不完整' });
                return;
            }

            const GOTIFY_URL = getConfig('GOTIFY_URL');
            const GOTIFY_TOKEN = getConfig('GOTIFY_TOKEN');
            const GOTIFY_PRIORITY = getConfig('GOTIFY_PRIORITY') || 0;

            const url = `${GOTIFY_URL}/message?token=${GOTIFY_TOKEN}`;
            const body = `title=${encodeURIComponent(text)}&message=${encodeURIComponent(desp)}&priority=${GOTIFY_PRIORITY}`;

            $.post({
                url,
                body,
                headers: { 'Content-Type': 'application/x-www-form-urlencoded' }
            }, (err, _resp, data) => {
                if (err) {
                    logger.error({ err }, 'Gotify 发送失败');
                    resolve({ success: false, error: err as Error });
                } else {
                    const result = data as { id?: number; message?: string };
                    if (result?.id) {
                        logger.info('Gotify 发送成功');
                        resolve({ success: true, message: '发送成功' });
                    } else {
                        logger.warn({ body: data }, 'Gotify 发送失败');
                        resolve({ success: false, message: result?.message || '发送失败' });
                    }
                }
            });
        });
    }
};

if (hasConfig('GOTIFY_URL', 'GOTIFY_TOKEN')) {
    gotifyChannel.enabled = true;
}

export default gotifyChannel;
