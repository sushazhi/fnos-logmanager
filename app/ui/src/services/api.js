const API_BASE = window.location.origin

let TOKEN = localStorage.getItem('logmanager_token') || ''

export function setToken(token) {
  TOKEN = token
  if (token) {
    localStorage.setItem('logmanager_token', token)
  } else {
    localStorage.removeItem('logmanager_token')
  }
}

export function getToken() {
  return TOKEN
}

function getAuthParams() {
  return TOKEN ? `?token=${encodeURIComponent(TOKEN)}` : ''
}

async function request(endpoint, options = {}) {
  const separator = endpoint.includes('?') ? '&' : '?'
  const url = TOKEN ? `${API_BASE}${endpoint}${separator}token=${encodeURIComponent(TOKEN)}` : `${API_BASE}${endpoint}`
  
  const headers = {
    'Content-Type': 'application/json',
    ...options.headers
  }
  
  if (TOKEN) {
    headers['Authorization'] = `Bearer ${TOKEN}`
  }
  
  const response = await fetch(url, {
    ...options,
    headers
  })
  
  if (response.status === 401) {
    setToken('')
    throw new Error('需要认证')
  }
  
  if (!response.ok) {
    const error = await response.json().catch(() => ({}))
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
  
  setToken,
  getToken
}

export default api
