<template>
  <div v-if="isCheckingAuth" class="auth-loading">
    <div class="auth-loading-spinner"></div>
  </div>
  
  <SetupModal v-else-if="!isInitialized" @setup="handleSetup" />
  
  <LoginModal v-else-if="!isLoggedIn" @login="handleLogin" />
  
  <div v-else class="container">
    <AppHeader />
    
    <UpdateNotification 
      v-if="updateInfo" 
      :update-info="updateInfo" 
      :current-version="appVersion"
      @close="updateInfo = null"
    />
    
    <StatsCard :stats="stats" />
    
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
      @backup="backupLogs"
      @list-archives="listArchives"
      @list-docker="listDockerContainers"
      @toggle-filter="toggleFilter"
      @open-settings="showSettings = true"
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
      @close="showLogModal = false"
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
    />
    
    <AuditLog 
      v-if="showAuditLog"
      @close="showAuditLog = false"
    />
    
    <ConfirmDialog ref="confirmDialog" />
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useStore, setConfirmFn } from './composables/useStore'
import api from './services/api'
import AppHeader from './components/AppHeader.vue'
import StatsCard from './components/StatsCard.vue'
import DirsCard from './components/DirsCard.vue'
import ActionsCard from './components/ActionsCard.vue'
import LogListCard from './components/LogListCard.vue'
import AppFooter from './components/AppFooter.vue'
import LogModal from './components/LogModal.vue'
import CleanModal from './components/CleanModal.vue'
import SearchModal from './components/SearchModal.vue'
import UpdateNotification from './components/UpdateNotification.vue'
import SettingsModal from './components/SettingsModal.vue'
import LoginModal from './components/LoginModal.vue'
import SetupModal from './components/SetupModal.vue'
import ConfirmDialog from './components/ConfirmDialog.vue'
import AuditLog from './components/AuditLog.vue'

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
  viewArchive,
  deleteArchive,
  deleteLog,
  truncateLog,
  listDockerContainers,
  viewDockerLogs,
  backupLogs,
  executeClean,
  toggleFilter,
  checkForUpdates,
  clearList
} = useStore()

const showSettings = ref(false)
const isLoggedIn = ref(false)
const isInitialized = ref(true)
const isCheckingAuth = ref(true)
const confirmDialog = ref(null)
const showAuditLog = ref(false)

async function showConfirm(options) {
  if (!confirmDialog.value) return false
  if (typeof options === 'string') {
    options = { message: options }
  }
  return confirmDialog.value.show(options)
}

setConfirmFn(showConfirm)

function handleLogin(csrfToken) {
  isLoggedIn.value = true
  loadFilterStatus()
  refreshAll()
  checkForUpdates()
}

function handleSetup() {
  isInitialized.value = true
}

async function checkAuth() {
  isCheckingAuth.value = true
  
  try {
    const data = await api.get('/api/auth/status')
    isInitialized.value = data.initialized
    
    if (data.isLoggedIn) {
      isLoggedIn.value = true
      if (!api.getCSRFToken()) {
        await api.fetchCSRFToken()
      }
      loadFilterStatus()
      refreshAll()
      checkForUpdates()
    } else {
      isLoggedIn.value = false
    }
  } catch (e) {
    console.error('认证检查失败:', e)
    isLoggedIn.value = false
  } finally {
    isCheckingAuth.value = false
  }
}

function loadSavedSettings() {
  try {
    const saved = localStorage.getItem('logmanager_settings')
    if (saved) {
      const settings = JSON.parse(saved)  // 添加错误处理
      const root = document.documentElement
      
      if (settings.fontSize) {
        root.style.setProperty('--base-font-size', `${settings.fontSize}px`)
      }
      
      if (settings.primaryColor) {
        root.style.setProperty('--primary-color', settings.primaryColor)
      }
      
      if (settings.theme) {
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
