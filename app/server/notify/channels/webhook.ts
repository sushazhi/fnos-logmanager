/**
 * 自定义 Webhook 推送渠道
 */
import Logger from '../../utils/logger';
import { NotifyChannel, NotifyResult } from '../types';
import { httpClient } from '../httpClient';
import { hasConfig, getConfig } from '../config';

const logger = Logger.child({ channel: 'webhook' });

export const webhookChannel: NotifyChannel = {
    name: 'webhook',
    enabled: false,

    async send(text: string, desp: string): Promise<NotifyResult> {
        if (!hasConfig('WEBHOOK_URL')) {
            return { success: false, message: 'WEBHOOK_URL 未配置' };
        }

        const WEBHOOK_URL = getConfig('WEBHOOK_URL');
        const WEBHOOK_BODY = getConfig('WEBHOOK_BODY');
        const WEBHOOK_HEADERS = getConfig('WEBHOOK_HEADERS');
        const WEBHOOK_METHOD = getConfig('WEBHOOK_METHOD') || 'POST';
        const WEBHOOK_CONTENT_TYPE = getConfig('WEBHOOK_CONTENT_TYPE') || 'application/json';

        try {
            let body: string | undefined;
            const headers: Record<string, string> = {};

            // 设置 Content-Type
            headers['content-type'] = WEBHOOK_CONTENT_TYPE;

            // 解析自定义请求头
            if (WEBHOOK_HEADERS) {
                try {
                    const customHeaders = JSON.parse(WEBHOOK_HEADERS);
                    Object.assign(headers, customHeaders);
                } catch {
                    logger.warn('WEBHOOK_HEADERS 解析失败');
                }
            }

            // 构建请求体
            if (WEBHOOK_BODY) {
                body = WEBHOOK_BODY
                    .replace(/\$\{title\}/g, text)
                    .replace(/\$\{content\}/g, desp);
            } else {
                body = JSON.stringify({ title: text, content: desp });
            }

            const response = await httpClient.request(WEBHOOK_URL || '', {
                method: WEBHOOK_METHOD as 'GET' | 'POST' | 'PUT' | 'DELETE',
                headers,
                body
            });

            if (response.statusCode >= 200 && response.statusCode < 300) {
                logger.info({ statusCode: response.statusCode }, 'Webhook 发送成功');
                return { success: true, message: '发送成功' };
            } else {
                logger.warn({ statusCode: response.statusCode, body: response.body }, 'Webhook 发送失败');
                return { success: false, message: `HTTP ${response.statusCode}` };
            }
        } catch (err) {
            logger.error({ err }, 'Webhook 发送失败');
            return { success: false, error: err as Error };
        }
    }
};

// 检查是否启用
if (hasConfig('WEBHOOK_URL')) {
    webhookChannel.enabled = true;
}

export default webhookChannel;
