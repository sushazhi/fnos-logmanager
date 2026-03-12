<template>
  <div class="update-notification" :class="{ closing: isClosing }">
    <div class="update-notification-content">
      <div class="update-notification-text">
        <!-- 更新中显示进度 -->
        <div v-if="updateStatus.updating" class="update-progress">
          <div class="update-notification-title">正在更新...</div>
          <div class="progress-bar">
            <div class="progress" :style="{ width: updateStatus.progress + '%' }"></div>
          </div>
          <div class="progress-text">{{ updateStatus.progress }}%</div>
          <div class="progress-message">{{ updateStatus.message }}</div>
        </div>
        
        <!-- 正常显示 -->
        <template v-else>
          <div class="update-notification-header">
            <div class="update-notification-title">发现新版本</div>
            <div class="update-notification-actions">
              <button class="update-notification-btn update-notification-btn-primary" @click="startUpdate">立即更新</button>
              <button class="update-notification-btn update-notification-btn-secondary" @click="ignoreVersion">忽略此版本</button>
            </div>
          </div>
          <div class="update-notification-version">
            当前: v{{ currentVersion }} → 最新: v{{ updateInfo.version }}
          </div>
          <!-- eslint-disable-next-line vue/no-v-html -->
          <div v-if="changelogHtml" class="update-notification-changelog" v-html="changelogHtml"></div>
        </template>
      </div>
      <button v-if="!updateStatus.updating" class="update-notification-close" @click="closeNotification">&times;</button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useUpdate } from '../composables/useUpdate'
import type { UpdateInfo } from '../types'

const props = defineProps<{
  updateInfo: UpdateInfo
  currentVersion: string
}>()

const emit = defineEmits<{
  close: []
}>()

const { installUpdate, updateStatus } = useUpdate()

const IGNORE_KEY = 'logmanager_ignore_version'
const CLOSE_TIME_KEY = 'logmanager_update_close_time'

const isClosing = ref(false)

const changelogHtml = computed(() => {
  if (!props.updateInfo.changelog) return ''
  let text: string | string[] = props.updateInfo.changelog.substring(0, 500)
  text = text.split('\n').filter((line: string) => line.trim().length > 0)
  text = text.map((line: string) => escapeHtml(line.replace(/^-\s*/, '• ')))
  const result = (text as string[]).join('<br>')
  if (props.updateInfo.changelog.length > 500) {
    return result + '...'
  }
  return result
})

function escapeHtml(text: string): string {
  if (!text) return ''
  return text
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#039;')
}

function getIgnoredVersion() {
  try {
    return localStorage.getItem(IGNORE_KEY) || ''
  } catch {
    return ''
  }
}

function setIgnoredVersion(version: string) {
  try {
    localStorage.setItem(IGNORE_KEY, version)
  } catch {
    // ignore
  }
}

function setCloseTime() {
  try {
    localStorage.setItem(CLOSE_TIME_KEY, Date.now().toString())
  } catch {
    // ignore
  }
}

function closeNotification() {
  setCloseTime()
  isClosing.value = true
  setTimeout(() => emit('close'), 300)
}

function ignoreVersion() {
  setIgnoredVersion(props.updateInfo.version)
  isClosing.value = true
  setTimeout(() => emit('close'), 300)
}

async function startUpdate() {
  try {
    // 开始更新（不关闭通知，让用户看到进度）
    await installUpdate()
    
    // 更新完成后关闭通知
    closeNotification()
  } catch (error) {
    console.error('更新失败:', error)
    // 更新失败也要关闭通知
    closeNotification()
  }
}

onMounted(() => {
  if (getIgnoredVersion() === props.updateInfo.version) {
    emit('close')
  }
})
</script>

<style scoped>
.update-notification {
  position: fixed;
  bottom: var(--spacing-xl);
  right: var(--spacing-xl);
  z-index: 99999;
  font-family: var(--font-family);
  animation: slideIn var(--transition-slow) ease-out;
  min-width: 480px;
  max-width: 520px;
}

.update-notification.closing {
  animation: slideOut var(--transition-slow) ease-in forwards;
}

@keyframes slideIn {
  from { transform: translateX(100%); opacity: 0; }
  to { transform: translateX(0); opacity: 1; }
}

@keyframes slideOut {
  from { transform: translateX(0); opacity: 1; }
  to { transform: translateX(100%); opacity: 0; }
}

.update-notification-content {
  display: flex;
  align-items: flex-start;
  gap: var(--spacing-md);
  background: var(--card-bg);
  backdrop-filter: blur(25px);
  -webkit-backdrop-filter: blur(25px);
  border: 1px solid var(--border-color);
  box-shadow: var(--shadow-xl);
  padding: var(--spacing-xl) var(--spacing-2xl);
  border-radius: var(--radius-md);
  position: relative;
}

.update-notification-text {
  flex: 1;
}

.update-notification-header {
  display: flex;
  align-items: center;
  justify-content: flex-start;
  gap: var(--spacing-md);
  flex-wrap: wrap;
  padding-right: 40px;
}

.update-notification-title {
  font-weight: 600;
  font-size: 1.125rem;
  color: var(--text-color-1);
  letter-spacing: -0.01em;
  -webkit-font-smoothing: antialiased;
  -moz-osx-font-smoothing: grayscale;
}

.update-notification-version {
  font-size: 0.875rem;
  color: var(--primary-color);
  margin-top: var(--spacing-xs);
  font-weight: 500;
  -webkit-font-smoothing: antialiased;
  -moz-osx-font-smoothing: grayscale;
}

.update-notification-changelog {
  font-size: 0.8125rem;
  color: var(--text-color-2);
  margin-top: var(--spacing-sm);
  line-height: 1.5;
  max-height: 120px;
  overflow-y: auto;
  -webkit-font-smoothing: antialiased;
  -moz-osx-font-smoothing: grayscale;
}

.update-notification-actions {
  display: flex;
  gap: var(--spacing-sm);
  align-items: center;
}

.update-notification-btn {
  padding: var(--spacing-sm) var(--spacing-lg);
  border-radius: var(--radius-sm);
  font-size: 0.875rem;
  font-weight: 500;
  text-decoration: none;
  cursor: pointer;
  border: none;
  transition: all var(--transition-fast);
  letter-spacing: -0.01em;
  white-space: nowrap;
  flex-shrink: 0;
}

.update-notification-btn-primary {
  background: #3b82f6;
  color: white;
  box-shadow: var(--shadow-md);
}

.update-notification-btn-primary:hover {
  background: #2563eb;
  transform: translateY(-2px);
  box-shadow: var(--shadow-lg);
}

.update-notification-btn-primary:active {
  transform: scale(0.98);
}

.update-notification-btn-secondary {
  background: var(--bg-color-2);
  color: var(--text-color-2);
  border: 1px solid var(--border-color);
}

.update-notification-btn-secondary:hover {
  background: var(--bg-color-3);
  transform: translateY(-2px);
  color: var(--text-color-1);
}

.update-notification-btn-secondary:active {
  transform: scale(0.98);
}

/* 更新进度样式 */
.update-progress {
  text-align: center;
  padding: var(--spacing-md) 0;
}

.progress-bar {
  width: 100%;
  height: 8px;
  background: var(--bg-color-3);
  border-radius: 4px;
  overflow: hidden;
  margin: var(--spacing-md) 0;
}

.progress {
  height: 100%;
  background: #3b82f6;
  transition: width 0.3s;
}

.progress-text {
  font-size: 1.5rem;
  font-weight: 600;
  color: #3b82f6;
  margin: var(--spacing-sm) 0;
}

.progress-message {
  font-size: 0.875rem;
  color: var(--text-color-2);
}

.update-notification-close {
  background: none;
  border: none;
  font-size: 1.5rem;
  color: var(--text-color-3);
  cursor: pointer;
  padding: var(--spacing-xs) var(--spacing-sm);
  line-height: 1;
  position: absolute;
  top: var(--spacing-sm);
  right: var(--spacing-sm);
  transition: all var(--transition-fast);
}

.update-notification-close:hover {
  color: var(--text-color-1);
  transform: scale(1.15);
}

/* 深色主题 */
:global(.dark-theme) .update-notification-content {
  background: var(--card-bg);
  border: 1px solid var(--border-color);
}

:global(.dark-theme) .update-notification-title {
  color: var(--text-color-1);
}

:global(.dark-theme) .update-notification-version {
  color: var(--primary-color);
}

:global(.dark-theme) .update-notification-changelog {
  color: var(--text-color-2);
}

:global(.dark-theme) .update-notification-close {
  color: var(--text-color-3);
}

:global(.dark-theme) .update-notification-close:hover {
  color: var(--text-color-1);
}

:global(.dark-theme) .update-notification-btn-secondary {
  background: var(--bg-color-2);
  color: var(--text-color-2);
  border: 1px solid var(--border-color);
}

:global(.dark-theme) .update-notification-btn-secondary:hover {
  background: var(--bg-color-3);
  color: var(--text-color-1);
}

/* 移动端适配 */
@media (max-width: 480px) {
  .update-notification {
    bottom: var(--spacing-sm);
    right: var(--spacing-sm);
    left: var(--spacing-sm);
    min-width: auto;
    max-width: none;
  }

  .update-notification-content {
    padding: var(--spacing-lg) 40px var(--spacing-lg) var(--spacing-lg);
    backdrop-filter: none;
    -webkit-backdrop-filter: none;
  }

  .update-notification-header {
    flex-direction: column;
    align-items: flex-start;
    gap: var(--spacing-sm);
    padding-right: 0;
  }

  .update-notification-title {
    font-size: 1rem;
    -webkit-font-smoothing: antialiased;
    -moz-osx-font-smoothing: grayscale;
  }

  .update-notification-version {
    font-size: 0.8125rem;
    -webkit-font-smoothing: antialiased;
    -moz-osx-font-smoothing: grayscale;
  }

  .update-notification-changelog {
    font-size: 0.8125rem;
    margin-top: var(--spacing-sm);
    max-height: 80px;
    -webkit-font-smoothing: antialiased;
    -moz-osx-font-smoothing: grayscale;
  }

  .update-notification-actions {
    width: 100%;
    flex-wrap: wrap;
    gap: var(--spacing-sm);
  }

  .update-notification-btn {
    padding: var(--spacing-xs) var(--spacing-md);
    font-size: 0.8125rem;
    flex: 1;
    min-width: 80px;
    text-align: center;
    -webkit-font-smoothing: antialiased;
    -moz-osx-font-smoothing: grayscale;
  }

  .update-notification-close {
    position: absolute;
    top: var(--spacing-xs);
    right: var(--spacing-xs);
    font-size: 1.25rem;
    width: 28px;
    height: 28px;
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 0;
    line-height: 1;
  }
}

/* 平板适配 */
@media (min-width: 481px) and (max-width: 768px) {
  .update-notification {
    bottom: var(--spacing-md);
    right: var(--spacing-md);
    min-width: auto;
    max-width: 450px;
  }

  .update-notification-content {
    padding: var(--spacing-lg) var(--spacing-xl);
  }

  .update-notification-title {
    font-size: 1rem;
  }

  .update-notification-version {
    font-size: 0.8125rem;
  }

  .update-notification-btn {
    padding: var(--spacing-xs) var(--spacing-lg);
    font-size: 0.8125rem;
  }

  .update-notification-changelog {
    font-size: 0.75rem;
  }
}
</style>
