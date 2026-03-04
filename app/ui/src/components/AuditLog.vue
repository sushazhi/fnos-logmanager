<template>
  <div class="modal-overlay" @click.self="$emit('close')">
    <div class="modal audit-modal">
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
              <span class="log-ip">IP: {{ log.ip }}</span>
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
  if (action.includes('delete')) return 'warning'
  if (action.includes('success')) return 'success'
  return 'info'
}

function getActionText(action) {
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
    'csrf_failed': 'CSRF验证失败'
  }
  return actionMap[action] || action
}

function formatTime(timestamp) {
  const date = new Date(timestamp)
  return date.toLocaleString('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit'
  })
}

function formatDetails(details) {
  if (!details) return ''
  const parts = []
  if (details.path) parts.push(`文件: ${details.path}`)
  if (details.action) parts.push(`操作: ${details.action}`)
  if (details.cleaned !== undefined) parts.push(`清理: ${details.cleaned}个`)
  return parts.join(' | ')
}
</script>

<style scoped>
.modal-overlay {
  position: fixed;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  background: rgba(0, 0, 0, 0.5);
  display: flex;
  justify-content: center;
  align-items: center;
  z-index: 2000;
  padding: 20px;
}

.modal {
  background: var(--card-bg, white);
  border-radius: 16px;
  box-shadow: 0 10px 40px rgba(0, 0, 0, 0.2);
  width: 100%;
  overflow: hidden;
}

.modal-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 16px 20px;
  border-bottom: 1px solid var(--border-color, #e0e0e0);
}

.modal-header h3 {
  margin: 0;
  font-size: 18px;
}

.close-btn {
  background: none;
  border: none;
  font-size: 24px;
  cursor: pointer;
  color: var(--text-secondary, #888);
  padding: 0;
  line-height: 1;
}

.close-btn:hover {
  color: var(--text-color, #333);
}

.filter-bar {
  display: flex;
  gap: 8px;
  padding: 10px 16px;
  background: var(--bg-color, #f5f7fa);
  border-bottom: 1px solid var(--border-color, #e0e0e0);
  overflow-x: auto;
}

.filter-btn {
  padding: 6px 14px;
  border: 1px solid var(--border-color, #ddd);
  border-radius: 16px;
  background: var(--card-bg, white);
  color: var(--text-color, #666);
  font-size: 13px;
  cursor: pointer;
  white-space: nowrap;
  transition: all 0.2s;
}

.filter-btn:hover {
  border-color: var(--primary-color, #667eea);
  color: var(--primary-color, #667eea);
}

.filter-btn.active {
  background: var(--primary-color, #667eea);
  border-color: var(--primary-color, #667eea);
  color: white;
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
  padding: 40px;
  text-align: center;
  color: var(--text-secondary, #888);
}

.log-list {
  padding: 10px;
}

.log-item {
  padding: 12px;
  border-radius: 8px;
  margin-bottom: 8px;
  background: var(--bg-color, #f5f5f5);
  border-left: 4px solid var(--primary-color, #667eea);
}

.log-item.danger {
  border-left-color: #f44336;
  background: #fff5f5;
}

.log-item.warning {
  border-left-color: #ff9800;
  background: #fff8f0;
}

.log-item.success {
  border-left-color: #4caf50;
  background: #f5fff5;
}

.log-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 6px;
}

.log-action {
  font-weight: 500;
  font-size: 14px;
}

.log-time {
  font-size: 12px;
  color: var(--text-secondary, #888);
}

.log-details {
  font-size: 12px;
  color: var(--text-secondary, #666);
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
}

.log-ip {
  background: var(--card-bg, white);
  padding: 2px 8px;
  border-radius: 4px;
}

.log-extra {
  color: var(--text-secondary, #888);
}

/* 移动端适配 */
@media (max-width: 768px) {
  .modal-overlay {
    padding: var(--spacing-sm);
    align-items: flex-end;
  }

  .audit-modal {
    max-width: 100%;
    max-height: 90vh;
    border-radius: var(--radius-lg) var(--radius-lg) 0 0;
  }

  .modal-header {
    padding: var(--spacing-sm) var(--spacing-md);
    border-bottom: 1px solid var(--border-color);
  }

  .modal-header h3 {
    font-size: 1rem;
    font-weight: 500;
    white-space: nowrap;
    color: var(--text-color-1);
  }

  .close-btn {
    font-size: 1.375rem;
    color: var(--text-color-2);
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
    font-size: 0.75rem;
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
    color: white;
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
    background: rgba(250, 42, 45, 0.1);
  }

  .log-item.warning {
    border-left-color: var(--warning-color);
    background: rgba(255, 176, 0, 0.1);
  }

  .log-item.success {
    border-left-color: var(--success-color);
    background: rgba(0, 186, 173, 0.1);
  }

  .log-header {
    margin-bottom: var(--spacing-xs);
  }

  .log-action {
    font-size: 0.875rem;
    font-weight: 500;
    color: var(--text-color-1);
  }

  .log-time {
    font-size: 0.6875rem;
    color: var(--text-color-3);
  }

  .log-details {
    font-size: 0.75rem;
    color: var(--text-color-2);
    flex-direction: column;
    gap: var(--spacing-xs);
  }

  .log-ip {
    padding: var(--spacing-xs) var(--spacing-sm);
    font-size: 0.75rem;
    background: var(--card-bg);
    border-radius: var(--radius-xs);
  }

  .log-extra {
    font-size: 0.75rem;
    color: var(--text-color-3);
  }

  .loading, .empty {
    padding: var(--spacing-2xl) var(--spacing-lg);
    font-size: 0.875rem;
    color: var(--text-color-2);
  }
}

@media (max-width: 480px) {
  .modal-overlay {
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
    font-size: 0.9375rem;
  }

  .close-btn {
    font-size: 1.25rem;
  }

  .filter-bar {
    padding: 4px var(--spacing-sm);
    gap: 4px;
  }

  .filter-btn {
    padding: 4px var(--spacing-xs);
    font-size: 0.6875rem;
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
    font-size: 0.8125rem;
  }

  .log-time {
    font-size: 0.625rem;
  }

  .log-details {
    font-size: 0.6875rem;
    gap: 2px;
  }

  .log-ip {
    padding: 2px var(--spacing-xs);
    font-size: 0.6875rem;
  }

  .log-extra {
    font-size: 0.6875rem;
  }

  .loading, .empty {
    padding: var(--spacing-xl) var(--spacing-md);
    font-size: 0.8125rem;
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
    font-size: 1.0625rem;
  }
}

/* 深色主题 */
:global(.dark-theme) .log-item {
  background: var(--bg-color-2);
}

:global(.dark-theme) .log-item.danger {
  background: rgba(250, 42, 45, 0.1);
}

:global(.dark-theme) .log-item.warning {
  background: rgba(255, 176, 0, 0.1);
}

:global(.dark-theme) .log-item.success {
  background: rgba(0, 186, 173, 0.1);
}

:global(.dark-theme) .log-ip {
  background: var(--bg-color-1);
}
</style>
