/**
 * useStatus - 代理到 Pinia useStatusStore
 * 保持向后兼容的 API
 */
import { useStatusStore } from '../stores/useStatusStore'

export function setConfirmFn(fn: (options: any) => Promise<boolean>): void {
  useStatusStore().setConfirmFn(fn)
}

export function useStatus() {
  const store = useStatusStore()
  return {
    status: store.status,
    setStatus: store.setStatus,
    confirm: store.confirm
  }
}
