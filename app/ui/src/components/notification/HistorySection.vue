<template>
  <div class="history-section">
    <div class="section-header">
      <h4>
        <svg class="header-icon" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <circle cx="12" cy="12" r="10"/>
          <polyline points="12 6 12 12 16 14"/>
        </svg>
        通知历史
      </h4>
      <div class="header-actions">
        <div class="filter-tabs">
          <button
            v-for="tab in filterTabs"
            :key="tab.value"
            :class="['filter-tab', { active: activeFilter === tab.value }]"
            @click="activeFilter = tab.value"
          >
            {{ tab.label }}
            <span v-if="tab.count !== undefined" class="tab-count">{{ tab.count }}</span>
          </button>
        </div>
        <button class="clear-btn" @click="$emit('clear')" v-if="filteredHistory.length > 0">
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <polyline points="3 6 5 6 21 6"/>
            <path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/>
          </svg>
          清空
        </button>
      </div>
    </div>

    <div class="history-list" v-if="filteredHistory.length > 0">
      <TransitionGroup name="history-item">
        <div
          class="history-card"
          v-for="item in filteredHistory"
          :key="item.id"
          :class="{ 'is-failed': !item.success }"
          @click="toggleExpand(item.id)"
        >
          <div class="card-main">
            <div class="card-left">
              <div :class="['status-dot', item.success ? 'success' : 'failed']">
                <svg v-if="item.success" width="10" height="10" viewBox="0 0 24 24" fill="currentColor">
                  <path d="M9 16.17L4.83 12l-1.42 1.41L9 19 21 7l-1.41-1.41z"/>
                </svg>
                <svg v-else width="10" height="10" viewBox="0 0 24 24" fill="currentColor">
                  <path d="M18.3 5.71L12 12l6.3 6.29-1.42 1.42L12 13.41l-5.88 5.88-1.42-1.42L10.59 12 4.7 5.71 6.12 4.29 12 10.59l5.88-5.88z"/>
                </svg>
              </div>
            </div>
            <div class="card-content">
              <div class="card-top">
                <span class="channel-badge" :style="{ background: channelColors[item.channel]?.bg || 'var(--info-bg)', color: channelColors[item.channel]?.fg || 'var(--info-color)' }">
                  <span class="channel-icon">{{ channelIcons[item.channel] || '📨' }}</span>
                  {{ getChannelName(item.channel) }}
                </span>
                <span class="time">{{ relativeTime(item.timestamp) }}</span>
              </div>
              <div class="card-title">{{ item.title || '系统通知' }}</div>
              <div class="card-message" v-if="item.message" :class="{ expanded: expandedItems.has(item.id) }">
                {{ item.message }}
              </div>
              <div class="card-error" v-if="!item.success && item.error">
                <svg width="12" height="12" viewBox="0 0 24 24" fill="currentColor">
                  <path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm-2 15l-5-5 1.41-1.41L10 14.17l7.59-7.59L19 8l-9 9z"/>
                </svg>
                {{ item.error }}
              </div>
            </div>
            <div class="card-chevron">
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"
                :class="{ rotated: expandedItems.has(item.id) }">
                <polyline points="6 9 12 15 18 9"/>
              </svg>
            </div>
          </div>
        </div>
      </TransitionGroup>
    </div>

    <div class="empty-state" v-else>
      <svg class="empty-icon" width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
        <path d="M22 17a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 2h9a2 2 0 0 1 2 2v10z"/>
        <line x1="12" y1="11" x2="12" y2="17"/>
        <line x1="9" y1="14" x2="15" y2="14"/>
      </svg>
      <p>暂无通知历史</p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'

interface HistoryItem {
  id: string
  ruleId?: string
  channel: string
  title?: string
  message?: string
  success: boolean
  error?: string
  timestamp: string
}

const props = defineProps<{
  history: HistoryItem[]
}>()

defineEmits<{
  clear: []
}>()

const activeFilter = ref('all')
const expandedItems = ref(new Set<string>())

const filterTabs = computed(() => {
  const total = props.history.length
  const failed = props.history.filter(h => !h.success).length
  const success = total - failed
  return [
    { label: '全部', value: 'all', count: total },
    { label: '成功', value: 'success', count: success },
    { label: '失败', value: 'failed', count: failed },
  ]
})

const filteredHistory = computed(() => {
  if (activeFilter.value === 'all') return props.history
  if (activeFilter.value === 'success') return props.history.filter(h => h.success)
  return props.history.filter(h => !h.success)
})

function toggleExpand(id: string) {
  const s = new Set(expandedItems.value)
  if (s.has(id)) s.delete(id)
  else s.add(id)
  expandedItems.value = s
}

const channelNames: Record<string, string> = {
  bark: 'Bark', dingtalk: '钉钉', feishu: '飞书', feishuApp: '飞书应用',
  qywxBot: '企微机器人', qywxApp: '企微应用', qywxSmart: '企微智能',
  telegram: 'Telegram', qqbot: 'QQ机器人', serverChan: 'Server酱',
  pushplus: 'PushPlus', ntfy: 'Ntfy', gotify: 'Gotify',
  pushdeer: 'PushDeer', webhook: 'Webhook', igot: 'iGot',
  synology: '群晖', qmsg: 'QMsg', pushme: 'PushMe',
  wxpusher: 'WxPusher', aibotk: 'AIBotK', weplus: 'WePlus',
  wechatClaw: '微信Claw',
}

const channelIcons: Record<string, string> = {
  bark: '📲', dingtalk: '🐜', feishu: '🐦', feishuApp: '🐦',
  qywxBot: '💼', qywxApp: '💼', qywxSmart: '💼',
  telegram: '✈️', qqbot: '💬', serverChan: '🔔',
  pushplus: '📣', ntfy: '📡', gotify: '📡',
  pushdeer: '🦌', webhook: '🔗', igot: '📱',
  synology: '🖥️', qmsg: '📧', pushme: '📨',
  wxpusher: '🔊', aibotk: '🤖', weplus: '➕',
  wechatClaw: '💚',
}

const channelColors: Record<string, { bg: string; fg: string }> = {
  bark: { bg: 'rgba(74, 144, 226, 0.1)', fg: '#4A90E2' },
  dingtalk: { bg: 'rgba(0, 150, 255, 0.1)', fg: '#0096FF' },
  feishu: { bg: 'rgba(51, 126, 255, 0.1)', fg: '#337EFF' },
  feishuApp: { bg: 'rgba(51, 126, 255, 0.1)', fg: '#337EFF' },
  qywxBot: { bg: 'rgba(7, 193, 96, 0.1)', fg: '#07C160' },
  qywxApp: { bg: 'rgba(7, 193, 96, 0.1)', fg: '#07C160' },
  qywxSmart: { bg: 'rgba(7, 193, 96, 0.1)', fg: '#07C160' },
  telegram: { bg: 'rgba(0, 136, 204, 0.1)', fg: '#0088CC' },
  qqbot: { bg: 'rgba(18, 183, 245, 0.1)', fg: '#12B7F5' },
  serverChan: { bg: 'rgba(232, 64, 38, 0.1)', fg: '#E84026' },
  pushplus: { bg: 'rgba(255, 107, 53, 0.1)', fg: '#FF6B35' },
  wechatClaw: { bg: 'rgba(7, 193, 96, 0.1)', fg: '#07C160' },
}

function getChannelName(channel: string): string {
  return channelNames[channel] || channel
}

function relativeTime(timestamp: string): string {
  try {
    const now = Date.now()
    const t = new Date(timestamp).getTime()
    if (isNaN(t)) return timestamp
    const diff = now - t

    if (diff < 0) {
      // Future time or clock skew - show absolute
      const d = new Date(t)
      return `${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`
    }

    const seconds = Math.floor(diff / 1000)
    if (seconds < 60) return '刚刚'
    const minutes = Math.floor(seconds / 60)
    if (minutes < 60) return `${minutes}分钟前`
    const hours = Math.floor(minutes / 60)
    if (hours < 6) return `${hours}小时前`

    const d = new Date(t)
    const today = new Date()
    today.setHours(0, 0, 0, 0)
    const dStart = new Date(d)
    dStart.setHours(0, 0, 0, 0)
    const dayDiff = Math.round((today.getTime() - dStart.getTime()) / 86400000)

    if (dayDiff === 0) return `今天 ${pad(d.getHours())}:${pad(d.getMinutes())}`
    if (dayDiff === 1) return `昨天 ${pad(d.getHours())}:${pad(d.getMinutes())}`
    if (dayDiff < 7) return `${dayDiff}天前`
    return `${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`
  } catch {
    return timestamp
  }
}

function pad(n: number): string {
  return n < 10 ? '0' + n : '' + n
}
</script>

<style scoped>
.history-section {
  display: flex;
  flex-direction: column;
  gap: var(--spacing-md);
}

.section-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: var(--spacing-sm);
}

.section-header h4 {
  margin: 0;
  font-size: var(--font-size-lg, 15px);
  font-weight: 600;
  color: var(--text-color-1);
  display: flex;
  align-items: center;
  gap: 6px;
}

.header-icon {
  color: var(--primary-color);
  opacity: 0.8;
}

.header-actions {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
}

.filter-tabs {
  display: flex;
  background: var(--bg-color-2);
  border-radius: var(--radius-full, 9999px);
  padding: var(--spacing-xs);
  gap: 1px;
}

.filter-tab {
  position: relative;
  border: none;
  background: transparent;
  color: var(--text-color-3);
  font-size: var(--font-size-xs, 11px);
  padding: var(--spacing-xs) var(--spacing-md);
  border-radius: var(--radius-full, 9999px);
  cursor: pointer;
  transition: all var(--transition-base);
  display: flex;
  align-items: center;
  gap: var(--spacing-xs);
  white-space: nowrap;
}

.filter-tab:hover {
  color: var(--text-color-2);
}

.filter-tab.active {
  background: var(--bg-color-1);
  color: var(--primary-color);
  box-shadow: var(--shadow-xs);
  font-weight: 500;
}

.tab-count {
  font-size: 0.75em;
  opacity: 0.6;
  min-width: 14px;
  text-align: center;
}

.clear-btn {
  display: inline-flex;
  align-items: center;
  gap: var(--spacing-xs);
  border: 1px solid var(--border-color);
  background: var(--bg-color-1);
  color: var(--text-color-3);
  font-size: var(--font-size-xs, 11px);
  padding: var(--spacing-xs) var(--spacing-md);
  border-radius: var(--radius-full, 9999px);
  cursor: pointer;
  transition: all var(--transition-fast);
  white-space: nowrap;
}

.clear-btn:hover {
  border-color: var(--error-color);
  color: var(--error-color);
  background: var(--error-bg);
}

.history-list {
  display: flex;
  flex-direction: column;
  gap: var(--spacing-xs);
  min-height: 60px;
}

.history-card {
  background: var(--bg-color-1);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-sm, 12px);
  cursor: pointer;
  transition: all 0.25s cubic-bezier(0.4, 0, 0.2, 1);
  overflow: hidden;
}

.history-card:hover {
  border-color: transparent;
  box-shadow: var(--shadow-sm);
  background: var(--card-bg);
}

.history-card.is-failed {
  border-left: 3px solid var(--error-color);
}

.card-main {
  display: flex;
  align-items: flex-start;
  padding: var(--spacing-sm) var(--spacing-md);
  gap: var(--spacing-sm);
}

.card-left {
  flex-shrink: 0;
  padding-top: 2px;
}

.status-dot {
  width: 22px;
  height: 22px;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.status-dot.success {
  background: var(--success-bg);
  color: var(--success-color);
}

.status-dot.failed {
  background: var(--error-bg);
  color: var(--error-color);
}

.card-content {
  flex: 1;
  min-width: 0;
}

.card-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--spacing-sm);
  margin-bottom: 4px;
}

.channel-badge {
  display: inline-flex;
  align-items: center;
  gap: 3px;
  font-size: var(--font-size-2xs, 10px);
  font-weight: 500;
  padding: 1px var(--spacing-sm);
  border-radius: var(--radius-full, 9999px);
  white-space: nowrap;
  line-height: 1.6;
}

.channel-icon {
  font-size: var(--font-size-xs);
  line-height: 1;
}

.time {
  font-size: var(--font-size-2xs, 10px);
  color: var(--text-color-4);
  white-space: nowrap;
  flex-shrink: 0;
}

.card-title {
  font-size: var(--font-size-sm, 12px);
  font-weight: 500;
  color: var(--text-color-1);
  line-height: 1.4;
  margin-bottom: 2px;
}

.card-message {
  font-size: var(--font-size-2xs, 10px);
  color: var(--text-color-3);
  line-height: 1.5;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
  transition: all var(--transition-base);
  word-break: break-all;
}

.card-message.expanded {
  -webkit-line-clamp: unset;
  display: block;
}

.card-error {
  display: flex;
  align-items: flex-start;
  gap: var(--spacing-xs);
  margin-top: 4px;
  font-size: var(--font-size-2xs, 10px);
  color: var(--error-color);
  background: var(--error-bg);
  padding: var(--spacing-xs) var(--spacing-sm);
  border-radius: var(--radius-sm, 12px);
  line-height: 1.4;
  word-break: break-all;
}

.card-error svg {
  flex-shrink: 0;
  margin-top: 1px;
}

.card-chevron {
  flex-shrink: 0;
  color: var(--text-color-4);
  padding-top: 4px;
  transition: transform var(--transition-fast);
}

.card-chevron svg {
  transition: transform var(--transition-base);
}

.card-chevron svg.rotated {
  transform: rotate(180deg);
}

/* Transition animations */
.history-item-enter-active {
  transition: all 0.35s cubic-bezier(0.4, 0, 0.2, 1);
}

.history-item-leave-active {
  transition: all var(--transition-base);
}

.history-item-enter-from {
  opacity: 0;
  transform: translateY(-8px) scale(0.97);
}

.history-item-leave-to {
  opacity: 0;
  transform: translateX(20px);
}

.history-item-move {
  transition: transform var(--transition-base);
}

.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: var(--spacing-3xl) var(--spacing-xl);
  color: var(--text-color-4);
}

.empty-icon {
  opacity: 0.4;
  margin-bottom: var(--spacing-sm);
}

.empty-state p {
  margin: 0;
  font-size: var(--font-size-sm, 12px);
  color: var(--text-color-3);
}
</style>
