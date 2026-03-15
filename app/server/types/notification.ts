/**
 * 通知功能类型定义
 */

// 支持的通知渠道类型
export type NotificationChannel =
    | 'bark'          // iOS Bark APP
    | 'dingtalk'      // 钉钉机器人
    | 'feishu'        // 飞书机器人
    | 'wecom'         // 企业微信机器人
    | 'wecom_app'     // 企业微信应用
    | 'wechat_bot'    // 企业微信智能机器人（长连接）
    | 'telegram'      // Telegram 机器人
    | 'serverchan'    // Server酱
    | 'pushplus'      // PushPlus
    | 'webhook'       // 自定义 Webhook
    | 'ntfy'          // Ntfy
    | 'gotify'        // Gotify
    | 'pushdeer'      // PushDeer
    | 'qqbot';        // QQ机器人

// 日志级别类型
export type LogLevel = 'error' | 'warn' | 'info' | 'debug' | 'all';

// 通知规则状态
export type NotificationRuleStatus = 'enabled' | 'disabled';

// 通知渠道配置
export interface NotificationChannelConfig {
    // 通用配置
    channel: NotificationChannel;
    enabled: boolean;
    name: string;

    // Bark 配置
    barkPush?: string;
    barkSound?: string;
    barkGroup?: string;
    barkIcon?: string;
    barkLevel?: string;
    barkArchive?: string;
    barkUrl?: string;

    // 钉钉配置
    ddBotToken?: string;
    ddBotSecret?: string;

    // 飞书配置（自定义机器人）
    fsKey?: string;
    fsSecret?: string;
    // 飞书配置（企业自建应用）
    feishuAppId?: string;
    feishuAppSecret?: string;
    // 飞书用户ID（可选，用于发送给指定用户）
    feishuUserId?: string;

    // 企业微信机器人配置
    qywxKey?: string;
    qywxOrigin?: string;

    // 企业微信应用配置
    qywxAm?: string;

    // Telegram 配置
    tgBotToken?: string;
    tgUserId?: string;
    tgApiHost?: string;
    tgProxyHost?: string;
    tgProxyPort?: string;
    tgProxyAuth?: string;

    // Server酱配置
    pushKey?: string;

    // PushPlus 配置
    pushPlusToken?: string;
    pushPlusUser?: string;
    pushPlusTemplate?: string;
    pushPlusChannel?: string;
    pushPlusWebhook?: string;
    pushPlusCallbackUrl?: string;
    pushPlusTo?: string;

    // Webhook 配置
    webhookUrl?: string;
    webhookMethod?: string;
    webhookHeaders?: string;
    webhookBody?: string;
    webhookContentType?: string;

    // Ntfy 配置
    ntfyUrl?: string;
    ntfyTopic?: string;
    ntfyPriority?: string;
    ntfyToken?: string;
    ntfyUsername?: string;
    ntfyPassword?: string;

    // Gotify 配置
    gotifyUrl?: string;
    gotifyToken?: string;
    gotifyPriority?: number;

    // PushDeer 配置
    deerKey?: string;
    deerUrl?: string;

    // 企业微信智能机器人配置（长连接模式）
    wechatBotId?: string;
    wechatBotSecret?: string;
    wechatBotChatId?: string;
    wechatBotWsUrl?: string;

    // QQ机器人配置
    qqAppId?: string;
    qqAppSecret?: string;
    qqOpenId?: string;
    qqGroupOpenId?: string;
}

// 通知规则
export interface NotificationRule {
    id: string;
    name: string;
    status: NotificationRuleStatus;

    // 监控目标
    appName: string;           // 应用名称，支持通配符 *
    logPaths?: string[];       // 指定日志文件路径（可选）

    // 触发条件
    logLevel: LogLevel;        // 日志级别
    keywords?: string[];       // 关键词匹配（可选，多个关键词为OR关系）
    excludeKeywords?: string[];// 排除关键词（可选，匹配则不通知）
    pattern?: string;          // 正则表达式匹配（可选）

    // 通知配置
    channels: string[];        // 使用的通知渠道ID列表

    // 频率控制
    cooldown: number;          // 冷却时间（秒），同一规则触发后多久内不再重复通知
    maxNotifications: number;  // 每小时最大通知次数

    // 时间配置
    quietHoursStart?: string;  // 静默时段开始（如 "22:00"）
    quietHoursEnd?: string;    // 静默时段结束（如 "08:00"）

    // 元数据
    createdAt: Date;
    updatedAt: Date;
    lastTriggeredAt?: Date;
    triggerCount: number;
}

// 通知历史记录
export interface NotificationHistory {
    id: string;
    ruleId: string;
    ruleName: string;
    channel: NotificationChannel;
    title: string;
    content: string;
    appName: string;
    logPath: string;
    matchedLine: string;
    success: boolean;
    error?: string;
    timestamp: Date;
}

// 通知发送结果
export interface NotificationResult {
    success: boolean;
    channel: NotificationChannel;
    error?: string;
}

// 通知请求
export interface NotificationRequest {
    title: string;
    content: string;
    appName: string;
    logPath: string;
    matchedLine: string;
    rule: NotificationRule;
}

// 日志监控状态
export interface LogMonitorStatus {
    running: boolean;
    watchedFiles: number;
    activeRules: number;
    lastCheckTime: Date;
    errors: string[];
}

// 通知统计
export interface NotificationStats {
    totalSent: number;
    successCount: number;
    failCount: number;
    last24Hours: number;
    byChannel: Record<NotificationChannel, { sent: number; success: number; failed: number }>;
    byRule: Record<string, { sent: number; success: number; failed: number }>;
}

// API 请求/响应类型
export interface CreateNotificationRuleRequest {
    name: string;
    appName: string;
    logPaths?: string[];
    logLevel: LogLevel;
    keywords?: string[];
    excludeKeywords?: string[];
    pattern?: string;
    channels: string[];
    cooldown: number;
    maxNotifications: number;
    quietHoursStart?: string;
    quietHoursEnd?: string;
}

export interface UpdateNotificationRuleRequest extends Partial<CreateNotificationRuleRequest> {
    status?: NotificationRuleStatus;
}

export interface CreateChannelRequest {
    channel: NotificationChannel;
    name: string;
    config: Omit<NotificationChannelConfig, 'channel' | 'enabled' | 'name'>;
}

export interface UpdateChannelRequest extends Partial<CreateChannelRequest> {
    enabled?: boolean;
}

// 通知配置文件结构
export interface NotificationConfigFile {
    version: string;
    channels: NotificationChannelConfig[];
    rules: NotificationRule[];
    settings: {
        enabled: boolean;
        checkInterval: number;     // 检查间隔（毫秒）
        maxHistoryDays: number;    // 历史记录保留天数
        maxHistoryCount: number;   // 最大历史记录数
    };
}
