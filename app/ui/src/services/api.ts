import { 
  NetworkError, 
  AuthenticationError, 
  ValidationError, 
  ServerError,
  withRetry,
  RequestCanceller,
  RequestDeduper,
  filterSensitiveInfo
} from '../utils/request'
import { getApiBase } from '../utils/env'

export const API_BASE = getApiBase()

// CSRF Token 存储说明：
// 使用 sessionStorage 存储 CSRF Token，相比 localStorage 更安全：
// 1. 数据仅在当前会话有效，关闭标签页后自动清除
// 2. 不会被其他标签页访问
// 3. 即使存在 XSS 漏洞，攻击者也只能在当前会话内窃取 Token
// 4. Token 会随会话过期而失效
let CSRF_TOKEN = ''

// 请求取消管理器
const canceller = new RequestCanceller()

// 请求去重器
const deduper = new RequestDeduper()

export function setCSRFToken(csrfToken: string): void {
  CSRF_TOKEN = csrfToken || ''
  if (csrfToken) {
    sessionStorage.setItem('logmanager_csrf_token', csrfToken)
  } else {
    sessionStorage.removeItem('logmanager_csrf_token')
  }
}

export function getCSRFToken(): string {
  return CSRF_TOKEN || sessionStorage.getItem('logmanager_csrf_token') || ''
}

export function clearCSRFToken(): void {
  CSRF_TOKEN = ''
  sessionStorage.removeItem('logmanager_csrf_token')
}

let SESSION_TOKEN = ''

export function setSessionToken(token: string): void {
  SESSION_TOKEN = token || ''
  if (token) {
    sessionStorage.setItem('logmanager_session_token', token)
  } else {
    sessionStorage.removeItem('logmanager_session_token')
  }
}

export function getSessionToken(): string {
  return SESSION_TOKEN || sessionStorage.getItem('logmanager_session_token') || ''
}

export async function fetchCSRFToken(): Promise<string | null> {
  try {
    const response = await fetch(`${API_BASE}/api/auth/csrf-token`, {
      credentials: 'include'
    })
    if (response.ok) {
      const data = await response.json() as { csrfToken: string; sessionToken?: string }
      if (data.csrfToken) {
        setCSRFToken(data.csrfToken)
      }
      if (data.sessionToken) {
        setSessionToken(data.sessionToken)
      }
      return data.csrfToken || null
    }
  } catch (e) {
    console.error('Failed to fetch CSRF token:', e)
  }
  return null
}

interface RequestOptions extends RequestInit {
  method?: 'GET' | 'POST' | 'PUT' | 'DELETE' | 'PATCH'
  retry?: boolean // 是否启用重试
  dedupe?: boolean // 是否启用去重
  cancelKey?: string // 取消请求的 key
}

/**
 * 应用会话 token 使用的自定义请求头名。
 *
 * 禁止改用标准的 Authorization 头：fnOS 1.2.0604 起，统一网关会把请求中的
 * `Authorization: Bearer <token>` 当成 fnOS 自身的票据（ticket/token）先行校验，
 * 校验不通过时直接短路返回纯文本 `invalid token`（HTTP 200），请求根本不会到达应用。
 * 前端拿到非 JSON 响应体后 `response.json()` 抛错，表现为
 * `列出日志失败: Unexpected token 'i', "invalid token" is not valid JSON`。
 */
export const SESSION_TOKEN_HEADER = 'X-Session-Token'

interface ParsedBody {
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  json: Record<string, any>
  text: string
}

/**
 * 按 Content-Type 解析响应体。
 * 统一网关在登录态失效时会返回 HTTP 200 + 纯文本（如 `invalid token`），
 * 因此不能直接调用 response.json()，否则会抛出 JSON 语法错误。
 */
async function parseBody(response: Response): Promise<ParsedBody> {
  const contentType = (response.headers.get('Content-Type') || '').toLowerCase()
  if (contentType.includes('application/json')) {
    try {
      const data = await response.json()
      if (data && typeof data === 'object') {
        return { json: data as Record<string, unknown>, text: '' }
      }
      return { json: {}, text: '' }
    } catch {
      return { json: {}, text: '' }
    }
  }
  let text = ''
  try {
    text = (await response.text()).trim()
  } catch {
    // ignore
  }
  return { json: {}, text }
}

/** 是否为 fnOS 统一网关返回的登录态失效提示 */
function isGatewayAuthFailure(text: string): boolean {
  return /invalid\s+token/i.test(text) || /invalid\s+ticket/i.test(text)
}

/**
 * 处理「HTTP 200 但响应体不是 JSON」的情况。
 * 典型场景是网关登录态失效（`invalid token`），此时清掉本地缓存的 token，
 * 避免后续请求继续被网关拒绝。
 */
function throwOnNonJSON(text: string): never {
  if (isGatewayAuthFailure(text)) {
    setSessionToken('')
    clearCSRFToken()
    throw new AuthenticationError('飞牛网关登录态已失效，请重新登录飞牛系统后从桌面重新打开本应用')
  }
  throw new ServerError(filterSensitiveInfo(text.slice(0, 200) || '服务器返回了非 JSON 响应'))
}

async function request<T = unknown>(endpoint: string, options: RequestOptions = {}): Promise<T> {
  const {
    retry = false,
    dedupe = false,
    cancelKey,
    ...fetchOptions
  } = options

  const executeRequest = async (): Promise<T> => {
    const url = `${API_BASE}${endpoint}`

    const headers: Record<string, string> = {
      'Content-Type': 'application/json',
      'X-Requested-With': 'XMLHttpRequest'
    }

    // 处理用户传入的 headers
    if (fetchOptions.headers) {
      const h = fetchOptions.headers as Record<string, string>
      for (const key of Object.keys(h)) {
        headers[key] = h[key]
      }
    }

    const method = fetchOptions.method || 'GET'
    const needCSRF = method === 'POST' || method === 'PUT' || method === 'DELETE'

    // POST/PUT/DELETE 请求需要 CSRF token
    if (needCSRF && !CSRF_TOKEN) {
      await fetchCSRFToken()
    }

    if (needCSRF && CSRF_TOKEN) {
      headers['X-CSRF-Token'] = CSRF_TOKEN
    }

    // 添加 session token 作为 cookie 的兜底方案（解决部分网络环境下 cookie 被阻断的问题）。
    // 必须使用自定义头而非 Authorization，原因见 SESSION_TOKEN_HEADER 注释。
    const sessionToken = getSessionToken()
    if (sessionToken && !headers[SESSION_TOKEN_HEADER]) {
      headers[SESSION_TOKEN_HEADER] = sessionToken
    }

    // 创建 AbortController
    const controller = cancelKey ? canceller.createController(cancelKey) : new AbortController()

    const response = await fetch(url, {
      ...fetchOptions,
      headers,
      credentials: 'include',
      signal: controller.signal
    })

    const body = await parseBody(response)

    if (!response.ok) {
      const error = body.json
      // 非 JSON 响应体（如网关返回的纯文本）也要作为错误信息兜底
      const rawMessage = error.error || body.text || `HTTP ${response.status}`

      if (response.status === 401) {
        clearCSRFToken()
        throw new AuthenticationError(String(rawMessage))
      }

      if (response.status === 400) {
        throw new ValidationError(String(rawMessage))
      }

      if (response.status >= 500) {
        throw new ServerError(String(rawMessage))
      }

      // CSRF 验证失败时，尝试获取新 token 并重试一次
      if (response.status === 403 && (error.error === 'CSRF验证失败' || error.code === 'CSRF_ERROR')) {
        clearCSRFToken()
        const newCSRFToken = await fetchCSRFToken()
        if (newCSRFToken) {
          headers['X-CSRF-Token'] = newCSRFToken
          const retryResponse = await fetch(url, {
            ...fetchOptions,
            headers,
            credentials: 'include'
          })
          const retryBody = await parseBody(retryResponse)
          if (retryResponse.ok && !retryBody.text) {
            return retryBody.json as T
          }
          // 过滤敏感信息
          const safeError = filterSensitiveInfo(
            retryBody.json.error || retryBody.text || `HTTP ${retryResponse.status}`
          )
          throw new ServerError(String(safeError))
        }
      }

      // 过滤敏感信息
      const safeError = filterSensitiveInfo(String(rawMessage))
      throw new NetworkError(safeError)
    }

    // HTTP 200 但响应体不是 JSON：通常是网关登录态失效返回的纯文本
    if (body.text) {
      throwOnNonJSON(body.text)
    }

    return body.json as T
  }

  // 根据配置决定是否启用重试或去重
  if (retry) {
    return withRetry(executeRequest)
  }

  if (dedupe) {
    return deduper.dedupe(endpoint, executeRequest)
  }

  return executeRequest()
}

export const api = {
  get<T = unknown>(endpoint: string): Promise<T> {
    return request<T>(endpoint)
  },

  post<T = unknown>(endpoint: string, data?: unknown): Promise<T> {
    return request<T>(endpoint, {
      method: 'POST',
      body: data ? JSON.stringify(data) : undefined
    })
  },

  put<T = unknown>(endpoint: string, data?: unknown): Promise<T> {
    return request<T>(endpoint, {
      method: 'PUT',
      body: data ? JSON.stringify(data) : undefined
    })
  },

  delete<T = unknown>(endpoint: string): Promise<T> {
    return request<T>(endpoint, {
      method: 'DELETE'
    })
  },

  setCSRFToken,
  getCSRFToken,
  setSessionToken,
  getSessionToken,
  clearCSRFToken,
  fetchCSRFToken
}

// 事件日志相关 API
export interface EventLoggerConfig {
  dbPath: string
  enabled: boolean
  checkInterval: number
  eventTypes: string[]
  minSeverity: string
  notificationChannels: string[]
  appFilter?: string[]
  excludeSources?: string[]
}

export interface EventLoggerStatus {
  isRunning: boolean
  lastCheckTime: string | null
  lastEventTime: string | null
  totalEventsProcessed: number
  lastError: string | null
  dbAccessible: boolean
  dbPath: string
}

export interface EventLogEntry {
  id: number
  timestamp: string
  source: string
  eventType: string
  severity: string
  message: string
  metadata?: string
  user?: string
}

export interface EventLoggerStats {
  totalEvents: number
}

export const eventLoggerApi = {
  getStatus: () => api.get<EventLoggerStatus>('/api/eventlogger/status'),
  
  getConfig: () => api.get<EventLoggerConfig>('/api/eventlogger/config'),
  
  updateConfig: (config: Partial<EventLoggerConfig>) => 
    api.put<EventLoggerConfig>('/api/eventlogger/config', config),
  
  getStats: () => api.get<EventLoggerStats>('/api/eventlogger/stats'),
  
  getEvents: (params: {
    limit?: number
    offset?: number
    startTime?: string
    endTime?: string
    severity?: string
    source?: string
    eventType?: string
    search?: string
  }) => api.get<{ events: EventLogEntry[]; total: number; hasMore: boolean }>(
    '/api/eventlogger/events?' + new URLSearchParams(params as any).toString()
  ),
  
  start: () => api.post<EventLoggerStatus>('/api/eventlogger/start'),
  
  stop: () => api.post<EventLoggerStatus>('/api/eventlogger/stop'),
  
  check: () => api.post<{ success: boolean }>('/api/eventlogger/check'),
  
  getSources: () => api.get<string[]>('/api/eventlogger/sources'),
  
  getAppNames: () => api.get<string[]>('/api/appnames')
}

// 与后端 services.CleanRule JSON 契约保持一致
export interface AutoCleanRule {
  id: string
  name: string
  enabled: boolean
  schedule: string // cron 表达式或秒级间隔（如 "3600s"）
  logDirs: string[]
  filePattern: string
  minSizeBytes: number
  maxSizeBytes: number
  retentionDays: number
  action: string // truncate | delete | deleteUninstalled | cleanEmpty
  maxFilesToClean: number
  description: string
  lastRun?: string
  lastResult?: string
  createdAt?: string
  updatedAt?: string
}

// 新增规则的请求体（对应后端 createCleanRuleBody）
export type AutoCleanRuleInput = Partial<
  Omit<AutoCleanRule, 'id' | 'lastRun' | 'lastResult' | 'createdAt' | 'updatedAt'>
>

export const autoCleanApi = {
  getRules: () => api.get<{ rules: AutoCleanRule[] }>('/api/auto-clean/rules'),

  addRule: (rule: AutoCleanRuleInput) =>
    api.post<{ rule: AutoCleanRule }>('/api/auto-clean/rules', rule),

  updateRule: (id: string, rule: AutoCleanRuleInput) =>
    api.put<{ rule: AutoCleanRule }>(`/api/auto-clean/rules/${id}`, rule),

  deleteRule: (id: string) =>
    api.delete<{ success: boolean }>(`/api/auto-clean/rules/${id}`),

  toggleRule: (id: string) =>
    api.post<{ rule: AutoCleanRule }>(`/api/auto-clean/rules/${id}/toggle`),

  executeRule: (id: string) =>
    api.post<{ cleaned: number; errors: string[] }>(`/api/auto-clean/rules/${id}/execute`)
}

export interface Bookmark {
  id: string
  name: string
  path: string
  displayPath?: string
  isDocker?: boolean
  createdAt: string
}

export const bookmarkApi = {
  getAll: () => api.get<{ bookmarks: Bookmark[] }>('/api/bookmarks'),

  add: (data: { name?: string; path: string; isDocker?: boolean }) =>
    api.post<{ bookmark: Bookmark }>('/api/bookmarks', data),

  delete: (id: string, path?: string, isDocker?: boolean) => {
    const params = new URLSearchParams()
    if (path) params.set('path', path)
    if (isDocker) params.set('isDocker', 'true')
    const qs = params.toString()
    return api.delete<{ success: boolean }>(`/api/bookmarks/${id}${qs ? '?' + qs : ''}`)
  }
}

// ==================== 内核版本 API ====================

export interface KernelVersion {
  version: string
  isCurrent: boolean
  bootSize: number
  bootSizeFormatted: string
  modulesSize: number
  modulesSizeFormatted: string
  srcSize: number
  srcSizeFormatted: string
  /** /usr/lib/linux-image-<version> 设备树目录占用 */
  imageSize: number
  imageSizeFormatted: string
  totalSize: number
  totalSizeFormatted: string
  hasModules: boolean
  hasSrc: boolean
  hasImage: boolean
  /**
   * 仍记录着该内核的 dpkg 包名。本功能只删文件、不经包管理器，因此删除后会留下
   * dpkg 空壳记录（半安装态）。
   *
   * undefined 表示无法查询 dpkg（例如非 Debian 系）——此时**不得**显示"无残留"；
   * 空数组表示已查询且确实没有记录。
   */
  dpkgPackages?: string[]
}

export interface KernelVersionsResponse {
  versions: KernelVersion[]
  total: number
  current: string
  unusedCount: number
  unusedSize: number
  unusedSizeFormatted: string
  totalSize: number
  totalSizeFormatted: string
  error?: string
}

export interface KernelCleanupResponse {
  removed: number
  total: number
  freedSize: number
  freedSizeFormatted: string
  errors: string[]
  /** 每个被删内核对应的 apt purge 命令 */
  dpkgLeftovers?: string[]
  /** 面向用户的残留说明，无残留时为空 */
  dpkgHint?: string
}

export interface KernelRemoveResponse {
  success: boolean
  message: string
  versions?: KernelVersion[]
  dpkgLeftovers?: string[]
  dpkgHint?: string
}

export const kernelApi = {
  getVersions: () => api.get<KernelVersionsResponse>('/api/kernel/versions'),

  cleanupUnused: () => api.post<KernelCleanupResponse>('/api/kernel/versions/cleanup'),

  removeVersion: (version: string) =>
    api.post<KernelRemoveResponse>(`/api/kernel/versions/${encodeURIComponent(version)}/remove`)
}

// ==================== 进程管理 API ====================

export interface ProcessItem {
  pid: number
  ppid: number
  user: string
  name: string
  state: string
  cpu: number
  memory: string
  memBytes: number
  startTime: string
  command: string
  exePath: string
  ports: number[]
  protect: boolean
  system: boolean
  isDocker?: boolean
  containerId?: string
  containerName?: string
}

export interface ProcessesResponse {
  total: number
  processes: ProcessItem[]
  error?: string
}

export interface KillProcessResult {
  success: boolean
  pid: number
  command: string
  signal: string
  terminated?: boolean
}

export type ProcessSortKey = 'pid' | 'name' | 'cpu' | 'mem'

export interface ProcessFile {
  path: string
  name: string
  isLog: boolean
  size: number
  sizeText: string
}

export interface ProcessLogResult {
  content: string
  totalLines: number
  size: number
  sizeFormatted: string
  truncated: boolean
  hasMore: boolean
}

export const processApi = {
  getProcesses: (params: {
    q?: string
    scope?: 'user' | 'all'
    sort?: ProcessSortKey
    order?: 'asc' | 'desc'
  } = {}) => {
    const search = new URLSearchParams()
    if (params.q) search.set('q', params.q)
    if (params.scope && params.scope !== 'user') search.set('scope', params.scope)
    if (params.sort && params.sort !== 'pid') search.set('sort', params.sort)
    if (params.order && params.order !== 'asc') search.set('order', params.order)
    const qs = search.toString()
    return api.get<ProcessesResponse>(`/api/processes${qs ? '?' + qs : ''}`)
  },

  killProcess: (pid: number, signal: 'term' | 'kill' = 'term') =>
    api.post<KillProcessResult>('/api/processes/kill', { pid, signal }),

  getProcessFiles: (pid: number) =>
    api.get<{ pid: number; files: ProcessFile[] }>(`/api/processes/${pid}/files`),

  readProcessLog: (pid: number, path: string, maxLines = 500, tail = false) => {
    const search = new URLSearchParams()
    search.set('path', path)
    search.set('maxLines', String(maxLines))
    if (tail) search.set('tail', 'true')
    return api.get<ProcessLogResult>(`/api/processes/${pid}/log?${search.toString()}`)
  }
}

// ==================== 系统空间清理 API ====================

export interface SpaceItem {
  path: string
  displayPath?: string
  name: string
  size: number
  sizeFormatted: string
  isDir: boolean
  fileCount: number
  reason: string
  safe: boolean
  /** 该条目仍在使用中（例如所属应用正在运行），本轮不提供删除 */
  inUse: boolean
  strategy: string // delete | recycle | truncate | none
  modified: string
}

export interface SpaceCategory {
  id: string
  name: string
  description: string
  risk: 'low' | 'medium' | 'high'
  icon: string
  itemCount: number
  totalSize: number
  totalSizeFormatted: string
  reclaimable: number
  reclaimableFormatted: string
  items: SpaceItem[]
}

export interface SpaceNotice {
  name: string
  path: string
  size: number
  sizeFormatted: string
  reason: string
  truncated: boolean
}

export interface VolumeUsage {
  /** 原始挂载路径（如 /vol1）。保留它是因为清理规则与列表 key 都基于真实路径。 */
  mount: string
  /** 磁盘类别：system=系统空间，storage=存储空间N，external=外接存储（面板里不展示）。 */
  kind?: string
  /** 飞牛语义名（系统空间 / 存储空间1），服务端给出，前端直接显示。 */
  label?: string
  total: number
  used: number
  free: number
  usedPercent: number
  totalFormatted: string
  usedFormatted: string
  freeFormatted: string
}

export interface SpaceUsageResponse {
  scannedAt: string
  /**
   * 生成当前数据的扫描耗时。注意它属于「产出这份数据的那次扫描」，
   * 不是本次打开面板的耗时，文案不要写成「本次扫描耗时」。
   */
  durationMs: number
  volumes: VolumeUsage[]
  categories: SpaceCategory[]
  notices: SpaceNotice[]
  reclaimable: number
  reclaimableFormatted: string
  categoriesWithItems: number
  /**
   * 数据年龄：服务端用「现在 - scannedAt」算出的毫秒数。
   * 之所以由服务端算，是因为面板刚打开时可能直接命中服务端持久化的快照，
   * 那份快照可能已经过去了几分钟；客户端自己拿本地时间减 scannedAt 会被
   * 机器时钟偏差污染（快照新却显示成几小时前，或者反过来）。
   */
  cacheAgeMs: number
  /**
   * 与 cacheAgeMs 同源的秒级年龄，供直接展示「数据更新于 x 前」使用。
   * 之所以不让前端自己除：服务端已经算过一次，前端再做除法只会引入取整分歧。
   */
  cacheAgeSeconds: number
  /**
   * 数据是否已超出新鲜窗口（服务端判定，目前约 5 分钟）。
   * 为 true 时表示展示的仍是旧快照，同时后台重扫已经启动或即将启动。
   */
  stale: boolean
  /**
   * 此刻是否真的有后台重扫在跑。
   * 它与 stale 是两件事：stale 只说明数据旧，refreshing 才说明有请求在飞。
   * 客户端据此决定要不要继续轮询（拿到更新的数据后自己停下来）。
   */
  refreshing: boolean
  /**
   * /tmp、/var/tmp 是否与系统分区同处一个文件系统。
   * undefined = 服务端无法判定，此时不要显示任何断言（/tmp 可能是 tmpfs）。
   */
  tempOnSystemDisk?: boolean
  errors?: string[]
  error?: string
}

export interface SpaceCleanItemResult {
  path: string
  status: string // deleted | recycled | truncated | skipped | failed
  freedBytes: number
  message?: string
}

export interface SpaceCleanResponse {
  freed: number
  freedFormatted: string
  deleted: number
  recycled: number
  truncated: number
  failed: number
  dryRun: boolean
  items: SpaceCleanItemResult[]
  errors?: string[]
}

export const spaceApi = {
  /**
   * 扫描系统空间占用。`refresh=true` 强制重扫（绕过服务端 TTL 缓存）。
   */
  getUsage: (refresh = false) =>
    api.get<SpaceUsageResponse>(`/api/space/usage${refresh ? '?refresh=true' : ''}`),

  /**
   * 清理选中的条目。
   * 路径会被服务端重新按规则表校验，规则外的路径一律跳过。
   */
  clean: (paths: string[], dryRun = false) =>
    api.post<SpaceCleanResponse>('/api/space/clean', { paths, dryRun })
}

// ==================== MCP 服务器 API ====================

export interface MCPConfig {
  enabled: boolean
  apiKey: string
  appName: string
  port: number
  bindAddr: string
  endpoint: string
  hostIp?: string
}

export interface MCPSaveResponse {
  ok: boolean
  portChanged: boolean
  requiresRestart: boolean
}

export const mcpApi = {
  getConfig: () => api.get<MCPConfig>('/api/mcp/config'),

  updateConfig: (config: {
    enabled: boolean
    apiKey: string
    appName: string
    port: number
    bindAddr: string
  }) => api.put<MCPSaveResponse>('/api/mcp/config', config)
}

export default api
