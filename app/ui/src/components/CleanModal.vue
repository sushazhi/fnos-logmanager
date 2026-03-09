<template>
  <div class="modal active" @click.self="$emit('close')">
    <div class="modal-content">
      <div class="modal-header">清理日志</div>
      <div class="modal-body">
        <div class="form-group">
          <label>清理方式</label>
          <select v-model="cleanType">
            <option value="truncate">清空大文件内容</option>
            <option value="deleteOld">删除旧归档文件</option>
          </select>
        </div>
        
        <div class="form-group" v-if="cleanType === 'truncate'">
          <label>文件大小阈值</label>
          <input type="text" v-model="threshold" placeholder="例如: 100M">
        </div>
        
        <div class="form-group" v-if="cleanType === 'deleteOld'">
          <label>删除多少天前的文件</label>
          <input type="number" v-model="days" placeholder="例如: 7">
        </div>
      </div>
      <div class="modal-footer">
        <button class="secondary" @click="$emit('close')">取消</button>
        <button class="danger" @click="execute">执行清理</button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'

const emit = defineEmits<{
  close: []
  execute: [type: string, threshold: string, days: number]
}>()

const cleanType = ref<'truncate' | 'deleteOld'>('truncate')
const threshold = ref('100M')
const days = ref(7)

function execute(): void {
  emit('execute', cleanType.value, threshold.value, days.value)
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

.modal-footer button.danger {
  background: var(--error-color);
  color: white;
}

.modal-footer button.danger:hover {
  background: #E52629;
}

.modal-footer button:active {
  transform: scale(0.98);
}
</style>
