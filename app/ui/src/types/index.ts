// ==================== 状态类型 ====================

export type StatusType = 'success' | 'error' | 'warning' | 'loading' | 'info'

export interface Status {
  message: string
  type: StatusType
}

// ==================== 统计数据类型 ====================

export interface Stats {
  totalLogs: number
  totalSize: string
  archiveCount: number
  largeFiles: number
}

export interface StatsResponse {
  totalLogs: number
  totalSizeFormatted: string
  totalArchives: number
  largeFiles: number
}

// ==================== 目录类型 ====================

export interface Dir {
  path: string
  displayName: string
  logCount?: number
  totalSize?: string
  exists?: boolean
  archiveCount?: number
}

// ==================== 日志类型 ====================

export interface LogItem {
  path: string
  size: number
  sizeFormatted: string
  showActions: boolean
  canDelete?: boolean
  isDocker?: boolean
}

export interface LogsResponse {
  logs: Array<{
    path: string
    size: number
    sizeFormatted: string
    canDelete?: boolean
  }>
  total: number
}

// ==================== Docker 类型 ====================

export interface DockerContainer {
  name: string
  image: string
  status: string
}

export interface DockerContainersResponse {
  containers: DockerContainer[]
  error?: string
}

// ==================== 归档类型 ====================

export interface Archive {
  path: string
  sizeFormatted: string
}

export interface ArchivesResponse {
  archives: Archive[]
  total: number
}

// ==================== 备份类型 ====================

export interface Backup {
  path: string
  sizeFormatted: string
  createdAt: string
}

export interface BackupResponse {
  backupPath: string
  backupSize: string
  success: boolean
}

// ==================== 更新类型 ====================

export interface UpdateInfo {
  version: string
  changelog: string
  publishedAt: string
  url: string
}

export interface UpdateStatus {
  success?: boolean
  updating: boolean
  progress: number
  message: string
  updateProgress?: number
  updateMessage?: string
}

export type CleanType = 'truncateLarge' | 'deleteOld' | 'deleteUninstalled'

// ==================== 确认对话框类型 ====================

export interface ConfirmOptions {
  title?: string
  message: string
  type?: 'warning' | 'danger' | 'info'
  confirmText?: string
  cancelText?: string
}

// ==================== 列表类型 ====================

export type ListType = 'logs' | 'docker' | 'archives'
