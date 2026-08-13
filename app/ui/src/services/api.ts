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

    // 添加 session token 作为 Bearer token（cookie 的兜底方案，解决部分网络环境下 cookie 被阻断的问题）
    const sessionToken = getSessionToken()
    if (sessionToken && !headers['Authorization']) {
      headers['Authorization'] = `Bearer ${sessionToken}`
    }

    // 创建 AbortController
    const controller = cancelKey ? canceller.createController(cancelKey) : new AbortController()

    const response = await fetch(url, {
      ...fetchOptions,
      headers,
      credentials: 'include',
      signal: controller.signal
    })

    if (!response.ok) {
      const error = await response.json().catch(() => ({}))

      if (response.status === 401) {
        clearCSRFToken()
        throw new AuthenticationError(error.error || '需要认证')
      }

      if (response.status === 400) {
        throw new ValidationError(error.error || '请求参数错误')
      }

      if (response.status >= 500) {
        throw new ServerError(error.error || '服务器错误')
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
          if (retryResponse.ok) {
            return retryResponse.json() as Promise<T>
          }
          const retryError = await retryResponse.json().catch(() => ({}))
          // 过滤敏感信息
          const safeError = filterSensitiveInfo(retryError.error || `HTTP ${retryResponse.status}`)
          throw new ServerError(safeError)
        }
      }

      // 过滤敏感信息
      const safeError = filterSensitiveInfo(error.error || `HTTP ${response.status}`)
      throw new NetworkError(safeError)
    }

    return response.json() as Promise<T>
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
  action: string // truncate | delete | deleteUninstalled
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
  totalSize: number
  totalSizeFormatted: string
  hasModules: boolean
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
}

export interface KernelRemoveResponse {
  success: boolean
  message: string
  versions?: KernelVersion[]
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
