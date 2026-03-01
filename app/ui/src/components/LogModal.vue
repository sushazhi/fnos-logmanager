<template>
  <div class="modal active" @click.self="$emit('close')">
    <div class="modal-content large">
      <div class="modal-header">
        <span class="title">{{ title }}</span>
        <div class="header-actions">
          <span class="line-count">{{ totalLines }} 行</span>
          <button class="close-btn" @click="$emit('close')">×</button>
        </div>
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

<script setup>
import { ref, computed, watch, nextTick, onMounted, onUnmounted } from 'vue'

const props = defineProps({
  title: {
    type: String,
    default: ''
  },
  content: {
    type: String,
    default: ''
  }
})

defineEmits(['close'])

const searchQuery = ref('')
const currentMatchIndex = ref(0)
const logBody = ref(null)
const currentLineIndex = ref(-1)

const LINE_HEIGHT = 24
const BUFFER_SIZE = 10
const visibleRange = ref({ start: 0, end: 50 })

const allLines = computed(() => {
  const content = props.content || ''
  if (!content || content === '(空文件)') return []
  return content.split('\n')
})

const totalLines = computed(() => allLines.value.length)

const totalHeight = computed(() => Math.max(totalLines.value * LINE_HEIGHT, 100))

const offsetY = computed(() => visibleRange.value.start * LINE_HEIGHT)

const visibleLines = computed(() => {
  const { start, end } = visibleRange.value
  const lines = []
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

const matchPositions = computed(() => {
  if (!searchQuery.value.trim()) return []
  const query = searchQuery.value.toLowerCase()
  const positions = []
  allLines.value.forEach((line, lineIndex) => {
    let searchPos = 0
    let pos = line.toLowerCase().indexOf(query, searchPos)
    while (pos !== -1) {
      positions.push({ lineIndex, charIndex: pos })
      searchPos = pos + 1
      pos = line.toLowerCase().indexOf(query, searchPos)
    }
  })
  return positions
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

function handleScroll() {
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

function clearSearch() {
  searchQuery.value = ''
  currentLineIndex.value = -1
}

function lineHasMatch(index) {
  return matchPositions.value.some(p => p.lineIndex === index)
}

function formatLine(line, lineIndex) {
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
      return html
    }
  }
  
  return html.replace(regex, '<mark class="highlight">$1</mark>')
}

function escapeHtml(text) {
  if (!text) return ''
  return text
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
}

function escapeRegex(string) {
  return string.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
}

function scrollToLine(lineIndex) {
  if (!logBody.value) return
  
  const targetTop = lineIndex * LINE_HEIGHT
  logBody.value.scrollTo({
    top: targetTop - logBody.value.clientHeight / 2 + LINE_HEIGHT,
    behavior: 'smooth'
  })
}

function nextMatch() {
  if (currentMatchIndex.value < matchPositions.value.length - 1) {
    currentMatchIndex.value++
    currentLineIndex.value = matchPositions.value[currentMatchIndex.value].lineIndex
    nextTick(() => scrollToLine(currentLineIndex.value))
  }
}

function prevMatch() {
  if (currentMatchIndex.value > 0) {
    currentMatchIndex.value--
    currentLineIndex.value = matchPositions.value[currentMatchIndex.value].lineIndex
    nextTick(() => scrollToLine(currentLineIndex.value))
  }
}

onMounted(() => {
  handleScroll()
})

onUnmounted(() => {
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
  max-width: 1000px;
  width: 95%;
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

.modal-body {
  flex: 1;
  overflow: auto;
  padding: 0;
  min-height: 300px;
}

.virtual-scroll-container {
  position: relative;
  width: 100%;
}

.virtual-scroll-content {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
}

.log-viewer {
  margin: 0;
  background: #1e1e1e;
  min-height: 100%;
  font-family: 'Monaco', 'Menlo', 'Consolas', monospace;
  font-size: 12px;
  line-height: 24px;
}

.log-line {
  display: flex;
  padding: 0 15px;
  height: 24px;
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
}

.line-content {
  color: #d4d4d4;
  white-space: pre;
  overflow: hidden;
  text-overflow: ellipsis;
  flex: 1;
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
    max-height: 90vh;
    border-radius: 16px 16px 0 0;
  }
  
  .modal-header {
    padding: 12px 15px;
    flex-wrap: wrap;
    gap: 8px;
  }
  
  .modal-header .title {
    font-size: 13px;
    width: 100%;
    order: 1;
  }
  
  .header-actions {
    width: 100%;
    justify-content: space-between;
    order: 2;
  }
  
  .line-count {
    font-size: 11px;
    padding: 3px 8px;
  }
  
  .modal-search {
    padding: 10px 15px;
    flex-wrap: wrap;
    gap: 8px;
  }
  
  .search-input {
    flex: 1;
    min-width: 120px;
    padding: 8px 10px;
    font-size: 12px;
  }
  
  .clear-btn {
    padding: 6px 8px;
    font-size: 14px;
  }
  
  .search-nav {
    order: 3;
    width: 100%;
    justify-content: center;
  }
  
  .match-count {
    order: 3;
    width: 100%;
    text-align: center;
  }
  
  .log-viewer {
    font-size: 12px;
  }
  
  .log-line {
    padding: 0 10px;
  }
  
  .line-number {
    min-width: 40px;
    padding-right: 8px;
    margin-right: 8px;
    font-size: 11px;
  }
  
  .line-content {
    font-size: 12px;
  }
}
</style>
