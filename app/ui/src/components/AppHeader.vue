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
  background: linear-gradient(135deg, var(--primary-color, #667eea) 0%, #764ba2 100%);
  color: white;
  padding: 20px;
  border-radius: 12px;
  margin-bottom: 20px;
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
  margin: 0 0 5px 0;
  font-size: 22px;
}

.version {
  font-size: 12px;
  opacity: 0.8;
}

@media (max-width: 768px) {
  header {
    padding: 15px;
  }
  
  h1 {
    font-size: 18px;
  }
}
</style>
