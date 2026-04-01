/**
 * Bark 推送渠道
 */
import Logger from '../../utils/logger';
import { NotifyChannel, NotifyResult, NotifyParams } from '../types';
import { $ } from '../httpClient';
import { hasConfig, getConfig } from '../config';

const logger = Logger.child({ channel: 'bark' });

export const barkChannel: NotifyChannel = {
    name: 'bark',
    enabled: false,

    async send(text: string, desp: string, params: NotifyParams = {}): Promise<NotifyResult> {
        return new Promise((resolve) => {
            if (!hasConfig('BARK_PUSH')) {
                resolve({ success: false, message: 'BARK_PUSH 未配置' });
                return;
            }

            let BARK_PUSH = getConfig('BARK_PUSH') || '';
            const BARK_ICON = getConfig('BARK_ICON');
            const BARK_SOUND = getConfig('BARK_SOUND');
            const BARK_GROUP = getConfig('BARK_GROUP');
            const BARK_LEVEL = getConfig('BARK_LEVEL');
            const BARK_ARCHIVE = getConfig('BARK_ARCHIVE');
            const BARK_URL = getConfig('BARK_URL');

            // 兼容BARK本地用户只填写设备码的情况
            if (!BARK_PUSH.startsWith('http')) {
                BARK_PUSH = `https://api.day.app/${BARK_PUSH}`;
            }

            const body = {
                title: text,
                body: desp,
                icon: BARK_ICON,
                sound: BARK_SOUND,
                group: BARK_GROUP,
                isArchive: BARK_ARCHIVE,
                level: BARK_LEVEL,
                url: BARK_URL,
                ...params,
            };

            $.post({ url: BARK_PUSH, json: body }, (err, _resp, data) => {
                if (err) {
                    logger.error({ err }, 'Bark 发送失败');
                    resolve({ success: false, error: err as Error });
                } else {
                    const result = data as { code?: number; message?: string };
                    if (result?.code === 200) {
                        logger.info('Bark 发送成功');
                        resolve({ success: true, message: '发送成功' });
                    } else {
                        logger.warn({ body: data }, 'Bark 发送失败');
                        resolve({ success: false, message: result?.message || '发送失败' });
                    }
                }
            });
        });
    }
};

// 检查是否启用
if (hasConfig('BARK_PUSH')) {
    barkChannel.enabled = true;
}

export default barkChannel;
