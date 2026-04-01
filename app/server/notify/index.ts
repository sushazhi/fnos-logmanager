/**
 * 通知系统入口
 * 导出 sendNotify 函数，保持与原有接口兼容
 */
import Logger from '../utils/logger';
import { registry } from './registry';
import { NotifyResult, NotifyParams } from './types';

// 重新导出配置函数
export { setConfig, hasConfig, getConfig } from './config';
export type { ChannelConfig } from './types';

// 导入所有渠道
import {
    barkChannel,
    dingtalkChannel,
    telegramChannel,
    serverChanChannel,
    pushplusChannel,
    webhookChannel,
    feishuBotChannel,
    feishuAppChannel,
    wechatWorkBotChannel,
    wechatWorkAppChannel,
    wechatBotChannel,
    ntfyChannel,
    gotifyChannel,
    igotChannel,
    pushdeerChannel,
    chatChannel,
    qmsgChannel,
    pushmeChannel,
    wxpusherChannel,
    aibotkChannel,
    weplusbotChannel,
    qqbotChannel
} from './channels';

const logger = Logger.child({ module: 'Notify' });

// 注册所有渠道
registry.register(barkChannel);
registry.register(dingtalkChannel);
registry.register(telegramChannel);
registry.register(serverChanChannel);
registry.register(pushplusChannel);
registry.register(webhookChannel);
registry.register(feishuBotChannel);
registry.register(feishuAppChannel);
registry.register(wechatWorkBotChannel);
registry.register(wechatWorkAppChannel);
registry.register(wechatBotChannel);
registry.register(ntfyChannel);
registry.register(gotifyChannel);
registry.register(igotChannel);
registry.register(pushdeerChannel);
registry.register(chatChannel);
registry.register(qmsgChannel);
registry.register(pushmeChannel);
registry.register(wxpusherChannel);
registry.register(aibotkChannel);
registry.register(weplusbotChannel);
registry.register(qqbotChannel);

/**
 * 发送通知
 * @param text 标题
 * @param desp 内容
 * @param params 额外参数
 */
export async function sendNotify(
    text: string,
    desp: string,
    params: NotifyParams = {}
): Promise<NotifyResult> {
    const channels = registry.getEnabled();

    if (channels.length === 0) {
        logger.warn('没有启用的通知渠道');
        return { success: false, message: '没有启用的通知渠道' };
    }

    logger.info({ channels: channels.map(c => c.name) }, '开始发送通知');

    const results: { channel: string; result: NotifyResult }[] = [];

    for (const channel of channels) {
        try {
            const result = await channel.send(text, desp, params);
            results.push({ channel: channel.name, result });

            if (result.success) {
                logger.info({ channel: channel.name }, '通知发送成功');
            } else {
                logger.warn({ channel: channel.name, message: result.message }, '通知发送失败');
            }
        } catch (err) {
            logger.error({ err, channel: channel.name }, '通知发送异常');
            results.push({
                channel: channel.name,
                result: { success: false, error: err as Error }
            });
        }
    }

    const successCount = results.filter(r => r.result.success).length;
    logger.info({ total: channels.length, success: successCount }, '通知发送完成');

    return {
        success: successCount > 0,
        message: `成功: ${successCount}/${channels.length}`
    };
}

/**
 * 获取所有已注册的渠道
 */
export function getChannels() {
    return registry.getAll();
}

/**
 * 获取已启用的渠道
 */
export function getEnabledChannels() {
    return registry.getEnabled();
}

/**
 * 启用指定渠道
 */
export function enableChannel(name: string) {
    registry.enable(name);
}

/**
 * 禁用指定渠道
 */
export function disableChannel(name: string) {
    registry.disable(name);
}

// 默认导出
export default sendNotify;
