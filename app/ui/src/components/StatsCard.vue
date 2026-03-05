<template>
  <div class="card">
    <h2>统计概览</h2>
    <div class="stats-grid">
      <div class="stat-card stat-card-blue">
        <div class="stat-icon">
          <!-- 鸿蒙6图标: 文档 -->
          <svg width="24" height="24" viewBox="0 0 24 24" fill="none">
            <path d="M14 2H6C4.89543 2 4 2.89543 4 4V20C4 21.1046 4.89543 22 6 22H18C19.1046 22 20 21.1046 20 20V8L14 2Z" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/>
            <path d="M14 2V8H20" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/>
            <path d="M8 13H16" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/>
            <path d="M8 17H16" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/>
          </svg>
        </div>
        <div class="stat-content">
          <div class="value">{{ stats.totalLogs }}</div>
          <div class="label">日志文件</div>
        </div>
      </div>
      <div class="stat-card stat-card-green">
        <div class="stat-icon">
          <!-- 鸿蒙6图标: 存储/数据库 -->
          <svg width="24" height="24" viewBox="0 0 24 24" fill="none">
            <ellipse cx="12" cy="5" rx="9" ry="3" stroke="currentColor" stroke-width="1.5"/>
            <path d="M21 12C21 13.66 16.9706 15 12 15C7.02944 15 3 13.66 3 12" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/>
            <path d="M3 5V19C3 20.66 7.02944 22 12 22C16.9706 22 21 20.66 21 19V5" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/>
          </svg>
        </div>
        <div class="stat-content">
          <div class="value">{{ stats.totalSize }}</div>
          <div class="label">总大小</div>
        </div>
      </div>
      <div class="stat-card stat-card-orange">
        <div class="stat-icon">
          <!-- 鸿蒙6图标: 归档/压缩包 -->
          <svg width="24" height="24" viewBox="0 0 24 24" fill="none">
            <path d="M21 8V21H3V8" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/>
            <rect x="1" y="3" width="22" height="5" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/>
            <path d="M10 12H14" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/>
          </svg>
        </div>
        <div class="stat-content">
          <div class="value">{{ stats.archiveCount }}</div>
          <div class="label">归档文件</div>
        </div>
      </div>
      <div class="stat-card stat-card-red">
        <div class="stat-icon">
          <!-- 鸿蒙6图标: 大文件/警告 -->
          <svg width="24" height="24" viewBox="0 0 24 24" fill="none">
            <path d="M13 2H6C4.89543 2 4 2.89543 4 4V20C4 21.1046 4.89543 22 6 22H18C19.1046 22 20 21.1046 20 20V9L13 2Z" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/>
            <path d="M13 2V9H20" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/>
          </svg>
        </div>
        <div class="stat-content">
          <div class="value">{{ stats.largeFiles }}</div>
          <div class="label">大文件</div>
        </div>
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
  position: relative;
  padding: var(--spacing-lg);
  border-radius: var(--radius-md);
  box-shadow: var(--shadow-sm);
  transition: transform var(--transition-fast), box-shadow var(--transition-fast);
  overflow: hidden;
  display: flex;
  align-items: center;
  gap: var(--spacing-md);
}

.stat-card:hover {
  transform: translateY(-2px);
  box-shadow: var(--shadow-md);
}

.stat-card::before {
  content: '';
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  opacity: 0.1;
  transition: opacity var(--transition-fast);
}

.stat-card:hover::before {
  opacity: 0.15;
}

/* 渐变色卡片 */
.stat-card-blue {
  background: linear-gradient(135deg,
    var(--card-color-1, #9b59b6) 0%,
    var(--card-color-1-light, #b07cc6) 50%,
    var(--card-color-1, #9b59b6) 100%
  );
  color: #4a4a4a;
}

.stat-card-green {
  background: linear-gradient(135deg,
    var(--card-color-2, #3498db) 0%,
    var(--card-color-2-light, #5dade2) 50%,
    var(--card-color-2, #3498db) 100%
  );
  color: #4a4a4a;
}

.stat-card-orange {
  background: linear-gradient(135deg,
    var(--card-color-3, #1abc9c) 0%,
    var(--card-color-3-light, #48c9b0) 50%,
    var(--card-color-3, #1abc9c) 100%
  );
  color: #4a4a4a;
}

.stat-card-red {
  background: linear-gradient(135deg,
    var(--card-color-4, #e74c8c) 0%,
    var(--card-color-4-light, #ec6a9d) 50%,
    var(--card-color-4, #e74c8c) 100%
  );
  color: #4a4a4a;
}

.stat-card-blue::before,
.stat-card-green::before,
.stat-card-orange::before,
.stat-card-red::before {
  background: linear-gradient(135deg, rgba(255, 255, 255, 0.3) 0%, transparent 100%);
}

.stat-icon {
  flex-shrink: 0;
  width: 48px;
  height: 48px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(255, 255, 255, 0.5);
  border-radius: var(--radius-sm);
  backdrop-filter: blur(10px);
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.08);
}

.stat-icon svg {
  width: 24px;
  height: 24px;
  color: #2D3748;
}

.stat-content {
  flex: 1;
  min-width: 0;
}

.stat-card .value {
  font-size: 1.5rem;
  font-weight: 600;
  margin-bottom: var(--spacing-xs);
  letter-spacing: -0.02em;
  line-height: 1.2;
}

.stat-card .label {
  font-size: 0.75rem;
  font-weight: 400;
  opacity: 0.9;
  white-space: nowrap;
}

@media (max-width: 768px) {
  .stats-grid {
    grid-template-columns: repeat(2, 1fr);
    gap: var(--spacing-sm);
  }

  .stat-card {
    padding: var(--spacing-md);
    flex-direction: column;
    text-align: center;
    gap: var(--spacing-sm);
  }

  .stat-icon {
    width: 40px;
    height: 40px;
  }

  .stat-icon svg {
    width: 20px;
    height: 20px;
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

  .stat-icon {
    width: 36px;
    height: 36px;
  }

  .stat-icon svg {
    width: 18px;
    height: 18px;
  }
}
</style>
