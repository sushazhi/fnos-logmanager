<template>
  <div class="section">
    <div class="section-header">
      <h4>通知规则</h4>
      <button class="add-btn" @click="$emit('add')">+ 添加规则</button>
    </div>
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

.add-btn {
  padding: 4px 12px;
  border: 1px solid var(--primary-color);
  border-radius: 4px;
  background: transparent;
  color: var(--primary-color);
  font-size: 12px;
  cursor: pointer;
}

.add-btn:hover {
  background: var(--primary-bg);
}

.rule-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.rule-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px;
  background: var(--bg-secondary);
  border-radius: 4px;
}

.rule-info {
  flex: 1;
}

.rule-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 8px;
}

.rule-name {
  font-size: 14px;
  font-weight: 500;
}

.rule-details {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.detail-item {
  font-size: 12px;
  color: var(--text-secondary);
  padding: 2px 6px;
  background: var(--bg-tertiary);
  border-radius: 4px;
}

.rule-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-left: 12px;
}

.edit-btn {
  padding: 4px 8px;
  border: 1px solid var(--border-color);
  border-radius: 4px;
  background: transparent;
  color: var(--text-secondary);
  font-size: 12px;
  cursor: pointer;
}

.edit-btn:hover {
  background: var(--bg-tertiary);
}

.delete-btn {
  padding: 4px 8px;
  border: none;
  border-radius: 4px;
  background: transparent;
  color: var(--danger-color);
  font-size: 16px;
  cursor: pointer;
}

.delete-btn:hover {
  background: var(--danger-bg);
}

.empty-hint {
  padding: 24px;
  text-align: center;
  color: var(--text-secondary);
  font-size: 13px;
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
  background-color: var(--bg-tertiary);
  transition: 0.3s;
  border-radius: 24px;
}

.slider:before {
  position: absolute;
  content: "";
  height: 18px;
  width: 18px;
  left: 3px;
  bottom: 3px;
  background-color: white;
  transition: 0.3s;
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
