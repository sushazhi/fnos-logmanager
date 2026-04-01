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
            signal: timeout ? AbortSignal.timeout(timeout) : undefined
        });

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
                } catch { }
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
                } catch { }
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
