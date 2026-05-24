<template>
  <div class="update-checker">
    <!-- 版本显示 -->
    <div class="version-info" @click="showUpdateDialog">
      <span class="version-text">v{{ appVersion }}</span>
      <span v-if="updateInfo" class="update-badge">新版本</span>
    </div>

    <!-- 更新对话框 -->
    <div v-if="showDialog" class="update-dialog-overlay" @click="closeDialog">
      <div class="update-dialog" @click.stop>
        <div class="dialog-header">
          <h3>应用更新</h3>
          <button class="close-btn" @click="closeDialog">&times;</button>
        </div>

        <div class="dialog-body">
          <!-- 检查更新中 -->
          <div v-if="checking" class="checking">
            <div class="spinner"></div>
            <p>正在检查更新...</p>
          </div>

          <!-- 更新可用 -->
          <div v-else-if="updateInfo && !updateStatus.updating" class="update-available">
            <div class="version-compare">
              <p>当前版本: <strong>v{{ appVersion }}</strong></p>
              <p>最新版本: <strong>v{{ updateInfo.version }}</strong></p>
            </div>
            <div v-if="updateInfo.changelog" class="changelog">
              <h4>更新内容：</h4>
              <!-- 使用 DOMPurify 清理后的安全 HTML -->
              <div class="changelog-content" v-html="safeChangelog"></div>
            </div>
            <div class="update-actions">
              <button class="btn-primary" @click="startUpdate">
                立即更新
              </button>
              <button class="btn-secondary" @click="handleIgnoreVersion">
                忽略此版本
              </button>
              <button class="btn-secondary" @click="closeDialog">
                稍后提醒
              </button>
            </div>
          </div>

          <!-- 更新中 -->
          <div v-else-if="updateStatus.updating" class="updating">
            <div class="progress-bar">
              <div class="progress" :style="{ width: updateStatus.progress + '%' }"></div>
            </div>
            <p class="progress-text">{{ updateStatus.progress }}%</p>
            <p class="update-message">{{ updateStatus.message }}</p>
          </div>

          <!-- 已是最新 -->
          <div v-else class="up-to-date">
            <div class="check-icon">✓</div>
            <p>已是最新版本</p>
            <p class="version-text">v{{ appVersion }}</p>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useUpdate } from '../composables/useUpdate'
import { showNotification } from '../utils/notification'
import DOMPurify from 'dompurify'

const {
  appVersion,
  updateInfo,
  updateStatus,
  checkForUpdates,
  installUpdate,
  ignoreVersion,
  setCloseTime
} = useUpdate()

const checking = ref(false)
const showDialog = ref(false)

// 安全清理 changelog HTML，防止 XSS
const safeChangelog = computed(() => {
  if (!updateInfo.value?.changelog) return ''
  return DOMPurify.sanitize(updateInfo.value.changelog, {
    ALLOWED_TAGS: ['p', 'br', 'ul', 'ol', 'li', 'strong', 'em', 'a', 'h1', 'h2', 'h3', 'h4', 'code', 'pre'],
    ALLOWED_ATTR: ['href', 'target', 'rel'],
    ALLOWED_URI_REGEXP: /^https?:\/\/(github\.com|api\.github\.com|objects\.githubusercontent\.com)\//i,
    ADD_ATTR: ['rel', 'target'],
    FORCE_BODY: true
  }).replace(/<a /g, '<a rel="noopener noreferrer" target="_blank" ')
})

// 检查更新
async function check() {
  checking.value = true
  try {
    const result = await checkForUpdates()
    if (result) {
      showDialog.value = true
    }
  } catch {
    showNotification('检查更新失败', 'error')
  } finally {
    checking.value = false
  }
}

// 开始更新
async function startUpdate() {
  try {
    await installUpdate()
  } catch {
    showNotification('安装更新失败', 'error')
  }
}

// 显示对话框
function showUpdateDialog() {
  showDialog.value = true
  if (!checking.value && !updateStatus.value.updating) {
    check()
  }
}

// 关闭对话框
function closeDialog() {
  if (!updateStatus.value.updating) {
    showDialog.value = false
    setCloseTime()
  }
}

// 忽略此版本
function handleIgnoreVersion() {
  if (updateInfo.value) {
    ignoreVersion(updateInfo.value.version)
    showDialog.value = false
    showNotification('已忽略此版本', 'info')
  }
}

// 组件挂载时检查更新
onMounted(() => {
  // 每天检查一次更新
  const lastCheck = localStorage.getItem('lastUpdateCheck')
  const now = Date.now()
  
  if (!lastCheck || now - parseInt(lastCheck) > 24 * 60 * 60 * 1000) {
    check().then(() => {
      localStorage.setItem('lastUpdateCheck', now.toString())
    })
  }
})
</script>

<style scoped>
.update-checker {
  display: inline-block;
}

.version-info {
  display: flex;
  align-items: center;
  gap: 8px;
  cursor: pointer;
  padding: 4px 8px;
  border-radius: var(--radius-2xs);
  transition: background-color 0.2s;
}

.version-info:hover {
  background-color: var(--bg-color-3);
}

.version-text {
  font-size: var(--font-size-sm);
  color: var(--text-color-3);
}

.update-badge {
  font-size: var(--font-size-2xs);
  color: var(--warning-color);
  font-weight: 600;
  animation: pulse 2s infinite;
}

@keyframes pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.5; }
}

.update-dialog-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background-color: var(--overlay);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 9999;
}

.update-dialog {
  background: var(--card-bg);
  border-radius: var(--radius-sm);
  width: 90%;
  max-width: 500px;
  max-height: 80vh;
  overflow-y: auto;
  box-shadow: 0 20px 25px -5px var(--overlay);
}

.dialog-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 16px 20px;
  border-bottom: 1px solid var(--border-color);
}

.dialog-header h3 {
  margin: 0;
  font-size: 18px;
  color: var(--text-color-1);
}

.close-btn {
  background: none;
  border: none;
  color: var(--text-color-3);
  font-size: 24px;
  cursor: pointer;
  padding: 0;
  line-height: 1;
}

.close-btn:hover {
  color: var(--text-color-1);
}

.dialog-body {
  padding: 20px;
}

.checking {
  text-align: center;
  padding: 20px 0;
}

.spinner {
  width: 40px;
  height: 40px;
  border: 3px solid var(--text-color-1);
  border-top-color: var(--primary-color);
  border-radius: 50%;
  animation: spin 1s linear infinite;
  margin: 0 auto 16px;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}

.update-available {
  text-align: center;
}

.version-compare {
  margin-bottom: 20px;
}

.version-compare p {
  margin: 8px 0;
  color: var(--text-color-3);
}

.version-compare strong {
  color: var(--text-color-1);
}

.changelog {
  margin: 20px 0;
  text-align: left;
  background: var(--bg-color-2);
  border-radius: var(--radius-xs);
  padding: 16px;
}

.changelog h4 {
  margin: 0 0 12px 0;
  color: var(--text-color-1);
  font-size: 14px;
}

.changelog-content {
  color: var(--text-color-3);
  font-size: 13px;
  line-height: 1.6;
  max-height: 200px;
  overflow-y: auto;
}

.changelog-content :deep(ul) {
  margin: 8px 0;
  padding-left: 20px;
}

.changelog-content :deep(li) {
  margin: 4px 0;
}

.update-actions {
  display: flex;
  gap: 12px;
  justify-content: center;
  flex-wrap: wrap;
}

.btn-primary,
.btn-secondary {
  padding: 10px 20px;
  border-radius: var(--radius-2xs);
  font-size: 14px;
  font-weight: 500;
  cursor: pointer;
  border: none;
  transition: all 0.2s;
}

.btn-primary {
  background: var(--primary-color);
  color: white;
}

.btn-primary:hover {
  background: var(--primary-hover);
}

.btn-secondary {
  background: var(--bg-color-2);
  color: var(--text-color-1);
  border: 1px solid var(--border-color);
}

.btn-secondary:hover {
  background: var(--bg-color-3);
}

.updating {
  text-align: center;
}

.progress-bar {
  width: 100%;
  height: 8px;
  background: var(--text-color-1);
  border-radius: var(--radius-2xs);
  overflow: hidden;
  margin-bottom: 12px;
}

.progress {
  height: 100%;
  background: var(--primary-color);
  transition: width 0.3s;
}

.progress-text {
  font-size: 24px;
  font-weight: 600;
  color: var(--primary-color);
  margin: 8px 0;
}

.update-message {
  color: var(--text-color-3);
  font-size: 14px;
}

.up-to-date {
  text-align: center;
  padding: 20px 0;
}

.check-icon {
  width: 60px;
  height: 60px;
  background: var(--success-color);
  color: white;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 32px;
  margin: 0 auto 16px;
}

.up-to-date p {
  margin: 8px 0;
  color: var(--text-color-3);
}

.up-to-date .version-text {
  font-size: 16px;
  color: var(--text-color-1);
  font-weight: 600;
}
</style>
