<template>
  <div class="card">
    <h2>操作功能</h2>
    
    <div class="actions-row">
      <button @click="$emit('listLogs')" title="列出所有日志文件">
        列出日志
      </button>
      <button @click="$emit('showSearch')" title="按大小或名称查找日志文件">
        查找日志
      </button>
      <button @click="$emit('listArchives')" title="查看压缩归档的日志文件">
        归档日志
      </button>
      <button @click="$emit('listDocker')" title="查看Docker容器日志">
        Docker日志
      </button>
    </div>
    
    <div class="actions-row">
      <button class="danger" @click="$emit('showClean')" title="清空或删除旧日志文件">
        清理
      </button>
      <button @click="$emit('backup')" title="备份日志到指定目录">
        备份
      </button>
      <button @click="$emit('refresh')" title="刷新统计数据">
        刷新
      </button>
      <button @click="$emit('openSettings')" title="显示设置">
        设置
      </button>
    </div>
    
    <div class="filter-section">
      <div class="filter-toggle">
        <span class="filter-label">敏感信息过滤</span>
        <label class="switch">
          <input 
            type="checkbox" 
            :checked="filterEnabled"
            @change="$emit('toggleFilter')"
          >
          <span class="slider"></span>
        </label>
        <span class="filter-status">{{ filterEnabled ? '已启用' : '已禁用' }}</span>
      </div>
    </div>
    
    <div class="status" :class="status.type">
      <span class="status-icon">{{ statusIcons[status.type] }}</span>
      <span>{{ status.message }}</span>
    </div>
  </div>
</template>

<script setup>
defineProps({
  status: {
    type: Object,
    required: true
  },
  filterEnabled: {
    type: Boolean,
    default: true
  }
})

defineEmits([
  'refresh',
  'listLogs',
  'showSearch',
  'showClean',
  'backup',
  'listArchives',
  'listDocker',
  'toggleFilter',
  'openSettings'
])

const statusIcons = {
  success: '✓',
  error: '✗',
  warning: '⚠',
  loading: '⟳'
}
</script>

<style>
.actions-row {
  display: flex;
  gap: var(--spacing-sm);
  margin-top: var(--spacing-lg);
}

.actions-row:first-of-type {
  margin-top: var(--spacing-lg);
}

button {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: var(--spacing-xs);
  padding: var(--spacing-md) var(--spacing-sm);
  background: var(--primary-gradient);
  color: white;
  font-size: 0.875rem;
  font-weight: 500;
  border-radius: var(--radius-sm);
  cursor: pointer;
  transition: all var(--transition-fast);
  box-shadow: var(--shadow-sm);
}

button:hover {
  transform: translateY(-2px);
  box-shadow: var(--shadow-md);
}

button:active {
  transform: scale(0.98);
}

button.danger {
  background: var(--error-color);
}

button.danger:hover {
  background: #E52629;
}

.filter-section {
  margin-top: var(--spacing-lg);
  padding: var(--spacing-md);
  background: var(--bg-color-2);
  border-radius: var(--radius-sm);
}

.filter-toggle {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
}

.filter-label {
  font-weight: 500;
  font-size: 0.875rem;
  color: var(--text-color-1);
}

.switch {
  position: relative;
  display: inline-block;
  width: 44px;
  height: 22px;
}

.switch input {
  opacity: 0;
  width: 0;
  height: 0;
}

.slider {
  position: absolute;
  cursor: pointer;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background-color: var(--bg-color-4);
  transition: var(--transition-base);
  border-radius: 22px;
}

.slider::before {
  position: absolute;
  content: "";
  height: 18px;
  width: 18px;
  left: 2px;
  bottom: 2px;
  background-color: white;
  transition: var(--transition-base);
  border-radius: 50%;
  box-shadow: var(--shadow-xs);
}

input:checked + .slider {
  background: var(--primary-color);
}

input:checked + .slider::before {
  transform: translateX(22px);
}

.filter-status {
  font-size: 0.75rem;
  font-weight: 400;
  color: var(--text-color-2);
}

.status {
  margin-top: var(--spacing-lg);
  padding: var(--spacing-sm) var(--spacing-md);
  border-radius: var(--radius-sm);
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
  font-size: 0.875rem;
  font-weight: 400;
}

.status.success {
  background: rgba(0, 186, 173, 0.1);
  color: var(--success-color);
}

.status.error {
  background: rgba(250, 42, 45, 0.1);
  color: var(--error-color);
}

.status.warning {
  background: rgba(255, 176, 0, 0.1);
  color: var(--warning-color);
}

.status.loading {
  background: rgba(0, 125, 255, 0.1);
  color: var(--info-color);
}

.status-icon {
  font-size: 0.875rem;
}

@media (max-width: 768px) {
  .actions-row {
    flex-wrap: wrap;
  }

  button {
    padding: var(--spacing-sm) var(--spacing-xs);
    font-size: 0.8125rem;
  }

  .filter-label {
    font-size: 0.8125rem;
  }
}

@media (max-width: 480px) {
  .actions-row {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: var(--spacing-xs);
  }

  button {
    width: 100%;
  }
}
</style>
