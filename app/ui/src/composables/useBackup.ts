/**
 * useBackup - 备份创建 / 备份管理（列表、预览、恢复、删除）
 */
import { useStatusStore } from '../stores/useStatusStore'
import api from '../services/api'
import type { BackupResponse, BackupListItem, BackupPreview, RestoreResult } from '../types'

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

  async function listBackups(): Promise<BackupListItem[]> {
    const data = await api.get<{ backups: BackupListItem[]; total: number }>('/api/backups/list')
    return data.backups || []
  }

  async function previewBackup(path: string, limit = 200): Promise<BackupPreview> {
    const data = await api.get<{ preview: BackupPreview }>(
      `/api/backups/preview?path=${encodeURIComponent(path)}&limit=${limit}`
    )
    return data.preview
  }

  async function restoreBackup(path: string, overwrite: boolean): Promise<RestoreResult> {
    const data = await api.post<{ success: boolean; result: RestoreResult }>(
      '/api/backups/restore',
      { path, overwrite }
    )
    return data.result
  }

  async function deleteBackup(path: string): Promise<void> {
    await api.post('/api/backups/delete', { path })
  }

  return {
    backupLogs,
    listBackups,
    previewBackup,
    restoreBackup,
    deleteBackup
  }
}
