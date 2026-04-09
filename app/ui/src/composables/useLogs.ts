/**
 * useLogs - 代理到 Pinia useLogsStore
 */
import { useLogsStore } from '../stores/useLogsStore'

export function useLogs() {
  const store = useLogsStore()
  return {
    logList: store.logList,
    listType: store.listType,
    showLogModal: store.showLogModal,
    showCleanModal: store.showCleanModal,
    showSearchModal: store.showSearchModal,
    logContent: store.logContent,
    logTitle: store.logTitle,
    filterEnabled: store.filterEnabled,
    loadFilterStatus: store.loadFilterStatus,
    toggleFilter: store.toggleFilter,
    listLogs: store.listLogs,
    searchLogs: store.searchLogs,
    viewLog: store.viewLog,
    truncateLog: store.truncateLog,
    deleteLog: store.deleteLog,
    executeClean: store.executeClean,
    cleanEmptyDirs: store.cleanEmptyDirs,
    clearList: store.clearList
  }
}
