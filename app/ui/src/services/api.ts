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

const API_BASE = window.location.origin

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

export async function fetchCSRFToken(): Promise<string | null> {
  try {
    const response = await fetch(`${API_BASE}/api/auth/csrf-token`, {
      credentials: 'include'
    })
    if (response.ok) {
      const data = await response.json() as { csrfToken: string }
      if (data.csrfToken) {
        setCSRFToken(data.csrfToken)
        return data.csrfToken
      }
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
  eventsBySeverity: Record<string, number>
  eventsBySource: Record<string, number>
  eventsByType: Record<string, number>
  timeRange: {
    earliest: string | null
    latest: string | null
  }
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
  
  restart: () => api.post<EventLoggerStatus>('/api/eventlogger/restart'),
  
  check: () => api.post<{ success: boolean }>('/api/eventlogger/check'),
  
  getSources: () => api.get<string[]>('/api/eventlogger/sources'),
  
  getAppNames: () => api.get<string[]>('/api/appnames')
}

export default api
