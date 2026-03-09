import { ref } from 'vue'
import type { Ref } from 'vue'
import type { Dir, LogsResponse } from '../types'
import api from '../services/api'
import { useStatus } from './useStatus'

const dirs = ref<Dir[]>([]) as Ref<Dir[]>
const selectedDir = ref<string | null>(null) as Ref<string | null>

const DIR_NAMES: Record<string, string> = {
  '/vol1/@appdata': '@appdata',
  '/vol1/@appconf': '@appconf',
  '/vol1/@apphome': '@apphome',
  '/vol1/@apptemp': '@apptemp',
  '/vol1/@appshare': '@appshare',
  '/var/log/apps': '/var/log/apps'
}

export function useDirs() {
  const { setStatus } = useStatus()

  async function loadDirs(): Promise<void> {
    try {
      const data = await api.get<{ dirs: Array<{ path: string; logCount?: number; totalSize?: string }> }>('/api/dirs')
      dirs.value = data.dirs.map(dir => ({
        ...dir,
        displayName: DIR_NAMES[dir.path] || dir.path
      }))
    } catch (e) {
      console.error('加载目录失败:', e)
    }
  }

  async function selectDir(dirPath: string): Promise<Array<{ path: string; size: number; sizeFormatted: string; showActions: boolean }>> {
    selectedDir.value = dirPath
    setStatus(`正在加载 ${dirPath} 下的日志...`, 'loading')
    try {
      const data = await api.get<LogsResponse>(`/api/logs/list?dir=${encodeURIComponent(dirPath)}&limit=200`)
      const logs = data.logs.map(log => ({
        ...log,
        showActions: true
      }))
      setStatus(`已加载 ${logs.length} 个日志文件`, 'success')
      return logs
    } catch (e) {
      const error = e as Error
      setStatus('加载失败: ' + error.message, 'error')
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
