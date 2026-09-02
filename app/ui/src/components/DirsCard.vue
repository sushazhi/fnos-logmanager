<template>
  <div class="card glass-card">
    <div class="header-row">
      <h2>日志目录</h2>
      <div class="header-actions-row">
        <button class="config-btn btn-primary" @click="showConfig = !showConfig" title="配置日志目录">
          设置
        </button>
      </div>
    </div>

    <div class="config-panel" v-if="showConfig">
      <div class="config-section">
        <div class="config-section-head">
          <span class="config-section-title">系统目录</span>
          <span class="config-section-count">{{ allDirs.length }}</span>
        </div>
        <div class="dir-checkboxes">
          <label v-for="dir in allDirs" :key="dir.path" class="dir-checkbox">
            <input
              type="checkbox"
              :checked="visibleDirs.includes(dir.path)"
              @change="toggleDir(dir.path)"
            >
            <span>{{ dir.displayName }}</span>
            <span class="dir-status" :class="{ exists: dir.exists, 'not-exists': !dir.exists }" :title="dir.exists ? '存在' : '不存在'">
              {{ dir.exists ? '√' : '×' }}
            </span>
          </label>
        </div>
      </div>

      <!-- 自定义目录展示 -->
      <div v-if="mergedDirs.length > 0" class="config-section">
        <div class="config-section-head">
          <span class="config-section-title">已添加目录</span>
          <span class="config-section-count">{{ mergedDirs.length }}</span>
        </div>
        <div v-for="item in mergedDirs" :key="item.path" class="dir-entry">
          <button class="dir-entry-copy" :title="'复制路径: ' + item.path" @click="copyPath(item.path)">
            <span class="dir-entry-path">{{ item.displayPath || item.path }}</span>
          </button>
          <span class="dir-entry-actions">
            <button v-if="isFnosEnv" class="dir-entry-act" title="在文件管理器中定位" @click="handleOpenManager(item.path)">
              <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M3 7a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z"/></svg>
            </button>
            <button v-if="isFnosEnv" class="dir-entry-act" title="查看目录详情" @click="handleShowDetails(item.path)">
              <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/><line x1="11" y1="8" x2="11" y2="14"/><line x1="8" y1="11" x2="14" y2="11"/></svg>
            </button>
            <button class="dir-entry-act dir-entry-remove" title="移除目录" @click="confirmRemoveDir(item)">
              <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="3 6 5 6 21 6"/><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/></svg>
            </button>
          </span>
        </div>
        <div v-if="copiedPath" class="copy-tip">已复制: {{ copiedPath }}</div>
      </div>

      <!-- 手动自定义目录输入 -->
      <div class="custom-dir-input-row">
        <input
          type="text"
          v-model="customDirInput"
          placeholder="输入日志目录路径，如 /vol1/@appdata/myapp"
          class="custom-dir-input"
          @keyup.enter="confirmCustomDir"
        >
        <button class="config-btn btn-primary" @click="confirmCustomDir">添加</button>
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
    <ConfirmDialog ref="confirmDialog" />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import type { Dir } from '../types'
import { authorizeUserFile, isFnosEnvironment, openFileManager, showFileDetails } from '../services/fnos'
import api from '../services/api'
import ConfirmDialog from './ConfirmDialog.vue'

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

interface ConfirmDialogExpose {
  show(options?: { title?: string; message?: string; type?: string; confirmText?: string }): Promise<boolean>
}

const showConfig = ref(false)
const visibleDirs = ref<string[]>([])
const isFnosEnv = ref(false)
const copiedPath = ref('')
const confirmDialog = ref<ConfirmDialogExpose | null>(null)

// P1: Custom directories
interface CustomDir {
  path: string
  displayPath?: string
}
const customDirs = ref<CustomDir[]>([])

onMounted(async () => {
  isFnosEnv.value = isFnosEnvironment()
  loadCustomDirs()
  // P2: Attempt to restore authorization for previously saved custom dirs
  // that are no longer in the user's accessible folders (e.g. after a
  // service restart, the SDK-side authorization may need re-confirmation).
  restoreCustomDirAuth()
})

// ---- 复制路径 ----
async function copyPath(path: string): Promise<void> {
  try {
    await navigator.clipboard.writeText(path)
    copiedPath.value = path
    setTimeout(() => { copiedPath.value = '' }, 2000)
  } catch {
    // fallback
    try {
      const ta = document.createElement('textarea')
      ta.value = path
      document.body.appendChild(ta)
      ta.select()
      document.execCommand('copy')
      document.body.removeChild(ta)
      copiedPath.value = path
      setTimeout(() => { copiedPath.value = '' }, 2000)
    } catch {
      // ignore
    }
  }
}

// ---- 移除目录（二次确认） ----
async function confirmRemoveDir(item: { path: string }): Promise<void> {
  const ok = await confirmDialog.value?.show({
    title: '移除目录',
    message: `确定要移除该目录吗？\n${item.path}`,
    type: 'warning',
    confirmText: '移除'
  })
  if (!ok) return
  removeCustomDir(item.path)
}

// P1: 手动输入自定义目录
const customDirInput = ref('')

// 自定义目录统一渲染
const mergedDirs = computed<Array<{ path: string; displayPath?: string }>>(() =>
  customDirs.value.map(d => ({ path: d.path, displayPath: d.displayPath }))
)



async function confirmCustomDir(): Promise<void> {
  const path = customDirInput.value.trim()
  if (!path) return
  addCustomDir(path)
  customDirInput.value = ''
  // 添加后立即申请授权；否则要等下次打开应用才由 restoreCustomDirAuth 补做
  if (isFnosEnvironment()) {
    const auth = await authorizeUserFile(path)
    if (!auth || auth.code !== 0) {
      // 手机壳 SDK 对按路径授权有 appApi 版本门槛，不足时直接抛错（封装吞成
      // null）。trim 授权按账号跨端共享，在桌面端添加一次即可两端通用。
      await confirmDialog.value?.show({
        title: '授权未成功',
        message: `目录已添加，但 fnOS 授权未完成，暂时无法读取该目录。\n可能是手机端不支持按路径授权，请在电脑端打开本应用重新添加一次（授权按账号生效，两端通用）。\n${path}`,
        type: 'warning',
        confirmText: '知道了'
      })
    }
  }
  emit('selectDir', path)
}

function addCustomDir(path: string): void {
  if (customDirs.value.some(d => d.path === path)) return
  customDirs.value.push({ path })
  saveCustomDirs()
}

function removeCustomDir(path: string): void {
  customDirs.value = customDirs.value.filter(d => d.path !== path)
  saveCustomDirs()
  // P1: Revoke fnOS user authorization for the removed directory
  if (isFnosEnvironment()) {
    api.post<{ success: boolean }>('/api/dirs/unauthorize', { path }).catch(err => {
      console.warn('移除目录授权失败:', err)
    })
  }
}

// P1: Open the host file manager at the given directory
function handleOpenManager(path: string): void {
  openFileManager(path).catch(() => {})
}

// P1: Show host file details panel for the given directory
function handleShowDetails(path: string): void {
  showFileDetails([path]).catch(() => {})
}

// P2: Re-request fnOS authorization for custom dirs whose authorization was
// lost (they still exist locally but the host no longer grants access).
async function restoreCustomDirAuth(): Promise<void> {
  if (!isFnosEnvironment() || customDirs.value.length === 0) return
  // Determine which custom dirs are still authorized by the host.
  let authorized = new Set<string>()
  try {
    const data = await api.get<{ dirs: Array<{ path: string }> }>('/api/dirs')
    for (const d of data.dirs || []) authorized.add(d.path)
  } catch {
    return
  }
  for (const cd of customDirs.value) {
    if (authorized.has(cd.path)) continue
    try {
      await authorizeUserFile(cd.path)
    } catch {
      // Ignore: the host may legitimately refuse (e.g. path no longer exists)
    }
  }
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
  '/var/log/apps': '/var/log/apps',
  '/var/log/trim_app_center': '应用中心',
  '/var/log/trim-connect': '连接服务',
  '/var/log/trim_open_gateway': '开放网关',
  '/var/log/accountsrv': '账号服务',
  '/var/log/updatemgr': '更新管理'
}

interface DirWithDisplay extends Dir {
  displayName: string
  exists?: boolean
}

const allDirs = computed(() => {
  if (!props.dirs || !Array.isArray(props.dirs)) return []
  return props.dirs.map(dir => ({
    ...dir,
    // P2: 优先使用后端语义化 displayPath；无则回退到固定映射/原始路径
    displayName: (dir as Dir).displayPath || dirNames[dir.path] || dir.path
  }))
})

const displayedDirs = computed(() => {
  const dirs = allDirs.value

  // 系统/后端目录：按可见性过滤
  let baseDirs: Array<DirWithDisplay & { isCustom?: boolean }> = []
  if (dirs && dirs.length > 0) {
    baseDirs = dirs.filter(d =>
      d && (visibleDirs.value.length === 0 ? d.exists : visibleDirs.value.includes(d.path))
    ).map(d => ({ ...d, isCustom: false }))
  }

  // 合并自定义目录（设置面板添加的，来自 localStorage）。
  // 后端已合并用户授权目录的真实统计，能对上路径就用真实值；
  // 对不上（尚未授权 / 独立模式）才兜底显示 0。
  const customDirs_: Array<DirWithDisplay & { isCustom?: boolean }> = customDirs.value.map(d => {
    const real = allDirs.value.find(x => x.path === d.path)
    return {
      path: d.path,
      displayName: d.displayPath || dirNames[d.path] || d.path,
      displayPath: d.displayPath,
      logCount: real?.logCount ?? 0,
      archiveCount: real?.archiveCount ?? 0,
      totalSize: real?.totalSize ?? '0B',
      exists: real?.exists ?? true,
      isCustom: true
    }
  })

  return [...baseDirs, ...customDirs_]
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

/* ===== 配置面板分组 ===== */
.config-section {
  margin-bottom: var(--spacing-md);
  padding-bottom: var(--spacing-md);
  border-bottom: 1px solid var(--glass-border);
}

.config-section:last-child {
  margin-bottom: 0;
  padding-bottom: 0;
  border-bottom: none;
}

.config-section-head {
  display: flex;
  align-items: center;
  gap: var(--spacing-xs);
  margin-bottom: var(--spacing-sm);
}

.config-section-title {
  font-size: var(--font-size-base);
  font-weight: var(--font-weight-medium);
  color: var(--text-color-1);
}

.config-section-count {
  font-size: var(--font-size-xs);
  color: var(--text-color-3);
  background: var(--bg-color-2);
  padding: 1px var(--spacing-xs);
  border-radius: var(--radius-full);
  min-width: 18px;
  text-align: center;
}

/* ===== 目录条目（卡片式） ===== */
.dir-entry {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
  padding: var(--spacing-sm);
  background: var(--card-bg);
  border: 1px solid var(--glass-border);
  border-radius: var(--radius-sm);
  margin-bottom: var(--spacing-xs);
  transition: all var(--transition-fast);
}

.dir-entry:hover {
  border-color: var(--glass-border-strong);
  box-shadow: var(--depth-1);
}

.dir-entry-copy {
  flex: 1;
  min-width: 0;
  text-align: left;
  background: none;
  border: none;
  padding: 0;
  cursor: pointer;
  font-family: monospace;
  font-size: var(--font-size-sm);
  color: var(--primary-color);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  transition: color var(--transition-fast);
}

.dir-entry-copy:hover {
  color: var(--primary-hover);
  text-decoration: underline;
}

.dir-entry-path {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.dir-entry-actions {
  display: flex;
  align-items: center;
  gap: 2px;
  flex-shrink: 0;
}

.dir-entry-act {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  background: none;
  border: none;
  border-radius: var(--radius-xs);
  color: var(--text-color-3);
  cursor: pointer;
  transition: all var(--transition-fast);
}

.dir-entry-act:hover {
  background: var(--bg-color-2);
  color: var(--primary-color);
}

.dir-entry-remove:hover {
  background: var(--error-bg);
  color: var(--error-color);
}

.copy-tip {
  margin-top: var(--spacing-xs);
  font-size: var(--font-size-xs);
  color: var(--success-color);
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

  .dir-entry {
    flex-wrap: wrap;
  }

  .dir-entry-copy {
    width: 100%;
    flex: none;
  }

  .dir-entry-actions {
    margin-left: auto;
  }

  .custom-dir-input-row {
    flex-wrap: wrap;
  }

  .custom-dir-input-row .config-btn {
    flex: 1;
    text-align: center;
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
