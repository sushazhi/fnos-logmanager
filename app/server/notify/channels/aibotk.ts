/**
 * 智能微秘书推送渠道
 */
import Logger from '../../utils/logger';
import { NotifyChannel, NotifyResult } from '../types';
import { $ } from '../httpClient';
import { hasConfig, getConfig } from '../config';

const logger = Logger.child({ channel: 'aibotk' });

export const aibotkChannel: NotifyChannel = {
    name: 'aibotk',
    enabled: false,

    async send(text: string, desp: string): Promise<NotifyResult> {
        return new Promise((resolve) => {
            if (!hasConfig('AIBOTK_KEY', 'AIBOTK_TYPE', 'AIBOTK_NAME')) {
                resolve({ success: false, message: 'AIBOTK 配置不完整' });
                return;
            }

            const AIBOTK_KEY = getConfig('AIBOTK_KEY');
            const AIBOTK_TYPE = getConfig('AIBOTK_TYPE');
            const AIBOTK_NAME = getConfig('AIBOTK_NAME');

            let url: string;
            let json: any;

            switch (AIBOTK_TYPE) {
                case 'room':
                    url = 'https://api-bot.aibotk.com/openapi/v1/chat/room';
                    json = {
                        apiKey: AIBOTK_KEY,
                        roomName: AIBOTK_NAME,
                        message: {
                            type: 1,
                            content: `【青龙快讯】\n\n${text}\n${desp}`,
                        },
                    };
                    break;
                case 'contact':
                    url = 'https://api-bot.aibotk.com/openapi/v1/chat/contact';
                    json = {
                        apiKey: AIBOTK_KEY,
                        name: AIBOTK_NAME,
                        message: {
                            type: 1,
                            content: `【青龙快讯】\n\n${text}\n${desp}`,
                        },
                    };
                    break;
                default:
                    resolve({ success: false, message: 'AIBOTK_TYPE 类型错误' });
                    return;
            }

            $.post({ url, json }, (err, _resp, data) => {
                if (err) {
                    logger.error({ err }, '智能微秘书发送失败');
                    resolve({ success: false, error: err as Error });
                } else {
                    const result = data as { code?: number; error?: string };
                    if (result?.code === 0) {
                        logger.info('智能微秘书发送成功');
                        resolve({ success: true, message: '发送成功' });
                    } else {
                        logger.warn({ body: data }, '智能微秘书发送失败');
                        resolve({ success: false, message: result?.error || '发送失败' });
                    }
                }
            });
        });
    }
};

if (hasConfig('AIBOTK_KEY', 'AIBOTK_TYPE', 'AIBOTK_NAME')) {
    aibotkChannel.enabled = true;
}

export default aibotkChannel;
