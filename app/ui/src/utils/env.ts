/**
 * 运行环境工具
 * 集中管理统一网关模式检测，所有模式判断集中在此文件
 *
 * 迁移到统一网关时：只需修改 getMode() 的返回值逻辑
 * - 当前：自动检测 gateway / cgi / direct 三种模式
 * - 迁移后：直接 return 'gateway'
 */

export const GATEWAY_PREFIX = '/app/logmanager'

export type AppMode = 'gateway' | 'cgi' | 'direct'

/**
 * 检测当前运行模式
 * - gateway: x86 统一网关模式（路径以 /app/logmanager 开头）
 * - cgi: ARM CGI 代理模式（路径以 /cgi/ 开头）
 * - direct: 其他直连模式
 */
export function getMode(): AppMode {
  if (window.location.pathname.startsWith(GATEWAY_PREFIX)) return 'gateway'
  if (window.location.pathname.startsWith('/cgi/')) return 'cgi'
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
  if (mode === 'cgi') return '/cgi/ThirdParty/logmanager/router.cgi'
  return ''
}

/**
 * 构建 WebSocket URL
 * 根据当前模式自动选择连接方式：
 * - gateway: 通过统一网关（同域名端口 + 前缀）
 * - cgi: 直连后端 8090 端口（CGI 不支持 WS 代理）
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
    // 通过统一网关：使用页面同域名端口，网关处理 TLS 终止
    return `${protocol}//${window.location.host}${GATEWAY_PREFIX}${apiPath}${tokenParam}`
  }
  if (mode === 'cgi') {
    // ARM 直连模式：CGI 无法代理 WebSocket，直连后端端口
    return `${protocol}//${window.location.hostname}:8090${apiPath}${tokenParam}`
  }
  // 兜底：同域名端口直连
  return `${protocol}//${window.location.host}${apiPath}${tokenParam}`
}
