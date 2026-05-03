<!-- eslint-disable vue/no-v-html -->
<template>
  <div class="modal active" @click.self="$emit('close')">
    <div class="modal-content large">
      <div class="modal-header">
        <span class="title">{{ title }}</span>
        <div class="header-actions">
          <span class="line-count">{{ totalLines }} 行{{ truncated ? ` / 共 ${totalLinesInFile} 行` : '' }}</span>
          <span class="tail-error" v-if="tailError">{{ tailError }}</span>
          <button
            class="action-btn"
            :class="{ 'tail-active': isTailing }"
            @click="toggleTail"
            :title="isTailing ? '停止追踪' : '实时追踪'"
          >
            <span class="action-icon">{{ isTailing ? '■' : '◉' }}</span>
            <span class="action-text">{{ isTailing ? '停止' : '追踪' }}</span>
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
import api from '../services/api'

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

const emit = defineEmits<{
  close: []
  loadAll: []
  export: [format: string]
  addBookmark: []
}>()

const searchQuery = ref('')
const currentMatchIndex = ref(0)
const logBody = ref<HTMLElement | null>(null)
const currentLineIndex = ref(-1)
const showExportMenu = ref(false)

const searchOptions = reactive({
  mode: 'keyword' as 'keyword' | 'regex'
})

const isTailing = ref(false)
let tailTimer: ReturnType<typeof setInterval> | null = null
let tailOffset = -1
let autoScrollToBottom = false
const tailContent = ref('')
const tailError = ref('')

const LINE_HEIGHT = 24
const BUFFER_SIZE = 10

interface VisibleRange {
  start: number
  end: number
}

const visibleRange = ref<VisibleRange>({ start: 0, end: 50 })

const allLines = computed(() => {
  const content = props.content || ''
  const fullContent = isTailing.value && tailContent.value ? content + '\n' + tailContent.value : content
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
  if (searchOptions.mode === 'regex') {
    try {
      highlightRegex = new RegExp(query, 'gi')
    } catch {
      return escaped
    }
  } else {
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
  // 移除 ANSI 转义码（颜色代码等）
  const ansiRegex = /\x1b\[[0-9;]*m/g
  return text
    .replace(ansiRegex, '')
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
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

  tailContent.value = ''
  tailError.value = ''
  tailOffset = -1

  const filePathToSend = props.isDocker ? props.containerName : props.filePath
  if (!filePathToSend) {
    tailError.value = '无法追踪: 缺少文件路径或容器名'
    return
  }

  isTailing.value = true
  scrollToBottom()

  async function pollTail(): Promise<void> {
    if (!isTailing.value) return

    try {
      const token = api.getSessionToken()
      let url: string
      if (props.isDocker) {
        url = `/api/docker/tail?container=${encodeURIComponent(props.containerName || props.filePath)}&offset=${tailOffset}`
      } else {
        url = `/api/log/tail?path=${encodeURIComponent(props.filePath || props.containerName)}&offset=${tailOffset}`
      }

      const headers: Record<string, string> = {
        'X-Requested-With': 'XMLHttpRequest'
      }
      if (token) {
        headers['X-Session-Token'] = token
      }

      const response = await fetch(url, { credentials: 'include', headers })
      if (!response.ok) {
        if (response.status === 401) {
          tailError.value = '认证已过期，请重新登录'
          stopTail()
          return
        }
        tailError.value = `追踪请求失败: ${response.status}`
        return
      }

      const data = await response.json()
      if (data.deleted) {
        tailError.value = '日志文件已被删除'
        stopTail()
        return
      }

      tailOffset = data.offset

      if (data.content) {
        tailContent.value += data.content
        scrollToBottom()
      }
    } catch (err) {
      tailError.value = '追踪请求错误'
    }
  }

  pollTail()
  tailTimer = setInterval(pollTail, 1000)
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
  if (tailTimer) {
    clearInterval(tailTimer)
    tailTimer = null
  }
  isTailing.value = false
  autoScrollToBottom = false
  tailOffset = -1
}

onMounted(() => {
  handleScroll()
  document.addEventListener('click', handleClickOutside)
})

onUnmounted(() => {
  stopTail()
  cleanupSearch()
  document.removeEventListener('click', handleClickOutside)
})
</script>

<style scoped>
.modal {
  position: fixed;
  top: 0;
  left: 0;
  width: 100%;
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
  font-size: 0.75rem;
  color: var(--text-color-3);
  background: var(--bg-color-3);
  padding: var(--spacing-xs) var(--spacing-sm);
  border-radius: var(--radius-xs);
}

.close-btn {
  background: none;
  border: none;
  font-size: 1.5rem;
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
  font-size: 0.75rem;
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
  font-size: 0.8125rem;
  line-height: 1;
}

.action-text {
  font-size: 0.75rem;
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
  font-size: 0.6875rem;
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
  font-size: 0.75rem;
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
  font-size: 0.8125rem;
}

.warning-icon {
  font-size: 1rem;
}

.load-all-btn {
  margin-left: auto;
  padding: var(--spacing-xs) var(--spacing-md);
  background: var(--warning-color);
  border: none;
  border-radius: var(--radius-xs);
  color: white;
  font-size: 0.75rem;
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
  font-size: 0.8125rem;
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
  font-size: 0.6875rem;
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
  font-size: 0.75rem;
  color: var(--error-color);
  white-space: nowrap;
  font-weight: 500;
}

.clear-btn {
  background: var(--bg-color-3);
  border: none;
  color: var(--text-color-2);
  font-size: 0.875rem;
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
  font-size: 0.75rem;
  color: var(--text-color-3);
  min-width: 70px;
}

.nav-btn {
  padding: var(--spacing-xs) var(--spacing-sm);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-xs);
  background: var(--card-bg);
  cursor: pointer;
  font-size: 0.75rem;
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
  font-size: 0.75rem;
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
  font-size: 0.75rem;
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
  border-radius: 2px;
}

:deep(.highlight.current) {
  background: #0a59f7;
  color: white;
  padding: 0 2px;
  border-radius: 2px;
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
    font-size: 0.8125rem;
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
    font-size: 0.625rem;
    padding: 2px 6px;
    white-space: nowrap;
    background: var(--bg-color-3);
    color: var(--text-color-2);
    border-radius: var(--radius-xs);
  }

  .close-btn {
    font-size: 1.25rem;
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
    font-size: 11px;
  }

  .action-icon {
    font-size: 11px;
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
    font-size: 11px;
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
    font-size: 0.5625rem;
  }

  .search-input {
    flex: 1;
    min-width: 120px;
    padding: var(--spacing-xs) var(--spacing-sm);
    font-size: 0.75rem;
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
    font-size: 0.75rem;
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
    font-size: 0.6875rem;
    min-width: 50px;
    color: var(--text-color-2);
  }

  .nav-btn {
    padding: 2px var(--spacing-sm);
    font-size: 0.6875rem;
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
    font-size: 0.6875rem;
    color: var(--text-color-2);
  }

  .jump-nav {
    gap: var(--spacing-xs);
  }

  .jump-btn {
    padding: 2px var(--spacing-sm);
    font-size: 0.6875rem;
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
    font-size: 0.75rem;
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
    font-size: 0.6875rem;
  }

  .line-content {
    font-size: 0.75rem;
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
    font-size: 0.75rem;
  }

  .line-count {
    font-size: 0.625rem;
    padding: 2px 4px;
  }

  .close-btn {
    font-size: 1.125rem;
  }

  .modal-search {
    padding: 4px var(--spacing-sm);
  }

  .search-input {
    min-width: 100px;
    font-size: 0.6875rem;
  }

  .nav-btn {
    padding: 2px 6px;
    font-size: 0.625rem;
    min-width: 40px;
  }

  .log-viewer {
    font-size: 0.6875rem;
  }

  .line-number {
    min-width: 35px;
    padding-right: 6px;
    margin-right: 6px;
    font-size: 0.625rem;
  }

  .line-content {
    font-size: 0.6875rem;
  }
}
</style>
