<template>
  <div class="login-overlay">
    <div class="login-modal">
      <div class="login-header">
        <div class="login-icon">
          <svg width="48" height="48" viewBox="0 0 48 48" fill="none">
            <path d="M24 4C12.95 4 4 12.95 4 24C4 35.05 12.95 44 24 44C35.05 44 44 35.05 44 24C44 12.95 35.05 4 24 4Z" fill="white" fill-opacity="0.2"/>
            <path d="M24 28C26.2091 28 28 26.2091 28 24C28 21.7909 26.2091 20 24 20C21.7909 20 20 21.7909 20 24C20 26.2091 21.7909 28 24 28Z" fill="white"/>
            <path d="M24 16C25.1046 16 26 15.1046 26 14C26 12.8954 25.1046 12 24 12C22.8954 12 22 12.8954 22 14C22 15.1046 22.8954 16 24 16Z" fill="white"/>
            <path d="M24 36C25.1046 36 26 35.1046 26 34C26 32.8954 25.1046 32 24 32C22.8954 32 22 32.8954 22 34C22 35.1046 22.8954 36 24 36Z" fill="white"/>
          </svg>
        </div>
        <h2>登录</h2>
      </div>
      <form @submit.prevent="handleLogin">
        <div class="form-group">
          <label>密码</label>
          <div class="password-input">
            <input 
              :type="showPassword ? 'text' : 'password'" 
              v-model="password" 
              placeholder="请输入密码"
              :disabled="loading"
              autocomplete="current-password"
              autocapitalize="off"
              autocorrect="off"
              spellcheck="false"
              inputmode="text"
              ref="passwordInput"
            >
            <button type="button" class="toggle-password-btn" @click="showPassword = !showPassword">
              <svg v-if="!showPassword" width="20" height="20" viewBox="0 0 20 20" fill="none">
                <path d="M12.9833 10C12.9833 11.6569 11.6402 13 9.9833 13C8.32645 13 6.9833 11.6569 6.9833 10C6.9833 8.34315 8.32645 7 9.9833 7C11.6402 7 12.9833 8.34315 12.9833 10Z" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/>
                <path d="M9.9833 15C7.23273 15 4.72309 13.9059 2.98328 12.1716C1.24888 10.4321 0.149902 7.92211 0.149902 5.17146C0.149902 2.4208 1.24888 -0.0891528 2.98328 -1.82863C4.72309 -3.56295 7.23273 -4.65698 9.9833 -4.65698C12.7339 -4.65698 15.2435 -3.56295 16.9833 -1.82863C18.7177 -0.0891528 19.8167 2.4208 19.8167 5.17146C19.8167 7.92211 18.7177 10.4321 16.9833 12.1716C15.2435 13.9059 12.7339 15 9.9833 15Z" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/>
              </svg>
              <svg v-else width="20" height="20" viewBox="0 0 20 20" fill="none">
                <path d="M2.91667 9.99996C2.91667 6.09538 6.09538 2.91663 10 2.91663C13.9046 2.91663 17.0833 6.09538 17.0833 9.99996C17.0833 13.9045 13.9046 17.0833 10 17.0833C6.09538 17.0833 2.91667 13.9045 2.91667 9.99996Z" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/>
                <path d="M10 6.66663V8.33329" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/>
                <path d="M10 11.6666V13.3333" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/>
              </svg>
            </button>
          </div>
        </div>
        <div class="error" v-if="error">{{ error }}</div>
        <div class="hint" v-if="remaining !== null && remaining > 0">
          剩余尝试次数: {{ remaining }}
        </div>
        <button type="submit" :disabled="loading || !password">
          {{ loading ? '登录中...' : '登录' }}
        </button>
      </form>
    </div>
  </div>
</template>

<script setup>
import { ref } from 'vue'
import api from '../services/api'

const emit = defineEmits(['login'])

const password = ref('')
const error = ref('')
const loading = ref(false)
const remaining = ref(null)
const showPassword = ref(false)
const passwordInput = ref(null)

async function handleLogin() {
  if (!password.value) return
  
  loading.value = true
  error.value = ''
  
  try {
    const data = await api.post('/api/auth/login', { password: password.value })
    if (data.success) {
      if (data.csrfToken) {
        api.setCSRFToken(data.csrfToken)
      }
      // 移除localStorage token存储，仅依赖httpOnly cookie
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
}

.login-modal {
  background: var(--card-bg);
  padding: var(--spacing-3xl);
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow-xl);
  width: 100%;
  max-width: 360px;
}

.login-header {
  display: flex;
  flex-direction: column;
  align-items: center;
  margin-bottom: var(--spacing-2xl);
}

.login-icon {
  margin-bottom: var(--spacing-md);
}

.login-icon svg {
  display: block;
}

.login-modal h2 {
  margin: 0;
  text-align: center;
  color: var(--text-color-1);
  font-size: 1.5rem;
  font-weight: 600;
  letter-spacing: -0.02em;
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
}

.password-input input {
  flex: 4;
  min-width: 0;
  padding: var(--spacing-md) var(--spacing-lg);
  border: 2px solid var(--border-color);
  border-radius: var(--radius-sm) 0 0 var(--radius-sm);
  font-size: 1rem;
  font-family: var(--font-family);
  transition: border-color var(--transition-fast);
  pointer-events: auto;
  -webkit-user-select: text;
  user-select: text;
  touch-action: manipulation;
}

.password-input input:focus {
  outline: none;
  border-color: var(--primary-color);
}

.password-input input::placeholder {
  color: var(--text-color-3);
}

.toggle-password-btn {
  flex: 1;
  min-width: 0;
  padding: 0;
  background: var(--card-bg);
  border: 2px solid var(--border-color);
  border-left: none;
  border-radius: 0 var(--radius-sm) var(--radius-sm) 0;
  font-size: 0.8rem;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: all var(--transition-fast);
  color: var(--text-color-2);
}

.toggle-password-btn:hover {
  color: var(--primary-color);
  background: var(--bg-color-2);
}

.toggle-password-btn:active {
  background: var(--bg-color-3);
}

.error {
  color: var(--error-color);
  font-size: 0.875rem;
  margin-bottom: var(--spacing-lg);
  text-align: center;
  font-weight: 400;
}

.hint {
  color: var(--warning-color);
  font-size: 0.8125rem;
  margin-bottom: var(--spacing-lg);
  text-align: center;
  font-weight: 400;
}

button[type="submit"] {
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
  box-shadow: var(--shadow-sm);
}

button[type="submit"]:hover:not(:disabled) {
  transform: translateY(-2px);
  box-shadow: var(--shadow-md);
}

button[type="submit"]:active:not(:disabled) {
  transform: scale(0.98);
}

button[type="submit"]:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

@media (max-width: 480px) {
  .login-modal {
    margin: var(--spacing-xl);
    padding: var(--spacing-2xl) var(--spacing-xl);
    border-radius: var(--radius-md);
  }

  .login-icon svg {
    width: 40px;
    height: 40px;
  }

  .login-modal h2 {
    font-size: 1.25rem;
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
}
</style>
