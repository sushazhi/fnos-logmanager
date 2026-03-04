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
  gap: var(--spacing-md);
  margin-top: var(--spacing-lg);
}

.stat-card {
  background: var(--primary-color);
  color: white;
  padding: var(--spacing-lg);
  border-radius: var(--radius-sm);
  text-align: center;
  box-shadow: var(--shadow-sm);
  transition: transform var(--transition-fast), box-shadow var(--transition-fast);
}

.stat-card:hover {
  transform: translateY(-2px);
  box-shadow: var(--shadow-md);
}

.stat-card .value {
  font-size: 1.5rem;
  font-weight: 600;
  margin-bottom: var(--spacing-xs);
  letter-spacing: -0.02em;
}

.stat-card .label {
  font-size: 0.75rem;
  font-weight: 400;
  opacity: 0.9;
}

@media (max-width: 768px) {
  .stats-grid {
    grid-template-columns: repeat(2, 1fr);
    gap: var(--spacing-sm);
  }

  .stat-card {
    padding: var(--spacing-md);
  }

  .stat-card .value {
    font-size: 1.25rem;
  }

  .stat-card .label {
    font-size: 0.6875rem;
  }
}

@media (max-width: 480px) {
  .stat-card .value {
    font-size: 1.125rem;
  }
}
</style>
