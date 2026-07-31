<template>
  <div class="card glass-card">
    <div class="header-row">
      <h2>日志目录</h2>
      <div class="header-actions-row">
        <button 
          class="config-btn btn-add-dir" 
          @click="handlePickDir" 
          title="添加自定义日志目录"
        >
          +目录
        </button>
        <button class="config-btn btn-primary" @click="showConfig = !showConfig" title="配置">
          设置
        </button>
      </div>
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
      <!-- P1: Custom directories added via file picker -->
      <div v-if="customDirs.length > 0" class="custom-dirs-section">
        <p class="config-hint">自定义目录：</p>
        <div v-for="cd in customDirs" :key="cd.path" class="custom-dir-item">
          <span class="custom-dir-path">{{ cd.displayPath || cd.path }}</span>
          <button class="custom-dir-remove" @click="removeCustomDir(cd.path)" title="移除">×</button>
        </div>
      </div>
      <!-- Manual custom dir input -->
      <div v-if="showCustomDirInput" class="custom-dir-input-row">
        <input
          type="text"
          v-model="customDirInput"
          placeholder="输入日志目录路径，如 /vol1/@appdata/myapp"
          class="custom-dir-input"
          @keyup.enter="confirmCustomDir"
        >
        <button class="config-btn btn-primary" @click="confirmCustomDir">确认</button>
        <button class="config-btn" @click="showCustomDirInput = false">取消</button>
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

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import type { Dir } from '../types'
import { pickUserFile, isFnosEnvironment, convertPathViaBackend } from '../services/fnos'

interface Props {
  dirs: Dir[]
  selectedDir: string | null
}

const props = defineProps<Props>()

const emit = defineEmits<{
  selectDir: [dirPath: string]
  addCustomDir: [dirPath: string]
}>()

const STORAGE_KEY = 'logmanager_visible_dirs'
const CUSTOM_DIRS_KEY = 'logmanager_custom_dirs'

const showConfig = ref(false)
const visibleDirs = ref<string[]>([])
const isFnosEnv = ref(false)

// P1: Custom directories
interface CustomDir {
  path: string
  displayPath?: string
}
const customDirs = ref<CustomDir[]>([])

onMounted(async () => {
  isFnosEnv.value = isFnosEnvironment()
  loadCustomDirs()
  // P2: Convert paths for display
  if (isFnosEnv.value && customDirs.value.length > 0) {
    for (const cd of customDirs.value) {
      try {
        cd.displayPath = await convertPathViaBackend(cd.path)
      } catch {
        cd.displayPath = cd.path
      }
    }
  }
})

// P1: Open fnOS file picker or prompt to add custom directory
const showCustomDirInput = ref(false)
const customDirInput = ref('')

async function handlePickDir(): Promise<void> {
  // Try fnOS file picker first
  if (isFnosEnvironment()) {
    try {
      const result = await pickUserFile({
        directory: true,
        title: '选择日志目录'
      })
      if (result && result.code === 0 && result.data && result.data.length > 0) {
        const dirPath = result.data[0]
        addCustomDir(dirPath)
        emit('selectDir', dirPath)
        return
      }
    } catch (e) {
      console.error('选择目录失败:', e)
    }
  }
  // Fallback: show manual input
  showCustomDirInput.value = !showCustomDirInput.value
}

function confirmCustomDir(): void {
  const path = customDirInput.value.trim()
  if (!path) return
  addCustomDir(path)
  emit('selectDir', path)
  customDirInput.value = ''
  showCustomDirInput.value = false
}

function addCustomDir(path: string): void {
  if (customDirs.value.some(d => d.path === path)) return
  customDirs.value.push({ path })
  saveCustomDirs()
}

function removeCustomDir(path: string): void {
  customDirs.value = customDirs.value.filter(d => d.path !== path)
  saveCustomDirs()
}

function saveCustomDirs(): void {
  try {
    localStorage.setItem(CUSTOM_DIRS_KEY, JSON.stringify(customDirs.value))
  } catch {
    // ignore
  }
}

function loadCustomDirs(): void {
  try {
    const saved = localStorage.getItem(CUSTOM_DIRS_KEY)
    if (saved) {
      customDirs.value = JSON.parse(saved)
    }
  } catch {
    localStorage.removeItem(CUSTOM_DIRS_KEY)
  }
}

const dirNames: Record<string, string> = {
  '/vol1/@appdata': '@appdata',
  '/vol1/@appconf': '@appconf',
  '/vol1/@apphome': '@apphome',
  '/vol1/@apptemp': '@apptemp',
  '/vol1/@appshare': '@appshare',
  '/var/log/apps': '/var/log/apps'
}

interface DirWithDisplay extends Dir {
  displayName: string
  exists?: boolean
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

function toggleDir(path: string): void {
  if (!path) return
  const index = visibleDirs.value.indexOf(path)
  if (index > -1) {
    visibleDirs.value.splice(index, 1)
  } else {
    visibleDirs.value.push(path)
  }
  saveVisibleDirs()
}

function handleDirClick(dir: DirWithDisplay): void {
  if (!dir || !('exists' in dir) || !dir.exists) return
  emit('selectDir', dir.path)
}

function saveVisibleDirs(): void {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(visibleDirs.value))
  } catch {
    // ignore
  }
}

function loadVisibleDirs(): void {
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

loadVisibleDirs()
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

.header-actions-row {
  display: flex;
  gap: var(--spacing-xs);
  flex-shrink: 0;
}

.config-btn {
  background: var(--glass-bg);
  backdrop-filter: blur(var(--glass-blur));
  -webkit-backdrop-filter: blur(var(--glass-blur));
  border: 1px solid var(--glass-border);
  padding: var(--spacing-xs) var(--spacing-sm);
  border-radius: var(--radius-xs);
  cursor: pointer;
  font-size: var(--font-size-md);
  color: var(--text-color-1);
  flex: none;
  width: auto;
  min-width: 0;
  flex-shrink: 0;
  transition: all var(--transition-fast);
  font-weight: var(--font-weight-medium);
}

.btn-add-dir {
  background: var(--primary-color);
  color: var(--text-color-on-primary);
  border-color: var(--primary-color);
}

.btn-add-dir:hover {
  background: var(--primary-hover);
  border-color: var(--primary-hover);
}

.config-btn:hover {
  background: var(--primary-color);
  color: var(--text-color-on-primary);
  border-color: var(--primary-color);
  box-shadow: 0 0 12px var(--glow-primary);
}

.config-btn:active {
  transform: scale(0.95);
}

/* P1: Custom directory styles */
.custom-dirs-section {
  margin-top: var(--spacing-md);
  padding-top: var(--spacing-sm);
  border-top: 1px solid var(--glass-border);
}

.custom-dir-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: var(--spacing-xs) var(--spacing-sm);
  background: var(--card-bg);
  border-radius: var(--radius-xs);
  margin-bottom: var(--spacing-xs);
  border: 1px solid var(--primary-color);
}

.custom-dir-path {
  font-size: var(--font-size-sm);
  color: var(--primary-color);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  flex: 1;
  font-family: monospace;
}

.custom-dir-remove {
  background: none;
  border: none;
  color: var(--text-color-3);
  font-size: var(--font-size-lg);
  cursor: pointer;
  padding: 0 4px;
  line-height: 1;
  transition: color var(--transition-fast);
  flex-shrink: 0;
}

.custom-dir-remove:hover {
  color: var(--error-color);
}

.custom-dir-input-row {
  display: flex;
  align-items: center;
  gap: var(--spacing-xs);
  margin-top: var(--spacing-sm);
  padding-top: var(--spacing-sm);
  border-top: 1px solid var(--glass-border);
}

.custom-dir-input {
  flex: 1;
  padding: var(--spacing-xs) var(--spacing-sm);
  border: 1px solid var(--glass-border);
  border-radius: var(--radius-xs);
  font-size: var(--font-size-sm);
  background: var(--glass-bg);
  color: var(--text-color);
  font-family: monospace;
  transition: border-color var(--transition-fast);
}

.custom-dir-input:focus {
  outline: none;
  border-color: var(--primary-color);
  box-shadow: var(--focus-ring);
}

.config-panel {
  background: var(--glass-bg);
  backdrop-filter: blur(var(--glass-blur));
  -webkit-backdrop-filter: blur(var(--glass-blur));
  border: 1px solid var(--glass-border);
  padding: var(--spacing-md);
  border-radius: var(--radius-sm);
  margin-bottom: var(--spacing-lg);
  box-shadow: var(--depth-1);
}

.config-hint {
  margin: 0 0 var(--spacing-sm) 0;
  font-size: var(--font-size-base);
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
  font-size: var(--font-size-base);
  cursor: pointer;
  color: var(--text-color-1);
  transition: all var(--transition-base);
  border: 1px solid transparent;
}

.dir-checkbox:hover {
  background: var(--bg-color-2);
  transform: translateY(-1px);
  box-shadow: var(--depth-1);
  border-color: var(--glass-border);
}

.dir-checkbox:active {
  transform: scale(0.97);
}

.dir-status {
  font-size: var(--font-size-xs);
  padding: 2px var(--spacing-xs);
  border-radius: var(--radius-xs);
  font-weight: 500;
}

.dir-status.exists {
  color: var(--success-color);
  background: var(--success-bg);
}

.dir-status.not-exists {
  color: var(--error-color);
  background: var(--error-bg);
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
  transition: all var(--transition-spring);
  position: relative;
  overflow: hidden;
  perspective: var(--perspective-near);
}

.log-dir-item::after {
  content: '';
  position: absolute;
  inset: 0;
  background: linear-gradient(135deg, rgba(255,255,255,0.06) 0%, transparent 60%);
  pointer-events: none;
  border-radius: inherit;
}

.log-dir-item:hover {
  transform: perspective(var(--perspective-near)) translateY(var(--translate-hover)) rotateX(var(--rotate-subtle));
  box-shadow: var(--depth-2);
  border-left-color: var(--primary-hover);
}

.log-dir-item:active {
  transform: scale(var(--scale-active));
}

.log-dir-item.active {
  background: var(--glass-bg);
  backdrop-filter: blur(var(--glass-blur));
  -webkit-backdrop-filter: blur(var(--glass-blur));
  border-left-color: var(--primary-hover);
  box-shadow: 0 0 12px var(--glow-primary-soft);
  animation: glow-pulse-dir 2.5s ease-in-out infinite;
}

@keyframes glow-pulse-dir {
  0%, 100% { box-shadow: 0 0 8px var(--glow-primary-soft); }
  50% { box-shadow: 0 0 20px var(--glow-primary); }
}

.log-dir-item.error {
  border-left-color: var(--error-color);
  background: var(--error-bg);
  cursor: not-allowed;
}

.log-dir-item.error:hover {
  transform: none;
  box-shadow: none;
}

.log-dir-item h3 {
  margin: 0 0 var(--spacing-xs) 0;
  font-size: var(--font-size-base);
  font-weight: var(--font-weight-medium);
  color: var(--text-color-1);
  position: relative;
  z-index: 1;
}

.log-dir-item .stats {
  display: flex;
  gap: var(--spacing-sm);
  position: relative;
  z-index: 1;
}

.log-dir-item .stat {
  text-align: center;
}

.log-dir-item .stat-value {
  display: block;
  font-size: var(--font-size-md);
  font-weight: var(--font-weight-semibold);
  color: var(--primary-color);
}

.log-dir-item .stat span:last-child {
  font-size: var(--font-size-xs);
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
    font-size: var(--font-size-sm);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .log-dir-item .stat-value {
    font-size: var(--font-size-base);
  }

  .log-dir-item .stat span:last-child {
    font-size: var(--font-size-2xs);
  }

  .dir-checkboxes {
    flex-direction: column;
    gap: var(--spacing-xs);
  }
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

@media (max-width: 480px) {
  .log-dirs {
    grid-template-columns: repeat(2, 1fr);
    gap: 4px;
  }

  .log-dir-item {
    padding: 4px;
  }

  .log-dir-item h3 {
    font-size: var(--font-size-xs);
  }

  .log-dir-item .stats {
    gap: 4px;
  }

  .log-dir-item .stat-value {
    font-size: var(--font-size-sm);
  }

  .log-dir-item .stat span:last-child {
    font-size: var(--font-size-2xs);
  }
}
</style>
