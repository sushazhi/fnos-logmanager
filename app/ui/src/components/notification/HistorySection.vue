<template>
  <div class="section">
    <div class="section-header">
      <h4>通知历史</h4>
      <div class="history-actions">
        <select v-model="localFilter" @change="$emit('filterChange', localFilter)" class="filter-select">
          <option value="all">全部</option>
          <option value="success">成功</option>
          <option value="failed">失败</option>
        </select>
        <button class="clear-btn" @click="$emit('clear')" v-if="history.length > 0">清空</button>
      </div>
    </div>
    <div class="history-list" v-if="history.length > 0">
      <div class="history-item" v-for="item in history" :key="item.id">
        <div class="history-header">
          <span :class="['history-status', item.success ? 'success' : 'failed']">
            {{ item.success ? '成功' : '失败' }}
          </span>
          <span class="history-time">{{ formatTime(item.timestamp) }}</span>
        </div>
        <div class="history-body">
          <span class="history-channel">{{ item.channel }}</span>
          <span class="history-title">{{ item.title }}</span>
        </div>
      </div>
    </div>
    <div class="empty-hint" v-else>
      暂无通知历史
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'

interface HistoryItem {
  id: string
  success: boolean
  timestamp: string
  channel: string
  title: string
}

defineProps<{
  history: HistoryItem[]
}>()

defineEmits<{
  filterChange: [filter: string]
  clear: []
}>()

const localFilter = ref('all')

function formatTime(timestamp: string): string {
  try {
    const date = new Date(timestamp)
    return date.toLocaleString('zh-CN', {
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit'
    })
  } catch {
    return timestamp
  }
}
</script>
