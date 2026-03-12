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

export interface DirsResponse {
  dirs: Array<{
    path: string
    logCount?: number
    totalSize?: string
  }>
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

export interface LogContentResponse {
  content: string
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

export interface DockerLogsResponse {
  logs: string
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

export interface ArchiveContentResponse {
  content: string
}

// ==================== 备份类型 ====================

export interface Backup {
  path: string
  sizeFormatted: string
  createdAt: string
}

export interface BackupsResponse {
  backups: Backup[]
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

export interface UpdateCheckResponse {
  success: boolean
  hasUpdate: boolean
  latestVersion?: string
  changelog?: string
  publishedAt?: string
}

export interface UpdateStatus {
  success?: boolean
  updating: boolean
  progress: number
  message: string
  updateProgress?: number
  updateMessage?: string
}

// ==================== 设置类型 ====================

export interface FilterSettings {
  enabled: boolean
}

export interface FilterSettingsResponse {
  enabled: boolean
}

export interface ThemeSettings {
  theme: 'light' | 'dark' | 'auto'
  primaryColor?: string
  fontSize?: number
}

export interface ThemeSettingsResponse {
  theme: 'light' | 'dark' | 'auto'
  primaryColor?: string
  fontSize?: number
}

// ==================== 认证类型 ====================

export interface AuthStatus {
  initialized: boolean
  isLoggedIn: boolean
}

export interface CSRFTokenResponse {
  csrfToken: string
}

export interface LoginRequest {
  password: string
}

export interface LoginResponse {
  success: boolean
  error?: string
}

export interface SetupRequest {
  password: string
}

export interface SetupResponse {
  success: boolean
  error?: string
}

// ==================== 清洁类型 ====================

export type CleanType = 'truncateLarge' | 'deleteOld' | 'deleteUninstalled'

export interface CleanRequest {
  type: CleanType
  threshold?: string
  days?: number | null
  action?: string
}

export interface CleanResponse {
  cleaned: number
  success: boolean
}

// ==================== API 错误类型 ====================

export interface ApiError {
  error: string
  code?: string
  message?: string
}

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
