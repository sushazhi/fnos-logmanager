<template>
  <Teleport to="body">
    <Transition name="modal">
      <div v-if="visible" class="modal-overlay" @click.self="cancel">
        <div class="modal-content" :class="type">
          <div class="modal-icon" v-if="type !== 'confirm'">
            <span v-if="type === 'success'" class="icon success">✓</span>
            <span v-else-if="type === 'error'" class="icon error">✕</span>
            <span v-else-if="type === 'warning'" class="icon warning">!</span>
            <span v-else class="icon info">i</span>
          </div>
          <div class="modal-body">
            <h4 v-if="title">{{ title }}</h4>
            <p>{{ message }}</p>
          </div>
          <div class="modal-footer">
            <button 
              v-if="copyText" 
              class="btn copy" 
              @click="copyToClipboard"
            >{{ copied ? '已复制' : '复制' }}</button>
            <button 
              v-if="type === 'confirm'" 
              class="btn cancel" 
              @click="cancel"
            >取消</button>
            <button 
              class="btn confirm" 
              :class="type"
              @click="confirm"
            >{{ confirmText }}</button>
          </div>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'

interface Props {
  modelValue: boolean
  title?: string
  message: string
  type?: 'info' | 'success' | 'error' | 'warning' | 'confirm'
  confirmText?: string
  copyText?: string
}

const props = withDefaults(defineProps<Props>(), {
  type: 'info',
  confirmText: '确定',
  copyText: ''
})

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  'confirm': []
  'cancel': []
}>()

const visible = ref(props.modelValue)
const copied = ref(false)

watch(() => props.modelValue, (val) => {
  visible.value = val
  if (val) {
    copied.value = false
  }
})

function close() {
  visible.value = false
  emit('update:modelValue', false)
}

function confirm() {
  emit('confirm')
  close()
}

function cancel() {
  emit('cancel')
  close()
}

function copyToClipboard() {
  if (props.copyText) {
    navigator.clipboard.writeText(props.copyText).then(() => {
      copied.value = true
      setTimeout(() => {
        copied.value = false
      }, 2000)
    }).catch(() => {
      // 复制失败
    })
  }
}
</script>

<style scoped>
.modal-overlay {
  position: fixed;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  background: rgba(0, 0, 0, 0.5);
  display: flex;
  justify-content: center;
  align-items: center;
  z-index: 2000;
}

.modal-content {
  background: var(--card-bg);
  border-radius: var(--radius-md);
  padding: var(--spacing-xl);
  max-width: 400px;
  width: 90%;
  text-align: center;
  box-shadow: var(--shadow-xl);
}

.modal-icon {
  margin-bottom: var(--spacing-md);
}

.icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 48px;
  height: 48px;
  border-radius: 50%;
  font-size: 1.5rem;
  font-weight: bold;
}

.icon.success {
  background: var(--success-color);
  color: white;
}

.icon.error {
  background: var(--error-color);
  color: white;
}

.icon.warning {
  background: #f59e0b;
  color: white;
}

.icon.info {
  background: var(--primary-color);
  color: white;
}

.modal-body h4 {
  margin: 0 0 var(--spacing-sm) 0;
  font-size: 1.125rem;
  color: var(--text-color-1);
}

.modal-body p {
  margin: 0;
  font-size: 0.9375rem;
  color: var(--text-color-2);
  line-height: 1.5;
  white-space: pre-wrap;
}

.modal-footer {
  margin-top: var(--spacing-xl);
  display: flex;
  justify-content: center;
  gap: var(--spacing-sm);
}

.btn {
  padding: var(--spacing-sm) var(--spacing-xl);
  border: none;
  border-radius: var(--radius-sm);
  font-size: 0.875rem;
  cursor: pointer;
  transition: all var(--transition-fast);
  min-width: 80px;
}

.btn.copy {
  background: var(--bg-color-2);
  color: var(--text-color-1);
  border: 1px solid var(--border-color);
}

.btn.copy:hover {
  background: var(--bg-color-3);
}

.btn.cancel {
  background: var(--bg-color-2);
  color: var(--text-color-1);
  border: 1px solid var(--border-color);
}

.btn.cancel:hover {
  background: var(--bg-color-3);
}

.btn.confirm {
  background: var(--primary-color);
  color: white;
}

.btn.confirm:hover {
  background: var(--primary-hover);
}

.btn.confirm.success {
  background: var(--success-color);
}

.btn.confirm.error {
  background: var(--error-color);
}

.btn.confirm.warning {
  background: #f59e0b;
}

/* Transition */
.modal-enter-active,
.modal-leave-active {
  transition: opacity 0.2s ease;
}

.modal-enter-from,
.modal-leave-to {
  opacity: 0;
}

.modal-enter-active .modal-content,
.modal-leave-active .modal-content {
  transition: transform 0.2s ease;
}

.modal-enter-from .modal-content,
.modal-leave-to .modal-content {
  transform: scale(0.9);
}
</style>
