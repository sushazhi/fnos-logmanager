const API_BASE = window.location.origin

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
    const response = await fetch(`${API_BASE}/api/csrf-token`, {
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
        throw Object.assign(new Error(retryError.error || `HTTP ${retryResponse.status}`), retryError)
      }
    }
    
    throw Object.assign(new Error(error.error || `HTTP ${response.status}`), error)
  }
  
  return response.json()
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
