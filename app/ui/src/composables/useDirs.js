import { ref } from 'vue'
import api from '../services/api'
import { useStatus } from './useStatus'

const dirs = ref([])
const selectedDir = ref(null)

const DIR_NAMES = {
  '/vol1/@appdata': '@appdata',
  '/vol1/@appconf': '@appconf',
  '/vol1/@apphome': '@apphome',
  '/vol1/@apptemp': '@apptemp',
  '/vol1/@appshare': '@appshare',
  '/var/log/apps': '/var/log/apps'
}

export function useDirs() {
  const { setStatus } = useStatus()

  async function loadDirs() {
    try {
      const data = await api.get('/api/dirs')
      dirs.value = data.dirs.map(dir => ({
        ...dir,
        displayName: DIR_NAMES[dir.path] || dir.path
      }))
    } catch (e) {
      console.error('加载目录失败:', e)
    }
  }

  async function selectDir(dirPath) {
    selectedDir.value = dirPath
    setStatus(`正在加载 ${dirPath} 下的日志...`, 'loading')
    try {
      const data = await api.get(`/api/logs/list?dir=${encodeURIComponent(dirPath)}&limit=200`)
      const logs = data.logs.map(log => ({
        ...log,
        showActions: true
      }))
      setStatus(`已加载 ${logs.length} 个日志文件`, 'success')
      return logs
    } catch (e) {
      setStatus('加载失败: ' + e.message, 'error')
      return []
    }
  }

  return {
    dirs,
    selectedDir,
    loadDirs,
    selectDir
  }
}
