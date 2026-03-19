<template>
  <div class="modal-overlay" @click.self="$emit('close')">
    <div class="event-logger-panel">
      <div class="panel-header">
        <h3>系统日志监控</h3>
        <button class="close-btn" @click="$emit('close')">×</button>
      </div>
      
      <div class="panel-body">
        <!-- 状态显示 -->
        <div class="status-card" :class="{ active: status?.isRunning, error: status?.lastError }">
          <div class="status-icon">
            <span v-if="status?.isRunning" class="running">●</span>
            <span v-else class="stopped">●</span>
          </div>
          <div class="status-info">
            <div class="status-title">
              {{ status?.isRunning ? '监控中' : '已停止' }}
            </div>
            <div class="status-detail" v-if="status?.dbAccessible">
              数据库: {{ status?.dbPath }}
            </div>
            <div class="status-error" v-else-if="status?.lastError">
              {{ status.lastError }}
            </div>
            <div class="status-detail" v-if="status && status.totalEventsProcessed && status.totalEventsProcessed > 0">
              已处理事件: {{ status.totalEventsProcessed }}
            </div>
          </div>
        </div>

        <!-- 配置表单 -->
        <div class="config-section">
          <h4>监控配置</h4>
          
          <div class="form-item">
            <label>数据库路径</label>
            <input 
              type="text" 
              v-model="config.dbPath" 
              placeholder="/usr/trim/var/eventlogger_service/logger_data.db3"
              :disabled="loading"
            >
          </div>

          <div class="form-item">
            <label>
              <input type="checkbox" v-model="config.enabled" :disabled="loading">
              启用监控
            </label>
          </div>

          <div class="form-item">
            <label>检查间隔（秒）</label>
            <input 
              type="number" 
              v-model.number="config.checkInterval" 
              min="5"
              max="300"
              :disabled="loading"
            >
          </div>

          <div class="form-actions">
            <button class="save-btn" @click="saveConfig" :disabled="loading">
              {{ loading ? '保存中...' : '保存配置' }}
            </button>
          </div>
        </div>

        <!-- 操作按钮 -->
        <div class="action-section">
          <h4>操作</h4>
          <div class="action-buttons">
            <button 
              v-if="!status?.isRunning" 
              class="action-btn start" 
              @click="startMonitor"
              :disabled="loading"
            >
              启动监控
            </button>
            <button 
              v-else 
              class="action-btn stop" 
              @click="stopMonitor"
              :disabled="loading"
            >
              停止监控
            </button>
            <button 
              class="action-btn" 
              @click="forceCheck"
              :disabled="loading || !status?.isRunning"
            >
              立即检查
            </button>
          </div>
        </div>

        <!-- 统计信息 -->
        <div class="stats-section" v-if="stats">
          <h4>事件统计</h4>
          <div class="stats-grid">
            <div class="stat-item">
              <span class="stat-value">{{ stats.totalEvents }}</span>
              <span class="stat-label">总事件数</span>
            </div>
            <div class="stat-item" v-for="(count, severity) in stats.eventsBySeverity" :key="severity">
              <span class="stat-value">{{ count }}</span>
              <span class="stat-label">{{ formatSeverity(severity) }}</span>
            </div>
          </div>
        </div>

        <!-- 最近事件 -->
        <div class="events-section">
          <h4>最近事件</h4>
          <div class="events-list" v-if="events.length > 0">
            <div 
              class="event-item" 
              v-for="event in events.slice(0, 10)" 
              :key="event.id"
              :class="event.severity"
            >
              <div class="event-header">
                <span class="event-source">{{ event.source }}</span>
                <span class="event-severity">{{ formatSeverity(event.severity) }}</span>
              </div>
              <div class="event-message">{{ event.message }}</div>
              <div class="event-time">{{ formatTime(event.timestamp) }}</div>
            </div>
          </div>
          <div class="no-events" v-else>
            暂无事件数据
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { 
  eventLoggerApi, 
  type EventLoggerConfig, 
  type EventLoggerStatus, 
  type EventLoggerStats,
  type EventLogEntry 
} from '../services/api'

const emit = defineEmits<{
  close: []
}>()

const loading = ref(false)
const status = ref<EventLoggerStatus | null>(null)
const config = ref<EventLoggerConfig>({
  dbPath: '/usr/trim/var/eventlogger_service/logger_data.db3',
  enabled: false,
  checkInterval: 30000,
  eventTypes: [],
  minSeverity: 'info',
  notificationChannels: []
})
const stats = ref<EventLoggerStats | null>(null)
const events = ref<EventLogEntry[]>([])

const severityColors: Record<string, string> = {
  debug: '#9ca3af',
  info: '#3b82f6',
  warning: '#f59e0b',
  error: '#ef4444',
  critical: '#dc2626'
}

function formatSeverity(severity: string): string {
  const map: Record<string, string> = {
    debug: '调试',
    info: '普通',
    warning: '警告',
    error: '错误',
    critical: '严重'
  }
  return map[severity] || severity
}

function formatTime(timestamp: string): string {
  if (!timestamp) return ''
  // 使用和审计日志相同的方式格式化时间
  // 这样可以正确处理时区转换
  const date = new Date(timestamp)
  if (isNaN(date.getTime())) {
    // 如果解析失败，直接返回原始字符串
    console.log('[EventLogger] 时间解析失败:', timestamp)
    return timestamp
  }
  const formatted = date.toLocaleString('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hour12: false
  }).replace(/\//g, '-')
  return formatted
}

async function loadData() {
  loading.value = true
  try {
    const [statusData, configData, statsData, eventsData] = await Promise.all([
      eventLoggerApi.getStatus(),
      eventLoggerApi.getConfig(),
      eventLoggerApi.getStats(),
      eventLoggerApi.getEvents({ limit: 20 })
    ])
    
    status.value = statusData
    // 将检查间隔从毫秒转换为秒
    config.value = {
      ...configData,
      checkInterval: Math.round((configData.checkInterval || 30000) / 1000)
    }
    stats.value = statsData
    events.value = eventsData.events
  } catch (e) {
    console.error('Failed to load event logger data:', e)
  } finally {
    loading.value = false
  }
}

async function saveConfig() {
  loading.value = true
  try {
    // 将检查间隔从毫秒转换为秒
    const configToSave = {
      ...config.value,
      checkInterval: config.value.checkInterval * 1000
    }
    await eventLoggerApi.updateConfig(configToSave)
    await loadData()
  } catch (e) {
    console.error('Failed to save config:', e)
  } finally {
    loading.value = false
  }
}

async function startMonitor() {
  loading.value = true
  try {
    await eventLoggerApi.start()
    await loadData()
  } catch (e) {
    console.error('Failed to start:', e)
  } finally {
    loading.value = false
  }
}

async function stopMonitor() {
  loading.value = true
  try {
    await eventLoggerApi.stop()
    await loadData()
  } catch (e) {
    console.error('Failed to stop:', e)
  } finally {
    loading.value = false
  }
}

async function forceCheck() {
  loading.value = true
  try {
    await eventLoggerApi.check()
    await loadData()
  } catch (e) {
    console.error('Failed to check:', e)
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  loadData()
})
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
  z-index: 1200;
  padding: var(--spacing-xl);
}

.event-logger-panel {
  background: var(--card-bg);
  border-radius: var(--radius-md);
  box-shadow: var(--shadow-xl);
  max-width: 900px;
  width: 95%;
  max-height: 90vh;
  display: flex;
  flex-direction: column;
}

.panel-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: var(--spacing-md) var(--spacing-xl);
  border-bottom: 1px solid var(--border-color);
  flex-shrink: 0;
}

.panel-header h3 {
  margin: 0;
  font-size: 1rem;
  font-weight: 500;
  color: var(--text-color-1);
}

.close-btn {
  background: none;
  border: none;
  font-size: 1.5rem;
  cursor: pointer;
  color: var(--text-color-2);
  padding: 0;
  line-height: 1;
}

.close-btn:hover {
  color: var(--text-color-1);
}

.panel-body {
  padding: var(--spacing-xl);
  overflow-y: auto;
  flex: 1;
}

.status-card {
  display: flex;
  align-items: flex-start;
  gap: var(--spacing-md);
  padding: var(--spacing-md);
  border-radius: var(--radius-sm);
  background: var(--bg-color-2);
  margin-bottom: var(--spacing-xl);
}

.status-card.active {
  border: 1px solid var(--success-color);
}

.status-card.error {
  border: 1px solid var(--error-color);
}

.status-icon .running {
  color: var(--success-color);
  font-size: 1.5rem;
}

.status-icon .stopped {
  color: var(--text-color-3);
  font-size: 1.5rem;
}

.status-info {
  flex: 1;
}

.status-title {
  font-weight: 500;
  color: var(--text-color-1);
  margin-bottom: 4px;
}

.status-detail {
  font-size: 0.8125rem;
  color: var(--text-color-2);
  word-break: break-all;
  line-height: 1.4;
}

.status-error {
  font-size: 0.8125rem;
  color: var(--error-color);
}

.config-section,
.action-section,
.stats-section,
.events-section {
  margin-bottom: var(--spacing-xl);
}

.config-section h4,
.action-section h4,
.stats-section h4,
.events-section h4 {
  margin: 0 0 var(--spacing-md) 0;
  font-size: 0.875rem;
  font-weight: 500;
  color: var(--text-color-1);
}

.form-item {
  margin-bottom: var(--spacing-md);
}

.form-item label {
  display: block;
  margin-bottom: var(--spacing-xs);
  font-size: 0.8125rem;
  color: var(--text-color-2);
}

.form-item input[type="text"],
.form-item input[type="number"],
.form-item select {
  width: 100%;
  padding: var(--spacing-sm) var(--spacing-md);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-xs);
  font-size: 0.875rem;
  background: var(--bg-color-2);
  color: var(--text-color-1);
  box-sizing: border-box;
}

.form-item input:disabled,
.form-item select:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.form-item input[type="checkbox"] {
  margin-right: var(--spacing-xs);
}

.form-actions {
  margin-top: var(--spacing-lg);
}

.save-btn {
  width: 100%;
  padding: var(--spacing-sm) var(--spacing-md);
  background: var(--primary-color);
  color: white;
  border: none;
  border-radius: var(--radius-xs);
  cursor: pointer;
  font-size: 0.875rem;
  transition: background var(--transition-fast);
}

.save-btn:hover:not(:disabled) {
  background: var(--primary-hover);
}

.save-btn:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.action-buttons {
  display: flex;
  gap: var(--spacing-sm);
  flex-wrap: wrap;
}

.action-btn {
  flex: 1;
  min-width: 100px;
  padding: var(--spacing-sm) var(--spacing-md);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-xs);
  background: var(--bg-color-2);
  color: var(--text-color-1);
  cursor: pointer;
  font-size: 0.8125rem;
  transition: all var(--transition-fast);
}

.action-btn:hover:not(:disabled) {
  background: var(--bg-color-3);
}

.action-btn:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.action-btn.start {
  background: var(--success-color);
  color: white;
  border-color: var(--success-color);
}

.action-btn.start:hover:not(:disabled) {
  background: #22c55e;
}

.action-btn.stop {
  background: var(--error-color);
  color: white;
  border-color: var(--error-color);
}

.action-btn.stop:hover:not(:disabled) {
  background: #dc2626;
}

.stats-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(80px, 1fr));
  gap: var(--spacing-sm);
}

.stat-item {
  text-align: center;
  padding: var(--spacing-sm);
  background: var(--bg-color-2);
  border-radius: var(--radius-xs);
}

.stat-value {
  display: block;
  font-size: 1.25rem;
  font-weight: 600;
  color: var(--text-color-1);
}

.stat-label {
  display: block;
  font-size: 0.75rem;
  color: var(--text-color-3);
  text-transform: capitalize;
}

.events-list {
  max-height: 500px;
  overflow-y: auto;
}

.event-item {
  padding: var(--spacing-sm);
  border-radius: var(--radius-xs);
  background: var(--bg-color-2);
  margin-bottom: var(--spacing-sm);
  border-left: 3px solid var(--text-color-3);
}

.event-item.debug {
  border-left-color: #9ca3af;
}

.event-item.info {
  border-left-color: #3b82f6;
}

.event-item.warning {
  border-left-color: #f59e0b;
}

.event-item.error {
  border-left-color: #ef4444;
}

.event-item.critical {
  border-left-color: #dc2626;
}

.event-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 4px;
}

.event-source {
  font-weight: 500;
  font-size: 0.8125rem;
  color: var(--text-color-1);
}

.event-severity {
  font-size: 0.6875rem;
  padding: 2px 6px;
  border-radius: 4px;
  background: var(--bg-color-3);
  color: var(--text-color-2);
  text-transform: uppercase;
}

.event-message {
  font-size: 0.8125rem;
  color: var(--text-color-2);
  word-break: break-word;
  margin-bottom: 4px;
}

.event-time {
  font-size: 0.6875rem;
  color: var(--text-color-3);
}

.no-events {
  text-align: center;
  padding: var(--spacing-xl);
  color: var(--text-color-3);
  font-size: 0.875rem;
}

@media (max-width: 600px) {
  .event-logger-panel {
    max-width: 100%;
    max-height: 90vh;
  }
  
  .panel-body {
    padding: var(--spacing-md);
  }
  
  .action-buttons {
    flex-direction: column;
  }
  
  .action-btn {
    width: 100%;
  }
}
</style>
