/**
 * Telegram Bot 推送渠道
 */
import Logger from '../../utils/logger';
import { NotifyChannel, NotifyResult } from '../types';
import { $ } from '../httpClient';
import { hasConfig, getConfig } from '../config';

const logger = Logger.child({ channel: 'telegram' });

export const telegramChannel: NotifyChannel = {
    name: 'telegram',
    enabled: false,

    async send(text: string, desp: string): Promise<NotifyResult> {
        return new Promise((resolve) => {
            if (!hasConfig('TG_BOT_TOKEN', 'TG_USER_ID')) {
                resolve({ success: false, message: 'TG_BOT_TOKEN 或 TG_USER_ID 未配置' });
                return;
            }

            const TG_BOT_TOKEN = getConfig('TG_BOT_TOKEN');
            const TG_USER_ID = getConfig('TG_USER_ID');
            const TG_API_HOST = getConfig('TG_API_HOST') || 'https://api.telegram.org';

            const url = `${TG_API_HOST}/bot${TG_BOT_TOKEN}/sendMessage`;
            const content = `${text}\n\n${desp}`.substring(0, 4096);

            const body = {
                chat_id: TG_USER_ID,
                text: content,
                parse_mode: 'HTML'
            };

            $.post({ url, json: body }, (err, _res, resBody) => {
                if (err) {
                    logger.error({ err }, 'Telegram 发送失败');
                    resolve({ success: false, error: err as Error });
                    return;
                }

                const data = resBody as { ok?: boolean; description?: string };
                if (data?.ok) {
                    logger.info('Telegram 发送成功');
                    resolve({ success: true, message: '发送成功' });
                } else {
                    logger.warn({ body: resBody }, 'Telegram 发送失败');
                    resolve({ success: false, message: data?.description || '发送失败' });
                }
            });
        });
    }
};

// 检查是否启用
if (hasConfig('TG_BOT_TOKEN', 'TG_USER_ID')) {
    telegramChannel.enabled = true;
}

export default telegramChannel;
