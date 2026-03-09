import { reactive } from 'vue'
import type { Status, StatusType, ConfirmOptions } from '../types'

const status = reactive<Status>({
  message: '就绪',
  type: 'success'
})

type ConfirmFn = (options: ConfirmOptions) => Promise<boolean>

let confirmFn: ConfirmFn | null = null

export function setConfirmFn(fn: ConfirmFn): void {
  confirmFn = fn
}

export function useStatus() {
  function setStatus(message: string, type: StatusType = 'success'): void {
    status.message = message
    status.type = type
  }

  async function confirm(options: ConfirmOptions): Promise<boolean> {
    if (!confirmFn) {
      return window.confirm(options.message || String(options))
    }
    return confirmFn({
      title: options.title || '确认',
      message: options.message || '',
      type: options.type || 'warning',
      confirmText: options.confirmText || '确定'
    })
  }

  return {
    status,
    setStatus,
    confirm
  }
}
