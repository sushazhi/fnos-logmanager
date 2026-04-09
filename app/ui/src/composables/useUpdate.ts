/**
 * useUpdate - 代理到 Pinia useUpdateStore
 */
import { useUpdateStore } from '../stores/useUpdateStore'

export function useUpdate() {
  const store = useUpdateStore()
  return {
    appVersion: store.appVersion,
    updateInfo: store.updateInfo,
    updateStatus: store.updateStatus,
    checkForUpdates: store.checkForUpdates,
    installUpdate: store.installUpdate,
    getUpdateStatus: store.getUpdateStatus,
    ignoreVersion: store.ignoreVersion,
    setCloseTime: store.setCloseTime
  }
}
