/**
 * iGot 聚合推送渠道
 */
import Logger from '../../utils/logger';
import { NotifyChannel, NotifyResult, NotifyParams } from '../types';
import { $ } from '../httpClient';
import { hasConfig, getConfig } from '../config';

const logger = Logger.child({ channel: 'igot' });

export const igotChannel: NotifyChannel = {
    name: 'igot',
    enabled: false,

    async send(text: string, desp: string, params: NotifyParams = {}): Promise<NotifyResult> {
        return new Promise((resolve) => {
            if (!hasConfig('IGOT_PUSH_KEY')) {
                resolve({ success: false, message: 'IGOT_PUSH_KEY 未配置' });
                return;
            }

            const IGOT_PUSH_KEY = getConfig('IGOT_PUSH_KEY') || '';

            // 校验传入的IGOT_PUSH_KEY是否有效
            const IGOT_PUSH_KEY_REGX = new RegExp('^[a-zA-Z0-9]{24}$');
            if (!IGOT_PUSH_KEY_REGX.test(IGOT_PUSH_KEY)) {
                resolve({ success: false, message: 'IGOT_PUSH_KEY 无效' });
                return;
            }

            const url = `https://push.hellyw.com/${IGOT_PUSH_KEY.toLowerCase()}`;

            // 构建额外参数
            const extraParams = Object.entries(params)
                .map(([k, v]) => `${k}=${v}`)
                .join('&');

            const body = `title=${text}&content=${desp}${extraParams ? '&' + extraParams : ''}`;

            $.post({
                url,
                body,
                headers: { 'Content-Type': 'application/x-www-form-urlencoded' }
            }, (err, _resp, data) => {
                if (err) {
                    logger.error({ err }, 'iGot 发送失败');
                    resolve({ success: false, error: err as Error });
                } else {
                    const result = data as { ret?: number; errMsg?: string };
                    if (result?.ret === 0) {
                        logger.info('iGot 发送成功');
                        resolve({ success: true, message: '发送成功' });
                    } else {
                        logger.warn({ body: data }, 'iGot 发送失败');
                        resolve({ success: false, message: result?.errMsg || '发送失败' });
                    }
                }
            });
        });
    }
};

if (hasConfig('IGOT_PUSH_KEY')) {
    igotChannel.enabled = true;
}

export default igotChannel;
