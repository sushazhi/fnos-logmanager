<template>
  <header>
    <div class="header-content">
      <div class="title-section">
        <h1>飞牛应用日志管理</h1>
        <div class="version" :class="{ 'has-update': !!updateInfo }" @click="handleVersionClick">
          <span class="version-text">版本: {{ appVersion }}</span>
          <span v-if="updateInfo" class="version-badge" title="有新版本可用"></span>
        </div>
      </div>
    </div>
  </header>
</template>

<script setup lang="ts">
import { useUpdate } from '../composables/useUpdate'

const emit = defineEmits<{
  showUpdate: []
}>()

const { appVersion, updateInfo } = useUpdate()

function handleVersionClick() {
  emit('showUpdate')
}
</script>

<style scoped>
header {
  background: var(--primary-gradient);
  color: #fff;
  padding: var(--spacing-2xl);
  border-radius: var(--radius-md);
  margin-bottom: var(--spacing-xl);
  box-shadow: var(--shadow-lg);
  position: relative;
  overflow: hidden;
}

header::before {
  content: '';
  position: absolute;
  inset: 0;
  background: radial-gradient(ellipse at 20% 50%, rgba(255, 255, 255, 0.08) 0%, transparent 60%);
  pointer-events: none;
}

header::after {
  content: '';
  position: absolute;
  top: -30%;
  right: -15%;
  width: 300px;
  height: 300px;
  background: radial-gradient(circle, rgba(255, 255, 255, 0.04) 0%, transparent 70%);
  border-radius: 50%;
  pointer-events: none;
}

.header-content {
  display: flex;
  justify-content: center;
  align-items: center;
  position: relative;
  z-index: 1;
}

.title-section {
  text-align: center;
}

h1 {
  margin: 0 0 var(--spacing-xs) 0;
  font-size: var(--font-size-4xl);
  font-weight: 600;
  letter-spacing: -0.02em;
  line-height: 1.3;
}

.version {
  display: inline-flex;
  align-items: center;
  gap: var(--spacing-xs);
  font-size: var(--font-size-sm);
  font-weight: 400;
  opacity: 0.8;
  cursor: pointer;
  transition: opacity var(--transition-fast);
  position: relative;
}

.version:hover {
  opacity: 1;
}

.version-badge {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--warning-color);
  animation: badge-pulse 2s infinite;
  flex-shrink: 0;
}

@keyframes badge-pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.4; }
}

@media (max-width: 768px) {
  header {
    padding: var(--spacing-xl);
    border-radius: var(--radius-sm);
  }

  h1 {
    font-size: var(--font-size-2xl);
  }
}

@media (max-width: 480px) {
  header {
    padding: var(--spacing-lg);
  }

  h1 {
    font-size: var(--font-size-xl);
  }

  .version {
    font-size: var(--font-size-xs);
  }
}
</style>
