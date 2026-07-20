<template>
  <div class="hm-overlay-base" @click.self="$emit('close')">
    <SettingsPanel 
      @close="$emit('close')" 
      @update="onUpdate" 
      @show-audit="onShowAudit" 
      @show-notification="onShowNotification"
      @show-event-logger="onShowEventLogger"
    />
  </div>
  <EventLoggerPanel v-if="showEventLogger" @close="showEventLogger = false" />
</template>

<script setup lang="ts">
import { ref } from 'vue'
import SettingsPanel from './SettingsPanel.vue'
import EventLoggerPanel from './EventLoggerPanel.vue'

const emit = defineEmits<{
  close: []
  showAudit: []
  showNotification: []
  showEventLogger: []
}>()

const showEventLogger = ref(false)

function onUpdate(_settings: unknown): void {
  // 设置更新处理
}

function onShowAudit(): void {
  emit('showAudit')
}

function onShowNotification(): void {
  emit('showNotification')
}

function onShowEventLogger(): void {
  showEventLogger.value = true
}
</script>

<style scoped>

</style>
