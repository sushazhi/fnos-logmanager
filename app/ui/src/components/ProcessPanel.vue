<template>
  <div class="hm-overlay-base" @click.self="$emit('close')">
    <div class="hm-modal-base process-panel">
      <div class="panel-header">
        <h3>进程管理</h3>
        <button class="close-btn" @click="$emit('close')">×</button>
      </div>

      <div class="panel-body">
        <div class="error-banner" v-if="errorMessage">
          <span class="error-text">{{ errorMessage }}</span>
          <button class="error-dismiss" @click="errorMessage = null">×</button>
        </div>

        <div class="success-banner" v-if="successMessage">
          <span class="success-text">{{ successMessage }}</span>
          <button class="error-dismiss" @click="successMessage = null">×</button>
        </div>

        <div class="toolbar">
          <div class="search-box">
            <input
              v-model="searchQuery"
              type="text"
              placeholder="按进程名 / PID / 端口过滤..."
              @keyup.enter="loadProcesses()"
            />
            <button class="search-btn" @click="loadProcesses()" :disabled="loading">搜索</button>
          </div>
          <div class="toolbar-actions">
            <div class="mobile-sort" v-if="isMobile">
              <select
                v-model="mobileSortValue"
                @change="applyMobileSort()"
                class="sort-select"
                title="排序"
              >
                <option value="cpu-desc">CPU 从高到低</option>
                <option value="cpu-asc">CPU 从低到高</option>
                <option value="mem-desc">内存从高到低</option>
                <option value="mem-asc">内存从低到高</option>
                <option value="pid-asc">PID 从小到大</option>
                <option value="pid-desc">PID 从大到小</option>
                <option value="name-asc">名称 A→Z</option>
                <option value="name-desc">名称 Z→A</option>
              </select>
            </div>
            <label class="scope-toggle" title="只显示用户进程（服务），隐藏系统底层进程">
              <input type="checkbox" v-model="userOnly" @change="loadProcesses()" />
              <span>仅用户进程</span>
            </label>
            <span class="count-badge">共 {{ total }} 个进程</span>
            <button class="action-btn" @click="loadProcesses()" :disabled="loading">
              {{ loading ? '刷新中...' : '刷新' }}
            </button>
          </div>
        </div>

        <div class="loading-state" v-if="loading && processes.length === 0">
          <div class="process-shimmer">
            <div class="hm-shimmer shimmer-row" v-for="i in 6" :key="i"></div>
          </div>
          <span class="loading-tip">加载进程列表...</span>
        </div>

        <template v-if="!loading || processes.length > 0">
          <div class="process-list-header">
            <span class="col-name sortable" @click="toggleSort('name')">进程名 {{ sortArrow('name') }}</span>
            <span class="col-pid sortable" @click="toggleSort('pid')">PID {{ sortArrow('pid') }}</span>
            <span class="col-user">用户</span>
            <span class="col-state">状态</span>
            <span class="col-cpu sortable" @click="toggleSort('cpu')">CPU {{ sortArrow('cpu') }}</span>
            <span class="col-mem sortable" @click="toggleSort('mem')">内存 {{ sortArrow('mem') }}</span>
            <span class="col-port">端口</span>
            <span class="col-action">操作</span>
          </div>

          <div class="process-list">
            <div
              v-for="p in displayedProcesses"
              :key="p.pid"
              class="process-item"
              :class="{ protected: p.protect }"
            >
              <span class="col-name">
                <span class="proc-name" :title="p.command || p.exePath || p.name">{{ p.name || '-' }}</span>
                <span v-if="p.protect" class="protect-badge" title="受保护进程，不可结束">🔒</span>
                <span v-if="p.isDocker" class="docker-badge" :title="'Docker 容器进程' + (p.containerName ? '：' + p.containerName : '')">🐳 {{ p.containerName || 'Docker' }}</span>
              </span>
              <span class="col-pid"><code class="pid">{{ p.pid }}</code></span>
              <span class="col-user">{{ p.user || '-' }}</span>
              <span class="col-state">
                <span class="state-badge" :class="stateClass(p.state)">{{ p.state }}</span>
              </span>
              <span class="col-cpu">
                <span class="cpu-value">{{ formatCpu(p.cpu) }}</span>
              </span>
              <span class="col-mem">{{ p.memory || '-' }}</span>
              <span class="col-port">
                <span v-if="p.ports && p.ports.length" class="ports">
                  <span v-for="port in p.ports" :key="port" class="port-badge">{{ port }}</span>
                </span>
                <span v-else class="no-port">-</span>
              </span>
              <span class="col-action">
                <button
                  v-if="!p.protect"
                  class="log-btn"
                  @click="openProcessFiles(p)"
                  :disabled="loadingFiles === p.pid"
                  :title="'查看进程 ' + p.pid + ' 的日志'"
                >
                  {{ loadingFiles === p.pid ? '...' : '日志' }}
                </button>
                <button
                  v-if="!p.protect"
                  class="kill-btn"
                  @click="killProcess(p)"
                  :disabled="killing === p.pid"
                  :title="'结束进程 ' + p.pid"
                >
                  {{ killing === p.pid ? '结束中...' : '结束' }}
                </button>
                <span v-else class="protected-text">受保护</span>
              </span>
            </div>

            <div class="show-more" v-if="processes.length > DISPLAY_LIMIT">
              <button class="show-more-btn" @click="showAll = !showAll">
                {{ showAll ? '收起，仅显示前 ' + DISPLAY_LIMIT + ' 个' : '仅显示前 ' + DISPLAY_LIMIT + ' 个，点击显示全部（共 ' + processes.length + ' 个）' }}
              </button>
            </div>

            <div class="no-processes" v-if="processes.length === 0 && !loading">
              未找到匹配的进程
            </div>
          </div>
        </template>
      </div>
    </div>

    <!-- 进程日志查看覆盖层 -->
    <div class="process-log-layer" v-if="logState !== 'closed'" @click.self="closeLogViewer">
      <div class="process-log-box" :class="{ full: logState === 'content' }">
        <div class="log-layer-header">
          <span class="log-layer-title">
            {{ logState === 'content' ? '日志内容 - ' + (logFile?.name || '') : (logState === 'files' ? '进程 ' + logPid + ' 打开的文件' : '') }}
          </span>
          <div class="log-layer-actions" v-if="logState === 'content'">
            <div class="log-search">
              <input
                v-model="logSearch"
                type="text"
                placeholder="搜索日志..."
                @keyup.enter="nextMatch()"
              />
              <span class="search-count" v-if="logSearch">{{ matchLines.length ? (currentMatch + 1) + '/' + matchLines.length : '无匹配' }}</span>
              <button class="search-nav" :disabled="!matchLines.length" @click="prevMatch()" title="上一个匹配">↑</button>
              <button class="search-nav" :disabled="!matchLines.length" @click="nextMatch()" title="下一个匹配">↓</button>
            </div>
            <span class="log-line-count">{{ logResult.totalLines }} 行</span>
            <button class="mini-btn" :class="{ active: logTailing }" @click="toggleTail" title="实时追踪">
              {{ logTailing ? '停止追踪' : '追踪' }}
            </button>
            <button class="mini-btn" @click="loadLogContent(false)" title="刷新日志">刷新</button>
          </div>
          <button class="close-btn" @click="closeLogViewer">×</button>
        </div>

        <!-- 文件选择阶段 -->
        <div class="file-picker" v-if="logState === 'files'">
          <div class="file-picker-hint">该进程当前打开的文件（日志文件优先）：</div>
          <div class="file-list">
            <div
              v-for="f in logFiles"
              :key="f.path"
              class="file-item"
              @click="openLogFile(f)"
              :title="f.path"
            >
              <span class="file-name">{{ f.name }}</span>
              <span class="file-path">{{ f.path }}</span>
              <span class="file-size">{{ f.sizeText }}</span>
              <span class="file-badge" :class="f.isLog ? 'log' : 'plain'">
                {{ f.isLog ? '日志' : '文件' }}
              </span>
            </div>
            <div class="no-file" v-if="logFiles.length === 0 && !loadingFilesText">
              该进程未打开任何可读的日志文件
            </div>
            <div class="no-file" v-if="loadingFilesText">{{ loadingFilesText }}</div>
          </div>
        </div>

        <!-- 日志内容阶段 -->
        <div class="log-content-view" v-if="logState === 'content'" ref="logViewport">
          <div
            v-for="(line, idx) in logLines"
            :key="idx"
            :ref="el => setLineRef(el, idx)"
            class="log-line"
            :class="{
              'match-line': isMatchLine(idx),
              'current-match': idx === matchLines[currentMatch]
            }"
          >
            <span class="line-num">{{ idx + 1 }}</span>
            <span class="line-text">{{ line }}</span>
          </div>
          <div class="log-empty" v-if="logLines.length === 0">{{ '(空文件)' }}</div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, nextTick } from 'vue'
import { processApi, type ProcessItem, type ProcessSortKey, type ProcessFile, type ProcessLogResult } from '../services/api'
import { useStore } from '../composables/useStore'

const { confirm } = useStore()

const emit = defineEmits<{
  close: []
}>()

// 一次性渲染的进程数上限，超出部分需点击"显示全部"再渲染，避免 DOM 过多导致卡顿
const DISPLAY_LIMIT = 300

const loading = ref(false)
const killing = ref(0)
const errorMessage = ref<string | null>(null)
const successMessage = ref<string | null>(null)
const processes = ref<ProcessItem[]>([])
const total = ref(0)
const searchQuery = ref('')
const userOnly = ref(true) // 默认只显示用户进程/服务
const sortKey = ref<ProcessSortKey>('cpu')
const sortOrder = ref<'asc' | 'desc'>('desc')
const showAll = ref(false)
const isMobile = ref(window.matchMedia('(max-width: 768px)').matches)
const mobileSortValue = ref('cpu-desc')

const mobileSortOptions: Record<string, [ProcessSortKey, 'asc' | 'desc']> = {
  'cpu-desc': ['cpu', 'desc'],
  'cpu-asc': ['cpu', 'asc'],
  'mem-desc': ['mem', 'desc'],
  'mem-asc': ['mem', 'asc'],
  'pid-asc': ['pid', 'asc'],
  'pid-desc': ['pid', 'desc'],
  'name-asc': ['name', 'asc'],
  'name-desc': ['name', 'desc'],
}

function applyMobileSort() {
  const [key, order] = mobileSortOptions[mobileSortValue.value] || ['cpu', 'desc']
  sortKey.value = key
  sortOrder.value = order
  loadProcesses()
}

const displayedProcesses = computed(() =>
  showAll.value || processes.value.length <= DISPLAY_LIMIT
    ? processes.value
    : processes.value.slice(0, DISPLAY_LIMIT)
)

// ---- 进程日志查看 ----
const logState = ref<'closed' | 'files' | 'content'>('closed')
const logPid = ref(0)
const logFiles = ref<ProcessFile[]>([])
const logFile = ref<ProcessFile | null>(null)
const logResult = ref<ProcessLogResult>({ content: '', totalLines: 0, size: 0, sizeFormatted: '', truncated: false, hasMore: false })
const logTailing = ref(false)
const loadingFiles = ref(0) // 正在加载文件列表的 PID
const loadingFilesText = ref('')

// ---- 日志搜索 ----
const logSearch = ref('')
const currentMatch = ref(0)
const logViewport = ref<HTMLElement | null>(null)
const lineRefs = new Map<number, HTMLElement>()

const logLines = computed(() => {
  const content = logResult.value.content || ''
  return content === '' ? [] : content.split('\n')
})

const matchLines = computed(() => {
  const keyword = logSearch.value.trim().toLowerCase()
  if (!keyword) return []
  const lines = logLines.value
  const matches: number[] = []
  for (let i = 0; i < lines.length; i++) {
    if (lines[i].toLowerCase().includes(keyword)) matches.push(i)
  }
  return matches
})

function isMatchLine(idx: number): boolean {
  return matchLines.value.includes(idx)
}

function setLineRef(el: unknown, idx: number) {
  if (el instanceof HTMLElement) {
    lineRefs.set(idx, el)
  } else {
    lineRefs.delete(idx)
  }
}

function nextMatch() {
  if (!matchLines.value.length) return
  currentMatch.value = (currentMatch.value + 1) % matchLines.value.length
  scrollToCurrentMatch()
}

function prevMatch() {
  if (!matchLines.value.length) return
  currentMatch.value = (currentMatch.value - 1 + matchLines.value.length) % matchLines.value.length
  scrollToCurrentMatch()
}

function scrollToCurrentMatch() {
  const target = matchLines.value[currentMatch.value]
  const lineEl = lineRefs.get(target)
  const viewport = logViewport.value
  if (lineEl && viewport) {
    lineEl.scrollIntoView({ block: 'center' })
  }
}

let logTimer: ReturnType<typeof setInterval> | null = null

async function openProcessFiles(p: ProcessItem) {
  loadingFiles.value = p.pid
  try {
    const data = await processApi.getProcessFiles(p.pid)
    logFiles.value = data.files || []
    logPid.value = p.pid
    logState.value = 'files'
  } catch (e: any) {
    errorMessage.value = e?.message || `获取进程 ${p.pid} 的文件失败`
  } finally {
    loadingFiles.value = 0
  }
}

async function openLogFile(f: ProcessFile) {
  if (!f.isLog) return
  logFile.value = f
  logState.value = 'content'
  logTailing.value = false
  logSearch.value = ''
  currentMatch.value = 0
  stopLogTail()
  await loadLogContent(false)
}

async function loadLogContent(tail: boolean) {
  if (!logFile.value) return
  try {
    const data = await processApi.readProcessLog(logPid.value, logFile.value.path, 1000, tail)
    logResult.value = data
  } catch (e: any) {
    errorMessage.value = e?.message || '读取进程日志失败'
  }
}

function toggleTail() {
  logTailing.value = !logTailing.value
  if (logTailing.value) {
    loadLogContent(true)
    startLogTail()
  } else {
    stopLogTail()
  }
}

function startLogTail() {
  stopLogTail()
  logTimer = setInterval(() => {
    if (logState.value === 'content' && logFile.value) {
      loadLogContent(true)
    }
  }, 3000)
}

function stopLogTail() {
  if (logTimer) {
    clearInterval(logTimer)
    logTimer = null
  }
}

function closeLogViewer() {
  stopLogTail()
  logState.value = 'closed'
  logFiles.value = []
  logFile.value = null
  logResult.value = { content: '', totalLines: 0, size: 0, sizeFormatted: '', truncated: false, hasMore: false }
  logTailing.value = false
  logSearch.value = ''
  currentMatch.value = 0
  lineRefs.clear()
}

onUnmounted(() => stopLogTail())

async function loadProcesses() {
  loading.value = true
  errorMessage.value = null
  try {
    const data = await processApi.getProcesses({
      q: searchQuery.value.trim() || undefined,
      scope: userOnly.value ? 'user' : 'all',
      sort: sortKey.value,
      order: sortOrder.value
    })
    processes.value = data.processes || []
    total.value = data.total ?? processes.value.length
    showAll.value = false
  } catch (e: any) {
    errorMessage.value = e?.message || '加载进程列表失败'
    processes.value = []
    total.value = 0
  } finally {
    loading.value = false
  }
}

function toggleSort(key: ProcessSortKey) {
  if (sortKey.value === key) {
    // 同一列：切换升降序
    sortOrder.value = sortOrder.value === 'asc' ? 'desc' : 'asc'
  } else {
    sortKey.value = key
    sortOrder.value = key === 'name' || key === 'pid' ? 'asc' : 'desc'
  }
  loadProcesses()
}

function sortArrow(key: ProcessSortKey): string {
  if (sortKey.value !== key) return ''
  return sortOrder.value === 'asc' ? '▲' : '▼'
}

function formatCpu(cpu: number): string {
  if (cpu == null || isNaN(cpu)) return '-'
  return cpu >= 100 ? `${cpu.toFixed(1)}%` : `${cpu.toFixed(1)}%`
}

async function killProcess(proc: ProcessItem) {
  const confirmed = await confirm({
    title: '结束进程',
    message: `确定要结束进程吗？\n\n进程名: ${proc.name || '未知'}\nPID: ${proc.pid}\n命令行: ${proc.command || proc.exePath || '未知'}\n\n结束进程后该程序将被终止，且无法恢复。`,
    type: 'danger',
    confirmText: '结束'
  })
  if (!confirmed) return

  killing.value = proc.pid
  errorMessage.value = null
  successMessage.value = null
  try {
    const result = await processApi.killProcess(proc.pid, 'term')
    successMessage.value = result.terminated
      ? `进程 ${proc.pid} 已优雅终止`
      : `进程 ${proc.pid} 已结束`
    await loadProcesses()
  } catch (e: any) {
    errorMessage.value = e?.message || `结束进程 ${proc.pid} 失败`
    await loadProcesses()
  } finally {
    killing.value = 0
  }
}

function stateClass(state: string): string {
  if (state === '运行中') return 'running'
  if (state === '睡眠') return 'sleeping'
  if (state === '僵尸') return 'zombie'
  if (state === '已停止') return 'stopped'
  return 'default'
}

let mobileQuery: MediaQueryList
let mobileQueryListener: (e: MediaQueryListEvent) => void

onMounted(() => {
  mobileQuery = window.matchMedia('(max-width: 768px)')
  mobileQueryListener = (e) => {
    isMobile.value = e.matches
  }
  mobileQuery.addEventListener('change', mobileQueryListener)
  loadProcesses()
})

onUnmounted(() => {
  if (mobileQuery && mobileQueryListener) {
    mobileQuery.removeEventListener('change', mobileQueryListener)
  }
})
</script>

<style scoped>
.process-panel {
  max-width: 1200px;
  width: 95%;
  max-height: 90vh;
  display: flex;
  flex-direction: column;
}

.panel-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: var(--spacing-md) var(--spacing-xl);
  border-bottom: 1px solid var(--border-color);
  flex-shrink: 0;
}

.panel-header h3 {
  margin: 0;
  font-size: var(--font-size-xl);
  font-weight: 500;
  color: var(--text-color-1);
}

.close-btn {
  background: none;
  border: none;
  font-size: var(--font-size-5xl);
  cursor: pointer;
  color: var(--text-color-2);
  padding: 0;
  line-height: 1;
}

.close-btn:hover {
  color: var(--text-color-1);
}

.panel-body {
  padding: var(--spacing-xl);
  overflow-y: auto;
  flex: 1;
}

.error-banner,
.success-banner {
  display: flex;
  align-items: flex-start;
  gap: var(--spacing-sm);
  padding: var(--spacing-sm) var(--spacing-md);
  border-radius: var(--radius-xs);
  margin-bottom: var(--spacing-md);
}

.error-banner {
  background: var(--error-bg);
  border: 1px solid var(--error-color);
}

.success-banner {
  background: var(--success-bg);
  border: 1px solid var(--success-color);
}

.error-text {
  flex: 1;
  font-size: var(--font-size-base);
  color: var(--error-color);
  word-break: break-word;
}

.success-text {
  flex: 1;
  font-size: var(--font-size-base);
  color: var(--success-color);
  word-break: break-word;
}

.error-dismiss {
  background: none;
  border: none;
  font-size: var(--font-size-3xl);
  cursor: pointer;
  color: var(--text-color-3);
  padding: 0;
  line-height: 1;
}

.error-dismiss:hover {
  color: var(--text-color-1);
}

.toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: var(--spacing-md);
  margin-bottom: var(--spacing-lg);
  flex-wrap: wrap;
}

.search-box {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
  flex: 1;
  min-width: 240px;
}

.search-box input {
  flex: 1;
  padding: var(--spacing-sm) var(--spacing-md);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-xs);
  background: var(--bg-color-2);
  color: var(--text-color-1);
  font-size: var(--font-size-base);
}

.search-box input:focus {
  outline: none;
  border-color: var(--primary-color);
}

.search-btn {
  padding: var(--spacing-sm) var(--spacing-md);
  border: 1px solid var(--primary-color);
  border-radius: var(--radius-xs);
  background: var(--primary-color);
  color: var(--text-color-on-primary);
  cursor: pointer;
  font-size: var(--font-size-base);
  white-space: nowrap;
}

.search-btn:disabled,
.action-btn:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.toolbar-actions {
  display: flex;
  align-items: center;
  gap: var(--spacing-md);
}

.scope-toggle {
  display: flex;
  align-items: center;
  gap: var(--spacing-xs);
  font-size: var(--font-size-sm);
  color: var(--text-color-2);
  cursor: pointer;
  white-space: nowrap;
  user-select: none;
}

.scope-toggle input {
  accent-color: var(--primary-color);
}

.count-badge {
  font-size: var(--font-size-sm);
  color: var(--text-color-2);
  white-space: nowrap;
}

.action-btn {
  padding: var(--spacing-sm) var(--spacing-md);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-xs);
  background: var(--bg-color-2);
  color: var(--text-color-1);
  cursor: pointer;
  font-size: var(--font-size-base);
  white-space: nowrap;
}

.action-btn:hover:not(:disabled) {
  background: var(--bg-color-3);
}

.loading-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: var(--spacing-sm);
  padding: var(--spacing-xl);
  color: var(--text-color-2);
}

.process-shimmer {
  width: 100%;
  display: flex;
  flex-direction: column;
  gap: var(--spacing-xs);
}

.shimmer-row {
  height: 44px;
  border-radius: var(--radius-xs);
}

.loading-tip {
  color: var(--text-color-3);
  font-size: var(--font-size-sm);
  animation: pulse-text 2s ease-in-out infinite;
}

@keyframes pulse-text {
  0%, 100% { opacity: 0.5; }
  50% { opacity: 1; }
}

.process-list-header {
  display: grid;
  grid-template-columns: 1.6fr 70px 90px 80px 80px 90px 100px 130px;
  gap: var(--spacing-sm);
  padding: var(--spacing-sm) var(--spacing-md);
  font-size: var(--font-size-xs);
  color: var(--text-color-3);
  font-weight: 500;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  border-bottom: 1px solid var(--border-color);
  margin-bottom: var(--spacing-xs);
}

.sortable {
  cursor: pointer;
  user-select: none;
}

.sortable:hover {
  color: var(--primary-color);
}

.process-list {
  display: flex;
  flex-direction: column;
  gap: var(--spacing-xs);
}

.process-item {
  display: grid;
  grid-template-columns: 1.6fr 70px 90px 80px 80px 90px 100px 130px;
  gap: var(--spacing-sm);
  align-items: center;
  padding: var(--spacing-sm) var(--spacing-md);
  background: var(--bg-color-2);
  border-radius: var(--radius-xs);
  border: 1px solid var(--border-color);
  border-left: 3px solid var(--text-color-3);
  transition: all var(--transition-fast);
  font-size: var(--font-size-base);
}

.process-item:hover {
  border-color: var(--primary-color);
}

.process-item.protected {
  border-left-color: var(--warning-color);
}

.col-name {
  display: flex;
  align-items: center;
  gap: var(--spacing-xs);
  min-width: 0;
  flex-wrap: wrap;
}

.proc-name {
  font-weight: 500;
  font-size: var(--font-size-base);
  color: var(--text-color-1);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.protect-badge {
  font-size: var(--font-size-sm);
  flex-shrink: 0;
}

.docker-badge {
  display: inline-flex;
  align-items: center;
  flex-shrink: 0;
  padding: 0 6px;
  border-radius: var(--radius-2xs);
  background: var(--info-bg);
  border: 1px solid var(--info-color);
  color: var(--info-color);
  font-size: var(--font-size-2xs);
  font-weight: 500;
  line-height: 1.6;
  white-space: nowrap;
}

.protected-text {
  font-size: var(--font-size-xs);
  color: var(--warning-color);
  font-weight: 500;
}

.col-pid .pid {
  font-family: var(--font-mono);
  font-size: var(--font-size-sm);
  color: var(--text-color-1);
}

.col-user {
  font-size: var(--font-size-sm);
  color: var(--text-color-2);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.state-badge {
  font-size: var(--font-size-xs);
  padding: 1px 8px;
  border-radius: var(--radius-2xs);
  font-weight: 500;
  white-space: nowrap;
}

.state-badge.running {
  background: var(--success-bg);
  color: var(--success-color);
}

.state-badge.sleeping {
  background: var(--info-bg);
  color: var(--info-color);
}

.state-badge.zombie {
  background: var(--warning-bg);
  color: var(--warning-color);
}

.state-badge.stopped {
  background: var(--bg-color-4);
  color: var(--text-color-2);
}

.state-badge.default {
  background: var(--bg-color-4);
  color: var(--text-color-2);
}

.col-cpu .cpu-value {
  font-size: var(--font-size-sm);
  color: var(--text-color-1);
  font-family: var(--font-mono);
}

.col-mem {
  font-size: var(--font-size-sm);
  color: var(--text-color-2);
  font-family: var(--font-mono);
}

.ports {
  display: flex;
  flex-wrap: wrap;
  gap: 3px;
}

.port-badge {
  font-size: var(--font-size-2xs);
  font-family: var(--font-mono);
  padding: 0 5px;
  border-radius: var(--radius-2xs);
  background: var(--bg-color-4);
  color: var(--text-color-2);
  border: 1px solid var(--border-color);
}

.no-port {
  font-size: var(--font-size-sm);
  color: var(--text-color-3);
}

.kill-btn {
  padding: calc(var(--spacing-xs) / 2) var(--spacing-sm);
  border: 1px solid var(--error-color);
  border-radius: var(--radius-xs);
  background: transparent;
  color: var(--error-color);
  cursor: pointer;
  font-size: var(--font-size-xs);
  transition: all var(--transition-fast);
  white-space: nowrap;
}

.kill-btn:hover:not(:disabled) {
  background: var(--error-color);
  color: var(--text-color-on-primary);
}

.kill-btn:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.log-btn {
  padding: calc(var(--spacing-xs) / 2) var(--spacing-sm);
  border: 1px solid var(--primary-color);
  border-radius: var(--radius-xs);
  background: transparent;
  color: var(--primary-color);
  cursor: pointer;
  font-size: var(--font-size-xs);
  transition: all var(--transition-fast);
  white-space: nowrap;
}

.log-btn:hover:not(:disabled) {
  background: var(--primary-color);
  color: var(--text-color-on-primary);
}

.log-btn:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.col-action {
  display: flex;
  gap: var(--spacing-xs);
  align-items: center;
}

.no-processes {
  text-align: center;
  padding: var(--spacing-3xl);
  color: var(--text-color-3);
  font-size: var(--font-size-md);
}

.show-more {
  text-align: center;
  padding: var(--spacing-sm) 0;
}

.show-more-btn {
  padding: var(--spacing-xs) var(--spacing-md);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-xs);
  background: var(--bg-color-2);
  color: var(--primary-color);
  cursor: pointer;
  font-size: var(--font-size-sm);
  transition: all var(--transition-fast);
}

.show-more-btn:hover {
  border-color: var(--primary-color);
  background: var(--primary-bg);
}

/* ---- 进程日志查看覆盖层 ---- */
.process-log-layer {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.5);
  z-index: 1200;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: var(--spacing-lg);
}

.process-log-box {
  width: min(900px, 100%);
  height: 70vh;
  background: var(--bg-color-1);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-md);
  display: flex;
  flex-direction: column;
  overflow: hidden;
  box-shadow: var(--shadow-lg);
}

.process-log-box.full {
  height: 80vh;
}

.log-layer-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--spacing-sm);
  padding: var(--spacing-sm) var(--spacing-md);
  border-bottom: 1px solid var(--border-color);
  flex-shrink: 0;
}

.log-layer-title {
  font-size: var(--font-size-sm);
  color: var(--text-color-1);
  font-weight: 500;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.log-layer-actions {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
  margin-left: auto;
}

.log-line-count {
  font-size: var(--font-size-xs);
  color: var(--text-color-3);
  white-space: nowrap;
}

.mini-btn {
  padding: calc(var(--spacing-xs) / 2) var(--spacing-sm);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-xs);
  background: var(--bg-color-2);
  color: var(--text-color-1);
  cursor: pointer;
  font-size: var(--font-size-xs);
  transition: all var(--transition-fast);
  white-space: nowrap;
}

.mini-btn:hover {
  border-color: var(--primary-color);
}

.mini-btn.active {
  background: var(--primary-color);
  border-color: var(--primary-color);
  color: var(--text-color-on-primary);
}

.file-picker {
  flex: 1;
  overflow-y: auto;
  padding: var(--spacing-md);
}

.file-picker-hint {
  font-size: var(--font-size-sm);
  color: var(--text-color-2);
  margin-bottom: var(--spacing-md);
}

.file-list {
  display: flex;
  flex-direction: column;
  gap: var(--spacing-xs);
}

.file-item {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
  padding: var(--spacing-sm) var(--spacing-md);
  background: var(--bg-color-2);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-xs);
  cursor: pointer;
  transition: all var(--transition-fast);
}

.file-item:hover {
  border-color: var(--primary-color);
}

.file-name {
  font-size: var(--font-size-sm);
  font-weight: 500;
  color: var(--text-color-1);
  flex-shrink: 0;
}

.file-path {
  flex: 1;
  font-size: var(--font-size-xs);
  font-family: var(--font-mono);
  color: var(--text-color-3);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.file-size {
  font-size: var(--font-size-xs);
  font-family: var(--font-mono);
  color: var(--text-color-2);
  flex-shrink: 0;
}

.file-badge {
  font-size: var(--font-size-2xs);
  padding: 1px 8px;
  border-radius: var(--radius-2xs);
  flex-shrink: 0;
}

.file-badge.log {
  background: var(--success-bg);
  color: var(--success-color);
}

.file-badge.plain {
  background: var(--bg-color-4);
  color: var(--text-color-2);
}

.no-file {
  text-align: center;
  padding: var(--spacing-xl);
  color: var(--text-color-3);
  font-size: var(--font-size-sm);
}

.log-content-view {
  flex: 1;
  overflow: auto;
  background: var(--terminal-bg, #0d1117);
  padding: var(--spacing-xs) 0;
}

.log-line {
  display: flex;
  align-items: flex-start;
  padding: 0 var(--spacing-md);
  font-family: var(--font-mono);
  font-size: var(--font-size-xs);
  line-height: 1.7;
  color: var(--terminal-text, #c9d1d9);
  white-space: pre-wrap;
  word-break: break-all;
  border-left: 2px solid transparent;
}

.log-line:hover {
  background: rgba(255, 255, 255, 0.03);
}

.log-line .line-num {
  flex-shrink: 0;
  min-width: 48px;
  margin-right: var(--spacing-sm);
  color: var(--terminal-dim, #6e7681);
  user-select: none;
  text-align: right;
}

.log-line .line-text {
  flex: 1;
}

.log-line.match-line {
  background: rgba(255, 196, 0, 0.12);
}

.log-line.current-match {
  background: rgba(255, 196, 0, 0.28);
  border-left-color: var(--warning-color);
}

.log-empty {
  padding: var(--spacing-xl);
  text-align: center;
  color: var(--terminal-dim, #6e7681);
  font-size: var(--font-size-sm);
}

/* 日志搜索栏 */
.log-search {
  display: flex;
  align-items: center;
  gap: var(--spacing-xs);
}

.log-search input {
  width: 180px;
  padding: calc(var(--spacing-xs) / 2) var(--spacing-sm);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-xs);
  background: var(--bg-color-2);
  color: var(--text-color-1);
  font-size: var(--font-size-xs);
}

.log-search input:focus {
  outline: none;
  border-color: var(--primary-color);
}

.search-count {
  font-size: var(--font-size-2xs);
  color: var(--text-color-3);
  white-space: nowrap;
  min-width: 40px;
  text-align: center;
}

.search-nav {
  width: 24px;
  height: 24px;
  display: flex;
  align-items: center;
  justify-content: center;
  border: 1px solid var(--border-color);
  border-radius: var(--radius-xs);
  background: var(--bg-color-2);
  color: var(--text-color-1);
  cursor: pointer;
  font-size: var(--font-size-xs);
  line-height: 1;
}

.search-nav:hover:not(:disabled) {
  border-color: var(--primary-color);
  color: var(--primary-color);
}

.search-nav:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}

@media (max-width: 768px) {
  .process-panel {
    max-width: 100%;
    width: 100%;
    height: 95vh;
    max-height: 95vh;
    border-radius: var(--radius-lg) var(--radius-lg) 0 0;
    margin-top: auto;
  }

  .panel-header {
    padding: var(--spacing-sm) var(--spacing-md);
  }

  .panel-header h3 {
    font-size: var(--font-size-lg);
  }

  .panel-body {
    padding: var(--spacing-md);
  }

  .toolbar {
    flex-direction: column;
    align-items: stretch;
  }

  .toolbar-actions {
    justify-content: space-between;
    flex-wrap: wrap;
  }

  .mobile-sort {
    width: 100%;
    margin-bottom: var(--spacing-xs);
  }

  .sort-select {
    width: 100%;
    padding: var(--spacing-sm) var(--spacing-md);
    border: 1px solid var(--border-color);
    border-radius: var(--radius-xs);
    background: var(--bg-color-2);
    color: var(--text-color-1);
    font-size: var(--font-size-base);
  }

  .process-list-header {
    display: none;
  }

  .process-item {
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    gap: var(--spacing-xs);
    padding: var(--spacing-sm) var(--spacing-md);
  }

  .col-name,
  .col-pid,
  .col-user,
  .col-state,
  .col-cpu,
  .col-mem,
  .col-port,
  .col-action {
    display: flex;
    align-items: center;
    gap: var(--spacing-sm);
    width: 100%;
    font-size: var(--font-size-sm);
  }

  .col-user::before {
    content: '用户:';
    color: var(--text-color-3);
    font-size: var(--font-size-xs);
    min-width: 36px;
  }

  .col-state::before {
    content: '状态:';
    color: var(--text-color-3);
    font-size: var(--font-size-xs);
    min-width: 36px;
  }

  .col-cpu::before {
    content: 'CPU:';
    color: var(--text-color-3);
    font-size: var(--font-size-xs);
    min-width: 36px;
  }

  .col-mem::before {
    content: '内存:';
    color: var(--text-color-3);
    font-size: var(--font-size-xs);
    min-width: 36px;
  }

  .col-port::before {
    content: '端口:';
    color: var(--text-color-3);
    font-size: var(--font-size-xs);
    min-width: 36px;
  }

  .proc-name {
    white-space: normal;
    word-break: break-all;
  }

  .kill-btn {
    width: 100%;
    text-align: center;
    padding: var(--spacing-xs) var(--spacing-sm);
  }
}
</style>
