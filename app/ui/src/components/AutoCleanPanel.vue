<template>
  <div class="hm-overlay-base" @click.self="$emit('close')">
    <div class="auto-clean-panel hm-modal-base">
      <div class="panel-header">
        <h3>自动清理策略</h3>
        <button class="close-btn" @click="$emit('close')">×</button>
      </div>

      <div class="panel-body">
        <div v-if="statusMsg" :class="['status-msg', statusType]" @click="statusMsg = ''">{{ statusMsg }}</div>
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
                <option value="truncateLarge">清空过大的日志文件</option>
                <option value="deleteOld">删除旧日志和归档文件</option>
                <option value="deleteUninstalled">清理未安装应用日志</option>
                <option value="cleanEmptyFiles">清理空文件和空目录</option>
              </select>
              <span class="hint">{{ typeHint }}</span>
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
              <div class="schedule-row">
                <select v-model="newRule.schedule">
                  <option value="hourly">每小时</option>
                  <option value="daily">每天凌晨3点</option>
                  <option value="weekly">每周日凌晨3点</option>
                  <option value="custom">自定义间隔</option>
                  <option value="cron">Cron表达式</option>
                </select>
                <input v-if="newRule.schedule === 'custom'" type="number" v-model.number="newRule.customInterval" min="60" placeholder="间隔秒数" class="schedule-input">
                <input v-if="newRule.schedule === 'cron'" type="text" v-model="newRule.cronExpression" placeholder="0 3 * * *" class="schedule-input">
              </div>
              <span class="hint" v-if="newRule.schedule === 'custom'">最小60秒</span>
              <span class="hint" v-if="newRule.schedule === 'cron'">格式: 分(0-59) 时(0-23) 日(1-31) 月(1-12) 周(0-6,0=周日)</span>
            </div>
            <div class="form-actions">
              <button class="secondary" @click="closeForm">取消</button>
              <button class="primary" @click="saveRule" :disabled="!canAddRule">{{ editingRuleId ? '保存' : '添加' }}</button>
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
import { autoCleanApi, type AutoCleanRule as CleanRuleDTO, type AutoCleanRuleInput } from '../services/api'
import { useStore } from '../composables/useStore'

// UI 展示模型（前后端字段名不同，通过下方映射函数互转）
interface AutoCleanRule {
  id: string
  name: string
  enabled: boolean
  type: 'truncateLarge' | 'deleteOld' | 'deleteUninstalled' | 'cleanEmptyFiles'
  threshold?: string
  days?: number
  schedule: string
  lastRun?: string
}

// ============ 前后端契约映射 ============
// UI type -> 后端 action
const typeToAction: Record<AutoCleanRule['type'], string> = {
  truncateLarge: 'truncate',
  deleteOld: 'delete',
  deleteUninstalled: 'deleteUninstalled',
  cleanEmptyFiles: 'cleanEmpty'
}
// 后端 action -> UI type
const actionToType: Record<string, AutoCleanRule['type']> = {
  truncate: 'truncateLarge',
  delete: 'deleteOld',
  deleteUninstalled: 'deleteUninstalled',
  cleanEmpty: 'cleanEmptyFiles'
}

// 阈值 量化字符串 -> 字节数，如 "100M" -> 104857600
function parseSizeToBytes(size: string): number {
  const m = /^(\d+(?:\.\d+)?)\s*([KMGT]?)$/i.exec(size.trim())
  if (!m) return 0
  const n = parseFloat(m[1])
  const unit = (m[2] || '').toUpperCase()
  const mult: Record<string, number> = { K: 1024, M: 1024 ** 2, G: 1024 ** 3, T: 1024 ** 4 }
  return Math.round(n * (mult[unit] || 1))
}

// 字节数 -> 量化字符串
function formatBytes(bytes: number): string {
  if (bytes <= 0) return ''
  const units: Array<[number, string]> = [
    [1024 ** 4, 'T'],
    [1024 ** 3, 'G'],
    [1024 ** 2, 'M'],
    [1024, 'K']
  ]
  for (const [v, u] of units) {
    if (bytes >= v) {
      const n = bytes / v
      return `${n % 1 === 0 ? n : n.toFixed(1)}${u}`
    }
  }
  return String(bytes)
}

// 后端 schedule 只接受 cron 表达式或 "Ns" 秒级间隔，UI 的选择需转换为后端格式
function scheduleToBackend(sched: string, customInterval: number, cronExpr: string): string {
  switch (sched) {
    case 'hourly': return '0 * * * *'
    case 'daily': return '0 3 * * *'
    case 'weekly': return '0 3 * * 0'
    case 'custom': return `${customInterval}s`
    case 'cron': return cronExpr.trim()
    default: return sched
  }
}

// 后端 schedule 表达式 -> UI 调度选择
function scheduleFromBackend(sched: string): { schedule: string; customInterval: number; cronExpression: string } {
  switch (sched) {
    case '0 * * * *': return { schedule: 'hourly', customInterval: 3600, cronExpression: '0 3 * * *' }
    case '0 3 * * *': return { schedule: 'daily', customInterval: 3600, cronExpression: '0 3 * * *' }
    case '0 3 * * 0': return { schedule: 'weekly', customInterval: 3600, cronExpression: '0 3 * * *' }
    default: {
      const secMatch = /^(\d+)s$/.exec(sched)
      if (secMatch) {
        const secs = parseInt(secMatch[1], 10)
        return { schedule: 'custom', customInterval: secs || 3600, cronExpression: '0 3 * * *' }
      }
      return { schedule: 'cron', customInterval: 3600, cronExpression: sched && sched.trim().split(/\s+/).length === 5 ? sched : '0 3 * * *' }
    }
  }
}

// 后端 CleanRule DTO -> UI 展示模型
function toDisplayRule(r: CleanRuleDTO): AutoCleanRule {
  const type: AutoCleanRule['type'] = actionToType[r.action] || 'truncateLarge'
  const sched = scheduleFromBackend(r.schedule || '')
  let uiSchedule = sched.schedule
  if (sched.schedule === 'custom') uiSchedule = String(sched.customInterval)
  else if (sched.schedule === 'cron') uiSchedule = sched.cronExpression
  return {
    id: r.id,
    name: r.name,
    enabled: r.enabled,
    type,
    threshold: type === 'truncateLarge' && r.minSizeBytes > 0 ? formatBytes(r.minSizeBytes) : undefined,
    days: type === 'deleteOld' && r.retentionDays > 0 ? r.retentionDays : undefined,
    schedule: uiSchedule,
    lastRun: r.lastRun || undefined
  }
}

const emit = defineEmits<{
  close: []
}>()

const { confirm: showConfirm } = useStore()
const statusMsg = ref('')
const statusType = ref<'success' | 'error' | ''>('')

const rules = ref<AutoCleanRule[]>([])
const loading = ref(false)
const showAddForm = ref(false)
const editingRuleId = ref<string | null>(null)

const newRule = ref({
  name: '',
  type: 'truncateLarge' as 'truncateLarge' | 'deleteOld' | 'deleteUninstalled' | 'cleanEmptyFiles',
  threshold: '100M',
  days: 7,
  schedule: 'daily' as string,
  customInterval: 3600,
  cronExpression: '0 3 * * *'
})

// 各清理类型的说明需与后端实际行为一致（如未安装应用日志现移入回收站而非直接删除）
const typeHint = computed(() => {
  switch (newRule.value.type) {
    case 'truncateLarge':
    case 'deleteOld':
      return '规则作用于所有日志目录，仅匹配日志/归档文件'
    case 'deleteUninstalled':
      return '已卸载应用的日志文件移入回收站，空文件夹直接清理，非空残留目录仅通知提醒'
    case 'cleanEmptyFiles':
      return '删除 0 字节的日志/归档文件（已安装应用的除外），及已卸载应用遗留的空文件夹'
  }
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
    case 'truncateLarge': return '清空大日志'
    case 'deleteOld': return '删除旧日志/归档'
    case 'deleteUninstalled': return '清理未安装应用'
    case 'cleanEmptyFiles': return '清理空文件'
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
    rules.value = (data.rules || []).map(toDisplayRule)
  } catch (e) {
    statusMsg.value = getErrorMessage(e, '加载自动清理规则失败')
    statusType.value = 'error'
  } finally {
    loading.value = false
  }
}

function closeForm(): void {
  showAddForm.value = false
  // Reset the edit target too, otherwise the next "+ 添加规则" would silently
  // overwrite the rule that was being edited
  editingRuleId.value = null
  newRule.value = {
    name: '',
    type: 'truncateLarge',
    threshold: '100M',
    days: 7,
    schedule: 'daily',
    customInterval: 3600,
    cronExpression: '0 3 * * *'
  }
}

async function saveRule(): Promise<void> {
  try {
    const payload: AutoCleanRuleInput = {
      name: newRule.value.name,
      action: typeToAction[newRule.value.type],
      schedule: scheduleToBackend(newRule.value.schedule, newRule.value.customInterval, newRule.value.cronExpression),
      minSizeBytes: newRule.value.type === 'truncateLarge' ? parseSizeToBytes(newRule.value.threshold || '') : 0,
      retentionDays: newRule.value.type === 'deleteOld' ? (newRule.value.days || 0) : 0
    }
    if (editingRuleId.value) {
      // Omit enabled so saving an edit never flips the rule's toggle state
      await autoCleanApi.updateRule(editingRuleId.value, payload)
      statusMsg.value = '保存规则成功'
    } else {
      await autoCleanApi.addRule({ ...payload, enabled: true })
      statusMsg.value = '添加规则成功'
    }
    statusType.value = 'success'
    closeForm()
    await loadRules()
  } catch (e) {
    statusMsg.value = getErrorMessage(e, '保存规则失败')
    statusType.value = 'error'
  }
}

async function toggleRule(id: string): Promise<void> {
  try {
    const result = await autoCleanApi.toggleRule(id)
    const index = rules.value.findIndex(r => r.id === id)
    if (index !== -1 && result.rule) {
      rules.value[index] = toDisplayRule(result.rule)
    }
  } catch (e) {
    statusMsg.value = getErrorMessage(e, '切换规则失败')
    statusType.value = 'error'
  }
}

function getErrorMessage(e: unknown, fallback: string): string {
  return e instanceof Error && e.message ? e.message : fallback
}

async function executeRule(id: string): Promise<void> {
  try {
    const result = await autoCleanApi.executeRule(id)
    if (result.cleaned !== undefined) {
      statusMsg.value = `执行完成，清理了 ${result.cleaned} 项`
      statusType.value = 'success'
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
  if (!await showConfirm({ message: `确定要删除规则 "${name}" 吗？` })) return
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
.status-msg {
  padding: var(--spacing-sm) var(--spacing-md);
  border-radius: var(--radius-2xs);
  margin-bottom: var(--spacing-md);
  cursor: pointer;
  font-size: var(--font-size-base);
}
.status-msg.success {
  background: var(--success-bg);
  color: var(--success-color);
  border: 1px solid var(--success-color);
}
.status-msg.error {
  background: var(--error-bg);
  color: var(--error-color);
  border: 1px solid var(--error-color);
}
.auto-clean-panel {
  width: 90%;
  max-width: 700px;
  max-height: 85vh;
  display: flex;
  flex-direction: column;
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
  font-size: var(--font-size-2xl);
  font-weight: 600;
  color: var(--text-color-1);
}

.close-btn {
  background: none;
  border: none;
  font-size: var(--font-size-5xl);
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
  font-size: var(--font-size-lg);
  font-weight: 500;
  color: var(--text-color-1);
}

.add-btn {
  flex-shrink: 0;
  white-space: nowrap;
  padding: var(--spacing-sm) var(--spacing-lg);
  background: var(--primary-color);
  color: var(--text-color-on-primary);
  border: none;
  border-radius: var(--radius-sm);
  cursor: pointer;
  font-size: var(--font-size-base);
  font-weight: 500;
}

.add-btn:hover {
  background: var(--primary-hover);
}

.add-btn:active {
  transform: scale(0.97);
  box-shadow: 0 0 20px var(--glow-primary);
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
  font-size: var(--font-size-base);
  font-weight: 500;
  color: var(--text-color-1);
}

.form-row input,
.form-row select {
  width: 100%;
  padding: var(--spacing-sm);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-xs);
  font-size: var(--font-size-base);
  background: var(--card-bg);
  color: var(--text-color-1);
  box-sizing: border-box;
}

.form-row input:focus,
.form-row select:focus {
  outline: none;
  border-color: var(--primary-color);
}

.schedule-row {
  display: flex;
  gap: var(--spacing-sm);
  align-items: center;
}

.schedule-row select {
  flex-shrink: 0;
  width: auto;
  max-width: 50%;
}

.schedule-input {
  flex: 1;
  min-width: 0;
  padding: var(--spacing-sm);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-xs);
  font-size: var(--font-size-base);
  background: var(--card-bg);
  color: var(--text-color-1);
  box-sizing: border-box;
}

.schedule-input:focus {
  outline: none;
  border-color: var(--primary-color);
}

.hint {
  display: block;
  margin-top: var(--spacing-xs);
  font-size: var(--font-size-xs);
  color: var(--text-color-3);
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
  border-radius: var(--radius-sm);
  cursor: pointer;
  font-size: var(--font-size-base);
  font-weight: 500;
}

.form-actions button.secondary {
  background: var(--bg-color-2);
  color: var(--text-color-1);
}

.form-actions button.secondary:hover {
  background: var(--bg-color-3);
}

.form-actions button.primary {
  background: var(--primary-color);
  color: var(--text-color-on-primary);
}

.form-actions button.primary:hover {
  background: var(--primary-hover);
}

.form-actions button:active {
  transform: scale(0.97);
}

.form-actions button.primary:active {
  box-shadow: 0 0 20px var(--glow-primary);
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
  font-size: var(--font-size-md);
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
  font-size: var(--font-size-md);
  color: var(--text-color-1);
}

.rule-type,
.rule-condition,
.rule-schedule {
  font-size: var(--font-size-sm);
  padding: var(--spacing-xs) var(--spacing-sm);
  border-radius: var(--radius-3xs);
  background: var(--bg-color-3);
  color: var(--text-color-2);
}

.rule-meta {
  font-size: var(--font-size-sm);
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
  border-radius: var(--radius-full);
}

.slider::before {
  position: absolute;
  content: "";
  height: 14px;
  width: 14px;
  left: 2px;
  bottom: 2px;
  background-color: var(--text-color-on-primary);
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
  font-size: var(--font-size-md);
  padding: var(--spacing-xs) var(--spacing-sm);
  color: var(--text-color-2);
  border-radius: var(--radius-3xs);
}

.action-btn:hover {
  background: var(--bg-color-3);
  color: var(--text-color-1);
  box-shadow: 0 0 20px var(--glow-primary);
}

.action-btn.danger:hover {
  color: var(--error-color);
  box-shadow: 0 0 20px var(--glow-primary-strong);
}

@media (max-width: 600px) {
  .auto-clean-panel {
    width: 95%;
    max-height: 90vh;
  }

  .panel-header {
    padding: var(--spacing-sm) var(--spacing-md);
  }

  .panel-header h3 {
    font-size: var(--font-size-lg);
  }

  .close-btn {
    font-size: var(--font-size-3xl);
  }

  .panel-body {
    padding: var(--spacing-md);
  }

  .rule-item {
    flex-direction: column;
    align-items: flex-start;
    padding: var(--spacing-sm);
  }

  .rule-fields {
    width: 100%;
  }

  .rule-fields input,
  .rule-fields select {
    width: 100%;
  }

  .rule-actions {
    width: 100%;
    justify-content: flex-end;
    margin-top: var(--spacing-xs);
  }

  .add-form {
    flex-direction: column;
    gap: var(--spacing-xs);
  }

  .add-form input,
  .add-form select {
    width: 100%;
  }
}

@media (max-width: 480px) {
  .auto-clean-panel {
    width: 100%;
    height: 95vh;
    max-height: 95vh;
    border-radius: var(--radius-lg) var(--radius-lg) 0 0;
  }

  .panel-header {
    padding: var(--spacing-xs) var(--spacing-sm);
  }

  .panel-header h3 {
    font-size: var(--font-size-base);
  }

  .panel-body {
    padding: var(--spacing-sm);
  }

  .form-group label {
    font-size: var(--font-size-sm);
  }

  .form-group input,
  .form-group select {
    padding: var(--spacing-sm) var(--spacing-xs);
    font-size: var(--font-size-base);
    height: 44px;
  }

  .action-btn {
    font-size: var(--font-size-sm);
    padding: var(--spacing-xs) var(--spacing-sm);
  }
}
</style>
