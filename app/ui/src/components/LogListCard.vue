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
import DOMPurify from 'dompurify'

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
    case 'docker': return 'Docker容器日志'
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
  const html = text.replace(regex, '<mark class="highlight">$1</mark>')
  // 使用DOMPurify清理HTML，防止XSS攻击
  return DOMPurify.sanitize(html)
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
  width: 900px;
  max-width: 95%;
  height: 100%;
  background: var(--card-bg);
  box-shadow: -4px 0 20px rgba(0, 0, 0, 0.15);
  display: flex;
  flex-direction: column;
  animation: slideIn var(--transition-slow) ease;
}

@keyframes slideIn {
  from { transform: translateX(100%); }
  to { transform: translateX(0); }
}

.drawer-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: var(--spacing-md) var(--spacing-xl);
  background: var(--primary-gradient);
  color: white;
}

.drawer-header h3 {
  margin: 0;
  font-size: 1rem;
  font-weight: 500;
  white-space: nowrap;
}

.close-btn {
  background: none;
  border: none;
  color: white;
  font-size: 1.5rem;
  cursor: pointer;
  padding: 0;
  line-height: 1;
  transition: opacity var(--transition-fast);
}

.close-btn:hover {
  opacity: 0.8;
}

.drawer-search {
  padding: var(--spacing-md) var(--spacing-xl);
  background: var(--bg-color-2);
  border-bottom: 1px solid var(--border-color);
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
}

.search-input {
  flex: 1;
  padding: var(--spacing-sm) var(--spacing-md);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-xs);
  font-size: 0.875rem;
  font-family: var(--font-family);
  background: var(--card-bg);
  color: var(--text-color-1);
  transition: border-color var(--transition-fast);
}

.search-input:focus {
  outline: none;
  border-color: var(--primary-color);
}

.search-input::placeholder {
  color: var(--text-color-3);
}

.clear-btn {
  background: var(--bg-color-3);
  border: none;
  color: var(--text-color-2);
  font-size: 1rem;
  cursor: pointer;
  padding: var(--spacing-xs) var(--spacing-sm);
  border-radius: var(--radius-xs);
  transition: all var(--transition-fast);
}

.clear-btn:hover {
  background: var(--primary-color);
  color: white;
}

.search-count {
  font-size: 0.75rem;
  color: var(--text-color-2);
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
  padding: var(--spacing-md) var(--spacing-xl);
  border-bottom: 1px solid var(--divider-color);
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-size: 0.8125rem;
  transition: background var(--transition-fast);
  gap: var(--spacing-md);
}

.log-item:hover {
  background-color: var(--bg-color-2);
}

.log-item.header {
  background: var(--bg-color-2);
  font-weight: 600;
  border-bottom: 2px solid var(--border-color);
  position: sticky;
  top: 0;
  color: var(--text-color-1);
}

.log-item.header .path {
  white-space: nowrap;
}

.log-item .path {
  font-family: 'Monaco', 'Menlo', 'Consolas', monospace;
  color: var(--text-color-1);
  flex: 1;
  min-width: 0;
  word-break: break-all;
  white-space: normal;
  line-height: 1.4;
}

.log-item .size {
  color: var(--primary-color);
  font-weight: 600;
  min-width: 90px;
  text-align: right;
  flex-shrink: 0;
}

.action-col {
  min-width: 120px;
  text-align: center;
  flex-shrink: 0;
}

.log-item .actions {
  display: flex;
  gap: var(--spacing-xs);
  flex-shrink: 0;
}

.log-item button {
  padding: 4px var(--spacing-sm);
  font-size: 0.75rem;
}

.no-results {
  padding: var(--spacing-3xl) var(--spacing-xl);
  text-align: center;
  color: var(--text-color-3);
  font-size: 0.875rem;
}

.highlight {
  background: rgba(255, 176, 0, 0.3);
  color: #B45309;
  padding: 0 2px;
  border-radius: 2px;
}

@media (max-width: 768px) {
  .drawer-overlay {
    align-items: flex-end;
  }

  .drawer {
    width: 100%;
    max-width: 100%;
    height: 85vh;
    border-radius: var(--radius-lg) var(--radius-lg) 0 0;
  }

  .drawer-header {
    padding: var(--spacing-sm) var(--spacing-lg);
  }

  .drawer-header h3 {
    font-size: 0.9375rem;
  }

  .close-btn {
    font-size: 1.125rem;
    width: 18px;
    padding: 0;
    margin-left: auto;
    overflow: hidden;
  }

  .drawer-search {
    padding: var(--spacing-xs) var(--spacing-lg);
  }

  .search-input {
    padding: var(--spacing-xs) var(--spacing-sm);
    font-size: 0.8125rem;
  }

  .clear-btn {
    padding: 4px var(--spacing-xs);
    font-size: 0.875rem;
  }

  .search-count {
    font-size: 0.6875rem;
  }

  .log-item {
    padding: var(--spacing-sm) var(--spacing-lg);
    flex-wrap: wrap;
  }

  .log-item .path {
    width: 100%;
    font-size: 0.75rem;
    margin-bottom: var(--spacing-xs);
    white-space: normal;
    word-break: break-all;
  }

  .log-item .size {
    font-size: 0.75rem;
    margin-left: 0;
  }

  .log-item .actions {
    width: 100%;
    margin-left: 0;
    margin-top: var(--spacing-sm);
    gap: var(--spacing-xs);
  }

  .log-item .actions button {
    flex: 1;
    padding: var(--spacing-xs) var(--spacing-sm);
    font-size: 0.75rem;
  }

  .log-item.header {
    display: none;
  }

  .no-results {
    padding: var(--spacing-2xl) var(--spacing-lg);
    font-size: 0.875rem;
  }
}

@media (max-width: 480px) {
  .drawer {
    height: 90vh;
  }

  .drawer-header {
    padding: var(--spacing-xs) var(--spacing-md);
  }

  .drawer-header h3 {
    font-size: 0.9375rem;
  }

  .drawer-search {
    padding: 4px var(--spacing-md);
    flex-wrap: wrap;
  }

  .search-input {
    padding: var(--spacing-xs) var(--spacing-xs);
    font-size: 0.875rem;
  }

  .log-item {
    padding: var(--spacing-xs) var(--spacing-md);
  }

  .log-item .path {
    font-size: 0.8125rem;
  }

  .log-item .size {
    font-size: 0.8125rem;
  }

  .log-item .actions button {
    padding: var(--spacing-xs) var(--spacing-xs);
    font-size: 0.8125rem;
  }
}
</style>
