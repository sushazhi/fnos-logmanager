<template>
  <div class="modal-overlay" @click.self="$emit('close')">
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
.modal-overlay {
  position: fixed;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  background: rgba(0, 0, 0, 0.5);
  display: flex;
  justify-content: center;
  align-items: center;
  z-index: 1200;
  padding: var(--spacing-xl);
}

@media (max-width: 768px) {
  .modal-overlay {
    padding: var(--spacing-sm);
    align-items: flex-end;
  }
}

@media (max-width: 480px) {
  .modal-overlay {
    padding: 0;
    align-items: flex-end;
  }
}
</style>
