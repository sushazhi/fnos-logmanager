<template>
  <div class="login-overlay">
    <div class="login-background">
      <div class="bg-shape bg-shape-1"></div>
      <div class="bg-shape bg-shape-2"></div>
      <div class="bg-shape bg-shape-3"></div>
    </div>

    <div class="login-modal">
      <div class="login-header">
        <div class="login-icon">
          <svg width="64" height="64" viewBox="0 0 64 64" fill="none">
            <circle cx="32" cy="32" r="28" fill="white" fill-opacity="0.15"/>
            <circle cx="32" cy="32" r="20" fill="white" fill-opacity="0.2"/>
            <path d="M32 38C35.3137 38 38 35.3137 38 32C38 28.6863 35.3137 26 32 26C28.6863 26 26 28.6863 26 32C26 35.3137 28.6863 38 32 38Z" fill="white"/>
            <path d="M32 22C33.1046 22 34 21.1046 34 20C34 18.8954 33.1046 18 32 18C30.8954 18 30 18.8954 30 20C30 21.1046 30.8954 22 32 22Z" fill="white" fill-opacity="0.8"/>
            <path d="M32 46C33.1046 46 34 45.1046 34 44C34 42.8954 33.1046 42 32 42C30.8954 42 30 42.8954 30 44C30 45.1046 30.8954 46 32 46Z" fill="white" fill-opacity="0.8"/>
          </svg>
        </div>
        <h2>飞牛应用日志管理</h2>
        <p class="login-subtitle">请输入密码以继续</p>
      </div>

      <form @submit.prevent="handleLogin">
        <div class="form-group">
          <label>访问密码</label>
          <div class="password-input">
            <div class="input-icon">
              <!-- 鸿蒙6图标: 锁 -->
              <svg width="20" height="20" viewBox="0 0 24 24" fill="none">
                <rect x="5" y="11" width="14" height="10" rx="2" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/>
                <path d="M8 11V7C8 4.79086 9.79086 3 12 3C14.2091 3 16 4.79086 16 7V11" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/>
                <circle cx="12" cy="16" r="1.5" fill="currentColor"/>
              </svg>
            </div>
            <input
              :type="showPassword ? 'text' : 'password'"
              v-model="password"
              placeholder="请输入访问密码"
              :disabled="loading"
              autocomplete="current-password"
              autocapitalize="off"
              autocorrect="off"
              spellcheck="false"
              inputmode="text"
              ref="passwordInput"
            >
            <div class="password-toggle-wrapper">
              <button type="button" class="toggle-password-btn" @click="showPassword = !showPassword" tabindex="-1">
                <!-- 鸿蒙6图标: 显示密码(眼睛) -->
                <svg v-if="!showPassword" width="20" height="20" viewBox="0 0 24 24" fill="none">
                  <path d="M12 4.5C7 4.5 2.73 7.61 1 12C2.73 16.39 7 19.5 12 19.5C17 19.5 21.27 16.39 23 12C21.27 7.61 17 4.5 12 4.5Z" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/>
                  <path d="M12 15.5C13.933 15.5 15.5 13.933 15.5 12C15.5 10.067 13.933 8.5 12 8.5C10.067 8.5 8.5 10.067 8.5 12C8.5 13.933 10.067 15.5 12 15.5Z" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/>
                </svg>
                <!-- 鸿蒙6图标: 隐藏密码(眼睛关闭) -->
                <svg v-else width="20" height="20" viewBox="0 0 24 24" fill="none">
                  <path d="M3 3L21 21" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/>
                  <path d="M10.5 10.5C10.181 10.819 9.983 11.247 9.983 11.72C9.983 12.669 10.753 13.439 11.702 13.439C12.175 13.439 12.603 13.241 12.922 12.922" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/>
                  <path d="M6.17 6.17C4.27 7.49 2.78 9.46 2 12C3.73 16.39 8 19.5 13 19.5C14.55 19.5 16.03 19.17 17.37 18.58" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/>
                  <path d="M19.42 15.54C20.47 14.53 21.33 13.35 22 12C20.27 7.61 16 4.5 11 4.5C10.29 4.5 9.6 4.57 8.93 4.7" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/>
                </svg>
              </button>
            </div>
          </div>
        </div>

        <div class="error" v-if="error">
          <!-- 鸿蒙6图标: 错误提示 -->
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none">
            <circle cx="12" cy="12" r="10" stroke="currentColor" stroke-width="1.5"/>
            <path d="M12 8V12" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/>
            <circle cx="12" cy="16" r="1" fill="currentColor"/>
          </svg>
          {{ error }}
        </div>

        <div class="hint" v-if="remaining !== null && remaining > 0">
          <!-- 鸿蒙6图标: 警告提示 -->
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none">
            <circle cx="12" cy="12" r="10" stroke="currentColor" stroke-width="1.5"/>
            <path d="M12 8V12" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/>
            <circle cx="12" cy="16" r="1" fill="currentColor"/>
          </svg>
          剩余尝试次数: {{ remaining }}
        </div>

        <div class="remember-me">
          <label class="checkbox-label">
            <input type="checkbox" v-model="rememberMe" :disabled="loading">
            <span class="checkbox-custom"></span>
            <span class="checkbox-text">记住密码</span>
          </label>
        </div>

        <button type="submit" :disabled="loading || !password" class="login-btn">
          <span v-if="!loading">登录</span>
          <span v-else class="loading-text">
            <span class="spinner"></span>
            登录中...
          </span>
        </button>
      </form>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import api from '../services/api'

const emit = defineEmits(['login'])

const password = ref('')
const error = ref('')
const loading = ref(false)
const remaining = ref(null)
const showPassword = ref(false)
const passwordInput = ref(null)
const rememberMe = ref(false)

// 存储键名
const SESSION_KEY = 'logmanager_session_password'
const PERSIST_KEY = 'logmanager_remembered_password'

// 简单的编码/解码函数（非加密，仅混淆）
// 注意：这不是真正的加密，只是增加一层混淆，防止明文存储
function encodePassword(pwd) {
  try {
    // 使用 base64 编码 + 简单的字符替换
    const encoded = btoa(encodeURIComponent(pwd))
    // 字符替换增加混淆
    return encoded.split('').map(c => String.fromCharCode(c.charCodeAt(0) + 1)).join('')
  } catch {
    return ''
  }
}

function decodePassword(encoded) {
  try {
    // 反向字符替换
    const decoded = encoded.split('').map(c => String.fromCharCode(c.charCodeAt(0) - 1)).join('')
    return decodeURIComponent(atob(decoded))
  } catch {
    return ''
  }
}

// 保存密码
function savePassword(pwd) {
  const encoded = encodePassword(pwd)
  sessionStorage.setItem(SESSION_KEY, encoded)
  localStorage.setItem(PERSIST_KEY, encoded)
}

// 清除保存的密码
function clearSavedPassword() {
  sessionStorage.removeItem(SESSION_KEY)
  localStorage.removeItem(PERSIST_KEY)
}

// 加载保存的密码
function loadSavedPassword() {
  // 优先从 sessionStorage 读取（当前会话）
  const sessionPassword = sessionStorage.getItem(SESSION_KEY)
  if (sessionPassword) {
    const decoded = decodePassword(sessionPassword)
    if (decoded) {
      return decoded
    }
  }

  // 如果 sessionStorage 没有，尝试从 localStorage 读取（持久存储）
  const savedPassword = localStorage.getItem(PERSIST_KEY)
  if (savedPassword) {
    const decoded = decodePassword(savedPassword)
    if (decoded) {
      return decoded
    }
  }

  return ''
}

async function handleLogin() {
  if (!password.value) return

  loading.value = true
  error.value = ''

  try {
    const data = await api.post('/api/auth/login', {
      password: password.value
    })
    if (data.success) {
      if (data.csrfToken) {
        api.setCSRFToken(data.csrfToken)
      }

      // 保存或清除密码
      if (rememberMe.value) {
        savePassword(password.value)
      } else {
        clearSavedPassword()
      }

      emit('login', data.csrfToken)
    }
  } catch (e) {
    const msg = e.message || '登录失败'
    error.value = msg
    remaining.value = e.remaining || null
  } finally {
    loading.value = false
  }
}

// 组件挂载时，检查是否有保存的密码
onMounted(() => {
  const savedPassword = loadSavedPassword()
  if (savedPassword) {
    password.value = savedPassword
    rememberMe.value = true
  }
  passwordInput.value?.focus()
})
</script>

<style scoped>
.login-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: var(--primary-gradient);
  display: flex;
  justify-content: center;
  align-items: center;
  z-index: 2000;
  overflow: hidden;
}

.login-background {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  overflow: hidden;
  pointer-events: none;
}

.bg-shape {
  position: absolute;
  border-radius: 50%;
  background: rgba(255, 255, 255, 0.1);
  animation: float 20s ease-in-out infinite;
}

.bg-shape-1 {
  width: 400px;
  height: 400px;
  top: -200px;
  left: -100px;
  animation-delay: 0s;
}

.bg-shape-2 {
  width: 300px;
  height: 300px;
  bottom: -150px;
  right: -50px;
  animation-delay: -5s;
}

.bg-shape-3 {
  width: 200px;
  height: 200px;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
  animation-delay: -10s;
}

@keyframes float {
  0%, 100% {
    transform: translate(0, 0) scale(1);
  }
  33% {
    transform: translate(30px, -30px) scale(1.1);
  }
  66% {
    transform: translate(-20px, 20px) scale(0.9);
  }
}

.login-modal {
  position: relative;
  background: var(--card-bg);
  padding: var(--spacing-3xl);
  border-radius: var(--radius-xl);
  box-shadow: 0 20px 60px rgba(0, 0, 0, 0.3);
  width: 100%;
  max-width: 400px;
  animation: slideUp 0.5s cubic-bezier(0.4, 0, 0.2, 1);
  backdrop-filter: blur(10px);
}

@keyframes slideUp {
  from {
    opacity: 0;
    transform: translateY(30px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

.login-header {
  display: flex;
  flex-direction: column;
  align-items: center;
  margin-bottom: var(--spacing-2xl);
}

.login-icon {
  margin-bottom: var(--spacing-lg);
  animation: pulse 2s ease-in-out infinite;
}

@keyframes pulse {
  0%, 100% {
    transform: scale(1);
  }
  50% {
    transform: scale(1.05);
  }
}

.login-icon svg {
  display: block;
  filter: drop-shadow(0 4px 12px rgba(0, 0, 0, 0.15));
}

.login-modal h2 {
  margin: 0 0 var(--spacing-xs) 0;
  text-align: center;
  color: var(--text-color-1);
  font-size: 1.5rem;
  font-weight: 600;
  letter-spacing: -0.02em;
}

.login-subtitle {
  margin: 0;
  text-align: center;
  color: var(--text-color-3);
  font-size: 0.875rem;
  font-weight: 400;
}

.form-group {
  margin-bottom: var(--spacing-xl);
}

.form-group label {
  display: block;
  margin-bottom: var(--spacing-sm);
  font-weight: 500;
  font-size: 0.875rem;
  color: var(--text-color-1);
}

.password-input {
  display: flex;
  align-items: stretch;
  position: relative;
}

.input-icon {
  position: absolute;
  left: var(--spacing-md);
  top: 50%;
  transform: translateY(-50%);
  color: var(--text-color-3);
  pointer-events: none;
  z-index: 1;
}

.password-input input {
  flex: 1;
  min-width: 0;
  padding: var(--spacing-md) var(--spacing-lg);
  padding-left: 48px;
  padding-right: 12px;
  border: 2px solid var(--border-color);
  border-radius: var(--radius-sm);
  font-size: 1rem;
  font-family: var(--font-family);
  transition: all var(--transition-fast);
  pointer-events: auto;
  -webkit-user-select: text;
  user-select: text;
  touch-action: manipulation;
  background: var(--card-bg);
  color: var(--text-color-1);
}

.password-input input:focus {
  outline: none;
  border-color: var(--primary-color);
  box-shadow: 0 0 0 3px rgba(0, 125, 255, 0.1);
}

.password-input input::placeholder {
  color: var(--text-color-3);
}

.password-toggle-wrapper {
  position: absolute;
  right: 0;
  top: 0;
  bottom: 0;
  width: 44px;
  display: flex;
  align-items: center;
  justify-content: center;
  pointer-events: none;
}

.toggle-password-btn {
  width: 44px;
  height: 100%;
  padding: 0;
  background: transparent;
  border: none;
  border-radius: var(--radius-xs);
  font-size: 0.8rem;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: all var(--transition-fast);
  color: var(--text-color-2);
  pointer-events: auto;
}

.toggle-password-btn:hover {
  color: var(--primary-color);
  background: var(--bg-color-2);
}

.toggle-password-btn:active {
  background: var(--bg-color-3);
}

.error {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
  color: var(--error-color);
  font-size: 0.875rem;
  margin-bottom: var(--spacing-lg);
  padding: var(--spacing-sm) var(--spacing-md);
  background: rgba(250, 42, 45, 0.1);
  border-radius: var(--radius-sm);
  animation: shake 0.5s cubic-bezier(0.4, 0, 0.2, 1);
}

@keyframes shake {
  0%, 100% {
    transform: translateX(0);
  }
  25% {
    transform: translateX(-5px);
  }
  75% {
    transform: translateX(5px);
  }
}

.hint {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
  color: var(--warning-color);
  font-size: 0.8125rem;
  margin-bottom: var(--spacing-lg);
  padding: var(--spacing-sm) var(--spacing-md);
  background: rgba(255, 176, 0, 0.1);
  border-radius: var(--radius-sm);
}

.remember-me {
  margin-bottom: var(--spacing-lg);
}

.checkbox-label {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
  cursor: pointer;
  user-select: none;
  font-size: 0.875rem;
  color: var(--text-color-2);
}

.checkbox-label input[type="checkbox"] {
  position: absolute;
  opacity: 0;
  width: 0;
  height: 0;
}

.checkbox-custom {
  position: relative;
  width: 18px;
  height: 18px;
  border: 2px solid var(--border-color);
  border-radius: var(--radius-xs);
  background: var(--card-bg);
  transition: all var(--transition-fast);
  flex-shrink: 0;
}

.checkbox-label input[type="checkbox"]:checked + .checkbox-custom {
  background: var(--primary-color);
  border-color: var(--primary-color);
}

.checkbox-label input[type="checkbox"]:checked + .checkbox-custom::after {
  content: '';
  position: absolute;
  left: 5px;
  top: 2px;
  width: 4px;
  height: 8px;
  border: solid white;
  border-width: 0 2px 2px 0;
  transform: rotate(45deg);
}

.checkbox-label input[type="checkbox"]:disabled + .checkbox-custom {
  opacity: 0.5;
  cursor: not-allowed;
}

.checkbox-label:hover .checkbox-custom {
  border-color: var(--primary-color);
}

.checkbox-text {
  line-height: 1.4;
}

.login-btn {
  width: 100%;
  padding: var(--spacing-lg);
  background: var(--primary-gradient);
  color: white;
  border: none;
  border-radius: var(--radius-sm);
  font-size: 1rem;
  font-weight: 500;
  cursor: pointer;
  transition: all var(--transition-fast);
  box-shadow: 0 4px 12px rgba(0, 125, 255, 0.3);
  position: relative;
  overflow: hidden;
}

.login-btn::before {
  content: '';
  position: absolute;
  top: 0;
  left: -100%;
  width: 100%;
  height: 100%;
  background: linear-gradient(90deg, transparent, rgba(255, 255, 255, 0.2), transparent);
  transition: left 0.5s;
}

.login-btn:hover:not(:disabled)::before {
  left: 100%;
}

.login-btn:hover:not(:disabled) {
  transform: translateY(-2px);
  box-shadow: 0 6px 20px rgba(0, 125, 255, 0.4);
}

.login-btn:active:not(:disabled) {
  transform: scale(0.98);
}

.login-btn:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.loading-text {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: var(--spacing-sm);
}

.spinner {
  width: 16px;
  height: 16px;
  border: 2px solid rgba(255, 255, 255, 0.3);
  border-top-color: white;
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
}

@keyframes spin {
  to {
    transform: rotate(360deg);
  }
}

@media (max-width: 480px) {
  .login-modal {
    margin: var(--spacing-xl);
    padding: var(--spacing-2xl) var(--spacing-xl);
    border-radius: var(--radius-lg);
  }

  .login-icon svg {
    width: 56px;
    height: 56px;
  }

  .login-modal h2 {
    font-size: 1.25rem;
  }

  .login-subtitle {
    font-size: 0.8125rem;
  }

  .password-input input {
    font-size: 16px;
  }
}

@media (max-width: 360px) {
  .login-modal {
    margin: 0;
    padding: var(--spacing-xl) var(--spacing-lg);
    border-radius: 0;
    max-width: 100%;
  }

  .login-modal h2 {
    font-size: 1.125rem;
  }

  .login-icon svg {
    width: 48px;
    height: 48px;
  }
}
</style>
