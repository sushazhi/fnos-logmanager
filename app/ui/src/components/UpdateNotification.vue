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
  bottom: 20px;
  right: 20px;
  z-index: 99999;
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
  animation: slideIn 0.3s ease-out;
  max-width: 450px;
}

.update-notification.closing {
  animation: slideOut 0.3s ease-in forwards;
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
  gap: 12px;
  background: rgba(255, 255, 255, 0.95);
  backdrop-filter: blur(25px);
  -webkit-backdrop-filter: blur(25px);
  border: 1px solid rgba(0, 0, 0, 0.1);
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.15);
  padding: 20px 24px;
  border-radius: 12px;
  position: relative;
}

.update-notification-text {
  flex: 1;
}

.update-notification-header {
  display: flex;
  align-items: center;
  justify-content: flex-start;
  gap: 12px;
  flex-wrap: wrap;
  padding-right: 40px;
}

.update-notification-title {
  font-weight: 700;
  font-size: 18px;
  color: #2c3e50;
  letter-spacing: 0.5px;
  -webkit-font-smoothing: antialiased;
  -moz-osx-font-smoothing: grayscale;
}

.update-notification-version {
  font-size: 14px;
  color: #667eea;
  margin-top: 6px;
  font-weight: 500;
  -webkit-font-smoothing: antialiased;
  -moz-osx-font-smoothing: grayscale;
}

.update-notification-changelog {
  font-size: 13px;
  color: #34495e;
  margin-top: 12px;
  line-height: 1.6;
  max-height: 120px;
  overflow-y: auto;
  -webkit-font-smoothing: antialiased;
  -moz-osx-font-smoothing: grayscale;
}

.update-notification-actions {
  display: flex;
  gap: 12px;
  align-items: center;
}

.update-notification-btn {
  padding: 10px 20px;
  border-radius: 8px;
  font-size: 14px;
  font-weight: 600;
  text-decoration: none;
  cursor: pointer;
  border: none;
  transition: all 0.3s ease;
  letter-spacing: 0.5px;
}

.update-notification-btn-primary {
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  color: white;
  box-shadow: 0 4px 15px rgba(102, 126, 234, 0.4);
}

.update-notification-btn-primary:hover {
  opacity: 0.9;
  transform: translateY(-2px);
  box-shadow: 0 6px 20px rgba(102, 126, 234, 0.5);
}

.update-notification-btn-secondary {
  background: rgba(245, 245, 245, 0.95);
  color: #7f8c8d;
  border: 1px solid rgba(0, 0, 0, 0.1);
}

.update-notification-btn-secondary:hover {
  background: rgba(232, 232, 232, 0.95);
  transform: translateY(-2px);
  color: #5a6b7d;
}

.update-notification-close {
  background: none;
  border: none;
  font-size: 24px;
  color: #999999;
  cursor: pointer;
  padding: 4px 8px;
  line-height: 1;
  position: absolute;
  top: 12px;
  right: 12px;
  transition: all 0.3s ease;
}

.update-notification-close:hover {
  color: #333333;
  transform: scale(1.15);
}

/* 深色主题 */
:global(.dark-theme) .update-notification-content {
  background: rgba(45, 45, 45, 0.95);
  border: 1px solid rgba(255, 255, 255, 0.1);
}

:global(.dark-theme) .update-notification-title {
  color: #fff;
}

:global(.dark-theme) .update-notification-version {
  color: #667eea;
}

:global(.dark-theme) .update-notification-changelog {
  color: #ccc;
}

:global(.dark-theme) .update-notification-close {
  color: #999;
}

:global(.dark-theme) .update-notification-close:hover {
  color: #fff;
}

:global(.dark-theme) .update-notification-btn-secondary {
  background: rgba(60, 60, 60, 0.95);
  color: #ccc;
  border: 1px solid rgba(255, 255, 255, 0.1);
}

:global(.dark-theme) .update-notification-btn-secondary:hover {
  background: rgba(70, 70, 70, 0.95);
  color: #fff;
}

/* 移动端适配 */
@media (max-width: 480px) {
  .update-notification {
    bottom: 10px;
    right: 10px;
    left: 10px;
    max-width: none;
  }

  .update-notification-content {
    padding: 16px 40px 16px 16px;
    backdrop-filter: none;
    -webkit-backdrop-filter: none;
  }

  .update-notification-header {
    flex-direction: column;
    align-items: flex-start;
    gap: 8px;
    padding-right: 0;
  }

  .update-notification-title {
    font-size: 16px;
    -webkit-font-smoothing: antialiased;
    -moz-osx-font-smoothing: grayscale;
  }

  .update-notification-version {
    font-size: 13px;
    -webkit-font-smoothing: antialiased;
    -moz-osx-font-smoothing: grayscale;
  }

  .update-notification-changelog {
    font-size: 13px;
    margin-top: 8px;
    max-height: 80px;
    -webkit-font-smoothing: antialiased;
    -moz-osx-font-smoothing: grayscale;
  }

  .update-notification-actions {
    width: 100%;
    flex-wrap: wrap;
    gap: 8px;
  }

  .update-notification-btn {
    padding: 8px 12px;
    font-size: 13px;
    flex: 1;
    min-width: 80px;
    text-align: center;
    -webkit-font-smoothing: antialiased;
    -moz-osx-font-smoothing: grayscale;
  }

  .update-notification-close {
    position: absolute;
    top: 8px;
    right: 8px;
    font-size: 20px;
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
    bottom: 15px;
    right: 15px;
    max-width: 400px;
  }

  .update-notification-content {
    padding: 18px 20px;
  }

  .update-notification-title {
    font-size: 16px;
  }

  .update-notification-version {
    font-size: 13px;
  }

  .update-notification-btn {
    padding: 9px 18px;
    font-size: 13px;
  }

  .update-notification-changelog {
    font-size: 12px;
  }
}
</style>
