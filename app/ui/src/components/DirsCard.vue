<template>
  <div class="card">
    <div class="header-row">
      <h2>日志目录</h2>
      <button class="config-btn" @click="showConfig = !showConfig">
        ⚙️
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
  } catch (e) {}
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
  margin-bottom: 10px;
  gap: 10px;
}

.header-row h2 {
  margin: 0;
  flex: 1;
}

.config-btn {
  background: var(--border-color, #f0f2f5);
  border: none;
  padding: 6px 8px;
  border-radius: 6px;
  cursor: pointer;
  font-size: 14px;
  color: var(--text-color, #333);
  flex: none;
  width: auto;
  min-width: 0;
  flex-shrink: 0;
}

.config-btn:hover {
  background: var(--primary-color, #667eea);
  color: white;
}

.config-panel {
  background: var(--bg-color, #f8f9fa);
  padding: 12px;
  border-radius: 8px;
  margin-bottom: 15px;
}

.config-hint {
  margin: 0 0 10px 0;
  font-size: 0.85rem;
  color: var(--text-secondary, #666);
}

.dir-checkboxes {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
}

.dir-checkbox {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 6px 10px;
  background: var(--card-bg, white);
  border-radius: 6px;
  font-size: 0.85rem;
  cursor: pointer;
  color: var(--text-color, #333);
}

.dir-status {
  font-size: 0.7rem;
  padding: 2px 6px;
  border-radius: 4px;
}

.dir-status.exists {
  color: #4CAF50;
  background: #e8f5e9;
}

.dir-status.not-exists {
  color: #f44336;
  background: #ffebee;
}

.log-dirs {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 0.75rem;
}

.log-dir-item {
  background: var(--bg-color, #f8f9fa);
  padding: 0.75rem;
  border-radius: 8px;
  border-left: 4px solid var(--primary-color, #667eea);
  cursor: pointer;
  transition: all 0.2s ease;
}

.log-dir-item:hover {
  transform: translateY(-2px);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
}

.log-dir-item.active {
  background: var(--border-color, #f0f2f5);
  border-left-color: #764ba2;
}

.log-dir-item.error {
  border-left-color: #f44336;
  background: #ffebee;
  cursor: not-allowed;
}

.log-dir-item.error:hover {
  transform: none;
  box-shadow: none;
}

.log-dir-item h3 {
  margin: 0 0 0.5rem 0;
  font-size: 0.85rem;
  color: var(--text-color, #333);
}

.log-dir-item .stats {
  display: flex;
  gap: 0.75rem;
}

.log-dir-item .stat {
  text-align: center;
}

.log-dir-item .stat-value {
  display: block;
  font-size: 0.9rem;
  font-weight: 600;
  color: var(--primary-color, #667eea);
}

.log-dir-item .stat span:last-child {
  font-size: 0.65rem;
  color: var(--text-secondary, #888);
}

@media (max-width: 768px) {
  .log-dirs {
    grid-template-columns: repeat(2, 1fr);
    gap: 0.6rem;
  }
  
  .log-dir-item {
    padding: 0.6rem;
  }
  
  .log-dir-item h3 {
    font-size: 0.8rem;
  }
  
  .log-dir-item .stat-value {
    font-size: 0.85rem;
  }
  
  .dir-checkboxes {
    flex-direction: column;
    gap: 6px;
  }
}

@media (max-width: 480px) {
  .log-dirs {
    grid-template-columns: repeat(2, 1fr);
  }
}
</style>
