import { ref } from 'vue'
import api from '../services/api'
import { useStatus } from './useStatus'

const logList = ref([])
const listType = ref('logs')
const showLogModal = ref(false)
const showCleanModal = ref(false)
const showSearchModal = ref(false)
const logContent = ref('')
const logTitle = ref('')
const filterEnabled = ref(true)

export function useLogs() {
  const { setStatus, confirm } = useStatus()

  async function loadFilterStatus() {
    try {
      const data = await api.get('/api/settings/filter')
      filterEnabled.value = data.enabled !== false
    } catch (e) {
      filterEnabled.value = true
    }
  }

  async function toggleFilter() {
    filterEnabled.value = !filterEnabled.value
    try {
      await api.post('/api/settings/filter', { enabled: filterEnabled.value })
      setStatus(filterEnabled.value ? '敏感信息过滤已启用' : '敏感信息过滤已禁用', 'success')
    } catch (e) {
      setStatus('设置保存失败: ' + e.message, 'error')
    }
  }

  async function listLogs() {
    listType.value = 'logs'
    setStatus('正在列出日志文件...', 'loading')
    try {
      const data = await api.get('/api/logs/list?limit=100')
      logList.value = data.logs.map(log => ({
        ...log,
        showActions: true
      }))
      setStatus(`找到 ${data.total} 个日志文件`, 'success')
    } catch (e) {
      setStatus('列出日志失败: ' + e.message, 'error')
    }
  }

  async function searchLogs(type, threshold, pattern) {
    listType.value = 'logs'
    showSearchModal.value = false
    
    if (type === 'size') {
      setStatus('正在查找大日志文件...', 'loading')
      try {
        const data = await api.get(`/api/logs/search?type=size&threshold=${encodeURIComponent(threshold)}&limit=50`)
        logList.value = data.logs.map(log => ({
          ...log,
          showActions: true
        }))
        setStatus(`找到 ${data.logs.length} 个大日志文件`, 'success')
      } catch (e) {
        setStatus('查找失败: ' + e.message, 'error')
      }
    } else if (type === 'name') {
      setStatus('正在按名称查找日志文件...', 'loading')
      try {
        const data = await api.get(`/api/logs/search?type=name&pattern=${encodeURIComponent(pattern)}&limit=50`)
        logList.value = data.logs.map(log => ({
          ...log,
          showActions: true
        }))
        setStatus(`找到 ${data.total} 个匹配的日志文件`, 'success')
      } catch (e) {
        setStatus('查找失败: ' + e.message, 'error')
      }
    }
  }

  async function viewLog(path) {
    setStatus('正在加载日志内容...', 'loading')
    try {
      const data = await api.get(`/api/log/content?path=${encodeURIComponent(path)}`)
      logTitle.value = path
      logContent.value = data.content || '(空文件)'
      showLogModal.value = true
      setStatus('日志加载完成', 'success')
    } catch (e) {
      setStatus('加载失败: ' + e.message, 'error')
    }
  }

  async function truncateLog(path) {
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
      setStatus('清空失败: ' + e.message, 'error')
      return false
    }
  }

  async function deleteLog(path) {
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
      setStatus('删除失败: ' + e.message, 'error')
      return false
    }
  }

  async function executeClean(type, threshold, days) {
    showCleanModal.value = false
    setStatus('正在清理日志...', 'loading')
    try {
      const data = await api.post('/api/logs/clean', {
        type,
        threshold,
        days: type === 'deleteOld' ? days : null,
        action: type === 'deleteOld' ? 'delete' : type
      })
      setStatus(`清理完成，共处理 ${data.cleaned} 个文件`, 'success')
    } catch (e) {
      setStatus('清理失败: ' + e.message, 'error')
    }
  }

  function clearList() {
    logList.value = []
    listType.value = 'logs'
    setStatus('就绪', 'success')
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
    loadFilterStatus,
    toggleFilter,
    listLogs,
    searchLogs,
    viewLog,
    truncateLog,
    deleteLog,
    executeClean,
    clearList
  }
}
