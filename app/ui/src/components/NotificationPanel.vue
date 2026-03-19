<template>
  <div class="modal-overlay" @click.self="$emit('close')">
    <div class="notification-panel">
      <div class="panel-header">
        <h3>通知设置</h3>
        <button class="close-btn" @click="$emit('close')">×</button>
      </div>

      <div class="panel-body">
        <!-- 全局设置 -->
        <div class="section">
          <div class="section-header">
            <h4>全局设置</h4>
            <label class="switch">
              <input type="checkbox" v-model="settings.enabled" @change="updateSettings">
              <span class="slider"></span>
            </label>
          </div>
          <div class="setting-row">
            <label>检查间隔</label>
            <select v-model="settings.checkInterval" @change="updateSettings" :disabled="!settings.enabled">
              <option :value="10000">10秒</option>
              <option :value="30000">30秒</option>
              <option :value="60000">1分钟</option>
              <option :value="300000">5分钟</option>
            </select>
          </div>
        </div>

        <div class="divider"></div>

        <!-- 监控状态 -->
        <div class="section">
          <div class="section-header">
            <h4>监控状态</h4>
            <span :class="['status-badge', monitorStatus.running ? 'running' : 'stopped']">
              {{ monitorStatus.running ? '运行中' : '已停止' }}
            </span>
          </div>
          <div class="status-info">
            <div class="info-item">
              <span class="label">监控文件数</span>
              <span class="value">{{ monitorStatus.watchedFiles }}</span>
            </div>
            <div class="info-item">
              <span class="label">活跃规则数</span>
              <span class="value">{{ monitorStatus.activeRules }}</span>
            </div>
          </div>
          <div class="btn-row">
            <button 
              class="control-btn" 
              @click="startMonitor" 
              :disabled="monitorStatus.running || !settings.enabled"
            >启动</button>
            <button 
              class="control-btn danger" 
              @click="stopMonitor" 
              :disabled="!monitorStatus.running"
            >停止</button>
            <button class="control-btn" @click="triggerCheck" :disabled="!settings.enabled">立即检查</button>
          </div>
        </div>

        <div class="divider"></div>

        <!-- 通知渠道 -->
        <div class="section">
          <div class="section-header">
            <h4>通知渠道</h4>
            <button class="add-btn" @click="showAddChannel = true">+ 添加渠道</button>
          </div>
          <div class="channel-list" v-if="channels.length > 0">
            <div class="channel-item" v-for="channel in channels" :key="channel.name">
              <div class="channel-info">
                <span class="channel-name">{{ channel.name }}</span>
                <span class="channel-type">{{ getChannelTypeName(channel.channel) }}</span>
              </div>
              <div class="channel-actions">
                <label class="switch small">
                  <input type="checkbox" v-model="channel.enabled" @change="updateChannel(channel)">
                  <span class="slider"></span>
                </label>
                <button class="edit-btn" @click="editChannel(channel)" title="编辑">编辑</button>
                <button class="test-btn" @click="testChannel(channel.name)" title="测试">测试</button>
                <button class="delete-btn" @click="confirmDeleteChannel(channel.name)" title="删除">×</button>
              </div>
            </div>
          </div>
          <div class="empty-hint" v-else>
            暂无通知渠道，请先添加
          </div>
        </div>

        <div class="divider"></div>

        <!-- 通知规则 -->
        <div class="section">
          <div class="section-header">
            <h4>通知规则</h4>
            <button class="add-btn" @click="showAddRule = true">+ 添加规则</button>
          </div>
          <div class="rule-list" v-if="rules.length > 0">
            <div class="rule-item" v-for="rule in rules" :key="rule.id">
              <div class="rule-header">
                <span class="rule-name">{{ rule.name }}</span>
                <span :class="['rule-status', rule.status]">
                  {{ rule.status === 'enabled' ? '已启用' : '已禁用' }}
                </span>
              </div>
              <div class="rule-info">
                <span class="info-tag">应用: {{ rule.appName }}</span>
                <span class="info-tag">级别: {{ rule.logLevel }}</span>
                <span class="info-tag">触发: {{ rule.triggerCount }}次</span>
              </div>
              <div class="rule-actions">
                <button class="toggle-btn" @click="toggleRule(rule.id)">
                  {{ rule.status === 'enabled' ? '禁用' : '启用' }}
                </button>
                <button class="edit-btn" @click="editRule(rule)">编辑</button>
                <button class="delete-btn" @click="confirmDeleteRule(rule.id)">删除</button>
              </div>
            </div>
          </div>
          <div class="empty-hint" v-else>
            暂无通知规则，请先添加
          </div>
        </div>

        <div class="divider"></div>

        <!-- 通知历史 -->
        <div class="section">
          <div class="section-header">
            <h4>通知历史</h4>
            <button class="clear-btn" @click="confirmClearHistory">清空</button>
          </div>
          <div class="history-list" v-if="history.length > 0">
            <div class="history-item" v-for="item in history" :key="item.id">
              <div class="history-header">
                <span :class="['history-status', item.success ? 'success' : 'failed']">
                  {{ item.success ? '成功' : '失败' }}
                </span>
                <span class="history-time">{{ formatTime(item.timestamp) }}</span>
              </div>
              <div class="history-content">
                <div class="history-row">
                  <span class="label">规则:</span>
                  <span class="value">{{ item.ruleName }}</span>
                </div>
                <div class="history-row">
                  <span class="label">应用:</span>
                  <span class="value">{{ item.appName }}</span>
                </div>
                <div class="history-row">
                  <span class="label">渠道:</span>
                  <span class="value">{{ getChannelTypeName(item.channel) }}</span>
                </div>
              </div>
            </div>
          </div>
          <div class="empty-hint" v-else>
            暂无通知历史
          </div>
        </div>
      </div>
    </div>

    <!-- 添加/编辑渠道弹窗 -->
    <div class="modal-overlay sub-modal" v-if="showAddChannel" @click.self="closeChannelModal">
      <div class="modal-content">
        <div class="modal-header">
          <h4>{{ editingChannel ? '编辑通知渠道' : '添加通知渠道' }}</h4>
          <button class="close-btn" @click="closeChannelModal">×</button>
        </div>
        <div class="modal-body">
          <div class="form-group">
            <label>渠道类型</label>
            <select v-model="newChannel.channel" :disabled="!!editingChannel">
              <option v-for="type in channelTypes" :key="type.type" :value="type.type">
                {{ type.name }}
              </option>
            </select>
          </div>
          <div class="form-group">
            <label>渠道名称</label>
            <input type="text" v-model="newChannel.name" placeholder="请输入渠道名称" :disabled="!!editingChannel">
          </div>
          <div class="form-group" v-for="field in currentChannelFields" :key="field">
            <label>
              {{ getFieldLabel(field) }}
              <a v-if="getFieldHelpUrl(field)" :href="getFieldHelpUrl(field)" target="_blank" class="help-link">?</a>
            </label>
            <input 
              :type="field.includes('password') || field.includes('secret') ? 'password' : 'text'" 
              v-model="newChannel.config[field]"
              :placeholder="getFieldPlaceholder(field)"
            >
            <div v-if="getFieldHint(field)" class="hint">{{ getFieldHint(field) }}</div>
          </div>
        </div>
        <div class="modal-footer">
          <button class="cancel-btn" @click="closeChannelModal">取消</button>
          <button class="submit-btn" @click="saveChannel">{{ editingChannel ? '保存' : '添加' }}</button>
        </div>
      </div>
    </div>

    <!-- 添加规则弹窗 -->
    <div class="modal-overlay sub-modal" v-if="showAddRule" @click.self="closeRuleModal">
      <div class="modal-content">
        <div class="modal-header">
          <h4>{{ editingRule ? '编辑规则' : '添加通知规则' }}</h4>
          <button class="close-btn" @click="closeRuleModal">×</button>
        </div>
        <div class="modal-body">
          <div class="form-group">
            <label>规则名称</label>
            <input type="text" v-model="newRule.name" placeholder="请输入规则名称">
          </div>
          <div class="form-group">
            <label>监控应用</label>
            <div class="dropdown-select" :class="{ open: showAppDropdown }">
              <div class="dropdown-trigger" @click="showAppDropdown = !showAppDropdown">
                <span class="dropdown-text">{{ selectedAppsText }}</span>
                <span class="dropdown-arrow">▼</span>
              </div>
              <div class="dropdown-menu" v-if="showAppDropdown">
                <label class="dropdown-item">
                  <input type="checkbox" :checked="isAllAppsSelected" @change="toggleAllApps">
                  <span>所有应用</span>
                </label>
                <label class="dropdown-item">
                  <input type="checkbox" :checked="selectedApps.includes('eventlogger')" @change="toggleAppSelect('eventlogger')">
                  <span>系统日志</span>
                </label>
                <div class="dropdown-divider" v-if="appNames.length > 0"></div>
                <label class="dropdown-item" v-for="app in appNames" :key="app">
                  <input type="checkbox" :checked="selectedApps.includes(app)" @change="toggleAppSelect(app)">
                  <span>{{ app }}</span>
                </label>
              </div>
            </div>
            <div class="hint" v-if="selectedApps.includes('eventlogger')">系统日志：监控飞牛系统事件（登录、硬盘、应用操作等）</div>
          </div>
          <div class="form-group">
            <label>日志级别</label>
            <select v-model="newRule.logLevel">
              <option value="error">错误 (Error)</option>
              <option value="warn">警告 (Warn)</option>
              <option value="info">信息 (Info)</option>
              <option value="debug">调试 (Debug)</option>
              <option value="all">全部 (All)</option>
            </select>
          </div>
          <div class="form-group">
            <label>关键词 (逗号分隔，任一匹配即触发)</label>
            <input type="text" v-model="keywordsInput" placeholder="如: error, failed, regex:\\d{3}">
            <div class="hint">支持正则: regex:pattern 或 /pattern/flags</div>
          </div>
          <div class="form-group">
            <label>排除关键词 (逗号分隔)</label>
            <input type="text" v-model="excludeKeywordsInput" placeholder="如: debug, test">
            <div class="hint">支持正则: regex:pattern 或 /pattern/flags</div>
          </div>
          <div class="form-group">
            <label>通知渠道</label>
            <div class="checkbox-group">
              <label class="checkbox" v-for="channel in channels" :key="channel.name">
                <input 
                  type="checkbox" 
                  :value="channel.name" 
                  :checked="newRule.channels.includes(channel.name)"
                  @change="toggleChannel(channel.name)"
                >
                <span>{{ channel.name }}</span>
              </label>
            </div>
            <div class="hint" v-if="channels.length === 0">请先添加通知渠道</div>
          </div>
          <div class="form-row">
            <div class="form-group">
              <label>冷却时间 (秒)</label>
              <input type="number" v-model.number="newRule.cooldown" min="10" max="3600">
            </div>
            <div class="form-group">
              <label>每小时最大通知数</label>
              <input type="number" v-model.number="newRule.maxNotifications" min="1" max="100">
            </div>
          </div>
          <div class="form-row">
            <div class="form-group">
              <label>免打扰开始时间</label>
              <input type="time" v-model="newRule.quietHoursStart">
            </div>
            <div class="form-group">
              <label>免打扰结束时间</label>
              <input type="time" v-model="newRule.quietHoursEnd">
            </div>
          </div>
        </div>
        <div class="modal-footer">
          <button class="cancel-btn" @click="closeRuleModal">取消</button>
          <button class="submit-btn" @click="saveRule">{{ editingRule ? '保存' : '添加' }}</button>
        </div>
      </div>
    </div>

    <!-- 提示弹窗 -->
    <AlertDialog 
      v-model="alertVisible" 
      :title="alertTitle"
      :message="alertMessage" 
      :type="alertType"
      :copyText="alertCopyText"
      @confirm="onAlertConfirm"
    />

    <!-- 确认弹窗 -->
    <AlertDialog 
      v-model="confirmVisible" 
      :title="confirmTitle"
      :message="confirmMessage" 
      type="confirm"
      @confirm="onConfirmOk"
      @cancel="onConfirmCancel"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import api, { eventLoggerApi } from '../services/api'
import AlertDialog from './AlertDialog.vue'

interface NotificationSettings {
  enabled: boolean
  checkInterval: number
  maxHistoryDays: number
  maxHistoryCount: number
}

interface ChannelConfig {
  channel: string
  name: string
  enabled: boolean
  [key: string]: unknown
}

interface NotificationRule {
  id: string
  name: string
  status: string
  appName: string
  sources?: string[]
  excludeSources?: string[]
  logLevel: string
  keywords?: string[]
  excludeKeywords?: string[]
  channels: string[]
  cooldown: number
  maxNotifications: number
  quietHoursStart?: string
  quietHoursEnd?: string
  triggerCount: number
}

interface HistoryItem {
  id: string
  ruleName: string
  channel: string
  appName: string
  success: boolean
  timestamp: string
}

interface MonitorStatus {
  running: boolean
  watchedFiles: number
  activeRules: number
}

interface ChannelType {
  type: string
  name: string
  fields: string[]
}

const emit = defineEmits<{
  close: []
}>()

const settings = ref<NotificationSettings>({
  enabled: false,
  checkInterval: 30000,
  maxHistoryDays: 7,
  maxHistoryCount: 1000
})

const monitorStatus = ref<MonitorStatus>({
  running: false,
  watchedFiles: 0,
  activeRules: 0
})

const channels = ref<ChannelConfig[]>([])
const rules = ref<NotificationRule[]>([])
const history = ref<HistoryItem[]>([])
const channelTypes = ref<ChannelType[]>([])

const showAddChannel = ref(false)
const showAddRule = ref(false)
const editingRule = ref<NotificationRule | null>(null)
const editingChannel = ref<ChannelConfig | null>(null)

// 定时刷新器
let refreshTimer: ReturnType<typeof setInterval> | null = null

// 弹窗状态
const alertVisible = ref(false)
const alertTitle = ref('')
const alertMessage = ref('')
const alertType = ref<'info' | 'success' | 'error' | 'warning'>('info')

const confirmVisible = ref(false)
const confirmTitle = ref('')
const confirmMessage = ref('')
const confirmAction = ref<(() => void) | null>(null)

const alertCopyText = ref('')

function showAlert(title: string, message: string, type: 'info' | 'success' | 'error' | 'warning' = 'info', copyText?: string) {
  alertTitle.value = title
  alertMessage.value = message
  alertType.value = type
  alertCopyText.value = copyText || ''
  alertVisible.value = true
}

function copyAlertText(): void {
  if (alertCopyText.value) {
    navigator.clipboard.writeText(alertCopyText.value).then(() => {
      // 复制成功，可以显示提示
    }).catch(() => {
      // 复制失败
    })
  }
}

function onAlertConfirm() {
  // 弹窗确认后的回调
}

function showConfirm(title: string, message: string, action: () => void) {
  confirmTitle.value = title
  confirmMessage.value = message
  confirmAction.value = action
  confirmVisible.value = true
}

function onConfirmOk() {
  if (confirmAction.value) {
    confirmAction.value()
  }
  confirmAction.value = null
}

function onConfirmCancel() {
  confirmAction.value = null
}

function toggleChannel(channelName: string) {
  // 确保 channels 是数组
  if (!Array.isArray(newRule.value.channels)) {
    newRule.value.channels = []
  }
  const index = newRule.value.channels.indexOf(channelName)
  if (index === -1) {
    newRule.value.channels.push(channelName)
  } else {
    newRule.value.channels.splice(index, 1)
  }
}

function toggleSource(source: string): void {
  if (!newRule.value.sources) newRule.value.sources = []
  const index = newRule.value.sources.indexOf(source)
  if (index > -1) {
    newRule.value.sources.splice(index, 1)
  } else {
    newRule.value.sources.push(source)
  }
}

function toggleExcludeSource(source: string): void {
  if (!newRule.value.excludeSources) newRule.value.excludeSources = []
  const index = newRule.value.excludeSources.indexOf(source)
  if (index > -1) {
    newRule.value.excludeSources.splice(index, 1)
  } else {
    newRule.value.excludeSources.push(source)
  }
}

function onAppNameChange(): void {
  // 切换应用名称时清空 sources
  newRule.value.sources = []
  newRule.value.excludeSources = []
  // 如果选择了系统日志，加载来源列表
  if (newRule.value.appName === 'eventlogger' && eventSources.value.length === 0) {
    loadEventSources()
  }
}

async function loadEventSources(): Promise<void> {
  try {
    const sources = await eventLoggerApi.getSources()
    eventSources.value = sources
  } catch (e) {
    console.error('Failed to load event sources:', e)
  }
}

const newChannel = ref<{
  channel: string
  name: string
  config: Record<string, string>
}>({
  channel: 'dingtalk',
  name: '',
  config: {}
})

const newRule = ref<{
  name: string
  appName: string
  sources?: string[]
  excludeSources?: string[]
  logLevel: string
  channels: string[]
  cooldown: number
  maxNotifications: number
  quietHoursStart: string
  quietHoursEnd: string
}>({
  name: '',
  appName: '*',
  sources: [],
  excludeSources: [],
  logLevel: 'error',
  channels: [],
  cooldown: 60,
  maxNotifications: 10,
  quietHoursStart: '',
  quietHoursEnd: ''
})

const eventSources = ref<string[]>([])
const appNames = ref<string[]>([])
const selectedApps = ref<string[]>([])
const showAppDropdown = ref(false)

const selectedAppsText = computed(() => {
  if (selectedApps.value.length === 0) {
    return '请选择应用'
  }
  if (isAllAppsSelected.value) {
    return '所有应用'
  }
  if (selectedApps.value.length === 1) {
    return selectedApps.value[0] === 'eventlogger' ? '系统日志' : selectedApps.value[0]
  }
  return `已选择 ${selectedApps.value.length} 个应用`
})

const isAllAppsSelected = computed(() => {
  const allApps = ['eventlogger', ...appNames.value]
  return allApps.every(app => selectedApps.value.includes(app))
})

function toggleAllApps(): void {
  const allApps = ['eventlogger', ...appNames.value]
  if (isAllAppsSelected.value) {
    // 取消全选
    selectedApps.value = []
  } else {
    // 全选
    selectedApps.value = [...allApps]
  }
}

function toggleAppSelect(app: string): void {
  const index = selectedApps.value.indexOf(app)
  if (index > -1) {
    selectedApps.value.splice(index, 1)
  } else {
    selectedApps.value.push(app)
  }
}

async function loadAppNames(): Promise<void> {
  try {
    const names = await eventLoggerApi.getAppNames()
    appNames.value = names
  } catch (e) {
    console.error('Failed to load app names:', e)
  }
}

const keywordsInput = ref('')
const excludeKeywordsInput = ref('')

const currentChannelFields = computed(() => {
  const type = channelTypes.value.find(t => t.type === newChannel.value.channel)
  return type?.fields || []
})

function getChannelTypeName(channel: string): string {
  const type = channelTypes.value.find(t => t.type === channel)
  return type?.name || channel
}

function getFieldLabel(field: string): string {
  const labels: Record<string, string> = {
    barkPush: 'Bark推送地址',
    barkSound: '提示音',
    barkGroup: '分组',
    ddBotToken: '钉钉机器人Token',
    ddBotSecret: '钉钉机器人Secret',
    fsKey: 'Webhook Key',
    fsSecret: '签名密钥(可选)',
    feishuAppId: 'App ID',
    feishuAppSecret: 'App Secret',
    feishuUserId: '用户ID',
    qywxKey: '企业微信机器人Key',
    qywxOrigin: '企业微信代理地址',
    qywxAm: '企业微信应用配置',
    wechatBotId: '机器人ID',
    wechatBotSecret: '机器人Secret',
    wechatBotChatId: '发送目标',
    wechatBotWsUrl: 'WebSocket地址',
    tgBotToken: 'Telegram Bot Token',
    tgUserId: 'Telegram User ID',
    pushKey: 'Server酱PushKey',
    pushPlusToken: 'PushPlus Token',
    pushPlusUser: 'PushPlus 用户/群组',
    webhookUrl: 'Webhook URL',
    webhookMethod: '请求方法',
    ntfyUrl: 'Ntfy服务器地址',
    ntfyTopic: 'Ntfy Topic',
    ntfyToken: 'Ntfy Token',
    gotifyUrl: 'Gotify服务器地址',
    gotifyToken: 'Gotify Token',
    deerKey: 'PushDeer Key',
    qqAppId: 'QQ机器人AppID',
    qqAppSecret: 'QQ机器人Secret',
    qqOpenId: 'QQ用户OpenID',
    qqGroupOpenId: 'QQ群OpenID'
  }
  return labels[field] || field
}

function getFieldPlaceholder(field: string): string {
  const placeholders: Record<string, string> = {
    barkPush: '如: https://api.day.app/xxx',
    ddBotToken: '钉钉机器人的access_token',
    fsKey: '飞书群机器人的Webhook Key',
    fsSecret: '签名密钥，用于验证消息来源',
    feishuAppId: '飞书企业自建应用的App ID',
    feishuAppSecret: '飞书企业自建应用的App Secret',
    feishuUserId: '接收消息的用户union_id（跨应用通用）',
    qywxKey: '企业微信机器人的key',
    wechatBotId: '企业微信智能机器人ID',
    wechatBotSecret: '企业微信智能机器人Secret',
    wechatBotChatId: 'user:xxx 或 group:xxx',
    wechatBotWsUrl: '默认: wss://openws.work.weixin.qq.com',
    tgBotToken: '如: 123456:ABC-DEF',
    pushKey: 'Server酱的SCKEY',
    pushPlusToken: 'PushPlus的token',
    webhookUrl: 'https://example.com/webhook',
    ntfyTopic: '订阅主题名称',
    gotifyUrl: '如: https://gotify.example.com',
    deerKey: 'PushDeer的pushkey'
  }
  return placeholders[field] || ''
}

function getFieldHelpUrl(field: string): string | undefined {
  const helpUrls: Record<string, string> = {
    feishuUserId: 'https://open.feishu.cn/document/platform-overveiw/basic-concepts/user-identity-introduction/open-id'
  }
  return helpUrls[field]
}

function getFieldHint(field: string): string | undefined {
  const hints: Record<string, string> = {
    qqOpenId: '留空，保存后点击测试，给机器人发消息可自动获取',
    qqGroupOpenId: '留空，保存后点击测试，给机器人发消息可自动获取'
  }
  return hints[field]
}

function formatTime(timestamp: string | Date): string {
  if (!timestamp) return '-'
  const date = new Date(timestamp)
  // 确保正确处理UTC时间，转换为本地时间
  return date.toLocaleString('zh-CN', {
    timeZone: Intl.DateTimeFormat().resolvedOptions().timeZone,
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hour12: false
  })
}

async function loadSettings(): Promise<void> {
  try {
    const data = await api.get('/api/notifications/settings') as { settings: NotificationSettings }
    settings.value = data.settings
  } catch (e) {
    console.error('加载设置失败:', e)
  }
}

async function loadMonitorStatus(): Promise<void> {
  try {
    const data = await api.get('/api/notifications/monitor/status') as { status: MonitorStatus }
    monitorStatus.value = data.status
  } catch (e) {
    console.error('加载监控状态失败:', e)
  }
}

async function loadChannels(): Promise<void> {
  try {
    const data = await api.get('/api/notifications/channels') as { channels: ChannelConfig[] }
    channels.value = data.channels
  } catch (e) {
    console.error('加载渠道失败:', e)
  }
}

async function loadChannelTypes(): Promise<void> {
  try {
    const data = await api.get('/api/notifications/channels/types') as { types: ChannelType[] }
    channelTypes.value = data.types
  } catch (e) {
    console.error('加载渠道类型失败:', e)
  }
}

async function loadRules(): Promise<void> {
  try {
    const data = await api.get('/api/notifications/rules') as { rules: NotificationRule[] }
    rules.value = data.rules
  } catch (e) {
    console.error('加载规则失败:', e)
  }
}

async function loadHistory(): Promise<void> {
  try {
    const data = await api.get('/api/notifications/history?limit=20') as { history: HistoryItem[] }
    history.value = data.history
  } catch (e) {
    console.error('加载历史失败:', e)
  }
}

async function updateSettings(): Promise<void> {
  try {
    await api.post('/api/notifications/settings', settings.value)
    await loadMonitorStatus()
  } catch (e) {
    console.error('更新设置失败:', e)
  }
}

async function startMonitor(): Promise<void> {
  try {
    await api.post('/api/notifications/monitor/start')
    await loadMonitorStatus()
    showAlert('成功', '监控已启动', 'success')
  } catch (e) {
    console.error('启动监控失败:', e)
    showAlert('错误', '启动监控失败', 'error')
  }
}

async function stopMonitor(): Promise<void> {
  try {
    await api.post('/api/notifications/monitor/stop')
    await loadMonitorStatus()
    showAlert('成功', '监控已停止', 'success')
  } catch (e) {
    console.error('停止监控失败:', e)
    showAlert('错误', '停止监控失败', 'error')
  }
}

async function triggerCheck(): Promise<void> {
  try {
    await api.post('/api/notifications/monitor/check')
    await loadMonitorStatus()
    showAlert('成功', '已触发检查', 'success')
  } catch (e) {
    console.error('触发检查失败:', e)
    showAlert('错误', '触发检查失败', 'error')
  }
}

function closeChannelModal(): void {
  showAddChannel.value = false
  editingChannel.value = null
  newChannel.value = { channel: 'dingtalk', name: '', config: {} }
}

function editChannel(channel: ChannelConfig): void {
  editingChannel.value = channel
  newChannel.value = {
    channel: channel.channel,
    name: channel.name,
    config: { ...channel } as Record<string, string>
  }
  // 移除非配置字段
  delete newChannel.value.config.channel
  delete newChannel.value.config.name
  delete newChannel.value.config.enabled
  showAddChannel.value = true
}

async function saveChannel(): Promise<void> {
  if (!newChannel.value.name) {
    showAlert('提示', '请输入渠道名称', 'warning')
    return
  }

  try {
    if (editingChannel.value) {
      // 编辑模式
      await api.put(`/api/notifications/channels/${editingChannel.value.name}`, {
        ...newChannel.value.config,
        enabled: editingChannel.value.enabled
      })
      showAlert('成功', '渠道已更新', 'success')
    } else {
      // 添加模式
      await api.post('/api/notifications/channels', {
        channel: newChannel.value.channel,
        name: newChannel.value.name,
        config: newChannel.value.config
      })
      showAlert('成功', '渠道添加成功', 'success')
    }
    closeChannelModal()
    await loadChannels()
  } catch (e) {
    console.error('保存渠道失败:', e)
    showAlert('错误', editingChannel.value ? '更新渠道失败' : '添加渠道失败', 'error')
  }
}

async function updateChannel(channel: ChannelConfig): Promise<void> {
  try {
    await api.put(`/api/notifications/channels/${channel.name}`, channel)
  } catch (e) {
    console.error('更新渠道失败:', e)
  }
}

function confirmDeleteChannel(name: string): void {
  showConfirm('确认删除', `确定删除渠道 "${name}" 吗？`, async () => {
    try {
      await api.delete(`/api/notifications/channels/${name}`)
      await loadChannels()
      showAlert('成功', '渠道已删除', 'success')
    } catch (e) {
      console.error('删除渠道失败:', e)
      showAlert('错误', '删除渠道失败', 'error')
    }
  })
}

async function testChannel(name: string): Promise<void> {
  try {
    // 先检查是否是 QQ 机器人且没有配置 openid
    const channel = channels.value.find(c => c.name === name)
    if (channel && channel.channel === 'qqbot') {
      const hasOpenId = channel.qqOpenId || newChannel.value.config?.qqOpenId
      const hasGroupOpenId = channel.qqGroupOpenId || newChannel.value.config?.qqGroupOpenId
      if (!hasOpenId && !hasGroupOpenId) {
        showAlert('提示', '未配置 OpenID，测试将启动监听模式。\n\n请在 60 秒内给 QQ 机器人发送消息，系统会自动获取 OpenID。', 'warning')
      }
    }

    const data = await api.post(`/api/notifications/channels/${name}/test`) as {
      result: {
        success: boolean;
        error?: string;
        openid?: string;
        groupOpenid?: string;
      }
    }

    if (data.result.success) {
      let message = '测试通知发送成功'
      let copyText = ''

      // 如果返回了 openid，显示并自动填入
      if (data.result.openid) {
        message += `\n\n获取到用户 OpenID: ${data.result.openid}`
        copyText = data.result.openid
        // 如果正在编辑该渠道，自动填入
        if (editingChannel.value && editingChannel.value.name === name) {
          newChannel.value.config.qqOpenId = data.result.openid
        }
      }
      if (data.result.groupOpenid) {
        message += `\n获取到群 OpenID: ${data.result.groupOpenid}`
        copyText = data.result.groupOpenid
        if (editingChannel.value && editingChannel.value.name === name) {
          newChannel.value.config.qqGroupOpenId = data.result.groupOpenid
        }
      }

      showAlert('成功', message, 'success', copyText)
    } else {
      // 检查是否是获取 openid 的情况
      if (data.result.openid || data.result.groupOpenid) {
        let message = '已获取到 OpenID：'
        let copyText = ''
        if (data.result.openid) {
          message += `\n用户 OpenID: ${data.result.openid}`
          copyText = data.result.openid
          if (editingChannel.value && editingChannel.value.name === name) {
            newChannel.value.config.qqOpenId = data.result.openid
          }
        }
        if (data.result.groupOpenid) {
          message += `\n群 OpenID: ${data.result.groupOpenid}`
          copyText = data.result.groupOpenid
          if (editingChannel.value && editingChannel.value.name === name) {
            newChannel.value.config.qqGroupOpenId = data.result.groupOpenid
          }
        }
        showAlert('获取成功', message, 'success', copyText)
      } else {
        // 检查是否是 QQ 机器人监听超时的情况
        const errorMsg = data.result.error || ''
        if (channel && channel.channel === 'qqbot' && errorMsg.includes('60秒')) {
          showAlert('提示', '监听超时，未获取到 OpenID。\n\n请确保在 60 秒内给 QQ 机器人发送了消息。', 'warning')
        } else {
          showAlert('失败', '测试通知发送失败: ' + (errorMsg || '未知错误'), 'error')
        }
      }
    }
  } catch (e) {
    console.error('测试渠道失败:', e)
    showAlert('错误', '测试渠道失败', 'error')
  }
}

function closeRuleModal(): void {
  showAddRule.value = false
  editingRule.value = null
  newRule.value = {
    name: '',
    appName: '*',
    sources: [],
    excludeSources: [],
    logLevel: 'error',
    channels: [],
    cooldown: 60,
    maxNotifications: 10,
    quietHoursStart: '',
    quietHoursEnd: ''
  }
  selectedApps.value = []
  keywordsInput.value = ''
  excludeKeywordsInput.value = ''
}

function editRule(rule: NotificationRule): void {
  editingRule.value = rule
  // 确保 channels 是数组
  const ruleChannels = Array.isArray(rule.channels) ? rule.channels : []
  newRule.value = {
    name: rule.name,
    appName: rule.appName,
    sources: rule.sources || [],
    excludeSources: rule.excludeSources || [],
    logLevel: rule.logLevel,
    channels: [...ruleChannels],
    cooldown: rule.cooldown,
    maxNotifications: rule.maxNotifications,
    quietHoursStart: rule.quietHoursStart || '',
    quietHoursEnd: rule.quietHoursEnd || ''
  }
  // 解析 appName 到 selectedApps
  if (rule.appName === '*') {
    selectedApps.value = ['eventlogger', ...appNames.value]
  } else if (rule.appName.includes(',')) {
    selectedApps.value = rule.appName.split(',')
  } else {
    selectedApps.value = [rule.appName]
  }
  keywordsInput.value = (rule.keywords || []).join(', ')
  excludeKeywordsInput.value = (rule.excludeKeywords || []).join(', ')
  showAddRule.value = true
}

async function saveRule(): Promise<void> {
  if (!newRule.value.name) {
    showAlert('提示', '请输入规则名称', 'warning')
    return
  }

  // 确保 channels 是数组
  const selectedChannels = Array.isArray(newRule.value.channels) ? newRule.value.channels : []
  
  if (selectedChannels.length === 0) {
    showAlert('提示', '请选择至少一个通知渠道', 'warning')
    return
  }

  // 根据选中的应用生成 appName
  let appName = '*'
  if (selectedApps.value.length === 0) {
    showAlert('提示', '请选择至少一个应用', 'warning')
    return
  }
  if (selectedApps.value.length === 1) {
    appName = selectedApps.value[0]
  } else if (!isAllAppsSelected.value) {
    // 多选但不是全选，用逗号分隔
    appName = selectedApps.value.join(',')
  }
  
  const ruleData = {
    ...newRule.value,
    appName,
    selectedApps: selectedApps.value,
    channels: selectedChannels,
    keywords: keywordsInput.value.split(',').map(k => k.trim()).filter(Boolean),
    excludeKeywords: excludeKeywordsInput.value.split(',').map(k => k.trim()).filter(Boolean)
  }

  try {
    if (editingRule.value) {
      await api.put(`/api/notifications/rules/${editingRule.value.id}`, ruleData)
      showAlert('成功', '规则已更新', 'success')
    } else {
      await api.post('/api/notifications/rules', ruleData)
      showAlert('成功', '规则添加成功', 'success')
    }
    closeRuleModal()
    await loadRules()
  } catch (e) {
    console.error('保存规则失败:', e)
    const errorMsg = e instanceof Error ? e.message : '保存规则失败'
    showAlert('错误', errorMsg, 'error')
  }
}

async function toggleRule(id: string): Promise<void> {
  try {
    await api.post(`/api/notifications/rules/${id}/toggle`)
    await loadRules()
  } catch (e) {
    console.error('切换规则状态失败:', e)
    showAlert('错误', '切换规则状态失败', 'error')
  }
}

function confirmDeleteRule(id: string): void {
  showConfirm('确认删除', '确定删除此规则吗？', async () => {
    try {
      await api.delete(`/api/notifications/rules/${id}`)
      await loadRules()
      showAlert('成功', '规则已删除', 'success')
    } catch (e) {
      console.error('删除规则失败:', e)
      showAlert('错误', '删除规则失败', 'error')
    }
  })
}

function confirmClearHistory(): void {
  showConfirm('确认清空', '确定清空通知历史吗？', async () => {
    try {
      await api.post('/api/notifications/history/clean')
      await loadHistory()
      showAlert('成功', '历史记录已清空', 'success')
    } catch (e) {
      console.error('清空历史失败:', e)
      showAlert('错误', '清空历史失败', 'error')
    }
  })
}

onMounted(async () => {
  await Promise.all([
    loadSettings(),
    loadMonitorStatus(),
    loadChannels(),
    loadChannelTypes(),
    loadRules(),
    loadHistory(),
    loadAppNames()
  ])
  
  // 定期刷新监控状态、规则和历史记录
  refreshTimer = setInterval(() => {
    loadMonitorStatus()
    loadRules()
    loadHistory()
  }, 5000) // 每5秒刷新一次
})

onUnmounted(() => {
  if (refreshTimer) {
    clearInterval(refreshTimer)
    refreshTimer = null
  }
})
</script>

<style scoped>
.modal-overlay {
  position: fixed;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  background: rgba(0, 0, 0, 0.5);
  display: flex;
  justify-content: center;
  align-items: center;
  z-index: 1200;
  padding: var(--spacing-xl);
}

.sub-modal {
  z-index: 1300;
}

.notification-panel {
  background: var(--card-bg);
  border-radius: var(--radius-md);
  box-shadow: var(--shadow-xl);
  max-width: 600px;
  width: 100%;
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
  font-size: 1rem;
  font-weight: 500;
  color: var(--text-color-1);
}

.close-btn {
  background: none;
  border: none;
  font-size: 1.5rem;
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

.section {
  margin-bottom: var(--spacing-lg);
}

.section-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: var(--spacing-md);
}

.section-header h4 {
  margin: 0;
  font-size: 0.9375rem;
  font-weight: 500;
  color: var(--text-color-1);
}

.divider {
  height: 1px;
  background: var(--divider-color);
  margin: var(--spacing-lg) 0;
}

.switch {
  position: relative;
  display: inline-block;
  width: 44px;
  height: 24px;
}

.switch.small {
  width: 36px;
  height: 20px;
}

.switch input {
  opacity: 0;
  width: 0;
  height: 0;
}

.slider {
  position: absolute;
  cursor: pointer;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background-color: var(--bg-color-3);
  transition: 0.3s;
  border-radius: 24px;
}

.slider:before {
  position: absolute;
  content: "";
  height: 18px;
  width: 18px;
  left: 3px;
  bottom: 3px;
  background-color: white;
  transition: 0.3s;
  border-radius: 50%;
}

.switch.small .slider:before {
  height: 14px;
  width: 14px;
}

input:checked + .slider {
  background-color: var(--primary-color);
}

input:checked + .slider:before {
  transform: translateX(20px);
}

.switch.small input:checked + .slider:before {
  transform: translateX(16px);
}

.setting-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: var(--spacing-sm);
}

.setting-row label {
  font-size: 0.875rem;
  color: var(--text-color-2);
}

.setting-row select {
  padding: var(--spacing-xs) var(--spacing-sm);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-xs);
  background: var(--bg-color-2);
  color: var(--text-color-1);
  font-size: 0.875rem;
}

.status-badge {
  padding: 2px 8px;
  border-radius: 10px;
  font-size: 0.75rem;
  font-weight: 500;
}

.status-badge.running {
  background: var(--success-color);
  color: white;
}

.status-badge.stopped {
  background: var(--bg-color-3);
  color: var(--text-color-2);
}

.status-info {
  display: flex;
  gap: var(--spacing-lg);
  margin-bottom: var(--spacing-md);
}

.info-item {
  display: flex;
  flex-direction: column;
}

.info-item .label {
  font-size: 0.75rem;
  color: var(--text-color-3);
}

.info-item .value {
  font-size: 1.25rem;
  font-weight: 600;
  color: var(--text-color-1);
}

.btn-row {
  display: flex;
  gap: var(--spacing-sm);
}

.control-btn {
  flex: 1;
  padding: var(--spacing-sm);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-sm);
  background: var(--bg-color-2);
  color: var(--text-color-1);
  font-size: 0.8125rem;
  cursor: pointer;
  transition: all var(--transition-fast);
}

.control-btn:hover:not(:disabled) {
  background: var(--primary-color);
  color: white;
  border-color: var(--primary-color);
}

.control-btn.danger:hover:not(:disabled) {
  background: var(--error-color);
  border-color: var(--error-color);
}

.control-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.add-btn {
  padding: 4px 12px;
  border: 1px solid var(--primary-color);
  border-radius: var(--radius-sm);
  background: transparent;
  color: var(--primary-color);
  font-size: 0.8125rem;
  cursor: pointer;
  transition: all var(--transition-fast);
}

.add-btn:hover {
  background: var(--primary-color);
  color: white;
}

.channel-list, .rule-list, .history-list {
  display: flex;
  flex-direction: column;
  gap: var(--spacing-sm);
}

.channel-item, .rule-item, .history-item {
  padding: var(--spacing-md);
  background: var(--bg-color-2);
  border-radius: var(--radius-sm);
}

.channel-info, .rule-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: var(--spacing-xs);
}

.channel-name, .rule-name {
  font-weight: 500;
  color: var(--text-color-1);
}

.channel-type {
  font-size: 0.75rem;
  color: var(--text-color-3);
}

.channel-actions, .rule-actions {
  display: flex;
  gap: var(--spacing-sm);
  align-items: center;
}

.test-btn, .edit-btn, .toggle-btn {
  padding: 4px 8px;
  border: 1px solid var(--border-color);
  border-radius: var(--radius-xs);
  background: var(--bg-color-2);
  color: var(--text-color-1);
  font-size: 0.75rem;
  cursor: pointer;
}

.test-btn:hover, .edit-btn:hover, .toggle-btn:hover {
  background: var(--primary-color);
  color: white;
  border-color: var(--primary-color);
}

.delete-btn {
  padding: 4px 8px;
  border: none;
  border-radius: var(--radius-xs);
  background: transparent;
  color: var(--text-color-3);
  font-size: 1rem;
  cursor: pointer;
}

.delete-btn:hover {
  color: var(--error-color);
}

.rule-status {
  padding: 2px 6px;
  border-radius: 4px;
  font-size: 0.6875rem;
}

.rule-status.enabled {
  background: var(--success-color);
  color: white;
}

.rule-status.disabled {
  background: var(--bg-color-3);
  color: var(--text-color-2);
}

.rule-info {
  display: flex;
  flex-wrap: wrap;
  gap: var(--spacing-xs);
  margin-bottom: var(--spacing-sm);
}

.info-tag {
  padding: 2px 6px;
  background: var(--bg-color-3);
  border-radius: 4px;
  font-size: 0.6875rem;
  color: var(--text-color-2);
}

.history-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: var(--spacing-xs);
}

.history-status {
  padding: 2px 6px;
  border-radius: 4px;
  font-size: 0.6875rem;
}

.history-status.success {
  background: var(--success-color);
  color: white;
}

.history-status.failed {
  background: var(--error-color);
  color: white;
}

.history-time {
  font-size: 0.75rem;
  color: var(--text-color-3);
}

.history-content {
  font-size: 0.8125rem;
}

.history-row {
  display: flex;
  gap: var(--spacing-sm);
}

.history-row .label {
  color: var(--text-color-3);
  min-width: 40px;
}

.history-row .value {
  color: var(--text-color-1);
}

.empty-hint {
  text-align: center;
  padding: var(--spacing-lg);
  color: var(--text-color-3);
  font-size: 0.875rem;
}

.clear-btn {
  padding: 4px 12px;
  border: 1px solid var(--border-color);
  border-radius: var(--radius-sm);
  background: transparent;
  color: var(--text-color-2);
  font-size: 0.8125rem;
  cursor: pointer;
}

.clear-btn:hover {
  background: var(--error-color);
  color: white;
  border-color: var(--error-color);
}

/* Modal styles */
.modal-content {
  background: var(--card-bg);
  border-radius: var(--radius-md);
  width: 90%;
  max-width: 500px;
  max-height: 90vh;
  display: flex;
  flex-direction: column;
}

.modal-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: var(--spacing-md) var(--spacing-lg);
  border-bottom: 1px solid var(--border-color);
}

.modal-header h4 {
  margin: 0;
  font-size: 1rem;
  color: var(--text-color-1);
}

.modal-body {
  padding: var(--spacing-lg);
  overflow-y: auto;
  flex: 1;
}

.modal-footer {
  display: flex;
  justify-content: flex-end;
  gap: var(--spacing-sm);
  padding: var(--spacing-md) var(--spacing-lg);
  border-top: 1px solid var(--border-color);
}

.form-group {
  margin-bottom: var(--spacing-md);
}

.form-group label {
  display: block;
  margin-bottom: var(--spacing-xs);
  font-size: 0.875rem;
  color: var(--text-color-1);
}

.form-group input,
.form-group select {
  width: 100%;
  padding: var(--spacing-sm);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-xs);
  background: var(--bg-color-2);
  color: var(--text-color-1);
  font-size: 0.875rem;
  box-sizing: border-box;
}

/* 不影响checkbox和radio的宽度 */
.form-group input[type="checkbox"],
.form-group input[type="radio"] {
  width: auto;
  padding: 0;
  border: none;
  background: transparent;
}

.form-group input:focus,
.form-group select:focus {
  outline: none;
  border-color: var(--primary-color);
}

.form-row {
  display: flex;
  gap: var(--spacing-md);
}

.form-row .form-group {
  flex: 1;
}

.checkbox-group {
  display: flex;
  flex-wrap: wrap;
  gap: var(--spacing-sm);
}

.sources-group {
  max-height: 200px;
  overflow-y: auto;
  padding: var(--spacing-sm);
  background: var(--bg-color-2);
  border-radius: var(--radius-xs);
  border: 1px solid var(--border-color);
}

.app-select-group {
  max-height: 300px;
  overflow-y: auto;
  padding: var(--spacing-sm);
  background: var(--bg-color-2);
  border-radius: var(--radius-xs);
  border: 1px solid var(--border-color);
}

.dropdown-select {
  position: relative;
}

.dropdown-trigger {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: var(--spacing-sm) var(--spacing-md);
  background: var(--bg-color-2);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-xs);
  cursor: pointer;
  font-size: 0.875rem;
  color: var(--text-color-1);
}

.dropdown-trigger:hover {
  border-color: var(--primary-color);
}

.dropdown-text {
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.dropdown-arrow {
  font-size: 0.75rem;
  color: var(--text-color-3);
  margin-left: var(--spacing-sm);
  transition: transform 0.2s;
}

.dropdown-select.open .dropdown-arrow {
  transform: rotate(180deg);
}

.dropdown-menu {
  position: absolute;
  top: 100%;
  left: 0;
  right: 0;
  margin-top: 4px;
  background: var(--card-bg);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-xs);
  box-shadow: var(--shadow-lg);
  max-height: 300px;
  overflow-y: auto;
  z-index: 100;
}

.dropdown-item {
  display: flex;
  align-items: center;
  padding: var(--spacing-sm) var(--spacing-md);
  cursor: pointer;
  font-size: 0.875rem;
  color: var(--text-color-1);
  transition: background 0.2s;
}

.dropdown-item:hover {
  background: var(--bg-color-2);
}

.dropdown-item input[type="checkbox"] {
  margin-right: var(--spacing-sm);
  flex-shrink: 0;
  cursor: pointer;
}

.dropdown-item span {
  flex: 1;
}

.dropdown-divider {
  height: 1px;
  background: var(--border-color);
  margin: var(--spacing-xs) 0;
}

.checkbox {
  display: flex;
  align-items: center;
  gap: var(--spacing-xs);
  cursor: pointer;
  font-size: 0.875rem;
  color: var(--text-color-1);
}

.checkbox input {
  width: auto;
}

.hint {
  font-size: 0.75rem;
  color: var(--text-color-3);
}

.help-link {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 16px;
  height: 16px;
  margin-left: 6px;
  font-size: 12px;
  font-weight: bold;
  color: var(--primary-color);
  background: var(--bg-color-2);
  border: 1px solid var(--primary-color);
  border-radius: 50%;
  text-decoration: none;
  cursor: pointer;
  transition: all 0.2s;
}

.help-link:hover {
  background: var(--primary-color);
  color: white;
}

.cancel-btn {
  padding: var(--spacing-sm) var(--spacing-lg);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-sm);
  background: var(--bg-color-2);
  color: var(--text-color-1);
  font-size: 0.875rem;
  cursor: pointer;
}

.submit-btn {
  padding: var(--spacing-sm) var(--spacing-lg);
  border: none;
  border-radius: var(--radius-sm);
  background: var(--primary-color);
  color: white;
  font-size: 0.875rem;
  cursor: pointer;
}

.submit-btn:hover {
  background: var(--primary-hover);
}

@media (max-width: 768px) {
  .modal-overlay {
    padding: var(--spacing-sm);
    align-items: flex-end;
  }

  .notification-panel {
    max-width: 100%;
    max-height: 85vh;
    border-radius: var(--radius-md) var(--radius-md) 0 0;
  }

  .panel-body {
    padding: var(--spacing-md);
  }

  .form-row {
    flex-direction: column;
  }
}
</style>
