<template>
  <div class="modal active" @click.self="$emit('close')">
    <div class="modal-content">
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
        <button class="secondary" @click="$emit('close')">取消</button>
        <button @click="execute">开始查找</button>
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
.modal {
  position: fixed;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  background: rgba(0, 0, 0, 0.5);
  display: flex;
  justify-content: center;
  align-items: center;
  z-index: 1000;
}

.modal-content {
  background: var(--card-bg);
  padding: var(--spacing-2xl);
  border-radius: var(--radius-md);
  max-width: 400px;
  width: 90%;
  box-shadow: var(--shadow-xl);
}

.modal-header {
  font-size: 1.125rem;
  font-weight: 600;
  margin-bottom: var(--spacing-xl);
  color: var(--text-color-1);
  letter-spacing: -0.01em;
}

.form-group {
  margin-bottom: var(--spacing-lg);
}

.form-group label {
  display: block;
  margin-bottom: var(--spacing-sm);
  font-weight: 500;
  font-size: 0.875rem;
  color: var(--text-color-1);
}

.form-group input,
.form-group select {
  width: 100%;
  padding: var(--spacing-sm);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-xs);
  font-size: 0.875rem;
  font-family: var(--font-family);
  background: var(--card-bg);
  color: var(--text-color-1);
  transition: border-color var(--transition-fast);
}

.form-group input:focus,
.form-group select:focus {
  outline: none;
  border-color: var(--primary-color);
}

.form-group input::placeholder {
  color: var(--text-color-3);
}

.hint {
  margin-top: var(--spacing-xs);
  font-size: 0.75rem;
  color: var(--text-color-2);
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
  border-radius: var(--radius-xs);
  cursor: pointer;
  font-size: 0.875rem;
  font-weight: 500;
  transition: all var(--transition-fast);
}

.modal-footer button.secondary {
  background: var(--bg-color-2);
  color: var(--text-color-1);
}

.modal-footer button.secondary:hover {
  background: var(--bg-color-3);
}

.modal-footer button:not(.secondary) {
  background: var(--primary-gradient);
  color: white;
}

.modal-footer button:not(.secondary):hover {
  transform: translateY(-2px);
  box-shadow: var(--shadow-md);
}

.modal-footer button:active {
  transform: scale(0.98);
}
</style>
