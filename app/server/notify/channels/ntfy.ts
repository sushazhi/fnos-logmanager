/**
 * Ntfy 推送渠道
 */
import Logger from '../../utils/logger';
import { NotifyChannel, NotifyResult } from '../types';
import { $ } from '../httpClient';
import { hasConfig, getConfig } from '../config';

const logger = Logger.child({ channel: 'ntfy' });

/**
 * RFC2047 编码
 */
function encodeRFC2047(text: string): string {
    const encodedBase64 = Buffer.from(text).toString('base64');
    return `=?utf-8?B?${encodedBase64}?=`;
}

export const ntfyChannel: NotifyChannel = {
    name: 'ntfy',
    enabled: false,

    async send(text: string, desp: string): Promise<NotifyResult> {
        return new Promise((resolve) => {
            if (!hasConfig('NTFY_TOPIC')) {
                resolve({ success: false, message: 'NTFY_TOPIC 未配置' });
                return;
            }

            const NTFY_URL = getConfig('NTFY_URL') || 'https://ntfy.sh';
            const NTFY_TOPIC = getConfig('NTFY_TOPIC');
            const NTFY_PRIORITY = getConfig('NTFY_PRIORITY') || '3';
            const NTFY_TOKEN = getConfig('NTFY_TOKEN');
            const NTFY_USERNAME = getConfig('NTFY_USERNAME');
            const NTFY_PASSWORD = getConfig('NTFY_PASSWORD');
            const NTFY_ACTIONS = getConfig('NTFY_ACTIONS');

            const url = `${NTFY_URL}/${NTFY_TOPIC}`;
            const headers: Record<string, string> = {
                Title: encodeRFC2047(text),
                Priority: NTFY_PRIORITY,
                Icon: 'https://qn.whyour.cn/logo.png',
            };

            // 认证
            if (NTFY_TOKEN) {
                headers['Authorization'] = `Bearer ${NTFY_TOKEN}`;
            } else if (NTFY_USERNAME && NTFY_PASSWORD) {
                headers['Authorization'] = `Basic ${Buffer.from(`${NTFY_USERNAME}:${NTFY_PASSWORD}`).toString('base64')}`;
            }

            // Actions
            if (NTFY_ACTIONS) {
                headers['Actions'] = encodeRFC2047(NTFY_ACTIONS);
            }

            $.post({ url, body: desp, headers }, (err, _resp, data) => {
                if (err) {
                    logger.error({ err }, 'Ntfy 发送失败');
                    resolve({ success: false, error: err as Error });
                } else {
                    const result = data as { id?: string };
                    if (result?.id) {
                        logger.info('Ntfy 发送成功');
                        resolve({ success: true, message: '发送成功' });
                    } else {
                        logger.warn({ body: data }, 'Ntfy 发送失败');
                        resolve({ success: false, message: '发送失败' });
                    }
                }
            });
        });
    }
};

if (hasConfig('NTFY_TOPIC')) {
    ntfyChannel.enabled = true;
}

export default ntfyChannel;
