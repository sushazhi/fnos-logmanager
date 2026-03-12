<template>
  <div class="update-checker">
    <!-- 版本显示 -->
    <div class="version-info" @click="showUpdateDialog">
      <span class="version-text">v{{ appVersion }}</span>
      <span v-if="updateInfo" class="update-badge">🚀 新版本</span>
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
              <!-- eslint-disable-next-line vue/no-v-html -->
              <div class="changelog-content" v-html="updateInfo.changelog"></div>
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
import { ref, onMounted } from 'vue'
import { useUpdate } from '../composables/useUpdate'
import { showNotification } from '../utils/notification'

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
  border-radius: 4px;
  transition: background-color 0.2s;
}

.version-info:hover {
  background-color: rgba(255, 255, 255, 0.1);
}

.version-text {
  font-size: 12px;
  color: #94a3b8;
}

.update-badge {
  font-size: 11px;
  color: #fbbf24;
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
  background-color: rgba(0, 0, 0, 0.5);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 9999;
}

.update-dialog {
  background: #1e293b;
  border-radius: 12px;
  width: 90%;
  max-width: 500px;
  max-height: 80vh;
  overflow-y: auto;
  box-shadow: 0 20px 25px -5px rgba(0, 0, 0, 0.3);
}

.dialog-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 16px 20px;
  border-bottom: 1px solid #334155;
}

.dialog-header h3 {
  margin: 0;
  font-size: 18px;
  color: #e2e8f0;
}

.close-btn {
  background: none;
  border: none;
  color: #94a3b8;
  font-size: 24px;
  cursor: pointer;
  padding: 0;
  line-height: 1;
}

.close-btn:hover {
  color: #e2e8f0;
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
  border: 3px solid #334155;
  border-top-color: #3b82f6;
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
  color: #94a3b8;
}

.version-compare strong {
  color: #e2e8f0;
}

.changelog {
  margin: 20px 0;
  text-align: left;
  background: #0f172a;
  border-radius: 8px;
  padding: 16px;
}

.changelog h4 {
  margin: 0 0 12px 0;
  color: #e2e8f0;
  font-size: 14px;
}

.changelog-content {
  color: #94a3b8;
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
  border-radius: 6px;
  font-size: 14px;
  font-weight: 500;
  cursor: pointer;
  border: none;
  transition: all 0.2s;
}

.btn-primary {
  background: #3b82f6;
  color: white;
}

.btn-primary:hover {
  background: #2563eb;
}

.btn-secondary {
  background: #475569;
  color: #e2e8f0;
}

.btn-secondary:hover {
  background: #64748b;
}

.updating {
  text-align: center;
}

.progress-bar {
  width: 100%;
  height: 8px;
  background: #334155;
  border-radius: 4px;
  overflow: hidden;
  margin-bottom: 12px;
}

.progress {
  height: 100%;
  background: #3b82f6;
  transition: width 0.3s;
}

.progress-text {
  font-size: 24px;
  font-weight: 600;
  color: #3b82f6;
  margin: 8px 0;
}

.update-message {
  color: #94a3b8;
  font-size: 14px;
}

.up-to-date {
  text-align: center;
  padding: 20px 0;
}

.check-icon {
  width: 60px;
  height: 60px;
  background: #10b981;
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
  color: #94a3b8;
}

.up-to-date .version-text {
  font-size: 16px;
  color: #e2e8f0;
  font-weight: 600;
}
</style>
