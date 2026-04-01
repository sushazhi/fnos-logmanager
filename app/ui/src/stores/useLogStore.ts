import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import type { LogItem, Stats, Dir, ListType } from '../types'
import { api } from '../services/api'

export const useLogStore = defineStore('log', () => {
  // State
  const logs = ref<LogItem[]>([])
  const stats = ref<Stats | null>(null)
  const dirs = ref<Dir[]>([])
  const selectedDir = ref<string | null>(null)
  const listType = ref<ListType>('logs')
  const filterEnabled = ref(false)
  const loading = ref(false)
  const error = ref<string | null>(null)

  // Getters
  const hasLogs = computed(() => logs.value.length > 0)
  const totalLogs = computed(() => stats.value?.totalLogs || 0)
  const totalSize = computed(() => stats.value?.totalSize || '0 B')

  // Actions
  async function loadStats() {
    try {
      const response = await api.get<{ 
        totalLogs: number
        totalSizeFormatted: string
        totalArchives: number
        largeFiles: number 
      }>('/api/stats')
      
      stats.value = {
        totalLogs: response.totalLogs,
        totalSize: response.totalSizeFormatted,
        archiveCount: response.totalArchives,
        largeFiles: response.largeFiles
      }
    } catch (err) {
      error.value = err instanceof Error ? err.message : '加载统计信息失败'
      throw err
    }
  }

  async function loadDirs() {
    try {
      loading.value = true
      const response = await api.get<{ dirs: Array<{ path: string; logCount?: number; totalSize?: string }> }>('/api/dirs')
      
      dirs.value = response.dirs.map(dir => ({
        path: dir.path,
        displayName: dir.path.split('/').pop() || dir.path,
        logCount: dir.logCount,
        totalSize: dir.totalSize
      }))
    } catch (err) {
      error.value = err instanceof Error ? err.message : '加载目录列表失败'
      throw err
    } finally {
      loading.value = false
    }
  }

  async function loadLogs(dirPath?: string) {
    try {
      loading.value = true
      const endpoint = dirPath 
        ? `/api/logs/list?dir=${encodeURIComponent(dirPath)}`
        : '/api/logs/list'
      
      const response = await api.get<{
        logs: Array<{
          path: string
          size: number
          sizeFormatted: string
          canDelete?: boolean
        }>
        total: number
      }>(endpoint)
      
      logs.value = response.logs.map(log => ({
        path: log.path,
        size: log.size,
        sizeFormatted: log.sizeFormatted,
        showActions: true,
        canDelete: log.canDelete
      }))
      
      selectedDir.value = dirPath || null
      listType.value = 'logs'
    } catch (err) {
      error.value = err instanceof Error ? err.message : '加载日志列表失败'
      throw err
    } finally {
      loading.value = false
    }
  }

  async function viewLog(path: string) {
    try {
      loading.value = true
      const response = await api.get<{ content: string }>(
        `/api/logs/view?path=${encodeURIComponent(path)}`
      )
      return response.content
    } catch (err) {
      error.value = err instanceof Error ? err.message : '加载日志内容失败'
      throw err
    } finally {
      loading.value = false
    }
  }

  async function truncateLog(path: string) {
    try {
      await api.post('/api/logs/truncate', { path })
      // 重新加载统计和日志列表
      await Promise.all([loadStats(), loadLogs(selectedDir.value || undefined)])
    } catch (err) {
      error.value = err instanceof Error ? err.message : '清空日志失败'
      throw err
    }
  }

  async function deleteLog(path: string) {
    try {
      await api.delete(`/api/logs/delete?path=${encodeURIComponent(path)}`)
      // 重新加载统计和日志列表
      await Promise.all([loadStats(), loadLogs(selectedDir.value || undefined)])
    } catch (err) {
      error.value = err instanceof Error ? err.message : '删除日志失败'
      throw err
    }
  }

  async function loadFilterStatus() {
    try {
      const response = await api.get<{ enabled: boolean }>('/api/settings/filter')
      filterEnabled.value = response.enabled
    } catch (err) {
      // 静默失败
      console.error('Failed to load filter status:', err)
    }
  }

  async function toggleFilter() {
    try {
      const newValue = !filterEnabled.value
      await api.post('/api/settings/filter', { enabled: newValue })
      filterEnabled.value = newValue
    } catch (err) {
      error.value = err instanceof Error ? err.message : '切换过滤状态失败'
      throw err
    }
  }

  function clearLogs() {
    logs.value = []
    selectedDir.value = null
    listType.value = 'logs'
  }

  function clearError() {
    error.value = null
  }

  return {
    // State
    logs,
    stats,
    dirs,
    selectedDir,
    listType,
    filterEnabled,
    loading,
    error,
    
    // Getters
    hasLogs,
    totalLogs,
    totalSize,
    
    // Actions
    loadStats,
    loadDirs,
    loadLogs,
    viewLog,
    truncateLog,
    deleteLog,
    loadFilterStatus,
    toggleFilter,
    clearLogs,
    clearError
  }
})
