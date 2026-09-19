<template>
  <Teleport to="body">
    <div class="drawer-overlay" v-if="logs.length > 0" @click.self="$emit('close')">
      <div class="drawer">
        <div class="drawer-header">
          <h3>{{ title }}</h3>
          <button class="close-btn" @click="$emit('close')">×</button>
        </div>
        <div class="drawer-search">
          <input 
            type="text" 
            v-model="searchQuery" 
            placeholder="搜索文件名..."
            class="search-input"
          >
          <button class="clear-btn" v-if="searchQuery" @click="searchQuery = ''" title="清除搜索">×</button>
          <span class="search-count" v-if="searchQuery">
            {{ filteredLogs.length }} / {{ logs.length }}
          </span>
          <template v-if="hasAppNames">
            <button
              class="collapse-all-btn"
              type="button"
              :disabled="isSearching || allCollapsed"
              :title="isSearching ? '搜索时自动展开全部' : '折叠所有分组'"
              @click="collapseAll"
            >全部折叠</button>
            <button
              class="collapse-all-btn"
              type="button"
              :disabled="isSearching || noneCollapsed"
              :title="isSearching ? '搜索时已自动展开' : '展开所有分组'"
              @click="expandAll"
            >全部展开</button>
          </template>
        </div>
        <div class="drawer-body">
          <div class="log-list">
            <div class="log-item header">
              <span class="path">{{ headerLabels.path }}</span>
              <span class="size">{{ headerLabels.size }}</span>
              <span class="action-col">操作</span>
            </div>
            <div
              v-for="group in groupedLogs"
              :key="group.key"
              class="log-group"
            >
            <button
              v-if="group.grouped"
              class="group-header"
              type="button"
              :aria-expanded="String(!isCollapsed(group.key))"
              @click="toggleGroup(group.key)"
            >
              <span class="group-caret" :class="{ collapsed: isCollapsed(group.key) }">▾</span>
              <span class="group-name" :title="group.key">{{ group.label }}</span>
              <span class="group-meta">
                {{ group.logs.length }} 个文件 · {{ formatGroupSize(group.totalSize) }}
              </span>
            </button>
            <template v-if="!group.grouped || !isCollapsed(group.key)">
            <div
              v-for="(log, index) in group.logs"
              :key="log.path || `${group.key}-${index}`"
              class="log-item"
            >
              <span class="path" :title="log.path">
                <template v-if="searchQuery">
                  <!-- eslint-disable-next-line vue/no-v-html -->
                  <span v-html="highlightText(displayPath(log), searchQuery)"></span>
                </template>
                <template v-else>
                  <span class="display-path">{{ displayPath(log) }}</span>
                  <span v-if="log.displayPath && log.displayPath !== log.path" class="original-path">{{ log.path }}</span>
                </template>
              </span>
              <span class="size">{{ log.sizeFormatted }}</span>
              <div class="actions">
                <button 
                  class="secondary small" 
                  @click="$emit('view', log.path)"
                  v-if="!log.isDocker && type !== 'archives' && !log.isArchive"
                >
                  查看
                </button>
                <button 
                  class="secondary small" 
                  @click="$emit('viewDocker', log.path)"
                  v-if="log.isDocker"
                >
                  查看日志
                </button>
                <button 
                  class="secondary small" 
                  @click="$emit('viewArchive', log.path)"
                  v-if="type === 'archives' || log.isArchive"
                >
                  查看
                </button>
                <button 
                  class="danger small" 
                  @click="$emit('truncate', log.path)"
                  v-if="!log.isDocker && type !== 'archives' && !log.isArchive"
                >
                  清空
                </button>
                <button 
                  class="danger small" 
                  @click="$emit('delete', log.path)"
                  v-if="type === 'archives' || log.isArchive"
                >
                  删除
                </button>
                <button 
                  class="danger small" 
                  @click="handleDeleteLog(log.path)"
                  v-if="!log.isDocker && type !== 'archives' && !log.isArchive && log.canDelete"
                  title="应用已卸载，可删除日志文件"
                >
                  删除
                </button>
              </div>
            </div>
            </template>
            </div>
            <div v-if="filteredLogs.length === 0 && logs.length > 0" class="no-results">
              未找到匹配的结果
            </div>
          </div>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<script setup>
import { ref, computed } from 'vue'
import DOMPurify from 'dompurify'

const props = defineProps({
  logs: {
    type: Array,
    default: () => []
  },
  type: {
    type: String,
    default: 'logs'
  }
})

const emit = defineEmits(['view', 'truncate', 'viewDocker', 'viewArchive', 'delete', 'delete-log', 'close'])

function handleDeleteLog(path) {
  emit('delete-log', path)
}

const searchQuery = ref('')

const title = computed(() => {
  switch (props.type) {
    case 'docker': return 'Docker容器日志'
    case 'archives': return '归档日志'
    default: return '结果列表'
  }
})

const headerLabels = computed(() => {
  switch (props.type) {
    case 'docker': 
      return { path: '容器名称', size: '镜像' }
    case 'archives': 
      return { path: '文件路径', size: '大小' }
    default: 
      return { path: '文件路径', size: '大小' }
  }
})

// P2: Use displayPath if available, fallback to path
function displayPath(log) {
  return log.displayPath || log.path || ''
}

const filteredLogs = computed(() => {
  if (!searchQuery.value.trim()) {
    return props.logs
  }
  const query = searchQuery.value.toLowerCase()
  return props.logs.filter(log => {
    const path = (log.path || '').toLowerCase()
    const displayP = (log.displayPath || '').toLowerCase()
    const size = (log.sizeFormatted || '').toLowerCase()
    return path.includes(query) || displayP.includes(query) || size.includes(query)
  })
})

// 分组视图：按应用归并日志文件。
//
// 后端 ListLogFiles 已为每条日志算好 appName（extractAppNameFromPath，
// 即 /vol\d+/@appdata/<app>/... 或 /vol\d+/@appshare/<app>/... 中的 <app>），
// 但扁平列表从未使用它。这里按它分组，让「某个应用的日志散在十几条路径里」
// 变得可聚焦、可整组收起。
//
// 注意：并非所有调用方都提供 appName —— ListLargeLogFiles（大文件）不填该字段，
// ListArchiveFiles（归档）用的是另一套 ArchiveFile 结构，Docker 列表同理。
// 因此这里按数据自适应：只有当结果里确实存在 appName 时才分组，否则保持原来的
// 扁平渲染，避免给一个孤零零的「未归类」组头。
const UNGROUPED_KEY = '__ungrouped__'

// 分组折叠状态。
//
// 分两层：
//   defaultCollapsed —— 「全部折叠/全部展开」设定的全局默认，持久化到 localStorage，
//                       刷新后保持；新出现的应用组也跟随它，不会因为记录里没有
//                       这个 key 就突然展开。
//   collapsedGroups  —— 在默认之外被单独切换过的组（即与默认相反的例外）。
//
// 最终某组是否收起 = collapsedGroups.has(key) 与 defaultCollapsed 的异或。
const COLLAPSE_STORAGE_KEY = 'logmanager:logListDefaultCollapsed'
const EXCEPTIONS_STORAGE_KEY = 'logmanager:logListCollapseExceptions'

function readDefaultCollapsed() {
  try {
    return localStorage.getItem(COLLAPSE_STORAGE_KEY) === '1'
  } catch {
    // localStorage 可能被禁用（隐私模式/策略），此时退化为不持久化。
    return false
  }
}

function writeDefaultCollapsed(value) {
  try {
    localStorage.setItem(COLLAPSE_STORAGE_KEY, value ? '1' : '0')
  } catch {
    // 忽略：持久化失败不应影响本次交互
  }
}

// 例外集合同样持久化：否则「单独收起某个吵闹的应用」在刷新后就会丢失，
// 而这与「全部折叠」是同一个痛点。
//
// 注意：本组件是 <script setup> 而非 <script setup lang="ts">，不能使用类型标注。
function readExceptions() {
  try {
    const raw = localStorage.getItem(EXCEPTIONS_STORAGE_KEY)
    if (!raw) return new Set()
    const parsed = JSON.parse(raw)
    if (!Array.isArray(parsed)) return new Set()
    return new Set(parsed.filter(x => typeof x === 'string'))
  } catch {
    return new Set()
  }
}

function writeExceptions(keys) {
  try {
    localStorage.setItem(EXCEPTIONS_STORAGE_KEY, JSON.stringify([...keys]))
  } catch {
    // 忽略
  }
}

const defaultCollapsed = ref(readDefaultCollapsed())
const collapsedGroups = ref(readExceptions())

function groupKeyOf(log) {
  const name = (log.appName || '').trim()
  return name === '' ? UNGROUPED_KEY : name
}

// 是否存在可分组的数据：至少一条日志带非空 appName。
const hasAppNames = computed(() =>
  filteredLogs.value.some(log => (log.appName || '').trim() !== '')
)

const groupedLogs = computed(() => {
  const rows = filteredLogs.value
  if (rows.length === 0) return []

  // 无可用的 appName 数据（大文件/归档/Docker）→ 单个匿名组，模板仍走同一条
  // 渲染路径但不渲染组头。
  if (props.type === 'docker' || !hasAppNames.value) {
    return [{ key: '__all__', label: '', logs: rows, totalSize: 0, grouped: false }]
  }

  const buckets = new Map()
  for (const log of rows) {
    const key = groupKeyOf(log)
    let bucket = buckets.get(key)
    if (!bucket) {
      bucket = { key, logs: [], totalSize: 0 }
      buckets.set(key, bucket)
    }
    bucket.logs.push(log)
    bucket.totalSize += Number(log.size) || 0
  }

  const groups = [...buckets.values()].map(b => ({
    ...b,
    grouped: true,
    label: b.key === UNGROUPED_KEY ? '未归类' : b.key
  }))

  // 组内保持 filteredLogs 的原有顺序（后端已按 ModTime 排过），
  // 组间按文件数降序，让日志最多的应用排在最前面。
  groups.sort((a, b) => b.logs.length - a.logs.length || a.label.localeCompare(b.label))
  return groups
})

// 搜索时无视手动收起状态：命中的结果必须可见，否则用户会以为没搜到。
const isSearching = computed(() => searchQuery.value.trim().length > 0)

// 某组此刻是否收起 = 默认态与「单独切换过的例外」的异或。
function isCollapsed(key) {
  if (isSearching.value) return false
  const exception = collapsedGroups.value.has(key)
  return defaultCollapsed.value ? !exception : exception
}

function toggleGroup(key) {
  const next = new Set(collapsedGroups.value)
  if (next.has(key)) {
    next.delete(key)
  } else {
    next.add(key)
  }
  collapsedGroups.value = next
  writeExceptions(next)
}

// 全部折叠/展开只作用于当前实际渲染的组（groupedLogs），而不是历史上点过的所有
// key —— 否则搜索换了一批结果后，按钮状态会和眼前看到的列表对不上。
const groupKeys = computed(() =>
  groupedLogs.value.filter(g => g.grouped).map(g => g.key)
)

const allCollapsed = computed(() =>
  groupKeys.value.length > 0 && groupKeys.value.every(k => isCollapsed(k))
)

const noneCollapsed = computed(() =>
  groupKeys.value.every(k => !isCollapsed(k))
)

// 全部折叠/展开切换的是「默认态」而非具体 key 集合：这样新出现的应用组也会跟随
// 该状态，且状态能持久化到下次打开。
function collapseAll() {
  defaultCollapsed.value = true
  collapsedGroups.value = new Set()
  writeDefaultCollapsed(true)
  writeExceptions(new Set())
}

function expandAll() {
  defaultCollapsed.value = false
  collapsedGroups.value = new Set()
  writeDefaultCollapsed(false)
  writeExceptions(new Set())
}

function formatGroupSize(bytes) {
  const n = Number(bytes) || 0
  if (n < 1024) return `${n} B`
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`
  if (n < 1024 * 1024 * 1024) return `${(n / (1024 * 1024)).toFixed(1)} MB`
  return `${(n / (1024 * 1024 * 1024)).toFixed(2)} GB`
}

function highlightText(text, query) {
  if (!query || !text) return text
  if (query.length > 200) return text
  // 先对文本进行HTML转义
  const escapedText = text
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#039;')
  
  const regex = new RegExp(`(${escapeRegex(query)})`, 'gi')
  const html = escapedText.replace(regex, '<mark class="highlight">$1</mark>')
  
  // 使用DOMPurify清理HTML，只允许mark标签和class属性
  return DOMPurify.sanitize(html, {
    ALLOWED_TAGS: ['mark'],
    ALLOWED_ATTR: ['class']
  })
}

function escapeRegex(string) {
  return string.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
}
</script>

<style scoped>
.drawer-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: var(--overlay);
  backdrop-filter: blur(12px);
  -webkit-backdrop-filter: blur(12px);
  z-index: 1000;
  display: flex;
  justify-content: flex-end;
}

.drawer {
  width: 900px;
  max-width: 95%;
  height: 100%;
  background: var(--glass-bg-strong);
  backdrop-filter: blur(var(--glass-blur-heavy));
  -webkit-backdrop-filter: blur(var(--glass-blur-heavy));
  box-shadow: var(--depth-5), var(--glass-shadow);
  display: flex;
  flex-direction: column;
  animation: hm-slide-right 0.5s var(--ease-spring);
  will-change: transform, opacity;
  border-left: 1px solid var(--glass-border-strong);
}

@keyframes slideIn {
  from { transform: translateX(100%); opacity: 0; }
  to { transform: translateX(0); opacity: 1; }
}

.drawer-header {
  position: relative;
  overflow: hidden;
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: var(--spacing-md) var(--spacing-xl);
  background: var(--primary-gradient);
  color: var(--text-color-on-primary);
}

.drawer-header::after {
  content: '';
  position: absolute;
  inset: 0;
  background: var(--glass-shine);
  pointer-events: none;
}

.drawer-header h3 {
  margin: 0;
  font-size: var(--font-size-xl);
  font-weight: var(--font-weight-semibold);
  letter-spacing: 0.03em;
  white-space: nowrap;
}

.close-btn {
  background: none;
  border: none;
  color: var(--text-color-on-primary);
  font-size: var(--font-size-5xl);
  cursor: pointer;
  padding: 0;
  line-height: 1;
  transition: opacity var(--transition-fast), transform var(--transition-slow) var(--ease-spring);
}

.close-btn:hover {
  opacity: 0.85;
  transform: scale(1.15);
}

.close-btn:active {
  transform: scale(0.9);
}

.drawer-search {
  padding: var(--spacing-md) var(--spacing-xl);
  background: var(--glass-bg);
  backdrop-filter: blur(var(--glass-blur));
  -webkit-backdrop-filter: blur(var(--glass-blur));
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
  font-size: var(--font-size-md);
  font-family: var(--font-family);
  background: var(--glass-bg);
  backdrop-filter: blur(6px);
  -webkit-backdrop-filter: blur(6px);
  color: var(--text-color-1);
  transition: border-color var(--transition-fast), box-shadow var(--transition-fast);
}

.search-input:focus {
  outline: none;
  border-color: var(--primary-color);
  box-shadow: var(--focus-ring);
}

.search-input::placeholder {
  color: var(--text-color-3);
}

.clear-btn {
  background: var(--bg-color-3);
  border: none;
  color: var(--text-color-2);
  font-size: var(--font-size-xl);
  cursor: pointer;
  padding: var(--spacing-xs) var(--spacing-sm);
  border-radius: var(--radius-xs);
  transition: all var(--transition-fast);
}

.clear-btn:hover {
  background: var(--primary-color);
  color: var(--text-color-on-primary);
  box-shadow: 0 0 12px var(--glow-primary);
  transform: scale(1.05);
}

.clear-btn:active {
  transform: scale(0.95);
}

.search-count {
  font-size: var(--font-size-sm);
  color: var(--text-color-2);
  white-space: nowrap;
}

.drawer-body {
  flex: 1;
  overflow: auto;
  padding: 0;
}

.log-list {
  border: none;
}

.log-item {
  position: relative;
  padding: var(--spacing-md) var(--spacing-xl);
  border-bottom: 1px solid var(--divider-color);
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-size: var(--font-size-base);
  transition: background var(--transition-fast);
  gap: var(--spacing-md);
}

.log-item::before {
  content: '';
  position: absolute;
  left: 0;
  top: 50%;
  transform: translateY(-50%);
  width: 3px;
  height: 0;
  background: var(--primary-color);
  border-radius: 0 3px 3px 0;
  transition: height var(--transition-slow) var(--ease-spring), opacity var(--transition-fast);
  opacity: 0;
  box-shadow: 0 0 8px var(--glow-primary);
}

.log-item:hover {
  background-color: var(--bg-color-2);
}

.log-item:hover::before {
  height: 60%;
  opacity: 1;
}

.log-item.header {
  background: var(--glass-bg-strong);
  backdrop-filter: blur(var(--glass-blur));
  -webkit-backdrop-filter: blur(var(--glass-blur));
  font-weight: 600;
  border-bottom: 2px solid var(--border-color);
  position: sticky;
  top: 0;
  z-index: 1;
  color: var(--text-color-1);
}

.log-item.header .path {
  white-space: nowrap;
}

.log-item .path {
  font-family: 'Monaco', 'Menlo', 'Consolas', monospace;
  color: var(--text-color-1);
  flex: 1;
  min-width: 0;
  word-break: break-all;
  white-space: normal;
  line-height: 1.4;
}

.log-item .size {
  color: var(--primary-color);
  font-weight: 600;
  min-width: 90px;
  text-align: right;
  flex-shrink: 0;
}

.action-col {
  min-width: 120px;
  text-align: center;
  flex-shrink: 0;
}

.log-item .actions {
  display: flex;
  gap: var(--spacing-xs);
  flex-shrink: 0;
}

.log-item button {
  padding: 4px var(--spacing-sm);
  font-size: var(--font-size-sm);
  transition: all var(--transition-slow) var(--ease-spring);
}

.log-item .actions button:hover {
  box-shadow: 0 0 20px var(--glow-primary);
  transform: translateY(-1px);
}

.log-item .actions button:active {
  transform: scale(0.95);
}

.no-results {
  padding: var(--spacing-3xl) var(--spacing-xl);
  text-align: center;
  color: var(--text-color-3);
  font-size: var(--font-size-md);
}

/* 全部折叠/展开 */
.collapse-all-btn {
  flex-shrink: 0;
  padding: 4px var(--spacing-sm);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-xs);
  background: var(--glass-bg);
  color: var(--text-color-2);
  font-family: var(--font-family);
  font-size: var(--font-size-sm);
  white-space: nowrap;
  cursor: pointer;
  transition: all var(--transition-fast);
}

.collapse-all-btn:hover:not(:disabled) {
  background: var(--glass-bg-strong);
  color: var(--text-color-1);
}

.collapse-all-btn:disabled {
  opacity: 0.45;
  cursor: default;
}

/* 分组视图 */
.log-group {
  display: flex;
  flex-direction: column;
}

.group-header {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
  width: 100%;
  padding: var(--spacing-sm) var(--spacing-xl);
  border: none;
  border-bottom: 1px solid var(--border-color);
  background: var(--glass-bg);
  color: var(--text-color-1);
  font-family: var(--font-family);
  font-size: var(--font-size-md);
  font-weight: var(--font-weight-semibold);
  text-align: left;
  cursor: pointer;
  transition: background var(--transition-fast);
}

.group-header:hover {
  background: var(--glass-bg-strong);
}

.group-caret {
  flex-shrink: 0;
  width: 1em;
  transition: transform var(--transition-fast);
}

.group-caret.collapsed {
  transform: rotate(-90deg);
}

.group-name {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.group-meta {
  flex-shrink: 0;
  font-size: var(--font-size-xs);
  font-weight: var(--font-weight-normal);
  color: var(--text-color-3);
  font-variant-numeric: tabular-nums;
}

.highlight {
  background: var(--warning-bg);
  color: var(--warning-color);
  padding: 0 2px;
  border-radius: var(--radius-3xs);
}

/* P2: Display path styling */
.display-path {
  display: block;
}

.original-path {
  display: block;
  font-size: var(--font-size-xs);
  color: var(--text-color-3);
  font-family: 'Monaco', 'Menlo', 'Consolas', monospace;
  opacity: 0.7;
  margin-top: 2px;
}

@media (max-width: 768px) {
  .drawer-overlay {
    align-items: flex-end;
  }

  .drawer {
    width: 100%;
    max-width: 100%;
    height: 85vh;
    border-radius: var(--radius-lg) var(--radius-lg) 0 0;
  }

  .drawer-header {
    padding: var(--spacing-sm) var(--spacing-lg);
  }

  .drawer-header h3 {
    font-size: var(--font-size-lg);
  }

  .close-btn {
    font-size: var(--font-size-2xl);
    width: 18px;
    padding: 0;
    margin-left: auto;
    overflow: hidden;
  }

  .drawer-search {
    padding: var(--spacing-xs) var(--spacing-lg);
  }

  .search-input {
    padding: var(--spacing-xs) var(--spacing-sm);
    font-size: var(--font-size-base);
  }

  .clear-btn {
    padding: 4px var(--spacing-xs);
    font-size: var(--font-size-md);
  }

  .search-count {
    font-size: var(--font-size-xs);
  }

  .log-item {
    padding: var(--spacing-sm) var(--spacing-lg);
    flex-wrap: wrap;
  }

  .log-item .path {
    width: 100%;
    font-size: var(--font-size-sm);
    margin-bottom: var(--spacing-xs);
    white-space: normal;
    word-break: break-all;
  }

  .log-item .size {
    font-size: var(--font-size-sm);
    margin-left: 0;
  }

  .log-item .actions {
    width: 100%;
    margin-left: 0;
    margin-top: var(--spacing-sm);
    gap: var(--spacing-xs);
  }

  .log-item .actions button {
    flex: 1;
    padding: var(--spacing-xs) var(--spacing-sm);
    font-size: var(--font-size-sm);
  }

  .log-item.header {
    display: none;
  }

  .group-header {
    padding: var(--spacing-sm) var(--spacing-lg);
    font-size: var(--font-size-base);
  }

  .group-meta {
    font-size: var(--font-size-xs);
  }

  .collapse-all-btn {
    padding: 4px var(--spacing-xs);
    font-size: var(--font-size-xs);
  }

  .no-results {
    padding: var(--spacing-2xl) var(--spacing-lg);
    font-size: var(--font-size-md);
  }
}

@media (max-width: 480px) {
  .drawer {
    height: 90vh;
  }

  .drawer-header {
    padding: var(--spacing-xs) var(--spacing-md);
  }

  .drawer-header h3 {
    font-size: var(--font-size-lg);
  }

  .drawer-search {
    padding: 4px var(--spacing-md);
    flex-wrap: wrap;
  }

  .search-input {
    padding: var(--spacing-xs) var(--spacing-xs);
    font-size: var(--font-size-md);
  }

  .log-item {
    padding: var(--spacing-xs) var(--spacing-md);
  }

  .log-item .path {
    font-size: var(--font-size-base);
  }

  .log-item .size {
    font-size: var(--font-size-base);
  }

  .log-item .actions button {
    padding: var(--spacing-xs) var(--spacing-xs);
    font-size: var(--font-size-base);
  }
}
</style>
