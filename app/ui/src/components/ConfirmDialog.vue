<template>
  <Teleport to="body">
    <Transition name="hm-overlay">
      <div v-if="visible" class="hm-overlay" @click.self="cancel">
        <Transition name="hm-modal" appear>
          <div v-if="visible" class="hm-modal" :class="[`hm-${currentType}`]">
            <!-- Icon -->
            <div class="hm-icon-wrap">
              <div class="hm-icon">
                <!-- danger / destructive -->
                <svg v-if="currentType === 'danger'" width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                  <path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"/>
                  <line x1="12" y1="9" x2="12" y2="13"/>
                  <line x1="12" y1="17" x2="12.01" y2="17"/>
                </svg>
                <!-- warning -->
                <svg v-else-if="currentType === 'warning'" width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                  <circle cx="12" cy="12" r="10"/>
                  <line x1="12" y1="8" x2="12" y2="12"/>
                  <line x1="12" y1="16" x2="12.01" y2="16"/>
                </svg>
                <!-- info / default -->
                <svg v-else width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                  <circle cx="12" cy="12" r="10"/>
                  <line x1="12" y1="16" x2="12" y2="12"/>
                  <line x1="12" y1="8" x2="12.01" y2="8"/>
                </svg>
              </div>
            </div>

            <!-- Title -->
            <div class="hm-title">{{ currentTitle }}</div>

            <!-- Message -->
            <div class="hm-message">
              <div v-for="(line, index) in messageLines" :key="index" :class="['hm-msg-line', { 'hm-msg-dir': isDirLine(line) }]">
                {{ line }}
              </div>
            </div>

            <!-- Actions -->
            <div class="hm-actions">
              <button class="hm-btn hm-btn-cancel" @click="cancel">
                取消
              </button>
              <button
                class="hm-btn hm-btn-confirm"
                :class="`hm-btn-${currentType}`"
                @click="confirm"
              >
                <span class="hm-btn-ripple"></span>
                {{ currentConfirmText }}
              </button>
            </div>
          </div>
        </Transition>
      </div>
    </Transition>
  </Teleport>
</template>

<script setup>
import { ref, computed } from 'vue'

const visible = ref(false)
let resolvePromise = null
const currentTitle = ref('确认')
const currentMessage = ref('')
const currentType = ref('info')
const currentConfirmText = ref('确定')

const messageLines = computed(() => {
  return currentMessage.value.split('\n').filter(line => line.trim())
})

function isDirLine(line) {
  return line.startsWith('/vol1/') || line.startsWith('/var/')
}

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
  z-index: 3000;
  padding: var(--spacing-xl);
}

/* ===== Modal ===== */
.hm-modal {
  background: var(--glass-bg-strong);
  backdrop-filter: blur(var(--glass-blur-heavy));
  -webkit-backdrop-filter: blur(var(--glass-blur-heavy));
  border: 1px solid var(--glass-border-strong);
  border-radius: var(--radius-xl);
  padding: var(--spacing-3xl) var(--spacing-2xl) var(--spacing-2xl);
  max-width: 340px;
  width: 100%;
  text-align: center;
  box-shadow: var(--glass-shadow), var(--depth-4);
  position: relative;
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
  position: relative;
  box-shadow: 0 0 20px var(--glow-primary-soft);
}

.hm-danger .hm-icon {
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

/* ===== Typography ===== */
.hm-title {
  font-size: var(--font-size-2xl);
  font-weight: 600;
  color: var(--text-color-1);
  margin-bottom: var(--spacing-sm);
  letter-spacing: -0.01em;
  line-height: 1.3;
}

.hm-message {
  font-size: var(--font-size-md);
  color: var(--text-color-2);
  margin-bottom: var(--spacing-2xl);
  line-height: 1.6;
  text-align: left;
  max-height: 300px;
  overflow-y: auto;
}

.hm-msg-line {
  padding: 2px 0;
}

.hm-msg-line:first-child {
  padding-top: 0;
}

.hm-msg-dir {
  font-family: 'Consolas', 'Monaco', 'Courier New', monospace;
  font-size: var(--font-size-base);
  color: var(--primary-color);
  background: var(--bg-color-2);
  padding: var(--spacing-xs) var(--spacing-sm);
  border-radius: var(--radius-xs);
  margin: var(--spacing-xs) 0;
  word-break: break-all;
  display: inline-block;
  max-width: 100%;
}

/* ===== Actions ===== */
.hm-actions {
  display: flex;
  gap: var(--spacing-sm);
}

.hm-btn {
  flex: 1;
  height: 44px;
  border: none;
  border-radius: var(--radius-sm);
  font-size: var(--font-size-md);
  font-weight: 500;
  cursor: pointer;
  transition: all var(--transition-fast);
  position: relative;
  overflow: hidden;
  outline: none;
}

/* Cancel button - glass style */
.hm-btn-cancel {
  background: var(--glass-bg);
  backdrop-filter: blur(var(--glass-blur));
  -webkit-backdrop-filter: blur(var(--glass-blur));
  color: var(--text-color-1);
  border: 1px solid var(--glass-border);
}

.hm-btn-cancel:hover {
  background: var(--glass-bg-strong);
  border-color: var(--glass-border-strong);
}

.hm-btn-cancel:active {
  transform: scale(0.97);
}

/* Confirm buttons */
.hm-btn-confirm {
  color: var(--text-color-on-primary);
}

.hm-btn-info,
.hm-btn-info:active {
  background: var(--primary-color);
}
.hm-btn-info:hover {
  background: var(--primary-hover);
  box-shadow: 0 0 28px var(--glow-primary), 0 0 60px var(--glow-primary-soft);
}

.hm-btn-info:active {
  box-shadow: 0 0 28px var(--glow-primary-strong);
}

.hm-btn-danger {
  background: var(--error-color);
}
.hm-btn-danger:hover {
  background: var(--log-critical-color);
  box-shadow: 0 0 20px var(--glow-primary);
}

.hm-btn-danger:active {
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

/* ===== Ripple effect ===== */
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

/* ===== Animations - HMOS6 spring curves ===== */
.hm-overlay-enter-active {
  transition: opacity var(--transition-base) var(--ease-harmony);
}
.hm-overlay-leave-active {
  transition: opacity var(--transition-micro) var(--ease-harmony);
}
.hm-overlay-enter-from,
.hm-overlay-leave-to {
  opacity: 0;
}

.hm-modal-enter-active {
  transition: all var(--transition-slow) var(--ease-spring);
}
.hm-modal-leave-active {
  transition: all var(--transition-fast) var(--ease-exit);
}
.hm-modal-enter-from {
  opacity: 0;
  transform: translateY(24px) scale(0.96);
}
.hm-modal-leave-to {
  opacity: 0;
  transform: translateY(12px) scale(0.98);
}

/* ===== Mobile ===== */
@media (max-width: 480px) {
  .hm-overlay {
    padding: var(--spacing-md);
  }

  .hm-modal {
    padding: var(--spacing-2xl) var(--spacing-lg) var(--spacing-lg);
    max-width: 100%;
  }

  .hm-title {
    font-size: var(--font-size-xl);
  }

  .hm-message {
    font-size: var(--font-size-sm);
    max-height: 200px;
    margin-bottom: var(--spacing-xl);
  }

  .hm-actions {
    flex-direction: column;
    gap: var(--spacing-xs);
  }

  .hm-btn {
    height: 44px;
    font-size: var(--font-size-base);
  }
}
</style>
