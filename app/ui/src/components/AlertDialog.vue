<template>
  <Teleport to="body">
    <Transition name="hm-overlay">
      <div v-if="visible" class="hm-overlay" @click.self="cancel">
        <Transition name="hm-modal" appear>
          <div v-if="visible" class="hm-modal" :class="`hm-${type}`">
            <!-- Icon (not for confirm type) -->
            <div v-if="type !== 'confirm'" class="hm-icon-wrap">
              <div class="hm-icon">
                <!-- success -->
                <svg v-if="type === 'success'" width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
                  <polyline points="20 6 9 17 4 12"/>
                </svg>
                <!-- error -->
                <svg v-else-if="type === 'error'" width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
                  <line x1="18" y1="6" x2="6" y2="18"/>
                  <line x1="6" y1="6" x2="18" y2="18"/>
                </svg>
                <!-- warning -->
                <svg v-else-if="type === 'warning'" width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                  <path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"/>
                  <line x1="12" y1="9" x2="12" y2="13"/>
                  <line x1="12" y1="17" x2="12.01" y2="17"/>
                </svg>
                <!-- info -->
                <svg v-else width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                  <circle cx="12" cy="12" r="10"/>
                  <line x1="12" y1="16" x2="12" y2="12"/>
                  <line x1="12" y1="8" x2="12.01" y2="8"/>
                </svg>
              </div>
            </div>

            <!-- Body -->
            <div class="hm-body">
              <h4 v-if="title" class="hm-title">{{ title }}</h4>
              <p class="hm-message">{{ message }}</p>
            </div>

            <!-- Footer -->
            <div class="hm-footer">
              <button
                v-if="copyText"
                class="hm-btn hm-btn-copy"
                @click="copyToClipboard"
              >
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                  <rect x="9" y="9" width="13" height="13" rx="2" ry="2"/>
                  <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/>
                </svg>
                <span>{{ copied ? '已复制' : '复制' }}</span>
              </button>

              <button
                v-if="type === 'confirm'"
                class="hm-btn hm-btn-cancel"
                @click="cancel"
              >取消</button>

              <button
                class="hm-btn hm-btn-confirm"
                :class="`hm-btn-${type}`"
                @click="confirm"
              >
                <span class="hm-btn-ripple"></span>
                {{ confirmText }}
              </button>
            </div>
          </div>
        </Transition>
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
  if (!props.copyText) return

  if (navigator.clipboard && typeof navigator.clipboard.writeText === 'function') {
    navigator.clipboard.writeText(props.copyText).then(() => {
      copied.value = true
      setTimeout(() => { copied.value = false }, 2000)
    }).catch(() => fallbackCopy())
  } else {
    fallbackCopy()
  }
}

function fallbackCopy() {
  const textArea = document.createElement('textarea')
  textArea.value = props.copyText || ''
  textArea.style.position = 'fixed'
  textArea.style.left = '-9999px'
  textArea.style.top = '0'
  document.body.appendChild(textArea)
  textArea.focus()
  textArea.select()
  try {
    if (document.execCommand('copy')) {
      copied.value = true
      setTimeout(() => { copied.value = false }, 2000)
    }
  } catch (err) {
    console.error('复制失败:', err)
  }
  document.body.removeChild(textArea)
}
</script>

<style scoped>
/* ===== Overlay ===== */
.hm-overlay {
  position: fixed;
  inset: 0;
  background: var(--overlay);
  backdrop-filter: blur(8px);
  -webkit-backdrop-filter: blur(8px);
  display: flex;
  justify-content: center;
  align-items: center;
  z-index: 2000;
  padding: var(--spacing-xl);
}

/* ===== Modal ===== */
.hm-modal {
  background: var(--card-bg);
  border-radius: var(--radius-xl);
  padding: var(--spacing-3xl) var(--spacing-2xl) var(--spacing-2xl);
  max-width: 360px;
  width: 100%;
  text-align: center;
  box-shadow: var(--shadow-xl);
  overflow: hidden;
}

/* ===== Icon ===== */
.hm-icon-wrap {
  display: flex;
  justify-content: center;
  margin-bottom: var(--spacing-lg);
}

.hm-icon {
  width: 56px;
  height: 56px;
  border-radius: var(--radius-full);
  display: flex;
  align-items: center;
  justify-content: center;
}

.hm-success .hm-icon {
  background: var(--success-bg);
  color: var(--success-color);
}
.hm-error .hm-icon {
  background: var(--error-bg);
  color: var(--error-color);
}
.hm-warning .hm-icon {
  background: var(--warning-bg);
  color: var(--warning-color);
}
.hm-info .hm-icon {
  background: var(--info-bg);
  color: var(--info-color);
}

/* ===== Body ===== */
.hm-body {
  margin-bottom: var(--spacing-xl);
}

.hm-title {
  margin: 0 0 var(--spacing-sm) 0;
  font-size: var(--font-size-2xl);
  font-weight: 600;
  color: var(--text-color-1);
  letter-spacing: -0.01em;
  line-height: 1.3;
}

.hm-message {
  margin: 0;
  font-size: var(--font-size-md);
  color: var(--text-color-2);
  line-height: 1.6;
  white-space: pre-wrap;
}

/* ===== Footer ===== */
.hm-footer {
  display: flex;
  justify-content: center;
  gap: var(--spacing-sm);
  flex-wrap: wrap;
}

/* ===== Buttons ===== */
.hm-btn {
  height: 40px;
  padding: 0 var(--spacing-xl);
  border: none;
  border-radius: var(--radius-sm);
  font-size: var(--font-size-md);
  font-weight: 500;
  cursor: pointer;
  transition: all var(--transition-fast);
  position: relative;
  overflow: hidden;
  outline: none;
  display: inline-flex;
  align-items: center;
  gap: var(--spacing-xs);
  white-space: nowrap;
}

/* Copy / secondary */
.hm-btn-copy {
  background: var(--bg-color-2);
  color: var(--text-color-2);
  border: 1px solid var(--border-color);
}
.hm-btn-copy:hover {
  background: var(--bg-color-3);
  color: var(--text-color-1);
}

/* Cancel - outline */
.hm-btn-cancel {
  background: transparent;
  color: var(--text-color-1);
  border: 1px solid var(--border-color);
}
.hm-btn-cancel:hover {
  background: var(--bg-color-2);
  border-color: var(--text-color-3);
}

/* Confirm */
.hm-btn-confirm {
  color: var(--text-color-on-primary);
}
.hm-btn-info {
  background: var(--primary-color);
}
.hm-btn-info:hover {
  background: var(--primary-hover);
  box-shadow: 0 0 20px var(--glow-primary);
}

.hm-btn-info:active {
  box-shadow: 0 0 28px var(--glow-primary-strong);
}
.hm-btn-success {
  background: var(--success-color);
}
.hm-btn-success:hover {
  background: var(--success-color);
  box-shadow: 0 0 20px var(--glow-primary);
}

.hm-btn-success:active {
  box-shadow: 0 0 28px var(--glow-primary-strong);
}
.hm-btn-error {
  background: var(--error-color);
}
.hm-btn-error:hover {
  background: var(--log-critical-color);
  box-shadow: 0 0 20px var(--glow-primary);
}

.hm-btn-error:active {
  box-shadow: 0 0 28px var(--glow-primary-strong);
}
.hm-btn-warning {
  background: var(--warning-color);
  color: var(--text-color-on-primary);
}
.hm-btn-warning:hover {
  background: var(--warning-color);
  box-shadow: 0 0 20px var(--glow-primary);
}

.hm-btn-warning:active {
  box-shadow: 0 0 28px var(--glow-primary-strong);
}

.hm-btn:active {
  transform: scale(0.97);
}

/* ===== Ripple ===== */
.hm-btn .hm-btn-ripple {
  position: absolute;
  inset: 0;
  border-radius: inherit;
  pointer-events: none;
}
.hm-btn:active .hm-btn-ripple {
  background: rgba(255, 255, 255, 0.2);
  animation: hm-ripple 0.4s ease-out;
}

@keyframes hm-ripple {
  0% { opacity: 1; transform: scale(0); }
  100% { opacity: 0; transform: scale(2); }
}

/* ===== Transitions ===== */
.hm-overlay-enter-active {
  transition: opacity 0.25s var(--ease-harmony);
}
.hm-overlay-leave-active {
  transition: opacity 0.15s var(--ease-harmony);
}
.hm-overlay-enter-from,
.hm-overlay-leave-to {
  opacity: 0;
}

.hm-modal-enter-active {
  transition: all 0.3s var(--ease-harmony);
}
.hm-modal-leave-active {
  transition: all 0.2s var(--ease-harmony);
}
.hm-modal-enter-from {
  opacity: 0;
  transform: translateY(24px) scale(0.96);
}
.hm-modal-leave-to {
  opacity: 0;
  transform: translateY(12px) scale(0.98);
}
</style>
