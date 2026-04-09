<template>
  <div class="section">
    <div class="section-header">
      <h4>全局设置</h4>
      <label class="switch">
        <input type="checkbox" v-model="localSettings.enabled" @change="$emit('updateSettings')">
        <span class="slider"></span>
      </label>
    </div>
    <div class="setting-row">
      <label>检查间隔</label>
      <select v-model="localSettings.checkInterval" @change="$emit('updateSettings')" :disabled="!localSettings.enabled">
        <option :value="10000">10秒</option>
        <option :value="30000">30秒</option>
        <option :value="60000">1分钟</option>
        <option :value="300000">5分钟</option>
      </select>
    </div>
  </div>
</template>

<script setup lang="ts">
interface NotificationSettings {
  enabled: boolean
  checkInterval: number
  maxHistoryDays: number
  maxHistoryCount: number
}

const localSettings = defineModel<NotificationSettings>('settings', { required: true })

defineEmits<{
  updateSettings: []
}>()
</script>
