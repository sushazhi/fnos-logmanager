const API_BASE = window.location.origin

// CSRF Token 存储说明：
// 使用 sessionStorage 存储 CSRF Token，相比 localStorage 更安全：
// 1. 数据仅在当前会话有效，关闭标签页后自动清除
// 2. 不会被其他标签页访问
// 3. 即使存在 XSS 漏洞，攻击者也只能在当前会话内窃取 Token
// 4. Token 会随会话过期而失效
let CSRF_TOKEN = ''

export function setCSRFToken(csrfToken) {
  CSRF_TOKEN = csrfToken || ''
  if (csrfToken) {
    sessionStorage.setItem('logmanager_csrf_token', csrfToken)
  } else {
    sessionStorage.removeItem('logmanager_csrf_token')
  }
}

export function getCSRFToken() {
  return CSRF_TOKEN || sessionStorage.getItem('logmanager_csrf_token') || ''
}

export function clearCSRFToken() {
  CSRF_TOKEN = ''
  sessionStorage.removeItem('logmanager_csrf_token')
}

export async function fetchCSRFToken() {
  try {
    const response = await fetch(`${API_BASE}/api/auth/csrf-token`, {
      credentials: 'include'
    })
    if (response.ok) {
      const data = await response.json()
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

async function request(endpoint, options = {}) {
  const url = `${API_BASE}${endpoint}`
  
  const headers = {
    'Content-Type': 'application/json',
    ...options.headers
  }
  
  const method = options.method || 'GET'
  const needCSRF = method === 'POST' || method === 'PUT' || method === 'DELETE'
  
  // POST/PUT/DELETE 请求需要 CSRF token
  if (needCSRF && !CSRF_TOKEN) {
    await fetchCSRFToken()
  }
  
  if (needCSRF && CSRF_TOKEN) {
    headers['X-CSRF-Token'] = CSRF_TOKEN
  }
  
  const response = await fetch(url, {
    ...options,
    headers,
    credentials: 'include'
  })
  
  if (!response.ok) {
    const error = await response.json().catch(() => ({}))
    
    if (response.status === 401) {
      clearCSRFToken()
      throw Object.assign(new Error(error.error || '需要认证'), error)
    }
    
    // CSRF 验证失败时，尝试获取新 token 并重试一次
    if (response.status === 403 && (error.error === 'CSRF验证失败' || error.code === 'CSRF_ERROR')) {
      clearCSRFToken()
      const newCSRFToken = await fetchCSRFToken()
      if (newCSRFToken) {
        headers['X-CSRF-Token'] = newCSRFToken
        const retryResponse = await fetch(url, {
          ...options,
          headers,
          credentials: 'include'
        })
        if (retryResponse.ok) {
          return retryResponse.json()
        }
        const retryError = await retryResponse.json().catch(() => ({}))
        // 过滤敏感信息
        const safeError = filterSensitiveInfo(retryError.error || `HTTP ${retryResponse.status}`)
        throw Object.assign(new Error(safeError), retryError)
      }
    }
    
    // 过滤敏感信息
    const safeError = filterSensitiveInfo(error.error || `HTTP ${response.status}`)
    throw Object.assign(new Error(safeError), error)
  }
  
  return response.json()
}

/**
 * 过滤错误信息中的敏感内容
 * @param {string} message - 原始错误消息
 * @returns {string} - 过滤后的安全消息
 */
function filterSensitiveInfo(message) {
  if (typeof message !== 'string') return message
  
  // 过滤路径信息
  let filtered = message.replace(/\/[\w\-./]+/g, '[PATH]')
  
  // 过滤IP地址
  filtered = filtered.replace(/\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}/g, '[IP]')
  
  // 过滤端口号
  filtered = filtered.replace(/:\d{2,5}/g, ':[PORT]')
  
  return filtered
}

export const api = {
  get(endpoint) {
    return request(endpoint)
  },
  
  post(endpoint, data) {
    return request(endpoint, {
      method: 'POST',
      body: JSON.stringify(data)
    })
  },
  
  setCSRFToken,
  getCSRFToken,
  clearCSRFToken,
  fetchCSRFToken
}

export default api
