<template>
  <div class="hm-overlay-base" @click.self="$emit('close')">
    <div class="modal-content hm-modal-base">
      <div class="modal-header">备份管理</div>
      <div class="modal-body">
        <!-- 列表视图 -->
        <template v-if="view === 'list'">
          <div class="list-header">
            <span>已有备份（{{ backups.length }}）</span>
            <div class="list-header-actions">
              <button class="ghost-btn hm-ripple-btn" :disabled="creating || loadingList" @click="createBackup">{{ creating ? '备份中...' : '立即备份' }}</button>
              <button class="ghost-btn hm-ripple-btn" :disabled="loadingList" @click="loadList">刷新</button>
            </div>
          </div>

          <div v-if="listError" class="feedback error">{{ listError }}</div>

          <div v-if="loadingList" class="empty-hint">加载中...</div>
          <div v-else-if="!listError && backups.length === 0" class="empty-hint">
            暂无备份，点击"立即备份"创建
          </div>

          <div v-else class="backup-list">
            <div v-for="item in backups" :key="item.path" class="backup-item">
              <div class="backup-info">
                <div class="backup-name">{{ item.name }}</div>
                <div class="backup-meta">{{ formatTime(item.created) }} · {{ item.size }}</div>
              </div>
              <div class="backup-actions">
                <button class="secondary hm-ripple-btn" :disabled="busy" @click="openPreview(item)">恢复</button>
                <button class="danger-btn hm-ripple-btn" :disabled="busy" @click="removeBackup(item)">删除</button>
              </div>
            </div>
          </div>
        </template>

        <!-- 预览视图 -->
        <template v-else-if="view === 'preview'">
          <div class="list-header">
            <span>恢复预览</span>
            <button class="ghost-btn hm-ripple-btn" :disabled="busy" @click="backToList">返回列表</button>
          </div>
          <div class="backup-name">{{ previewing?.name }}</div>

          <div v-if="previewError" class="feedback error">{{ previewError }}</div>

          <template v-else-if="preview">
            <div class="preview-summary">
              共 {{ preview.totalFiles }} 个文件（{{ preview.totalSizeFormatted }}）
              <span v-if="preview.deniedFiles > 0" class="denied-hint">
                ，{{ preview.deniedFiles }} 个文件不在允许的日志目录内，恢复时将跳过
              </span>
              <span v-if="preview.truncated">，仅显示前 {{ preview.entries.length }} 个</span>
            </div>

            <div class="conflict-row">
              <span class="conflict-label">目标已存在时：</span>
              <label class="radio-item">
                <input type="radio" value="skip" v-model="conflictPolicy" />
                跳过已存在文件
              </label>
              <label class="radio-item">
                <input type="radio" value="overwrite" v-model="conflictPolicy" />
                覆盖已存在文件
              </label>
            </div>

            <div class="preview-list">
              <div
                v-for="entry in preview.entries"
                :key="entry.name"
                class="preview-item"
                :class="{ denied: entry.denied }"
              >
                <span class="preview-path">{{ entry.targetPath }}</span>
                <span class="preview-meta">
                  <span v-if="entry.denied" class="badge badge-denied">不允许</span>
                  <span v-else-if="entry.exists" class="badge badge-exists">已存在</span>
                  {{ entry.sizeFormatted }}
                </span>
              </div>
            </div>

            <div class="restore-actions">
              <button
                class="primary-btn hm-ripple-btn"
                :disabled="busy || preview.totalFiles === 0"
                @click="doRestore"
              >
                {{ busy ? '恢复中...' : `恢复 ${restorableCount} 个文件` }}
              </button>
            </div>
          </template>
        </template>

        <!-- 结果视图 -->
        <template v-else>
          <div class="list-header">
            <span>恢复结果</span>
            <button class="ghost-btn hm-ripple-btn" @click="backToList">返回列表</button>
          </div>

          <div class="result-summary">
            <div class="result-stat success">成功 {{ result?.restored ?? 0 }}</div>
            <div class="result-stat">跳过 {{ result?.skipped ?? 0 }}</div>
            <div class="result-stat" :class="{ failed: (result?.failed ?? 0) > 0 }">失败 {{ result?.failed ?? 0 }}</div>
          </div>

          <div v-if="result?.errors?.length" class="feedback error">
            {{ result.errors.slice(0, 5).join('；') }}
          </div>

          <div class="preview-list">
            <div
              v-for="item in result?.details || []"
              :key="item.path"
              class="preview-item"
              :class="{ denied: item.status === 'failed' }"
            >
              <span class="preview-path">{{ item.path }}</span>
              <span class="preview-meta">
                <span class="badge" :class="statusBadgeClass(item.status)">{{ statusLabel(item.status) }}</span>
                <span v-if="item.message" class="item-message">{{ item.message }}</span>
              </span>
            </div>
          </div>
        </template>

        <ConfirmDialog ref="confirmDialog" />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useBackup } from '../composables/useBackup'
import type { BackupListItem, BackupPreview, RestoreItemResult, RestoreResult } from '../types'
import ConfirmDialog from './ConfirmDialog.vue'

defineEmits<{ close: [] }>()

const { backupLogs, listBackups, previewBackup, restoreBackup, deleteBackup } = useBackup()

const view = ref<'list' | 'preview' | 'result'>('list')
const backups = ref<BackupListItem[]>([])
const loadingList = ref(false)
const listError = ref('')
const busy = ref(false)
const creating = ref(false)
const previewing = ref<BackupListItem | null>(null)
const preview = ref<BackupPreview | null>(null)
const previewError = ref('')
const conflictPolicy = ref<'skip' | 'overwrite'>('skip')
const result = ref<RestoreResult | null>(null)

const confirmDialog = ref<ConfirmDialogExpose | null>(null)
interface ConfirmDialogExpose {
  show(options?: { title?: string; message?: string; type?: string; confirmText?: string }): Promise<boolean>
}

const restorableCount = computed(() => preview.value?.totalFiles ?? 0)

async function loadList(): Promise<void> {
  loadingList.value = true
  listError.value = ''
  try {
    backups.value = await listBackups()
  } catch {
    listError.value = '无法读取备份列表'
  } finally {
    loadingList.value = false
  }
}

async function createBackup(): Promise<void> {
  creating.value = true
  try {
    await backupLogs()
    await loadList()
  } finally {
    creating.value = false
  }
}

async function openPreview(item: BackupListItem): Promise<void> {
  busy.value = true
  previewing.value = item
  preview.value = null
  previewError.value = ''
  conflictPolicy.value = 'skip'
  view.value = 'preview'
  try {
    preview.value = await previewBackup(item.path)
  } catch (e) {
    previewError.value = (e as Error).message || '读取备份内容失败'
  } finally {
    busy.value = false
  }
}

function backToList(): void {
  view.value = 'list'
  previewing.value = null
  preview.value = null
  result.value = null
  loadList()
}

async function doRestore(): Promise<void> {
  if (!previewing.value || !preview.value) return
  const overwrite = conflictPolicy.value === 'overwrite'
  const ok = await confirmDialog.value?.show({
    title: '确认恢复',
    message: overwrite
      ? `将把备份中的 ${restorableCount.value} 个文件恢复到原位置，已存在的文件会被覆盖。确定继续吗？`
      : `将把备份中的 ${restorableCount.value} 个文件恢复到原位置，已存在的文件将被跳过。确定继续吗？`,
    type: 'danger',
    confirmText: '恢复'
  })
  if (!ok) return

  busy.value = true
  try {
    result.value = await restoreBackup(previewing.value.path, overwrite)
    view.value = 'result'
  } catch (e) {
    previewError.value = (e as Error).message || '恢复失败'
    view.value = 'preview'
  } finally {
    busy.value = false
  }
}

async function removeBackup(item: BackupListItem): Promise<void> {
  const ok = await confirmDialog.value?.show({
    title: '删除备份',
    message: `确定删除备份「${item.name}」吗？删除后无法恢复。`,
    type: 'danger',
    confirmText: '删除'
  })
  if (!ok) return
  busy.value = true
  try {
    await deleteBackup(item.path)
    await loadList()
  } catch (e) {
    listError.value = (e as Error).message || '删除失败'
  } finally {
    busy.value = false
  }
}

function statusLabel(status: RestoreItemResult['status']): string {
  switch (status) {
    case 'restored': return '已恢复'
    case 'skipped': return '已跳过'
    default: return '失败'
  }
}

function statusBadgeClass(status: RestoreItemResult['status']): string {
  switch (status) {
    case 'restored': return 'badge-restored'
    case 'skipped': return 'badge-skipped'
    default: return 'badge-denied'
  }
}

function formatTime(t: string): string {
  if (!t) return ''
  const d = new Date(t)
  return isNaN(d.getTime()) ? '' : d.toLocaleString()
}

onMounted(loadList)
</script>

<style scoped>
.modal-content {
  padding: var(--spacing-2xl);
  max-width: 640px;
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
}

.modal-body {
  position: relative;
  z-index: 1;
}

.list-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: var(--spacing-md);
  font-weight: var(--font-weight-semibold);
  color: var(--text-color-1);
}

.list-header-actions {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
}

.ghost-btn {
  padding: var(--spacing-xs) var(--spacing-md);
  font-size: var(--font-size-sm);
  background: var(--glass-bg);
  border: 1px solid var(--glass-border);
  border-radius: var(--radius-sm);
  color: var(--text-color-1);
  cursor: pointer;
  min-height: 36px;
}

.backup-list {
  display: flex;
  flex-direction: column;
  gap: var(--spacing-sm);
  max-height: 46vh;
  overflow-y: auto;
}

.backup-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--spacing-sm);
  padding: var(--spacing-sm) var(--spacing-md);
  background: var(--glass-bg);
  border: 1px solid var(--glass-border);
  border-radius: var(--radius-sm);
}

.backup-name {
  font-weight: var(--font-weight-medium);
  color: var(--text-color-1);
  word-break: break-all;
}

.backup-meta {
  font-size: var(--font-size-sm);
  color: var(--text-color-2);
  margin-top: 2px;
}

.backup-actions {
  display: flex;
  gap: var(--spacing-xs);
  flex-shrink: 0;
}

.backup-actions button {
  min-height: 44px;
  min-width: 64px;
  border: none;
  border-radius: var(--radius-sm);
  cursor: pointer;
  font-size: var(--font-size-sm);
}

.danger-btn {
  background: linear-gradient(135deg, var(--card-color-4) 0%, var(--card-color-3) 100%);
  color: var(--text-color);
}

.primary-btn {
  background: linear-gradient(135deg, var(--card-color-1) 0%, var(--card-color-2) 100%);
  color: var(--text-color);
  border: none;
  border-radius: var(--radius-sm);
  padding: var(--spacing-md) var(--spacing-lg);
  min-height: 44px;
  cursor: pointer;
  font-size: var(--font-size-md);
  font-weight: var(--font-weight-semibold);
}

.primary-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.preview-summary {
  font-size: var(--font-size-sm);
  color: var(--text-color-2);
  margin-bottom: var(--spacing-sm);
}

.denied-hint {
  color: var(--warning-color);
}

.conflict-row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: var(--spacing-md);
  padding: var(--spacing-sm) var(--spacing-md);
  background: var(--glass-bg);
  border: 1px solid var(--glass-border);
  border-radius: var(--radius-sm);
  margin-bottom: var(--spacing-sm);
}

.conflict-label {
  font-size: var(--font-size-sm);
  color: var(--text-color-1);
}

.radio-item {
  display: flex;
  align-items: center;
  gap: var(--spacing-xs);
  font-size: var(--font-size-sm);
  color: var(--text-color-1);
  min-height: 44px;
  cursor: pointer;
}

.preview-list {
  display: flex;
  flex-direction: column;
  gap: 2px;
  max-height: 40vh;
  overflow-y: auto;
  margin-top: var(--spacing-sm);
}

.preview-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--spacing-sm);
  padding: var(--spacing-xs) var(--spacing-sm);
  border-radius: var(--radius-xs);
  font-size: var(--font-size-sm);
}

.preview-item.denied {
  opacity: 0.6;
}

.preview-path {
  color: var(--text-color-1);
  word-break: break-all;
  font-family: monospace;
}

.preview-meta {
  display: flex;
  align-items: center;
  gap: var(--spacing-xs);
  color: var(--text-color-2);
  flex-shrink: 0;
}

.item-message {
  max-width: 180px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.badge {
  padding: 1px 6px;
  border-radius: var(--radius-xl);
  font-size: var(--font-size-xs);
}

.badge-exists {
  background: var(--warning-bg);
  color: var(--warning-color);
}

.badge-denied {
  background: var(--error-bg);
  color: var(--error-color);
}

.badge-restored {
  background: var(--success-bg);
  color: var(--success-color);
}

.badge-skipped {
  background: var(--info-bg);
  color: var(--info-color);
}

.restore-actions {
  margin-top: var(--spacing-md);
  display: flex;
  justify-content: flex-end;
}

.result-summary {
  display: flex;
  gap: var(--spacing-sm);
  margin-bottom: var(--spacing-md);
}

.result-stat {
  flex: 1;
  text-align: center;
  padding: var(--spacing-sm);
  background: var(--glass-bg);
  border: 1px solid var(--glass-border);
  border-radius: var(--radius-sm);
  color: var(--text-color-1);
}

.result-stat.success {
  background: var(--success-bg);
  color: var(--success-color);
}

.result-stat.failed {
  background: var(--error-bg);
  color: var(--error-color);
}

.feedback {
  padding: var(--spacing-sm) var(--spacing-md);
  border-radius: var(--radius-sm);
  font-size: var(--font-size-sm);
  margin-bottom: var(--spacing-sm);
  word-break: break-all;
}

.feedback.error {
  background: var(--error-bg);
  color: var(--error-color);
}

.empty-hint {
  text-align: center;
  color: var(--text-color-2);
  padding: var(--spacing-lg) 0;
}

@media (max-width: 480px) {
  .modal-content {
    width: 96%;
    padding: var(--spacing-lg);
  }

  .backup-item {
    flex-direction: column;
    align-items: stretch;
  }

  .backup-actions {
    justify-content: flex-end;
  }

  .result-summary {
    flex-direction: column;
  }
}
</style>
