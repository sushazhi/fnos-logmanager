/**
 * Server酱 推送渠道
 */
import Logger from '../../utils/logger';
import { NotifyChannel, NotifyResult } from '../types';
import { $ } from '../httpClient';
import { hasConfig, getConfig } from '../config';

const logger = Logger.child({ channel: 'serverChan' });

export const serverChanChannel: NotifyChannel = {
    name: 'serverChan',
    enabled: false,

    async send(text: string, desp: string): Promise<NotifyResult> {
        return new Promise((resolve) => {
            if (!hasConfig('PUSH_KEY')) {
                resolve({ success: false, message: 'PUSH_KEY 未配置' });
                return;
            }

            const PUSH_KEY = getConfig('PUSH_KEY') || '';

            // 微信server酱推送通知一个\n不会换行，需要两个\n才能换行
            const formattedDesp = desp.replace(/[\n\r]/g, '\n\n');

            // 判断是 Turbo 版还是旧版
            const matchResult = PUSH_KEY.match(/^sctp(\d+)t/i);
            const url = matchResult && matchResult[1]
                ? `https://${matchResult[1]}.push.ft07.com/send/${PUSH_KEY}.send`
                : `https://sctapi.ftqq.com/${PUSH_KEY}.send`;

            const body = `text=${encodeURIComponent(text)}&desp=${encodeURIComponent(formattedDesp)}`;

            $.post({
                url,
                body,
                headers: { 'Content-Type': 'application/x-www-form-urlencoded' }
            }, (err, _resp, data) => {
                if (err) {
                    logger.error({ err }, 'Server酱 发送失败');
                    resolve({ success: false, error: err as Error });
                } else {
                    // server酱和Server酱·Turbo版的返回json格式不太一样
                    const result = data as { errno?: number; data?: { errno?: number }; errmsg?: string };
                    if (result?.errno === 0 || result?.data?.errno === 0) {
                        logger.info('Server酱 发送成功');
                        resolve({ success: true, message: '发送成功' });
                    } else if (result?.errno === 1024) {
                        // 一分钟内发送相同的内容会触发
                        logger.warn({ msg: result.errmsg }, 'Server酱 发送频率限制');
                        resolve({ success: false, message: result.errmsg || '发送频率限制' });
                    } else {
                        logger.warn({ body: data }, 'Server酱 发送失败');
                        resolve({ success: false, message: '发送失败' });
                    }
                }
            });
        });
    }
};

// 检查是否启用
if (hasConfig('PUSH_KEY')) {
    serverChanChannel.enabled = true;
}

export default serverChanChannel;
