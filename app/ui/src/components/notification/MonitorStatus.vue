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
  margin-bottom: 16px;
}

.section-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 12px;
}

.section-header h4 {
  margin: 0;
  font-size: 14px;
  font-weight: 600;
}

.status-badge {
  padding: 2px 8px;
  border-radius: 4px;
  font-size: 12px;
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
  gap: 8px;
  margin-bottom: 12px;
}

.info-item {
  display: flex;
  justify-content: space-between;
  padding: 8px 12px;
  background: var(--bg-secondary);
  border-radius: 4px;
}

.info-item .label {
  font-size: 12px;
  color: var(--text-secondary);
}

.info-item .value {
  font-size: 14px;
  font-weight: 600;
  color: var(--text-primary);
}

.btn-row {
  display: flex;
  gap: 8px;
}

.control-btn {
  flex: 1;
  padding: 8px 16px;
  border: none;
  border-radius: 4px;
  background: var(--primary-color);
  color: white;
  font-size: 13px;
  cursor: pointer;
  transition: opacity 0.2s;
}

.control-btn:hover:not(:disabled) {
  opacity: 0.9;
}

.control-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.control-btn.danger {
  background: var(--danger-color);
}
</style>
