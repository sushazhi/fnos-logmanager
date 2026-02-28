<template>
  <Teleport to="body">
    <div class="drawer-overlay" v-if="logs.length > 0" @click.self="$emit('close')">
      <div class="drawer">
        <div class="drawer-header">
          <h3>{{ title }}</h3>
          <button class="close-btn" @click="$emit('close')">×</button>
        </div>
        <div class="drawer-search">
          <input 
            type="text" 
            v-model="searchQuery" 
            placeholder="搜索文件名..."
            class="search-input"
          >
          <button class="clear-btn" v-if="searchQuery" @click="searchQuery = ''" title="清除搜索">×</button>
          <span class="search-count" v-if="searchQuery">
            {{ filteredLogs.length }} / {{ logs.length }}
          </span>
        </div>
        <div class="drawer-body">
          <div class="log-list">
            <div class="log-item header">
              <span class="path">{{ headerLabels.path }}</span>
              <span class="size">{{ headerLabels.size }}</span>
              <span class="action-col">操作</span>
            </div>
            <div 
              v-for="(log, index) in filteredLogs" 
              :key="index"
              class="log-item"
            >
              <span class="path" :title="log.path">
                <template v-if="searchQuery">
                  <span v-html="highlightText(log.path, searchQuery)"></span>
                </template>
                <template v-else>{{ log.path }}</template>
              </span>
              <span class="size">{{ log.sizeFormatted }}</span>
              <div class="actions">
                <button 
                  class="secondary small" 
                  @click="$emit('view', log.path)"
                  v-if="!log.isDocker && type !== 'archives'"
                >
                  查看
                </button>
                <button 
                  class="secondary small" 
                  @click="$emit('viewDocker', log.path)"
                  v-if="log.isDocker"
                >
                  查看日志
                </button>
                <button 
                  class="secondary small" 
                  @click="$emit('viewArchive', log.path)"
                  v-if="type === 'archives'"
                >
                  查看
                </button>
                <button 
                  class="danger small" 
                  @click="$emit('truncate', log.path)"
                  v-if="!log.isDocker && type !== 'archives'"
                >
                  清空
                </button>
                <button 
                  class="danger small" 
                  @click="$emit('delete', log.path)"
                  v-if="type === 'archives'"
                >
                  删除
                </button>
                <button 
                  class="danger small" 
                  @click="handleDeleteLog(log.path)"
                  v-if="!log.isDocker && type !== 'archives' && log.canDelete"
                  title="应用已卸载，可删除日志文件"
                >
                  删除
                </button>
              </div>
            </div>
            <div v-if="filteredLogs.length === 0 && logs.length > 0" class="no-results">
              未找到匹配的结果
            </div>
          </div>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<script setup>
import { ref, computed } from 'vue'

const props = defineProps({
  logs: {
    type: Array,
    default: () => []
  },
  type: {
    type: String,
    default: 'logs'
  }
})

const emit = defineEmits(['view', 'truncate', 'viewDocker', 'viewArchive', 'delete', 'delete-log', 'close'])

function handleDeleteLog(path) {
  emit('delete-log', path)
}

const searchQuery = ref('')

const title = computed(() => {
  switch (props.type) {
    case 'docker': return 'Docker容器'
    case 'archives': return '归档日志'
    default: return '结果列表'
  }
})

const headerLabels = computed(() => {
  switch (props.type) {
    case 'docker': 
      return { path: '容器名称', size: '镜像' }
    case 'archives': 
      return { path: '文件路径', size: '大小' }
    default: 
      return { path: '文件路径', size: '大小' }
  }
})

const filteredLogs = computed(() => {
  if (!searchQuery.value.trim()) {
    return props.logs
  }
  const query = searchQuery.value.toLowerCase()
  return props.logs.filter(log => {
    const path = (log.path || '').toLowerCase()
    const size = (log.sizeFormatted || '').toLowerCase()
    return path.includes(query) || size.includes(query)
  })
})

function highlightText(text, query) {
  if (!query || !text) return text
  const regex = new RegExp(`(${escapeRegex(query)})`, 'gi')
  return text.replace(regex, '<mark class="highlight">$1</mark>')
}

function escapeRegex(string) {
  return string.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
}
</script>

<style scoped>
.drawer-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(0, 0, 0, 0.5);
  z-index: 1000;
  display: flex;
  justify-content: flex-end;
}

.drawer {
  width: 600px;
  max-width: 90%;
  height: 100%;
  background: var(--card-bg, white);
  box-shadow: -4px 0 20px rgba(0, 0, 0, 0.15);
  display: flex;
  flex-direction: column;
  animation: slideIn 0.3s ease;
}

@keyframes slideIn {
  from { transform: translateX(100%); }
  to { transform: translateX(0); }
}

.drawer-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 15px 20px;
  background: linear-gradient(135deg, var(--primary-color, #667eea) 0%, #764ba2 100%);
  color: white;
}

.drawer-header h3 {
  margin: 0;
  font-size: 16px;
}

.close-btn {
  background: none;
  border: none;
  color: white;
  font-size: 24px;
  cursor: pointer;
  padding: 0;
  line-height: 1;
}

.drawer-search {
  padding: 12px 20px;
  background: var(--bg-color, #f5f7fa);
  border-bottom: 1px solid var(--border-color, #e0e0e0);
  display: flex;
  align-items: center;
  gap: 10px;
}

.search-input {
  flex: 1;
  padding: 10px 15px;
  border: 1px solid var(--border-color, #ddd);
  border-radius: 8px;
  font-size: 14px;
  background: var(--card-bg, white);
  color: var(--text-color, #333);
}

.search-input:focus {
  outline: none;
  border-color: var(--primary-color, #667eea);
}

.clear-btn {
  background: var(--border-color, #e0e0e0);
  border: none;
  color: var(--text-color, #666);
  font-size: 16px;
  cursor: pointer;
  padding: 8px 12px;
  border-radius: 6px;
}

.clear-btn:hover {
  background: var(--primary-color, #667eea);
  color: white;
}

.search-count {
  font-size: 12px;
  color: var(--text-secondary, #666);
  white-space: nowrap;
}

.drawer-body {
  flex: 1;
  overflow: auto;
  padding: 0;
}

.log-list {
  border: none;
}

.log-item {
  padding: 12px 20px;
  border-bottom: 1px solid var(--border-color, #f0f0f0);
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-size: 13px;
}

.log-item:hover {
  background-color: var(--bg-color, #f8f9fa);
}

.log-item.header {
  background: var(--bg-color, #f0f2f5);
  font-weight: 600;
  border-bottom: 2px solid var(--border-color, #e0e0e0);
  position: sticky;
  top: 0;
}

.log-item .path {
  font-family: 'Monaco', 'Menlo', monospace;
  color: var(--text-color, #333);
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.log-item .size {
  color: var(--primary-color, #667eea);
  font-weight: 600;
  margin-left: 15px;
  min-width: 80px;
  text-align: right;
}

.action-col {
  width: 100px;
  text-align: center;
}

.log-item .actions {
  display: flex;
  gap: 5px;
  margin-left: 15px;
}

.log-item button {
  padding: 6px 12px;
  font-size: 12px;
}

.no-results {
  padding: 40px 20px;
  text-align: center;
  color: var(--text-secondary, #888);
}

.highlight {
  background: #fff3cd;
  padding: 0 2px;
  border-radius: 2px;
}

@media (max-width: 768px) {
  .drawer {
    width: 100%;
    max-width: 100%;
  }
  
  .log-item {
    padding: 10px 15px;
    flex-wrap: wrap;
  }
  
  .log-item .path {
    width: 100%;
    font-size: 11px;
    margin-bottom: 5px;
  }
  
  .log-item .size {
    font-size: 12px;
  }
  
  .log-item .actions {
    width: 100%;
    margin-left: 0;
    margin-top: 8px;
  }
  
  .log-item .actions button {
    flex: 1;
  }
}
</style>
