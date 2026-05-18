<template>
  <div v-if="isCheckingAuth" class="auth-loading">
    <div class="auth-loading-spinner"></div>
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
      @clean-empty-dirs="cleanEmptyDirs"
      @backup="backupLogs"
      @list-archives="listArchives"
      @list-docker="listDockerContainers"
      @toggle-filter="toggleFilter"
      @open-settings="showSettings = true"
      @show-notification="showNotification = true"
      @show-event-logger="showEventLogger = true"
      @show-auto-clean="showAutoClean = true"
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
      @close="handleCloseLogModal"
      @back="showLogModal = false"
      @load-all="handleLoadAllLines"
      @export="handleExportLog"
      @add-bookmark="handleLogModalAddBookmark"
    />
    
    <CleanModal 
      v-if="showCleanModal"
      @close="showCleanModal = false"
      @execute="executeClean"
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
    
    <ConfirmDialog ref="confirmDialog" />
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useStore, setConfirmFn } from './composables/useStore'
import { useLogsStore } from './stores/useLogsStore'
import { applyThemeColor } from './composables/useThemeColor'
import api from './services/api'
import { bookmarkApi } from './services/api'
import AppHeader from './components/AppHeader.vue'
import StatsCard from './components/StatsCard.vue'
import BookmarkBar from './components/BookmarkBar.vue'
import DirsCard from './components/DirsCard.vue'
import ActionsCard from './components/ActionsCard.vue'
import LogListCard from './components/LogListCard.vue'
import AppFooter from './components/AppFooter.vue'
import LogModal from './components/LogModal.vue'
import CleanModal from './components/CleanModal.vue'
import SearchModal from './components/SearchModal.vue'
import UpdateNotification from './components/UpdateNotification.vue'
import SettingsModal from './components/SettingsModal.vue'
import ConfirmDialog from './components/ConfirmDialog.vue'
import AuditLog from './components/AuditLog.vue'
import NotificationPanel from './components/NotificationPanel.vue'
import EventLoggerPanel from './components/EventLoggerPanel.vue'
import AutoCleanPanel from './components/AutoCleanPanel.vue'

const {
  stats,
  dirs,
  logList,
  status,
  filterEnabled,
  showLogModal,
  showCleanModal,
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
const loadingAllLines = ref(false)
const bookmarks = ref([])

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
}

async function handleDeleteBookmark(id) {
  try {
    await bookmarkApi.delete(id)
    bookmarks.value = bookmarks.value.filter(b => b.id !== id)
  } catch (e) {
    console.error('删除书签失败:', e)
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

async function handleLogModalAddBookmark() {
  if (!logCurrentPath.value) return
  const pathVal = logCurrentPath.value
  const name = pathVal.split('/').pop() || pathVal
  const isDocker = logIsDocker.value
  try {
    const result = await bookmarkApi.add({ path: pathVal, name, isDocker })
    bookmarks.value.push(result.bookmark)
  } catch (e) {
    console.error('添加书签失败:', e)
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
    checkForUpdates()
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
      
      if (typeof settings.primaryColor === 'string' && validColorRegex.test(settings.primaryColor)) {
        applyThemeColor(settings.primaryColor)
      }
      
      if (settings.theme === 'dark' || settings.theme === 'light' || settings.theme === 'auto') {
        const isDark = settings.theme === 'dark' || 
          (settings.theme === 'auto' && window.matchMedia('(prefers-color-scheme: dark)').matches)
        if (isDark) {
          root.classList.add('dark-theme')
        }
      }
    }
  } catch (e) {
    console.warn('Failed to load settings:', e)
    localStorage.removeItem('logmanager_settings')
  }
}

onMounted(() => {
  loadSavedSettings()
  checkAuth()
})
</script>
