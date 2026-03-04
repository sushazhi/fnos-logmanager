<template>
  <Teleport to="body">
    <div class="confirm-overlay" v-if="visible" @click.self="cancel">
      <div class="confirm-modal">
        <div class="confirm-icon">{{ currentType === 'danger' ? '!' : '?' }}</div>
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
  animation: fadeIn var(--transition-base) ease;
}

@keyframes fadeIn {
  from { opacity: 0; }
  to { opacity: 1; }
}

.confirm-modal {
  background: var(--card-bg);
  border-radius: var(--radius-lg);
  padding: var(--spacing-2xl);
  max-width: 360px;
  width: 90%;
  text-align: center;
  box-shadow: var(--shadow-xl);
  animation: slideUp var(--transition-base) ease;
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
  font-size: 3rem;
  margin-bottom: var(--spacing-lg);
}

.confirm-title {
  font-size: 1.125rem;
  font-weight: 600;
  color: var(--text-color-1);
  margin-bottom: var(--spacing-sm);
  letter-spacing: -0.01em;
}

.confirm-message {
  font-size: 0.875rem;
  color: var(--text-color-2);
  margin-bottom: var(--spacing-2xl);
  line-height: 1.5;
}

.confirm-actions {
  display: flex;
  gap: var(--spacing-md);
}

.confirm-actions button {
  flex: 1;
  padding: var(--spacing-md);
  border: none;
  border-radius: var(--radius-sm);
  font-size: 0.875rem;
  font-weight: 500;
  cursor: pointer;
  transition: all var(--transition-fast);
}

.btn-cancel {
  background: var(--bg-color-2);
  color: var(--text-color-1);
}

.btn-cancel:hover {
  background: var(--bg-color-3);
}

.btn-cancel:active {
  transform: scale(0.98);
}

.btn-confirm {
  background: var(--primary-color);
  color: white;
}

.btn-confirm:hover {
  opacity: 0.9;
  transform: translateY(-1px);
}

.btn-confirm:active {
  transform: scale(0.98);
}

.btn-confirm.danger {
  background: var(--error-color);
}

.btn-confirm.danger:hover {
  opacity: 0.9;
}
</style>
