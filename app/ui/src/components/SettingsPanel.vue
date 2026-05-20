<template>
  <div class="settings-panel">
    <div class="settings-header">
      <h3>设置</h3>
      <button class="close-btn" @click="$emit('close')">×</button>
    </div>
    
    <div class="settings-body">
      <div class="setting-item">
        <label>字体大小</label>
        <div class="font-size-controls">
          <button @click="decreaseFontSize">A-</button>
          <span class="font-size-value">{{ fontSize }}px</span>
          <button @click="increaseFontSize">A+</button>
        </div>
      </div>
      
      <div class="setting-item">
        <label>主题模式</label>
        <div class="theme-buttons">
          <button
            :class="{ active: theme === 'light' }"
            @click="setTheme('light')"
          >日间</button>
          <button
            :class="{ active: theme === 'dark' }"
            @click="setTheme('dark')"
          >夜间</button>
          <button
            :class="{ active: theme === 'auto' }"
            @click="setTheme('auto')"
          >自动</button>
        </div>
      </div>
      
      <div class="setting-item">
        <label>主题色</label>
        <div class="color-options">
          <button 
            v-for="color in colors" 
            :key="color.value"
            class="color-btn"
            :class="{ active: primaryColor === color.value }"
            :style="{ background: color.gradient }"
            @click="setColor(color.value)"
            :title="color.name"
          ></button>
        </div>
      </div>
      
      <div class="divider"></div>
      
      <div class="setting-item">
        <label>审计日志</label>
        <button class="action-btn" @click="$emit('showAudit')">
          查看审计日志
        </button>
      </div>

      <div class="divider"></div>

      <div class="setting-item">
        <label>版本更新</label>
        <div class="version-display">当前版本: <strong>v{{ appVersion }}</strong></div>
        <button class="action-btn" :disabled="checking" @click="manualCheck">
          <span v-if="checking" class="checking-spinner"></span>
          {{ checking ? '正在检查...' : '检查更新' }}
        </button>
        <div v-if="checkResult" class="check-result" :class="checkResult.type">
          {{ checkResult.message }}
          <span v-if="checkResult.type === 'success' && updateInfo" class="result-update-btn" @click="startUpdate">立即更新</span>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import api from '../services/api'
import { applyThemeColor } from '../composables/useThemeColor'
import { useUpdate } from '../composables/useUpdate'

interface ThemeSettings {
  fontSize: number
  theme: string
  primaryColor: string
}

interface ColorOption {
  name: string
  value: string
  gradient: string
}

const emit = defineEmits<{
  close: []
  update: [settings: ThemeSettings]
  showAudit: []
}>()

const { appVersion, updateInfo, checkForUpdates, installUpdate } = useUpdate()

const checking = ref(false)
const checkResult = ref<{ type: 'success' | 'error' | 'info'; message: string } | null>(null)

async function manualCheck() {
  if (checking.value) return
  checking.value = true
  checkResult.value = null
  try {
    const result = await checkForUpdates()
    if (result) {
      checkResult.value = { type: 'success', message: `发现新版本 v${result.version}` }
    } else {
      checkResult.value = { type: 'info', message: '已是最新版本' }
    }
  } catch {
    checkResult.value = { type: 'error', message: '检查更新失败，请稍后重试' }
  } finally {
    checking.value = false
  }
}

async function startUpdate() {
  try {
    await installUpdate()
  } catch {
    checkResult.value = { type: 'error', message: '安装更新失败' }
  }
}

const fontSize = ref<number>(16)
const theme = ref<'light' | 'dark' | 'auto'>('light')
const primaryColor = ref<string>('#5b9bd5')

const colors: ColorOption[] = [
  { name: '紫色', value: '#9b59b6', gradient: 'linear-gradient(135deg, #9b59b6 0%, #8e44ad 100%)' },
  { name: '蓝色', value: '#3498db', gradient: 'linear-gradient(135deg, #3498db 0%, #2980b9 100%)' },
  { name: '青色', value: '#1abc9c', gradient: 'linear-gradient(135deg, #1abc9c 0%, #16a085 100%)' },
  { name: '粉色', value: '#e74c8c', gradient: 'linear-gradient(135deg, #e74c8c 0%, #c0392b 100%)' },
  { name: '莫兰迪渐变', value: '#8ec5fc', gradient: 'linear-gradient(135deg, #e0c3fc 0%, #8ec5fc 33%, #a8edea 66%, #fed6e3 100%)' },
]

function increaseFontSize(): void {
  if (fontSize.value < 24) {
    fontSize.value += 2
    applyFontSize()
    saveSettings()
  }
}

function decreaseFontSize(): void {
  if (fontSize.value > 12) {
    fontSize.value -= 2
    applyFontSize()
    saveSettings()
  }
}

function setTheme(newTheme: 'light' | 'dark' | 'auto'): void {
  theme.value = newTheme
  applyTheme()
  saveSettings()
}

function setColor(color: string): void {
  primaryColor.value = color
  applyColor()
  saveSettings()
}

function applyTheme() {
  const root = document.documentElement
  const isDark = theme.value === 'dark' || 
    (theme.value === 'auto' && window.matchMedia('(prefers-color-scheme: dark)').matches)
  
  if (isDark) {
    root.classList.add('dark-theme')
  } else {
    root.classList.remove('dark-theme')
  }
}

function applyColor() {
  applyThemeColor(primaryColor.value)
}

function applyFontSize(): void {
  const root = document.documentElement
  root.style.setProperty('--base-font-size', `${fontSize.value}px`)
}

function applyAll(): void {
  applyTheme()
  applyColor()
  applyFontSize()
}

function saveSettings(): void {
  const settings: ThemeSettings = {
    fontSize: fontSize.value,
    theme: theme.value,
    primaryColor: primaryColor.value
  }
  localStorage.setItem('logmanager_settings', JSON.stringify(settings))
  emit('update', settings)
}

function loadSettings(): void {
  try {
    const saved = localStorage.getItem('logmanager_settings')
    if (saved) {
      const settings = JSON.parse(saved) as ThemeSettings
      fontSize.value = settings.fontSize || 16
      theme.value = (settings.theme as 'light' | 'dark' | 'auto') || 'light'
      primaryColor.value = settings.primaryColor || '#5b9bd5'
    }
  } catch (e) {
    console.warn('Failed to load settings:', e)
    localStorage.removeItem('logmanager_settings')
  }
  
  applyAll()
}

onMounted(() => {
  loadSettings()
  
  window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', () => {
    if (theme.value === 'auto') {
      applyTheme()
    }
  })
})
</script>

<style>
.settings-panel {
  background: var(--card-bg);
  border-radius: var(--radius-md);
  box-shadow: var(--shadow-xl);
  max-width: 400px;
  width: 100%;
}

.settings-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: var(--spacing-md) var(--spacing-xl);
  border-bottom: 1px solid var(--border-color);
  gap: var(--spacing-md);
}

.settings-header h3 {
  margin: 0;
  font-size: var(--font-size-xl);
  font-weight: 500;
  color: var(--text-color-1);
  flex: 1;
  white-space: nowrap;
}

.close-btn {
  background: none;
  border: none;
  font-size: var(--font-size-5xl);
  cursor: pointer;
  color: var(--text-color-2);
  padding: 0;
  line-height: 1;
  flex: none;
  flex-shrink: 0;
  transition: color var(--transition-fast);
}

.close-btn:hover {
  color: var(--text-color-1);
}

.settings-body {
  padding: var(--spacing-xl);
  max-height: 70vh;
  overflow-y: auto;
}

.setting-item {
  margin-bottom: var(--spacing-xl);
}

.setting-item:last-child {
  margin-bottom: 0;
}

.setting-item label {
  display: block;
  margin-bottom: var(--spacing-sm);
  font-weight: 500;
  font-size: var(--font-size-md);
  color: var(--text-color-1);
}

.font-size-controls {
  display: flex;
  align-items: center;
  gap: var(--spacing-md);
}

.font-size-controls button {
  width: 40px;
  height: 40px;
  border: 1px solid var(--border-color);
  border-radius: var(--radius-sm);
  background: var(--bg-color-2);
  cursor: pointer;
  font-size: var(--font-size-md);
  font-weight: 600;
  color: var(--text-color-1);
  transition: all var(--transition-fast);
}

.font-size-controls button:hover {
  background: var(--primary-color);
  color: white;
  border-color: var(--primary-color);
}

.font-size-controls button:active {
  transform: scale(0.95);
}

.font-size-value {
  font-size: var(--font-size-xl);
  font-weight: 600;
  min-width: 50px;
  text-align: center;
  color: var(--text-color-1);
}

.theme-buttons {
  display: flex;
  gap: var(--spacing-sm);
}

.theme-buttons button {
  flex: 1;
  padding: var(--spacing-sm);
  border: 2px solid var(--border-color);
  border-radius: var(--radius-sm);
  background: var(--bg-color-2);
  cursor: pointer;
  font-size: var(--font-size-base);
  transition: all var(--transition-fast);
  color: var(--text-color-1);
}

.theme-buttons button.active {
  border-color: var(--primary-color);
  background: var(--primary-color);
  color: white;
}

.theme-buttons button:hover {
  border-color: var(--primary-hover);
}

.theme-buttons button:active {
  transform: scale(0.98);
}

.color-options {
  display: flex;
  gap: var(--spacing-sm);
  flex-wrap: wrap;
}

.color-btn {
  width: 36px;
  height: 36px;
  min-width: 36px;
  max-width: 36px;
  border-radius: 50%;
  border: 3px solid transparent;
  cursor: pointer;
  transition: all var(--transition-fast);
  flex-shrink: 0;
  padding: 0;
  box-sizing: border-box;
  display: block;
}

.color-btn:hover {
  transform: scale(1.1);
}

.color-btn.active {
  border-color: var(--text-color-1);
  box-shadow: 0 0 0 2px var(--card-bg), 0 0 0 4px var(--primary-color);
}

.divider {
  height: 1px;
  background: var(--divider-color);
  margin: var(--spacing-xl) 0;
}

.version-display {
  font-size: var(--font-size-md);
  color: var(--text-color-2);
  margin-bottom: var(--spacing-sm);
}

.version-display strong {
  color: var(--text-color-1);
  font-weight: 600;
}

.action-btn {
  width: 100%;
  padding: var(--spacing-sm);
  background: var(--bg-color-2);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-sm);
  cursor: pointer;
  font-size: var(--font-size-md);
  color: var(--text-color-1);
  transition: all var(--transition-fast);
  display: flex;
  align-items: center;
  justify-content: center;
  gap: var(--spacing-sm);
}

.action-btn:hover:not(:disabled) {
  background: var(--primary-color);
  color: white;
  border-color: var(--primary-color);
}

.action-btn:active:not(:disabled) {
  transform: scale(0.98);
}

.action-btn:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.checking-spinner {
  width: 14px;
  height: 14px;
  border: 2px solid var(--border-color);
  border-top-color: var(--primary-color);
  border-radius: 50%;
  animation: settings-spin 0.6s linear infinite;
  display: inline-block;
}

@keyframes settings-spin {
  to { transform: rotate(360deg); }
}

.check-result {
  margin-top: var(--spacing-sm);
  padding: var(--spacing-xs) var(--spacing-sm);
  border-radius: var(--radius-2xs);
  font-size: var(--font-size-sm);
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--spacing-sm);
}

.check-result.success {
  color: var(--success-color);
  background: color-mix(in srgb, var(--success-color) 10%, transparent);
}

.check-result.error {
  color: var(--danger-color);
  background: color-mix(in srgb, var(--danger-color) 10%, transparent);
}

.check-result.info {
  color: var(--text-color-2);
  background: var(--bg-color-3);
}

.result-update-btn {
  color: var(--primary-color);
  cursor: pointer;
  font-weight: 500;
  white-space: nowrap;
  text-decoration: underline;
  text-underline-offset: 2px;
}

.result-update-btn:hover {
  color: var(--primary-hover);
}

.notification-btn {
  width: 100%;
  padding: var(--spacing-sm);
  background: var(--bg-color-2);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-sm);
  cursor: pointer;
  font-size: var(--font-size-md);
  color: var(--text-color-1);
  transition: all var(--transition-fast);
}

.notification-btn:hover {
  background: var(--primary-color);
  color: white;
  border-color: var(--primary-color);
}

.notification-btn:active {
  transform: scale(0.98);
}

@media (max-width: 768px) {
  .settings-panel {
    max-width: 100%;
    border-radius: var(--radius-md) var(--radius-md) 0 0;
  }

  .settings-body {
    padding: var(--spacing-lg);
  }

  .theme-buttons {
    flex-direction: row;
    flex-wrap: wrap;
  }

  .theme-buttons button {
    flex: 1;
    min-width: 80px;
    padding: var(--spacing-xs) var(--spacing-sm);
    font-size: var(--font-size-sm);
  }

  .color-options {
    justify-content: space-between;
  }

  .color-btn {
    width: 32px;
    height: 32px;
    min-width: 32px;
    max-width: 32px;
  }
}

@media (max-width: 480px) {
  .settings-panel {
    max-width: 100%;
    border-radius: 0;
  }

  .settings-header {
    padding: var(--spacing-sm) var(--spacing-md);
  }

  .settings-header h3 {
    font-size: var(--font-size-lg);
  }

  .close-btn {
    font-size: var(--font-size-2xl);
    width: 18px;
    padding: 0;
    margin-left: auto;
  }

  .settings-body {
    padding: var(--spacing-md);
  }

  .setting-item {
    margin-bottom: var(--spacing-lg);
  }

  .setting-item label {
    font-size: var(--font-size-base);
  }

  .font-size-controls {
    gap: var(--spacing-sm);
  }

  .font-size-controls button {
    width: 36px;
    height: 36px;
    font-size: var(--font-size-base);
  }

  .font-size-value {
    font-size: var(--font-size-lg);
  }

  .theme-buttons {
    gap: var(--spacing-xs);
  }

  .theme-buttons button {
    padding: var(--spacing-xs);
    font-size: var(--font-size-sm);
  }

  .color-options {
    gap: var(--spacing-xs);
  }

  .color-btn {
    width: 30px;
    height: 30px;
    min-width: 30px;
    max-width: 30px;
  }
}
</style>
