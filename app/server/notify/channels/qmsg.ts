/**
 * Qmsg 酱推送渠道
 */
import Logger from '../../utils/logger';
import { NotifyChannel, NotifyResult } from '../types';
import { $ } from '../httpClient';
import { hasConfig, getConfig } from '../config';

const logger = Logger.child({ channel: 'qmsg' });

export const qmsgChannel: NotifyChannel = {
    name: 'qmsg',
    enabled: false,

    async send(text: string, desp: string): Promise<NotifyResult> {
        return new Promise((resolve) => {
            if (!hasConfig('QMSG_KEY', 'QMSG_TYPE')) {
                resolve({ success: false, message: 'QMSG 配置不完整' });
                return;
            }

            const QMSG_KEY = getConfig('QMSG_KEY');
            const QMSG_TYPE = getConfig('QMSG_TYPE');
            const url = `https://qmsg.zendee.cn/${QMSG_TYPE}/${QMSG_KEY}`;

            // 替换特殊字符
            const content = `${text}\n\n${desp.replace('----', '-')}`;
            const body = `msg=${encodeURIComponent(content)}`;

            $.post({
                url,
                body,
                headers: { 'Content-Type': 'application/x-www-form-urlencoded' }
            }, (err, _resp, data) => {
                if (err) {
                    logger.error({ err }, 'Qmsg 发送失败');
                    resolve({ success: false, error: err as Error });
                } else {
                    const result = data as { code?: number };
                    if (result?.code === 0) {
                        logger.info('Qmsg 发送成功');
                        resolve({ success: true, message: '发送成功' });
                    } else {
                        logger.warn({ body: data }, 'Qmsg 发送失败');
                        resolve({ success: false, message: '发送失败' });
                    }
                }
            });
        });
    }
};

if (hasConfig('QMSG_KEY', 'QMSG_TYPE')) {
    qmsgChannel.enabled = true;
}

export default qmsgChannel;
