<template>
  <div class="update-notification" :class="{ closing: isClosing }">
    <div class="update-notification-content">
      <div class="update-notification-text">
        <div class="update-notification-header">
          <div class="update-notification-title">发现新版本</div>
          <div class="update-notification-actions">
            <a :href="updateInfo.url" target="_blank" class="update-notification-btn update-notification-btn-primary">前往下载</a>
            <button class="update-notification-btn update-notification-btn-secondary" @click="ignoreVersion">忽略此版本</button>
          </div>
        </div>
        <div class="update-notification-version">
          当前: v{{ currentVersion }} → 最新: v{{ updateInfo.version }}
        </div>
        <div v-if="changelogHtml" class="update-notification-changelog" v-html="changelogHtml"></div>
      </div>
      <button class="update-notification-close" @click="closeNotification">&times;</button>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'

const props = defineProps({
  updateInfo: {
    type: Object,
    required: true
  },
  currentVersion: {
    type: String,
    default: ''
  }
})

const emit = defineEmits(['close'])

const IGNORE_KEY = 'logmanager_ignore_version'
const CLOSE_TIME_KEY = 'logmanager_update_close_time'
const CLOSE_DURATION = 24 * 60 * 60 * 1000

const isClosing = ref(false)

const changelogHtml = computed(() => {
  if (!props.updateInfo.changelog) return ''
  let text = props.updateInfo.changelog.substring(0, 500)
  text = text.split('\n').filter(line => line.trim().length > 0)
  text = text.map(line => escapeHtml(line.replace(/^-\s*/, '• ')))
  const result = text.join('<br>')
  if (props.updateInfo.changelog.length > 500) {
    return result + '...'
  }
  return result
})

function escapeHtml(text) {
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
  } catch (e) {
    return ''
  }
}

function setIgnoredVersion(version) {
  try {
    localStorage.setItem(IGNORE_KEY, version)
  } catch (e) {}
}

function setCloseTime() {
  try {
    localStorage.setItem(CLOSE_TIME_KEY, Date.now().toString())
  } catch (e) {}
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
  max-width: 450px;
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
}

.update-notification-btn-primary {
  background: var(--primary-gradient);
  color: white;
  box-shadow: var(--shadow-md);
}

.update-notification-btn-primary:hover {
  opacity: 0.9;
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
    max-width: 400px;
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
