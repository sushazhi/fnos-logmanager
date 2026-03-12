import { ref } from 'vue'
import type { Ref } from 'vue'
import type { UpdateInfo, UpdateStatus } from '../types'

const APP_VERSION = '__APP_VERSION__'
const appVersion = ref<string>(APP_VERSION)
const REPO_OWNER = 'sushazhi'
const REPO_NAME = 'fnos-logmanager'
const API_BASE = window.location.origin

// 调试模式：通过 URL 参数 ?debug=true 开启
const DEBUG_MODE = new URLSearchParams(window.location.search).has('debug')

const IGNORE_KEY = 'logmanager_ignore_version'
const CLOSE_TIME_KEY = 'logmanager_update_close_time'
const CLOSE_DURATION = 24 * 60 * 60 * 1000

const updateInfo = ref<UpdateInfo | null>(null) as Ref<UpdateInfo | null>
const updateStatus = ref<UpdateStatus>({
  updating: false,
  progress: 0,
  message: ''
})

export function useUpdate() {
  function getIgnoredVersion(): string {
    try {
      return localStorage.getItem(IGNORE_KEY) || ''
    } catch {
      return ''
    }
  }

  function getCloseTime(): number {
    try {
      const closeTime = localStorage.getItem(CLOSE_TIME_KEY)
      return closeTime ? parseInt(closeTime, 10) : 0
    } catch {
      return 0
    }
  }

  function isRecentlyClosed(): boolean {
    return Date.now() - getCloseTime() < CLOSE_DURATION
  }

  function ignoreVersion(version: string): void {
    try {
      localStorage.setItem(IGNORE_KEY, version)
    } catch {
      // ignore
    }
  }

  function setCloseTime(): void {
    try {
      localStorage.setItem(CLOSE_TIME_KEY, Date.now().toString())
    } catch {
      // ignore
    }
  }

  async function checkForUpdates(): Promise<UpdateInfo | null> {
    try {
      const response = await fetch(`${API_BASE}/api/update/check`, {
        credentials: 'include',
        cache: 'no-store'
      })
      if (!response.ok) return null

      const data = await response.json() as {
        success: boolean
        hasUpdate: boolean
        latestVersion?: string
        changelog?: string
        publishedAt?: string
      }

      if (data.success && data.hasUpdate) {
        // 调试模式下绕过所有限制
        if (!DEBUG_MODE) {
          if (getIgnoredVersion() === data.latestVersion) {
            return null
          }

          if (isRecentlyClosed()) {
            return null
          }
        }

        const info: UpdateInfo = {
          version: data.latestVersion || '',
          changelog: data.changelog || '',
          publishedAt: data.publishedAt || '',
          url: `https://github.com/${REPO_OWNER}/${REPO_NAME}/releases/latest`
        }
        updateInfo.value = info
        return updateInfo.value
      }
      return null
    } catch (e) {
      console.error('检查更新失败:', e)
      return null
    }
  }

  async function installUpdate(): Promise<void> {
    try {
      updateStatus.value = {
        updating: true,
        progress: 0,
        message: '准备更新...'
      }

      // 先检查是否已经在更新中
      try {
        const statusResponse = await fetch(`${API_BASE}/api/update/status`, {
          credentials: 'include'
        })
        const statusData = await statusResponse.json() as UpdateStatus

        if (statusData.updating) {
          // 已经在更新中，直接开始轮询
          pollUpdateStatus()
          return
        }
      } catch {
        // 服务器可能正在重启，开始轮询
        pollUpdateStatus()
        return
      }

      const data = await api.post<{ success: boolean; error?: string }>('/api/update/install')

      if (data.success) {
        // 开始轮询更新状态
        pollUpdateStatus()
      } else {
        throw new Error(data.error || '安装更新失败')
      }
    } catch (err) {
      console.error('安装更新失败:', err)

      // 如果是连接错误，说明服务器正在重启
      if (err instanceof Error && (err.message === 'Failed to fetch' || err.name === 'TypeError')) {
        updateStatus.value = {
          updating: true,
          progress: 90,
          message: '更新完成，正在重启应用...'
        }
        // 开始轮询，等待服务器重启
        pollUpdateStatus()
      } else {
        const errorObj = err as Error
        updateStatus.value = {
          updating: false,
          progress: 0,
          message: '更新失败: ' + errorObj.message
        }
      }
    }
  }

  async function getUpdateStatus(): Promise<UpdateStatus> {
    try {
      const response = await fetch(`${API_BASE}/api/update/status`, {
        credentials: 'include'
      })

      if (!response.ok) {
        throw new Error('获取更新状态失败')
      }

      return await response.json() as UpdateStatus
    } catch (e) {
      console.error('获取更新状态失败:', e)
      return {
        success: false,
        updating: false,
        progress: 0,
        message: ''
      }
    }
  }

  function pollUpdateStatus(interval = 2000): () => void {
    let consecutiveErrors = 0
    const maxErrors = 10 // 最多允许连续错误10次（20秒）

    const timer = setInterval(async () => {
      try {
        const status = await getUpdateStatus()
        consecutiveErrors = 0 // 重置错误计数

        if (status.success) {
          updateStatus.value = {
            updating: status.updating,
            progress: status.progress ?? status.updateProgress ?? 0,
            message: status.message ?? status.updateMessage ?? ''
          }

          // 如果更新完成，停止轮询
          if (!status.updating && (status.progress ?? status.updateProgress ?? 0) === 100) {
            clearInterval(timer)
            updateStatus.value.message = '更新完成，正在重启应用...'
            setTimeout(() => {
              window.location.reload()
            }, 2000)
          }
        }
      } catch {
        consecutiveErrors++

        // 如果连续错误次数达到上限，说明服务器正在重启
        if (consecutiveErrors >= maxErrors) {
          clearInterval(timer)
          updateStatus.value = {
            updating: true,
            progress: 95,
            message: '更新完成，正在重启应用...'
          }

          // 等待服务器重启后自动刷新
          setTimeout(() => {
            // 尝试重新连接
            const checkServer = setInterval(() => {
              fetch(`${API_BASE}/api/update/status`, {
                credentials: 'include'
              }).then(response => {
                if (response.ok) {
                  clearInterval(checkServer)
                  window.location.reload()
                }
              }).catch(() => {
                // 服务器仍在重启
              })
            }, 3000)
          }, 5000)
        }
      }
    }, interval)

    return () => clearInterval(timer)
  }

  return {
    appVersion,
    updateInfo,
    updateStatus,
    checkForUpdates,
    installUpdate,
    getUpdateStatus,
    ignoreVersion,
    setCloseTime
  }
}

// 导入 api 用于类型推断
import api from '../services/api'
