/**
 * PushDeer 推送渠道
 */
import Logger from '../../utils/logger';
import { NotifyChannel, NotifyResult } from '../types';
import { $ } from '../httpClient';
import { hasConfig, getConfig } from '../config';

const logger = Logger.child({ channel: 'pushdeer' });

export const pushdeerChannel: NotifyChannel = {
    name: 'pushdeer',
    enabled: false,

    async send(text: string, desp: string): Promise<NotifyResult> {
        return new Promise((resolve) => {
            if (!hasConfig('DEER_KEY')) {
                resolve({ success: false, message: 'DEER_KEY 未配置' });
                return;
            }

            const DEER_KEY = getConfig('DEER_KEY');
            const DEER_URL = getConfig('DEER_URL') || 'https://api2.pushdeer.com/message/push';

            // PushDeer 建议对消息内容进行 urlencode
            const encodedDesp = encodeURI(desp);
            const body = `pushkey=${DEER_KEY}&text=${text}&desp=${encodedDesp}&type=markdown`;

            $.post({
                url: DEER_URL,
                body,
                headers: { 'Content-Type': 'application/x-www-form-urlencoded' }
            }, (err, _resp, data) => {
                if (err) {
                    logger.error({ err }, 'PushDeer 发送失败');
                    resolve({ success: false, error: err as Error });
                } else {
                    const result = data as { content?: { result?: any[] } };
                    if (result?.content?.result && result.content.result.length > 0) {
                        logger.info('PushDeer 发送成功');
                        resolve({ success: true, message: '发送成功' });
                    } else {
                        logger.warn({ body: data }, 'PushDeer 发送失败');
                        resolve({ success: false, message: '发送失败' });
                    }
                }
            });
        });
    }
};

if (hasConfig('DEER_KEY')) {
    pushdeerChannel.enabled = true;
}

export default pushdeerChannel;
