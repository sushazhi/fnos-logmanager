/**
 * 通知实时推送 Composable
 * 使用 HTTP 轮询替代 WebSocket（fnOS iframe 代理不支持 WebSocket 长连接）
 */

import { ref, onUnmounted } from 'vue'
import api from '../services/api'

export function useNotifyWebSocket() {
  const isConnected = ref(false)
  const lastMonitorUpdate = ref<any>(null)
  const lastHistoryUpdate = ref<any>(null)
  const lastRulesUpdate = ref<any>(null)

  let pollTimer: ReturnType<typeof setInterval> | null = null
  const POLL_INTERVAL = 5000

  async function pollUpdates(): Promise<void> {
    try {
      const [statusRes, historyRes, rulesRes] = await Promise.allSettled([
        api.get<{ status?: any }>('/api/notifications/monitor/status'),
        api.get<{ history?: any[] }>('/api/notifications/history?limit=5'),
        api.get<{ rules?: any[] }>('/api/notifications/rules')
      ])

      if (statusRes.status === 'fulfilled' && statusRes.value?.status) {
        lastMonitorUpdate.value = statusRes.value.status
      }
      if (historyRes.status === 'fulfilled' && historyRes.value?.history) {
        lastHistoryUpdate.value = historyRes.value.history
      }
      if (rulesRes.status === 'fulfilled' && rulesRes.value?.rules) {
        lastRulesUpdate.value = rulesRes.value.rules
      }

      isConnected.value = true
    } catch {
      isConnected.value = false
    }
  }

  function connect(): void {
    if (pollTimer) return
    isConnected.value = true
    pollUpdates()
    pollTimer = setInterval(pollUpdates, POLL_INTERVAL)
  }

  function disconnect(): void {
    if (pollTimer) {
      clearInterval(pollTimer)
      pollTimer = null
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
