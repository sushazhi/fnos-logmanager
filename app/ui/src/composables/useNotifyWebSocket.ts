/**
 * 通知实时推送 Composable
 * 使用 WebSocket 替代轮询，接收监控状态和通知历史更新
 */

import { ref, onUnmounted } from 'vue'

interface NotifyWSMessage {
  type: 'connected' | 'update'
  channel?: string
  data?: any
  timestamp?: number
}

type NotifyChannel = 'monitor' | 'history' | 'rules'

export function useNotifyWebSocket() {
  const ws = ref<WebSocket | null>(null)
  const isConnected = ref(false)
  const lastMonitorUpdate = ref<any>(null)
  const lastHistoryUpdate = ref<any>(null)
  const lastRulesUpdate = ref<any>(null)

  let reconnectTimer: ReturnType<typeof setTimeout> | null = null
  let reconnectAttempts = 0
  const MAX_RECONNECT = 5

  function getWsUrl(): string {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    return `${protocol}//${window.location.host}/api/notifications/ws`
  }

  function connect(channels: NotifyChannel[] = ['monitor', 'history', 'rules']): void {
    if (ws.value && ws.value.readyState === WebSocket.OPEN) return

    try {
      const socket = new WebSocket(getWsUrl())

      socket.onopen = () => {
        isConnected.value = true
        reconnectAttempts = 0
        // 订阅频道
        socket.send(JSON.stringify({ type: 'subscribe', channels }))
      }

      socket.onmessage = (event) => {
        try {
          const message: NotifyWSMessage = JSON.parse(event.data)
          handleMessage(message)
        } catch {
          // Ignore
        }
      }

      socket.onclose = () => {
        isConnected.value = false
        ws.value = null
        attemptReconnect(channels)
      }

      socket.onerror = () => {
        // Error handled by onclose
      }

      ws.value = socket
    } catch {
      // WebSocket not available, fall back to polling
    }
  }

  function handleMessage(message: NotifyWSMessage): void {
    if (message.type === 'update' && message.channel) {
      switch (message.channel) {
        case 'monitor':
          lastMonitorUpdate.value = message.data
          break
        case 'history':
          lastHistoryUpdate.value = message.data
          break
        case 'rules':
          lastRulesUpdate.value = message.data
          break
      }
    }
  }

  function attemptReconnect(channels: NotifyChannel[]): void {
    if (reconnectAttempts >= MAX_RECONNECT) return
    if (reconnectTimer) return

    const delay = 1000 * Math.pow(2, reconnectAttempts)
    reconnectTimer = setTimeout(() => {
      reconnectTimer = null
      reconnectAttempts++
      connect(channels)
    }, delay)
  }

  function disconnect(): void {
    if (reconnectTimer) {
      clearTimeout(reconnectTimer)
      reconnectTimer = null
    }
    reconnectAttempts = MAX_RECONNECT

    if (ws.value) {
      ws.value.close()
      ws.value = null
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
