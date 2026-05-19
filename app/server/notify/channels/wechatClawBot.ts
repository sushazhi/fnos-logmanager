/**
 * 微信 ClawBot 推送渠道（基于微信 iLink 协议）
 * 参考: https://github.com/jxxghp/MoviePilot/blob/v2/app/modules/wechatclawbot/wechatclawbot.py
 * 文档: https://ilinkai.weixin.qq.com
 *
 * 微信个人号机器人通知渠道，通过 iLink API 发送文本通知消息。
 * 使用流程：
 * 1. 调用 getQRCode() 获取二维码
 * 2. 用户微信扫描二维码完成授权
 * 3. 轮询 checkQRCodeStatus() 获取 BotToken 和 AccountID
 * 4. BotToken 会自动保存到配置，用户可发送通知
 */
import crypto from 'crypto';
import Logger from '../../utils/logger';
import { NotifyChannel, NotifyResult, NotifyParams } from '../types';
import { httpClient, isPrivateUrl } from '../httpClient';
import { hasConfig, getConfig, setConfig } from '../config';

const logger = Logger.child({ channel: 'wechat-claw-bot' });

const DEFAULT_BASE_URL = 'https://ilinkai.weixin.qq.com';

// ==================== 捕获的凭证（内存缓存） ====================
let capturedBotToken: string | null = null;
let capturedAccountId: string | null = null;
let capturedBaseUrl: string | null = null;

/**
 * 最近互动过的用户 ID（当未配置 toUser 时使用）
 */
let lastInteractedUser: string | null = null;

/**
 * 获取已捕获的凭证
 */
export function getCapturedCredentials(): { botToken: string | null; accountId: string | null; baseUrl: string | null } {
    return {
        botToken: capturedBotToken,
        accountId: capturedAccountId,
        baseUrl: capturedBaseUrl
    };
}

/**
 * 记录最近互动过的用户 ID
 */
export function setLastInteractedUser(userId: string): void {
    if (userId) {
        lastInteractedUser = userId;
    }
}

/**
 * 获取最近互动过的用户 ID
 */
export function getLastInteractedUser(): string | null {
    return lastInteractedUser;
}

// ==================== iLink 客户端 ====================

class ILinkClient {
    private baseUrl: string;
    private botToken: string | null = null;
    private accountId: string | null = null;
    private syncBuf: string | null = null;
    private readonly timeout: number;

    constructor(baseUrl: string = DEFAULT_BASE_URL, botToken?: string, accountId?: string) {
        this.baseUrl = baseUrl.replace(/\/+$/, '');
        this.botToken = botToken || null;
        this.accountId = accountId || null;
        this.syncBuf = null;
        this.timeout = 20000;
    }

    /**
     * 规范化二维码展示字段，兼容图片 URL、data URL 与裸 base64
     * 参考 MoviePilot ILinkClient._normalize_qrcode_url
     */
    static normalizeQRCodeUrl(value: unknown): string | null {
        if (value === null || value === undefined) return null;
        const raw = String(value).trim();
        if (!raw) return null;
        const lowered = raw.toLowerCase();
        if (lowered.startsWith('data:image/')) return raw;
        if (lowered.startsWith('//')) return `https:${raw}`;
        // 裸 base64：长度 >= 128 且只含 base64 合法字符
        if (raw.length >= 128 && /^[A-Za-z0-9+/=_-]+$/.test(raw)) {
            return `data:image/png;base64,${raw}`;
        }
        return raw;
    }

    /**
     * 设置登录凭证
     */
    setCredentials(botToken: string, accountId?: string, syncBuf?: string): void {
        this.botToken = botToken;
        if (accountId) this.accountId = accountId;
        if (syncBuf !== undefined) this.syncBuf = syncBuf;
    }

    /**
     * 构建请求头
     */
    private buildHeaders(authRequired: boolean = true): Record<string, string> {
        const headers: Record<string, string> = {
            'Content-Type': 'application/json',
            'Accept': 'application/json, text/plain, */*',
            'User-Agent': 'LogManager-WechatClawBot/1.0',
        };
        if (authRequired && this.botToken) {
            headers['AuthorizationType'] = 'ilink_bot_token';
            headers['Authorization'] = `Bearer ${this.botToken}`;
            headers['X-WECHAT-UIN'] = Buffer.from(
                String(Math.floor(Math.random() * 0xFFFFFFFF))
            ).toString('base64');
        }
        return headers;
    }

    /**
     * 尝试解析响应 JSON
     */
    private parseJson(resp: { body: string }): Record<string, unknown> {
        try {
            return JSON.parse(resp.body);
        } catch {
            return {};
        }
    }

    /**
     * 递归查找字段值（支持嵌套）
     */
    private findFirstValue(data: unknown, keys: string[], maxDepth: number = 5): unknown {
        if (maxDepth < 0 || data === null || data === undefined) return undefined;
        if (typeof data !== 'object') return undefined;

        if (Array.isArray(data)) {
            for (const item of data) {
                const found = this.findFirstValue(item, keys, maxDepth - 1);
                if (found !== undefined && found !== null && found !== '') return found;
            }
            return undefined;
        }

        const obj = data as Record<string, unknown>;
        for (const key of keys) {
            if (key in obj) {
                const val = obj[key];
                if (val !== undefined && val !== null && val !== '') return val;
            }
        }
        for (const [, value] of Object.entries(obj)) {
            if (typeof value === 'object') {
                const found = this.findFirstValue(value, keys, maxDepth - 1);
                if (found !== undefined && found !== null && found !== '') return found;
            }
        }
        return undefined;
    }

    /**
     * 检查响应是否成功
     */
    private isOk(payload: Record<string, unknown>): boolean {
        if (!payload || Object.keys(payload).length === 0) return false;
        const code = payload.errcode ?? payload.code ?? payload.ret;
        if (code !== undefined) {
            try { return Number(code) === 0; } catch { /* fall through */ }
        }
        const err = payload.errmsg || payload.message || payload.error || '';
        if (typeof err === 'string' && err.toLowerCase() !== 'ok' && err.toLowerCase() !== 'success' && err !== '') return false;
        return true;
    }

    /**
     * 获取二维码登录信息
     * POST {base_url}/ilink/bot/get_bot_qrcode?bot_type=3
     */
    async getQRCode(): Promise<{
        success: boolean;
        qrcode?: string;
        qrcodeUrl?: string;
        message?: string;
    }> {
        const url = `${this.baseUrl}/ilink/bot/get_bot_qrcode?bot_type=3`;

        try {
            const resp = await httpClient.get(url, {
                headers: this.buildHeaders(false),
                timeout: this.timeout
            });

            const payload = this.parseJson(resp);

            if (!payload || Object.keys(payload).length === 0) {
                return { success: false, message: '获取二维码失败：空响应' };
            }

            const data = (payload.data || payload.result || payload) as Record<string, unknown>;

            const qrcode = String(data.qrcode || data.qr_code || data.qrcode_id || data.ticket || '');
            const qrcodeUrl = ILinkClient.normalizeQRCodeUrl(
                data.qrcode_url || data.url || data.qrcodeUrl || data.qr_url ||
                data.qrcode_img_content || data.qrcode_img_url || data.qr_img || null
            ) || (qrcode ? `https://liteapp.weixin.qq.com/q/7GiQu1?qrcode=${qrcode}&bot_type=3` : '');

            if (this.isOk(payload) && (qrcode || qrcodeUrl)) {
                return { success: true, qrcode, qrcodeUrl, message: '获取二维码成功' };
            }

            return {
                success: false,
                message: (payload.errmsg as string) || (payload.message as string) || '获取二维码失败'
            };
        } catch (err) {
            const error = err as Error;
            logger.error({ err: error.message }, '微信 ClawBot 获取二维码异常');
            return { success: false, message: `获取二维码异常: ${error.message}` };
        }
    }

    /**
     * 查询二维码扫描状态
     * GET {base_url}/ilink/bot/get_qrcode_status?qrcode=xxx
     */
    async getQRCodeStatus(qrcode: string): Promise<{
        success: boolean;
        status: string;
        token?: string;
        accountId?: string;
        scannerId?: string;
        message?: string;
    }> {
        const fullUrl = `${this.baseUrl}/ilink/bot/get_qrcode_status?qrcode=${encodeURIComponent(qrcode)}`;

        try {
            let payload: Record<string, unknown> = {};

            // 方式1: 使用 URL 参数形式
            try {
                const resp = await httpClient.get(fullUrl, {
                    headers: this.buildHeaders(false),
                    timeout: this.timeout
                });
                payload = this.parseJson(resp);
            } catch {
                // 方式2: 无参数形式重试
                const retryResp = await httpClient.get(`${this.baseUrl}/ilink/bot/get_qrcode_status`, {
                    headers: this.buildHeaders(false),
                    timeout: this.timeout
                });
                payload = this.parseJson(retryResp);
            }

            if (!payload || Object.keys(payload).length === 0) {
                return { success: false, status: 'waiting', message: '二维码状态接口返回空响应' };
            }

            const data = (payload.data || payload.result || payload) as Record<string, unknown>;

            const token = String(
                data.bot_token || data.token || data.access_token ||
                this.findFirstValue(data, ['bot_token', 'access_token', 'token', 'jwt', 'auth_token']) || ''
            );
            const accountId = String(
                data.account_id || data.ilink_bot_id || data.wxid || data.uid || data.user_id ||
                this.findFirstValue(data, ['account_id', 'ilink_bot_id', 'wxid', 'uid', 'user_id', 'from_user']) || ''
            );
            const newBaseUrl = String(
                data.baseurl || data.base_url || payload.baseurl || payload.base_url || ''
            );

            // 扫码用户的 ID（微信用户 ID；注意 account_id 是机器人自身 ID，不要混淆）
            const scannerId = String(
                data.wxid || data.from_user || data.uid || data.user_id || data.from_uid ||
                this.findFirstValue(data, ['wxid', 'from_user', 'from_user_id', 'from_uid', 'uid', 'user_id']) || ''
            );

            const status = String(
                data.status || data.state || payload.status || payload.state ||
                this.findFirstValue(data, ['status', 'state', 'scan_status']) || 'waiting'
            ).toLowerCase();

            // 登录成功，保存凭证并记录扫码用户为最近互动用户
            if ((status === 'scanned' || status === 'confirmed' || status === 'success' || status === 'ok') && token) {
                this.botToken = token;
                capturedBotToken = token;
                if (accountId) {
                    this.accountId = accountId;
                    capturedAccountId = accountId;
                }
                if (newBaseUrl) {
                    this.baseUrl = newBaseUrl.replace(/\/+$/, '');
                    capturedBaseUrl = this.baseUrl;
                }
                // 扫码用户自动设为最近互动用户
                if (scannerId) {
                    lastInteractedUser = scannerId;
                }

                logger.info({ accountId, baseUrl: newBaseUrl }, '微信 ClawBot 扫码登录成功，已获取凭证');
            }

            return {
                success: this.isOk(payload),
                status,
                token: token || undefined,
                accountId: accountId || undefined,
                scannerId: scannerId || undefined,
                message: (payload.errmsg as string) || (payload.message as string) || undefined
            };
        } catch (err) {
            const error = err as Error;
            logger.error({ err: error.message }, '微信 ClawBot 查询二维码状态异常');
            return { success: false, status: 'waiting', message: `查询异常: ${error.message}` };
        }
    }

    /**
     * 轮询获取未读消息（用于获取扫码用户的 ID）
     * 参考 MoviePilot ILinkClient.get_updates
     */
    async getUpdates(): Promise<{ success: boolean; messages: Array<{ userId: string; text?: string }>; syncBuf?: string; message?: string }> {
        if (!this.botToken) {
            return { success: false, messages: [], message: 'BotToken 未配置' };
        }

        const url = `${this.baseUrl}/ilink/bot/getupdates`;
        const body: Record<string, unknown> = { sync_buf: this.syncBuf || '' };

        try {
            const resp = await httpClient.post(url, {
                json: body,
                headers: this.buildHeaders(true),
                timeout: 20000
            });
            const payload = this.parseJson(resp);

            if (resp.statusCode < 200 || resp.statusCode >= 300) {
                return { success: false, messages: [], message: `HTTP ${resp.statusCode}` };
            }

            // 更新 sync_buf 游标
            const newSyncBuf = this.findFirstValue(payload, ['sync_buf', 'syncBuf', 'sync_buf_id']) as string | undefined;
            if (newSyncBuf !== undefined) {
                this.syncBuf = newSyncBuf;
            }

            // iLink API 返回格式: { "msgs": [...], "sync_buf": "..." }
            const data = (payload as any).data || (payload as any).result || payload;
            let items: unknown[] = [];

            // 按优先级搜索消息列表字段
            const msgKeys = ['msgs', 'messages', 'msg_list', 'message_list', 'items', 'list', 'data', 'records', 'events', 'updates', 'messageList', 'msgList'];
            for (const key of msgKeys) {
                const field = (data as any)[key];
                if (Array.isArray(field) && field.length > 0) {
                    items = field;
                    break;
                }
            }
            // 递归查找
            if (items.length === 0) {
                const found = this.findFirstValue(data, msgKeys);
                if (Array.isArray(found)) items = found;
            }
            // 顶层直接是数组
            if (items.length === 0 && Array.isArray(data)) {
                items = data;
            }

            if (items.length === 0) {
                return { success: true, messages: [], syncBuf: this.syncBuf || undefined };
            }

            // 解析每条消息提取用户 ID
            const messages: Array<{ userId: string; text?: string }> = [];
            for (const item of items) {
                const obj = (typeof item === 'object' && item !== null) ? item as Record<string, unknown> : {};
                // 字段直接在消息顶层（from_user_id），也可能在嵌套 msg/message 中
                const userId = String(
                    (obj as any).from_user_id || (obj as any).fromUserId
                    || (obj as any).from_user || (obj as any).fromUser
                    || (obj as any).sender || (obj as any).userid || (obj as any).userId
                    || (obj as any).from_uid
                    || ''
                );
                if (!userId) continue;
                // 文本可能在 item_list[].text_item.text 或顶层 text/content
                let text = '';
                const itemList = (obj as any).item_list || (obj as any).items || (obj as any).itemList;
                if (Array.isArray(itemList)) {
                    for (const it of itemList) {
                        const ti = (it as any).text_item || (it as any).textItem;
                        if (ti && (ti as any).text) { text = (ti as any).text; break; }
                    }
                }
                if (!text) {
                    text = String(
                        (obj as any).text || (obj as any).content || (obj as any).msg || (obj as any).message || ''
                    );
                }
                messages.push({ userId, text: text.substring(0, 200) });
            }

            return {
                success: true,
                messages,
                syncBuf: this.syncBuf || undefined,
            };
        } catch (err) {
            const error = err as Error;
            logger.error({ err: error.message }, '微信 ClawBot getUpdates 异常');
            return { success: false, messages: [], message: `getUpdates 异常: ${error.message}` };
        }
    }

    /**
     * 发送文本消息
     */
    async sendText(toUser: string, text: string): Promise<{ success: boolean; message?: string }> {
        if (!this.botToken) {
            return { success: false, message: 'BotToken 未配置，请先扫码登录' };
        }
        if (!toUser || !text) {
            return { success: false, message: 'toUser 或 text 为空' };
        }

        // 参考 MoviePilot 的协议格式
        // https://github.com/jxxghp/MoviePilot/blob/v2/app/modules/wechatclawbot/wechatclawbot.py
        // from_user 需要补齐 @im.bot 后缀
        const fromUserId = this.accountId
            ? (this.accountId.includes('@') ? this.accountId : `${this.accountId}@im.bot`)
            : '';

        // 协议格式（主格式，MoviePilot 用这个）
        const protocolPayload = {
            base_info: { channel_version: '1.0.2' },
            msg: {
                from_user_id: fromUserId,
                to_user_id: toUser,
                client_id: `mp-${crypto.randomUUID()}`,
                message_type: 2,
                message_state: 2,
                item_list: [{ type: 1, text_item: { text } }]
            }
        };

        // 简单格式候选
        const simplePayloads: Record<string, unknown>[] = [
            { base_info: { channel_version: '1.0.2' }, to_user: toUser, msg_type: 'text', text: { content: text } },
            { to_user: toUser, msg_type: 'text', text: { content: text } },
            { touser: toUser, msgtype: 'text', text: { content: text } },
            { to_user_id: toUser, msg_type: 'text', content: text },
        ];

        // URL 候选（MoviePilot 用这两个）
        const urlCandidates = [
            `${this.baseUrl}/ilink/bot/sendmessage`,
            `${this.baseUrl}/ilink/bot/sendmessage?bot_type=3`,
        ];

        // 用户 ID 候选（保留原始 + 各种后缀变体）
        const userCandidates = [...new Set([
            toUser,
            ...(toUser.includes('@') ? [toUser.split('@')[0]] : []),
            ...(toUser.endsWith('@im.wechat')
                ? [toUser.slice(0, -'@im.wechat'.length)]
                : [`${toUser}@im.wechat`]),
        ])];

        let lastError = '';

        for (const userId of userCandidates) {
            // 按 userId 更新 payload 中的用户字段
            const payloads: Record<string, unknown>[] = [
                // 协议格式 - 更新 to_user_id
                {
                    base_info: { channel_version: '1.0.2' },
                    msg: {
                        from_user_id: fromUserId,
                        to_user_id: userId,
                        client_id: `mp-${crypto.randomUUID()}`,
                        message_type: 2,
                        message_state: 2,
                        item_list: [{ type: 1, text_item: { text } }]
                    }
                },
                // 简单格式 - 更新 to_user
                { base_info: { channel_version: '1.0.2' }, to_user: userId, msg_type: 'text', text: { content: text } },
                { to_user: userId, msg_type: 'text', text: { content: text } },
                { touser: userId, msgtype: 'text', text: { content: text } },
                { to_user_id: userId, msg_type: 'text', content: text },
            ];

            for (const url of urlCandidates) {
                for (let i = 0; i < payloads.length; i++) {
                    try {
                        const resp = await httpClient.post(url, {
                            json: payloads[i],
                            headers: this.buildHeaders(true),
                            timeout: 15000
                        });

                        if (resp.statusCode >= 200 && resp.statusCode < 300) {
                            return { success: true, message: '发送成功' };
                        } else {
                            lastError = `user=${userId}, variant=${i + 1}, http=${resp.statusCode}`;
                        }
                    } catch (err) {
                        const error = err as Error;
                        lastError = `user=${userId}, variant=${i + 1}, ${error.message}`;
                    }
                }
            }
        }

        logger.warn({ lastError }, '微信 ClawBot 所有发送方式均失败');
        return { success: false, message: `发送失败: ${lastError}` };
    }
}

// ==================== 单例客户端管理 ====================

let clientInstance: ILinkClient | null = null;

/**
 * 获取或创建 ILink 客户端
 */
function getClient(): ILinkClient {
    if (!clientInstance) {
        const baseUrl = getConfig('WECHAT_CLAWBOT_BASE_URL') || DEFAULT_BASE_URL;
        const botToken = getConfig('WECHAT_CLAWBOT_BOT_TOKEN') || undefined;
        const accountId = getConfig('WECHAT_CLAWBOT_ACCOUNT_ID') || undefined;
        clientInstance = new ILinkClient(baseUrl, botToken, accountId);
    }
    return clientInstance;
}

// ==================== 公开 API（给路由用） ====================

/**
 * 获取二维码
 */
export async function getQRCode(): Promise<{
    success: boolean;
    qrcode?: string;
    qrcodeUrl?: string;
    message?: string;
}> {
    const client = getClient();
    return client.getQRCode();
}

/**
 * 获取未读消息（获取扫码用户的 ID）
 */
export async function getUpdates(): Promise<{
    success: boolean;
    messages: Array<{ userId: string; text?: string }>;
    message?: string;
}> {
    // 用 captured token 初始化客户端
    const baseUrl = capturedBaseUrl || getConfig('WECHAT_CLAWBOT_BASE_URL') || DEFAULT_BASE_URL;
    const botToken = capturedBotToken || getConfig('WECHAT_CLAWBOT_BOT_TOKEN') || '';
    const accountId = capturedAccountId || getConfig('WECHAT_CLAWBOT_ACCOUNT_ID') || '';
    if (!botToken) {
        return { success: false, messages: [], message: 'BotToken 未配置，请先扫码登录' };
    }
    const client = new ILinkClient(baseUrl, botToken, accountId);
    const result = await client.getUpdates();
    // 如果拿到用户，自动设为最近互动用户
    if (result.success && result.messages.length > 0) {
        const firstUserId = result.messages[0].userId;
        if (firstUserId) {
            lastInteractedUser = firstUserId;
            logger.info({ userId: firstUserId }, '微信 ClawBot 通过 getUpdates 获取到互动用户');
        }
    }
    return result;
}

/**
 * 查询二维码状态
 */
export async function checkQRCodeStatus(qrcode: string): Promise<{
    success: boolean;
    status: string;
    token?: string;
    accountId?: string;
    scannerId?: string;
    message?: string;
}> {
    const client = getClient();
    return client.getQRCodeStatus(qrcode);
}

// ==================== NotifyChannel ====================

export const wechatClawBotChannel: NotifyChannel = {
    name: 'wechat-claw-bot',
    enabled: false,

    async send(text: string, desp: string, params: NotifyParams = {}): Promise<NotifyResult> {
        if (!hasConfig('WECHAT_CLAWBOT_BOT_TOKEN')) {
            return { success: false, message: 'WECHAT_CLAWBOT_BOT_TOKEN 未配置，请先扫码登录' };
        }

        const botToken = getConfig('WECHAT_CLAWBOT_BOT_TOKEN') || '';
        let toUser = getConfig('WECHAT_CLAWBOT_TO_USER') || '';
        const baseUrl = getConfig('WECHAT_CLAWBOT_BASE_URL') || DEFAULT_BASE_URL;
        const accountId = getConfig('WECHAT_CLAWBOT_ACCOUNT_ID') || undefined;

        // 未配置 toUser 时，使用最近互动过的用户
        if (!toUser) {
            if (lastInteractedUser) {
                toUser = lastInteractedUser;
            } else {
                return { success: false, message: 'WECHAT_CLAWBOT_TO_USER 未配置，且无最近互动用户' };
            }
        }

        // SSRF 防护
        if (isPrivateUrl(baseUrl)) {
            return { success: false, message: 'WECHAT_CLAWBOT_BASE_URL 指向内网地址，不允许' };
        }

        // 确保客户端使用最新的凭证
        const client = getClient();
        client.setCredentials(botToken, accountId);

        // 构建消息内容
        const content = desp ? `**${text}**\n\n${desp}` : text;

        try {
            const result = await client.sendText(toUser, content);

            if (result.success) {
                logger.info({ toUser }, '微信 ClawBot 通知发送成功');
                return { success: true, message: result.message };
            } else {
                logger.warn({ message: result.message }, '微信 ClawBot 通知发送失败');
                return { success: false, message: result.message };
            }
        } catch (err) {
            const error = err as Error;
            logger.error({ err: error.message }, '微信 ClawBot 通知发送异常');
            return { success: false, error };
        }
    }
};

if (hasConfig('WECHAT_CLAWBOT_BOT_TOKEN')) {
    wechatClawBotChannel.enabled = true;
}

export default wechatClawBotChannel;
