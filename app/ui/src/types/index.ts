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
  displayPath?: string
  isShared?: boolean
}

// ==================== 日志类型 ====================

export interface LogItem {
  path: string
  size: number
  sizeFormatted: string
  showActions: boolean
  canDelete?: boolean
  isDocker?: boolean
  isArchive?: boolean
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

// ==================== 进程类型 ====================

export interface Process {
  pid: number
  ppid: number
  user: string
  name: string
  state: string
  cpu: number
  memory: string
  memBytes: number
  startTime: string
  command: string
  exePath: string
  ports: number[]
  protect: boolean
  system: boolean
}

export interface ProcessesResponse {
  total: number
  processes: Process[]
  error?: string
}

export interface KillProcessResult {
  success: boolean
  pid: number
  command: string
  signal: string
  terminated?: boolean
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

export interface BackupListItem {
  name: string
  path: string
  size: string
  created: string
}

export interface BackupPreviewEntry {
  name: string
  targetPath: string
  size: number
  sizeFormatted: string
  exists: boolean
  denied: boolean
}

export interface BackupPreview {
  backupPath: string
  totalFiles: number
  totalSize: number
  totalSizeFormatted: string
  deniedFiles: number
  entries: BackupPreviewEntry[]
  truncated: boolean
}

export interface RestoreItemResult {
  path: string
  status: 'restored' | 'skipped' | 'failed'
  message?: string
}

export interface RestoreResult {
  restored: number
  skipped: number
  failed: number
  errors?: string[]
  details: RestoreItemResult[]
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
