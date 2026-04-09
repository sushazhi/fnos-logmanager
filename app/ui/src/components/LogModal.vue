<!-- eslint-disable vue/no-v-html -->
<template>
  <div class="modal active" @click.self="$emit('close')">
    <div class="modal-content large">
      <div class="modal-header">
        <span class="title">{{ title }}</span>
        <div class="header-actions">
          <span class="line-count">{{ totalLines }} 行{{ truncated ? ` / 共 ${totalLinesInFile} 行` : '' }}</span>
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
        <input 
          type="text" 
          v-model="searchQuery" 
          placeholder="搜索日志内容..."
          class="search-input"
        >
        <button class="clear-btn" v-if="searchQuery" @click="clearSearch" title="清除搜索">×</button>
        <div class="search-nav" v-if="searchQuery && matchPositions.length > 0">
          <span class="match-info">{{ currentMatchIndex + 1 }} / {{ matchPositions.length }}</span>
          <button class="nav-btn" @click="prevMatch" :disabled="currentMatchIndex <= 0" title="上一个匹配">↑</button>
          <button class="nav-btn" @click="nextMatch" :disabled="currentMatchIndex >= matchPositions.length - 1" title="下一个匹配">↓</button>
        </div>
        <span class="match-count" v-if="searchQuery && matchPositions.length === 0">
          无匹配
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
import { ref, computed, watch, nextTick, onMounted, onUnmounted } from 'vue'
import DOMPurify from 'dompurify'
import { useLogSearch } from '../composables/useLogSearch'

interface Props {
  title?: string
  content?: string
  truncated?: boolean
  totalLinesInFile?: number
  loadingAll?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  title: '',
  content: '',
  truncated: false,
  totalLinesInFile: 0,
  loadingAll: false
})

defineEmits<{
  close: []
  loadAll: []
}>()

const searchQuery = ref('')
const currentMatchIndex = ref(0)
const logBody = ref<HTMLElement | null>(null)
const currentLineIndex = ref(-1)

const LINE_HEIGHT = 24
const BUFFER_SIZE = 10

interface VisibleRange {
  start: number
  end: number
}

const visibleRange = ref<VisibleRange>({ start: 0, end: 50 })

const allLines = computed(() => {
  const content = props.content || ''
  if (!content || content === '(空文件)') return []
  return content.split('\n')
})

// 使用 Web Worker 搜索 composable
const { matchPositions, isSearching, cleanup: cleanupSearch } = useLogSearch(allLines, searchQuery)

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
    if (allLines.value[i] !== undefined) {
      lines.push({
        index: i,
        line: allLines.value[i]
      })
    }
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
  if (!searchQuery.value || !line) return escapeHtml(line)
  const query = searchQuery.value
  const regex = new RegExp(`(${escapeRegex(query)})`, 'gi')
  let html = escapeHtml(line)
  
  if (lineIndex === currentLineIndex.value) {
    const currentPos = matchPositions.value[currentMatchIndex.value]
    if (currentPos && currentPos.lineIndex === lineIndex) {
      const plainText = line
      const before = escapeHtml(plainText.substring(0, currentPos.charIndex))
      const match = escapeHtml(plainText.substring(currentPos.charIndex, currentPos.charIndex + query.length))
      const after = escapeHtml(plainText.substring(currentPos.charIndex + query.length))
      // 对前后部分都应用高亮，当前匹配用 current 类
      html = before.replace(regex, '<mark class="highlight">$1</mark>') + 
             '<mark class="highlight current">' + match + '</mark>' + 
             after.replace(regex, '<mark class="highlight">$1</mark>')
      // 使用 DOMPurify 清理 HTML，只允许 mark 标签
      return DOMPurify.sanitize(html, {
        ALLOWED_TAGS: ['mark'],
        ALLOWED_ATTR: ['class']
      })
    }
  }
  
  const highlightedHtml = html.replace(regex, '<mark class="highlight">$1</mark>')
  // 使用 DOMPurify 清理 HTML，只允许 mark 标签
  return DOMPurify.sanitize(highlightedHtml, {
    ALLOWED_TAGS: ['mark'],
    ALLOWED_ATTR: ['class']
  })
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

onMounted(() => {
  handleScroll()
})

onUnmounted(() => {
  cleanupSearch()
})
</script>

<style scoped>
.modal {
  position: fixed;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  background: rgba(0, 0, 0, 0.5);
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
  background: var(--card-bg, white);
  border-radius: 12px;
  overflow: hidden;
  display: flex;
  flex-direction: column;
}

.modal-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 15px 20px;
  background: var(--bg-color, #f8f9fa);
  border-bottom: 1px solid var(--border-color, #e0e0e0);
}

.modal-header .title {
  font-weight: 600;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  flex: 1;
  color: var(--text-color, #333);
}

.header-actions {
  display: flex;
  align-items: center;
  gap: 15px;
}

.line-count {
  font-size: 12px;
  color: var(--text-secondary, #666);
  background: var(--border-color, #e8e8e8);
  padding: 4px 10px;
  border-radius: 12px;
}

.close-btn {
  background: none;
  border: none;
  font-size: 24px;
  cursor: pointer;
  color: var(--text-secondary, #666);
  padding: 0;
  line-height: 1;
}

.close-btn:hover {
  color: var(--text-color, #333);
}

.truncated-warning {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 8px 20px;
  background: #fff3cd;
  border-bottom: 1px solid #ffc107;
  color: #856404;
  font-size: 13px;
}

.warning-icon {
  font-size: 16px;
}

.load-all-btn {
  margin-left: auto;
  padding: 4px 12px;
  background: #ffc107;
  border: none;
  border-radius: 4px;
  color: #212529;
  font-size: 12px;
  cursor: pointer;
  font-weight: 500;
}

.load-all-btn:hover:not(:disabled) {
  background: #e0a800;
}

.load-all-btn:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.modal-search {
  padding: 12px 20px;
  background: var(--bg-color, #f5f7fa);
  border-bottom: 1px solid var(--border-color, #e0e0e0);
  display: flex;
  align-items: center;
  gap: 10px;
}

.search-input {
  flex: 1;
  padding: 8px 12px;
  border: 1px solid var(--border-color, #ddd);
  border-radius: 6px;
  font-size: 13px;
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
  font-size: 14px;
  cursor: pointer;
  padding: 4px 8px;
  border-radius: 4px;
  line-height: 1;
  min-width: 28px;
  max-width: 28px;
}

.clear-btn:hover {
  background: var(--primary-color, #667eea);
  color: white;
}

.search-nav {
  display: flex;
  align-items: center;
  gap: 8px;
}

.match-info {
  font-size: 12px;
  color: var(--text-secondary, #666);
  min-width: 70px;
}

.nav-btn {
  padding: 4px 12px;
  border: 1px solid var(--border-color, #ddd);
  border-radius: 4px;
  background: var(--card-bg, white);
  cursor: pointer;
  font-size: 12px;
  color: var(--text-color, #333);
  min-width: 65px;
}

.nav-btn:hover:not(:disabled) {
  background: var(--primary-color, #667eea);
  color: white;
  border-color: var(--primary-color, #667eea);
}

.nav-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.match-count {
  font-size: 12px;
  color: var(--text-secondary, #666);
  white-space: nowrap;
}

.jump-nav {
  display: flex;
  align-items: center;
  gap: 6px;
}

.jump-btn {
  padding: 4px 10px;
  border: 1px solid var(--border-color, #ddd);
  border-radius: 4px;
  background: var(--card-bg, white);
  cursor: pointer;
  font-size: 12px;
  color: var(--text-color, #333);
}

.jump-btn:hover {
  background: var(--primary-color, #667eea);
  color: white;
  border-color: var(--primary-color, #667eea);
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
  background: #1e1e1e;
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
  background: #2a2a2a;
}

.log-line.has-match {
  background: #2a2a1a;
}

.log-line.current-match {
  background: #3a3a2a;
}

.line-number {
  color: #858585;
  min-width: 50px;
  text-align: right;
  padding-right: 15px;
  user-select: none;
  border-right: 1px solid #333;
  margin-right: 15px;
  flex-shrink: 0;
}

.line-content {
  color: #d4d4d4;
  white-space: pre;
  flex: none;
}

:deep(.highlight) {
  background: #ffeb3b;
  color: #000;
  padding: 0 2px;
  border-radius: 2px;
}

:deep(.highlight.current) {
  background: #ff9800;
  color: #000;
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

  .modal-search {
    padding: var(--spacing-xs) var(--spacing-md);
    flex-wrap: wrap;
    gap: var(--spacing-xs);
    background: var(--bg-color-2);
    border-bottom: 1px solid var(--border-color);
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
    color: var(--text-color-3);
    border-right-color: var(--bg-color-3);
  }

  .line-content {
    font-size: 0.75rem;
    white-space: pre;
    overflow: visible;
    text-overflow: unset;
    flex: none;
    color: var(--text-color-2);
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
