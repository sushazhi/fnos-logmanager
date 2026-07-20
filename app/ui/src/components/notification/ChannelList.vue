<template>
  <div class="section">
    <div class="section-header">
      <h4>通知渠道</h4>
    </div>
    <button class="add-btn" @click="$emit('add')">+ 添加渠道</button>
    <div class="channel-list" v-if="channels.length > 0">
      <div class="channel-item" v-for="channel in channels" :key="channel.name">
        <div class="channel-info">
          <span class="channel-name">{{ channel.name }}</span>
          <span class="channel-type">{{ getChannelTypeName(channel.channel) }}</span>
        </div>
        <div class="channel-actions">
          <label class="switch small">
            <input 
              type="checkbox" 
              :checked="channel.enabled" 
              @change="$emit('toggle', channel.name, ($event.target as HTMLInputElement).checked)"
            >
            <span class="slider"></span>
          </label>
          <button class="edit-btn" @click="$emit('edit', channel)" title="编辑">编辑</button>
          <button class="test-btn" @click="$emit('test', channel.name)" title="测试">测试</button>
          <button class="delete-btn" @click="$emit('delete', channel.name)" title="删除">×</button>
        </div>
      </div>
    </div>
    <div class="empty-hint" v-else>
      暂无通知渠道，请先添加
    </div>
  </div>
</template>

<script setup lang="ts">
import { ChannelItem, CHANNEL_TYPES } from './types';

defineProps<{
  channels: ChannelItem[];
}>();

defineEmits<{
  (e: 'add'): void;
  (e: 'edit', channel: ChannelItem): void;
  (e: 'toggle', name: string, enabled: boolean): void;
  (e: 'test', name: string): void;
  (e: 'delete', name: string): void;
}>();

function getChannelTypeName(channel: string): string {
  return CHANNEL_TYPES[channel] || channel;
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

.channel-list {
  display: flex;
  flex-direction: column;
  gap: var(--spacing-sm);
}

.channel-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: var(--spacing-md);
  background: var(--bg-color-2);
  border-radius: var(--radius-sm);
}

.channel-info {
  display: flex;
  flex-direction: column;
  gap: var(--spacing-xs);
}

.channel-name {
  font-size: var(--font-size-md);
  font-weight: 500;
}

.channel-type {
  font-size: var(--font-size-sm);
  color: var(--text-color-2);
}

.channel-actions {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
}

.switch.small {
  width: 36px;
  height: 20px;
}

.switch.small .slider:before {
  height: 14px;
  width: 14px;
}

.switch.small input:checked + .slider:before {
  transform: translateX(16px);
}

.edit-btn, .test-btn {
  padding: var(--spacing-xs) var(--spacing-sm);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-sm);
  background: transparent;
  color: var(--text-color-2);
  font-size: var(--font-size-sm);
  cursor: pointer;
}

.edit-btn:hover, .test-btn:hover {
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
