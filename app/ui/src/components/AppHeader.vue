<template>
  <header class="app-header">
    <div class="header-content">
      <div class="title-section">
        <h1>飞牛应用日志管理</h1>
        <div class="version">
          <span class="version-text">版本: {{ appVersion }}</span>
          <span v-if="updateInfo" class="version-badge" title="有新版本可用"></span>
        </div>
      </div>
    </div>
  </header>
</template>

<script setup lang="ts">
import { useUpdate } from '../composables/useUpdate'

const { appVersion, updateInfo } = useUpdate()
</script>

<style scoped>
.app-header {
  color: var(--text-color-on-primary);
  padding: var(--spacing-2xl);
  border-radius: var(--radius-lg);
  margin-bottom: var(--spacing-xl);
  position: relative;
  overflow: hidden;
  background: var(--primary-gradient);
  box-shadow: var(--depth-3), 0 0 30px var(--glow-primary-soft);
  transition: background var(--transition-base), box-shadow var(--transition-base);
}

.app-header::before {
  content: '';
  position: absolute;
  inset: 0;
  background: var(--glass-shine);
  pointer-events: none;
  border-radius: inherit;
}

/* 鸿蒙 7.0 液态高光装饰 */
.app-header::after {
  content: '';
  position: absolute;
  top: -50%;
  right: -10%;
  width: 60%;
  height: 200%;
  background: radial-gradient(circle, rgba(255, 255, 255, 0.25) 0%, transparent 70%);
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
  font-weight: var(--font-weight-semibold);
  letter-spacing: var(--letter-spacing-tight);
  line-height: 1.3;
  color: var(--text-color-on-primary);
  text-shadow: 0 1px 2px rgba(0,0,0,0.12);
}

.version {
  display: inline-flex;
  align-items: center;
  gap: var(--spacing-xs);
  font-size: var(--font-size-sm);
  font-weight: 400;
  opacity: 0.8;
  position: relative;
}

.version-text {
  transition: transform var(--transition-base), opacity var(--transition-base);
}

.version-text:hover {
  transform: scale(1.05);
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
  .app-header {
    padding: var(--spacing-xl);
    border-radius: var(--radius-md);
  }

  h1 {
    font-size: var(--font-size-2xl);
  }
}

@media (max-width: 480px) {
  .app-header {
    padding: var(--spacing-lg);
    border-radius: var(--radius-md);
  }

  h1 {
    font-size: var(--font-size-xl);
  }

  .version {
    font-size: var(--font-size-xs);
  }
}
</style>
