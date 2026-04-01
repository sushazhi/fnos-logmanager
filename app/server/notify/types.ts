/**
 * 通知系统类型定义
 */

export interface NotifyChannel {
    name: string;
    enabled: boolean;
    send(text: string, desp: string, params?: NotifyParams): Promise<NotifyResult>;
}

export interface NotifyResult {
    success: boolean;
    message?: string;
    error?: Error;
}

export interface NotifyParams {
    [key: string]: unknown;
}

export interface ChannelConfig {
    // Bark
    BARK_PUSH?: string;
    BARK_ARCHIVE?: string;
    BARK_GROUP?: string;
    BARK_SOUND?: string;
    BARK_ICON?: string;
    BARK_LEVEL?: string;
    BARK_URL?: string;

    // 钉钉
    DD_BOT_SECRET?: string;
    DD_BOT_TOKEN?: string;

    // 飞书
    FSKEY?: string;
    FSSECRET?: string;
    FEISHU_APP_ID?: string;
    FEISHU_APP_SECRET?: string;
    FEISHU_USER_ID?: string;

    // Gotify
    GOTIFY_URL?: string;
    GOTIFY_TOKEN?: string;
    GOTIFY_PRIORITY?: number;

    // iGot
    IGOT_PUSH_KEY?: string;

    // Server酱
    PUSH_KEY?: string;

    // PushDeer
    DEER_KEY?: string;
    DEER_URL?: string;

    // Synology Chat
    CHAT_URL?: string;
    CHAT_TOKEN?: string;

    // PushPlus
    PUSH_PLUS_TOKEN?: string;
    PUSH_PLUS_USER?: string;
    PUSH_PLUS_TEMPLATE?: string;
    PUSH_PLUS_CHANNEL?: string;
    PUSH_PLUS_WEBHOOK?: string;
    PUSH_PLUS_CALLBACKURL?: string;
    PUSH_PLUS_TO?: string;

    // 微加机器人
    WE_PLUS_BOT_TOKEN?: string;
    WE_PLUS_BOT_RECEIVER?: string;
    WE_PLUS_BOT_VERSION?: string;

    // QMSG
    QMSG_KEY?: string;
    QMSG_TYPE?: string;

    // 企业微信
    QYWX_ORIGIN?: string;
    QYWX_AM?: string;
    QYWX_KEY?: string;

    // 企业微信智能机器人
    WECHAT_BOT_ID?: string;
    WECHAT_BOT_SECRET?: string;
    WECHAT_BOT_CHAT_ID?: string;
    WECHAT_BOT_WS_URL?: string;

    // Telegram
    TG_BOT_TOKEN?: string;
    TG_USER_ID?: string;
    TG_API_HOST?: string;
    TG_PROXY_AUTH?: string;
    TG_PROXY_HOST?: string;
    TG_PROXY_PORT?: string;

    // 智能微秘书
    AIBOTK_KEY?: string;
    AIBOTK_TYPE?: string;
    AIBOTK_NAME?: string;

    // PushMe
    PUSHME_KEY?: string;

    // Webhook
    WEBHOOK_URL?: string;
    WEBHOOK_BODY?: string;
    WEBHOOK_HEADERS?: string;
    WEBHOOK_METHOD?: string;
    WEBHOOK_CONTENT_TYPE?: string;

    // Ntfy
    NTFY_URL?: string;
    NTFY_TOPIC?: string;
    NTFY_PRIORITY?: string;
    NTFY_TOKEN?: string;
    NTFY_USERNAME?: string;
    NTFY_PASSWORD?: string;
    NTFY_ACTIONS?: string;

    // WxPusher
    WXPUSHER_APP_TOKEN?: string;
    WXPUSHER_TOPIC_IDS?: string;
    WXPUSHER_UIDS?: string;

    // QQ机器人
    QQ_APP_ID?: string;
    QQ_APP_SECRET?: string;
    QQ_OPENID?: string;
    QQ_GROUP_OPENID?: string;
}

export interface HttpRequestOptions {
    method?: 'GET' | 'POST' | 'PUT' | 'DELETE';
    headers?: Record<string, string>;
    json?: unknown;
    form?: unknown;
    body?: string | Buffer;
    timeout?: number;
}

export interface HttpResponse {
    statusCode: number;
    body: string;
    headers: Record<string, string>;
}
