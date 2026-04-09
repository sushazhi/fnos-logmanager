import { defineStore } from 'pinia'
import { ref } from 'vue'
import type { Status, StatusType, ConfirmOptions } from '../types'

type ConfirmFn = (options: ConfirmOptions) => Promise<boolean>

export const useStatusStore = defineStore('status', () => {
  const status = ref<Status>({
    message: '就绪',
    type: 'success'
  })

  let confirmFn: ConfirmFn | null = null

  function setConfirmFn(fn: ConfirmFn): void {
    confirmFn = fn
  }

  function setStatus(message: string, type: StatusType = 'success'): void {
    status.value.message = message
    status.value.type = type
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
    setConfirmFn,
    setStatus,
    confirm
  }
})
