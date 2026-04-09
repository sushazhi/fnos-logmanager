import type { LogItem } from '../types'
import { setConfirmFn } from './useStatus'
import { setCSRFToken, fetchCSRFToken } from '../services/api'
import { storeToRefs } from 'pinia'
import { useStatusStore } from '../stores/useStatusStore'
import { useStatsStore } from '../stores/useStatsStore'
import { useDirsStore } from '../stores/useDirsStore'
import { useLogsStore } from '../stores/useLogsStore'
import { useDockerStore } from '../stores/useDockerStore'
import { useUpdateStore } from '../stores/useUpdateStore'
import { useArchive } from './useArchive'
import { useBackup } from './useBackup'

export { setConfirmFn }

export function useStore() {
  // 直接使用 Pinia stores，通过 storeToRefs 保持响应性
  const statusStore = useStatusStore()
  const statsStore = useStatsStore()
  const dirsStore = useDirsStore()
  const logsStore = useLogsStore()
  const dockerStore = useDockerStore()
  const updateStore = useUpdateStore()

  const { status } = storeToRefs(statusStore)
  const { stats } = storeToRefs(statsStore)
  const { dirs, selectedDir } = storeToRefs(dirsStore)
  const { logList, listType, showLogModal, showCleanModal, showSearchModal, logContent, logTitle, filterEnabled, logTruncated, logHasMore, logTotalLines } = storeToRefs(logsStore)
  const { dockerContainers } = storeToRefs(dockerStore)
  const { updateInfo, appVersion } = storeToRefs(updateStore)

  const { setStatus, confirm } = statusStore
  const { loadStats } = statsStore
  const { loadDirs, selectDir } = dirsStore
  const { listLogs, searchLogs, viewLog, loadAllLines, truncateLog, deleteLog, executeClean, cleanEmptyDirs, clearList, loadFilterStatus, toggleFilter } = logsStore
  const { listDockerContainers, viewDockerLogs } = dockerStore
  const { checkForUpdates } = updateStore

  const { listArchives, viewArchive, deleteArchive } = useArchive()
  const { backupLogs } = useBackup()

  async function refreshAll(): Promise<void> {
    setStatus('正在刷新...', 'loading')
    selectedDir.value = null
    logList.value = []
    await Promise.all([loadStats(), loadDirs()])
    setStatus('刷新完成', 'success')
  }

  async function handleSelectDir(dirPath: string): Promise<LogItem[]> {
    const logs = await selectDir(dirPath)
    logList.value = logs
    listType.value = 'logs'
    return logs
  }

  async function handleListDockerContainers(): Promise<void> {
    selectedDir.value = null
    listType.value = 'docker'
    const containers = await listDockerContainers()
    logList.value = containers
  }

  async function handleViewDockerLogs(container: string): Promise<void> {
    const result = await viewDockerLogs(container)
    if (result) {
      logTitle.value = result.title
      logContent.value = result.content
      showLogModal.value = true
    }
  }

  async function handleListArchives(): Promise<void> {
    selectedDir.value = null
    listType.value = 'archives'
    const archives = await listArchives()
    logList.value = archives
  }

  async function handleViewArchive(path: string): Promise<void> {
    const result = await viewArchive(path)
    if (result) {
      logTitle.value = result.title
      logContent.value = result.content
      showLogModal.value = true
    }
  }

  async function handleDeleteArchive(path: string): Promise<void> {
    const success = await deleteArchive(path)
    if (success) {
      handleListArchives()
    }
  }

  async function handleBackupLogs(): Promise<void> {
    await backupLogs()
    refreshAll()
  }

  async function handleTruncateLog(path: string): Promise<void> {
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

  async function handleDeleteLog(path: string): Promise<void> {
    const success = await deleteLog(path)
    if (success) {
      loadStats()
    }
  }

  function saveCSRFToken(csrfToken: string): void {
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
    logTruncated,
    logHasMore,
    logTotalLines,
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
    loadAllLines,
    truncateLog: handleTruncateLog,
    deleteLog: handleDeleteLog,
    listDockerContainers: handleListDockerContainers,
    viewDockerLogs: handleViewDockerLogs,
    viewArchive: handleViewArchive,
    deleteArchive: handleDeleteArchive,
    backupLogs: handleBackupLogs,
    executeClean,
    cleanEmptyDirs,
    saveCSRFToken,
    fetchCSRFToken,
    checkForUpdates,
    clearList
  }
}
