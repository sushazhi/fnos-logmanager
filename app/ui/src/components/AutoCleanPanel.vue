<template>
  <div class="modal-overlay" @click.self="$emit('close')">
    <div class="auto-clean-panel">
      <div class="panel-header">
        <h3>自动清理策略</h3>
        <button class="close-btn" @click="$emit('close')">×</button>
      </div>

      <div class="panel-body">
        <div class="section">
          <div class="section-header">
            <h4>清理规则</h4>
            <button class="add-btn" @click="showAddForm = true" :disabled="showAddForm">+ 添加规则</button>
          </div>

          <div v-if="showAddForm" class="add-form">
            <div class="form-row">
              <label>规则名称</label>
              <input type="text" v-model="newRule.name" placeholder="例如：每日清理大文件">
            </div>
            <div class="form-row">
              <label>清理类型</label>
              <select v-model="newRule.type">
                <option value="truncateLarge">清空大文件内容</option>
                <option value="deleteOld">删除旧归档文件</option>
                <option value="deleteUninstalled">删除未安装应用日志</option>
              </select>
            </div>
            <div class="form-row" v-if="newRule.type === 'truncateLarge'">
              <label>大小阈值</label>
              <input type="text" v-model="newRule.threshold" placeholder="例如: 100M">
            </div>
            <div class="form-row" v-if="newRule.type === 'deleteOld'">
              <label>天数</label>
              <input type="number" v-model.number="newRule.days" min="1" max="365" placeholder="例如: 7">
            </div>
            <div class="form-row">
              <label>执行计划</label>
              <select v-model="newRule.schedule">
                <option value="hourly">每小时</option>
                <option value="daily">每天凌晨3点</option>
                <option value="weekly">每周日凌晨3点</option>
                <option value="custom">自定义间隔</option>
                <option value="cron">Cron表达式</option>
              </select>
            </div>
            <div class="form-row" v-if="newRule.schedule === 'custom'">
              <label>间隔(秒)</label>
              <input type="number" v-model.number="newRule.customInterval" min="60" placeholder="例如: 3600">
            </div>
            <div class="form-row" v-if="newRule.schedule === 'cron'">
              <label>Cron表达式 (分 时 日 月 周)</label>
              <input type="text" v-model="newRule.cronExpression" placeholder="例如: 0 3 * * * (每天3点)">
              <span class="hint">格式: 分(0-59) 时(0-23) 日(1-31) 月(1-12) 周(0-6,0=周日)</span>
            </div>
            <div class="form-actions">
              <button class="secondary" @click="showAddForm = false">取消</button>
              <button class="primary" @click="addRule" :disabled="!canAddRule">添加</button>
            </div>
          </div>

          <div v-if="loading" class="loading-text">加载中...</div>
          <div v-else-if="rules.length === 0" class="empty-text">暂无清理规则，点击上方按钮添加</div>
          <div v-else class="rule-list">
            <div v-for="rule in rules" :key="rule.id" class="rule-item">
              <div class="rule-main">
                <div class="rule-info">
                  <span class="rule-name">{{ rule.name }}</span>
                  <span class="rule-type">{{ typeLabel(rule.type) }}</span>
                  <span class="rule-condition" v-if="rule.type === 'truncateLarge'">阈值: {{ rule.threshold }}</span>
                  <span class="rule-condition" v-else-if="rule.type === 'deleteOld'">{{ rule.days }}天前</span>
                  <span class="rule-schedule">{{ scheduleLabel(rule.schedule) }}</span>
                </div>
                <div class="rule-meta">
                  <span class="rule-last-run" v-if="rule.lastRun">上次执行: {{ formatTime(rule.lastRun) }}</span>
                  <span class="rule-last-run" v-else>未执行</span>
                </div>
              </div>
              <div class="rule-actions">
                <label class="switch">
                  <input type="checkbox" :checked="rule.enabled" @change="toggleRule(rule.id)">
                  <span class="slider"></span>
                </label>
                <button class="action-btn" @click="executeRule(rule.id)" title="立即执行">▶</button>
                <button class="action-btn edit" @click="editRule(rule)" title="编辑">✎</button>
                <button class="action-btn danger" @click="deleteRule(rule.id, rule.name)" title="删除">✕</button>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { autoCleanApi } from '../services/api'

interface AutoCleanRule {
  id: string
  name: string
  enabled: boolean
  type: 'truncateLarge' | 'deleteOld' | 'deleteUninstalled'
  threshold?: string
  days?: number
  schedule: string
  lastRun?: string
}

const emit = defineEmits<{
  close: []
}>()

const rules = ref<AutoCleanRule[]>([])
const loading = ref(false)
const showAddForm = ref(false)
const editingRuleId = ref<string | null>(null)

const newRule = ref({
  name: '',
  type: 'truncateLarge' as 'truncateLarge' | 'deleteOld' | 'deleteUninstalled',
  threshold: '100M',
  days: 7,
  schedule: 'daily' as string,
  customInterval: 3600,
  cronExpression: '0 3 * * *'
})

const canAddRule = computed(() => {
  if (!newRule.value.name.trim()) return false
  if (newRule.value.type === 'truncateLarge' && !newRule.value.threshold) return false
  if (newRule.value.type === 'deleteOld' && (!newRule.value.days || newRule.value.days < 1)) return false
  if (newRule.value.schedule === 'custom' && (!newRule.value.customInterval || newRule.value.customInterval < 60)) return false
  if (newRule.value.schedule === 'cron' && (!newRule.value.cronExpression.trim() || newRule.value.cronExpression.trim().split(/\s+/).length !== 5)) return false
  return true
})

function typeLabel(type: string): string {
  switch (type) {
    case 'truncateLarge': return '清空大文件'
    case 'deleteOld': return '删除旧文件'
    case 'deleteUninstalled': return '删除未安装应用'
    default: return type
  }
}

function scheduleLabel(schedule: string): string {
  switch (schedule) {
    case 'hourly': return '每小时'
    case 'daily': return '每天凌晨3点'
    case 'weekly': return '每周日凌晨3点'
    default: {
      if (/^\d+$/.test(schedule)) {
        const seconds = parseInt(schedule, 10)
        if (seconds >= 86400) return `每${Math.round(seconds / 86400)}天`
        if (seconds >= 3600) return `每${Math.round(seconds / 3600)}小时`
        return `每${Math.round(seconds / 60)}分钟`
      }
      if (schedule.trim().split(/\s+/).length === 5) {
        return `Cron: ${schedule}`
      }
      return schedule
    }
  }
}

function formatTime(iso: string): string {
  try {
    const d = new Date(iso)
    return d.toLocaleString('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' })
  } catch {
    return iso
  }
}

async function loadRules(): Promise<void> {
  loading.value = true
  try {
    const data = await autoCleanApi.getRules()
    rules.value = data.rules
  } catch (e) {
    console.error('加载自动清理规则失败:', e)
  } finally {
    loading.value = false
  }
}

async function addRule(): Promise<void> {
  try {
    let schedule: string
    if (newRule.value.schedule === 'custom') {
      schedule = String(newRule.value.customInterval)
    } else if (newRule.value.schedule === 'cron') {
      schedule = newRule.value.cronExpression.trim()
    } else {
      schedule = newRule.value.schedule
    }

    await autoCleanApi.addRule({
      name: newRule.value.name,
      enabled: true,
      type: newRule.value.type,
      threshold: newRule.value.type === 'truncateLarge' ? newRule.value.threshold : undefined,
      days: newRule.value.type === 'deleteOld' ? newRule.value.days : undefined,
      schedule
    })
    showAddForm.value = false
    newRule.value = {
      name: '',
      type: 'truncateLarge',
      threshold: '100M',
      days: 7,
      schedule: 'daily',
      customInterval: 3600,
      cronExpression: '0 3 * * *'
    }
    await loadRules()
  } catch (e) {
    console.error('添加规则失败:', e)
  }
}

async function toggleRule(id: string): Promise<void> {
  try {
    const result = await autoCleanApi.toggleRule(id)
    const index = rules.value.findIndex(r => r.id === id)
    if (index !== -1 && result.rule) {
      rules.value[index] = result.rule
    }
  } catch (e) {
    console.error('切换规则失败:', e)
  }
}

async function executeRule(id: string): Promise<void> {
  try {
    const result = await autoCleanApi.executeRule(id)
    if (result.cleaned !== undefined) {
      alert(`执行完成，清理了 ${result.cleaned} 个文件`)
    }
    await loadRules()
  } catch (e) {
    console.error('执行规则失败:', e)
  }
}

function editRule(rule: AutoCleanRule): void {
  editingRuleId.value = rule.id
  const isCron = rule.schedule.trim().split(/\s+/).length === 5 && !['hourly', 'daily', 'weekly'].includes(rule.schedule)
  const isCustomInt = /^\d+$/.test(rule.schedule) && !['hourly', 'daily', 'weekly'].includes(rule.schedule)
  newRule.value = {
    name: rule.name,
    type: rule.type,
    threshold: rule.threshold || '100M',
    days: rule.days || 7,
    schedule: isCron ? 'cron' : isCustomInt ? 'custom' : rule.schedule,
    customInterval: isCustomInt ? parseInt(rule.schedule, 10) : 3600,
    cronExpression: isCron ? rule.schedule : '0 3 * * *'
  }
  showAddForm.value = true
}

async function deleteRule(id: string, name: string): Promise<void> {
  if (!confirm(`确定要删除规则 "${name}" 吗？`)) return
  try {
    await autoCleanApi.deleteRule(id)
    await loadRules()
  } catch (e) {
    console.error('删除规则失败:', e)
  }
}

onMounted(() => {
  loadRules()
})
</script>

<style scoped>
.modal-overlay {
  position: fixed;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  background: var(--overlay);
  display: flex;
  justify-content: center;
  align-items: center;
  z-index: 1000;
}

.auto-clean-panel {
  background: var(--card-bg);
  border-radius: var(--radius-md);
  width: 90%;
  max-width: 700px;
  max-height: 85vh;
  display: flex;
  flex-direction: column;
  box-shadow: var(--shadow-xl);
}

.panel-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: var(--spacing-lg) var(--spacing-xl);
  border-bottom: 1px solid var(--border-color);
}

.panel-header h3 {
  margin: 0;
  font-size: 1.125rem;
  font-weight: 600;
  color: var(--text-color-1);
}

.close-btn {
  background: none;
  border: none;
  font-size: 1.5rem;
  cursor: pointer;
  color: var(--text-color-2);
  padding: 0 var(--spacing-sm);
  line-height: 1;
}

.close-btn:hover {
  color: var(--text-color-1);
}

.panel-body {
  padding: var(--spacing-lg) var(--spacing-xl);
  overflow-y: auto;
  flex: 1;
}

.section-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: var(--spacing-md);
}

.section-header h4 {
  margin: 0;
  font-size: 0.9375rem;
  font-weight: 500;
  color: var(--text-color-1);
}

.add-btn {
  padding: var(--spacing-xs) var(--spacing-md);
  background: var(--primary-color);
  color: white;
  border: none;
  border-radius: var(--radius-xs);
  cursor: pointer;
  font-size: 0.8125rem;
  font-weight: 500;
}

.add-btn:hover {
  opacity: 0.9;
}

.add-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.add-form {
  background: var(--bg-color-2);
  padding: var(--spacing-md);
  border-radius: var(--radius-sm);
  margin-bottom: var(--spacing-lg);
}

.form-row {
  margin-bottom: var(--spacing-md);
}

.form-row label {
  display: block;
  margin-bottom: var(--spacing-xs);
  font-size: 0.8125rem;
  font-weight: 500;
  color: var(--text-color-1);
}

.form-row input,
.form-row select {
  width: 100%;
  padding: var(--spacing-sm);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-xs);
  font-size: 0.8125rem;
  background: var(--card-bg);
  color: var(--text-color-1);
  box-sizing: border-box;
}

.form-row input:focus,
.form-row select:focus {
  outline: none;
  border-color: var(--primary-color);
}

.hint {
  display: block;
  margin-top: 4px;
  font-size: 0.6875rem;
  color: var(--text-color-3, #999);
}

.form-actions {
  display: flex;
  gap: var(--spacing-sm);
  justify-content: flex-end;
  margin-top: var(--spacing-md);
}

.form-actions button {
  padding: var(--spacing-sm) var(--spacing-lg);
  border: none;
  border-radius: var(--radius-xs);
  cursor: pointer;
  font-size: 0.8125rem;
  font-weight: 500;
}

.form-actions button.secondary {
  background: var(--bg-color-3);
  color: var(--text-color-1);
}

.form-actions button.primary {
  background: var(--primary-color);
  color: white;
}

.form-actions button.primary:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.loading-text,
.empty-text {
  text-align: center;
  color: var(--text-color-2);
  padding: var(--spacing-xl);
  font-size: 0.875rem;
}

.rule-list {
  display: flex;
  flex-direction: column;
  gap: var(--spacing-sm);
}

.rule-item {
  background: var(--bg-color-2);
  padding: var(--spacing-md);
  border-radius: var(--radius-sm);
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: var(--spacing-md);
}

.rule-main {
  flex: 1;
  min-width: 0;
}

.rule-info {
  display: flex;
  flex-wrap: wrap;
  gap: var(--spacing-sm);
  align-items: center;
  margin-bottom: var(--spacing-xs);
}

.rule-name {
  font-weight: 500;
  font-size: 0.875rem;
  color: var(--text-color-1);
}

.rule-type,
.rule-condition,
.rule-schedule {
  font-size: 0.75rem;
  padding: 2px 6px;
  border-radius: 3px;
  background: var(--bg-color-3);
  color: var(--text-color-2);
}

.rule-meta {
  font-size: 0.75rem;
  color: var(--text-color-3);
}

.rule-actions {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
  flex-shrink: 0;
}

.switch {
  position: relative;
  display: inline-block;
  width: 36px;
  height: 18px;
}

.switch input {
  opacity: 0;
  width: 0;
  height: 0;
}

.slider {
  position: absolute;
  cursor: pointer;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background-color: var(--bg-color-4);
  transition: var(--transition-base);
  border-radius: 18px;
}

.slider::before {
  position: absolute;
  content: "";
  height: 14px;
  width: 14px;
  left: 2px;
  bottom: 2px;
  background-color: white;
  transition: var(--transition-base);
  border-radius: 50%;
}

input:checked + .slider {
  background: var(--primary-color);
}

input:checked + .slider::before {
  transform: translateX(18px);
}

.action-btn {
  background: none;
  border: none;
  cursor: pointer;
  font-size: 0.875rem;
  padding: 2px 6px;
  color: var(--text-color-2);
  border-radius: 3px;
}

.action-btn:hover {
  background: var(--bg-color-3);
  color: var(--text-color-1);
}

.action-btn.danger:hover {
  color: var(--error-color);
}

@media (max-width: 600px) {
  .auto-clean-panel {
    width: 95%;
    max-height: 90vh;
  }

  .rule-item {
    flex-direction: column;
    align-items: flex-start;
  }

  .rule-actions {
    width: 100%;
    justify-content: flex-end;
  }
}
</style>
