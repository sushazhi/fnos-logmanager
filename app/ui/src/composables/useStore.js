import { useStatus, setConfirmFn } from './useStatus'
import { useStats } from './useStats'
import { useDirs } from './useDirs'
import { useLogs } from './useLogs'
import { useDocker } from './useDocker'
import { useArchive } from './useArchive'
import { useBackup } from './useBackup'
import { useUpdate } from './useUpdate'
import { setCSRFToken, fetchCSRFToken } from '../services/api'

export { setConfirmFn }

export function useStore() {
  const { status, setStatus } = useStatus()
  const { stats, loadStats } = useStats()
  const { dirs, selectedDir, loadDirs, selectDir } = useDirs()
  const {
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
  } = useLogs()
  const { dockerContainers, listDockerContainers, viewDockerLogs } = useDocker()
  const { listArchives, viewArchive, deleteArchive } = useArchive()
  const { backupLogs } = useBackup()
  const { appVersion, updateInfo, checkForUpdates } = useUpdate()

  async function refreshAll() {
    setStatus('正在刷新...', 'loading')
    selectedDir.value = null
    logList.value = []
    await Promise.all([loadStats(), loadDirs()])
    setStatus('刷新完成', 'success')
  }

  async function handleSelectDir(dirPath) {
    const logs = await selectDir(dirPath)
    logList.value = logs
    listType.value = 'logs'
  }

  async function handleListDockerContainers() {
    selectedDir.value = null
    listType.value = 'docker'
    const containers = await listDockerContainers()
    logList.value = containers
  }

  async function handleViewDockerLogs(container) {
    const result = await viewDockerLogs(container)
    if (result) {
      logTitle.value = result.title
      logContent.value = result.content
      showLogModal.value = true
    }
  }

  async function handleListArchives() {
    selectedDir.value = null
    listType.value = 'archives'
    const archives = await listArchives()
    logList.value = archives
  }

  async function handleViewArchive(path) {
    const result = await viewArchive(path)
    if (result) {
      logTitle.value = result.title
      logContent.value = result.content
      showLogModal.value = true
    }
  }

  async function handleDeleteArchive(path) {
    const success = await deleteArchive(path)
    if (success) {
      handleListArchives()
    }
  }

  async function handleBackupLogs() {
    await backupLogs()
    refreshAll()
  }

  async function handleTruncateLog(path) {
    const success = await truncateLog(path)
    if (success) {
      loadStats()
      if (selectedDir.value) {
        handleSelectDir(selectedDir.value)
      } else if (listType.value === 'logs') {
        listLogs()
      }
    }
  }

  async function handleDeleteLog(path) {
    const success = await deleteLog(path)
    if (success) {
      loadStats()
    }
  }

  function saveCSRFToken(csrfToken) {
    setCSRFToken(csrfToken)
    refreshAll()
  }

  return {
    stats,
    dirs,
    logList,
    dockerContainers,
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
    loadStats,
    loadDirs,
    loadFilterStatus,
    toggleFilter,
    setStatus,
    refreshAll,
    selectDir: handleSelectDir,
    listLogs,
    searchLogs,
    listArchives: handleListArchives,
    viewLog,
    truncateLog: handleTruncateLog,
    deleteLog: handleDeleteLog,
    listDockerContainers: handleListDockerContainers,
    viewDockerLogs: handleViewDockerLogs,
    viewArchive: handleViewArchive,
    deleteArchive: handleDeleteArchive,
    backupLogs: handleBackupLogs,
    executeClean,
    saveCSRFToken,
    fetchCSRFToken,
    checkForUpdates,
    clearList
  }
}
