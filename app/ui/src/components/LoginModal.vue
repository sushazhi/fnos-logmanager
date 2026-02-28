<template>
  <div class="login-overlay">
    <div class="login-modal">
      <h2>🔐 登录</h2>
      <form @submit.prevent="handleLogin">
        <div class="form-group">
          <label>密码</label>
          <div class="password-input">
            <input 
              :type="showPassword ? 'text' : 'password'" 
              v-model="password" 
              placeholder="请输入密码"
              :disabled="loading"
            >
            <button type="button" class="toggle-password" @click="showPassword = !showPassword">
              {{ showPassword ? '🙈' : '👁️' }}
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

async function handleLogin() {
  if (!password.value) return
  
  loading.value = true
  error.value = ''
  
  try {
    const data = await api.post('/api/auth/login', { password: password.value })
    if (data.success) {
      api.setToken(data.token)
      emit('login', data.token)
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
  margin: 0 0 24px 0;
  text-align: center;
  color: var(--text-color, #333);
  font-size: 1.5rem;
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
  position: relative;
  display: flex;
  align-items: center;
}

.password-input input {
  width: 100%;
  padding: 12px 44px 12px 16px;
  border: 2px solid var(--border-color, #e0e0e0);
  border-radius: 8px;
  font-size: 1rem;
  transition: border-color 0.2s;
}

.password-input input:focus {
  outline: none;
  border-color: var(--primary-color, #667eea);
}

.toggle-password {
  position: absolute;
  right: 8px;
  background: none;
  border: none;
  cursor: pointer;
  font-size: 1.2rem;
  padding: 4px 8px;
  opacity: 0.7;
  transition: opacity 0.2s;
}

.toggle-password:hover {
  opacity: 1;
}

.error {
  color: #f44336;
  font-size: 0.9rem;
  margin-bottom: 16px;
  text-align: center;
}

.hint {
  color: #ff9800;
  font-size: 0.85rem;
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
</style>
