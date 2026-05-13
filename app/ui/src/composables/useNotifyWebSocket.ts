/**
 * 通知实时推送 Composable
 * 通过统一网关使用 WebSocket 实时推送通知状态
 */

import { ref, onUnmounted } from 'vue'
import api from '../services/api'

interface NotifyMessage {
  type: 'status' | 'history' | 'rules'
  data: any
}

export function useNotifyWebSocket() {
  const isConnected = ref(false)
  const lastMonitorUpdate = ref<any>(null)
  const lastHistoryUpdate = ref<any>(null)
  const lastRulesUpdate = ref<any>(null)

  let ws: WebSocket | null = null
  let reconnectTimer: ReturnType<typeof setTimeout> | null = null
  let reconnectAttempts = 0
  const MAX_RECONNECT = 5
  const RECONNECT_DELAY = 3000

  function getWsUrl(): string {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const base = `${protocol}//${window.location.host}`
    const token = api.getSessionToken()
    const tokenParam = token ? `?token=${encodeURIComponent(token)}` : ''
    if (window.location.pathname.startsWith('/app/logmanager')) {
      return `${base}/app/logmanager/api/notifications/ws${tokenParam}`
    }
    return `${base}/api/notifications/ws${tokenParam}`
  }

  function connect(): void {
    if (ws && ws.readyState === WebSocket.OPEN) return

    try {
      ws = new WebSocket(getWsUrl())

      ws.onopen = () => {
        isConnected.value = true
        reconnectAttempts = 0
      }

      ws.onmessage = (event) => {
        try {
          const msg = JSON.parse(event.data) as NotifyMessage
          switch (msg.type) {
            case 'status':
              lastMonitorUpdate.value = msg.data
              break
            case 'history':
              lastHistoryUpdate.value = msg.data
              break
            case 'rules':
              lastRulesUpdate.value = msg.data
              break
          }
        } catch { /* ignore */ }
      }

      ws.onclose = () => {
        isConnected.value = false
        scheduleReconnect()
      }

      ws.onerror = () => {
        isConnected.value = false
      }
    } catch {
      isConnected.value = false
      scheduleReconnect()
    }
  }

  function scheduleReconnect(): void {
    if (reconnectAttempts >= MAX_RECONNECT) return
    if (reconnectTimer) return
    reconnectTimer = setTimeout(() => {
      reconnectTimer = null
      reconnectAttempts++
      connect()
    }, RECONNECT_DELAY * (reconnectAttempts + 1))
  }

  function disconnect(): void {
    if (reconnectTimer) {
      clearTimeout(reconnectTimer)
      reconnectTimer = null
    }
    if (ws) {
      ws.onclose = null
      ws.close()
      ws = null
    }
    isConnected.value = false
  }

  onUnmounted(() => {
    disconnect()
  })

  return {
    isConnected,
    lastMonitorUpdate,
    lastHistoryUpdate,
    lastRulesUpdate,
    connect,
    disconnect
  }
}
