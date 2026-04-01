<template>
  <div class="section">
    <div class="section-header">
      <h4>通知渠道</h4>
      <button class="add-btn" @click="$emit('add')">+ 添加渠道</button>
    </div>
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

.channel-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.channel-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px;
  background: var(--bg-secondary);
  border-radius: 4px;
}

.channel-info {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.channel-name {
  font-size: 14px;
  font-weight: 500;
}

.channel-type {
  font-size: 12px;
  color: var(--text-secondary);
}

.channel-actions {
  display: flex;
  align-items: center;
  gap: 8px;
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
  padding: 4px 8px;
  border: 1px solid var(--border-color);
  border-radius: 4px;
  background: transparent;
  color: var(--text-secondary);
  font-size: 12px;
  cursor: pointer;
}

.edit-btn:hover, .test-btn:hover {
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

input:checked + .slider {
  background-color: var(--primary-color);
}

input:checked + .slider:before {
  transform: translateX(20px);
}
</style>
