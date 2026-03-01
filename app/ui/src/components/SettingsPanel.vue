<template>
  <div class="settings-panel">
    <div class="settings-header">
      <h3>⚙️ 设置</h3>
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
          >☀️ 日间</button>
          <button 
            :class="{ active: theme === 'dark' }" 
            @click="setTheme('dark')"
          >🌙 夜间</button>
          <button 
            :class="{ active: theme === 'auto' }" 
            @click="setTheme('auto')"
          >🔄 自动</button>
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
        <label>🔐 安全设置</label>
        <button class="password-toggle" @click="showPasswordForm = !showPasswordForm">
          {{ showPasswordForm ? '取消修改密码' : '修改密码' }}
        </button>
        
        <div class="password-form" v-if="showPasswordForm">
          <input 
            type="password" 
            v-model="currentPassword" 
            placeholder="当前密码"
          >
          <input 
            type="password" 
            v-model="newPassword" 
            placeholder="新密码（至少8位）"
          >
          <input 
            type="password" 
            v-model="confirmPassword" 
            placeholder="确认新密码"
          >
          <div class="error" v-if="passwordError">{{ passwordError }}</div>
          <div class="success" v-if="passwordSuccess">{{ passwordSuccess }}</div>
          <div class="btn-row">
            <button class="cancel-btn" @click="cancelPasswordChange">取消</button>
            <button class="submit-btn" @click="changePassword">确认修改</button>
          </div>
        </div>
      </div>
      
      <div class="divider"></div>
      
      <div class="setting-item">
        <label>📋 审计日志</label>
        <button class="audit-btn" @click="$emit('showAudit')">
          查看审计日志
        </button>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import api from '../services/api'

const emit = defineEmits(['close', 'update', 'showAudit'])

const fontSize = ref(16)
const theme = ref('light')
const primaryColor = ref('#667eea')

const showPasswordForm = ref(false)
const currentPassword = ref('')
const newPassword = ref('')
const confirmPassword = ref('')
const passwordError = ref('')
const passwordSuccess = ref('')

const colors = [
  { name: '紫蓝', value: '#667eea', gradient: 'linear-gradient(135deg, #667eea 0%, #764ba2 100%)' },
  { name: '青绿', value: '#00bcd4', gradient: 'linear-gradient(135deg, #00bcd4 0%, #009688 100%)' },
  { name: '橙色', value: '#ff9800', gradient: 'linear-gradient(135deg, #ff9800 0%, #f57c00 100%)' },
  { name: '粉色', value: '#e91e63', gradient: 'linear-gradient(135deg, #e91e63 0%, #c2185b 100%)' },
  { name: '蓝色', value: '#2196f3', gradient: 'linear-gradient(135deg, #2196f3 0%, #1976d2 100%)' },
  { name: '绿色', value: '#4caf50', gradient: 'linear-gradient(135deg, #4caf50 0%, #388e3c 100%)' },
]

function increaseFontSize() {
  if (fontSize.value < 24) {
    fontSize.value += 2
    applyFontSize()
    saveSettings()
  }
}

function decreaseFontSize() {
  if (fontSize.value > 12) {
    fontSize.value -= 2
    applyFontSize()
    saveSettings()
  }
}

function setTheme(newTheme) {
  theme.value = newTheme
  applyTheme()
  saveSettings()
}

function setColor(color) {
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
  const root = document.documentElement
  root.style.setProperty('--primary-color', primaryColor.value)
}

function applyFontSize() {
  const root = document.documentElement
  root.style.setProperty('--base-font-size', `${fontSize.value}px`)
}

function applyAll() {
  applyTheme()
  applyColor()
  applyFontSize()
}

function saveSettings() {
  const settings = {
    fontSize: fontSize.value,
    theme: theme.value,
    primaryColor: primaryColor.value
  }
  localStorage.setItem('logmanager_settings', JSON.stringify(settings))
  emit('update', settings)
}

function loadSettings() {
  try {
    const saved = localStorage.getItem('logmanager_settings')
    if (saved) {
      const settings = JSON.parse(saved)
      fontSize.value = settings.fontSize || 16
      theme.value = settings.theme || 'light'
      primaryColor.value = settings.primaryColor || '#667eea'
    }
  } catch (e) {
    console.warn('Failed to load settings:', e)
    localStorage.removeItem('logmanager_settings')
  }
  
  applyAll()
}

function cancelPasswordChange() {
  showPasswordForm.value = false
  currentPassword.value = ''
  newPassword.value = ''
  confirmPassword.value = ''
  passwordError.value = ''
  passwordSuccess.value = ''
}

async function changePassword() {
  passwordError.value = ''
  passwordSuccess.value = ''
  
  if (!currentPassword.value || !newPassword.value || !confirmPassword.value) {
    passwordError.value = '请填写所有字段'
    return
  }
  
  if (newPassword.value !== confirmPassword.value) {
    passwordError.value = '两次输入的新密码不一致'
    return
  }
  
  if (newPassword.value.length < 8) {
    passwordError.value = '新密码至少8位'
    return
  }
  
  try {
    await api.post('/api/auth/password', {
      currentPassword: currentPassword.value,
      newPassword: newPassword.value
    })
    passwordSuccess.value = '密码已修改'
    currentPassword.value = ''
    newPassword.value = ''
    confirmPassword.value = ''
    setTimeout(() => {
      showPasswordForm.value = false
      passwordSuccess.value = ''
    }, 2000)
  } catch (e) {
    passwordError.value = e.message || '修改失败'
  }
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
  background: var(--card-bg, white);
  border-radius: 12px;
  box-shadow: 0 4px 20px rgba(0, 0, 0, 0.15);
  max-width: 400px;
  width: 100%;
}

.settings-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 15px 20px;
  border-bottom: 1px solid var(--border-color, #e0e0e0);
  gap: 15px;
}

.settings-header h3 {
  margin: 0;
  font-size: 1rem;
  color: var(--text-color, #333);
  flex: 1;
  white-space: nowrap;
}

.close-btn {
  background: none;
  border: none;
  font-size: 1.5rem;
  cursor: pointer;
  color: var(--text-color, #666);
  padding: 0;
  line-height: 1;
  flex: none;
  flex-shrink: 0;
}

.close-btn:hover {
  color: var(--text-color, #333);
}

.settings-body {
  padding: 20px;
  max-height: 70vh;
  overflow-y: auto;
}

.setting-item {
  margin-bottom: 20px;
}

.setting-item:last-child {
  margin-bottom: 0;
}

.setting-item label {
  display: block;
  margin-bottom: 10px;
  font-weight: 500;
  color: var(--text-color, #333);
}

.font-size-controls {
  display: flex;
  align-items: center;
  gap: 15px;
}

.font-size-controls button {
  width: 40px;
  height: 40px;
  border: 1px solid var(--border-color, #ddd);
  border-radius: 8px;
  background: var(--card-bg, #f5f7fa);
  cursor: pointer;
  font-size: 0.9rem;
  font-weight: bold;
  color: var(--text-color, #333);
}

.font-size-controls button:hover {
  background: var(--primary-color, #667eea);
  color: white;
  border-color: var(--primary-color, #667eea);
}

.font-size-value {
  font-size: 1rem;
  font-weight: 600;
  min-width: 50px;
  text-align: center;
  color: var(--text-color, #333);
}

.theme-buttons {
  display: flex;
  gap: 10px;
}

.theme-buttons button {
  flex: 1;
  padding: 10px;
  border: 2px solid var(--border-color, #e0e0e0);
  border-radius: 8px;
  background: var(--card-bg, #f5f7fa);
  cursor: pointer;
  font-size: 0.85rem;
  transition: all 0.2s;
  color: var(--text-color, #333);
}

.theme-buttons button.active {
  border-color: var(--primary-color, #667eea);
  background: var(--primary-color, #667eea);
  color: white;
}

.color-options {
  display: flex;
  gap: 10px;
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
  transition: all 0.2s;
  flex-shrink: 0;
  padding: 0;
  box-sizing: border-box;
  display: block;
}

.color-btn:hover {
  transform: scale(1.1);
}

.color-btn.active {
  border-color: var(--text-color, #333);
  box-shadow: 0 0 0 2px var(--card-bg, white), 0 0 0 4px var(--primary-color, #667eea);
}

.divider {
  height: 1px;
  background: var(--border-color, #e0e0e0);
  margin: 20px 0;
}

.password-toggle {
  width: 100%;
  padding: 10px;
  border: 1px solid var(--border-color, #e0e0e0);
  border-radius: 8px;
  background: var(--card-bg, #f5f7fa);
  cursor: pointer;
  font-size: 0.85rem;
  color: var(--text-color, #333);
  text-align: center;
}

.password-toggle:hover {
  background: var(--border-color, #e8e8e8);
}

.password-form {
  padding: 15px;
  background: var(--bg-color, #f8f9fa);
  border-radius: 8px;
  margin-top: 10px;
}

.password-form input {
  width: 100%;
  padding: 10px 12px;
  border: 1px solid var(--border-color, #ddd);
  border-radius: 6px;
  font-size: 0.9rem;
  margin-bottom: 10px;
  background: var(--card-bg, white);
  color: var(--text-color, #333);
  box-sizing: border-box;
}

.password-form input:focus {
  outline: none;
  border-color: var(--primary-color, #667eea);
}

.password-form .error {
  color: #f44336;
  font-size: 0.85rem;
  margin-bottom: 10px;
}

.password-form .success {
  color: #4caf50;
  font-size: 0.85rem;
  margin-bottom: 10px;
}

.password-form .btn-row {
  display: flex;
  gap: 10px;
}

.password-form button {
  flex: 1;
  padding: 10px;
  border: none;
  border-radius: 6px;
  cursor: pointer;
  font-size: 0.85rem;
}

.password-form .cancel-btn {
  background: var(--border-color, #e0e0e0);
  color: var(--text-color, #333);
}

.password-form .submit-btn {
  background: var(--primary-color, #667eea);
  color: white;
}

.audit-btn {
  width: 100%;
  padding: 10px;
  background: var(--bg-color, #f5f5f5);
  border: 1px solid var(--border-color, #e0e0e0);
  border-radius: 8px;
  cursor: pointer;
  font-size: 0.9rem;
  color: var(--text-color, #333);
  transition: all 0.2s;
}

.audit-btn:hover {
  background: var(--primary-color, #667eea);
  color: white;
  border-color: var(--primary-color, #667eea);
}

@media (max-width: 480px) {
  .settings-panel {
    max-width: 100%;
    border-radius: 0;
  }
  
  .settings-body {
    padding: 15px;
  }
  
  .theme-buttons {
    flex-direction: row;
    flex-wrap: wrap;
  }
  
  .theme-buttons button {
    flex: 1;
    min-width: 80px;
    padding: 8px;
    font-size: 0.8rem;
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
</style>
