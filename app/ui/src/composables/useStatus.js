import { reactive } from 'vue'

const status = reactive({
  message: '就绪',
  type: 'success'
})

let confirmFn = null

export function setConfirmFn(fn) {
  confirmFn = fn
}

export function useStatus() {
  function setStatus(message, type = 'success') {
    status.message = message
    status.type = type
  }

  async function confirm(options) {
    if (!confirmFn) {
      return window.confirm(options.message || options)
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
