<template>
  <div class="section">
    <div class="section-header">
      <h4>全局设置</h4>
      <label class="switch">
        <input type="checkbox" :checked="enabled" @change="$emit('update:enabled', ($event.target as HTMLInputElement).checked)">
        <span class="slider"></span>
      </label>
    </div>
    <div class="setting-row">
      <label>检查间隔</label>
      <select 
        :value="checkInterval" 
        @change="$emit('update:checkInterval', parseInt(($event.target as HTMLSelectElement).value))" 
        :disabled="!enabled"
      >
        <option v-for="interval in intervals" :key="interval.value" :value="interval.value">
          {{ interval.label }}
        </option>
      </select>
    </div>
  </div>
</template>

<script setup lang="ts">
import { CHECK_INTERVALS } from './types';

defineProps<{
  enabled: boolean;
  checkInterval: number;
}>();

defineEmits<{
  (e: 'update:enabled', value: boolean): void;
  (e: 'update:checkInterval', value: number): void;
}>();

const intervals = CHECK_INTERVALS;
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

.setting-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: var(--spacing-sm) 0;
}

.setting-row label {
  font-size: var(--font-size-base);
  color: var(--text-color-2);
}

.setting-row select {
  padding: var(--spacing-xs) var(--spacing-sm);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-sm);
  background: var(--bg-color-2);
  color: var(--text-color-1);
  font-size: var(--font-size-base);
}

.setting-row select:disabled {
  opacity: 0.5;
}

.switch {
  position: relative;
  display: inline-block;
  width: 44px;
  height: 24px;
}

.switch input {
  opacity: 0;
  width: 0;
  height: 0;
}

.slider {
  position: absolute;
  cursor: pointer;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background-color: var(--bg-color-3);
  transition: var(--transition-fast);
  border-radius: var(--radius-xl);
}

.slider:before {
  position: absolute;
  content: "";
  height: 18px;
  width: 18px;
  left: 3px;
  bottom: 3px;
  background-color: var(--text-color-on-primary);
  transition: var(--transition-base);
  border-radius: 50%;
}

input:checked + .slider {
  background-color: var(--primary-color);
}

input:checked + .slider:before {
  transform: translateX(20px);
}
</style>
