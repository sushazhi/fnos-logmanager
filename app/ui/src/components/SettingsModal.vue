<template>
  <div class="modal-overlay" @click.self="$emit('close')">
    <SettingsPanel @close="$emit('close')" @update="onUpdate" @show-audit="onShowAudit" />
  </div>
</template>

<script setup>
import SettingsPanel from './SettingsPanel.vue'

const emit = defineEmits(['close', 'showAudit'])

function onUpdate(settings) {
}

function onShowAudit() {
  emit('showAudit')
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
