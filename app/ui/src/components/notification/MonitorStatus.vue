<template>
  <div class="section">
    <div class="section-header">
      <h4>监控状态</h4>
      <span :class="['status-badge', status.running ? 'running' : 'stopped']">
        {{ status.running ? '运行中' : '已停止' }}
      </span>
    </div>
    <div class="status-info">
      <div class="info-item">
        <span class="label">监控文件数</span>
        <span class="value">{{ status.watchedFiles }}</span>
      </div>
      <div class="info-item">
        <span class="label">活跃规则数</span>
        <span class="value">{{ status.activeRules }}</span>
      </div>
    </div>
    <div class="btn-row">
      <button 
        class="control-btn" 
        @click="$emit('start')" 
        :disabled="status.running || !enabled"
      >启动</button>
      <button 
        class="control-btn danger" 
        @click="$emit('stop')" 
        :disabled="!status.running"
      >停止</button>
      <button 
        class="control-btn" 
        @click="$emit('check')" 
        :disabled="!enabled"
      >立即检查</button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { MonitorStatusProps } from './types';

defineProps<{
  status: MonitorStatusProps;
  enabled: boolean;
}>();

defineEmits<{
  (e: 'start'): void;
  (e: 'stop'): void;
  (e: 'check'): void;
}>();
</script>

<style scoped>
.section {
  margin-bottom: var(--spacing-lg);
}

.section-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: var(--spacing-md);
}

.section-header h4 {
  margin: 0;
  font-size: var(--font-size-md);
  font-weight: 600;
}

.status-badge {
  padding: var(--spacing-xs) var(--spacing-sm);
  border-radius: var(--radius-sm);
  font-size: var(--font-size-sm);
  font-weight: 500;
}

.status-badge.running {
  background: var(--success-bg);
  color: var(--success-color);
}

.status-badge.stopped {
  background: var(--warning-bg);
  color: var(--warning-color);
}

.status-info {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: var(--spacing-sm);
  margin-bottom: var(--spacing-md);
}

.info-item {
  display: flex;
  justify-content: space-between;
  padding: var(--spacing-sm) var(--spacing-md);
  background: var(--bg-color-2);
  border-radius: var(--radius-sm);
}

.info-item .label {
  font-size: var(--font-size-sm);
  color: var(--text-color-2);
}

.info-item .value {
  font-size: var(--font-size-md);
  font-weight: 600;
  color: var(--text-color-1);
}

.btn-row {
  display: flex;
  gap: var(--spacing-sm);
}

.control-btn {
  flex: 1;
  padding: var(--spacing-sm) var(--spacing-lg);
  border: none;
  border-radius: var(--radius-sm);
  background: var(--primary-color);
  color: var(--text-color-on-primary);
  font-size: var(--font-size-base);
  cursor: pointer;
  transition: opacity var(--transition-fast);
}

.control-btn:hover:not(:disabled) {
  opacity: 0.9;
}

.control-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.control-btn.danger {
  background: var(--error-color);
}
</style>
