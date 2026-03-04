<template>
  <div class="card">
    <div class="header-row">
      <h2>日志目录</h2>
      <button class="config-btn" @click="showConfig = !showConfig" title="配置">
        设置
      </button>
    </div>
    
    <div class="config-panel" v-if="showConfig">
      <p class="config-hint">选择要展示的目录：</p>
      <div class="dir-checkboxes">
        <label v-for="dir in allDirs" :key="dir.path" class="dir-checkbox">
          <input 
            type="checkbox" 
            :checked="visibleDirs.includes(dir.path)"
            @change="toggleDir(dir.path)"
          >
          <span>{{ dir.displayName }}</span>
          <span class="dir-status" :class="{ exists: dir.exists, 'not-exists': !dir.exists }">
            {{ dir.exists ? '√' : '×' }}
          </span>
        </label>
      </div>
    </div>
    
    <div class="log-dirs">
      <div 
        v-for="dir in displayedDirs" 
        :key="dir.path" 
        class="log-dir-item"
        :class="{ error: !dir.exists, active: selectedDir === dir.path }"
        @click="handleDirClick(dir)"
      >
        <h3>{{ dir.displayName }}</h3>
        <div class="stats">
          <div class="stat">
            <span class="stat-value">{{ dir.logCount || 0 }}</span>
            <span>日志</span>
          </div>
          <div class="stat">
            <span class="stat-value">{{ dir.archiveCount || 0 }}</span>
            <span>归档</span>
          </div>
          <div class="stat">
            <span class="stat-value">{{ dir.totalSize || '0B' }}</span>
            <span>大小</span>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'

const props = defineProps({
  dirs: {
    type: Array,
    default: () => []
  },
  selectedDir: {
    type: String,
    default: null
  }
})

const emit = defineEmits(['selectDir'])

const STORAGE_KEY = 'logmanager_visible_dirs'

const showConfig = ref(false)
const visibleDirs = ref([])

const dirNames = {
  '/vol1/@appdata': '@appdata',
  '/vol1/@appconf': '@appconf',
  '/vol1/@apphome': '@apphome',
  '/vol1/@apptemp': '@apptemp',
  '/vol1/@appshare': '@appshare',
  '/var/log/apps': '/var/log/apps'
}

const allDirs = computed(() => {
  if (!props.dirs || !Array.isArray(props.dirs)) return []
  return props.dirs.map(dir => ({
    ...dir,
    displayName: dirNames[dir.path] || dir.path
  }))
})

const displayedDirs = computed(() => {
  const dirs = allDirs.value
  if (!dirs || dirs.length === 0) return []
  
  if (visibleDirs.value.length === 0) {
    return dirs.filter(d => d && d.exists)
  }
  return dirs.filter(d => d && visibleDirs.value.includes(d.path))
})

function toggleDir(path) {
  if (!path) return
  const index = visibleDirs.value.indexOf(path)
  if (index > -1) {
    visibleDirs.value.splice(index, 1)
  } else {
    visibleDirs.value.push(path)
  }
  saveVisibleDirs()
}

function handleDirClick(dir) {
  if (!dir || !dir.exists) return
  emit('selectDir', dir.path)
}

function saveVisibleDirs() {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(visibleDirs.value))
  } catch (e) {}
}

function loadVisibleDirs() {
  try {
    const saved = localStorage.getItem(STORAGE_KEY)
    if (saved) {
      visibleDirs.value = JSON.parse(saved)
    }
  } catch (e) {
    console.warn('Failed to load visible directories:', e)
    localStorage.removeItem(STORAGE_KEY)
  }
}

onMounted(() => {
  loadVisibleDirs()
})
</script>

<style>
.header-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: var(--spacing-sm);
  gap: var(--spacing-sm);
}

.header-row h2 {
  margin: 0;
  flex: 1;
}

.config-btn {
  background: var(--bg-color-2);
  border: none;
  padding: var(--spacing-xs) var(--spacing-sm);
  border-radius: var(--radius-xs);
  cursor: pointer;
  font-size: 0.875rem;
  color: var(--text-color-1);
  flex: none;
  width: auto;
  min-width: 0;
  flex-shrink: 0;
  transition: all var(--transition-fast);
}

.config-btn:hover {
  background: var(--primary-color);
  color: white;
}

.config-btn:active {
  transform: scale(0.95);
}

.config-panel {
  background: var(--bg-color-2);
  padding: var(--spacing-md);
  border-radius: var(--radius-sm);
  margin-bottom: var(--spacing-lg);
}

.config-hint {
  margin: 0 0 var(--spacing-sm) 0;
  font-size: 0.8125rem;
  color: var(--text-color-2);
}

.dir-checkboxes {
  display: flex;
  flex-wrap: wrap;
  gap: var(--spacing-sm);
}

.dir-checkbox {
  display: flex;
  align-items: center;
  gap: var(--spacing-xs);
  padding: var(--spacing-xs) var(--spacing-sm);
  background: var(--card-bg);
  border-radius: var(--radius-xs);
  font-size: 0.8125rem;
  cursor: pointer;
  color: var(--text-color-1);
  transition: all var(--transition-fast);
}

.dir-checkbox:hover {
  background: var(--bg-color-2);
}

.dir-status {
  font-size: 0.6875rem;
  padding: 2px var(--spacing-xs);
  border-radius: var(--radius-xs);
  font-weight: 500;
}

.dir-status.exists {
  color: var(--success-color);
  background: rgba(0, 186, 173, 0.1);
}

.dir-status.not-exists {
  color: var(--error-color);
  background: rgba(250, 42, 45, 0.1);
}

.log-dirs {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: var(--spacing-sm);
}

.log-dir-item {
  background: var(--bg-color-2);
  padding: var(--spacing-sm);
  border-radius: var(--radius-sm);
  border-left: 4px solid var(--primary-color);
  cursor: pointer;
  transition: all var(--transition-fast);
}

.log-dir-item:hover {
  transform: translateY(-2px);
  box-shadow: var(--shadow-md);
}

.log-dir-item:active {
  transform: translateY(0);
}

.log-dir-item.active {
  background: var(--bg-color-3);
  border-left-color: var(--primary-hover);
}

.log-dir-item.error {
  border-left-color: var(--error-color);
  background: rgba(250, 42, 45, 0.1);
  cursor: not-allowed;
}

.log-dir-item.error:hover {
  transform: none;
  box-shadow: none;
}

.log-dir-item h3 {
  margin: 0 0 var(--spacing-xs) 0;
  font-size: 0.8125rem;
  font-weight: 500;
  color: var(--text-color-1);
}

.log-dir-item .stats {
  display: flex;
  gap: var(--spacing-sm);
}

.log-dir-item .stat {
  text-align: center;
}

.log-dir-item .stat-value {
  display: block;
  font-size: 0.875rem;
  font-weight: 600;
  color: var(--primary-color);
}

.log-dir-item .stat span:last-child {
  font-size: 0.6875rem;
  color: var(--text-color-3);
}

@media (max-width: 768px) {
  .log-dirs {
    grid-template-columns: repeat(2, 1fr);
    gap: var(--spacing-xs);
  }

  .log-dir-item {
    padding: var(--spacing-xs);
  }

  .log-dir-item h3 {
    font-size: 0.75rem;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .log-dir-item .stat-value {
    font-size: 0.8125rem;
  }

  .log-dir-item .stat span:last-child {
    font-size: 0.625rem;
  }

  .dir-checkboxes {
    flex-direction: column;
    gap: var(--spacing-xs);
  }
}

@media (max-width: 480px) {
  .log-dirs {
    grid-template-columns: repeat(2, 1fr);
    gap: 4px;
  }

  .log-dir-item {
    padding: 4px;
  }

  .log-dir-item h3 {
    font-size: 0.6875rem;
  }

  .log-dir-item .stats {
    gap: 4px;
  }

  .log-dir-item .stat-value {
    font-size: 0.75rem;
  }

  .log-dir-item .stat span:last-child {
    font-size: 0.625rem;
  }
}
</style>
