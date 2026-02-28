<template>
  <div class="card">
    <h2>操作功能</h2>
    
    <div class="actions-row">
      <button @click="$emit('listLogs')" title="列出所有日志文件">
        <span>📋</span> 列出日志
      </button>
      <button @click="$emit('findLarge')" :title="`查找超过${largeThreshold}的日志文件`">
        <span>🔍</span> 大日志
      </button>
      <button @click="$emit('listArchives')" title="查看压缩归档的日志文件">
        <span>📂</span> 归档日志
      </button>
      <button @click="$emit('listDocker')" title="查看Docker容器日志">
        <span>🐳</span> Docker日志
      </button>
    </div>
    
    <div class="actions-row">
      <button class="danger" @click="$emit('showClean')" title="清空或删除旧日志文件">
        <span>🗑️</span> 清理
      </button>
      <button @click="$emit('compress')" title="压缩大日志文件节省空间">
        <span>📦</span> 压缩
      </button>
      <button @click="$emit('backup')" title="备份日志到指定目录">
        <span>💾</span> 备份
      </button>
      <button @click="$emit('refresh')" title="刷新统计数据">
        <span>🔄</span> 刷新
      </button>
    </div>
    
    <div class="filter-section">
      <div class="filter-toggle">
        <span class="filter-label">🔐 敏感信息过滤</span>
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
  },
  largeThreshold: {
    type: String,
    default: '10M'
  }
})

defineEmits([
  'refresh',
  'listLogs',
  'findLarge',
  'showClean',
  'compress',
  'backup',
  'listArchives',
  'listDocker',
  'toggleFilter'
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
  gap: 10px;
  margin-top: 15px;
}

.actions-row:first-of-type {
  margin-top: 15px;
}

button {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  padding: 12px 14px;
  border: none;
  border-radius: 8px;
  cursor: pointer;
  font-size: 0.9rem;
  font-weight: 500;
  background: linear-gradient(135deg, var(--primary-color, #667eea) 0%, #764ba2 100%);
  color: white;
  transition: all 0.3s ease;
}

button:hover {
  transform: translateY(-2px);
  box-shadow: 0 4px 12px rgba(102, 126, 234, 0.4);
}

button.danger {
  background: linear-gradient(135deg, #ff6b6b 0%, #ee5a5a 100%);
}

.filter-section {
  margin-top: 15px;
  padding: 12px;
  background: var(--bg-color, #f8f9fa);
  border-radius: 8px;
}

.filter-toggle {
  display: flex;
  align-items: center;
  gap: 10px;
}

.filter-label {
  font-weight: 500;
  font-size: 0.9rem;
  color: var(--text-color, #333);
}

.switch {
  position: relative;
  display: inline-block;
  width: 44px;
  height: 24px;
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
  background-color: #ccc;
  transition: .3s;
  border-radius: 24px;
}

.slider::before {
  position: absolute;
  content: "";
  height: 18px;
  width: 18px;
  left: 3px;
  bottom: 3px;
  background-color: white;
  transition: .3s;
  border-radius: 50%;
}

input:checked + .slider {
  background: linear-gradient(135deg, var(--primary-color, #667eea) 0%, #764ba2 100%);
}

input:checked + .slider::before {
  transform: translateX(20px);
}

.filter-status {
  font-size: 0.75rem;
  color: var(--text-secondary, #666);
}

.status {
  margin-top: 15px;
  padding: 10px 12px;
  border-radius: 8px;
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 0.9rem;
}

.status.success {
  background: linear-gradient(135deg, #e8f5e9 0%, #c8e6c9 100%);
  color: #2e7d32;
}

.status.error {
  background: linear-gradient(135deg, #ffebee 0%, #ffcdd2 100%);
  color: #c62828;
}

.status.warning {
  background: linear-gradient(135deg, #fff8e1 0%, #fff3e0 100%);
  color: #f57c00;
}

.status.loading {
  background: linear-gradient(135deg, #e3f2fd 0%, #e8eaf6 100%);
  color: #1565c0;
}

.status-icon {
  font-size: 0.9rem;
}

@media (max-width: 768px) {
  .actions-row {
    flex-wrap: wrap;
  }
  
  button {
    padding: 10px 12px;
    font-size: 0.85rem;
  }
  
  .filter-label {
    font-size: 0.85rem;
  }
}

@media (max-width: 480px) {
  .actions-row {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 8px;
  }
  
  button {
    width: 100%;
  }
}
</style>
