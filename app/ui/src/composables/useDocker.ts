import { ref } from 'vue'
import type { Ref } from 'vue'
import type { DockerContainer, DockerContainersResponse, LogItem } from '../types'
import api from '../services/api'
import { useStatus } from './useStatus'

const dockerContainers = ref<DockerContainer[]>([]) as Ref<DockerContainer[]>

export function useDocker() {
  const { setStatus } = useStatus()

  async function listDockerContainers(): Promise<LogItem[]> {
    setStatus('正在获取Docker容器...', 'loading')
    try {
      const data = await api.get<DockerContainersResponse>('/api/docker/containers')
      if (data.error) {
        setStatus(data.error, 'warning')
        return []
      }
      const containers: DockerContainer[] = Array.isArray(data.containers) ? data.containers : []
      dockerContainers.value = containers
      setStatus(`已加载 ${containers.length} 个容器`, 'success')
      return containers.map(c => ({
        path: c.name,
        sizeFormatted: c.image || '-',
        isDocker: true,
        size: 0,
        showActions: true
      }))
    } catch (e) {
      const error = e as Error
      setStatus('获取失败: ' + error.message, 'error')
      return []
    }
  }

  async function viewDockerLogs(container: string): Promise<{ title: string; content: string } | null> {
    setStatus('正在获取容器日志...', 'loading')
    try {
      const data = await api.get<{ logs: string }>(`/api/docker/logs?container=${encodeURIComponent(container)}`)
      return {
        title: `Docker: ${container}`,
        content: data.logs || '(无日志)'
      }
    } catch (e) {
      const error = e as Error
      setStatus('加载失败: ' + error.message, 'error')
      return null
    }
  }

  return {
    dockerContainers,
    listDockerContainers,
    viewDockerLogs
  }
}
