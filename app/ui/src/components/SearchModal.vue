<template>
  <div class="hm-overlay-base" @click.self="$emit('close')">
    <div class="modal-content hm-modal-base">
      <div class="modal-header">查找日志</div>
      <div class="modal-body">
        <div class="form-group">
          <label>查找方式</label>
          <select v-model="searchType">
            <option value="size">按文件大小</option>
            <option value="name">按文件名称</option>
          </select>
        </div>
        
        <div class="form-group" v-if="searchType === 'size'">
          <label>文件大小阈值</label>
          <input type="text" v-model="threshold" placeholder="例如: 10M, 100M, 1G">
          <div class="hint">查找超过指定大小的日志文件</div>
        </div>
        
        <div class="form-group" v-if="searchType === 'name'">
          <label>文件名包含</label>
          <input type="text" v-model="pattern" placeholder="例如: error, access, app">
          <div class="hint">查找文件名包含关键字的日志文件</div>
        </div>
      </div>
      <div class="modal-footer">
        <button class="secondary hm-ripple-btn" @click="$emit('close')">取消</button>
        <button class="hm-ripple-btn" @click="execute">开始查找</button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'

const emit = defineEmits<{
  close: []
  execute: [type: 'size' | 'name', threshold: string, pattern: string]
}>()

const searchType = ref<'size' | 'name'>('size')
const threshold = ref('10M')
const pattern = ref('')

function execute(): void {
  emit('execute', searchType.value, threshold.value, pattern.value)
}
</script>

<style scoped>
.modal-content {
  padding: var(--spacing-2xl);
  max-width: 400px;
  width: 90%;
  position: relative;
  overflow: hidden;
}

.modal-content::before {
  content: '';
  position: absolute;
  inset: 0;
  background: var(--glass-shine);
  pointer-events: none;
  border-radius: inherit;
  z-index: 1;
}

.modal-content::after {
  content: '';
  position: absolute;
  inset: 0;
  box-shadow: var(--glass-edge-light);
  pointer-events: none;
  border-radius: inherit;
  z-index: 2;
}

.modal-header {
  position: relative;
  z-index: 3;
  font-size: var(--font-size-2xl);
  font-weight: var(--font-weight-semibold);
  margin-bottom: 0;
  padding-bottom: var(--spacing-md);
  color: var(--text-color-1);
  letter-spacing: var(--letter-spacing-tight);
  border-bottom: 1px solid var(--divider-color);
}

.form-group {
  margin-bottom: var(--spacing-lg);
}

.form-group label {
  display: block;
  margin-bottom: var(--spacing-sm);
  font-weight: var(--font-weight-medium);
  font-size: var(--font-size-md);
  color: var(--text-color-2);
}

.form-group input,
.form-group select {
  width: 100%;
  padding: var(--spacing-sm) var(--spacing-md);
  border: 1px solid var(--glass-border);
  border-radius: var(--radius-sm);
  font-size: var(--font-size-md);
  font-family: var(--font-family);
  background: var(--glass-bg);
  backdrop-filter: blur(var(--glass-blur));
  -webkit-backdrop-filter: blur(var(--glass-blur));
  color: var(--text-color-1);
  transition: border-color var(--transition-fast), box-shadow var(--transition-fast);
  position: relative;
  z-index: 3;
}

.form-group input:focus,
.form-group select:focus {
  outline: none;
  border-color: var(--glass-border-strong);
  box-shadow: var(--focus-ring);
}

.form-group input::placeholder {
  color: var(--text-color-3);
}

.hint {
  margin-top: var(--spacing-xs);
  font-size: var(--font-size-sm);
  color: var(--text-color-2);
  position: relative;
  z-index: 3;
}

.modal-footer {
  margin-top: var(--spacing-xl);
  display: flex;
  gap: var(--spacing-sm);
  justify-content: flex-end;
}

.modal-footer button {
  padding: var(--spacing-sm) var(--spacing-lg);
  border: none;
  border-radius: var(--radius-sm);
  cursor: pointer;
  font-size: var(--font-size-md);
  font-weight: var(--font-weight-medium);
  transition: all var(--transition-fast);
  position: relative;
  z-index: 3;
}

.modal-footer button.secondary {
  background: var(--glass-bg-strong);
  backdrop-filter: blur(var(--glass-blur));
  -webkit-backdrop-filter: blur(var(--glass-blur));
  border: 1px solid var(--glass-border);
  color: var(--text-color-1);
}

.modal-footer button.secondary:hover {
  background: var(--glass-bg-heavy);
  border-color: var(--glass-border-strong);
}

.modal-footer button:not(.secondary) {
  background: var(--primary-gradient);
  color: var(--text-color-on-primary);
}

.modal-footer button:not(.secondary):hover {
  transform: translateY(-2px);
  box-shadow: 0 0 20px var(--glow-primary), var(--shadow-md);
}

.modal-footer button:active {
  transform: scale(0.97);
}

.modal-footer button:not(.secondary):active {
  box-shadow: 0 0 28px var(--glow-primary-strong);
}

/* ===== Mobile ===== */
@media (max-width: 480px) {
  .modal-content {
    padding: var(--spacing-xl) var(--spacing-md);
    max-width: 100%;
    width: 95%;
    border-radius: var(--radius-lg) var(--radius-lg) var(--radius-md) var(--radius-md);
  }

  .modal-header {
    font-size: var(--font-size-xl);
    padding-bottom: var(--spacing-sm);
    margin-bottom: var(--spacing-md);
  }

  .modal-body {
    z-index: 3;
    position: relative;
  }

  .form-group {
    margin-bottom: var(--spacing-md);
  }

  .form-group label {
    font-size: var(--font-size-sm);
  }

  .form-group input,
  .form-group select {
    padding: var(--spacing-sm) var(--spacing-sm);
    font-size: var(--font-size-base);
  }

  .modal-footer {
    flex-direction: column;
    gap: var(--spacing-xs);
  }

  .modal-footer button {
    width: 100%;
    padding: var(--spacing-sm) var(--spacing-md);
    font-size: var(--font-size-base);
    height: 44px;
  }
}
</style>
