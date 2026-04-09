/**
 * 大小阈值解析工具
 * 提取自 routes/logs.ts 中重复 3 处的解析逻辑
 */

/**
 * 大小单位乘数映射
 */
const SIZE_MULTIPLIERS: Record<string, number> = {
    '': 1,
    'K': 1024,
    'M': 1024 * 1024,
    'G': 1024 * 1024 * 1024,
    'T': 1024 * 1024 * 1024 * 1024
};

/**
 * 解析大小阈值字符串为字节数
 * 支持格式: "10", "10K", "10M", "10G", "10T"（不区分大小写）
 * 
 * @param threshold 阈值字符串，如 "10M"
 * @param defaultBytes 解析失败时的默认值，默认 10MB
 * @returns 字节数
 */
export function parseSizeThreshold(threshold: string, defaultBytes: number = 10 * 1024 * 1024): number {
    const match = threshold.match(/^([0-9]+)([KMGT]?)$/i);
    if (match) {
        const num = parseInt(match[1]);
        const unit = (match[2] || '').toUpperCase();
        return num * (SIZE_MULTIPLIERS[unit] || 1);
    }
    return defaultBytes;
}
