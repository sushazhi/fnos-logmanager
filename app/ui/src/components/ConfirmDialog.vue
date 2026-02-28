<template>
  <Teleport to="body">
    <div class="confirm-overlay" v-if="visible" @click.self="cancel">
      <div class="confirm-modal">
        <div class="confirm-icon">{{ currentType === 'danger' ? '⚠️' : '❓' }}</div>
        <div class="confirm-title">{{ currentTitle }}</div>
        <div class="confirm-message">{{ currentMessage }}</div>
        <div class="confirm-actions">
          <button class="btn-cancel" @click="cancel">取消</button>
          <button class="btn-confirm" :class="{ danger: currentType === 'danger' }" @click="confirm">
            {{ currentConfirmText }}
          </button>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<script setup>
import { ref } from 'vue'

const visible = ref(false)
let resolvePromise = null
let currentTitle = ref('确认')
let currentMessage = ref('')
let currentType = ref('info')
let currentConfirmText = ref('确定')

function show(options = {}) {
  if (typeof options === 'string') {
    currentMessage.value = options
  } else {
    currentTitle.value = options.title || '确认'
    currentMessage.value = options.message || ''
    currentType.value = options.type || 'info'
    currentConfirmText.value = options.confirmText || '确定'
  }
  visible.value = true
  return new Promise((resolve) => {
    resolvePromise = resolve
  })
}

function confirm() {
  visible.value = false
  if (resolvePromise) {
    resolvePromise(true)
    resolvePromise = null
  }
}

function cancel() {
  visible.value = false
  if (resolvePromise) {
    resolvePromise(false)
    resolvePromise = null
  }
}

defineExpose({ show })
</script>

<style scoped>
.confirm-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(0, 0, 0, 0.5);
  display: flex;
  justify-content: center;
  align-items: center;
  z-index: 3000;
  animation: fadeIn 0.2s ease;
}

@keyframes fadeIn {
  from { opacity: 0; }
  to { opacity: 1; }
}

.confirm-modal {
  background: var(--card-bg, white);
  border-radius: 16px;
  padding: 24px;
  max-width: 360px;
  width: 90%;
  text-align: center;
  box-shadow: 0 10px 40px rgba(0, 0, 0, 0.2);
  animation: slideUp 0.2s ease;
}

@keyframes slideUp {
  from {
    opacity: 0;
    transform: translateY(20px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

.confirm-icon {
  font-size: 48px;
  margin-bottom: 16px;
}

.confirm-title {
  font-size: 18px;
  font-weight: 600;
  color: var(--text-color, #333);
  margin-bottom: 8px;
}

.confirm-message {
  font-size: 14px;
  color: var(--text-secondary, #666);
  margin-bottom: 24px;
  line-height: 1.5;
}

.confirm-actions {
  display: flex;
  gap: 12px;
}

.confirm-actions button {
  flex: 1;
  padding: 12px;
  border: none;
  border-radius: 8px;
  font-size: 14px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.2s;
}

.btn-cancel {
  background: var(--border-color, #e0e0e0);
  color: var(--text-color, #333);
}

.btn-cancel:hover {
  background: var(--bg-color, #f0f0f0);
}

.btn-confirm {
  background: var(--primary-color, #667eea);
  color: white;
}

.btn-confirm:hover {
  opacity: 0.9;
  transform: translateY(-1px);
}

.btn-confirm.danger {
  background: linear-gradient(135deg, #ff6b6b 0%, #ee5a5a 100%);
}

.btn-confirm.danger:hover {
  opacity: 0.9;
}
</style>
