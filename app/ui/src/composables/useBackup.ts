/**
 * useBackup - 使用 Pinia useStatusStore
 */
import { useStatusStore } from '../stores/useStatusStore'
import api from '../services/api'
import type { BackupResponse } from '../types'

export function useBackup() {
  const { setStatus } = useStatusStore()

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

  return {
    backupLogs
  }
}
