<template>
  <div class="section">
    <div class="section-header">
      <h4>通知规则</h4>
    </div>
    <button class="add-btn" @click="$emit('add')">+ 添加规则</button>
    <div class="rule-list" v-if="rules.length > 0">
      <div class="rule-item" v-for="rule in rules" :key="rule.id">
        <div class="rule-info">
          <div class="rule-header">
            <span class="rule-name">{{ rule.name }}</span>
            <label class="switch small">
              <input 
                type="checkbox" 
                :checked="rule.enabled" 
                @change="$emit('toggle', rule.id, ($event.target as HTMLInputElement).checked)"
              >
              <span class="slider"></span>
            </label>
          </div>
          <div class="rule-details">
            <span class="detail-item" v-if="rule.appName">
              应用: {{ rule.appName }}
            </span>
            <span class="detail-item">
              级别: {{ getLogLevelName(rule.logLevel) }}
            </span>
            <span class="detail-item" v-if="rule.keywords?.length">
              关键词: {{ rule.keywords.join(', ') }}
            </span>
          </div>
        </div>
        <div class="rule-actions">
          <button class="edit-btn" @click="$emit('edit', rule)" title="编辑">编辑</button>
          <button class="delete-btn" @click="$emit('delete', rule.id)" title="删除">×</button>
        </div>
      </div>
    </div>
    <div class="empty-hint" v-else>
      暂无通知规则，请先添加
    </div>
  </div>
</template>

<script setup lang="ts">
import { RuleItem, LOG_LEVELS } from './types';

defineProps<{
  rules: RuleItem[];
}>();

defineEmits<{
  (e: 'add'): void;
  (e: 'edit', rule: RuleItem): void;
  (e: 'toggle', id: string, enabled: boolean): void;
  (e: 'delete', id: string): void;
}>();

function getLogLevelName(level: string): string {
  const found = LOG_LEVELS.find(l => l.value === level);
  return found?.label || level;
}
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

.add-btn {
  width: 100%;
  padding: var(--spacing-sm) var(--spacing-lg);
  border: 1px solid var(--primary-color);
  border-radius: var(--radius-sm);
  background: transparent;
  color: var(--primary-color);
  font-size: var(--font-size-base);
  cursor: pointer;
}

.add-btn:hover {
  background: var(--info-bg);
}

.rule-list {
  display: flex;
  flex-direction: column;
  gap: var(--spacing-sm);
}

.rule-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: var(--spacing-md);
  background: var(--bg-color-2);
  border-radius: var(--radius-sm);
}

.rule-info {
  flex: 1;
}

.rule-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: var(--spacing-sm);
}

.rule-name {
  font-size: var(--font-size-md);
  font-weight: 500;
}

.rule-details {
  display: flex;
  flex-wrap: wrap;
  gap: var(--spacing-sm);
}

.detail-item {
  font-size: var(--font-size-sm);
  color: var(--text-color-2);
  padding: var(--spacing-xs) var(--spacing-sm);
  background: var(--bg-color-3);
  border-radius: var(--radius-sm);
}

.rule-actions {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
  margin-left: var(--spacing-md);
}

.edit-btn {
  padding: var(--spacing-xs) var(--spacing-sm);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-sm);
  background: transparent;
  color: var(--text-color-2);
  font-size: var(--font-size-sm);
  cursor: pointer;
}

.edit-btn:hover {
  background: var(--bg-color-3);
}

.delete-btn {
  padding: var(--spacing-xs) var(--spacing-sm);
  border: none;
  border-radius: var(--radius-sm);
  background: transparent;
  color: var(--error-color);
  font-size: var(--font-size-xl);
  cursor: pointer;
}

.delete-btn:hover {
  background: var(--error-bg);
}

.empty-hint {
  padding: var(--spacing-2xl);
  text-align: center;
  color: var(--text-color-2);
  font-size: var(--font-size-base);
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

.switch.small {
  width: 36px;
  height: 20px;
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

.switch.small .slider:before {
  height: 14px;
  width: 14px;
}

input:checked + .slider {
  background-color: var(--primary-color);
}

input:checked + .slider:before {
  transform: translateX(20px);
}

.switch.small input:checked + .slider:before {
  transform: translateX(16px);
}
</style>
