<template>
  <div class="card">
    <h2>统计概览</h2>
    <div class="stats-grid">
      <div class="stat-card">
        <div class="value">{{ stats.totalLogs }}</div>
        <div class="label">日志文件</div>
      </div>
      <div class="stat-card">
        <div class="value">{{ stats.totalSize }}</div>
        <div class="label">总大小</div>
      </div>
      <div class="stat-card">
        <div class="value">{{ stats.archiveCount }}</div>
        <div class="label">归档文件</div>
      </div>
      <div class="stat-card">
        <div class="value">{{ stats.largeFiles }}</div>
        <div class="label">大文件</div>
      </div>
    </div>
  </div>
</template>

<script setup>
defineProps({
  stats: {
    type: Object,
    required: true
  }
})
</script>

<style>
.stats-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 12px;
  margin-top: 15px;
}

.stat-card {
  background: linear-gradient(135deg, var(--primary-color, #667eea) 0%, #764ba2 100%);
  color: white;
  padding: 15px;
  border-radius: 10px;
  text-align: center;
}

.stat-card .value {
  font-size: 24px;
  font-weight: 700;
  margin-bottom: 4px;
}

.stat-card .label {
  font-size: 12px;
  opacity: 0.9;
}

@media (max-width: 768px) {
  .stats-grid {
    grid-template-columns: repeat(2, 1fr);
    gap: 10px;
  }
  
  .stat-card {
    padding: 12px;
  }
  
  .stat-card .value {
    font-size: 20px;
  }
  
  .stat-card .label {
    font-size: 11px;
  }
}

@media (max-width: 480px) {
  .stat-card .value {
    font-size: 18px;
  }
}
</style>
