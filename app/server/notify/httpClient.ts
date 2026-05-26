/**
 * HTTP 客户端封装
 */
import undici from 'undici';
import Logger from '../utils/logger';
import { HttpRequestOptions, HttpResponse } from './types';

const { request: undiciRequest } = undici;

const logger = Logger.child({ module: 'HttpClient' });

const DEFAULT_TIMEOUT = 15000;

/**
 * 发送 HTTP 请求
 */
export async function httpRequest(
    url: string,
    options: HttpRequestOptions = {}
): Promise<HttpResponse> {
    const { json, form, body, headers = {}, timeout = DEFAULT_TIMEOUT, method = 'GET' } = options;

    const finalHeaders: Record<string, string> = { ...headers };
    let finalBody: string | Buffer | undefined = body;

    if (json) {
        finalHeaders['content-type'] = 'application/json';
        finalBody = JSON.stringify(json);
    } else if (form) {
        finalBody = JSON.stringify(form);
        finalHeaders['content-type'] = 'application/x-www-form-urlencoded';
    }

    try {
        const response = await undiciRequest(url, {
            method,
            headers: finalHeaders,
            body: finalBody,
            signal: timeout ? AbortSignal.timeout(timeout) : undefined,
        });

        // 检查重定向状态码，防止SSRF
        if (response.statusCode >= 300 && response.statusCode < 400) {
            const location = response.headers?.location || response.headers?.Location || '(unknown)';
            throw new Error(`不允许跟随重定向: ${response.statusCode} -> ${location}`);
        }

        const responseBody = await response.body.text();

        // 转换 headers 为普通对象
        const responseHeaders: Record<string, string> = {};
        for (const [key, value] of Object.entries(response.headers)) {
            if (typeof value === 'string') {
                responseHeaders[key] = value;
            } else if (Array.isArray(value)) {
                responseHeaders[key] = value.join(', ');
            }
        }

        return {
            statusCode: response.statusCode,
            body: responseBody,
            headers: responseHeaders
        };
    } catch (err) {
        logger.error({ err, url, method }, 'HTTP request failed');
        throw err;
    }
}

/**
 * HTTP 客户端对象
 */
export const httpClient = {
    request: httpRequest,
    post: (url: string, options?: HttpRequestOptions) => httpRequest(url, { ...options, method: 'POST' }),
    get: (url: string, options?: HttpRequestOptions) => httpRequest(url, { ...options, method: 'GET' })
};

/**
 * 兼容旧版的 $ 对象
 */
export const $ = {
    post: (params: { url: string; [key: string]: unknown }, callback: (err: Error | null, res: unknown, body: unknown) => void) => {
        const { url, ...others } = params;
        httpClient.post(url, others as HttpRequestOptions).then(
            async (res) => {
                let body = res.body;
                try {
                    body = JSON.parse(body);
                } catch {
                    // 非 JSON 响应，使用原始字符串
                }
                callback(null, res, body);
            },
            (err) => {
                callback(err, null, null);
            }
        );
    },
    get: (params: { url: string; [key: string]: unknown }, callback: (err: Error | null, res: unknown, body: unknown) => void) => {
        const { url, ...others } = params;
        httpClient.get(url, others as HttpRequestOptions).then(
            async (res) => {
                let body = res.body;
                try {
                    body = JSON.parse(body);
                } catch {
                    // 非 JSON 响应，使用原始字符串
                }
                callback(null, res, body);
            },
            (err) => {
                callback(err, null, null);
            }
        );
    },
    logErr: (err: unknown) => {
        logger.error({ err }, 'Notification error');
    }
};

export default httpClient;

export function isPrivateUrl(urlStr: string): boolean {
    try {
        const url = new URL(urlStr);
        let hostname = url.hostname;

        // 处理 IPv4-mapped IPv6 地址 (::ffff:x.x.x.x)
        if (hostname.startsWith('::ffff:')) {
            hostname = hostname.slice(7);
        }

        if (hostname === 'localhost' || hostname === '127.0.0.1' || hostname === '::1' || hostname === '0.0.0.0') return true;
        if (hostname.startsWith('192.168.') || hostname.startsWith('10.') || hostname.startsWith('169.254.')) return true;
        if (/^172\.(1[6-9]|2\d|3[01])\./.test(hostname)) return true;
        // fc00::/7 覆盖 fc* 和 fd* 两个前缀
        if (hostname.startsWith('fc') || hostname.startsWith('fd') || hostname.startsWith('fe80')) return true;
        if (url.protocol !== 'http:' && url.protocol !== 'https:') return true;
        return false;
    } catch {
        return true;
    }
}
