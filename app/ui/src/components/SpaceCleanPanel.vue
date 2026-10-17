<template>
  <div class="hm-overlay-base" @click.self="close">
    <div class="hm-modal-base space-panel">
      <div class="panel-header">
        <div class="header-title">
          <h3>系统空间清理</h3>
          <span class="header-sub" v-if="usage">
            <template v-if="dataFreshness">{{ dataFreshness }} · </template>扫描耗时 {{ durationLabel }} ·
            发现 {{ usage.categoriesWithItems }} 类可清理项
          </span>
          <!-- 数据偏旧 / 后台更新中：只是提示，不是错误，所以走弱化文字而不是错误横幅。 -->
          <span class="header-freshness" v-if="freshnessHint">{{ freshnessHint }}</span>
        </div>
        <div class="header-actions">
          <button class="ghost-btn" :disabled="loading" @click="load(true)">
            {{ loading ? '扫描中…' : '重新扫描' }}
          </button>
          <button class="close-btn" @click="close">×</button>
        </div>
      </div>

      <div class="panel-body">
        <!-- 错误 / 结果提示 -->
        <div class="error-banner" v-if="errorMessage">
          <span class="banner-text">{{ errorMessage }}</span>
          <button class="banner-dismiss" @click="errorMessage = null">×</button>
        </div>
        <div class="success-banner" v-if="successMessage">
          <span class="banner-text">{{ successMessage }}</span>
          <button class="banner-dismiss" @click="successMessage = null">×</button>
        </div>

        <!-- 加载骨架 -->
        <div class="loading-state" v-if="loading && !usage">
          <div class="hm-shimmer shimmer-hero"></div>
          <div class="shimmer-grid">
            <div class="hm-shimmer shimmer-card" v-for="i in 4" :key="i"></div>
          </div>
          <span class="loading-tip">正在扫描磁盘占用，首次扫描需要几秒…</span>
        </div>

        <template v-if="usage">
          <!-- ══ 可视化一：磁盘容量条 ══ -->
          <section class="viz-section">
            <div class="section-title">磁盘与分区使用情况</div>
            <div class="volume-list">
              <div class="volume-row" v-for="vol in visibleVolumes()" :key="vol.mount">
                <div class="volume-head">
                  <!-- title 里放原始挂载路径：日常只看语义名，排查挂载问题时鼠标一停就能看到真身。 -->
                  <span class="volume-mount" :title="vol.mount">{{ volLabel(vol) }}</span>
                  <span class="volume-numbers">
                    <b :class="usageClass(vol.usedPercent)">{{ vol.usedFormatted }}</b>
                    / {{ vol.totalFormatted }}
                    <span class="volume-free">可用 {{ vol.freeFormatted }}</span>
                  </span>
                </div>
                <div class="bar-track" :title="`已用 ${vol.usedPercent.toFixed(1)}%`">
                  <div
                    class="bar-used"
                    :class="usageClass(vol.usedPercent)"
                    :style="{ width: barWidth(vol.usedPercent) }"
                  ></div>
                </div>
                <div class="bar-legend">
                  <span class="legend-dot used"></span>已用 {{ vol.usedPercent.toFixed(1) }}%
                  <span class="legend-dot free"></span>可用 {{ (100 - vol.usedPercent).toFixed(1) }}%
                  <span class="legend-hint" v-if="tempHint(vol)">{{ tempHint(vol) }}</span>
                </div>
              </div>
            </div>
          </section>

          <!-- ══ 可视化二：可释放空间总览 ══ -->
          <section class="viz-section">
            <div class="section-title">可释放空间总览</div>
            <div class="stats-row">
              <div class="stat-card primary">
                <span class="stat-value">{{ usage.reclaimableFormatted }}</span>
                <span class="stat-label">可安全释放</span>
              </div>
              <div class="stat-card">
                <span class="stat-value">{{ selectedCount }}</span>
                <span class="stat-label">已选条目</span>
              </div>
              <div class="stat-card" :class="{ success: selectedBytes > 0 }">
                <span class="stat-value">{{ formatBytes(selectedBytes) }}</span>
                <span class="stat-label">已选可释放</span>
              </div>
              <div class="stat-card">
                <span class="stat-value">{{ cleanableCategories.length }}</span>
                <span class="stat-label">可清理分类</span>
              </div>
            </div>

            <!-- 可视化三：分类占用占比条（堆叠条形图） -->
            <div class="stack-wrap" v-if="usage.reclaimable > 0">
              <div class="stack-bar">
                <div
                  v-for="(cat, i) in cleanableCategories"
                  :key="cat.id"
                  class="stack-seg"
                  :class="`seg-${i % 6}`"
                  :style="{ width: stackWidth(cat.reclaimable) }"
                  :title="`${cat.name}: ${cat.reclaimableFormatted}`"
                ></div>
              </div>
              <div class="stack-legend">
                <span class="stack-legend-item" v-for="(cat, i) in cleanableCategories" :key="cat.id">
                  <span class="legend-swatch" :class="`seg-${i % 6}`"></span>
                  {{ cat.name }}
                  <b>{{ cat.reclaimableFormatted }}</b>
                </span>
              </div>
            </div>
          </section>

          <!-- ══ 分类列表（可展开、可勾选） ══ -->
          <section class="viz-section">
            <div class="section-title-row">
              <div class="section-title">清理分类</div>
              <div class="bulk-actions">
                <button class="link-btn" @click="selectAllSafe">全选低风险</button>
                <button class="link-btn" @click="clearSelection">清空选择</button>
              </div>
            </div>

            <div class="category-list">
              <div
                v-for="cat in usage.categories"
                :key="cat.id"
                class="category-card"
                :class="[`risk-${cat.risk}`, { empty: cat.itemCount === 0, open: expanded.has(cat.id) }]"
              >
                <div class="category-head" @click="toggleExpand(cat.id)">
                  <span class="expand-arrow" :class="{ open: expanded.has(cat.id) }">▸</span>
                  <div class="category-main">
                    <div class="category-name-row">
                      <span class="category-name">{{ cat.name }}</span>
                      <span class="risk-badge" :class="`risk-${cat.risk}`">{{ riskLabel(cat.risk) }}</span>
                      <span class="category-count" v-if="cat.itemCount > 0">{{ cat.itemCount }} 项</span>
                    </div>
                    <div class="category-desc">{{ cat.description }}</div>
                  </div>
                  <div class="category-size">
                    <span class="size-value" :class="{ zero: cat.reclaimable === 0 }">
                      {{ cat.reclaimableFormatted }}
                    </span>
                    <span class="size-hint">{{ cat.risk === 'high' ? '不可清理' : '可释放' }}</span>
                  </div>
                  <label
                    class="cat-check"
                    v-if="cat.itemCount > 0 && cat.reclaimable > 0"
                    @click.stop
                  >
                    <input
                      type="checkbox"
                      :checked="isCategoryFullySelected(cat)"
                      :indeterminate.prop="isCategoryPartiallySelected(cat)"
                      @change="toggleCategory(cat, ($event.target as HTMLInputElement).checked)"
                    />
                  </label>
                </div>

                <!-- 展开后的条目明细 -->
                <div class="item-list" v-if="expanded.has(cat.id) && cat.itemCount > 0">
                  <div
                    v-for="item in cat.items"
                    :key="item.path"
                    class="item-row"
                    :class="{ unsafe: !item.safe }"
                  >
                    <label class="item-check" v-if="item.safe" @click.stop>
                      <input
                        type="checkbox"
                        :checked="selected.has(item.path)"
                        @change="toggleItem(item, ($event.target as HTMLInputElement).checked)"
                      />
                    </label>
                    <span class="item-check placeholder" v-else :title="item.inUse ? '使用中，暂不可清理' : '该项不可清理'">
                      {{ item.inUse ? '⏳' : '🔒' }}
                    </span>

                    <div class="item-main">
                      <div class="item-name-row">
                        <span class="item-icon">{{ item.isDir ? '📁' : '📄' }}</span>
                        <span class="item-name" :title="item.path">{{ item.name }}</span>
                        <span class="strategy-tag" :class="`strat-${item.strategy}`">
                          {{ strategyLabel(item.strategy) }}
                        </span>
                        <span class="strategy-tag strat-live" v-if="item.inUse">使用中</span>
                      </div>
                      <div class="item-meta">
                        <code class="item-path" :title="item.path">{{ item.path }}</code>
                      </div>
                      <div class="item-reason">{{ item.reason }}</div>
                      <div class="item-extra">
                        <span v-if="item.fileCount > 0">{{ item.fileCount.toLocaleString() }} 个文件</span>
                        <span v-if="item.modified">最后修改 {{ formatTime(item.modified) }}</span>
                      </div>
                    </div>

                    <div class="item-size">{{ item.sizeFormatted }}</div>
                  </div>
                </div>

                <div class="category-empty" v-if="cat.itemCount === 0">未发现此类占用</div>
              </div>
            </div>
          </section>

          <!-- ══ 不可清理的大占用（解释原因） ══ -->
          <section class="viz-section" v-if="usage.notices.length > 0">
            <div class="section-title">
              占用较大但不可清理
              <span class="section-title-hint">这些目录属于系统或用户数据，删除会导致功能异常或数据丢失</span>
            </div>
            <div class="notice-list">
              <div class="notice-row" v-for="n in usage.notices" :key="n.path">
                <div class="notice-main">
                  <div class="notice-name">
                    {{ n.name }}
                    <span class="notice-size">{{ n.sizeFormatted }}{{ n.truncated ? '+' : '' }}</span>
                  </div>
                  <code class="item-path">{{ n.path }}</code>
                  <div class="notice-reason">{{ n.reason }}</div>
                </div>
              </div>
            </div>
          </section>

          <!-- 清理结果明细 -->
          <section class="viz-section" v-if="lastResult">
            <div class="section-title">
              上次清理结果
              <span class="section-title-hint">
                释放 {{ lastResult.freedFormatted }}
                · 删除 {{ lastResult.deleted }}
                · 回收站 {{ lastResult.recycled }}
                · 截断 {{ lastResult.truncated }}
                · 失败 {{ lastResult.failed }}
              </span>
            </div>
            <div class="result-list">
              <div
                v-for="r in lastResult.items"
                :key="r.path"
                class="result-row"
                :class="`status-${r.status}`"
              >
                <span class="result-status">{{ statusLabel(r.status) }}</span>
                <code class="item-path">{{ r.path }}</code>
                <span class="result-freed">{{ formatBytes(r.freedBytes) }}</span>
                <span class="result-msg" v-if="r.message">{{ r.message }}</span>
              </div>
            </div>
          </section>
        </template>
      </div>

      <div class="panel-footer" v-if="usage">
        <div class="footer-info">
          <template v-if="selectedCount > 0">
            已选 <b>{{ selectedCount }}</b> 项，预计释放 <b>{{ formatBytes(selectedBytes) }}</b>
          </template>
          <template v-else>请勾选要清理的项目</template>
        </div>
        <div class="footer-actions">
          <button class="ghost-btn" :disabled="loading || selectedCount === 0" @click="runDryRun">
            试运行
          </button>
          <button
            class="danger-btn"
            :disabled="loading || selectedCount === 0"
            @click="confirmOpen = true"
          >
            {{ loading ? '清理中…' : '立即清理' }}
          </button>
        </div>
      </div>
    </div>

    <!-- 二次确认弹窗 -->
    <div class="hm-overlay-base confirm-overlay" v-if="confirmOpen" @click.self="confirmOpen = false">
      <div class="hm-modal-base confirm-box">
        <div class="confirm-title">确认清理？</div>
        <div class="confirm-body">
          <p>
            即将清理 <b>{{ selectedCount }}</b> 个项目，预计释放
            <b class="danger-text">{{ formatBytes(selectedBytes) }}</b>。
          </p>
          <div class="confirm-breakdown">
            <div class="breakdown-row" v-for="g in selectionBreakdown" :key="g.strategy">
              <span class="breakdown-label">{{ strategyLabel(g.strategy) }}</span>
              <span class="breakdown-value">{{ g.count }} 项 · {{ formatBytes(g.bytes) }}</span>
            </div>
          </div>
          <p class="confirm-note">
            纯缓存类项目将直接删除以立即释放空间；数据库备份类项目会移入回收站，可在保留期内还原。
            此操作会记录审计日志。
          </p>
        </div>
        <div class="confirm-actions">
          <button class="ghost-btn" @click="confirmOpen = false">取消</button>
          <button class="danger-btn" @click="doClean">确认清理</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, reactive, onMounted, onBeforeUnmount } from 'vue'
import { spaceApi, type SpaceCategory, type SpaceItem, type SpaceUsageResponse, type SpaceCleanResponse, type VolumeUsage } from '../services/api'

const emit = defineEmits<{
  close: []
}>()

const loading = ref(false)
const usage = ref<SpaceUsageResponse | null>(null)
const errorMessage = ref<string | null>(null)
const successMessage = ref<string | null>(null)
const expanded = reactive(new Set<string>())
const selected = reactive(new Set<string>())
const confirmOpen = ref(false)
const lastResult = ref<SpaceCleanResponse | null>(null)

/**
 * 页头「扫描耗时」文案。
 *
 * 这个数字属于「产出当前这份数据的那次扫描」，不是本次打开面板的耗时，所以文案只写
 * 「扫描耗时」，不写「本次耗时」。耗时超一秒后毫秒数没有阅读价值（真机上常见 3~10 秒），
 * 换成秒；再长就换成分钟，避免出现「扫描耗时 512340 ms」这种要用户自己换算的数字。
 */
const durationLabel = computed(() => {
  const ms = usage.value?.durationMs
  if (typeof ms !== 'number' || !isFinite(ms) || ms < 0) return '未知'
  if (ms < 1000) return `${Math.round(ms)} ms`
  if (ms < 60000) return `${(ms / 1000).toFixed(1)} 秒`
  return `${Math.round(ms / 60000)} 分钟`
})

/** 把服务端给的秒级年龄转成「x 前」。与 formatTime 同样把极小值收敛成「刚刚」。 */
function formatAgeSeconds(seconds: number): string {
  if (seconds < 60) return '刚刚'
  if (seconds < 3600) return `${Math.floor(seconds / 60)} 分钟前`
  if (seconds < 86400) return `${Math.floor(seconds / 3600)} 小时前`
  return `${Math.floor(seconds / 86400)} 天前`
}

/**
 * 页头「数据更新于 …」文案。
 *
 * 优先用服务端算好的 cacheAgeSeconds，而不是本地 Date.now() - scannedAt：面板刚打开时
 * 可能直接命中服务端持久化的快照，而客户端时钟与服务端不一致时，本地算差会得出
 * 「来自未来」或「几小时前」的错数据。服务端没给（旧服务端/旧缓存）时才退回 scannedAt。
 */
const dataFreshness = computed(() => {
  const u = usage.value
  if (!u) return ''
  const secs = u.cacheAgeSeconds
  if (typeof secs === 'number' && isFinite(secs) && secs >= 0) {
    const rel = formatAgeSeconds(secs)
    return rel === '刚刚' ? '数据刚刚更新' : `数据更新于 ${rel}`
  }
  const fallback = formatTime(u.scannedAt)
  return fallback ? `数据更新于 ${fallback}` : ''
})

/**
 * 数据偏旧 / 后台重扫进行中的提示。
 *
 * 这里是「提示」不是「错误」：stale 只说数据旧、refreshing 只说此刻有请求在飞，
 * 面板展示的仍是可用数据，所以走页头弱化文字，而不是错误横幅。
 * 两个标志的可能组合都要有话说：stale+refreshing（旧且正在更新）、
 * 只有 refreshing（正在更新）、只有 stale（旧但此刻没在扫，等用户手动重扫）。
 */
const freshnessHint = computed(() => {
  const u = usage.value
  if (!u) return ''
  if (u.refreshing) return u.stale ? '数据已偏旧，正在后台更新…' : '正在后台更新…'
  if (u.stale) return '数据已偏旧，可点「重新扫描」刷新'
  return ''
})

/** 选中条目的路径 -> 条目，用于计算体积与清理策略分组。 */
const selectedItems = computed<SpaceItem[]>(() => {
  if (!usage.value) return []
  const out: SpaceItem[] = []
  for (const cat of usage.value.categories) {
    for (const item of cat.items) {
      if (selected.has(item.path)) out.push(item)
    }
  }
  return out
})

const selectedCount = computed(() => selectedItems.value.length)
const selectedBytes = computed(() => selectedItems.value.reduce((s, i) => s + i.size, 0))

/** 真正有可释放空间的分类，用于堆叠占比条。 */
const cleanableCategories = computed(() =>
  (usage.value?.categories || []).filter(c => c.reclaimable > 0)
)

/** 只统计低风险分类，作为「全选」的默认范围。 */
const selectAllSafe = () => {
  if (!usage.value) return
  for (const cat of usage.value.categories) {
    if (cat.risk !== 'low') continue
    for (const item of cat.items) {
      if (item.safe) selected.add(item.path)
    }
  }
}

const clearSelection = () => selected.clear()

/** 按清理策略分组，供确认弹窗展示不同处理方式的量。 */
const selectionBreakdown = computed(() => {
  const map = new Map<string, { strategy: string; count: number; bytes: number }>()
  for (const item of selectedItems.value) {
    const g = map.get(item.strategy) || { strategy: item.strategy, count: 0, bytes: 0 }
    g.count++
    g.bytes += item.size
    map.set(item.strategy, g)
  }
  return [...map.values()]
})

function isCategoryFullySelected(cat: SpaceCategory) {
  const safeItems = cat.items.filter(i => i.safe)
  return safeItems.length > 0 && safeItems.every(i => selected.has(i.path))
}

function isCategoryPartiallySelected(cat: SpaceCategory) {
  const safeItems = cat.items.filter(i => i.safe)
  const n = safeItems.filter(i => selected.has(i.path)).length
  return n > 0 && n < safeItems.length
}

function toggleCategory(cat: SpaceCategory, on: boolean) {
  for (const item of cat.items) {
    if (!item.safe) continue
    if (on) selected.add(item.path)
    else selected.delete(item.path)
  }
}

function toggleItem(item: SpaceItem, on: boolean) {
  if (on) selected.add(item.path)
  else selected.delete(item.path)
}

function toggleExpand(id: string) {
  if (expanded.has(id)) expanded.delete(id)
  else expanded.add(id)
}

function formatBytes(bytes: number): string {
  if (!bytes || bytes <= 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let v = bytes
  let i = 0
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024
    i++
  }
  return `${v >= 100 || i === 0 ? Math.round(v) : v.toFixed(1)} ${units[i]}`
}

function formatTime(iso: string): string {
  if (!iso) return ''
  const d = new Date(iso)
  if (isNaN(d.getTime())) return ''
  const now = Date.now()
  const diff = now - d.getTime()
  const day = 86400000
  if (diff < 0) return d.toLocaleDateString()
  if (diff < 3600000) return `${Math.max(1, Math.floor(diff / 60000))} 分钟前`
  if (diff < day) return `${Math.floor(diff / 3600000)} 小时前`
  if (diff < 30 * day) return `${Math.floor(diff / day)} 天前`
  return d.toLocaleDateString()
}

/** 容量条宽度：保留极小值的可见性，避免 0.3% 完全看不见。 */
function barWidth(percent: number): string {
  if (percent <= 0) return '0%'
  return `${Math.max(1.5, Math.min(100, percent)).toFixed(2)}%`
}

function stackWidth(bytes: number): string {
  const total = usage.value?.reclaimable || 0
  if (!total) return '0%'
  return `${Math.max(0.8, (bytes / total) * 100).toFixed(2)}%`
}

function usageClass(percent: number): string {
  if (percent >= 90) return 'danger'
  if (percent >= 75) return 'warning'
  return 'ok'
}

/**
 * 挂载点显示名。
 *
 * 标签由服务端给出：只有服务端能看到挂载表，因而只有它能判断一个路径是系统空间、
 * 存储空间还是外接存储。前端不再自己解析 /volN 路径——那正是「把 /vol00、/vol02
 * 这类普通目录当成分区显示」的来源。
 *
 * 兜底顺序：服务端 label → 旧的本地推断（兼容未升级的服务端/旧缓存）。本地推断只
 * 认系统分区，其余一律显示原始路径而不是猜一个语义名。
 */
function volLabel(vol: VolumeUsage): string {
  if (vol.label) return vol.label
  if (vol.mount === '/') return '系统空间'
  return vol.mount
}

/**
 * 磁盘与分区使用情况要渲染的行。
 *
 * 外接存储不显示：「系统空间清理」的清理规则不扫外接盘，把外接盘列在这里只会让人
 * 以为这一屏能管到它的空间。服务端已经不再下发外接盘，这里再过滤一次是为了挡住
 * 旧服务端/旧快照缓存里的外接盘行。
 */
function visibleVolumes(): VolumeUsage[] {
  const list = usage.value?.volumes ?? []
  return list.filter((v) => v.kind !== 'external' && !/^\/vol00(\/|$)/.test(v.mount || ''))
}

/**
 * tempHint 返回容量条旁的提示文案。
 *
 * 只有服务端实测确认 /tmp、/var/tmp 与系统分区同处一个文件系统时才提示；
 * 读不到设备号（tempOnSystemDisk 为 undefined）时返回空串，不显示任何断言——
 * /tmp 有可能是 tmpfs，那时清理释放的是内存而非该分区空间。
 */
function tempHint(vol: VolumeUsage): string {
  if (vol.mount !== '/') return ''
  if (usage.value?.tempOnSystemDisk === true) return '系统分区，/tmp 与 /var/tmp 位于此处'
  if (usage.value?.tempOnSystemDisk === false) return '系统分区（/tmp、/var/tmp 不在该分区上）'
  return ''
}

function riskLabel(risk: string): string {
  return { low: '低风险', medium: '中风险', high: '高风险' }[risk] || risk
}

function strategyLabel(strategy: string): string {
  return {
    delete: '直接删除',
    recycle: '移入回收站',
    truncate: '截断',
    none: '不可清理',
  }[strategy] || strategy
}

function statusLabel(status: string): string {
  return {
    deleted: '已删除',
    recycled: '已入回收站',
    truncated: '已截断',
    skipped: '已跳过',
    failed: '失败',
  }[status] || status
}

/**
 * 后台重扫的收敛轮询参数。
 *
 * 服务端拿到陈旧快照时会自己在后台起一次重扫并立即返回旧数据（refreshing=true），
 * 所以前端不需要为「看到新数据」再触发一次全量扫描，只要隔几秒回来取一次结果。
 * 上限 12 次（约 1 分钟）是必要的兜底：真机上扫描失败或某次重扫卡住时，
 * refreshing 可能一直是 true，没有上限的定时器会变成一个永不停止的背景请求。
 */
const POLL_INTERVAL_MS = 5000
const POLL_MAX_ATTEMPTS = 12

let pollTimer: ReturnType<typeof setTimeout> | null = null
let pollAttempts = 0
let disposed = false
let requestSeq = 0
let inFlight = false

/** 停表并归还轮询预算。任何一次「重新开始」都要先清掉旧定时器。 */
function stopPolling() {
  if (pollTimer !== null) {
    clearTimeout(pollTimer)
    pollTimer = null
  }
  pollAttempts = 0
}

/**
 * 安排下一次取数。
 *
 * 先清掉旧定时器：任何一次新的 load 都不允许留下两个并存的轮询者，
 * 否则重扫几次之后请求会成倍增长，而失败的次数也会跟着成倍。
 */
function schedulePoll() {
  if (pollTimer !== null) {
    clearTimeout(pollTimer)
    pollTimer = null
  }
  if (disposed || pollAttempts >= POLL_MAX_ATTEMPTS) return
  pollAttempts++
  pollTimer = setTimeout(() => {
    pollTimer = null
    // 必须走非强制请求：服务端自己负责那次重扫，客户端只是来取结果；
    // 传 refresh=true 会再触发一次全量扫描，把「打开面板不再等全盘扫描」
    // 重新变成一次同步全盘扫描。background=true 让它不闪骨架屏。
    void load(false, true)
  }, POLL_INTERVAL_MS)
}

/**
 * 拉取一次系统空间数据。
 *
 * @param refresh  true 表示强制服务端同步全量重扫（用户点「重新扫描」时用）。
 * @param background true 表示这是后台轮询取数：不改动 loading，
 *                   避免面板每 5 秒闪一次加载态、按钮也跟着禁用。
 */
async function load(refresh = false, background = false) {
  if (disposed) return
  // 并发保护：非强制请求在已有请求在飞时直接跳过——挂载加载与后台轮询都是
  // 「取当前快照」，重复发起只会更快撞上服务端 30 次/分钟的读限流。
  // 用户主动点的强制刷新不跳过（结果按序号取舍，见下方 seq 判断）。
  if (inFlight && !refresh) return

  // 用户主动发起的加载重新给一轮轮询预算；后台轮询不能重置，
  // 否则「最多 12 次」永远到不了，定时器会一直活着。
  if (!background) stopPolling()

  const seq = ++requestSeq
  inFlight = true
  if (!background) loading.value = true
  errorMessage.value = null
  try {
    const data = await spaceApi.getUsage(refresh)
    // 迟到的旧响应（例如用户在轮询期间点了重新扫描）不能覆盖更新的数据。
    if (seq !== requestSeq) return
    usage.value = data
    if (data.error) {
      errorMessage.value = data.error
      // 出错后不再轮询：持续失败只会反复弹同一个错误。
      // 用户看到错误后自己点「重新扫描」比自动重试更可控。
      stopPolling()
    } else if (data.refreshing) {
      schedulePoll()
    } else {
      stopPolling()
    }
    // 扫描后清理已经不存在的选中项，避免残留选择被提交。
    const valid = new Set<string>()
    for (const cat of data.categories) for (const i of cat.items) valid.add(i.path)
    for (const p of [...selected]) if (!valid.has(p)) selected.delete(p)
    // 分类明细默认折叠：分类本身已有名称、条目数与可释放体积，足够判断是否需要下钻；
    // 默认全展开会把十几个分类的子项一次性铺开，把汇总信息挤出视野。
    // 展开状态由用户点击分类头决定，重扫后按分类 id 保留。
  } catch (e: any) {
    if (seq !== requestSeq) return
    errorMessage.value = e?.message || '扫描系统空间失败'
    stopPolling()
  } finally {
    // 只有最新一次请求归位时才收尾，否则迟到的旧请求会把新请求的加载态提前关掉。
    if (seq === requestSeq) {
      inFlight = false
      loading.value = false
    }
  }
}

async function runDryRun() {
  loading.value = true
  errorMessage.value = null
  successMessage.value = null
  try {
    const res = await spaceApi.clean(selectedItems.value.map(i => i.path), true)
    lastResult.value = res
    successMessage.value = `试运行完成：将处理 ${res.items.length} 项，预计释放 ${res.freedFormatted}（未做任何修改）`
  } catch (e: any) {
    errorMessage.value = e?.message || '试运行失败'
  } finally {
    loading.value = false
  }
}

async function doClean() {
  confirmOpen.value = false
  loading.value = true
  errorMessage.value = null
  successMessage.value = null
  const paths = selectedItems.value.map(i => i.path)
  try {
    const res = await spaceApi.clean(paths, false)
    lastResult.value = res
    const parts = [`释放 ${res.freedFormatted}`]
    if (res.deleted) parts.push(`删除 ${res.deleted} 项`)
    if (res.recycled) parts.push(`回收站 ${res.recycled} 项`)
    if (res.truncated) parts.push(`截断 ${res.truncated} 项`)
    if (res.failed) parts.push(`失败 ${res.failed} 项`)
    successMessage.value = '清理完成：' + parts.join('，')
    if (res.errors && res.errors.length) {
      errorMessage.value = '部分项目失败：' + res.errors.slice(0, 3).join('；')
    }
    selected.clear()
    await load(true)
  } catch (e: any) {
    errorMessage.value = e?.message || '清理失败'
  } finally {
    loading.value = false
  }
}

function close() {
  // 关面板即停表：面板是 v-if 挂载的，但定时器不会随 DOM 消失而消失，
  // 留着就会在面板关闭后继续向服务端要数据。
  stopPolling()
  emit('close')
}

onMounted(() => load(false))

onBeforeUnmount(() => {
  // 卸载（关面板）后迟到的响应不再写状态，也不再安排下一次轮询。
  disposed = true
  stopPolling()
})
</script>

<style scoped>
.space-panel {
  max-width: 1080px;
  width: 95%;
  max-height: 92vh;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

/* ── 头部 ── */
.panel-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: var(--spacing-md) var(--spacing-xl);
  border-bottom: 1px solid var(--border-color);
  flex-shrink: 0;
  gap: var(--spacing-md);
}

.header-title h3 {
  margin: 0;
  font-size: var(--font-size-xl);
  font-weight: 500;
  color: var(--text-color-1);
}

.header-sub {
  display: block;
  margin-top: 2px;
  font-size: var(--font-size-xs);
  color: var(--text-color-3);
}

/* 数据偏旧 / 后台更新中：弱化文字 + 呼吸动画，与错误横幅区分开。
   用 text-color-3 而非警示色：展示的仍是可用数据，标成警告会让人以为面板坏了。 */
.header-freshness {
  display: block;
  margin-top: 2px;
  font-size: var(--font-size-xs);
  color: var(--text-color-3);
  animation: pulse-text 2s ease-in-out infinite;
}

@keyframes pulse-text {
  0%, 100% { opacity: 0.5; }
  50% { opacity: 1; }
}

.header-actions {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
  flex-shrink: 0;
}

.close-btn {
  width: 32px;
  height: 32px;
  border: none;
  border-radius: var(--radius-full);
  background: transparent;
  color: var(--text-color-2);
  font-size: 22px;
  line-height: 1;
  cursor: pointer;
  transition: background var(--transition-fast), color var(--transition-fast);
}

.close-btn:hover {
  background: var(--error-bg);
  color: var(--error-color);
}

.ghost-btn {
  padding: 6px 14px;
  border: 1px solid var(--border-color);
  border-radius: var(--radius-full);
  background: transparent;
  color: var(--text-color-1);
  font-size: var(--font-size-sm);
  cursor: pointer;
  transition: background var(--transition-fast), border-color var(--transition-fast);
}

.ghost-btn:hover:not(:disabled) {
  background: var(--primary-soft);
  border-color: var(--primary-color);
  color: var(--primary-color);
}

.ghost-btn:disabled {
  opacity: 0.45;
  cursor: not-allowed;
}

.danger-btn {
  padding: 6px 18px;
  border: none;
  border-radius: var(--radius-full);
  background: var(--error-color);
  color: #fff;
  font-size: var(--font-size-sm);
  font-weight: 500;
  cursor: pointer;
  transition: filter var(--transition-fast), transform var(--transition-fast);
}

.danger-btn:hover:not(:disabled) {
  filter: brightness(1.08);
}

.danger-btn:active:not(:disabled) {
  transform: scale(0.97);
}

.danger-btn:disabled {
  opacity: 0.45;
  cursor: not-allowed;
}

/* ── 主体 ── */
.panel-body {
  flex: 1;
  overflow-y: auto;
  padding: var(--spacing-lg) var(--spacing-xl);
  display: flex;
  flex-direction: column;
  gap: var(--spacing-lg);
}

.banner-text {
  flex: 1;
  font-size: var(--font-size-sm);
}

.error-banner,
.success-banner {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
  padding: var(--spacing-sm) var(--spacing-md);
  border-radius: var(--radius-sm);
  font-size: var(--font-size-sm);
}

.error-banner {
  background: var(--error-bg);
  color: var(--error-color);
  border: 1px solid rgba(232, 64, 38, 0.25);
}

.success-banner {
  background: var(--success-bg);
  color: var(--success-color);
  border: 1px solid rgba(100, 187, 90, 0.25);
}

.banner-dismiss {
  border: none;
  background: transparent;
  color: inherit;
  font-size: 18px;
  line-height: 1;
  cursor: pointer;
  opacity: 0.7;
}

.banner-dismiss:hover {
  opacity: 1;
}

/* ── 骨架屏 ── */
.loading-state {
  display: flex;
  flex-direction: column;
  gap: var(--spacing-md);
  align-items: center;
}

.shimmer-hero {
  width: 100%;
  height: 92px;
}

.shimmer-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: var(--spacing-md);
  width: 100%;
}

.shimmer-card {
  height: 78px;
}

.loading-tip {
  font-size: var(--font-size-sm);
  color: var(--text-color-3);
}

/* ── 区块 ── */
.viz-section {
  display: flex;
  flex-direction: column;
  gap: var(--spacing-sm);
}

.section-title {
  font-size: var(--font-size-sm);
  font-weight: 600;
  color: var(--text-color-2);
  letter-spacing: var(--letter-spacing-wide);
}

.section-title-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.section-title-hint {
  margin-left: var(--spacing-sm);
  font-weight: 400;
  font-size: var(--font-size-xs);
  color: var(--text-color-3);
}

.bulk-actions {
  display: flex;
  gap: var(--spacing-md);
}

.link-btn {
  border: none;
  background: transparent;
  color: var(--primary-color);
  font-size: var(--font-size-sm);
  cursor: pointer;
  padding: 2px 4px;
  border-radius: var(--radius-2xs);
}

.link-btn:hover {
  background: var(--primary-soft);
}

/* ── 容量条 ── */
.volume-list {
  display: flex;
  flex-direction: column;
  gap: var(--spacing-md);
}

.volume-row {
  padding: var(--spacing-md);
  background: var(--card-bg);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-sm);
}

.volume-head {
  display: flex;
  justify-content: space-between;
  align-items: baseline;
  margin-bottom: var(--spacing-sm);
  gap: var(--spacing-sm);
}

.volume-mount {
  /* 这里显示的是飞牛语义名（系统空间 / 存储空间1），不是路径，因此不用等宽字体。
     标签理论上有长度上限，但仍允许换行作为兜底，避免把右侧容量数字挤出卡片。 */
  font-size: var(--font-size-md);
  font-weight: 600;
  color: var(--text-color-1);
  min-width: 0;
  overflow-wrap: anywhere;
}

.volume-numbers {
  flex-shrink: 0;
  font-size: var(--font-size-sm);
  color: var(--text-color-2);
}

.volume-numbers b.ok { color: var(--success-color); }
.volume-numbers b.warning { color: var(--warning-color); }
.volume-numbers b.danger { color: var(--error-color); }

.volume-free {
  margin-left: var(--spacing-sm);
  color: var(--text-color-3);
}

.bar-track {
  position: relative;
  height: 14px;
  border-radius: var(--radius-full);
  background: var(--bg-color-3);
  overflow: hidden;
}

.bar-used {
  height: 100%;
  border-radius: var(--radius-full);
  transition: width var(--transition-base);
}

.bar-used.ok { background: var(--primary-gradient); }
.bar-used.warning { background: linear-gradient(90deg, #E8B339, #F0C860); }
.bar-used.danger { background: linear-gradient(90deg, #E84026, #F06048); }

.bar-legend {
  display: flex;
  align-items: center;
  gap: var(--spacing-xs);
  margin-top: 6px;
  font-size: var(--font-size-xs);
  color: var(--text-color-3);
}

.legend-dot {
  display: inline-block;
  width: 8px;
  height: 8px;
  border-radius: var(--radius-full);
  margin-left: var(--spacing-sm);
}

.legend-dot.used { background: var(--primary-color); margin-left: 0; }
.legend-dot.free { background: var(--bg-color-3); }

/* 系统分区的补充说明：它最紧张却最容易被忽略，需要额外提示。 */
.legend-hint {
  margin-left: auto;
  padding: 1px 8px;
  border-radius: var(--radius-full);
  background: var(--warning-bg);
  color: #B98A16;
  white-space: nowrap;
}

/* ── 统计卡 ── */
.stats-row {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: var(--spacing-md);
}

.stat-card {
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding: var(--spacing-md);
  background: var(--card-bg);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-sm);
  transition: border-color var(--transition-fast);
}

.stat-card.primary {
  border-color: rgba(10, 89, 247, 0.35);
  background: var(--primary-mist);
}

.stat-card.success {
  border-color: rgba(100, 187, 90, 0.35);
}

.stat-value {
  font-size: var(--font-size-xl);
  font-weight: 600;
  color: var(--text-color-1);
  font-variant-numeric: tabular-nums;
}

.stat-card.primary .stat-value { color: var(--primary-color); }
.stat-card.success .stat-value { color: var(--success-color); }

.stat-label {
  font-size: var(--font-size-xs);
  color: var(--text-color-3);
}

/* ── 堆叠占比条 ── */
.stack-wrap {
  margin-top: var(--spacing-sm);
}

.stack-bar {
  display: flex;
  height: 18px;
  border-radius: var(--radius-full);
  overflow: hidden;
  background: var(--bg-color-3);
}

.stack-seg {
  height: 100%;
  transition: width var(--transition-base);
}

.seg-0 { background: #0A59F7; }
.seg-1 { background: #54B2F7; }
.seg-2 { background: #64BB5A; }
.seg-3 { background: #E8B339; }
.seg-4 { background: #8E7CF0; }
.seg-5 { background: #F06048; }

.stack-legend {
  display: flex;
  flex-wrap: wrap;
  gap: var(--spacing-xs) var(--spacing-md);
  margin-top: var(--spacing-sm);
  font-size: var(--font-size-xs);
  color: var(--text-color-2);
}

.stack-legend-item {
  display: inline-flex;
  align-items: center;
  gap: 5px;
}

.stack-legend-item b {
  color: var(--text-color-1);
  font-variant-numeric: tabular-nums;
}

.legend-swatch {
  width: 10px;
  height: 10px;
  border-radius: var(--radius-3xs);
}

/* ── 分类卡 ── */
.category-list {
  display: flex;
  flex-direction: column;
  gap: var(--spacing-sm);
}

.category-card {
  background: var(--card-bg);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-sm);
  overflow: hidden;
  transition: border-color var(--transition-fast), box-shadow var(--transition-fast);
}

.category-card:hover {
  box-shadow: var(--depth-1);
}

.category-card.risk-low { border-left: 3px solid var(--success-color); }
.category-card.risk-medium { border-left: 3px solid var(--warning-color); }
.category-card.risk-high { border-left: 3px solid var(--error-color); }

.category-card.empty {
  opacity: 0.55;
}

.category-head {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
  padding: var(--spacing-md);
  cursor: pointer;
}

.expand-arrow {
  color: var(--text-color-3);
  font-size: 12px;
  transition: transform var(--transition-fast);
  flex-shrink: 0;
}

.expand-arrow.open {
  transform: rotate(90deg);
}

.category-main {
  flex: 1;
  min-width: 0;
}

.category-name-row {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
  flex-wrap: wrap;
}

.category-name {
  font-size: var(--font-size-md);
  font-weight: 600;
  color: var(--text-color-1);
}

.category-count {
  font-size: var(--font-size-xs);
  color: var(--text-color-3);
}

.risk-badge {
  padding: 1px 8px;
  border-radius: var(--radius-full);
  font-size: var(--font-size-xs);
  font-weight: 500;
}

.risk-badge.risk-low { background: var(--success-bg); color: var(--success-color); }
.risk-badge.risk-medium { background: var(--warning-bg); color: #B98A16; }
.risk-badge.risk-high { background: var(--error-bg); color: var(--error-color); }

.category-desc {
  margin-top: 3px;
  font-size: var(--font-size-xs);
  color: var(--text-color-3);
  line-height: 1.45;
}

.category-size {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  flex-shrink: 0;
  min-width: 88px;
}

.size-value {
  font-size: var(--font-size-lg);
  font-weight: 600;
  color: var(--primary-color);
  font-variant-numeric: tabular-nums;
}

.size-value.zero {
  color: var(--text-color-4);
  font-size: var(--font-size-md);
}

.size-hint {
  font-size: var(--font-size-xs);
  color: var(--text-color-3);
}

.cat-check {
  flex-shrink: 0;
  display: flex;
  align-items: center;
  cursor: pointer;
}

.cat-check input,
.item-check input {
  width: 17px;
  height: 17px;
  accent-color: var(--primary-color);
  cursor: pointer;
}

/* ── 条目明细 ── */
.item-list {
  border-top: 1px solid var(--divider-color);
  background: var(--bg-color-2);
}

.item-row {
  display: flex;
  align-items: flex-start;
  gap: var(--spacing-sm);
  padding: var(--spacing-sm) var(--spacing-md);
  border-bottom: 1px solid var(--divider-color);
}

.item-row:last-child {
  border-bottom: none;
}

.item-row.unsafe {
  opacity: 0.62;
  background: var(--error-bg);
}

.item-check {
  display: flex;
  align-items: center;
  padding-top: 2px;
  flex-shrink: 0;
  cursor: pointer;
}

.item-check.placeholder {
  width: 17px;
  font-size: 12px;
  cursor: not-allowed;
}

.item-main {
  flex: 1;
  min-width: 0;
}

.item-name-row {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}

.item-icon {
  font-size: 13px;
}

.item-name {
  font-size: var(--font-size-sm);
  font-weight: 500;
  color: var(--text-color-1);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  max-width: 380px;
}

.strategy-tag {
  padding: 0 6px;
  border-radius: var(--radius-2xs);
  font-size: 11px;
  line-height: 16px;
  flex-shrink: 0;
}

.strat-delete { background: var(--error-bg); color: var(--error-color); }
.strat-recycle { background: var(--info-bg); color: var(--info-color); }
.strat-truncate { background: var(--warning-bg); color: #B98A16; }
.strat-none { background: var(--bg-color-3); color: var(--text-color-2); }
.strat-live { background: var(--warning-bg); color: #B98A16; }

.item-meta {
  margin-top: 3px;
}

.item-path {
  font-family: var(--font-mono);
  font-size: 11px;
  color: var(--text-color-3);
  word-break: break-all;
}

.item-reason {
  margin-top: 3px;
  font-size: var(--font-size-xs);
  color: var(--text-color-2);
  line-height: 1.45;
}

.item-extra {
  display: flex;
  gap: var(--spacing-md);
  margin-top: 3px;
  font-size: 11px;
  color: var(--text-color-4);
}

.item-size {
  flex-shrink: 0;
  font-size: var(--font-size-md);
  font-weight: 600;
  color: var(--text-color-1);
  font-variant-numeric: tabular-nums;
  padding-top: 1px;
}

.category-empty {
  padding: var(--spacing-sm) var(--spacing-md);
  font-size: var(--font-size-xs);
  color: var(--text-color-4);
  border-top: 1px solid var(--divider-color);
}

/* ── 不可清理提示 ── */
.notice-list {
  display: flex;
  flex-direction: column;
  gap: var(--spacing-xs);
}

.notice-row {
  padding: var(--spacing-sm) var(--spacing-md);
  background: var(--bg-color-2);
  border: 1px dashed var(--border-color);
  border-radius: var(--radius-xs);
}

.notice-name {
  display: flex;
  align-items: baseline;
  gap: var(--spacing-sm);
  font-size: var(--font-size-sm);
  font-weight: 500;
  color: var(--text-color-2);
}

.notice-size {
  font-variant-numeric: tabular-nums;
  color: var(--text-color-1);
}

.notice-reason {
  margin-top: 2px;
  font-size: var(--font-size-xs);
  color: var(--text-color-3);
}

/* ── 清理结果 ── */
.result-list {
  display: flex;
  flex-direction: column;
  gap: 3px;
  max-height: 220px;
  overflow-y: auto;
}

.result-row {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
  padding: 5px var(--spacing-sm);
  border-radius: var(--radius-2xs);
  background: var(--bg-color-2);
  font-size: var(--font-size-xs);
}

.result-status {
  flex-shrink: 0;
  min-width: 68px;
  font-weight: 500;
}

.result-row.status-deleted .result-status { color: var(--success-color); }
.result-row.status-recycled .result-status { color: var(--info-color); }
.result-row.status-truncated .result-status { color: #B98A16; }
.result-row.status-skipped .result-status { color: var(--text-color-3); }
.result-row.status-failed .result-status { color: var(--error-color); }

.result-row .item-path {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.result-freed {
  flex-shrink: 0;
  font-variant-numeric: tabular-nums;
  color: var(--text-color-2);
}

.result-msg {
  flex-shrink: 0;
  color: var(--text-color-3);
}

/* ── 底部 ── */
.panel-footer {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: var(--spacing-md);
  padding: var(--spacing-md) var(--spacing-xl);
  border-top: 1px solid var(--border-color);
  flex-shrink: 0;
}

.footer-info {
  font-size: var(--font-size-sm);
  color: var(--text-color-2);
}

.footer-info b {
  color: var(--primary-color);
  font-variant-numeric: tabular-nums;
}

.footer-actions {
  display: flex;
  gap: var(--spacing-sm);
  flex-shrink: 0;
}

/* ── 确认弹窗 ── */
.confirm-overlay {
  z-index: var(--z-modal);
}

.confirm-box {
  max-width: 520px;
  width: 92%;
  padding: var(--spacing-xl);
  display: flex;
  flex-direction: column;
  gap: var(--spacing-md);
}

.confirm-title {
  font-size: var(--font-size-lg);
  font-weight: 600;
  color: var(--text-color-1);
}

.confirm-body {
  font-size: var(--font-size-sm);
  color: var(--text-color-2);
  line-height: 1.6;
}

.confirm-body p {
  margin: 0 0 var(--spacing-sm);
}

.danger-text {
  color: var(--error-color);
}

.confirm-breakdown {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: var(--spacing-sm) var(--spacing-md);
  background: var(--bg-color-2);
  border-radius: var(--radius-xs);
  margin-bottom: var(--spacing-sm);
}

.breakdown-row {
  display: flex;
  justify-content: space-between;
  font-size: var(--font-size-xs);
}

.breakdown-label { color: var(--text-color-2); }
.breakdown-value { color: var(--text-color-1); font-variant-numeric: tabular-nums; }

.confirm-note {
  font-size: var(--font-size-xs);
  color: var(--text-color-3);
  line-height: 1.55;
  margin: 0;
}

.confirm-actions {
  display: flex;
  justify-content: flex-end;
  gap: var(--spacing-sm);
}

@media (max-width: 720px) {
  .stats-row { grid-template-columns: repeat(2, 1fr); }
  .shimmer-grid { grid-template-columns: repeat(2, 1fr); }
  .item-name { max-width: 180px; }
}
</style>
