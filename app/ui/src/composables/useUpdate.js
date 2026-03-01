import { ref } from 'vue'

const APP_VERSION = '__APP_VERSION__'
const appVersion = ref(APP_VERSION)
const REPO_OWNER = 'sushazhi'
const REPO_NAME = 'fnos-logmanager'

const IGNORE_KEY = 'logmanager_ignore_version'
const CLOSE_TIME_KEY = 'logmanager_update_close_time'
const CLOSE_DURATION = 24 * 60 * 60 * 1000

const updateInfo = ref(null)

export function useUpdate() {
  function getIgnoredVersion() {
    try {
      return localStorage.getItem(IGNORE_KEY) || ''
    } catch (e) {
      return ''
    }
  }

  function getCloseTime() {
    try {
      const closeTime = localStorage.getItem(CLOSE_TIME_KEY)
      return closeTime ? parseInt(closeTime, 10) : 0
    } catch (e) {
      return 0
    }
  }

  function isRecentlyClosed() {
    return Date.now() - getCloseTime() < CLOSE_DURATION
  }

  function ignoreVersion(version) {
    try {
      localStorage.setItem(IGNORE_KEY, version)
    } catch (e) {}
  }

  function setCloseTime() {
    try {
      localStorage.setItem(CLOSE_TIME_KEY, Date.now().toString())
    } catch (e) {}
  }

  function compareVersions(v1, v2) {
    const parts1 = v1.split('.').map(Number)
    const parts2 = v2.split('.').map(Number)
    
    for (let i = 0; i < Math.max(parts1.length, parts2.length); i++) {
      const n1 = parts1[i] || 0
      const n2 = parts2[i] || 0
      if (n1 > n2) return 1
      if (n1 < n2) return -1
    }
    return 0
  }

  async function checkForUpdates() {
    try {
      const response = await fetch(`https://api.github.com/repos/${REPO_OWNER}/${REPO_NAME}/releases/latest`, {
        headers: { 'Accept': 'application/vnd.github.v3+json' },
        cache: 'no-store'
      })
      if (!response.ok) return null
      
      const data = await response.json()
      const latestVersion = (data.tag_name || '').replace(/^v/, '')
      
      if (compareVersions(latestVersion, APP_VERSION) > 0) {
        if (getIgnoredVersion() === latestVersion) {
          console.log('[Update] 已忽略版本', latestVersion)
          return null
        }

        if (isRecentlyClosed()) {
          console.log('[Update] 24小时内已关闭，跳过通知')
          return null
        }

        updateInfo.value = {
          version: latestVersion,
          url: data.html_url || '',
          changelog: data.body || ''
        }
        return updateInfo.value
      }
      return null
    } catch (e) {
      console.error('检查更新失败:', e)
      return null
    }
  }

  return {
    appVersion,
    updateInfo,
    checkForUpdates,
    ignoreVersion,
    setCloseTime
  }
}
