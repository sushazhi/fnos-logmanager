import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import type { LogItem, ListType, LogsResponse, CleanType } from '../types'
import api, { API_BASE } from '../services/api'
import { useStatusStore } from './useStatusStore'
import { safeErrorMessage } from '../utils/request'

export interface LogTab {
  id: string
  title: string
  content: string
  filePath: string
  isDocker: boolean
  truncated: boolean
  hasMore: boolean
  totalLines: number
}

export const useLogsStore = defineStore('logs', () => {
  const logList = ref<LogItem[]>([])
  const listType = ref<ListType>('logs')
  const showLogModal = ref(false)
  const showCleanModal = ref(false)
  const showSearchModal = ref(false)
  const logContent = ref('')
  const logTitle = ref('')
  const filterEnabled = ref(true)
  const logTruncated = ref(false)
  const logHasMore = ref(false)
  const logTotalLines = ref(0)
  const logCurrentPath = ref('')
  const logIsDocker = ref(false)

  const logTabs = ref<LogTab[]>([])
  const activeTabId = ref<string>('')
  const activeTab = computed(() => logTabs.value.find(t => t.id === activeTabId.value) || null)

  async function loadFilterStatus(): Promise<void> {
    try {
      const data = await api.get<{ enabled: boolean }>('/api/settings/filter')
      filterEnabled.value = data.enabled !== false
    } catch {
      filterEnabled.value = true
    }
  }

  async function toggleFilter(): Promise<void> {
    const { setStatus } = useStatusStore()
    filterEnabled.value = !filterEnabled.value
    try {
      await api.post('/api/settings/filter', { enabled: filterEnabled.value })
      setStatus(filterEnabled.value ? '敏感信息过滤已启用' : '敏感信息过滤已禁用', 'success')
    } catch (e) {
      setStatus('设置保存失败: ' + safeErrorMessage(e), 'error')
    }
  }

  async function listLogs(): Promise<void> {
    const { setStatus } = useStatusStore()
    listType.value = 'logs'
    setStatus('正在列出日志文件...', 'loading')
    try {
      const data = await api.get<LogsResponse>('/api/logs/list?limit=100')
      logList.value = data.logs.map(log => ({
        ...log,
        showActions: true
      }))
      setStatus(`找到 ${data.total} 个日志文件`, 'success')
    } catch (e) {
      setStatus('列出日志失败: ' + safeErrorMessage(e), 'error')
    }
  }

  async function searchLogs(type: 'size' | 'name', threshold: string, pattern: string): Promise<void> {
    const { setStatus } = useStatusStore()
    listType.value = 'logs'
    showSearchModal.value = false

    if (type === 'size') {
      setStatus('正在查找大日志文件...', 'loading')
      try {
        const data = await api.get<LogsResponse>(`/api/logs/search?type=size&threshold=${encodeURIComponent(threshold)}&limit=50`)
        logList.value = data.logs.map(log => ({
          ...log,
          showActions: true
        }))
        setStatus(`找到 ${data.logs.length} 个大日志文件`, 'success')
      } catch (e) {
        setStatus('查找失败: ' + safeErrorMessage(e), 'error')
      }
    } else if (type === 'name') {
      setStatus('正在按名称查找日志文件...', 'loading')
      try {
        const data = await api.get<LogsResponse>(`/api/logs/search?type=name&pattern=${encodeURIComponent(pattern)}&limit=50`)
        logList.value = data.logs.map(log => ({
          ...log,
          showActions: true
        }))
        setStatus(`找到 ${data.total} 个匹配的日志文件`, 'success')
      } catch (e) {
        setStatus('查找失败: ' + safeErrorMessage(e), 'error')
      }
    }
  }

  async function viewLog(path: string, maxLines: number = 5000): Promise<void> {
    const { setStatus } = useStatusStore()
    const existing = logTabs.value.find(t => t.filePath === path && !t.isDocker)
    if (existing) {
      activeTabId.value = existing.id
      syncFromActiveTab()
      showLogModal.value = true
      return
    }
    setStatus('正在加载日志内容...', 'loading')
    try {
      const data = await api.get<{
        content: string
        totalLines?: number
        truncated?: boolean
        hasMore?: boolean
      }>(`/api/log/content?path=${encodeURIComponent(path)}&maxLines=${maxLines}`)
      const tab: LogTab = {
        id: `log_${Date.now()}`,
        title: path.split('/').pop() || path,
        content: data.content || '(空文件)',
        filePath: path,
        isDocker: false,
        totalLines: data.totalLines || 0,
        truncated: data.truncated || false,
        hasMore: data.hasMore || false
      }
      addTab(tab)
      showLogModal.value = true
      setStatus('日志加载完成', 'success')
    } catch (e) {
      setStatus('加载失败: ' + safeErrorMessage(e), 'error')
    }
  }

  async function loadAllLines(): Promise<void> {
    if (!logCurrentPath.value) return
    const { setStatus } = useStatusStore()
    setStatus('正在加载全部日志...', 'loading')
    try {
      const data = await api.get<{
        content: string
        totalLines?: number
        truncated?: boolean
        hasMore?: boolean
      }>(`/api/log/content?path=${encodeURIComponent(logCurrentPath.value)}&maxLines=200000`)
      logContent.value = data.content || '(空文件)'
      logTotalLines.value = data.totalLines || 0
      logTruncated.value = data.truncated || false
      logHasMore.value = data.hasMore || false
      setStatus('日志加载完成', 'success')
    } catch (e) {
      setStatus('加载失败: ' + safeErrorMessage(e), 'error')
    }
  }

  async function truncateLog(path: string): Promise<boolean> {
    const { setStatus, confirm } = useStatusStore()
    const confirmed = await confirm({
      title: '清空日志',
      message: '确定要清空此日志文件吗？',
      type: 'warning',
      confirmText: '清空'
    })
    if (!confirmed) return false

    setStatus('正在清空日志...', 'loading')
    try {
      await api.post('/api/log/truncate', { path })
      setStatus('日志已清空', 'success')
      return true
    } catch (e) {
      setStatus('清空失败: ' + safeErrorMessage(e), 'error')
      return false
    }
  }

  async function deleteLog(path: string): Promise<boolean> {
    const { setStatus, confirm } = useStatusStore()
    const confirmed = await confirm({
      title: '删除日志',
      message: '确定要删除此日志文件吗？此操作不可恢复！',
      type: 'danger',
      confirmText: '删除'
    })
    if (!confirmed) return false

    setStatus('正在删除日志文件...', 'loading')
    try {
      await api.post('/api/log/delete', { path })
      logList.value = logList.value.filter(log => log.path !== path)
      setStatus('日志文件已删除', 'success')
      return true
    } catch (e) {
      setStatus('删除失败: ' + safeErrorMessage(e), 'error')
      return false
    }
  }

  async function executeClean(type: CleanType, threshold: string, days: number | null): Promise<void> {
    const { setStatus } = useStatusStore()
    showCleanModal.value = false
    setStatus('正在清理日志...', 'loading')
    try {
      const data = await api.post<{ cleaned: number }>('/api/logs/clean', {
        type,
        threshold,
        days: type === 'deleteOld' ? days : null,
        action: type === 'deleteOld' ? 'delete' : type
      })
      setStatus(`清理完成，共处理 ${data.cleaned} 个文件`, 'success')
    } catch (e) {
      setStatus('清理失败: ' + safeErrorMessage(e), 'error')
    }
  }

  function clearList(): void {
    const { setStatus } = useStatusStore()
    logList.value = []
    listType.value = 'logs'
    setStatus('就绪', 'success')
  }

  async function cleanEmptyDirs(): Promise<void> {
    const { setStatus, confirm } = useStatusStore()
    const confirmed = await confirm({
      title: '清理空文件夹',
      message: '确定要删除已卸载应用的空文件夹吗？\n\n将检查以下目录：\n/vol1/@appcenter\n/vol1/@appconf\n/vol1/@appdata\n/vol1/@apphome\n/vol1/@appmeta\n/vol1/@apptemp\n/vol1/@appshare',
      type: 'warning',
      confirmText: '开始清理'
    })
    if (!confirmed) return

    setStatus('正在清理空文件夹...', 'loading')
    try {
      const data = await api.post<{ cleaned: number; dirs: string[]; errors: string[] }>('/api/dirs/clean-empty')
      if (data.errors && data.errors.length > 0) {
        setStatus(`清理完成，删除 ${data.cleaned} 个文件夹，但有 ${data.errors.length} 个错误`, 'warning')
      } else if (data.cleaned === 0) {
        setStatus('没有找到需要清理的空文件夹', 'success')
      } else {
        setStatus(`清理完成，共删除 ${data.cleaned} 个空文件夹`, 'success')
      }
    } catch (e) {
      setStatus('清理失败: ' + safeErrorMessage(e), 'error')
    }
  }

  function addTab(tab: LogTab): void {
    const existing = logTabs.value.find(t => t.filePath === tab.filePath && t.isDocker === tab.isDocker)
    if (existing) {
      activeTabId.value = existing.id
      syncFromActiveTab()
      return
    }
    logTabs.value.push(tab)
    activeTabId.value = tab.id
    syncFromActiveTab()
  }

  function removeTab(tabId: string): void {
    const idx = logTabs.value.findIndex(t => t.id === tabId)
    if (idx === -1) return
    logTabs.value.splice(idx, 1)
    if (logTabs.value.length === 0) {
      activeTabId.value = ''
      showLogModal.value = false
      return
    }
    if (activeTabId.value === tabId) {
      const newIdx = Math.min(idx, logTabs.value.length - 1)
      activeTabId.value = logTabs.value[newIdx].id
      syncFromActiveTab()
    }
  }

  function switchTab(tabId: string): void {
    syncToTab(activeTabId.value)
    activeTabId.value = tabId
    syncFromActiveTab()
  }

  function syncFromActiveTab(): void {
    const tab = activeTab.value
    if (!tab) return
    logTitle.value = tab.title
    logContent.value = tab.content
    logCurrentPath.value = tab.filePath
    logIsDocker.value = tab.isDocker
    logTotalLines.value = tab.totalLines
    logTruncated.value = tab.truncated
    logHasMore.value = tab.hasMore
  }

  function syncToTab(tabId: string): void {
    const tab = logTabs.value.find(t => t.id === tabId)
    if (!tab) return
    tab.title = logTitle.value
    tab.content = logContent.value
    tab.filePath = logCurrentPath.value
    tab.isDocker = logIsDocker.value
    tab.totalLines = logTotalLines.value
    tab.truncated = logTruncated.value
    tab.hasMore = logHasMore.value
  }

  async function exportLog(path: string, format: string = 'txt', isDocker: boolean = false): Promise<void> {
    const { setStatus } = useStatusStore()
    const allowedFormats = ['txt', 'json', 'csv']
    const safeFormat = allowedFormats.includes(format) ? format : 'txt'
    setStatus('正在导出日志...', 'loading')
    try {
      const baseApi = isDocker
        ? `/api/docker/export?container=${encodeURIComponent(path)}&format=${encodeURIComponent(safeFormat)}`
        : `/api/log/export?path=${encodeURIComponent(path)}&format=${encodeURIComponent(safeFormat)}`
      const url = `${window.location.origin}${API_BASE}${baseApi}`
      const csrfToken = api.getCSRFToken()
      const response = await fetch(url, {
        credentials: 'include',
        headers: {
          'X-Requested-With': 'XMLHttpRequest',
          ...(csrfToken ? { 'X-CSRF-Token': csrfToken } : {})
        }
      })
      if (!response.ok) {
        throw new Error(`导出失败: HTTP ${response.status}`)
      }
      const blob = await response.blob()
      const disposition = response.headers.get('Content-Disposition')
      let filename = `log_export.${safeFormat}`
      if (disposition) {
        const match = disposition.match(/filename[^;=\n]*=((["']).*?\2|[^;\n]*)/)
        if (match && match[1]) {
          filename = match[1].replace(/["']/g, '')
        }
      }
      const link = document.createElement('a')
      link.href = URL.createObjectURL(blob)
      link.download = filename
      link.click()
      URL.revokeObjectURL(link.href)
      setStatus('日志导出成功', 'success')
    } catch (e) {
      setStatus('导出失败: ' + safeErrorMessage(e), 'error')
    }
  }

  return {
    logList,
    listType,
    showLogModal,
    showCleanModal,
    showSearchModal,
    logContent,
    logTitle,
    filterEnabled,
    logTruncated,
    logHasMore,
    logTotalLines,
    logCurrentPath,
    logIsDocker,
    logTabs,
    activeTabId,
    activeTab,
    loadFilterStatus,
    toggleFilter,
    listLogs,
    searchLogs,
    viewLog,
    loadAllLines,
    truncateLog,
    deleteLog,
    executeClean,
    cleanEmptyDirs,
    exportLog,
    clearList,
    addTab,
    removeTab,
    switchTab
  }
})
