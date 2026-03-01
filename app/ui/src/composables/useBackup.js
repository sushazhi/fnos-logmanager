import { useStatus } from './useStatus'
import api from '../services/api'

export function useBackup() {
  const { setStatus } = useStatus()

  async function backupLogs() {
    setStatus('正在备份日志...', 'loading')
    try {
      const data = await api.post('/api/logs/backup')
      setStatus(`备份完成: ${data.backupPath} (${data.backupSize})`, 'success')
      return data
    } catch (e) {
      setStatus('备份失败: ' + e.message, 'error')
      return null
    }
  }

  async function listBackups() {
    try {
      const data = await api.get('/api/backups/list')
      return data.backups || []
    } catch (e) {
      console.error('获取备份列表失败:', e)
      return []
    }
  }

  async function deleteBackup(path) {
    try {
      await api.post('/api/backups/delete', { path })
      return true
    } catch (e) {
      console.error('删除备份失败:', e)
      return false
    }
  }

  async function cleanOldBackups(days = 30) {
    try {
      const data = await api.post('/api/backups/clean', { days })
      return data.deleted || 0
    } catch (e) {
      console.error('清理备份失败:', e)
      return 0
    }
  }

  return {
    backupLogs,
    listBackups,
    deleteBackup,
    cleanOldBackups
  }
}
