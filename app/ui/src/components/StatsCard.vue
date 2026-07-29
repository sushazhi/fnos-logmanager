<template>
  <div class="card glass-card">
    <h2>统计概览</h2>
    <div class="stats-grid">
      <div class="stat-card stat-card-blue">
        <div class="stat-icon">
          <!-- 鸿蒙6.1图标: 文档 -->
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
          <!-- 鸿蒙6.1图标: 存储/数据库 -->
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
          <!-- 鸿蒙6.1图标: 归档/压缩包 -->
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
          <!-- 鸿蒙6.1图标: 大文件/警告 -->
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

<script setup lang="ts">
import type { Stats } from '../types'

defineProps<{
  stats: Stats
}>()
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
  box-shadow: var(--depth-1);
  transition: transform var(--transition-spring), box-shadow var(--transition-spring);
  overflow: hidden;
  display: flex;
  align-items: center;
  gap: var(--spacing-md);
  perspective: var(--perspective-near);
}

.stat-card:hover {
  transform: perspective(var(--perspective-near)) translateY(var(--translate-hover)) rotateX(var(--rotate-subtle));
  box-shadow: var(--depth-2), 0 0 20px var(--glow-primary-soft);
}

.stat-card::before {
  content: '';
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  opacity: 0;
  transition: opacity var(--transition-base);
  background: var(--glass-shine);
  border-radius: inherit;
  pointer-events: none;
}

.stat-card::after {
  content: '';
  position: absolute;
  inset: 0;
  background: linear-gradient(180deg, rgba(255,255,255,0.12) 0%, transparent 40%, rgba(0,0,0,0.06) 100%);
  pointer-events: none;
  border-radius: inherit;
}

.stat-card:hover::before {
  opacity: 1;
}

.stat-card:active {
  transform: scale(0.97);
}

/* 渐变色卡片 */
.stat-card-blue {
  background: linear-gradient(135deg,
    var(--card-color-1) 0%,
    var(--card-color-1-light) 50%,
    var(--card-color-1) 100%
  );
  color: var(--text-color-on-primary);
}

.stat-card-green {
  background: linear-gradient(135deg,
    var(--card-color-2) 0%,
    var(--card-color-2-light) 50%,
    var(--card-color-2) 100%
  );
  color: var(--text-color-on-primary);
}

.stat-card-orange {
  background: linear-gradient(135deg,
    var(--card-color-3) 0%,
    var(--card-color-3-light) 50%,
    var(--card-color-3) 100%
  );
  color: var(--text-color-on-primary);
}

.stat-card-red {
  background: linear-gradient(135deg,
    var(--card-color-4) 0%,
    var(--card-color-4-light) 50%,
    var(--card-color-4) 100%
  );
  color: var(--text-color-on-primary);
}

.stat-card-blue::before,
.stat-card-green::before,
.stat-card-orange::before,
.stat-card-red::before {
  background: linear-gradient(135deg, var(--bg-color-3) 0%, transparent 100%);
}

.stat-icon {
  flex-shrink: 0;
  width: 48px;
  height: 48px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(255, 255, 255, 0.15);
  border-radius: var(--radius-sm);
  backdrop-filter: blur(12px);
  -webkit-backdrop-filter: blur(12px);
  box-shadow: var(--depth-1), inset 0 1px 0 rgba(255, 255, 255, 0.3);
  position: relative;
  overflow: hidden;
}

.stat-icon::after {
  content: '';
  position: absolute;
  inset: 0;
  background: linear-gradient(135deg, rgba(255,255,255,0.2) 0%, transparent 50%);
  pointer-events: none;
  border-radius: inherit;
}

.stat-icon svg {
  width: 24px;
  height: 24px;
  color: var(--text-color-on-primary);
  position: relative;
  z-index: 1;
  filter: drop-shadow(0 1px 2px rgba(0,0,0,0.15));
}

.stat-content {
  flex: 1;
  min-width: 0;
}

.stat-card .value {
  font-size: var(--font-size-5xl);
  font-weight: var(--font-weight-bold);
  margin-bottom: var(--spacing-xs);
  letter-spacing: var(--letter-spacing-tight);
  line-height: 1.2;
  text-shadow: 0 1px 3px rgba(0,0,0,0.1);
}

.stat-card .label {
  font-size: var(--font-size-sm);
  font-weight: var(--font-weight-regular);
  opacity: 0.9;
  white-space: nowrap;
}

.glass-card {
  border: 1px solid var(--glass-border);
  position: relative;
}

.glass-card::before {
  content: '';
  position: absolute;
  inset: 0;
  background: var(--glass-shine);
  pointer-events: none;
  border-radius: inherit;
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
    font-size: var(--font-size-4xl);
  }

  .stat-card .label {
    font-size: var(--font-size-xs);
  }
}

@media (max-width: 480px) {
  .stat-card .value {
    font-size: var(--font-size-3xl);
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
