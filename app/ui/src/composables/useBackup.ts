import { useStatus } from './useStatus'
import api from '../services/api'
import type { Backup, BackupResponse } from '../types'

export function useBackup() {
  const { setStatus } = useStatus()

  async function backupLogs(): Promise<BackupResponse | null> {
    setStatus('正在备份日志...', 'loading')
    try {
      const data = await api.post<BackupResponse>('/api/logs/backup')
      setStatus(`备份完成: ${data.backupPath} (${data.backupSize})`, 'success')
      return data
    } catch (e) {
      const error = e as Error
      setStatus('备份失败: ' + error.message, 'error')
      return null
    }
  }

  async function listBackups(): Promise<Backup[]> {
    try {
      const data = await api.get<{ backups: Backup[] }>('/api/backups/list')
      return data.backups || []
    } catch (e) {
      console.error('获取备份列表失败:', e)
      return []
    }
  }

  async function deleteBackup(path: string): Promise<boolean> {
    try {
      await api.post('/api/backups/delete', { path })
      return true
    } catch (e) {
      console.error('删除备份失败:', e)
      return false
    }
  }

  async function cleanOldBackups(days: number = 30): Promise<number> {
    try {
      const data = await api.post<{ deleted: number }>('/api/backups/clean', { days })
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
