<template>
  <header>
    <div class="header-content">
      <div class="title-section">
        <h1>飞牛应用日志管理</h1>
        <div class="version">版本: {{ appVersion }}</div>
      </div>
    </div>
  </header>
</template>

<script setup lang="ts">
import { ref } from 'vue'

const appVersion = ref<string>('__APP_VERSION__')
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
  font-size: 1.375rem;
  font-weight: 600;
  letter-spacing: -0.02em;
  line-height: 1.3;
}

.version {
  font-size: 0.75rem;
  font-weight: 400;
  opacity: 0.8;
}

@media (max-width: 768px) {
  header {
    padding: var(--spacing-xl);
    border-radius: var(--radius-sm);
  }

  h1 {
    font-size: 1.125rem;
  }
}

@media (max-width: 480px) {
  header {
    padding: var(--spacing-lg);
  }

  h1 {
    font-size: 1rem;
  }

  .version {
    font-size: 0.6875rem;
  }
}
</style>
