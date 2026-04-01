/**
 * WxPusher 推送渠道
 */
import Logger from '../../utils/logger';
import { NotifyChannel, NotifyResult } from '../types';
import { $ } from '../httpClient';
import { hasConfig, getConfig } from '../config';

const logger = Logger.child({ channel: 'wxpusher' });

export const wxpusherChannel: NotifyChannel = {
    name: 'wxpusher',
    enabled: false,

    async send(text: string, desp: string): Promise<NotifyResult> {
        return new Promise((resolve) => {
            if (!hasConfig('WXPUSHER_APP_TOKEN')) {
                resolve({ success: false, message: 'WXPUSHER_APP_TOKEN 未配置' });
                return;
            }

            const WXPUSHER_APP_TOKEN = getConfig('WXPUSHER_APP_TOKEN');
            const WXPUSHER_TOPIC_IDS = getConfig('WXPUSHER_TOPIC_IDS');
            const WXPUSHER_UIDS = getConfig('WXPUSHER_UIDS');

            // 处理topic_ids，将分号分隔的字符串转为数组
            const topicIds = WXPUSHER_TOPIC_IDS
                ? WXPUSHER_TOPIC_IDS.split(';')
                    .map((id) => id.trim())
                    .filter((id) => id)
                    .map((id) => parseInt(id))
                : [];

            // 处理uids，将分号分隔的字符串转为数组
            const uids = WXPUSHER_UIDS
                ? WXPUSHER_UIDS.split(';')
                    .map((uid) => uid.trim())
                    .filter((uid) => uid)
                : [];

            // topic_ids uids 至少有一个
            if (!topicIds.length && !uids.length) {
                resolve({ success: false, message: 'WXPUSHER_TOPIC_IDS 和 WXPUSHER_UIDS 至少设置一个' });
                return;
            }

            const url = 'https://wxpusher.zjiecode.com/api/send/message';
            const body = {
                appToken: WXPUSHER_APP_TOKEN,
                content: `<h1>${text}</h1><br/><div style='white-space: pre-wrap;'>${desp}</div>`,
                summary: text,
                contentType: 2, // HTML格式
                topicIds: topicIds,
                uids: uids,
                verifyPayType: 0,
            };

            $.post({ url, json: body }, (err, _resp, data) => {
                if (err) {
                    logger.error({ err }, 'WxPusher 发送失败');
                    resolve({ success: false, error: err as Error });
                } else {
                    const result = data as { code?: number; msg?: string };
                    if (result?.code === 1000) {
                        logger.info('WxPusher 发送成功');
                        resolve({ success: true, message: '发送成功' });
                    } else {
                        logger.warn({ body: data }, 'WxPusher 发送失败');
                        resolve({ success: false, message: result?.msg || '发送失败' });
                    }
                }
            });
        });
    }
};

if (hasConfig('WXPUSHER_APP_TOKEN')) {
    wxpusherChannel.enabled = true;
}

export default wxpusherChannel;
