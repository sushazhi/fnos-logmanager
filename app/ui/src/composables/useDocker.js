import { ref } from 'vue'
import api from '../services/api'
import { useStatus } from './useStatus'

const dockerContainers = ref([])

export function useDocker() {
  const { setStatus } = useStatus()

  async function listDockerContainers() {
    setStatus('正在获取Docker容器...', 'loading')
    try {
      const data = await api.get('/api/docker/containers')
      if (data.error) {
        setStatus(data.error, 'warning')
        return []
      }
      const containers = Array.isArray(data.containers) ? data.containers : []
      dockerContainers.value = containers
      setStatus(`已加载 ${containers.length} 个容器`, 'success')
      return containers.map(c => ({
        path: c.name,
        sizeFormatted: c.image || '-',
        isDocker: true
      }))
    } catch (e) {
      setStatus('获取失败: ' + e.message, 'error')
      return []
    }
  }

  async function viewDockerLogs(container) {
    setStatus('正在获取容器日志...', 'loading')
    try {
      // 不传 lines 参数，获取全部日志
      const data = await api.get(`/api/docker/logs?container=${encodeURIComponent(container)}`)
      return {
        title: `Docker: ${container}`,
        content: data.logs || '(无日志)'
      }
    } catch (e) {
      setStatus('加载失败: ' + e.message, 'error')
      return null
    }
  }

  return {
    dockerContainers,
    listDockerContainers,
    viewDockerLogs
  }
}
