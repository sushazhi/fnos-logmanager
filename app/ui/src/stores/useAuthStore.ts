import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { api } from '../services/api'

export const useAuthStore = defineStore('auth', () => {
  // State
  const initialized = ref(false)
  const isLoggedIn = ref(false)
  const loading = ref(false)
  const error = ref<string | null>(null)

  // Getters
  const needsSetup = computed(() => !initialized.value)
  const needsLogin = computed(() => initialized.value && !isLoggedIn.value)

  // Actions
  async function checkStatus() {
    try {
      const response = await api.get<{ initialized: boolean; isLoggedIn: boolean }>('/api/auth/status')
      initialized.value = response.initialized
      isLoggedIn.value = response.isLoggedIn
      return response
    } catch (err) {
      error.value = err instanceof Error ? err.message : '检查认证状态失败'
      throw err
    }
  }

  async function setup(password: string) {
    try {
      loading.value = true
      const response = await api.post<{ success: boolean; error?: string }>('/api/auth/setup', { password })
      
      if (response.success) {
        initialized.value = true
        isLoggedIn.value = true
      }
      
      return response
    } catch (err) {
      error.value = err instanceof Error ? err.message : '初始化失败'
      throw err
    } finally {
      loading.value = false
    }
  }

  async function login(password: string) {
    try {
      loading.value = true
      const response = await api.post<{ success: boolean; error?: string }>('/api/auth/login', { password })
      
      if (response.success) {
        isLoggedIn.value = true
      }
      
      return response
    } catch (err) {
      error.value = err instanceof Error ? err.message : '登录失败'
      throw err
    } finally {
      loading.value = false
    }
  }

  async function logout() {
    try {
      await api.post('/api/auth/logout')
      isLoggedIn.value = false
      api.clearCSRFToken()
    } catch (err) {
      error.value = err instanceof Error ? err.message : '登出失败'
      throw err
    }
  }

  async function changePassword(oldPassword: string, newPassword: string) {
    try {
      loading.value = true
      const response = await api.post<{ success: boolean; error?: string }>(
        '/api/auth/change-password',
        { oldPassword, newPassword }
      )
      return response
    } catch (err) {
      error.value = err instanceof Error ? err.message : '修改密码失败'
      throw err
    } finally {
      loading.value = false
    }
  }

  function clearError() {
    error.value = null
  }

  return {
    // State
    initialized,
    isLoggedIn,
    loading,
    error,
    
    // Getters
    needsSetup,
    needsLogin,
    
    // Actions
    checkStatus,
    setup,
    login,
    logout,
    changePassword,
    clearError
  }
})
