<template>
  <LoginModal v-if="!isLoggedIn" @login="handleLogin" />
  
  <div v-else class="container">
    <AppHeader @open-settings="showSettings = true" />
    
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
      @find-large="findLargeLogs"
      @show-clean="showCleanModal = true"
      @compress="compressLogs"
      @backup="backupLogs"
      @list-archives="listArchives"
      @list-docker="listDockerContainers"
      @toggle-filter="toggleFilter"
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
import UpdateNotification from './components/UpdateNotification.vue'
import SettingsModal from './components/SettingsModal.vue'
import LoginModal from './components/LoginModal.vue'
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
  findLargeLogs,
  listArchives,
  viewLog,
  viewArchive,
  deleteArchive,
  deleteLog,
  truncateLog,
  listDockerContainers,
  viewDockerLogs,
  compressLogs,
  backupLogs,
  executeClean,
  toggleFilter,
  checkForUpdates,
  clearList
} = useStore()

const showSettings = ref(false)
const isLoggedIn = ref(false)
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

function handleLogin(token) {
  isLoggedIn.value = true
  refreshAll()
  checkForUpdates()
}

async function checkAuth() {
  const token = api.getToken()
  if (!token) {
    isLoggedIn.value = false
    return
  }
  
  try {
    await api.get('/api/dirs')
    isLoggedIn.value = true
    refreshAll()
    checkForUpdates()
  } catch (e) {
    isLoggedIn.value = false
  }
}

function loadSavedSettings() {
  try {
    const saved = localStorage.getItem('logmanager_settings')
    if (saved) {
      const settings = JSON.parse(saved)
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
  } catch (e) {}
}

onMounted(() => {
  loadSavedSettings()
  loadFilterStatus()
  checkAuth()
})
</script>
