<template>
  <div class="hm-overlay-base" @click.self="$emit('close')">
    <div class="modal-content hm-modal-base">
      <div class="modal-header">清理日志</div>
      <div class="modal-body">
        <div class="form-group">
          <label>清理方式</label>
          <select v-model="cleanType">
            <option value="truncate">清空大文件内容</option>
            <option value="deleteOld">删除旧归档文件</option>
            <option value="deleteUninstalled">删除未安装应用日志</option>
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
        
        <div class="form-group" v-if="cleanType === 'deleteUninstalled'">
          <p class="hint-text">将扫描所有日志目录，删除已卸载应用对应的日志文件。此操作不可恢复！</p>
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

const cleanType = ref<'truncate' | 'deleteOld' | 'deleteUninstalled'>('truncate')
const threshold = ref('100M')
const days = ref(7)

function execute(): void {
  emit('execute', cleanType.value, threshold.value, days.value)
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

.hint-text {
  font-size: var(--font-size-sm);
  color: var(--text-color-3);
  padding: var(--spacing-sm) var(--spacing-md);
  background: var(--glass-bg);
  backdrop-filter: blur(var(--glass-blur));
  -webkit-backdrop-filter: blur(var(--glass-blur));
  border: 1px solid var(--glass-border);
  border-radius: var(--radius-sm);
  line-height: 1.5;
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

.modal-footer button.danger {
  background: var(--error-color);
  color: var(--text-color-on-primary);
}

.modal-footer button.danger:hover {
  background: var(--log-critical-color);
  box-shadow: 0 0 20px var(--glow-primary);
}

.modal-footer button:active {
  transform: scale(0.97);
}

.modal-footer button.danger:active {
  box-shadow: 0 0 28px var(--glow-primary-strong);
}
</style>
