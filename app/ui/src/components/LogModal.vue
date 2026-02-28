<template>
  <div class="modal active" @click.self="$emit('close')">
    <div class="modal-content large">
      <div class="modal-header">
        <span class="title">{{ title }}</span>
        <div class="header-actions">
          <span class="line-count">{{ lineCount }} 行</span>
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
        <div class="search-nav" v-if="searchQuery && matchCount > 0">
          <span class="match-info">{{ currentMatch }} / {{ matchCount }}</span>
          <button class="nav-btn" @click="prevMatch" :disabled="currentMatch <= 1">↑</button>
          <button class="nav-btn" @click="nextMatch" :disabled="currentMatch >= matchCount">↓</button>
        </div>
        <span class="match-count" v-if="searchQuery && matchCount === 0">
          无匹配
        </span>
      </div>
      <div class="modal-body" ref="logBody">
        <pre class="log-viewer"><div 
          v-for="(line, index) in lines" 
          :key="index" 
          class="log-line"
          :class="{ 'has-match': lineHasMatch(index) }"
          :ref="el => { if (el) lineRefs[index] = el }"
        ><span class="line-number">{{ index + 1 }}</span><span class="line-content" v-html="formatLine(line)"></span></div></pre>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, watch, nextTick } from 'vue'

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
const currentMatch = ref(0)
const matchCount = ref(0)
const logBody = ref(null)
const lineRefs = ref({})

const lines = computed(() => {
  return (props.content || '').split('\n')
})

const lineCount = computed(() => lines.value.length)

const matchLines = computed(() => {
  if (!searchQuery.value.trim()) return new Set()
  const query = searchQuery.value.toLowerCase()
  const matches = new Set()
  lines.value.forEach((line, index) => {
    if (line.toLowerCase().includes(query)) {
      matches.add(index)
    }
  })
  return matches
})

watch(searchQuery, () => {
  matchCount.value = 0
  currentMatch.value = 0
  
  if (searchQuery.value.trim()) {
    const query = searchQuery.value.toLowerCase()
    lines.value.forEach(line => {
      const matches = line.toLowerCase().match(new RegExp(escapeRegex(query), 'g'))
      if (matches) {
        matchCount.value += matches.length
      }
    })
    if (matchCount.value > 0) {
      currentMatch.value = 1
      nextTick(() => scrollToFirstMatch())
    }
  }
})

function clearSearch() {
  searchQuery.value = ''
}

function lineHasMatch(index) {
  return matchLines.value.has(index)
}

function formatLine(line) {
  if (!searchQuery.value || !line) return escapeHtml(line)
  const query = searchQuery.value
  const regex = new RegExp(`(${escapeRegex(query)})`, 'gi')
  return escapeHtml(line).replace(regex, '<mark class="highlight">$1</mark>')
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

function scrollToFirstMatch() {
  const firstMatchLine = Array.from(matchLines.value)[0]
  if (firstMatchLine !== undefined && lineRefs.value[firstMatchLine]) {
    lineRefs.value[firstMatchLine].scrollIntoView({ behavior: 'smooth', block: 'center' })
  }
}

function nextMatch() {
  if (currentMatch.value < matchCount.value) {
    currentMatch.value++
  }
}

function prevMatch() {
  if (currentMatch.value > 1) {
    currentMatch.value--
  }
}
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
  font-size: 16px;
  cursor: pointer;
  padding: 6px 10px;
  border-radius: 6px;
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
  min-width: 50px;
}

.nav-btn {
  padding: 4px 8px;
  border: 1px solid var(--border-color, #ddd);
  border-radius: 4px;
  background: var(--card-bg, white);
  cursor: pointer;
  font-size: 12px;
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
}

.log-viewer {
  margin: 0;
  background: #1e1e1e;
  min-height: 300px;
  font-family: 'Monaco', 'Menlo', 'Consolas', monospace;
  font-size: 12px;
  line-height: 1.6;
}

.log-line {
  display: flex;
  padding: 0 15px;
}

.log-line:hover {
  background: #2a2a2a;
}

.log-line.has-match {
  background: #2a2a1a;
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
  white-space: pre-wrap;
  word-break: break-all;
  flex: 1;
}

.highlight {
  background: #ffeb3b;
  color: #000;
  padding: 0 2px;
  border-radius: 2px;
}

@media (max-width: 768px) {
  .modal-content.large {
    width: 100%;
    max-height: 100vh;
    border-radius: 0;
  }
  
  .modal-header {
    padding: 12px 15px;
  }
  
  .modal-header .title {
    font-size: 13px;
  }
  
  .log-viewer {
    font-size: 11px;
  }
  
  .line-number {
    min-width: 35px;
    padding-right: 10px;
    margin-right: 10px;
    font-size: 10px;
  }
}
</style>
