import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { api } from '../services/api'

export interface NotificationChannel {
  id: string
  type: string
  name: string
  config: Record<string, any>
  enabled: boolean
}

export interface NotificationRule {
  id: string
  name: string
  pattern: string
  level: string
  channels: string[]
  enabled: boolean
  cooldown?: number
  quietHours?: { start: string; end: string }
}

export const useNotificationStore = defineStore('notification', () => {
  // State
  const channels = ref<NotificationChannel[]>([])
  const rules = ref<NotificationRule[]>([])
  const loading = ref(false)
  const error = ref<string | null>(null)

  // Getters
  const enabledChannels = computed(() => channels.value.filter(c => c.enabled))
  const enabledRules = computed(() => rules.value.filter(r => r.enabled))

  // Actions
  async function loadChannels() {
    try {
      loading.value = true
      const response = await api.get<{ channels: NotificationChannel[] }>('/api/notifications/channels')
      channels.value = response.channels
    } catch (err) {
      error.value = err instanceof Error ? err.message : '加载通知渠道失败'
      throw err
    } finally {
      loading.value = false
    }
  }

  async function addChannel(channel: Omit<NotificationChannel, 'id'>) {
    try {
      const response = await api.post<{ channel: NotificationChannel }>('/api/notifications/channels', channel)
      channels.value.push(response.channel)
      return response.channel
    } catch (err) {
      error.value = err instanceof Error ? err.message : '添加通知渠道失败'
      throw err
    }
  }

  async function updateChannel(id: string, updates: Partial<NotificationChannel>) {
    try {
      await api.put(`/api/notifications/channels/${id}`, updates)
      const index = channels.value.findIndex(c => c.id === id)
      if (index !== -1) {
        channels.value[index] = { ...channels.value[index], ...updates }
      }
    } catch (err) {
      error.value = err instanceof Error ? err.message : '更新通知渠道失败'
      throw err
    }
  }

  async function deleteChannel(id: string) {
    try {
      await api.delete(`/api/notifications/channels/${id}`)
      channels.value = channels.value.filter(c => c.id !== id)
    } catch (err) {
      error.value = err instanceof Error ? err.message : '删除通知渠道失败'
      throw err
    }
  }

  async function testChannel(id: string) {
    try {
      const response = await api.post<{ success: boolean; message?: string }>(
        `/api/notifications/channels/${id}/test`
      )
      return response
    } catch (err) {
      error.value = err instanceof Error ? err.message : '测试通知渠道失败'
      throw err
    }
  }

  async function loadRules() {
    try {
      loading.value = true
      const response = await api.get<{ rules: NotificationRule[] }>('/api/notifications/rules')
      rules.value = response.rules
    } catch (err) {
      error.value = err instanceof Error ? err.message : '加载通知规则失败'
      throw err
    } finally {
      loading.value = false
    }
  }

  async function addRule(rule: Omit<NotificationRule, 'id'>) {
    try {
      const response = await api.post<{ rule: NotificationRule }>('/api/notifications/rules', rule)
      rules.value.push(response.rule)
      return response.rule
    } catch (err) {
      error.value = err instanceof Error ? err.message : '添加通知规则失败'
      throw err
    }
  }

  async function updateRule(id: string, updates: Partial<NotificationRule>) {
    try {
      await api.put(`/api/notifications/rules/${id}`, updates)
      const index = rules.value.findIndex(r => r.id === id)
      if (index !== -1) {
        rules.value[index] = { ...rules.value[index], ...updates }
      }
    } catch (err) {
      error.value = err instanceof Error ? err.message : '更新通知规则失败'
      throw err
    }
  }

  async function deleteRule(id: string) {
    try {
      await api.delete(`/api/notifications/rules/${id}`)
      rules.value = rules.value.filter(r => r.id !== id)
    } catch (err) {
      error.value = err instanceof Error ? err.message : '删除通知规则失败'
      throw err
    }
  }

  function clearError() {
    error.value = null
  }

  return {
    // State
    channels,
    rules,
    loading,
    error,
    
    // Getters
    enabledChannels,
    enabledRules,
    
    // Actions
    loadChannels,
    addChannel,
    updateChannel,
    deleteChannel,
    testChannel,
    loadRules,
    addRule,
    updateRule,
    deleteRule,
    clearError
  }
})
