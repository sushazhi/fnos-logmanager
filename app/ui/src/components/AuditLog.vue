<template>
  <div class="hm-overlay-base" @click.self="$emit('close')">
    <div class="hm-modal-base modal audit-modal">
      <div class="modal-header">
        <h3>审计日志</h3>
        <button class="close-btn" @click="$emit('close')">×</button>
      </div>
      <div class="filter-bar">
        <button 
          v-for="cat in categories" 
          :key="cat.value"
          :class="['filter-btn', { active: activeCategory === cat.value }]"
          @click="activeCategory = cat.value"
        >
          {{ cat.label }}
        </button>
      </div>
      <div class="modal-body">
        <div v-if="loading" class="loading">加载中...</div>
        <div v-else-if="filteredLogs.length === 0" class="empty">
          {{ logs.length === 0 ? '暂无审计日志' : '该分类暂无日志' }}
        </div>
        <div v-else class="log-list">
          <div v-for="(log, index) in filteredLogs" :key="index" class="log-item" :class="getLogClass(log.action)">
            <div class="log-header">
              <span class="log-action">{{ getActionText(log.action) }}</span>
              <span class="log-time">{{ formatTime(log.timestamp) }}</span>
            </div>
            <div class="log-details">
              <span class="log-ip">来源IP: {{ log.ip }}</span>
              <span v-if="log.details && Object.keys(log.details).length" class="log-extra">
                {{ formatDetails(log.details) }}
              </span>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import api from '../services/api'

defineEmits(['close'])

const logs = ref([])
const loading = ref(true)
const activeCategory = ref('all')

const categories = [
  { label: '全部', value: 'all' },
  { label: '登录', value: 'login' },
  { label: '删除', value: 'delete' },
  { label: '清空', value: 'truncate' },
  { label: '其他', value: 'other' }
]

const filteredLogs = computed(() => {
  if (!Array.isArray(logs.value)) return []
  if (activeCategory.value === 'all') return logs.value
  
  return logs.value.filter(log => {
    const action = log.action || ''
    switch (activeCategory.value) {
      case 'login':
        return action.includes('login') || action.includes('logout') || action.includes('auth') || action.includes('password')
      case 'delete':
        return action.includes('delete')
      case 'truncate':
        return action.includes('truncate') || action.includes('clean')
      case 'other':
        return !action.includes('login') && !action.includes('logout') && 
               !action.includes('auth') && !action.includes('password') &&
               !action.includes('delete') && !action.includes('truncate') && !action.includes('clean')
      default:
        return true
    }
  })
})

onMounted(async () => {
  try {
    const data = await api.get('/api/audit/log')
    logs.value = data.logs || []
  } catch (e) {
    console.error('加载审计日志失败:', e)
  } finally {
    loading.value = false
  }
})

function getLogClass(action) {
  if (action.includes('failed') || action.includes('locked')) return 'danger'
  if (action.includes('delete') || action.includes('truncate') || action.includes('clean') || action.includes('remov')) return 'warning'
  if (action.includes('success')) return 'success'
  return 'info'
}

function getActionText(action) {
  // 后端 AddSecurityAuditLog 会给 action 加 SECURITY_ 前缀，MCP/系统类审计均带此前缀
  const actionMap = {
    'login_success': '登录成功',
    'login_failed': '登录失败',
    'login_locked': '账户锁定',
    'logout': '登出',
    'password_setup': '设置密码',
    'password_changed': '密码修改',
    'password_change_failed': '密码修改失败',
    'log_truncate': '日志清空',
    'log_delete': '日志删除',
    'archive_delete': '归档删除',
    'logs_clean': '批量清理',
    'logs_backup': '日志备份',
    'backup_delete': '备份删除',
    'backups_clean': '备份清理',
    'auth_failed': '认证失败',
    'csrf_failed': 'CSRF验证失败',
    'bookmark_add': '添加书签',
    'bookmark_delete': '删除书签',
    'bookmark_update': '更新书签',
    'autoclean_add': '添加清理规则',
    'autoclean_delete': '删除清理规则',
    'autoclean_update': '更新清理规则',
    'autoclean_trigger': '触发自动清理',
    'dirs_clean_empty': '清理空文件夹',
    'log_export': '日志导出',
    // 安全审计（带 SECURITY_ 前缀）
    'SECURITY_MCP_CONFIG_UPDATE': '更新 MCP 配置',
    'SECURITY_MCP_LOG_TRUNCATE': 'MCP 清空日志',
    'SECURITY_MCP_LOG_DELETE': 'MCP 删除日志文件',
    'SECURITY_MCP_LOGS_CLEAN': 'MCP 批量清理日志',
    'SECURITY_MCP_DIRS_CLEAN_EMPTY': 'MCP 清理空文件夹',
    'SECURITY_MCP_LOGS_BACKUP': 'MCP 备份日志',
    'SECURITY_MCP_BACKUP_DELETE': 'MCP 删除备份',
    'SECURITY_MCP_BACKUPS_CLEAN': 'MCP 清理旧备份',
    'SECURITY_MCP_KERNEL_REMOVE': 'MCP 删除内核',
    'SECURITY_MCP_KERNEL_CLEANUP': 'MCP 清理旧内核',
    'SECURITY_SENSITIVE_INFO_SCAN': '敏感信息扫描',
    'SECURITY_APP_UPDATED': '应用升级',
    'SECURITY_UPDATE_FAILED': '应用升级失败',
    'SECURITY_PROCESS_KILL': '结束进程',
    'SECURITY_PROCESS_LOG_READ': '查看进程日志',
    'SECURITY_MCP_DIRS_CLEAN_UNINSTALLED': 'MCP 清理已卸载残留',
    'SECURITY_MCP_DIRS_RESTORE': 'MCP 还原残留目录',
    'SECURITY_MCP_PROCESS_KILL': '结束进程(MCP)',
    'SECURITY_MCP_PROCESS_LOG_READ': '查看进程日志(MCP)',
    'SECURITY_UNCAUGHT_EXCEPTION': '安全异常-未捕获异常',
    'SECURITY_UNHANDLED_REJECTION': '安全异常-未处理Promise'
  }
  return actionMap[action] || action
}

function formatTime(timestamp) {
  const date = new Date(timestamp)
  return date.toLocaleString('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit'
  })
}

function formatDetails(details) {
  if (!details) return ''
  const originalAction = details.originalAction || ''
  const parts = []

  // 进程类操作：展示 PID/命令/信号等更有意义的信息，且其 path 是 API 请求路径，不应标为"文件"
  const isProcessAction = /PROCESS_KILL|PROCESS_LOG_READ/.test(originalAction)
  if (isProcessAction) {
    if (details.pid !== undefined) parts.push(`PID: ${details.pid}`)
    if (details.command) parts.push(`命令: ${details.command}`)
    if (details.signal) parts.push(`信号: ${details.signal}`)
    if (details.outcome) parts.push(`结果: ${details.outcome === 'terminated' ? '已退出' : '已结束'}`)
    if (details.path && !details.path.startsWith('/api/')) parts.push(`文件: ${details.path}`)
    return parts.join(' | ')
  }

  // 其余操作：优先展示文件/归档路径，避免把 API 请求路径当作文件路径展示
  const isFileOp = /LOG_TRUNCATE|LOG_DELETE|LOGS_BACKUP|BACKUP_DELETE|KERNEL_|FILE/.test(originalAction)
  if (details.path && isFileOp) parts.push(`文件: ${details.path}`)
  if (details.path && !isFileOp && !details.path.startsWith('/api/')) parts.push(`路径: ${details.path}`)
  if (details.action) parts.push(`操作: ${details.action}`)
  if (details.version) parts.push(`版本: ${details.version}`)
  if (details.cleaned !== undefined) parts.push(`清理: ${details.cleaned}个`)
  if (details.deleted !== undefined) parts.push(`删除: ${details.deleted}个`)
  if (details.restored !== undefined) parts.push(`还原: ${details.restored}个`)
  if (details.moved !== undefined) parts.push(`移动: ${details.moved}个`)
  if (details.freedSize !== undefined) parts.push(`释放: ${details.freedSize}`)
  return parts.join(' | ')
}
</script>

<style scoped>
.modal {
  width: 100%;
  overflow: hidden;
}

.modal-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: var(--spacing-lg) var(--spacing-xl);
  border-bottom: 1px solid var(--border-color);
}

.modal-header h3 {
  margin: 0;
  font-size: var(--font-size-2xl);
}

.close-btn {
  background: none;
  border: none;
  font-size: var(--font-size-4xl);
  cursor: pointer;
  color: var(--text-color-3);
  padding: 0;
  line-height: 1;
}

.close-btn:hover {
  color: var(--text-color);
}

.filter-bar {
  display: flex;
  gap: var(--spacing-sm);
  padding: var(--spacing-sm) var(--spacing-lg);
  background: var(--bg-color-2);
  border-bottom: 1px solid var(--border-color);
  overflow-x: auto;
}

.filter-btn {
  padding: var(--spacing-xs) var(--spacing-md);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-lg);
  background: var(--card-bg);
  color: var(--text-color-2);
  font-size: var(--font-size-base);
  cursor: pointer;
  white-space: nowrap;
  transition: all var(--transition-fast);
}

.filter-btn:hover {
  border-color: var(--primary-color);
  color: var(--primary-color);
}

.filter-btn.active {
  background: var(--primary-color);
  border-color: var(--primary-color);
  color: var(--text-color-on-primary);
  box-shadow: 0 0 20px var(--glow-primary);
}

.filter-btn.active:active {
  box-shadow: 0 0 28px var(--glow-primary-strong);
}

.audit-modal {
  max-width: 600px;
  max-height: 80vh;
}

.modal-body {
  padding: 0;
  max-height: 60vh;
  overflow-y: auto;
}

.loading, .empty {
  padding: var(--spacing-3xl);
  text-align: center;
  color: var(--text-color-3);
}

.log-list {
  padding: var(--spacing-sm);
}

.log-item {
  padding: var(--spacing-md);
  border-radius: var(--radius-md);
  margin-bottom: var(--spacing-sm);
  background: var(--bg-color-2);
  border-left: 4px solid var(--primary-color);
}

.log-item.danger {
  border-left-color: var(--error-color);
  background: var(--error-bg);
}

.log-item.warning {
  border-left-color: var(--warning-color);
  background: var(--warning-bg);
}

.log-item.success {
  border-left-color: var(--success-color);
  background: var(--success-bg);
}

.log-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: var(--spacing-xs);
}

.log-action {
  font-weight: 500;
  font-size: var(--font-size-md);
}

.log-time {
  font-size: var(--font-size-sm);
  color: var(--text-color-2);
}

.log-details {
  font-size: var(--font-size-sm);
  color: var(--text-color-2);
  display: flex;
  flex-wrap: wrap;
  gap: var(--spacing-sm);
}

.log-ip {
  background: var(--card-bg);
  padding: calc(var(--spacing-xs) / 2) var(--spacing-sm);
  border-radius: var(--radius-2xs);
}

.log-extra {
  color: var(--text-color-2);
}

/* 移动端适配 */
@media (max-width: 768px) {
  .hm-overlay-base {
    padding: var(--spacing-sm);
    align-items: flex-end;
  }

  .audit-modal {
    max-width: 100%;
    max-height: 90vh;
    border-radius: var(--radius-xl) var(--radius-xl) 0 0;
  }

  .modal-header {
    padding: var(--spacing-sm) var(--spacing-md);
    border-bottom: 1px solid var(--border-color);
  }

  .modal-header h3 {
    font-size: var(--font-size-xl);
    font-weight: 500;
    white-space: nowrap;
    color: var(--text-color-1);
  }

  .close-btn {
    font-size: var(--font-size-4xl);
    color: var(--text-color-2);
    min-width: 32px;
    min-height: 32px;
    display: flex;
    align-items: center;
    justify-content: center;
  }

  .close-btn:hover {
    color: var(--text-color-1);
  }

  .filter-bar {
    padding: var(--spacing-xs) var(--spacing-md);
    gap: var(--spacing-xs);
    background: var(--bg-color-2);
    border-bottom: 1px solid var(--border-color);
  }

  .filter-btn {
    padding: var(--spacing-xs) var(--spacing-sm);
    font-size: var(--font-size-sm);
    background: var(--card-bg);
    border-color: var(--border-color);
    color: var(--text-color-2);
    border-radius: var(--radius-lg);
  }

  .filter-btn:hover {
    border-color: var(--primary-color);
    color: var(--primary-color);
  }

  .filter-btn.active {
    background: var(--primary-color);
    border-color: var(--primary-color);
    color: var(--text-color-on-primary);
  }

  .modal-body {
    max-height: 75vh;
  }

  .log-list {
    padding: var(--spacing-sm);
  }

  .log-item {
    padding: var(--spacing-sm);
    margin-bottom: var(--spacing-sm);
    border-radius: var(--radius-sm);
    background: var(--bg-color-2);
    border-left: 4px solid var(--primary-color);
  }

  .log-item.danger {
    border-left-color: var(--error-color);
    background: var(--error-bg);
  }

  .log-item.warning {
    border-left-color: var(--warning-color);
    background: var(--warning-bg);
  }

  .log-item.success {
    border-left-color: var(--success-color);
    background: var(--success-bg);
  }

  .log-header {
    margin-bottom: var(--spacing-xs);
  }

  .log-action {
    font-size: var(--font-size-md);
    font-weight: 500;
    color: var(--text-color-1);
  }

  .log-time {
    font-size: var(--font-size-xs);
    color: var(--text-color-3);
  }

  .log-details {
    font-size: var(--font-size-sm);
    color: var(--text-color-2);
    flex-direction: column;
    gap: var(--spacing-xs);
  }

  .log-ip {
    padding: var(--spacing-xs) var(--spacing-sm);
    font-size: var(--font-size-sm);
    background: var(--card-bg);
    border-radius: var(--radius-xs);
  }

  .log-extra {
    font-size: var(--font-size-sm);
    color: var(--text-color-3);
  }

  .loading, .empty {
    padding: var(--spacing-2xl) var(--spacing-lg);
    font-size: var(--font-size-md);
    color: var(--text-color-2);
  }
}

@media (max-width: 480px) {
  .hm-overlay-base {
    padding: 0;
    align-items: flex-end;
  }

  .audit-modal {
    max-width: 100%;
    max-height: 95vh;
    border-radius: 0;
  }

  .modal-header {
    padding: var(--spacing-xs) var(--spacing-sm);
  }

  .modal-header h3 {
    font-size: var(--font-size-lg);
  }

  .close-btn {
    font-size: var(--font-size-3xl);
  }

  .filter-bar {
    padding: var(--spacing-xs) var(--spacing-sm);
    gap: var(--spacing-xs);
  }

  .filter-btn {
    padding: var(--spacing-xs);
    font-size: var(--font-size-xs);
  }

  .modal-body {
    max-height: 80vh;
  }

  .log-list {
    padding: var(--spacing-xs);
  }

  .log-item {
    padding: var(--spacing-xs);
    margin-bottom: var(--spacing-xs);
    border-radius: var(--radius-xs);
  }

  .log-action {
    font-size: var(--font-size-base);
  }

  .log-time {
    font-size: var(--font-size-2xs);
  }

  .log-details {
    font-size: var(--font-size-xs);
    gap: calc(var(--spacing-xs) / 2);
  }

  .log-ip {
    padding: calc(var(--spacing-xs) / 2) var(--spacing-xs);
    font-size: var(--font-size-xs);
  }

  .log-extra {
    font-size: var(--font-size-xs);
  }

  .loading, .empty {
    padding: var(--spacing-xl) var(--spacing-md);
    font-size: var(--font-size-base);
  }
}

/* 平板适配 */
@media (min-width: 481px) and (max-width: 768px) {
  .audit-modal {
    max-width: 500px;
  }

  .modal-header {
    padding: var(--spacing-sm) var(--spacing-lg);
  }

  .modal-header h3 {
    font-size: var(--font-size-2xl);
  }
}

/* 深色主题 */
:global(.dark-theme) .log-item {
  background: var(--bg-color-2);
}

:global(.dark-theme) .log-item.danger {
  background: var(--error-bg);
}

:global(.dark-theme) .log-item.warning {
  background: var(--warning-bg);
}

:global(.dark-theme) .log-item.success {
  background: var(--success-bg);
}

:global(.dark-theme) .log-ip {
  background: var(--bg-color-1);
}
</style>
