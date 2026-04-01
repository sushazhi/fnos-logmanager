/**
 * Synology Chat 推送渠道
 */
import Logger from '../../utils/logger';
import { NotifyChannel, NotifyResult } from '../types';
import { $ } from '../httpClient';
import { hasConfig, getConfig } from '../config';

const logger = Logger.child({ channel: 'chat' });

export const chatChannel: NotifyChannel = {
    name: 'synology-chat',
    enabled: false,

    async send(text: string, desp: string): Promise<NotifyResult> {
        return new Promise((resolve) => {
            if (!hasConfig('CHAT_URL', 'CHAT_TOKEN')) {
                resolve({ success: false, message: 'CHAT 配置不完整' });
                return;
            }

            const CHAT_URL = getConfig('CHAT_URL');
            const CHAT_TOKEN = getConfig('CHAT_TOKEN');
            const url = `${CHAT_URL}${CHAT_TOKEN}`;

            // 对消息内容进行 urlencode
            const encodedDesp = encodeURI(desp);
            const body = `payload={"text":"${text}\n${encodedDesp}"}`;

            $.post({
                url,
                body,
                headers: { 'Content-Type': 'application/x-www-form-urlencoded' }
            }, (err, _resp, data) => {
                if (err) {
                    logger.error({ err }, 'Synology Chat 发送失败');
                    resolve({ success: false, error: err as Error });
                } else {
                    const result = data as { success?: boolean };
                    if (result?.success) {
                        logger.info('Synology Chat 发送成功');
                        resolve({ success: true, message: '发送成功' });
                    } else {
                        logger.warn({ body: data }, 'Synology Chat 发送失败');
                        resolve({ success: false, message: '发送失败' });
                    }
                }
            });
        });
    }
};

if (hasConfig('CHAT_URL', 'CHAT_TOKEN')) {
    chatChannel.enabled = true;
}

export default chatChannel;
