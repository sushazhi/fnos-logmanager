import { ref, reactive } from 'vue'
import api, { getToken, setToken } from '../services/api'

const APP_VERSION = '__APP_VERSION__'
const appVersion = ref(APP_VERSION)
const REPO_OWNER = 'sushazhi'
const REPO_NAME = 'fnos-logmanager'

const IGNORE_KEY = 'logmanager_ignore_version'
const CLOSE_TIME_KEY = 'logmanager_update_close_time'
const CLOSE_DURATION = 24 * 60 * 60 * 1000

let confirmFn = null
export function setConfirmFn(fn) {
  confirmFn = fn
}

const stats = reactive({
  totalLogs: 0,
  totalSize: '0B',
  archiveCount: 0,
  largeFiles: 0
})

const dirs = ref([])
const logList = ref([])
const dockerContainers = ref([])
const status = reactive({
  message: '就绪',
  type: 'success'
})

const filterEnabled = ref(true)
const showTokenModal = ref(false)
const showLogModal = ref(false)
const showCleanModal = ref(false)
const logContent = ref('')
const logTitle = ref('')
const selectedDir = ref(null)
const updateInfo = ref(null)
const listType = ref('logs')

export function useStore() {
  async function loadStats() {
    try {
      const data = await api.get('/api/logs/stats')
      stats.totalLogs = data.totalLogs || 0
      stats.totalSize = data.totalSizeFormatted || '0B'
      stats.archiveCount = data.totalArchives || 0
      stats.largeFiles = data.largeFiles || 0
    } catch (e) {
      console.error('加载统计失败:', e)
    }
  }
  
  async function loadDirs() {
    try {
      const data = await api.get('/api/dirs')
      const dirNames = {
        '/vol1/@appdata': '@appdata',
        '/vol1/@appconf': '@appconf',
        '/vol1/@apphome': '@apphome',
        '/vol1/@apptemp': '@apptemp',
        '/vol1/@appshare': '@appshare',
        '/var/log/apps': '/var/log/apps'
      }
      dirs.value = data.dirs.map(dir => ({
        ...dir,
        displayName: dirNames[dir.path] || dir.path
      }))
    } catch (e) {
      console.error('加载目录失败:', e)
    }
  }
  
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
  
  function setStatus(message, type = 'success') {
    status.message = message
    status.type = type
  }
  
  async function refreshAll() {
    setStatus('正在刷新...', 'loading')
    selectedDir.value = null
    logList.value = []
    await Promise.all([loadStats(), loadDirs()])
    setStatus('刷新完成', 'success')
  }
  
  async function selectDir(dirPath) {
    selectedDir.value = dirPath
    setStatus(`正在加载 ${dirPath} 下的日志...`, 'loading')
    try {
      const data = await api.get(`/api/logs/list?dir=${encodeURIComponent(dirPath)}&limit=200`)
      logList.value = data.logs.map(log => ({
        ...log,
        showActions: true
      }))
      setStatus(`找到 ${data.total} 个日志文件`, 'success')
    } catch (e) {
      setStatus('加载失败: ' + e.message, 'error')
    }
  }
  
  async function listLogs() {
    selectedDir.value = null
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
  
  async function findLargeLogs() {
    selectedDir.value = null
    listType.value = 'logs'
    setStatus('正在查找大日志文件...', 'loading')
    try {
      const data = await api.get('/api/logs/large?threshold=10M&limit=50')
      logList.value = data.logs.map(log => ({
        ...log,
        showActions: true
      }))
      setStatus(`找到 ${data.logs.length} 个大日志文件`, 'success')
    } catch (e) {
      setStatus('查找失败: ' + e.message, 'error')
    }
  }
  
  async function listArchives() {
    selectedDir.value = null
    listType.value = 'archives'
    setStatus('正在查找归档日志...', 'loading')
    try {
      const data = await api.get('/api/archives/list?limit=50')
      logList.value = data.archives.map(a => ({
        path: a.path,
        sizeFormatted: a.sizeFormatted,
        showActions: false
      }))
      setStatus(`找到 ${data.total} 个归档文件`, 'success')
    } catch (e) {
      setStatus('查找失败: ' + e.message, 'error')
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
    if (!confirm('确定要清空此日志文件吗？')) return
    setStatus('正在清空日志...', 'loading')
    try {
      await api.post('/api/log/truncate', { path })
      setStatus('日志已清空', 'success')
      refreshAll()
    } catch (e) {
      setStatus('清空失败: ' + e.message, 'error')
    }
  }
  
  async function deleteLog(path) {
    if (!confirmFn) {
      if (!window.confirm('确定要删除此日志文件吗？此操作不可恢复！')) return
    } else {
      const confirmed = await confirmFn({
        title: '删除日志',
        message: '确定要删除此日志文件吗？此操作不可恢复！',
        type: 'danger',
        confirmText: '删除'
      })
      if (!confirmed) return
    }
    setStatus('正在删除日志文件...', 'loading')
    try {
      await api.post('/api/log/delete', { path })
      setStatus('日志文件已删除', 'success')
      logList.value = logList.value.filter(log => log.path !== path)
      loadStats()
    } catch (e) {
      setStatus('删除失败: ' + e.message, 'error')
    }
  }
  
  async function listDockerContainers() {
    selectedDir.value = null
    listType.value = 'docker'
    setStatus('正在获取Docker容器...', 'loading')
    try {
      const data = await api.get('/api/docker/containers')
      if (data.error) {
        setStatus(data.error, 'warning')
        return
      }
      dockerContainers.value = data.containers
      logList.value = data.containers.map(c => ({
        path: c.name,
        sizeFormatted: c.image || '-',
        isDocker: true
      }))
      setStatus(`找到 ${data.containers.length} 个容器`, 'success')
    } catch (e) {
      setStatus('获取失败: ' + e.message, 'error')
    }
  }
  
  async function viewDockerLogs(container) {
    setStatus('正在获取容器日志...', 'loading')
    try {
      const data = await api.get(`/api/docker/logs?container=${encodeURIComponent(container)}&lines=500`)
      logTitle.value = `Docker: ${container}`
      logContent.value = data.logs || '(无日志)'
      showLogModal.value = true
      setStatus('日志加载完成', 'success')
    } catch (e) {
      setStatus('加载失败: ' + e.message, 'error')
    }
  }
  
  async function viewArchive(path) {
    setStatus('正在加载归档日志...', 'loading')
    try {
      const data = await api.get(`/api/archive/content?path=${encodeURIComponent(path)}`)
      logTitle.value = path
      logContent.value = data.content || '(空文件)'
      showLogModal.value = true
      setStatus('归档日志加载完成', 'success')
    } catch (e) {
      setStatus('加载失败: ' + e.message, 'error')
    }
  }
  
  async function deleteArchive(path) {
    if (!confirmFn) {
      if (!window.confirm('确定要删除此归档文件吗？')) return
    } else {
      const confirmed = await confirmFn({
        title: '删除归档',
        message: '确定要删除此归档文件吗？',
        type: 'danger',
        confirmText: '删除'
      })
      if (!confirmed) return
    }
    setStatus('正在删除归档文件...', 'loading')
    try {
      await api.post('/api/archives/delete', { path })
      setStatus('归档文件已删除', 'success')
      listArchives()
    } catch (e) {
      setStatus('删除失败: ' + e.message, 'error')
    }
  }
  
  async function compressLogs() {
    setStatus('正在压缩日志...', 'loading')
    try {
      const data = await api.post('/api/logs/compress', { threshold: '10M' })
      setStatus(`压缩完成，共处理 ${data.compressed} 个文件`, 'success')
      refreshAll()
    } catch (e) {
      setStatus('压缩失败: ' + e.message, 'error')
    }
  }
  
  async function backupLogs() {
    setStatus('正在备份日志...', 'loading')
    try {
      const data = await api.post('/api/logs/backup')
      setStatus(`备份完成: ${data.backupPath} (${data.backupSize})`, 'success')
      refreshAll()
    } catch (e) {
      setStatus('备份失败: ' + e.message, 'error')
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
      refreshAll()
    } catch (e) {
      setStatus('清理失败: ' + e.message, 'error')
    }
  }
  
  function saveToken(token) {
    setToken(token)
    showTokenModal.value = false
    refreshAll()
  }
  
  function getIgnoredVersion() {
    try {
      return localStorage.getItem(IGNORE_KEY) || ''
    } catch (e) {
      return ''
    }
  }

  function getCloseTime() {
    try {
      const closeTime = localStorage.getItem(CLOSE_TIME_KEY)
      return closeTime ? parseInt(closeTime, 10) : 0
    } catch (e) {
      return 0
    }
  }

  function isRecentlyClosed() {
    return Date.now() - getCloseTime() < CLOSE_DURATION
  }

  async function checkForUpdates() {
    try {
      const response = await fetch(`https://api.github.com/repos/${REPO_OWNER}/${REPO_NAME}/releases/latest`, {
        headers: { 'Accept': 'application/vnd.github.v3+json' },
        cache: 'no-store'
      })
      if (!response.ok) return
      
      const data = await response.json()
      const latestVersion = (data.tag_name || '').replace(/^v/, '')
      
      if (compareVersions(latestVersion, APP_VERSION) > 0) {
        if (getIgnoredVersion() === latestVersion) {
          console.log('[Update] 已忽略版本', latestVersion)
          return
        }

        if (isRecentlyClosed()) {
          console.log('[Update] 24小时内已关闭，跳过通知')
          return
        }

        updateInfo.value = {
          version: latestVersion,
          url: data.html_url || '',
          changelog: data.body || ''
        }
      }
    } catch (e) {
      console.error('检查更新失败:', e)
    }
  }
  
  function compareVersions(v1, v2) {
    const parts1 = v1.split('.').map(Number)
    const parts2 = v2.split('.').map(Number)
    
    for (let i = 0; i < Math.max(parts1.length, parts2.length); i++) {
      const n1 = parts1[i] || 0
      const n2 = parts2[i] || 0
      if (n1 > n2) return 1
      if (n1 < n2) return -1
    }
    return 0
  }
  
  function clearList() {
    logList.value = []
    listType.value = 'logs'
    selectedDir.value = null
  }
  
  return {
    stats,
    dirs,
    logList,
    dockerContainers,
    status,
    filterEnabled,
    showTokenModal,
    showLogModal,
    showCleanModal,
    logContent,
    logTitle,
    selectedDir,
    updateInfo,
    listType,
    appVersion,
    loadStats,
    loadDirs,
    loadFilterStatus,
    toggleFilter,
    setStatus,
    refreshAll,
    selectDir,
    listLogs,
    findLargeLogs,
    listArchives,
    viewLog,
    truncateLog,
    deleteLog,
    listDockerContainers,
    viewDockerLogs,
    viewArchive,
    deleteArchive,
    compressLogs,
    backupLogs,
    executeClean,
    saveToken,
    checkForUpdates,
    clearList
  }
}
