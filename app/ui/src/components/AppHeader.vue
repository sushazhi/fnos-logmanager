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

<script setup>
import { ref } from 'vue'

const appVersion = ref('__APP_VERSION__')
</script>

<style scoped>
header {
  background: linear-gradient(135deg,
    var(--card-color-1, #9b59b6) 0%,
    var(--card-color-2, #3498db) 33%,
    var(--card-color-3, #1abc9c) 66%,
    var(--card-color-4, #e74c8c) 100%
  );
  color: #4a4a4a;
  padding: var(--spacing-2xl);
  border-radius: var(--radius-md);
  margin-bottom: var(--spacing-xl);
  box-shadow: var(--shadow-md);
}

.header-content {
  display: flex;
  justify-content: center;
  align-items: center;
}

.title-section {
  text-align: center;
}

h1 {
  margin: 0 0 var(--spacing-xs) 0;
  font-size: 1.375rem;
  font-weight: 600;
  letter-spacing: -0.02em;
}

.version {
  font-size: 0.75rem;
  font-weight: 400;
  opacity: 0.85;
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
