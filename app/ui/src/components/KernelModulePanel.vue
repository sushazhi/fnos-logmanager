<template>
  <div class="hm-overlay-base" @click.self="$emit('close')">
    <div class="hm-modal-base kernel-panel">
      <div class="panel-header">
        <h3>Linux 内核版本</h3>
        <button class="close-btn" @click="$emit('close')">×</button>
      </div>

      <div class="panel-body">
        <div class="error-banner" v-if="errorMessage">
          <span class="error-text">{{ errorMessage }}</span>
          <button class="error-dismiss" @click="errorMessage = null">×</button>
        </div>

        <div class="success-banner" v-if="cleanResult">
          <span class="success-text">{{ cleanResult }}</span>
          <button class="error-dismiss" @click="cleanResult = null">×</button>
        </div>

        <div class="loading-state" v-if="loading && versions.length === 0">
          <div class="kernel-shimmer">
            <div class="shimmer-stat-row">
              <div class="hm-shimmer shimmer-stat-card"></div>
              <div class="hm-shimmer shimmer-stat-card"></div>
              <div class="hm-shimmer shimmer-stat-card"></div>
            </div>
            <div class="hm-shimmer shimmer-current"></div>
            <div class="shimmer-list-rows">
              <div class="hm-shimmer shimmer-list-item"></div>
              <div class="hm-shimmer shimmer-list-item"></div>
              <div class="hm-shimmer shimmer-list-item"></div>
            </div>
          </div>
          <span class="loading-tip">加载内核版本信息...</span>
        </div>

        <template v-if="!loading || versions.length > 0">
          <div class="stats-row">
            <div class="stat-card">
              <span class="stat-value">{{ stats.total }}</span>
              <span class="stat-label">已安装</span>
            </div>
            <div class="stat-card success">
              <span class="stat-value">{{ stats.current }}</span>
              <span class="stat-label">当前使用</span>
            </div>
            <div class="stat-card" :class="{ warning: stats.unusedCount > 0 }">
              <span class="stat-value">{{ stats.unusedCount }}</span>
              <span class="stat-label">未使用</span>
            </div>
            <div class="stat-card" :class="{ warning: stats.unusedCount > 0 }">
              <span class="stat-value">{{ stats.unusedSizeFormatted }}</span>
              <span class="stat-label">未使用占用</span>
            </div>
          </div>

          <div class="current-info">
            <span class="current-label">当前运行内核:</span>
            <code class="current-version">{{ currentKernel }}</code>
          </div>

          <div class="action-section">
            <button class="action-btn" @click="loadVersions" :disabled="loading">
              {{ loading ? '刷新中...' : '刷新' }}
            </button>
            <button
              class="action-btn cleanup-btn"
              @click="cleanupUnused"
              :disabled="loading || stats.unusedCount === 0"
              :title="'可释放 ' + stats.unusedSizeFormatted"
            >
              清理旧内核 ({{ stats.unusedSizeFormatted }})
            </button>
          </div>

          <div class="version-list-header">
            <span class="col-version">内核版本</span>
            <span class="col-status">状态</span>
            <span class="col-boot">引导文件</span>
            <span class="col-modules">模块</span>
            <span class="col-total">总计</span>
            <span class="col-action">操作</span>
          </div>

          <div class="version-list">
            <div
              v-for="v in versions"
              :key="v.version"
              class="version-item"
              :class="{ current: v.isCurrent, unused: !v.isCurrent }"
            >
              <span class="col-version">
                <span class="version-name">{{ v.version }}</span>
              </span>
              <span class="col-status">
                <span class="version-status" :class="v.isCurrent ? 'running' : 'old'">
                  {{ v.isCurrent ? '当前使用' : '旧版本' }}
                </span>
              </span>
              <span class="col-boot">{{ v.bootSizeFormatted }}</span>
              <span class="col-modules">{{ v.modulesSizeFormatted }}</span>
              <span class="col-total">{{ v.totalSizeFormatted }}</span>
              <span class="col-action">
                <button
                  v-if="!v.isCurrent"
                  class="remove-btn"
                  @click="removeVersion(v.version)"
                  :disabled="removing === v.version"
                  :title="'删除内核 ' + v.version"
                >
                  {{ removing === v.version ? '删除中...' : '删除' }}
                </button>
                <span v-else class="current-badge">运行中</span>
              </span>
            </div>

            <div class="no-versions" v-if="versions.length === 0 && !loading">
              未检测到已安装的内核版本
            </div>
          </div>
        </template>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { kernelApi, type KernelVersion } from '../services/api'

const emit = defineEmits<{
  close: []
}>()

const loading = ref(false)
const removing = ref('')
const errorMessage = ref<string | null>(null)
const cleanResult = ref<string | null>(null)
const versions = ref<KernelVersion[]>([])
const currentKernel = ref('')
const stats = reactive({
  total: 0,
  current: '',
  unusedCount: 0,
  unusedSizeFormatted: '0 B',
})

async function loadVersions() {
  loading.value = true
  errorMessage.value = null
  try {
    const data = await kernelApi.getVersions()
    versions.value = data.versions || []
    currentKernel.value = data.current || ''
    stats.total = data.total || versions.value.length
    stats.current = data.current || ''
    stats.unusedCount = data.unusedCount ?? 0
    stats.unusedSizeFormatted = data.unusedSizeFormatted || '0 B'
    if (data.error) {
      errorMessage.value = data.error
    }
  } catch (e: any) {
    errorMessage.value = e?.message || '加载内核版本信息失败'
    versions.value = []
    currentKernel.value = ''
    stats.total = 0
    stats.current = ''
    stats.unusedCount = 0
    stats.unusedSizeFormatted = '0 B'
  } finally {
    loading.value = false
  }
}

async function cleanupUnused() {
  loading.value = true
  errorMessage.value = null
  cleanResult.value = null
  try {
    const data = await kernelApi.cleanupUnused()
    if (data.removed > 0) {
      cleanResult.value = `已删除 ${data.removed} 个旧内核，释放 ${data.freedSizeFormatted} 磁盘空间`
    }
    if (data.errors && data.errors.length > 0) {
      errorMessage.value = '部分内核删除失败: ' + data.errors.join('; ')
    }
    await loadVersions()
  } catch (e: any) {
    errorMessage.value = e?.message || '清理失败'
    await loadVersions()
  } finally {
    loading.value = false
  }
}

async function removeVersion(version: string) {
  removing.value = version
  errorMessage.value = null
  try {
    const data = await kernelApi.removeVersion(version)
    versions.value = data.versions || versions.value.filter(v => v.version !== version)
    stats.total = versions.value.length
    stats.unusedCount = versions.value.filter(v => !v.isCurrent).length
    stats.unusedSizeFormatted = versions.value
      .filter(v => !v.isCurrent)
      .reduce((sum, v) => sum + v.totalSize, 0)
      .toLocaleString() + ' B'
  } catch (e: any) {
    errorMessage.value = e?.message || `删除内核 ${version} 失败`
  } finally {
    removing.value = ''
  }
}

onMounted(() => {
  loadVersions()
})
</script>

<style scoped>
.kernel-panel {
  max-width: 960px;
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
  font-size: var(--font-size-xl);
  font-weight: 500;
  color: var(--text-color-1);
}

.close-btn {
  background: none;
  border: none;
  font-size: var(--font-size-5xl);
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

.error-banner,
.success-banner {
  display: flex;
  align-items: flex-start;
  gap: var(--spacing-sm);
  padding: var(--spacing-sm) var(--spacing-md);
  border-radius: var(--radius-xs);
  margin-bottom: var(--spacing-md);
}

.error-banner {
  background: var(--error-bg);
  border: 1px solid var(--error-color);
}

.success-banner {
  background: var(--success-bg);
  border: 1px solid var(--success-color);
}

.error-text {
  flex: 1;
  font-size: var(--font-size-base);
  color: var(--error-color);
  word-break: break-word;
}

.success-text {
  flex: 1;
  font-size: var(--font-size-base);
  color: var(--success-color);
  word-break: break-word;
}

.error-dismiss {
  background: none;
  border: none;
  font-size: var(--font-size-3xl);
  cursor: pointer;
  color: var(--text-color-3);
  padding: 0;
  line-height: 1;
}

.error-dismiss:hover {
  color: var(--text-color-1);
}

.loading-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: var(--spacing-sm);
  padding: var(--spacing-xl);
  color: var(--text-color-2);
}

.kernel-shimmer {
  width: 100%;
  display: flex;
  flex-direction: column;
  gap: var(--spacing-md);
}

.shimmer-stat-row {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: var(--spacing-sm);
}

.shimmer-stat-card {
  height: 72px;
  border-radius: var(--radius-sm);
}

.shimmer-current {
  height: 56px;
  border-radius: var(--radius-sm);
}

.shimmer-list-rows {
  display: flex;
  flex-direction: column;
  gap: var(--spacing-xs);
}

.shimmer-list-item {
  height: 48px;
  border-radius: var(--radius-xs);
}

.loading-tip {
  color: var(--text-color-3);
  font-size: var(--font-size-sm);
  animation: pulse-text 2s ease-in-out infinite;
}

@keyframes pulse-text {
  0%, 100% { opacity: 0.5; }
  50% { opacity: 1; }
}

.stats-row {
  display: flex;
  gap: var(--spacing-md);
  margin-bottom: var(--spacing-md);
}

.stat-card {
  flex: 1;
  text-align: center;
  padding: var(--spacing-md);
  background: var(--bg-color-2);
  border-radius: var(--radius-xs);
  border: 1px solid var(--border-color);
}

.stat-card.success {
  border-color: var(--success-color);
}

.stat-card.warning {
  border-color: var(--warning-color);
}

.stat-value {
  display: block;
  font-size: var(--font-size-3xl);
  font-weight: 600;
  color: var(--text-color-1);
}

.stat-label {
  display: block;
  font-size: var(--font-size-sm);
  color: var(--text-color-3);
  margin-top: calc(var(--spacing-xs) / 2);
}

.current-info {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
  padding: var(--spacing-md);
  background: var(--bg-color-2);
  border-radius: var(--radius-xs);
  margin-bottom: var(--spacing-lg);
  font-size: var(--font-size-base);
}

.current-label {
  color: var(--text-color-2);
  font-weight: 500;
}

.current-version {
  font-family: var(--font-mono);
  font-size: var(--font-size-base);
  background: var(--bg-color-3);
  padding: calc(var(--spacing-xs) / 2) var(--spacing-sm);
  border-radius: var(--radius-2xs);
  color: var(--primary-color);
  font-weight: 600;
}

.action-section {
  display: flex;
  gap: var(--spacing-sm);
  margin-bottom: var(--spacing-lg);
}

.action-btn {
  flex: 1;
  padding: var(--spacing-sm) var(--spacing-md);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-xs);
  background: var(--bg-color-2);
  color: var(--text-color-1);
  cursor: pointer;
  font-size: var(--font-size-base);
  transition: all var(--transition-fast);
}

.action-btn:hover:not(:disabled) {
  background: var(--bg-color-3);
}

.action-btn:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.cleanup-btn {
  background: var(--warning-color);
  color: var(--text-color-on-primary);
  border-color: var(--warning-color);
}

.cleanup-btn:hover:not(:disabled) {
  box-shadow: 0 0 20px var(--glow-primary);
}

.version-list-header {
  display: grid;
  grid-template-columns: 2fr 0.8fr 0.7fr 0.7fr 0.7fr 0.8fr;
  gap: var(--spacing-sm);
  padding: var(--spacing-sm) var(--spacing-md);
  font-size: var(--font-size-xs);
  color: var(--text-color-3);
  font-weight: 500;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  border-bottom: 1px solid var(--border-color);
  margin-bottom: var(--spacing-xs);
}

.version-list {
  display: flex;
  flex-direction: column;
  gap: var(--spacing-xs);
}

.version-item {
  display: grid;
  grid-template-columns: 2fr 0.8fr 0.7fr 0.7fr 0.7fr 0.8fr;
  gap: var(--spacing-sm);
  align-items: center;
  padding: var(--spacing-sm) var(--spacing-md);
  background: var(--bg-color-2);
  border-radius: var(--radius-xs);
  border: 1px solid var(--border-color);
  border-left: 3px solid var(--text-color-3);
  transition: all var(--transition-fast);
  font-size: var(--font-size-base);
}

.version-item:hover {
  border-color: var(--primary-color);
}

.version-item.current {
  border-left-color: var(--success-color);
}

.version-item.unused {
  border-left-color: var(--text-color-3);
}

.version-name {
  font-weight: 500;
  font-size: var(--font-size-base);
  color: var(--text-color-1);
  font-family: var(--font-mono);
  word-break: break-all;
}

.version-status {
  font-size: var(--font-size-xs);
  padding: 1px 8px;
  border-radius: var(--radius-2xs);
  font-weight: 500;
  white-space: nowrap;
}

.version-status.running {
  background: var(--success-bg);
  color: var(--success-color);
}

.version-status.old {
  background: var(--bg-color-4);
  color: var(--text-color-2);
}

.col-boot,
.col-modules,
.col-total {
  font-size: var(--font-size-sm);
  color: var(--text-color-2);
  font-family: var(--font-mono);
}

.remove-btn {
  padding: calc(var(--spacing-xs) / 2) var(--spacing-sm);
  border: 1px solid var(--error-color);
  border-radius: var(--radius-xs);
  background: transparent;
  color: var(--error-color);
  cursor: pointer;
  font-size: var(--font-size-xs);
  transition: all var(--transition-fast);
  white-space: nowrap;
}

.remove-btn:hover:not(:disabled) {
  background: var(--error-color);
  color: var(--text-color-on-primary);
}

.remove-btn:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.current-badge {
  font-size: var(--font-size-xs);
  color: var(--success-color);
  font-weight: 500;
}

.no-versions {
  text-align: center;
  padding: var(--spacing-3xl);
  color: var(--text-color-3);
  font-size: var(--font-size-md);
}

@media (max-width: 768px) {
  .kernel-panel {
    max-width: 100%;
    width: 100%;
    height: 95vh;
    max-height: 95vh;
    border-radius: var(--radius-lg) var(--radius-lg) 0 0;
    margin-top: auto;
  }

  .panel-header {
    padding: var(--spacing-sm) var(--spacing-md);
    min-height: auto;
  }

  .panel-header h3 {
    font-size: var(--font-size-lg);
    font-weight: 500;
  }

  .close-btn {
    font-size: var(--font-size-3xl);
    line-height: 1;
    min-width: 32px;
    min-height: 32px;
    display: flex;
    align-items: center;
    justify-content: center;
  }

  .panel-body {
    padding: var(--spacing-md);
  }

  /* Stats: 2 per row */
  .stats-row {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: var(--spacing-xs);
  }

  .stat-card {
    padding: var(--spacing-sm);
  }

  .stat-value {
    font-size: var(--font-size-3xl);
  }

  .stat-label {
    font-size: var(--font-size-xs);
  }

  /* Current kernel info: wrap */
  .current-info {
    flex-direction: column;
    align-items: flex-start;
    gap: var(--spacing-xs);
    padding: var(--spacing-sm) var(--spacing-md);
    font-size: var(--font-size-sm);
  }

  .current-version {
    font-size: var(--font-size-sm);
    word-break: break-all;
  }

  /* Action buttons: stacked */
  .action-section {
    flex-direction: column;
    gap: var(--spacing-xs);
  }

  .action-btn {
    flex: none;
    width: 100%;
    padding: var(--spacing-sm) var(--spacing-md);
    font-size: var(--font-size-sm);
  }

  .cleanup-btn {
    font-size: var(--font-size-sm);
  }

  /* Table header: hide on mobile */
  .version-list-header {
    display: none;
  }

  /* Version cards: stacked card layout */
  .version-item {
    display: flex;
    flex-direction: column;
    gap: var(--spacing-xs);
    padding: var(--spacing-sm) var(--spacing-md);
  }

  .col-version {
    display: flex;
    align-items: center;
    gap: var(--spacing-xs);
  }

  .version-name {
    font-size: var(--font-size-sm);
  }

  .col-status {
    display: flex;
    align-items: center;
    gap: var(--spacing-sm);
    font-size: var(--font-size-xs);
  }

  .col-status::before {
    content: '状态:';
    color: var(--text-color-3);
    font-size: var(--font-size-xs);
    white-space: nowrap;
    min-width: 36px;
  }

  .version-status {
    font-size: var(--font-size-2xs);
  }

  .col-boot {
    display: flex;
    align-items: center;
    gap: var(--spacing-sm);
    font-size: var(--font-size-xs);
  }

  .col-boot::before {
    content: '引导:';
    color: var(--text-color-3);
    font-size: var(--font-size-xs);
    white-space: nowrap;
    min-width: 36px;
  }

  .col-modules {
    display: flex;
    align-items: center;
    gap: var(--spacing-sm);
    font-size: var(--font-size-xs);
  }

  .col-modules::before {
    content: '模块:';
    color: var(--text-color-3);
    font-size: var(--font-size-xs);
    white-space: nowrap;
    min-width: 36px;
  }

  .col-total {
    display: flex;
    align-items: center;
    gap: var(--spacing-sm);
    font-size: var(--font-size-xs);
  }

  .col-total::before {
    content: '总计:';
    color: var(--text-color-3);
    font-size: var(--font-size-xs);
    white-space: nowrap;
    min-width: 36px;
  }

  .col-action {
    margin-top: var(--spacing-xs);
  }

  .remove-btn {
    width: 100%;
    padding: var(--spacing-xs) var(--spacing-sm);
    font-size: var(--font-size-sm);
    text-align: center;
  }

  .current-badge {
    display: block;
    text-align: center;
    font-size: var(--font-size-sm);
  }

  .no-versions {
    font-size: var(--font-size-sm);
    padding: var(--spacing-2xl);
  }

  .loading-state {
    padding: var(--spacing-lg);
    font-size: var(--font-size-sm);
  }

  .shimmer-stat-row {
    grid-template-columns: repeat(3, 1fr);
    gap: 4px;
  }

  .shimmer-stat-card {
    height: 56px;
  }
}

@media (max-width: 480px) {
  .kernel-panel {
    height: 100vh;
    max-height: 100vh;
    border-radius: 0;
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

  .stat-value {
    font-size: var(--font-size-2xl);
  }
}
</style>
