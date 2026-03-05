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
        <h2>设置密码</h2>
      </div>
      <p class="setup-hint">首次使用，请设置登录密码</p>
      <form @submit.prevent="handleSetup">
        <div class="form-group">
          <label>新密码</label>
          <div class="password-input">
            <input 
              :type="showPassword ? 'text' : 'password'" 
              v-model="password" 
              placeholder="请输入密码（至少8位）"
              :disabled="loading"
              autocomplete="new-password"
              ref="passwordInput"
            >
            <button type="button" class="toggle-password-btn" @click="showPassword = !showPassword">
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
        <div class="form-group">
          <label>确认密码</label>
          <div class="password-input">
            <input 
              :type="showConfirmPassword ? 'text' : 'password'" 
              v-model="confirmPassword" 
              placeholder="请再次输入密码"
              :disabled="loading"
              autocomplete="new-password"
            >
            <button type="button" class="toggle-password-btn" @click="showConfirmPassword = !showConfirmPassword">
              <!-- 鸿蒙6图标: 显示密码(眼睛) -->
              <svg v-if="!showConfirmPassword" width="20" height="20" viewBox="0 0 24 24" fill="none">
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
        <div class="error" v-if="error">{{ error }}</div>
        <button type="submit" :disabled="loading || !password || !confirmPassword">
          {{ loading ? '设置中...' : '设置密码' }}
        </button>
      </form>
    </div>
  </div>
</template>

<script setup>
import { ref } from 'vue'
import api from '../services/api'

const emit = defineEmits(['setup'])

const password = ref('')
const confirmPassword = ref('')
const error = ref('')
const loading = ref(false)
const showPassword = ref(false)
const showConfirmPassword = ref(false)
const passwordInput = ref(null)

async function handleSetup() {
  if (!password.value || !confirmPassword.value) return
  
  if (password.value.length < 8) {
    error.value = '密码至少8位'
    return
  }
  
  if (password.value !== confirmPassword.value) {
    error.value = '两次密码不一致'
    return
  }
  
  loading.value = true
  error.value = ''
  
  try {
    const data = await api.post('/api/auth/setup', { password: password.value })
    if (data.success) {
      emit('setup')
    } else {
      error.value = data.message || '设置失败'
    }
  } catch (e) {
    error.value = e.message || '设置失败'
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
  margin-bottom: var(--spacing-lg);
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

.setup-hint {
  text-align: center;
  color: var(--text-color-2);
  margin: 0 0 var(--spacing-xl) 0;
  font-size: 0.875rem;
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
}

.toggle-password-btn:active {
  background: var(--bg-color-2);
}

.error {
  color: var(--error-color);
  font-size: 0.875rem;
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

  .setup-hint {
    font-size: 0.8125rem;
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

  .setup-hint {
    font-size: 0.75rem;
  }
}
</style>
