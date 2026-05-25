/**
 * Toast 通知工具
 * 简单的 DOM 消息提示
 */

const TOAST_DURATION = 3000

interface ToastOptions {
  duration?: number
}

let toastContainer: HTMLDivElement | null = null

function getContainer(): HTMLDivElement {
  if (!toastContainer) {
    toastContainer = document.createElement('div')
    toastContainer.id = 'toast-container'
    toastContainer.style.cssText =
      'position:fixed;top:16px;right:16px;z-index:10000;display:flex;flex-direction:column;gap:8px;pointer-events:none'
    document.body.appendChild(toastContainer)
  }
  return toastContainer
}

export function showNotification(message: string, type: 'info' | 'error' | 'success' | 'warning' = 'info', options?: ToastOptions) {
  const container = getContainer()
  const el = document.createElement('div')

  const bgColors: Record<string, string> = {
    info: 'var(--primary-color, #1677ff)',
    success: 'var(--success-color, #52c41a)',
    warning: 'var(--warning-color, #faad14)',
    error: 'var(--danger-color, #ff4d4f)'
  }

  el.style.cssText =
    `padding:10px 20px;border-radius:6px;color:#fff;font-size:14px;` +
    `background:${bgColors[type] || bgColors.info};` +
    `box-shadow:0 4px 12px rgba(0,0,0,0.15);` +
    `pointer-events:auto;transition:all 0.3s ease;` +
    `transform:translateX(120%);opacity:0`

  el.textContent = message
  container.appendChild(el)

  // 入场
  requestAnimationFrame(() => {
    el.style.transform = 'translateX(0)'
    el.style.opacity = '1'
  })

  // 自动移除
  const duration = options?.duration ?? TOAST_DURATION
  setTimeout(() => {
    el.style.transform = 'translateX(120%)'
    el.style.opacity = '0'
    setTimeout(() => el.remove(), 300)
  }, duration)
}
