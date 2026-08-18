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
              <div class="option-desc">直接删除已卸载应用留下的空文件夹，立即释放空间。</div>
            </div>
            <button class="secondary hm-ripple-btn" :disabled="running" @click="run('empty')">
              执行
            </button>
          </div>

          <div class="option-item danger-option">
            <div class="option-info">
              <div class="option-title">清理非空残留（移入回收站）</div>
              <div class="option-desc">将已卸载应用的非空残留目录移入回收站（可恢复），24 小时后自动清空。</div>
            </div>
            <button class="danger hm-ripple-btn" :disabled="running" @click="run('trash')">
              执行
            </button>
          </div>
        </div>

        <div class="recycle-section">
          <div class="recycle-header">
            <span>回收站内容（可手动恢复）</span>
            <button class="refresh-btn" @click="loadRecycle">刷新</button>
          </div>

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
                <span class="recycle-misc">{{ item.sizeFormatted }} · {{ formatTime(item.movedAt) }}</span>
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
import { ref, onMounted } from 'vue'
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

const emit = defineEmits<{
  close: []
  cleanEmpty: []
  cleanTrash: []
}>()

const running = ref(false)
const restoring = ref(false)
const recycleItems = ref<RecycleItem[]>([])
const recycleLoaded = ref(false)
const recycleError = ref('')
// 还原失败的提示独立于列表加载错误：还原失败时仍保留回收站列表可见，
// 而不是用错误文案整段替换掉列表（否则"点一个，全部条目都消失"）。
const restoreError = ref('')

async function loadRecycle(): Promise<void> {
  recycleLoaded.value = false
  recycleError.value = ''
  restoreError.value = ''
  try {
    const data = await api.get<{ items: RecycleItem[] }>('/api/dirs/recycle-list')
    recycleItems.value = data.items || []
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
  running.value = true
  // 由父组件接管真正的清理逻辑（含二次确认与状态提示）
  if (kind === 'empty') {
    emit('cleanEmpty')
  } else {
    emit('cleanTrash')
  }
}

function formatTime(t: string): string {
  if (!t) return ''
  const d = new Date(t)
  return isNaN(d.getTime()) ? '' : d.toLocaleString()
}

onMounted(loadRecycle)
</script>

<style scoped>
.modal-content {
  padding: var(--spacing-2xl);
  max-width: 480px;
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

.recycle-section {
  margin-top: var(--spacing-lg);
  padding: var(--spacing-md);
  border: 1px solid var(--glass-border);
  border-radius: var(--radius-sm);
  background: var(--glass-bg);
  backdrop-filter: blur(var(--glass-blur));
  -webkit-backdrop-filter: blur(var(--glass-blur));
  max-height: 240px;
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

.refresh-btn {
  padding: 2px var(--spacing-sm);
  border: 1px solid var(--glass-border);
  border-radius: var(--radius-sm);
  background: transparent;
  color: var(--text-color-2);
  font-size: var(--font-size-sm);
  cursor: pointer;
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

.recycle-hint {
  margin-top: var(--spacing-xs);
  font-size: var(--font-size-xs);
  color: var(--text-color-3);
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
