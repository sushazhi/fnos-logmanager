<template>
  <div v-if="isCheckingAuth" class="auth-loading">
    <div class="auth-shimmer">
      <div class="shimmer-header hm-shimmer"></div>
      <div class="shimmer-cards">
        <div class="shimmer-card hm-shimmer hm-shimmer-card"></div>
        <div class="shimmer-card hm-shimmer hm-shimmer-card"></div>
        <div class="shimmer-card hm-shimmer hm-shimmer-card"></div>
        <div class="shimmer-card hm-shimmer hm-shimmer-card"></div>
      </div>
      <div class="shimmer-list">
        <div class="shimmer-row hm-shimmer hm-shimmer-text"></div>
        <div class="shimmer-row hm-shimmer hm-shimmer-text short"></div>
        <div class="shimmer-row hm-shimmer hm-shimmer-text"></div>
        <div class="shimmer-row hm-shimmer hm-shimmer-text short"></div>
      </div>
    </div>
  </div>
  
  <div v-else class="container">
    <AppHeader />
    
    <UpdateNotification 
      v-if="updateInfo" 
      :update-info="updateInfo" 
      :current-version="appVersion"
      @close="updateInfo = null"
    />
    
    <StatsCard :stats="stats" />
    
    <BookmarkBar 
      :bookmarks="bookmarks"
      :current-path="logCurrentPath"
      @open-bookmark="handleOpenBookmark"
      @delete-bookmark="handleDeleteBookmark"
      @add-bookmark="handleAddBookmark"
    />
    
    <DirsCard 
      :dirs="dirs"
      :selected-dir="selectedDir"
      @select-dir="selectDir"
    />
    
    <ActionsCard 
      :status="status"
      :filter-enabled="filterEnabled"
      @refresh="refreshAll"
      @list-logs="listLogs"
      @show-search="showSearchModal = true"
      @show-clean="showCleanModal = true"
      @show-uninstalled-clean="showUninstalledCleanModal = true"
      @backup="backupLogs"
      @list-archives="listArchives"
      @list-docker="listDockerContainers"
      @toggle-filter="toggleFilter"
      @open-settings="showSettings = true"
      @show-notification="showNotification = true"
      @show-event-logger="showEventLogger = true"
      @show-auto-clean="showAutoClean = true"
      @show-kernel-modules="showKernelModules = true"
      @show-processes="showProcesses = true"
    />
    
    <AppFooter />
    
    <LogListCard 
      :logs="logList"
      :type="listType"
      @view="viewLog"
      @truncate="truncateLog"
      @view-docker="viewDockerLogs"
      @view-archive="viewArchive"
      @delete="deleteArchive"
      @delete-log="deleteLog"
      @close="clearList"
    />
    
    <LogModal 
      v-if="showLogModal"
      :title="logTitle"
      :content="logContent"
      :truncated="logTruncated"
      :total-lines-in-file="logTotalLines"
      :loading-all="loadingAllLines"
      :is-docker="logIsDocker"
      :container-name="logCurrentPath"
      :file-path="logCurrentPath"
      :bookmarks="bookmarks"
      @close="handleCloseLogModal"
      @back="showLogModal = false"
      @load-all="handleLoadAllLines"
      @export="handleExportLog"
      @toggle-bookmark="handleLogModalToggleBookmark"
      @truncate="handleLogModalTruncate"
    />
    
    <CleanModal 
      v-if="showCleanModal"
      @close="showCleanModal = false"
      @execute="executeClean"
    />
    
    <UninstalledCleanModal 
      v-if="showUninstalledCleanModal"
      @close="showUninstalledCleanModal = false"
      @clean-empty="cleanEmptyDirs"
      @clean-trash="cleanUninstalledDirs"
    />
    
    <SearchModal 
      v-if="showSearchModal"
      @close="showSearchModal = false"
      @execute="searchLogs"
    />
    
    <SettingsModal 
      v-if="showSettings"
      @close="showSettings = false"
      @show-audit="showSettings = false; showAuditLog = true"
      @show-notification="showSettings = false; showNotification = true"
    />
    
    <AuditLog 
      v-if="showAuditLog"
      @close="showAuditLog = false"
    />
    
    <NotificationPanel
      v-if="showNotification"
      @close="showNotification = false"
    />
    
    <EventLoggerPanel
      v-if="showEventLogger"
      @close="showEventLogger = false"
    />
    
    <AutoCleanPanel
      v-if="showAutoClean"
      @close="showAutoClean = false"
    />
    
    <KernelModulePanel
      v-if="showKernelModules"
      @close="showKernelModules = false"
    />
    
    <ProcessPanel
      v-if="showProcesses"
      @close="showProcesses = false"
    />
    
    <ConfirmDialog ref="confirmDialog" />
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted } from 'vue'
import { useStore, setConfirmFn } from './composables/useStore'
import { useLogsStore } from './stores/useLogsStore'
import { applyThemeColor } from './composables/useThemeColor'
import api, { bookmarkApi, API_BASE } from './services/api'
import { setTitle, onThemeChange, onLanguageChange, waitForReady, getHostSnapshot, getPlatformConfig, setBackendApiBase } from './services/fnos'
import AppHeader from './components/AppHeader.vue'
import StatsCard from './components/StatsCard.vue'
import BookmarkBar from './components/BookmarkBar.vue'
import DirsCard from './components/DirsCard.vue'
import ActionsCard from './components/ActionsCard.vue'
import LogListCard from './components/LogListCard.vue'
import AppFooter from './components/AppFooter.vue'
import LogModal from './components/LogModal.vue'
import CleanModal from './components/CleanModal.vue'
import UninstalledCleanModal from './components/UninstalledCleanModal.vue'
import SearchModal from './components/SearchModal.vue'
import UpdateNotification from './components/UpdateNotification.vue'
import SettingsModal from './components/SettingsModal.vue'
import ConfirmDialog from './components/ConfirmDialog.vue'
import AuditLog from './components/AuditLog.vue'
import NotificationPanel from './components/NotificationPanel.vue'
import EventLoggerPanel from './components/EventLoggerPanel.vue'
import AutoCleanPanel from './components/AutoCleanPanel.vue'
import KernelModulePanel from './components/KernelModulePanel.vue'
import ProcessPanel from './components/ProcessPanel.vue'

const {
  stats,
  dirs,
  logList,
  status,
  setStatus,
  filterEnabled,
  showLogModal,
  showCleanModal,
  showUninstalledCleanModal,
  showSearchModal,
  logContent,
  logTitle,
  logTruncated,
  logTotalLines,
  logCurrentPath,
  logIsDocker,
  activeTabId,
  logTabs,
  selectedDir,
  updateInfo,
  listType,
  appVersion,
  loadFilterStatus,
  loadDirs,
  refreshAll,
  selectDir,
  listLogs,
  searchLogs,
  listArchives,
  viewLog,
  loadAllLines,
  viewArchive,
  deleteArchive,
  deleteLog,
  truncateLog,
  listDockerContainers,
  viewDockerLogs,
  backupLogs,
  removeTab,
  switchTab,
  executeClean,
  cleanEmptyDirs,
  cleanUninstalledDirs,
  exportLog,
  toggleFilter,
  checkForUpdates,
  clearList
} = useStore()

const showSettings = ref(false)
const isCheckingAuth = ref(true)
const confirmDialog = ref(null)
const showAuditLog = ref(false)
const showNotification = ref(false)
const showEventLogger = ref(false)
const showAutoClean = ref(false)
const showKernelModules = ref(false)
const showProcesses = ref(false)
const loadingAllLines = ref(false)
const bookmarks = ref([])

// P3: fnOS page interaction cleanup functions
const themeCleanupRef = ref(null)
const languageCleanupRef = ref(null)

async function loadBookmarks() {
  try {
    const data = await bookmarkApi.getAll()
    bookmarks.value = data.bookmarks || []
  } catch (e) {
    console.error('加载书签失败:', e)
  }
}

async function handleOpenBookmark(bookmark) {
  if (bookmark.isDocker) {
    viewDockerLogs(bookmark.path)
  } else {
    viewLog(bookmark.path)
  }
  // 从书签直接打开详情页时，清理残留的背景列表状态（如之前浏览目录留下的
  // selectedDir / logList），避免后续在详情页内点击"清空"时无端刷新并弹出结果列表抽屉。
  selectedDir.value = null
  logList.value = []
}

async function handleDeleteBookmark(bookmark) {
  const id = bookmark.id
  const path = bookmark.path
  const isDocker = bookmark.isDocker
  try {
    await bookmarkApi.delete(id, path, isDocker)
    bookmarks.value = bookmarks.value.filter(b => b.id !== id && !(b.path === path && (b.isDocker || false) === (isDocker || false)))
    setStatus('已移除书签', 'success')
  } catch (e) {
    setStatus('移除书签失败: ' + (e?.message || e), 'error')
  }
}

async function handleAddBookmark(data) {
  try {
    const result = await bookmarkApi.add(data)
    bookmarks.value.push(result.bookmark)
  } catch (e) {
    console.error('添加书签失败:', e)
  }
}

function handleCloseLogModal() {
  const logsStore = useLogsStore()
  logsStore.logTabs.splice(0)
  logsStore.activeTabId = ''
  showLogModal.value = false
}

async function handleLogModalToggleBookmark() {
  if (!logCurrentPath.value) return
  const pathVal = logCurrentPath.value
  const isDocker = logIsDocker.value
  const existing = bookmarks.value.find(b => b.path === pathVal && (b.isDocker || false) === isDocker)
  if (existing) {
    try {
      await bookmarkApi.delete(existing.id, existing.path, existing.isDocker)
      bookmarks.value = bookmarks.value.filter(b => b.id !== existing.id && !(b.path === existing.path && (b.isDocker || false) === (existing.isDocker || false)))
      setStatus('已移除书签', 'success')
    } catch (e) {
      setStatus('移除书签失败: ' + (e?.message || e), 'error')
    }
    return
  }
  const name = pathVal.split('/').pop() || pathVal
  try {
    const result = await bookmarkApi.add({ path: pathVal, name, isDocker })
    // Avoid pushing duplicates (server is idempotent: returns existing bookmark if already present)
    if (!bookmarks.value.some(b => b.id === result.bookmark.id)) {
      bookmarks.value.push(result.bookmark)
    }
    setStatus('已添加书签', 'success')
  } catch (e) {
    setStatus('添加书签失败: ' + (e?.message || e), 'error')
  }
}

async function handleLogModalTruncate() {
  if (!logCurrentPath.value) return
  const logsStore = useLogsStore()
  // 注意：这里必须用底层的 logsStore.truncateLog，而不是 useStore 导出的
  // truncateLog（即 handleTruncateLog）。后者专为结果列表的清空按钮设计，
  // 会在 selectedDir 为空时自动调用 listLogs() 填充 logList，导致从书签打开
  // 详情页后点击清空时无端弹出结果列表抽屉。这里由本函数自行控制刷新。
  const ok = await logsStore.truncateLog(logCurrentPath.value)
  if (ok) {
    await logsStore.reloadActiveTab()
    loadDirs()
    // 清空后同步刷新底层结果列表（抽屉），避免其继续显示被清空文件的旧大小/旧条目，
    // 保持详情页背后的列表数据一致。若本来就没有列表（如从书签直接打开详情页），则不刷新，
    // 以免无端弹出结果列表抽屉。
    if (selectedDir.value) {
      logList.value = await selectDir(selectedDir.value)
    } else if (logList.value.length > 0) {
      await listLogs()
    }
  }
}

async function handleLoadAllLines() {
  loadingAllLines.value = true
  try {
    await loadAllLines()
  } finally {
    loadingAllLines.value = false
  }
}

function handleExportLog(format) {
  if (logCurrentPath.value) {
    exportLog(logCurrentPath.value, format, logIsDocker.value)
  }
}

async function showConfirm(options) {
  if (!confirmDialog.value) return false
  if (typeof options === 'string') {
    options = { message: options }
  }
  return confirmDialog.value.show(options)
}

setConfirmFn(showConfirm)

async function checkAuth() {
  isCheckingAuth.value = true
  
  try {
    const data = await api.get('/api/auth/status')
    
    if (data.csrfToken) {
      api.setCSRFToken(data.csrfToken)
    }
    if (data.sessionToken) {
      api.setSessionToken(data.sessionToken)
    }
    if (!data.initialized || !data.isLoggedIn) {
      isCheckingAuth.value = false
      return
    }
    if (!api.getCSRFToken() && !data.isLoggedIn) {
      await api.fetchCSRFToken()
    }
    loadFilterStatus()
    refreshAll()
    checkForUpdates().catch(() => {})
    loadBookmarks()
  } catch (e) {
    console.error('认证检查失败:', e)
  } finally {
    isCheckingAuth.value = false
  }
}

function loadSavedSettings() {
  try {
    const saved = localStorage.getItem('logmanager_settings')
    if (saved) {
      const settings = JSON.parse(saved)
      const root = document.documentElement
      const validColorRegex = /^#[0-9a-fA-F]{6}$/
      
      if (typeof settings.fontSize === 'number' && settings.fontSize >= 10 && settings.fontSize <= 24) {
        root.style.setProperty('--base-font-size', `${settings.fontSize}px`)
      }
      
      // 先应用主题模式（dark-theme 类），再应用主题色，
      // 确保 applyThemeColor 能读到正确的亮/暗模式并生成对应的卡片色
      if (settings.theme === 'dark' || settings.theme === 'light' || settings.theme === 'auto') {
        const isDark = settings.theme === 'dark' || 
          (settings.theme === 'auto' && window.matchMedia('(prefers-color-scheme: dark)').matches)
        if (isDark) {
          root.classList.add('dark-theme')
        } else {
          root.classList.remove('dark-theme')
        }
      }
      
      if (typeof settings.primaryColor === 'string' && validColorRegex.test(settings.primaryColor)) {
        applyThemeColor(settings.primaryColor)
      }
    }
  } catch (e) {
    console.warn('Failed to load settings:', e)
    localStorage.removeItem('logmanager_settings')
  }
}

// 读取已保存的自定义主题色并重新应用（含亮/暗模式下的卡片色重算）
function reapplySavedPrimaryColor() {
  try {
    const saved = localStorage.getItem('logmanager_settings')
    if (!saved) return
    const settings = JSON.parse(saved)
    if (typeof settings.primaryColor === 'string' && /^#[0-9a-fA-F]{6}$/.test(settings.primaryColor)) {
      applyThemeColor(settings.primaryColor)
    }
  } catch {
    // ignore
  }
}

onMounted(async () => {
  loadSavedSettings()

  // 设置后端 API 地址（供 fnos.ts 中 convertPathViaBackend 等方法使用）
  setBackendApiBase(API_BASE)

  // P0: 等待 SDK 就绪并获取用户信息
  await waitForReady()
  getHostSnapshot().then(snapshot => {
    if (snapshot) {
      console.log('[fnOS] 用户:', snapshot.username, '会话数:', snapshot.sessions?.length)
    }
  }).catch(() => {})

  // P0: 初始化时读取平台配置（主题/语言/系统版本），避免首屏主题闪烁
  getPlatformConfig().then(config => {
    if (!config) return
    const root = document.documentElement
    if (config.theme === 'dark') {
      root.classList.add('dark-theme')
    } else {
      root.classList.remove('dark-theme')
    }
    // 系统主题就绪后重新应用主题色，确保卡片色随亮/暗模式正确
    reapplySavedPrimaryColor()
    try {
      localStorage.setItem('logmanager_lang', config.language || 'zh-CN')
    } catch { /* ignore */ }
  }).catch(() => {})
  
  checkAuth()
  
  // P3: Set page title via fnOS SDK
  setTitle('飞牛日志管理').catch(() => {})
  
  // P3: Listen for fnOS theme changes
  themeCleanupRef.value = onThemeChange((theme) => {
    const root = document.documentElement
    if (theme === 'dark') {
      root.classList.add('dark-theme')
    } else {
      root.classList.remove('dark-theme')
    }
    // 重新应用主题色，使莫兰迪渐变卡片色随亮/暗模式重算（夜间深底/亮色浅底）
    reapplySavedPrimaryColor()
  })
  
  // P3: Listen for fnOS language changes
  languageCleanupRef.value = onLanguageChange((lang) => {
    // Store language preference for future use
    try {
      localStorage.setItem('logmanager_lang', lang)
    } catch { /* ignore */ }
  })
})

onUnmounted(() => {
  if (themeCleanupRef.value) themeCleanupRef.value()
  if (languageCleanupRef.value) languageCleanupRef.value()
})
</script>
