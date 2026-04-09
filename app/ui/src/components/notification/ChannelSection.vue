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
            <input type="checkbox" v-model="channel.enabled" @change="$emit('update', channel)">
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
interface ChannelConfig {
  channel: string
  name: string
  enabled: boolean
  [key: string]: unknown
}

interface ChannelType {
  type: string
  name: string
  fields: string[]
}

const props = defineProps<{
  channels: ChannelConfig[]
  channelTypes: ChannelType[]
}>()

defineEmits<{
  add: []
  edit: [channel: ChannelConfig]
  update: [channel: ChannelConfig]
  test: [name: string]
  delete: [name: string]
}>()

function getChannelTypeName(channel: string): string {
  const type = props.channelTypes.find(t => t.type === channel)
  return type?.name || channel
}
</script>
