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
          <div v-if="changelogLines.length" class="update-notification-changelog">
            <p
              v-for="(line, index) in changelogLines"
              :key="index"
              class="update-notification-changelog-line"
            >{{ line }}</p>
          </div>
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

const CHANGELOG_MAX_LENGTH = 600

// 更新内容按行拆成数组渲染，交给浏览器自动换行。
// 之前把各行用 <br> 拼成一段 HTML 后整体输出，长条目在窄卡片里会横向溢出
// （出现横向滚动条、文字被裁掉），且无法在行之间留出间距。
const changelogLines = computed<string[]>(() => {
  const raw = props.updateInfo.changelog
  if (!raw) return []

  const truncated = raw.length > CHANGELOG_MAX_LENGTH
  const lines = raw
    .substring(0, CHANGELOG_MAX_LENGTH)
    .split('\n')
    .map(line => line.trim())
    // 丢掉空行与 Markdown 分隔线（如 ---），纯文本展示里它们没有意义
    .filter(line => line.length > 0 && !/^[-*_]{3,}$/.test(line))
    .map(line => line.replace(/^[-*]\s+/, '• '))

  if (truncated && lines.length > 0) {
    lines.push('...')
  }
  return lines
})

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
  /* 卡片本身不允许超出视口：窄窗口下自动收窄，避免整体被裁切 */
  min-width: min(480px, calc(100vw - 2 * var(--spacing-xl)));
  max-width: min(600px, calc(100vw - 2 * var(--spacing-xl)));
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
  background: var(--glass-bg-strong);
  backdrop-filter: blur(var(--glass-blur));
  -webkit-backdrop-filter: blur(var(--glass-blur));
  border: 1px solid var(--glass-border-strong);
  box-shadow: var(--glass-shadow), var(--depth-4);
  padding: var(--spacing-xl) var(--spacing-2xl);
  border-radius: var(--radius-md);
  position: relative;
}

.update-notification-text {
  flex: 1;
  /* flex 子项默认 min-width:auto，长英文/URL 会把卡片顶宽并溢出，必须显式归零 */
  min-width: 0;
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
  font-size: var(--font-size-2xl);
  color: var(--text-color-1);
  letter-spacing: -0.01em;
  -webkit-font-smoothing: antialiased;
  -moz-osx-font-smoothing: grayscale;
}

.update-notification-version {
  font-size: var(--font-size-md);
  color: var(--primary-color);
  margin-top: var(--spacing-xs);
  font-weight: 500;
  -webkit-font-smoothing: antialiased;
  -moz-osx-font-smoothing: grayscale;
}

.update-notification-changelog {
  font-size: var(--font-size-base);
  color: var(--text-color-2);
  margin-top: var(--spacing-sm);
  line-height: 1.5;
  /* 更新内容通常较长：卡片给足高度，仅在超出时纵向滚动，横向永不溢出 */
  max-height: min(320px, 40vh);
  overflow-y: auto;
  overflow-x: hidden;
  overscroll-behavior: contain;
  /* 中文正常换行；长英文单词 / URL 强制断行，避免撑破卡片 */
  overflow-wrap: anywhere;
  word-break: break-word;
  -webkit-font-smoothing: antialiased;
  -moz-osx-font-smoothing: grayscale;
}

.update-notification-changelog-line {
  margin: 0;
}

.update-notification-changelog-line + .update-notification-changelog-line {
  margin-top: var(--spacing-xs);
}

.update-notification-actions {
  display: flex;
  gap: var(--spacing-sm);
  align-items: center;
}

.update-notification-btn {
  padding: var(--spacing-sm) var(--spacing-lg);
  border-radius: var(--radius-sm);
  font-size: var(--font-size-md);
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
  background: var(--primary-color);
  color: var(--text-color-on-primary);
  box-shadow: var(--shadow-md);
}

.update-notification-btn-primary:hover {
  background: var(--primary-hover);
  transform: translateY(-2px);
  box-shadow: 0 0 24px var(--glow-primary), var(--shadow-lg);
}

.update-notification-btn-primary:active {
  transform: scale(0.98);
}

.update-notification-btn-secondary {
  background: var(--glass-bg);
  backdrop-filter: blur(var(--glass-blur));
  -webkit-backdrop-filter: blur(var(--glass-blur));
  color: var(--text-color-2);
  border: 1px solid var(--glass-border);
}

.update-notification-btn-secondary:hover {
  background: var(--glass-bg-strong);
  border-color: var(--glass-border-strong);
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
  border-radius: var(--radius-2xs);
  overflow: hidden;
  margin: var(--spacing-md) 0;
}

.progress {
  height: 100%;
  background: linear-gradient(90deg, var(--primary-color), var(--primary-hover));
  box-shadow: 0 0 12px var(--glow-primary);
  border-radius: inherit;
  transition: width var(--transition-slow) var(--ease-spring);
}

.progress-text {
  font-size: var(--font-size-5xl);
  font-weight: 600;
  color: var(--primary-color);
  margin: var(--spacing-sm) 0;
}

.progress-message {
  font-size: var(--font-size-md);
  color: var(--text-color-2);
}

.update-notification-close {
  background: none;
  border: none;
  font-size: var(--font-size-5xl);
  color: var(--text-color-3);
  cursor: pointer;
  padding: var(--spacing-xs) var(--spacing-sm);
  line-height: 1;
  position: absolute;
  top: var(--spacing-sm);
  right: var(--spacing-sm);
  transition: all var(--transition-spring);
}

.update-notification-close:hover {
  color: var(--text-color-1);
  transform: scale(1.2) rotate(90deg);
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
  background: var(--glass-bg);
  color: var(--text-color-2);
  border: 1px solid var(--glass-border);
}

:global(.dark-theme) .update-notification-btn-secondary:hover {
  background: var(--glass-bg-strong);
  border-color: var(--glass-border-strong);
  color: var(--text-color-1);
}

/* 移动端适配 */
@media (max-width: 480px) {
  .update-notification {
    bottom: var(--spacing-sm);
    right: var(--spacing-sm);
    left: var(--spacing-sm);
    /* 不能用 min-width: auto：长英文/URL 会让卡片按 min-content 撑宽并溢出屏幕，
       必须允许收缩到 left/right 限定的宽度，由内部文本自行换行 */
    min-width: 0;
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
    font-size: var(--font-size-xl);
    -webkit-font-smoothing: antialiased;
    -moz-osx-font-smoothing: grayscale;
  }

  .update-notification-version {
    font-size: var(--font-size-base);
    -webkit-font-smoothing: antialiased;
    -moz-osx-font-smoothing: grayscale;
  }

  .update-notification-changelog {
    font-size: var(--font-size-base);
    margin-top: var(--spacing-sm);
    max-height: min(240px, 32vh);
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
    font-size: var(--font-size-base);
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
    font-size: var(--font-size-3xl);
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
    font-size: var(--font-size-xl);
  }

  .update-notification-version {
    font-size: var(--font-size-base);
  }

  .update-notification-btn {
    padding: var(--spacing-xs) var(--spacing-lg);
    font-size: var(--font-size-base);
  }

  .update-notification-changelog {
    font-size: var(--font-size-sm);
  }
}
</style>
