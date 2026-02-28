<template>
  <div class="modal-overlay" @click.self="$emit('close')">
    <div class="modal audit-modal">
      <div class="modal-header">
        <h3>📋 审计日志</h3>
        <button class="close-btn" @click="$emit('close')">×</button>
      </div>
      <div class="modal-body">
        <div v-if="loading" class="loading">加载中...</div>
        <div v-else-if="logs.length === 0" class="empty">暂无审计日志</div>
        <div v-else class="log-list">
          <div v-for="(log, index) in logs" :key="index" class="log-item" :class="getLogClass(log.action)">
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
import { ref, onMounted } from 'vue'
import api from '../services/api'

defineEmits(['close'])

const logs = ref([])
const loading = ref(true)

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
    'login_success': '✅ 登录成功',
    'login_failed': '❌ 登录失败',
    'login_locked': '🔒 账户锁定',
    'logout': '🚪 登出',
    'password_changed': '🔑 密码修改',
    'password_change_failed': '❌ 密码修改失败',
    'log_truncate': '🗑️ 日志清空',
    'log_delete': '🗑️ 日志删除',
    'archive_delete': '🗑️ 归档删除',
    'logs_clean': '🧹 批量清理',
    'auth_failed': '❌ 认证失败'
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
  if (details.ip) parts.push(`IP: ${details.ip}`)
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
</style>
