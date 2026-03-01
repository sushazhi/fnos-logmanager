import { useStatus } from './useStatus'
import api from '../services/api'

export function useArchive() {
  const { setStatus, confirm } = useStatus()

  async function listArchives() {
    setStatus('正在查找归档日志...', 'loading')
    try {
      const data = await api.get('/api/archives/list?limit=50')
      setStatus(`找到 ${data.total} 个归档文件`, 'success')
      return data.archives.map(a => ({
        path: a.path,
        sizeFormatted: a.sizeFormatted,
        showActions: false
      }))
    } catch (e) {
      setStatus('查找失败: ' + e.message, 'error')
      return []
    }
  }

  async function viewArchive(path) {
    setStatus('正在加载归档日志...', 'loading')
    try {
      const data = await api.get(`/api/archive/content?path=${encodeURIComponent(path)}`)
      setStatus('归档日志加载完成', 'success')
      return {
        title: path,
        content: data.content || '(空文件)'
      }
    } catch (e) {
      setStatus('加载失败: ' + e.message, 'error')
      return null
    }
  }

  async function deleteArchive(path) {
    const confirmed = await confirm({
      title: '删除归档',
      message: '确定要删除此归档文件吗？',
      type: 'danger',
      confirmText: '删除'
    })
    if (!confirmed) return false

    setStatus('正在删除归档文件...', 'loading')
    try {
      await api.post('/api/archives/delete', { path })
      setStatus('归档文件已删除', 'success')
      return true
    } catch (e) {
      setStatus('删除失败: ' + e.message, 'error')
      return false
    }
  }

  return {
    listArchives,
    viewArchive,
    deleteArchive
  }
}
