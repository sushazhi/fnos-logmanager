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
        <label>安全设置</label>
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
        <label>应用日志</label>
        <button class="audit-btn" @click="$emit('showAudit')">
          查看应用日志
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
const primaryColor = ref('#5b9bd5')

const showPasswordForm = ref(false)
const currentPassword = ref('')
const newPassword = ref('')
const confirmPassword = ref('')
const passwordError = ref('')
const passwordSuccess = ref('')

const colors = [
  { name: '紫色', value: '#9b59b6', gradient: 'linear-gradient(135deg, #9b59b6 0%, #8e44ad 100%)' },
  { name: '蓝色', value: '#3498db', gradient: 'linear-gradient(135deg, #3498db 0%, #2980b9 100%)' },
  { name: '青色', value: '#1abc9c', gradient: 'linear-gradient(135deg, #1abc9c 0%, #16a085 100%)' },
  { name: '粉色', value: '#e74c8c', gradient: 'linear-gradient(135deg, #e74c8c 0%, #c0392b 100%)' },
  { name: '莫兰迪渐变', value: '#8ec5fc', gradient: 'linear-gradient(135deg, #e0c3fc 0%, #8ec5fc 33%, #a8edea 66%, #fed6e3 100%)' },
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

  // 动态计算渐变色
  const color = primaryColor.value
  const darkerColor = adjustColor(color, -20)
  root.style.setProperty('--primary-gradient', `linear-gradient(135deg, ${color} 0%, ${darkerColor} 100%)`)

  // 计算悬停和按下状态的颜色
  root.style.setProperty('--primary-hover', adjustColor(color, -15))
  root.style.setProperty('--primary-pressed', adjustColor(color, -30))
  
  // 生成基于主题色的色系
  const hsl = hexToHSL(color)
  const hue = hsl.h
  
  // 为统计卡片生成色系
  const card1Color = hslToHex((hue + 0) % 360, Math.min(hsl.s * 0.6, 60), Math.max(hsl.l, 50))
  const card2Color = hslToHex((hue + 60) % 360, Math.min(hsl.s * 0.6, 60), Math.max(hsl.l, 50))
  const card3Color = hslToHex((hue + 120) % 360, Math.min(hsl.s * 0.6, 60), Math.max(hsl.l, 50))
  const card4Color = hslToHex((hue + 180) % 360, Math.min(hsl.s * 0.6, 60), Math.max(hsl.l, 50))
  
  root.style.setProperty('--card-color-1', card1Color)
  root.style.setProperty('--card-color-1-light', hslToHex((hue + 0) % 360, Math.min(hsl.s * 0.5, 50), Math.max(hsl.l + 10, 60)))
  root.style.setProperty('--card-color-2', card2Color)
  root.style.setProperty('--card-color-2-light', hslToHex((hue + 60) % 360, Math.min(hsl.s * 0.5, 50), Math.max(hsl.l + 10, 60)))
  root.style.setProperty('--card-color-3', card3Color)
  root.style.setProperty('--card-color-3-light', hslToHex((hue + 120) % 360, Math.min(hsl.s * 0.5, 50), Math.max(hsl.l + 10, 60)))
  root.style.setProperty('--card-color-4', card4Color)
  root.style.setProperty('--card-color-4-light', hslToHex((hue + 180) % 360, Math.min(hsl.s * 0.5, 50), Math.max(hsl.l + 10, 60)))
}

// HEX转HSL
function hexToHSL(hex) {
  let color = hex.replace('#', '')
  let r = parseInt(color.substring(0, 2), 16) / 255
  let g = parseInt(color.substring(2, 4), 16) / 255
  let b = parseInt(color.substring(4, 6), 16) / 255

  const max = Math.max(r, g, b)
  const min = Math.min(r, g, b)
  let h, s, l = (max + min) / 2

  if (max === min) {
    h = s = 0
  } else {
    const d = max - min
    s = l > 0.5 ? d / (2 - max - min) : d / (max + min)
    switch (max) {
      case r: h = ((g - b) / d + (g < b ? 6 : 0)) / 6; break
      case g: h = ((b - r) / d + 2) / 6; break
      case b: h = ((r - g) / d + 4) / 6; break
    }
  }

  return {
    h: Math.round(h * 360),
    s: Math.round(s * 100),
    l: Math.round(l * 100)
  }
}

// HSL转HEX
function hslToHex(h, s, l) {
  s /= 100
  l /= 100

  const c = (1 - Math.abs(2 * l - 1)) * s
  const x = c * (1 - Math.abs((h / 60) % 2 - 1))
  const m = l - c / 2
  let r = 0, g = 0, b = 0

  if (0 <= h && h < 60) {
    r = c; g = x; b = 0
  } else if (60 <= h && h < 120) {
    r = x; g = c; b = 0
  } else if (120 <= h && h < 180) {
    r = 0; g = c; b = x
  } else if (180 <= h && h < 240) {
    r = 0; g = x; b = c
  } else if (240 <= h && h < 300) {
    r = x; g = 0; b = c
  } else if (300 <= h && h < 360) {
    r = c; g = 0; b = x
  }

  r = Math.round((r + m) * 255)
  g = Math.round((g + m) * 255)
  b = Math.round((b + m) * 255)

  return `#${r.toString(16).padStart(2, '0')}${g.toString(16).padStart(2, '0')}${b.toString(16).padStart(2, '0')}`
}

// 颜色调整函数
function adjustColor(hex, amount) {
  let color = hex.replace('#', '')
  let r = parseInt(color.substring(0, 2), 16)
  let g = parseInt(color.substring(2, 4), 16)
  let b = parseInt(color.substring(4, 6), 16)

  r = Math.min(255, Math.max(0, r + amount))
  g = Math.min(255, Math.max(0, g + amount))
  b = Math.min(255, Math.max(0, b + amount))

  return `#${r.toString(16).padStart(2, '0')}${g.toString(16).padStart(2, '0')}${b.toString(16).padStart(2, '0')}`
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
      primaryColor.value = settings.primaryColor || '#5b9bd5'
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
  font-size: 1rem;
  font-weight: 500;
  color: var(--text-color-1);
  flex: 1;
  white-space: nowrap;
}

.close-btn {
  background: none;
  border: none;
  font-size: 1.5rem;
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
  font-size: 0.875rem;
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
  font-size: 0.875rem;
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
  font-size: 1rem;
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
  font-size: 0.8125rem;
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

.password-toggle {
  width: 100%;
  padding: var(--spacing-sm);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-sm);
  background: var(--bg-color-2);
  cursor: pointer;
  font-size: 0.8125rem;
  color: var(--text-color-1);
  text-align: center;
  transition: all var(--transition-fast);
}

.password-toggle:hover {
  background: var(--bg-color-3);
}

.password-toggle:active {
  transform: scale(0.98);
}

.password-form {
  padding: var(--spacing-md);
  background: var(--bg-color-2);
  border-radius: var(--radius-sm);
  margin-top: var(--spacing-sm);
}

.password-form input {
  width: 100%;
  padding: var(--spacing-sm) var(--spacing-md);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-xs);
  font-size: 0.875rem;
  margin-bottom: var(--spacing-sm);
  background: var(--card-bg);
  color: var(--text-color-1);
  box-sizing: border-box;
  font-family: var(--font-family);
  transition: border-color var(--transition-fast);
}

.password-form input:focus {
  outline: none;
  border-color: var(--primary-color);
}

.password-form input::placeholder {
  color: var(--text-color-3);
}

.password-form .error {
  color: var(--error-color);
  font-size: 0.8125rem;
  margin-bottom: var(--spacing-sm);
}

.password-form .success {
  color: var(--success-color);
  font-size: 0.8125rem;
  margin-bottom: var(--spacing-sm);
}

.password-form .btn-row {
  display: flex;
  gap: var(--spacing-sm);
}

.password-form button {
  flex: 1;
  padding: var(--spacing-sm);
  border: none;
  border-radius: var(--radius-xs);
  cursor: pointer;
  font-size: 0.8125rem;
  transition: all var(--transition-fast);
}

.password-form .cancel-btn {
  background: var(--bg-color-2);
  color: var(--text-color-1);
}

.password-form .cancel-btn:hover {
  background: var(--bg-color-3);
}

.password-form .submit-btn {
  background: var(--primary-color);
  color: white;
}

.password-form .submit-btn:hover {
  background: var(--primary-hover);
}

.password-form button:active {
  transform: scale(0.98);
}

.audit-btn {
  width: 100%;
  padding: var(--spacing-sm);
  background: var(--bg-color-2);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-sm);
  cursor: pointer;
  font-size: 0.875rem;
  color: var(--text-color-1);
  transition: all var(--transition-fast);
}

.audit-btn:hover {
  background: var(--primary-color);
  color: white;
  border-color: var(--primary-color);
}

.audit-btn:active {
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
    font-size: 0.75rem;
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
    font-size: 0.9375rem;
  }

  .close-btn {
    font-size: 1.125rem;
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
    font-size: 0.8125rem;
  }

  .font-size-controls {
    gap: var(--spacing-sm);
  }

  .font-size-controls button {
    width: 36px;
    height: 36px;
    font-size: 0.8125rem;
  }

  .font-size-value {
    font-size: 0.9375rem;
  }

  .theme-buttons {
    gap: var(--spacing-xs);
  }

  .theme-buttons button {
    padding: var(--spacing-xs);
    font-size: 0.75rem;
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

  .password-form {
    padding: var(--spacing-sm);
  }

  .password-form input {
    padding: var(--spacing-xs) var(--spacing-sm);
    font-size: 0.8125rem;
  }

  .password-form .btn-row {
    gap: var(--spacing-xs);
  }

  .password-form button {
    padding: var(--spacing-xs);
    font-size: 0.75rem;
  }
}
</style>
