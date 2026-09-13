/**
 * 通知渠道扫码相关的共享工具。
 *
 * 这些约束用于把「服务端返回的任意字符串」收敛为「可以安全放进 <img src> 的
 * 二维码图片」，以及避免过期的异步响应覆盖新一轮扫码的状态。
 */

/** 只接受内联的 PNG/WebP base64 图片，拒绝一切外部 URL。 */
const QR_DATA_URL = /^data:image\/(?:png|webp);base64,[a-z\d+/]+={0,2}$/i

/** 单张二维码图片的字节上限，防止异常响应撑爆内存或 DOM。 */
const MAX_QR_SOURCE_LENGTH = 2 * 1024 * 1024

/**
 * 校验二维码图片来源。
 *
 * 渠道接口的返回结构不完全一致，历史实现直接把后端返回值拼进 `<img src>`，
 * 这会让 `javascript:`、任意 http(s) 地址或超长字符串直接进入 DOM。这里强制
 * 只允许内联图片数据，其余一律视为无效。
 *
 * @param value 待校验的字符串
 * @returns 合法的 data URL，或 undefined
 */
export function safeQrSource(value: unknown): string | undefined {
  if (typeof value !== 'string' || value.length === 0) return undefined
  if (value.length > MAX_QR_SOURCE_LENGTH) return undefined
  return QR_DATA_URL.test(value) ? value : undefined
}

/**
 * 把 base64 载荷包装为内联图片 data URL，并做同样的校验。
 *
 * @param base64 不含前缀的 base64 字符串
 * @param mime 图片类型，默认 png
 */
export function qrDataUrlFromBase64(base64: unknown, mime: 'png' | 'webp' = 'png'): string | undefined {
  if (typeof base64 !== 'string' || base64.length === 0) return undefined
  return safeQrSource(`data:image/${mime};base64,${base64}`)
}

/**
 * 把毫秒格式化为 `mm:ss`，用于二维码有效期倒计时。
 */
export function formatRemaining(milliseconds: number): string {
  const totalSeconds = Math.max(0, Math.ceil((Number(milliseconds) || 0) / 1000))
  const minutes = Math.floor(totalSeconds / 60)
  const seconds = totalSeconds % 60
  return `${String(minutes).padStart(2, '0')}:${String(seconds).padStart(2, '0')}`
}

/**
 * 生成一个单调递增的代际令牌。
 *
 * 扫码流程里存在真实的竞态：用户点击「刷新二维码」时，上一轮已经发出的请求
 * 无法取消，它的响应回来后会写入新一轮的状态。每次开始新一轮扫码都递增代际，
 * 响应返回时比对代际，不一致就丢弃——这样旧响应永远不会覆盖新状态。
 */
export function createGenerationGuard(): {
  begin: () => number
  isCurrent: (generation: number) => boolean
  invalidate: () => void
} {
  let current = 0
  return {
    begin: () => {
      current += 1
      return current
    },
    isCurrent: (generation: number) => generation === current,
    invalidate: () => {
      current += 1
    },
  }
}
