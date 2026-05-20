/**
 * 运行环境工具
 * 集中管理统一网关模式检测，所有模式判断集中在此文件
 *
 * 全部架构统一使用网关模式
 */

export const GATEWAY_PREFIX = '/app/logmanager'

export type AppMode = 'gateway' | 'direct'

/**
 * 检测当前运行模式
 * - gateway: 统一网关模式（路径以 /app/logmanager 开头）
 * - direct: 其他直连模式
 */
export function getMode(): AppMode {
  if (window.location.pathname.startsWith(GATEWAY_PREFIX)) return 'gateway'
  return 'direct'
}

/** 是否运行在统一网关模式 */
export function isGatewayMode(): boolean {
  return getMode() === 'gateway'
}

/**
 * 获取 API 请求基础路径
 * HTTP API 统一前置此路径
 */
export function getApiBase(): string {
  const mode = getMode()
  if (mode === 'gateway') return GATEWAY_PREFIX
  return ''
}

/**
 * 构建 WebSocket URL
 * 根据当前模式自动选择连接方式：
 * - gateway: 通过统一网关（同域名端口 + 前缀）
 * - direct: 同域名端口直连
 *
 * @param apiPath - WebSocket API 路径，如 '/api/logs/stream'
 * @param token - 会话 token，用于直连模式认证
 */
export function buildWsUrl(apiPath: string, token: string): string {
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  const tokenParam = token ? `?token=${encodeURIComponent(token)}` : ''
  const mode = getMode()

  if (mode === 'gateway') {
    return `${protocol}//${window.location.host}${GATEWAY_PREFIX}${apiPath}${tokenParam}`
  }
  return `${protocol}//${window.location.host}${apiPath}${tokenParam}`
}
