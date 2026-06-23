<!-- eslint-disable vue/no-v-html -->
<template>
  <div class="modal active non-blocking">
    <div class="modal-content large">
      <div class="tab-bar" v-if="logsStore.logTabs.length > 0" @click.right.prevent="closeContextMenu">
        <template v-for="(tab, idx) in logsStore.logTabs" :key="tab.id">
          <div class="tab-group-divider" v-if="showGroupDivider(idx)" title="分组边界"></div>
          <div 
            :class="['tab-item', { active: tab.id === logsStore.activeTabId }]"
            :style="tabStyle(tab)"
            @click="handleSwitchTab(tab.id)"
            @contextmenu.prevent.stop="openContextMenu($event, tab)"
          >
            <span class="tab-indicator" :style="{ background: tabColor(tab.filePath) }"></span>
            <span class="tab-title" :title="tab.filePath">{{ tab.title }}</span>
            <span class="tab-close" @click.stop="handleCloseTab(tab.id)">×</span>
          </div>
        </template>
      </div>
      <!-- 右键菜单 -->
      <div 
        class="tab-context-menu" 
        v-if="contextMenu.visible"
        :style="{ left: contextMenu.x + 'px', top: contextMenu.y + 'px' }"
        @click.right.prevent
      >
        <div class="context-item" @click="closeCurrentTab">关闭当前</div>
        <div class="context-item" @click="closeOtherTabs">关闭其他</div>
        <div class="context-item" @click="closeAllTabs">关闭全部</div>
        <div class="context-divider"></div>
        <div class="context-item" @click="closeTabsToRight">关闭右侧标签</div>
      </div>
      <div class="modal-header">
        <span class="title">{{ title }}</span>
        <div class="header-actions">
          <span class="line-count">{{ totalLines }} 行{{ truncated ? ` / 共 ${totalLinesInFile} 行` : '' }}</span>
          <span class="tail-error" v-if="streamError">{{ streamError }}</span>
          <button
            class="action-btn"
            :class="{ 'tail-active': isTailing }"
            @click="toggleTail"
            :title="isTailing ? '停止追踪' : '实时追踪'"
          >
            <span class="action-icon">{{ isTailing ? '■' : '◉' }}</span>
            <span class="action-text">{{ isTailing ? '停止' : '追踪' }}</span>
          </button>

          <button v-if="!isDocker" class="action-btn" @click="handleClear" title="清空日志">
            <span class="action-icon">✕</span>
            <span class="action-text">清空</span>
          </button>

          <div class="action-dropdown">
            <button class="action-btn" @click="toggleExportMenu" title="导出日志">
              <span class="action-icon">↓</span>
              <span class="action-text">导出</span>
            </button>
            <div class="dropdown-menu" v-if="showExportMenu">
              <button class="dropdown-option" @click="handleExport('txt')">TXT 纯文本</button>
              <button class="dropdown-option" @click="handleExport('json')">JSON 结构化</button>
              <button class="dropdown-option" @click="handleExport('csv')">CSV 表格</button>
            </div>
          </div>
          <button class="action-btn" @click="$emit('addBookmark')" title="添加书签">
            <span class="action-icon">☆</span>
            <span class="action-text">书签</span>
          </button>
          <button class="action-btn back-btn" @click="$emit('back')" title="返回主页">
            <span class="action-icon">↩</span>
            <span class="action-text">主页</span>
          </button>
          <button class="close-btn" @click="$emit('close')">×</button>
        </div>
      </div>
      <div class="truncated-warning" v-if="truncated">
        <span class="warning-icon">⚠</span>
        <span>日志已截断，仅显示最后 {{ totalLines }} 行</span>
        <button class="load-all-btn" @click="$emit('loadAll')" :disabled="loadingAll">
          {{ loadingAll ? '加载中...' : '加载全部' }}
        </button>
      </div>
      <div class="modal-search">
        <div class="search-mode-segment">
          <button 
            class="segment-btn" 
            :class="{ active: searchOptions.mode === 'keyword' }"
            @click="searchOptions.mode = 'keyword'"
          >关键词</button>
          <button 
            class="segment-btn" 
            :class="{ active: searchOptions.mode === 'regex' }"
            @click="searchOptions.mode = 'regex'"
          >正则</button>
        </div>
        <input 
          type="text" 
          v-model="searchQuery" 
          :placeholder="searchOptions.mode === 'regex' ? '输入正则表达式...' : '搜索日志内容...'"
          class="search-input"
          :class="{ 'regex-mode': searchOptions.mode === 'regex' }"
          :disabled="isTailing"
        >
        <button class="clear-btn" v-if="searchQuery" @click="clearSearch" title="清除搜索">×</button>
        <div class="search-nav" v-if="searchQuery && matchPositions.length > 0">
          <span class="match-info">{{ currentMatchIndex + 1 }} / {{ matchPositions.length }}</span>
          <button class="nav-btn" @click="prevMatch" :disabled="currentMatchIndex <= 0" title="上一个匹配">↑</button>
          <button class="nav-btn" @click="nextMatch" :disabled="currentMatchIndex >= matchPositions.length - 1" title="下一个匹配">↓</button>
        </div>
        <span class="match-count" v-if="searchQuery && matchPositions.length === 0 && !searchError">
          无匹配
        </span>
        <span class="search-error" v-if="searchError">
          {{ searchError }}
        </span>
        <div class="jump-nav" v-if="!searchQuery">
          <button class="jump-btn" @click="goToFirstLine" title="跳转到首行">↑</button>
          <button class="jump-btn" @click="goToLastLine" title="跳转到末行">↓</button>
        </div>
      </div>
      <div class="modal-body" ref="logBody" @scroll="handleScroll">
        <div class="virtual-scroll-container" :style="{ height: totalHeight }">
          <div class="virtual-scroll-content" :style="{ transform: `translateY(${offsetY}px)` }">
            <pre class="log-viewer"><div 
              v-for="item in visibleLines"
              :key="item.index"
              class="log-line"
              :class="{ 'has-match': lineHasMatch(item.index), 'current-match': currentLineIndex === item.index }"
            ><span class="line-number">{{ item.index + 1 }}</span><span class="line-content" v-html="formatLine(item.line, item.index)"></span></div></pre>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, nextTick, onMounted, onUnmounted, reactive } from 'vue'
import DOMPurify from 'dompurify'
import { useLogSearch } from '../composables/useLogSearch'
import { useLogStream } from '../composables/useLogStream'
import { useDockerLogStream } from '../composables/useDockerLogStream'
import { useLogsStore } from '../stores/useLogsStore'
interface Props {
  title?: string
  content?: string
  truncated?: boolean
  totalLinesInFile?: number
  loadingAll?: boolean
  isDocker?: boolean
  containerName?: string
  filePath?: string
}

const props = withDefaults(defineProps<Props>(), {
  title: '',
  content: '',
  truncated: false,
  totalLinesInFile: 0,
  loadingAll: false,
  isDocker: false,
  containerName: '',
  filePath: ''
})

const logsStore = useLogsStore()

function handleSwitchTab(tabId: string): void {
  closeContextMenu()
  logsStore.switchTab(tabId)
}

function handleCloseTab(tabId: string): void {
  closeContextMenu()
  logsStore.removeTab(tabId)
}

const emit = defineEmits<{
  close: []
  back: []
  loadAll: []
  export: [format: string]
  addBookmark: []
  truncate: []
}>()

/** 标签页色彩方案 */
const TAB_COLORS = [
  '#5b9bd5', '#70ad47', '#ed7d31', '#ffc000', '#4472c4',
  '#a5a5a5', '#ff4081', '#00bcd4', '#8bc34a', '#ff9800',
  '#9c27b0', '#00acc1', '#e91e63', '#4caf50', '#3f51b5'
]

/** 从文件路径生成一致的色彩索引 */
function tabColor(filePath: string): string {
  let hash = 0
  for (let i = 0; i < filePath.length; i++) {
    hash = ((hash << 5) - hash) + filePath.charCodeAt(i)
    hash |= 0
  }
  return TAB_COLORS[Math.abs(hash) % TAB_COLORS.length]
}

/** 标签内联样式 */
function tabStyle(tab: { filePath: string; id: string }): Record<string, string> {
  const color = tabColor(tab.filePath)
  return {
    '--tab-color': color,
    '--tab-color-dim': color + '33'
  }
}

/** 提取标签分组 key（基于应用目录） */
function tabGroupKey(tab: { filePath: string; isDocker?: boolean }): string {
  if (tab.isDocker) return '__docker__'
  const parts = tab.filePath.replace(/\\/g, '/').split('/')
  // 尝试取 appName（/var/log/apps/<appName>/... 或 /vol*/@appdata/<appName>/...）
  for (let i = 0; i < parts.length; i++) {
    if (parts[i] === 'apps' && i + 1 < parts.length) return parts[i + 1]
    if (parts[i]?.startsWith('@app') && i + 1 < parts.length) return parts[i + 1]
  }
  return tab.filePath
}

/** 是否在两组之间显示分隔线 */
function showGroupDivider(idx: number): boolean {
  if (idx === 0) return false
  const tabs = logsStore.logTabs
  if (idx >= tabs.length) return false
  return tabGroupKey(tabs[idx]) !== tabGroupKey(tabs[idx - 1])
}

/** 右键菜单状态 */
const contextMenu = ref<{ visible: boolean; x: number; y: number; tab: { id: string } | null }>({
  visible: false,
  x: 0,
  y: 0,
  tab: null
})

function openContextMenu(event: MouseEvent, tab: { id: string }): void {
  contextMenu.value = { visible: true, x: event.clientX, y: event.clientY, tab }
}

function closeContextMenu(): void {
  contextMenu.value.visible = false
}

function closeCurrentTab(): void {
  if (contextMenu.value.tab) {
    logsStore.removeTab(contextMenu.value.tab.id)
  }
  closeContextMenu()
}

function closeOtherTabs(): void {
  if (!contextMenu.value.tab) return
  const keepId = contextMenu.value.tab.id
  const toRemove = logsStore.logTabs.filter(t => t.id !== keepId).map(t => t.id)
  for (const id of toRemove) {
    logsStore.removeTab(id)
  }
  closeContextMenu()
}

function closeAllTabs(): void {
  const ids = logsStore.logTabs.map(t => t.id)
  for (const id of ids) {
    logsStore.removeTab(id)
  }
  closeContextMenu()
}

function closeTabsToRight(): void {
  if (!contextMenu.value.tab) return
  const idx = logsStore.logTabs.findIndex(t => t.id === contextMenu.value.tab?.id)
  if (idx < 0) return
  const toRemove = logsStore.logTabs.slice(idx + 1).map(t => t.id)
  for (const id of toRemove) {
    logsStore.removeTab(id)
  }
  closeContextMenu()
}

/** 点击页面任意位置关闭右键菜单 */
onMounted(() => {
  document.addEventListener('click', closeContextMenu)
})
onUnmounted(() => {
  document.removeEventListener('click', closeContextMenu)
})

const searchQuery = ref('')
const currentMatchIndex = ref(0)
const logBody = ref<HTMLElement | null>(null)
const currentLineIndex = ref(-1)
const showExportMenu = ref(false)

const searchOptions = reactive({
  mode: 'keyword' as 'keyword' | 'regex'
})

// 文件日志 WebSocket 流
const logStream = useLogStream()
// Docker 日志 WebSocket 流
const dockerLogStream = useDockerLogStream()

const isTailing = ref(false)
let autoScrollToBottom = false

// 使用 streamingContent 替换 tailContent，streamError 替换 tailError
const streamingContent = computed(() =>
  props.isDocker ? dockerLogStream.streamingContent.value : logStream.streamingContent.value
)
const streamError = computed(() =>
  props.isDocker ? dockerLogStream.streamError.value : logStream.streamError.value
)

const LINE_HEIGHT = 24
const BUFFER_SIZE = 10

interface VisibleRange {
  start: number
  end: number
}

const visibleRange = ref<VisibleRange>({ start: 0, end: 50 })

const allLines = computed(() => {
  const content = props.content || ''
  const tailLines = isTailing.value && streamingContent.value.length
    ? '\n' + streamingContent.value.join('\n')
    : ''
  const fullContent = content + tailLines
  if (!fullContent || fullContent === '(空文件)') return []
  return fullContent.split('\n')
})

const searchOptionsRef = computed(() => ({
  mode: searchOptions.mode
}))

const { matchPositions, isSearching, searchError, cleanup: cleanupSearch } = useLogSearch(allLines, searchQuery, searchOptionsRef)

const totalLines = computed(() => allLines.value.length)

const totalHeight = computed(() => Math.max(totalLines.value * LINE_HEIGHT, 100))

const offsetY = computed(() => visibleRange.value.start * LINE_HEIGHT)

interface VisibleLine {
  index: number
  line: string
}

const visibleLines = computed(() => {
  const { start, end } = visibleRange.value
  const lines: VisibleLine[] = []
  const maxEnd = Math.min(end, allLines.value.length - 1)

  for (let i = start; i <= maxEnd && i < allLines.value.length; i++) {
    if (allLines.value[i] === undefined) continue
    lines.push({
      index: i,
      line: allLines.value[i]
    })
  }
  return lines
})

watch(searchQuery, () => {
  currentMatchIndex.value = 0
  currentLineIndex.value = -1
  if (matchPositions.value.length > 0) {
    currentLineIndex.value = matchPositions.value[0].lineIndex
    nextTick(() => scrollToLine(currentLineIndex.value))
  }
})

watch(() => props.content, () => {
  nextTick(() => {
    visibleRange.value = { start: 0, end: 50 }
    handleScroll()
  })
}, { immediate: true })

function handleScroll(): void {
  if (!logBody.value) return
  if (autoScrollToBottom) return
  
  const scrollTop = logBody.value.scrollTop
  const clientHeight = logBody.value.clientHeight
  
  const startLine = Math.max(0, Math.floor(scrollTop / LINE_HEIGHT) - BUFFER_SIZE)
  const endLine = Math.min(
    allLines.value.length - 1,
    Math.ceil((scrollTop + clientHeight) / LINE_HEIGHT) + BUFFER_SIZE
  )
  
  visibleRange.value = { start: startLine, end: endLine }
}

function clearSearch(): void {
  searchQuery.value = ''
  currentLineIndex.value = -1
}

function lineHasMatch(index: number): boolean {
  return matchPositions.value.some(p => p.lineIndex === index)
}

function formatLine(line: string, lineIndex: number): string {
  if (!line) return ''
  const escaped = escapeHtml(line)
  if (!searchQuery.value) return escaped

  const query = searchQuery.value

  let highlightRegex: RegExp
  if (!query || query.length > 500) return escaped
  if (searchOptions.mode === 'regex') {
    // ReDoS 防护
    if (/\(.+[+*?]\{[0-9]|\([^)]+\)[+*?]+\(|[+*?]\{[0-9]{3,}/.test(query)) return escaped
    try {
      highlightRegex = new RegExp(query, 'gi')
    } catch {
      return escaped
    }
  } else {
    if (query.length > 200) return escaped
    highlightRegex = new RegExp(`(${escapeRegex(query)})`, 'gi')
  }

  const isCurrentLine = lineIndex === currentLineIndex.value
  const currentPos = isCurrentLine ? matchPositions.value[currentMatchIndex.value] : null
  const isCurrentMatch = currentPos && currentPos.lineIndex === lineIndex

  if (isCurrentMatch) {
    const charIndex = currentPos.charIndex
    const matchLen = searchOptions.mode === 'regex'
      ? getRegexMatchLength(line, charIndex)
      : query.length
    const before = escapeHtml(line.substring(0, charIndex))
    const match = escapeHtml(line.substring(charIndex, charIndex + matchLen))
    const after = escapeHtml(line.substring(charIndex + matchLen))
    const beforeHighlighted = before.replace(highlightRegex, '<mark class="highlight">$1</mark>')
    const afterHighlighted = after.replace(highlightRegex, '<mark class="highlight">$1</mark>')
    const html = beforeHighlighted + '<mark class="highlight current">' + match + '</mark>' + afterHighlighted
    return DOMPurify.sanitize(html, {
      ALLOWED_TAGS: ['mark'],
      ALLOWED_ATTR: ['class']
    })
  }

  const highlightedHtml = escaped.replace(highlightRegex, '<mark class="highlight">$1</mark>')
  return DOMPurify.sanitize(highlightedHtml, {
    ALLOWED_TAGS: ['mark'],
    ALLOWED_ATTR: ['class']
  })
}

function getRegexMatchLength(line: string, charIndex: number): number {
  try {
    const regex = new RegExp(searchQuery.value, 'gi')
    regex.lastIndex = 0
    let match = regex.exec(line)
    while (match !== null) {
      if (match.index === charIndex) return match[0].length
      if (match[0].length === 0) regex.lastIndex++
      match = regex.exec(line)
    }
  } catch { /* ignore */ }
  return searchQuery.value.length
}

function escapeHtml(text: string): string {
  if (!text) return ''
  const ansiRegex = /\x1b\[[0-9;]*m/g
  return text
    .replace(ansiRegex, '')
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;')
}

function escapeRegex(str: string): string {
  return str.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
}

function scrollToLine(lineIndex: number): void {
  if (!logBody.value) return
  
  const targetTop = lineIndex * LINE_HEIGHT
  logBody.value.scrollTo({
    top: targetTop - logBody.value.clientHeight / 2 + LINE_HEIGHT,
    behavior: 'smooth'
  })
}

function nextMatch(): void {
  if (currentMatchIndex.value < matchPositions.value.length - 1) {
    currentMatchIndex.value++
    currentLineIndex.value = matchPositions.value[currentMatchIndex.value].lineIndex
    nextTick(() => scrollToLine(currentLineIndex.value))
  }
}

function prevMatch(): void {
  if (currentMatchIndex.value > 0) {
    currentMatchIndex.value--
    currentLineIndex.value = matchPositions.value[currentMatchIndex.value].lineIndex
    nextTick(() => scrollToLine(currentLineIndex.value))
  }
}

function goToFirstLine(): void {
  if (!logBody.value || allLines.value.length === 0) return
  visibleRange.value = { start: 0, end: 50 }
  requestAnimationFrame(() => {
    setTimeout(() => {
      logBody.value?.scrollTo({
        top: 0,
        behavior: 'auto'
      })
    }, 100)
  })
}

function goToLastLine(): void {
  if (!logBody.value || allLines.value.length === 0) return
  const lastLineIndex = allLines.value.length - 1
  visibleRange.value = { 
    start: Math.max(0, lastLineIndex - 50), 
    end: lastLineIndex + 10 
  }
  requestAnimationFrame(() => {
    setTimeout(() => {
      logBody.value?.scrollTo({
        top: logBody.value.scrollHeight,
        behavior: 'auto'
      })
    }, 100)
  })
}

function toggleExportMenu(): void {
  showExportMenu.value = !showExportMenu.value
}

function handleExport(format: string): void {
  showExportMenu.value = false
  emit('export', format)
}

function handleClear(): void {
  emit('truncate')
}

function handleClickOutside(e: MouseEvent): void {
  const target = e.target as HTMLElement
  if (!target.closest('.action-dropdown')) {
    showExportMenu.value = false
  }
}

function toggleTail(): void {
  if (isTailing.value) {
    stopTail()
    return
  }

  const filePathToSend = props.isDocker ? props.containerName : props.filePath
  if (!filePathToSend) {
    return
  }

  // 清空之前的内容
  logStream.clearContent()
  dockerLogStream.clearContent()

  isTailing.value = true
  scrollToBottom()

  if (props.isDocker) {
    dockerLogStream.subscribe(filePathToSend)
  } else {
    logStream.subscribe(filePathToSend)
  }
}

function scrollToBottom(): void {
  const lineCount = allLines.value.length
  const start = Math.max(0, lineCount - 50)
  visibleRange.value = { start, end: lineCount + 50 }
  autoScrollToBottom = true
  nextTick(() => {
    if (logBody.value) {
      logBody.value.scrollTop = logBody.value.scrollHeight
    }
    nextTick(() => {
      autoScrollToBottom = false
    })
  })
}

function stopTail(): void {
  isTailing.value = false
  autoScrollToBottom = false
  logStream.unsubscribe()
  dockerLogStream.unsubscribe()
}

// WebSocket 推送新内容时自动滚到底部
// 使用 deep:true 因为 composable 内用 push() 原地修改数组，引用不变
watch(streamingContent, () => {
  if (isTailing.value && logBody.value) {
    nextTick(() => {
      if (logBody.value) {
        logBody.value.scrollTop = logBody.value.scrollHeight
      }
    })
  }
}, { deep: true })

onMounted(() => {
  handleScroll()
  document.addEventListener('click', handleClickOutside)
})

onUnmounted(() => {
  stopTail()
  logStream.disconnect()
  dockerLogStream.disconnect()
  cleanupSearch()
  document.removeEventListener('click', handleClickOutside)
})
</script>

<style scoped>
.tab-bar {
  display: flex;
  align-items: center;
  background: var(--bg-color-2);
  border-bottom: 1px solid var(--bg-color-4);
  padding: 0 var(--spacing-sm);
  overflow-x: auto;
  flex-shrink: 0;
  gap: var(--spacing-xs);
}
.tab-item {
  display: flex;
  align-items: center;
  gap: var(--spacing-xs);
  padding: var(--spacing-sm) var(--spacing-md);
  cursor: pointer;
  white-space: nowrap;
  border-bottom: 2px solid transparent;
  font-size: var(--font-size-base);
  color: var(--text-color-2);
  transition: all var(--transition-fast);
  min-width: 0;
  border-radius: var(--radius-xs) var(--radius-xs) 0 0;
}
.tab-item:hover {
  color: var(--text-color-1);
  background: var(--info-bg);
}
.tab-item.active {
  color: var(--tab-color, var(--primary-color));
  border-bottom-color: var(--tab-color, var(--primary-color));
  font-weight: 500;
  background: var(--bg-color-1);
}
.tab-indicator {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  flex-shrink: 0;
  opacity: 0.6;
}
.tab-item.active .tab-indicator {
  opacity: 1;
}
.tab-title {
  overflow: hidden;
  text-overflow: ellipsis;
  max-width: 160px;
}
.tab-close {
  font-size: var(--font-size-sm);
  line-height: 1;
  width: 16px;
  height: 16px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  color: var(--text-color-3);
  transition: all var(--transition-fast);
}
.tab-close:hover {
  color: var(--error-color);
  background: var(--error-bg);
}
.tab-group-divider {
  width: 1px;
  height: 20px;
  background: var(--border-color);
  flex-shrink: 0;
  margin: 0 var(--spacing-xs);
  opacity: 0.5;
}
.tab-context-menu {
  position: fixed;
  z-index: 1300;
  background: var(--card-bg);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-sm);
  box-shadow: var(--shadow-lg);
  padding: var(--spacing-xs) 0;
  min-width: 140px;
}
.context-item {
  padding: var(--spacing-xs) var(--spacing-lg);
  cursor: pointer;
  font-size: var(--font-size-sm);
  color: var(--text-color-1);
  transition: background var(--transition-fast);
  white-space: nowrap;
}
.context-item:hover {
  background: var(--primary-color);
  color: white;
}
.context-divider {
  height: 1px;
  background: var(--divider-color);
  margin: var(--spacing-xs) 0;
}
.modal {
  position: fixed;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  z-index: 1000;
}
.modal.non-blocking {
  pointer-events: none;
  background: transparent;
}
.modal.non-blocking .modal-content {
  pointer-events: auto;
}
.modal.active {
  height: 100%;
  background: var(--overlay);
  display: flex;
  justify-content: center;
  align-items: center;
  z-index: 1100;
}

.modal-content.large {
  width: 90%;
  max-width: 1200px;
  height: 85vh;
  max-height: 85vh;
  background: var(--card-bg);
  border-radius: var(--radius-md);
  overflow: hidden;
  display: flex;
  flex-direction: column;
}

.modal-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: var(--spacing-md) var(--spacing-xl);
  background: var(--bg-color-2);
  border-bottom: 1px solid var(--border-color);
}

.modal-header .title {
  font-weight: 600;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  flex: 1;
  color: var(--text-color-1);
}

.header-actions {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
}

.line-count {
  font-size: var(--font-size-sm);
  color: var(--text-color-3);
  background: var(--bg-color-3);
  padding: var(--spacing-xs) var(--spacing-sm);
  border-radius: var(--radius-xs);
}

.close-btn {
  background: none;
  border: none;
  font-size: var(--font-size-5xl);
  cursor: pointer;
  color: var(--text-color-3);
  padding: 0;
  line-height: 1;
  transition: color var(--transition-fast);
}

.close-btn:hover {
  color: var(--text-color-1);
}

.action-btn {
  display: inline-flex;
  align-items: center;
  gap: var(--spacing-xs);
  padding: var(--spacing-xs) var(--spacing-sm);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-xs);
  background: transparent;
  color: var(--text-color-2);
  font-size: var(--font-size-sm);
  cursor: pointer;
  line-height: 1.4;
  transition: all var(--transition-fast);
  white-space: nowrap;
  font-weight: 500;
}

.action-btn:hover {
  border-color: var(--primary-color);
  color: var(--primary-color);
  background: var(--info-bg);
}

.action-btn:active {
  transform: scale(0.96);
}

.action-icon {
  font-size: var(--font-size-base);
  line-height: 1;
}

.action-text {
  font-size: var(--font-size-sm);
}

.action-btn.tail-active {
  border-color: var(--error-color);
  color: white;
  background: var(--error-color);
  animation: tail-pulse 2s ease-in-out infinite;
}

.action-btn.tail-active:hover {
  background: var(--log-critical-color);
  border-color: var(--log-critical-color);
  color: white;
}

@keyframes tail-pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.85; }
}

.tail-error {
  color: var(--error-color);
  font-size: var(--font-size-xs);
  max-width: 160px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.action-dropdown {
  position: relative;
}

.dropdown-menu {
  position: absolute;
  top: calc(100% + var(--spacing-xs));
  right: 0;
  background: var(--card-bg);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-sm);
  box-shadow: var(--shadow-md);
  z-index: 1200;
  min-width: 140px;
  overflow: hidden;
  animation: dropdown-enter var(--transition-fast) ease-out;
}

@keyframes dropdown-enter {
  from {
    opacity: 0;
    transform: translateY(-4px) scale(0.96);
  }
  to {
    opacity: 1;
    transform: translateY(0) scale(1);
  }
}

.dropdown-option {
  display: block;
  width: 100%;
  padding: var(--spacing-sm) var(--spacing-md);
  border: none;
  background: none;
  text-align: left;
  font-size: var(--font-size-sm);
  color: var(--text-color-2);
  cursor: pointer;
  transition: all var(--transition-fast);
  font-weight: 400;
}

.dropdown-option:hover {
  background: var(--info-bg);
  color: var(--primary-color);
}

.dropdown-option + .dropdown-option {
  border-top: 1px solid var(--divider-color);
}

.truncated-warning {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
  padding: var(--spacing-sm) var(--spacing-xl);
  background: var(--warning-bg);
  border-bottom: 1px solid var(--warning-color);
  color: var(--text-color-1);
  font-size: var(--font-size-base);
}

.warning-icon {
  font-size: var(--font-size-xl);
}

.load-all-btn {
  margin-left: auto;
  padding: var(--spacing-xs) var(--spacing-md);
  background: var(--warning-color);
  border: none;
  border-radius: var(--radius-xs);
  color: white;
  font-size: var(--font-size-sm);
  cursor: pointer;
  font-weight: 500;
  transition: background var(--transition-fast);
}

.load-all-btn:hover:not(:disabled) {
  background: var(--primary-pressed);
}

.load-all-btn:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.modal-search {
  padding: var(--spacing-sm) var(--spacing-xl);
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
  font-size: var(--font-size-base);
  background: var(--card-bg);
  color: var(--text-color-1);
  transition: border-color var(--transition-fast), box-shadow var(--transition-fast);
}

.search-input:focus {
  outline: none;
  border-color: var(--primary-color);
  box-shadow: 0 0 0 2px var(--info-bg);
}

.search-input.regex-mode {
  border-color: var(--warning-color);
}

.search-input.regex-mode:focus {
  border-color: var(--warning-color);
  box-shadow: 0 0 0 2px var(--warning-bg);
}

.search-mode-segment {
  display: flex;
  background: var(--bg-color-3);
  border-radius: var(--radius-xs);
  padding: 2px;
  flex-shrink: 0;
}

.segment-btn {
  padding: var(--spacing-xs) var(--spacing-sm);
  border: none;
  background: transparent;
  color: var(--text-color-3);
  font-size: var(--font-size-xs);
  cursor: pointer;
  border-radius: calc(var(--radius-xs) - 2px);
  transition: all var(--transition-fast);
  font-weight: 500;
  white-space: nowrap;
  line-height: 1.4;
}

.segment-btn.active {
  background: var(--primary-color);
  color: white;
  box-shadow: var(--shadow-sm);
}

.segment-btn:hover:not(.active) {
  color: var(--text-color-1);
}

.search-error {
  font-size: var(--font-size-sm);
  color: var(--error-color);
  white-space: nowrap;
  font-weight: 500;
}

.clear-btn {
  background: var(--bg-color-3);
  border: none;
  color: var(--text-color-2);
  font-size: var(--font-size-md);
  cursor: pointer;
  padding: var(--spacing-xs) var(--spacing-sm);
  border-radius: var(--radius-xs);
  line-height: 1;
  min-width: 28px;
  max-width: 28px;
  transition: all var(--transition-fast);
}

.clear-btn:hover {
  background: var(--primary-color);
  color: white;
}

.search-nav {
  display: flex;
  align-items: center;
  gap: var(--spacing-xs);
}

.match-info {
  font-size: var(--font-size-sm);
  color: var(--text-color-3);
  min-width: 70px;
}

.nav-btn {
  padding: var(--spacing-xs) var(--spacing-sm);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-xs);
  background: var(--card-bg);
  cursor: pointer;
  font-size: var(--font-size-sm);
  color: var(--text-color-1);
  min-width: 65px;
  transition: all var(--transition-fast);
  font-weight: 500;
}

.nav-btn:hover:not(:disabled) {
  background: var(--primary-color);
  color: white;
  border-color: var(--primary-color);
}

.nav-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.match-count {
  font-size: var(--font-size-sm);
  color: var(--text-color-3);
  white-space: nowrap;
}

.jump-nav {
  display: flex;
  align-items: center;
  gap: var(--spacing-xs);
}

.jump-btn {
  padding: var(--spacing-xs) var(--spacing-sm);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-xs);
  background: var(--card-bg);
  cursor: pointer;
  font-size: var(--font-size-sm);
  color: var(--text-color-1);
  transition: all var(--transition-fast);
}

.jump-btn:hover {
  background: var(--primary-color);
  color: white;
  border-color: var(--primary-color);
}

.modal-body {
  flex: 1;
  overflow: auto;
  padding: 0;
  min-height: 300px;
}

.virtual-scroll-container {
  position: relative;
  width: 100%;
  min-width: max-content;
}

.virtual-scroll-content {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  min-width: max-content;
}

.log-viewer {
  margin: 0;
  background: #1a1a2e;
  min-height: 100%;
  font-family: 'Monaco', 'Menlo', 'Consolas', monospace;
  font-size: 12px;
  line-height: 24px;
  min-width: max-content;
}

.log-line {
  display: flex;
  padding: 0 15px;
  height: 24px;
  min-width: max-content;
}

.log-line:hover {
  background: rgba(255, 255, 255, 0.06);
}

.log-line.has-match {
  background: rgba(255, 255, 255, 0.04);
}

.log-line.current-match {
  background: rgba(10, 89, 247, 0.3);
}

.line-number {
  color: #5a6a7a;
  min-width: 50px;
  text-align: right;
  padding-right: var(--spacing-md);
  user-select: none;
  border-right: 1px solid #2a2a3e;
  margin-right: var(--spacing-md);
  flex-shrink: 0;
}

.line-content {
  color: #d4d4d4;
  white-space: pre;
  flex: none;
}

:deep(.highlight) {
  background: #e8b339;
  color: #1a1a2e;
  padding: 0 2px;
  border-radius: var(--radius-3xs);
}

:deep(.highlight.current) {
  background: #0a59f7;
  color: white;
  padding: 0 2px;
  border-radius: var(--radius-3xs);
  font-weight: bold;
}

@media (max-width: 768px) {
  .modal {
    align-items: flex-end;
  }

  .modal-content.large {
    width: 100%;
    max-height: 95vh;
    height: 95vh;
    border-radius: var(--radius-lg) var(--radius-lg) 0 0;
  }

  .modal-header {
    padding: var(--spacing-sm) var(--spacing-md);
    flex-wrap: nowrap;
    gap: var(--spacing-sm);
    min-height: auto;
    background: var(--bg-color-2);
    border-bottom: 1px solid var(--border-color);
  }

  .modal-header .title {
    font-size: var(--font-size-base);
    font-weight: 500;
    flex: 1;
    order: 0;
    color: var(--text-color-1);
  }

  .header-actions {
    width: auto;
    justify-content: flex-end;
    order: 0;
    gap: var(--spacing-sm);
  }

  .line-count {
    font-size: var(--font-size-2xs);
    padding: 2px 6px;
    white-space: nowrap;
    background: var(--bg-color-3);
    color: var(--text-color-2);
    border-radius: var(--radius-xs);
  }

  .close-btn {
    font-size: var(--font-size-3xl);
    line-height: 1;
    color: var(--text-color-2);
    transition: color var(--transition-fast);
    min-width: 32px;
    min-height: 32px;
    display: flex;
    align-items: center;
    justify-content: center;
  }

  .close-btn:hover {
    color: var(--text-color-1);
  }

  .action-btn {
    padding: 2px 7px;
    font-size: var(--font-size-2xs);
  }

  .action-icon {
    font-size: var(--font-size-2xs);
  }

  .action-dropdown {
    order: 0;
  }

  .dropdown-menu {
    right: 0;
    min-width: 110px;
  }

  .dropdown-option {
    padding: 5px 10px;
    font-size: var(--font-size-2xs);
  }

  .modal-search {
    padding: var(--spacing-xs) var(--spacing-md);
    flex-wrap: wrap;
    gap: var(--spacing-xs);
    background: var(--bg-color-2);
    border-bottom: 1px solid var(--border-color);
  }

  .segment-btn {
    padding: 2px 6px;
    font-size: var(--font-size-2xs);
  }

  .search-input {
    flex: 1;
    min-width: 120px;
    padding: var(--spacing-xs) var(--spacing-sm);
    font-size: var(--font-size-sm);
    background: var(--card-bg);
    border-color: var(--border-color);
    color: var(--text-color-1);
  }

  .search-input:focus {
    border-color: var(--primary-color);
  }

  .search-input::placeholder {
    color: var(--text-color-3);
  }

  .clear-btn {
    padding: 2px 6px;
    font-size: var(--font-size-sm);
    min-width: 24px;
    max-width: 24px;
    background: var(--bg-color-3);
    color: var(--text-color-2);
    border-radius: var(--radius-xs);
  }

  .clear-btn:hover {
    background: var(--primary-color);
    color: white;
  }

  .search-nav {
    order: 3;
    width: 100%;
    justify-content: center;
    gap: var(--spacing-xs);
  }

  .match-info {
    font-size: var(--font-size-xs);
    min-width: 50px;
    color: var(--text-color-2);
  }

  .nav-btn {
    padding: 2px var(--spacing-sm);
    font-size: var(--font-size-xs);
    min-width: 50px;
    background: var(--card-bg);
    border-color: var(--border-color);
    color: var(--text-color-1);
    border-radius: var(--radius-xs);
  }

  .nav-btn:hover:not(:disabled) {
    background: var(--primary-color);
    color: white;
    border-color: var(--primary-color);
  }

  .match-count {
    order: 3;
    width: 100%;
    text-align: center;
    font-size: var(--font-size-xs);
    color: var(--text-color-2);
  }

  .jump-nav {
    gap: var(--spacing-xs);
  }

  .jump-btn {
    padding: 2px var(--spacing-sm);
    font-size: var(--font-size-xs);
    background: var(--card-bg);
    border-color: var(--border-color);
    color: var(--text-color-1);
    border-radius: var(--radius-xs);
  }

  .jump-btn:hover {
    background: var(--primary-color);
    color: white;
    border-color: var(--primary-color);
  }

  .modal-body {
    flex: 1;
    overflow: auto;
    min-height: 0;
  }

  .virtual-scroll-container {
    min-width: max-content;
  }

  .virtual-scroll-content {
    min-width: max-content;
  }

  .log-viewer {
    font-size: var(--font-size-sm);
    min-width: max-content;
  }

  .log-line {
    padding: 0 var(--spacing-sm);
    min-width: max-content;
  }

  .line-number {
    min-width: 40px;
    padding-right: var(--spacing-sm);
    margin-right: var(--spacing-sm);
    font-size: var(--font-size-xs);
  }

  .line-content {
    font-size: var(--font-size-sm);
    white-space: pre;
    overflow: visible;
    text-overflow: unset;
    flex: none;
  }
}

@media (max-width: 480px) {
  .modal-content.large {
    height: 100vh;
    max-height: 100vh;
    border-radius: 0;
  }

  .modal-header {
    padding: var(--spacing-xs) var(--spacing-sm);
  }

  .modal-header .title {
    font-size: var(--font-size-sm);
  }

  .line-count {
    font-size: var(--font-size-2xs);
    padding: 2px 4px;
  }

  .close-btn {
    font-size: var(--font-size-2xl);
  }

  .modal-search {
    padding: 4px var(--spacing-sm);
  }

  .search-input {
    min-width: 100px;
    font-size: var(--font-size-xs);
  }

  .nav-btn {
    padding: 2px 6px;
    font-size: var(--font-size-2xs);
    min-width: 40px;
  }

  .log-viewer {
    font-size: var(--font-size-xs);
  }

  .line-number {
    min-width: 35px;
    padding-right: 6px;
    margin-right: 6px;
    font-size: var(--font-size-2xs);
  }

  .line-content {
    font-size: var(--font-size-xs);
  }
}


</style>
