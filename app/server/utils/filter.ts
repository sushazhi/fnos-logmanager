/**
 * 敏感信息过滤工具
 * 提取自 routes/logs.ts 和 routes/docker.ts 的重复逻辑
 */

import config from './config';

let FILTER_SENSITIVE = process.env.FILTER_SENSITIVE !== 'false';

/**
 * 过滤内容中的敏感信息
 * 将匹配敏感模式的内容替换为 [FILTERED]
 */
export function filterSensitiveInfo(content: string): string {
    if (!FILTER_SENSITIVE) return content;
    if (!content || typeof content !== 'string') return content;
    let filtered = content;
    for (const pattern of config.sensitivePatterns) {
        filtered = filtered.replace(pattern, '[FILTERED]');
    }
    return filtered;
}

/**
 * 获取敏感信息过滤状态
 */
export function isFilterEnabled(): boolean {
    return FILTER_SENSITIVE;
}

/**
 * 设置敏感信息过滤状态
 */
export function setFilterEnabled(enabled: boolean): void {
    process.env.FILTER_SENSITIVE = enabled ? 'true' : 'false';
    FILTER_SENSITIVE = enabled;
}
