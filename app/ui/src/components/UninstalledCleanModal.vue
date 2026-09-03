<template>
  <div class="hm-overlay-base" @click.self="$emit('close')">
    <div class="modal-content hm-modal-base">
      <div class="modal-header">清理已卸载应用残留</div>
      <div class="modal-body">
        <p class="modal-desc">
          仅处理<strong>已卸载应用</strong>残留的数据，不会影响已安装应用。请选择要执行的清理操作：
        </p>

        <div class="option-list">
          <div class="option-item">
            <div class="option-info">
              <div class="option-title">清理空文件夹</div>
              <div class="option-desc">直接删除已卸载应用留下的空文件夹（不含任何数据），保持数据目录整洁。</div>
            </div>
            <button class="secondary hm-ripple-btn" :disabled="running" @click="run('empty')">
              执行
            </button>
          </div>

          <div class="option-item danger-option">
            <div class="option-info">
              <div class="option-title">扫描残留并选择性清理</div>
              <div class="option-desc">
                扫描已卸载应用遗留的数据目录、失效链接和系统账号（安装 Docker 应用时创建的 docker-* 账号），勾选确认后清理。目录移入回收站（{{ retentionHours }} 小时内可还原），链接和账号直接删除。
              </div>
            </div>
            <button class="danger hm-ripple-btn" :disabled="running || scanning" @click="scan">
              {{ scanning ? '扫描中...' : '扫描' }}
            </button>
          </div>
        </div>

        <div v-if="scanError" class="scan-error">{{ scanError }}</div>

        <div v-if="scanned" class="scan-section">
          <div class="scan-header">
            <span>扫描结果（勾选要清理的项目）</span>
            <div class="scan-header-actions">
              <button class="refresh-btn" @click="toggleSelectAll">
                {{ allSelected ? '全不选' : '全选' }}
              </button>
              <button class="refresh-btn" :disabled="scanning" @click="scan">重新扫描</button>
            </div>
          </div>

          <div v-if="scanEmpty" class="recycle-empty">未发现已卸载应用的残留</div>
          <template v-else>
            <div v-if="scanResult.dirs.length" class="scan-group">
              <div class="scan-group-title">
                残留目录（{{ scanResult.dirs.length }}）
                <span class="scan-group-hint">移入回收站，{{ retentionHours }} 小时内可还原</span>
              </div>
              <label v-for="item in scanResult.dirs" :key="'d-' + item.path" class="scan-item">
                <input type="checkbox" :value="item.path" v-model="selectedDirs" />
                <span class="scan-item-main">
                  <span class="scan-item-name">
                    {{ item.app }}
                    <span class="risk-badge" :class="'risk-' + item.risk">{{ riskLabel(item.risk) }}</span>
                    <span v-if="item.rootType" class="scan-item-root">@app{{ item.rootType }}</span>
                  </span>
                  <span class="scan-item-path">{{ item.path }}</span>
                </span>
                <span class="scan-item-size">{{ item.sizeFormatted }}</span>
              </label>
            </div>

            <div v-if="scanResult.links.length" class="scan-group">
              <div class="scan-group-title">
                残留符号链接（{{ scanResult.links.length }}）
                <span class="scan-group-hint">仅删除链接本身，直接清理</span>
              </div>
              <label v-for="item in scanResult.links" :key="'l-' + item.path" class="scan-item">
                <input type="checkbox" :value="item.path" v-model="selectedLinks" />
                <span class="scan-item-main">
                  <span class="scan-item-name">{{ item.app }}</span>
                  <span class="scan-item-path">{{ item.path }} → {{ item.detail }}</span>
                </span>
              </label>
            </div>

            <div v-if="scanResult.users.length" class="scan-group">
              <div class="scan-group-title">
                遗留系统账号（{{ scanResult.users.length }}）
                <span class="scan-group-hint">Docker 应用卸载后遗留的 docker-* 账号，删除后不可恢复</span>
              </div>
              <label v-for="item in scanResult.users" :key="'u-' + item.path" class="scan-item">
                <input type="checkbox" :value="item.path" v-model="selectedUsers" />
                <span class="scan-item-main">
                  <span class="scan-item-name">{{ item.path }}</span>
                  <span class="scan-item-path">{{ item.detail }}</span>
                </span>
              </label>
            </div>

            <div class="scan-actions">
              <button
                class="danger hm-ripple-btn"
                :disabled="running || selectedCount === 0"
                @click="cleanSelected"
              >
                清理所选（{{ selectedCount }}）
              </button>
            </div>
          </template>
        </div>

        <div class="recycle-section">
          <div class="recycle-header">
            <span>回收站内容（{{ retentionHours }} 小时自动清空，可手动还原）</span>
            <button class="refresh-btn" @click="loadRecycle">刷新</button>
          </div>

          <div class="retention-row">
            <label class="retention-label" for="retention-input">保留时长（小时）</label>
            <input
              id="retention-input"
              class="retention-input"
              type="number"
              min="1"
              max="8760"
              v-model.number="retentionInput"
              :disabled="savingRetention"
            />
            <button
              class="refresh-btn"
              :disabled="savingRetention || retentionInput === retentionHours"
              @click="saveRetention"
            >
              {{ savingRetention ? '保存中...' : '保存' }}
            </button>
          </div>

          <div v-if="retentionMsg" class="retention-msg" :class="{ error: retentionError }">{{ retentionMsg }}</div>

          <div v-if="restoreError" class="recycle-error">{{ restoreError }}</div>
          <div v-if="recycleError" class="recycle-empty">{{ recycleError }}</div>
          <div v-else-if="!recycleLoaded" class="recycle-empty">加载中...</div>
          <div v-else-if="recycleItems.length === 0" class="recycle-empty">回收站为空</div>
          <div v-else class="recycle-list">
            <div v-for="item in recycleItems" :key="item.relPath" class="recycle-item">
              <div class="recycle-item-head">
                <div class="recycle-item-name">{{ item.name }}</div>
                <button class="restore-btn" :disabled="restoring" @click="restoreItem(item)">
                  {{ restoring ? '还原中...' : '还原' }}
                </button>
              </div>
              <div class="recycle-item-meta">
                <span v-if="item.originalPath" class="recycle-original" title="原始位置">
                  原始位置：{{ item.originalPath }}
                </span>
                <span v-else class="recycle-original muted">原始位置：未知</span>
                <span class="recycle-misc">{{ item.sizeFormatted }} · 移入 {{ formatTime(item.movedAt) }}</span>
                <span v-if="expiryTime(item.movedAt)" class="recycle-expiry">将于 {{ expiryTime(item.movedAt) }} 永久清空</span>
              </div>
            </div>
          </div>
        </div>
      </div>
      <div class="modal-footer">
        <button class="secondary hm-ripple-btn" @click="$emit('close')">关闭</button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import api from '../services/api'

interface RecycleItem {
  name: string
  relPath: string
  root: string
  trashPath: string
  originalPath: string
  size: number
  sizeFormatted: string
  modified: string
  movedAt: string
}

interface LeftoverCandidate {
  kind: string
  app: string
  path: string
  rootType?: string
  size?: number
  sizeFormatted?: string
  risk: string
  detail?: string
}

interface ScanResult {
  dirs: LeftoverCandidate[]
  links: LeftoverCandidate[]
  users: LeftoverCandidate[]
  retentionHours: number
  errors?: string[]
}

export interface UninstalledCleanSelection {
  dirs: string[]
  links: string[]
  users: string[]
  retentionHours: number
}

const emit = defineEmits<{
  close: []
  cleanEmpty: []
  cleanTrash: [selection: UninstalledCleanSelection]
}>()

const running = ref(false)
const restoring = ref(false)
const recycleItems = ref<RecycleItem[]>([])
const recycleLoaded = ref(false)
const recycleError = ref('')
// 还原失败的提示独立于列表加载错误：还原失败时仍保留回收站列表可见，
// 而不是用错误文案整段替换掉列表（否则"点一个，全部条目都消失"）。
const restoreError = ref('')

// ---- 扫描预览状态 ----
const scanning = ref(false)
const scanned = ref(false)
const scanError = ref('')
const scanResult = ref<ScanResult>({ dirs: [], links: [], users: [], retentionHours: 24 })
const selectedDirs = ref<string[]>([])
const selectedLinks = ref<string[]>([])
const selectedUsers = ref<string[]>([])

// ---- 保留期设置 ----
const retentionHours = ref(24)
const retentionInput = ref(24)
const savingRetention = ref(false)
const retentionMsg = ref('')
const retentionError = ref(false)

const selectedCount = computed(
  () => selectedDirs.value.length + selectedLinks.value.length + selectedUsers.value.length
)
const scanEmpty = computed(
  () => !scanResult.value.dirs.length && !scanResult.value.links.length && !scanResult.value.users.length
)
const allSelected = computed(() => !scanEmpty.value && selectedCount.value === totalCount.value)
const totalCount = computed(
  () => scanResult.value.dirs.length + scanResult.value.links.length + scanResult.value.users.length
)

function riskLabel(risk: string): string {
  return risk === 'high' ? '高风险' : risk === 'medium' ? '中风险' : '低风险'
}

async function scan(): Promise<void> {
  if (scanning.value) return
  scanning.value = true
  scanError.value = ''
  try {
    const data = await api.get<ScanResult>('/api/dirs/clean-uninstalled-scan')
    scanResult.value = {
      dirs: data.dirs || [],
      links: data.links || [],
      users: data.users || [],
      retentionHours: data.retentionHours || retentionHours.value,
      errors: data.errors
    }
    if (data.retentionHours) {
      retentionHours.value = data.retentionHours
      retentionInput.value = data.retentionHours
    }
    // 默认全选，用户可取消不想清理的项目
    selectedDirs.value = scanResult.value.dirs.map(d => d.path)
    selectedLinks.value = scanResult.value.links.map(l => l.path)
    selectedUsers.value = scanResult.value.users.map(u => u.path)
    scanned.value = true
    if (data.errors && data.errors.length > 0) {
      scanError.value = '部分目录扫描失败：' + data.errors.join('；')
    }
  } catch (e) {
    scanError.value = '扫描失败，无法获取已安装应用列表时不会执行任何清理'
  } finally {
    scanning.value = false
  }
}

function toggleSelectAll(): void {
  if (allSelected.value) {
    selectedDirs.value = []
    selectedLinks.value = []
    selectedUsers.value = []
  } else {
    selectedDirs.value = scanResult.value.dirs.map(d => d.path)
    selectedLinks.value = scanResult.value.links.map(l => l.path)
    selectedUsers.value = scanResult.value.users.map(u => u.path)
  }
}

function cleanSelected(): void {
  if (running.value || selectedCount.value === 0) return
  running.value = true
  emit('cleanTrash', {
    dirs: selectedDirs.value,
    links: selectedLinks.value,
    users: selectedUsers.value,
    retentionHours: retentionHours.value
  })
}

async function loadRetention(): Promise<void> {
  try {
    const data = await api.get<{ retentionHours: number }>('/api/dirs/recycle-settings')
    if (data.retentionHours) {
      retentionHours.value = data.retentionHours
      retentionInput.value = data.retentionHours
    }
  } catch (e) {
    // 读取失败保留默认值，不阻塞其他功能
  }
}

async function saveRetention(): Promise<void> {
  const hours = Math.floor(Number(retentionInput.value))
  if (!Number.isFinite(hours) || hours < 1 || hours > 8760) {
    retentionMsg.value = '保留时长需在 1 到 8760 小时之间'
    retentionError.value = true
    return
  }
  savingRetention.value = true
  retentionMsg.value = ''
  retentionError.value = false
  try {
    const data = await api.post<{ retentionHours: number }>('/api/dirs/recycle-settings', {
      retentionHours: hours
    })
    retentionHours.value = data.retentionHours || hours
    retentionInput.value = retentionHours.value
    retentionMsg.value = `已保存：回收站保留 ${retentionHours.value} 小时`
  } catch (e) {
    retentionMsg.value = '保存失败'
    retentionError.value = true
  } finally {
    savingRetention.value = false
  }
}

async function loadRecycle(): Promise<void> {
  recycleLoaded.value = false
  recycleError.value = ''
  restoreError.value = ''
  try {
    const data = await api.get<{ items: RecycleItem[]; retentionHours?: number }>('/api/dirs/recycle-list')
    recycleItems.value = data.items || []
    if (data.retentionHours) {
      retentionHours.value = data.retentionHours
      retentionInput.value = data.retentionHours
    }
  } catch (e) {
    recycleError.value = '无法读取回收站内容'
  } finally {
    recycleLoaded.value = true
  }
}

async function restoreItem(item: RecycleItem): Promise<void> {
  if (restoring.value) return
  restoring.value = true
  restoreError.value = ''
  try {
    const data = await api.post<{ restored: number; errors: string[]; message: string }>('/api/dirs/recycle-restore', {
      root: item.root,
      rels: [item.relPath]
    })
    if (data.errors && data.errors.length > 0) {
      restoreError.value = data.errors.join('；')
    } else {
      restoreError.value = ''
      await loadRecycle()
    }
  } catch (e) {
    restoreError.value = '还原失败'
  } finally {
    restoring.value = false
  }
}

function run(kind: 'empty' | 'trash'): void {
  if (running.value) return
  if (kind === 'empty') {
    running.value = true
    emit('cleanEmpty')
  } else {
    scan()
  }
}

function formatTime(t: string): string {
  if (!t) return ''
  const d = new Date(t)
  return isNaN(d.getTime()) ? '' : d.toLocaleString()
}

function expiryTime(movedAt: string): string {
  if (!movedAt || !retentionHours.value) return ''
  const d = new Date(movedAt)
  if (isNaN(d.getTime())) return ''
  return new Date(d.getTime() + retentionHours.value * 3600 * 1000).toLocaleString()
}

onMounted(() => {
  loadRecycle()
  loadRetention()
})
</script>

<style scoped>
.modal-content {
  padding: var(--spacing-2xl);
  max-width: 560px;
  width: 90%;
  position: relative;
  overflow: hidden;
}

.modal-content::before {
  content: '';
  position: absolute;
  inset: 0;
  background: var(--glass-shine);
  pointer-events: none;
  border-radius: inherit;
  z-index: 1;
}

.modal-header {
  position: relative;
  z-index: 3;
  font-size: var(--font-size-2xl);
  font-weight: var(--font-weight-semibold);
  margin-bottom: 0;
  padding-bottom: var(--spacing-md);
  color: var(--text-color-1);
  letter-spacing: var(--letter-spacing-tight);
  border-bottom: 1px solid var(--divider-color);
}

.modal-body {
  position: relative;
  z-index: 3;
  margin-top: var(--spacing-lg);
}

.modal-desc {
  font-size: var(--font-size-sm);
  color: var(--text-color-2);
  line-height: 1.6;
  margin-bottom: var(--spacing-lg);
  padding: var(--spacing-sm) var(--spacing-md);
  background: var(--glass-bg);
  backdrop-filter: blur(var(--glass-blur));
  -webkit-backdrop-filter: blur(var(--glass-blur));
  border: 1px solid var(--glass-border);
  border-radius: var(--radius-sm);
}

.option-list {
  display: flex;
  flex-direction: column;
  gap: var(--spacing-md);
}

.option-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--spacing-md);
  padding: var(--spacing-md);
  border: 1px solid var(--glass-border);
  border-radius: var(--radius-sm);
  background: var(--glass-bg);
  backdrop-filter: blur(var(--glass-blur));
  -webkit-backdrop-filter: blur(var(--glass-blur));
}

.option-info {
  flex: 1;
}

.option-title {
  font-size: var(--font-size-md);
  font-weight: var(--font-weight-semibold);
  color: var(--text-color-1);
  margin-bottom: var(--spacing-xs);
}

.option-desc {
  font-size: var(--font-size-sm);
  color: var(--text-color-3);
  line-height: 1.5;
}

.option-item button {
  flex-shrink: 0;
  padding: var(--spacing-sm) var(--spacing-lg);
  border: none;
  border-radius: var(--radius-sm);
  cursor: pointer;
  font-size: var(--font-size-md);
  font-weight: var(--font-weight-medium);
  transition: all var(--transition-fast);
  white-space: nowrap;
}

.option-item button.secondary {
  background: var(--glass-bg-strong);
  backdrop-filter: blur(var(--glass-blur));
  -webkit-backdrop-filter: blur(var(--glass-blur));
  border: 1px solid var(--glass-border);
  color: var(--text-color-1);
}

.option-item button.danger {
  background: var(--error-color);
  color: var(--text-color-on-primary);
}

.option-item button:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.scan-error {
  margin-top: var(--spacing-md);
  font-size: var(--font-size-sm);
  color: var(--error-color);
  padding: var(--spacing-sm) var(--spacing-md);
  border: 1px solid var(--error-color);
  border-radius: var(--radius-sm);
  background: var(--glass-bg);
  word-break: break-all;
}

.scan-section {
  margin-top: var(--spacing-md);
  padding: var(--spacing-md);
  border: 1px solid var(--glass-border);
  border-radius: var(--radius-sm);
  background: var(--glass-bg);
  backdrop-filter: blur(var(--glass-blur));
  -webkit-backdrop-filter: blur(var(--glass-blur));
  max-height: 320px;
  overflow-y: auto;
}

.scan-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  font-size: var(--font-size-sm);
  font-weight: var(--font-weight-semibold);
  color: var(--text-color-2);
  margin-bottom: var(--spacing-sm);
}

.scan-header-actions {
  display: flex;
  gap: var(--spacing-xs);
}

.scan-group {
  margin-top: var(--spacing-sm);
}

.scan-group-title {
  font-size: var(--font-size-sm);
  font-weight: var(--font-weight-semibold);
  color: var(--text-color-1);
  margin-bottom: var(--spacing-xs);
}

.scan-group-hint {
  font-weight: var(--font-weight-regular);
  font-size: var(--font-size-xs);
  color: var(--text-color-3);
  margin-left: var(--spacing-xs);
}

.scan-item {
  display: flex;
  align-items: flex-start;
  gap: var(--spacing-sm);
  padding: var(--spacing-xs) var(--spacing-sm);
  border-radius: var(--radius-sm);
  cursor: pointer;
  transition: background var(--transition-fast);
}

.scan-item:hover {
  background: var(--glass-bg-strong);
}

.scan-item input[type='checkbox'] {
  margin-top: 3px;
  accent-color: var(--primary-color);
  cursor: pointer;
}

.scan-item-main {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

.scan-item-name {
  font-size: var(--font-size-sm);
  font-weight: var(--font-weight-medium);
  color: var(--text-color-1);
  display: flex;
  align-items: center;
  gap: var(--spacing-xs);
  flex-wrap: wrap;
}

.scan-item-root {
  font-size: var(--font-size-xs);
  color: var(--text-color-3);
}

.scan-item-path {
  font-size: var(--font-size-xs);
  color: var(--text-color-3);
  word-break: break-all;
}

.scan-item-size {
  flex-shrink: 0;
  font-size: var(--font-size-xs);
  color: var(--text-color-2);
  margin-top: 2px;
}

.risk-badge {
  font-size: var(--font-size-xs);
  padding: 0 var(--spacing-xs);
  border-radius: var(--radius-sm);
  line-height: 1.6;
}

.risk-high {
  color: var(--error-color);
  border: 1px solid var(--error-color);
}

.risk-medium {
  color: var(--warning-color, #d97706);
  border: 1px solid var(--warning-color, #d97706);
}

.risk-low {
  color: var(--text-color-3);
  border: 1px solid var(--glass-border);
}

.scan-actions {
  margin-top: var(--spacing-md);
  display: flex;
  justify-content: flex-end;
}

.scan-actions button {
  padding: var(--spacing-sm) var(--spacing-lg);
  border: none;
  border-radius: var(--radius-sm);
  cursor: pointer;
  font-size: var(--font-size-md);
  font-weight: var(--font-weight-medium);
  transition: all var(--transition-fast);
}

.scan-actions button.danger {
  background: var(--error-color);
  color: var(--text-color-on-primary);
}

.scan-actions button:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.refresh-btn {
  padding: 2px var(--spacing-sm);
  border: 1px solid var(--glass-border);
  border-radius: var(--radius-sm);
  background: transparent;
  color: var(--text-color-2);
  font-size: var(--font-size-sm);
  cursor: pointer;
}

.refresh-btn:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.recycle-section {
  margin-top: var(--spacing-lg);
  padding: var(--spacing-md);
  border: 1px solid var(--glass-border);
  border-radius: var(--radius-sm);
  background: var(--glass-bg);
  backdrop-filter: blur(var(--glass-blur));
  -webkit-backdrop-filter: blur(var(--glass-blur));
  max-height: 280px;
  overflow-y: auto;
}

.recycle-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  font-size: var(--font-size-sm);
  font-weight: var(--font-weight-semibold);
  color: var(--text-color-2);
  margin-bottom: var(--spacing-sm);
}

.retention-row {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
  margin-bottom: var(--spacing-sm);
}

.retention-label {
  font-size: var(--font-size-sm);
  color: var(--text-color-2);
}

.retention-input {
  width: 72px;
  padding: 2px var(--spacing-xs);
  border: 1px solid var(--glass-border);
  border-radius: var(--radius-sm);
  background: var(--glass-bg-strong);
  color: var(--text-color-1);
  font-size: var(--font-size-sm);
}

.retention-msg {
  font-size: var(--font-size-xs);
  color: var(--text-color-3);
  margin-bottom: var(--spacing-xs);
}

.retention-msg.error {
  color: var(--error-color);
}

.recycle-empty {
  font-size: var(--font-size-sm);
  color: var(--text-color-3);
  padding: var(--spacing-sm) 0;
}

.recycle-error {
  font-size: var(--font-size-sm);
  color: var(--error-color);
  padding: var(--spacing-sm);
  margin-bottom: var(--spacing-sm);
  background: var(--glass-bg);
  border: 1px solid var(--error-color);
  border-radius: var(--radius-sm);
  word-break: break-all;
}

.recycle-list {
  display: flex;
  flex-direction: column;
  gap: var(--spacing-sm);
}

.recycle-item {
  padding: var(--spacing-sm);
  border: 1px solid var(--glass-border);
  border-radius: var(--radius-sm);
  background: var(--glass-bg-strong);
}

.recycle-item-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--spacing-sm);
}

.recycle-item-name {
  font-size: var(--font-size-md);
  font-weight: var(--font-weight-medium);
  color: var(--text-color-1);
  word-break: break-all;
}

.restore-btn {
  flex-shrink: 0;
  padding: 2px var(--spacing-md);
  border: 1px solid var(--glass-border);
  border-radius: var(--radius-sm);
  background: var(--glass-bg-strong);
  color: var(--text-color-1);
  font-size: var(--font-size-sm);
  cursor: pointer;
  transition: all var(--transition-fast);
}

.restore-btn:hover {
  border-color: var(--primary-color);
  color: var(--primary-color);
}

.restore-btn:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.recycle-item-meta {
  display: flex;
  flex-direction: column;
  gap: 2px;
  margin-top: var(--spacing-xs);
}

.recycle-original {
  font-size: var(--font-size-sm);
  color: var(--text-color-2);
  word-break: break-all;
}

.recycle-original.muted {
  color: var(--text-color-3);
}

.recycle-misc {
  font-size: var(--font-size-xs);
  color: var(--text-color-3);
}

.recycle-expiry {
  font-size: var(--font-size-xs);
  color: var(--warning-color, #d97706);
}

.modal-footer {
  margin-top: var(--spacing-xl);
  display: flex;
  gap: var(--spacing-sm);
  justify-content: flex-end;
}

.modal-footer button {
  padding: var(--spacing-sm) var(--spacing-lg);
  border: none;
  border-radius: var(--radius-sm);
  cursor: pointer;
  font-size: var(--font-size-md);
  font-weight: var(--font-weight-medium);
  transition: all var(--transition-fast);
  position: relative;
  z-index: 3;
}

.modal-footer button.secondary {
  background: var(--glass-bg-strong);
  backdrop-filter: blur(var(--glass-blur));
  -webkit-backdrop-filter: blur(var(--glass-blur));
  border: 1px solid var(--glass-border);
  color: var(--text-color-1);
}

/* ===== Mobile ===== */
@media (max-width: 480px) {
  .modal-content {
    padding: var(--spacing-xl) var(--spacing-md);
    max-width: 100%;
    width: 95%;
    border-radius: var(--radius-lg) var(--radius-lg) var(--radius-md) var(--radius-md);
  }

  .modal-header {
    font-size: var(--font-size-xl);
    padding-bottom: var(--spacing-sm);
    margin-bottom: var(--spacing-md);
  }

  .option-item {
    flex-direction: column;
    align-items: stretch;
  }

  .option-item button {
    width: 100%;
  }
}
</style>
