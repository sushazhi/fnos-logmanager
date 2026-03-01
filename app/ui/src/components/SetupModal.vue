<template>
  <div class="login-overlay">
    <div class="login-modal">
      <h2>🔐 设置密码</h2>
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
              {{ showPassword ? '🙈' : '👁️' }}
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
              {{ showConfirmPassword ? '🙈' : '👁️' }}
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
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  display: flex;
  justify-content: center;
  align-items: center;
  z-index: 2000;
}

.login-modal {
  background: var(--card-bg, white);
  padding: 40px;
  border-radius: 16px;
  box-shadow: 0 10px 40px rgba(0, 0, 0, 0.2);
  width: 100%;
  max-width: 360px;
}

.login-modal h2 {
  margin: 0 0 8px 0;
  text-align: center;
  color: var(--text-color, #333);
  font-size: 1.5rem;
}

.setup-hint {
  text-align: center;
  color: var(--text-secondary, #666);
  margin: 0 0 24px 0;
  font-size: 0.9rem;
}

.form-group {
  margin-bottom: 20px;
}

.form-group label {
  display: block;
  margin-bottom: 8px;
  font-weight: 500;
  color: var(--text-color, #333);
}

.password-input {
  display: flex;
  align-items: stretch;
}

.password-input input {
  flex: 4;
  min-width: 0;
  padding: 12px 16px;
  border: 2px solid var(--border-color, #e0e0e0);
  border-radius: 8px 0 0 8px;
  font-size: 1rem;
  transition: border-color 0.2s;
}

.password-input input:focus {
  outline: none;
  border-color: var(--primary-color, #667eea);
}

.toggle-password-btn {
  flex: 1;
  min-width: 0;
  padding: 0;
  background: var(--card-bg, white);
  border: 2px solid var(--border-color, #e0e0e0);
  border-left: none;
  border-radius: 0 8px 8px 0;
  font-size: 0.8rem;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: all 0.2s;
}

.toggle-password-btn:active {
  background: var(--border-color, #e0e0e0);
}

.error {
  color: #f44336;
  font-size: 0.9rem;
  margin-bottom: 16px;
  text-align: center;
}

button[type="submit"] {
  width: 100%;
  padding: 14px;
  background: linear-gradient(135deg, var(--primary-color, #667eea) 0%, #764ba2 100%);
  color: white;
  border: none;
  border-radius: 8px;
  font-size: 1rem;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s;
}

button[type="submit"]:hover:not(:disabled) {
  transform: translateY(-2px);
  box-shadow: 0 4px 12px rgba(102, 126, 234, 0.4);
}

button[type="submit"]:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

@media (max-width: 480px) {
  .login-modal {
    margin: 20px;
    padding: 30px 20px;
  }
}
</style>
