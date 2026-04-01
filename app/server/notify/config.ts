/**
 * 通知配置管理
 */
import { ChannelConfig } from './types';

/**
 * 默认通知配置
 */
const defaultConfig: ChannelConfig = {
    BARK_PUSH: '',
    BARK_ARCHIVE: '',
    BARK_GROUP: '',
    BARK_SOUND: '',
    BARK_ICON: '',
    BARK_LEVEL: '',
    BARK_URL: '',

    DD_BOT_SECRET: '',
    DD_BOT_TOKEN: '',

    FSKEY: '',
    FSSECRET: '',
    FEISHU_APP_ID: '',
    FEISHU_APP_SECRET: '',
    FEISHU_USER_ID: '',

    GOTIFY_URL: '',
    GOTIFY_TOKEN: '',
    GOTIFY_PRIORITY: 0,

    IGOT_PUSH_KEY: '',

    PUSH_KEY: '',

    DEER_KEY: '',
    DEER_URL: '',

    CHAT_URL: '',
    CHAT_TOKEN: '',

    PUSH_PLUS_TOKEN: '',
    PUSH_PLUS_USER: '',
    PUSH_PLUS_TEMPLATE: 'html',
    PUSH_PLUS_CHANNEL: 'wechat',
    PUSH_PLUS_WEBHOOK: '',
    PUSH_PLUS_CALLBACKURL: '',
    PUSH_PLUS_TO: '',

    WE_PLUS_BOT_TOKEN: '',
    WE_PLUS_BOT_RECEIVER: '',
    WE_PLUS_BOT_VERSION: 'pro',

    QMSG_KEY: '',
    QMSG_TYPE: '',

    QYWX_ORIGIN: 'https://qyapi.weixin.qq.com',
    QYWX_AM: '',
    QYWX_KEY: '',

    WECHAT_BOT_ID: '',
    WECHAT_BOT_SECRET: '',
    WECHAT_BOT_CHAT_ID: '',
    WECHAT_BOT_WS_URL: 'wss://openws.work.weixin.qq.com',

    TG_BOT_TOKEN: '',
    TG_USER_ID: '',
    TG_API_HOST: 'https://api.telegram.org',
    TG_PROXY_AUTH: '',
    TG_PROXY_HOST: '',
    TG_PROXY_PORT: '',

    AIBOTK_KEY: '',
    AIBOTK_TYPE: '',
    AIBOTK_NAME: '',

    PUSHME_KEY: '',

    WEBHOOK_URL: '',
    WEBHOOK_BODY: '',
    WEBHOOK_HEADERS: '',
    WEBHOOK_METHOD: '',
    WEBHOOK_CONTENT_TYPE: '',

    NTFY_URL: '',
    NTFY_TOPIC: '',
    NTFY_PRIORITY: '3',
    NTFY_TOKEN: '',
    NTFY_USERNAME: '',
    NTFY_PASSWORD: '',
    NTFY_ACTIONS: '',

    WXPUSHER_APP_TOKEN: '',
    WXPUSHER_TOPIC_IDS: '',
    WXPUSHER_UIDS: '',

    QQ_APP_ID: '',
    QQ_APP_SECRET: '',
    QQ_OPENID: '',
    QQ_GROUP_OPENID: ''
};

/**
 * 推送配置（从环境变量加载）
 */
export const pushConfig: ChannelConfig = { ...defaultConfig };

// 从环境变量加载配置
for (const key in pushConfig) {
    const v = process.env[key];
    if (v) {
        (pushConfig as Record<string, unknown>)[key] = v;
    }
}

/**
 * 获取配置值
 */
export function getConfig<K extends keyof ChannelConfig>(key: K): ChannelConfig[K] {
    return pushConfig[key];
}

/**
 * 设置配置值
 */
export function setConfig<K extends keyof ChannelConfig>(key: K, value: ChannelConfig[K]): void {
    pushConfig[key] = value;
}

/**
 * 检查配置是否有效
 */
export function hasConfig(...keys: (keyof ChannelConfig)[]): boolean {
    return keys.every(key => {
        const value = pushConfig[key];
        return value !== undefined && value !== '' && value !== null;
    });
}

export default pushConfig;
