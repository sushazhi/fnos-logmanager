<template>
  <div class="hm-modal-base settings-panel">
    <div class="settings-header">
      <h3>设置</h3>
      <button class="close-btn" @click="$emit('close')">×</button>
    </div>
    
    <div class="settings-body">
      <div class="setting-item">
        <label>字体大小</label>
        <div class="font-size-controls">
          <button @click="decreaseFontSize">A-</button>
          <span class="font-size-value">{{ fontSize }}px</span>
          <button @click="increaseFontSize">A+</button>
        </div>
      </div>
      
      <div class="setting-item">
        <label>主题模式</label>
        <div class="theme-buttons">
          <button
            :class="{ active: theme === 'light' }"
            @click="setTheme('light')"
          >日间</button>
          <button
            :class="{ active: theme === 'dark' }"
            @click="setTheme('dark')"
          >夜间</button>
          <button
            :class="{ active: theme === 'auto' }"
            @click="setTheme('auto')"
          >自动</button>
        </div>
      </div>
      
      <div class="setting-item">
        <label>主题色</label>
        <div class="color-options">
          <button 
            v-for="color in colors" 
            :key="color.value"
            class="color-btn"
            :class="{ active: primaryColor === color.value }"
            :style="{ background: color.gradient }"
            @click="setColor(color.value)"
            :title="color.name"
          ></button>
        </div>
      </div>
      
      <div class="divider"></div>
      
      <div class="setting-item">
        <label>审计日志</label>
        <button class="action-btn" @click="$emit('showAudit')">
          查看审计日志
        </button>
      </div>

      <div class="divider"></div>

      <div class="setting-item">
        <label>版本更新</label>
        <div class="version-display">当前版本: <strong>v{{ appVersion }}</strong></div>
        <button class="action-btn" :disabled="checking" @click="manualCheck">
          <span v-if="checking" class="checking-spinner"></span>
          {{ checking ? '正在检查...' : '检查更新' }}
        </button>
        <div v-if="checkResult" class="check-result" :class="checkResult.type">
          {{ checkResult.message }}
          <span v-if="checkResult.type === 'success' && updateInfo" class="result-update-btn" @click="startUpdate">立即更新</span>
        </div>
      </div>

      <div class="divider"></div>

      <div class="setting-item">
        <div class="mcp-header">
          <label>MCP 服务器 (AI Agent 接入)</label>
          <label class="switch">
            <input type="checkbox" v-model="mcp.enabled" :disabled="savingMCP">
            <span class="switch-slider"></span>
          </label>
        </div>
        <p class="mcp-desc">通过标准 Model Context Protocol (Streamable HTTP) 将日志管理能力开放给 QwenPAW、OpenClaw、Hermes 等 AI Agent。</p>

        <template v-if="mcp.enabled">
          <div class="mcp-field">
            <label class="mcp-label">API Key</label>
            <div class="mcp-key-row">
              <input
                class="mcp-input"
                type="text"
                v-model="mcp.apiKey"
                :placeholder="mcp.hasKey ? '已设置（留空保持不变）' : '输入访问密钥'"
                :disabled="savingMCP"
              >
              <button v-if="mcp.hasKey" class="mcp-gen-btn" type="button" :disabled="savingMCP" @click="generateKey" title="生成随机密钥">生成</button>
            </div>
            <p class="mcp-hint">Agent 通过 <code>Authorization: Bearer &lt;key&gt;</code> 访问。留空仅允许本机访问。</p>
          </div>

          <div class="mcp-field">
            <label class="mcp-label">独立监听端口</label>
            <div class="mcp-key-row">
              <input
                class="mcp-input"
                type="number"
                v-model.number="mcp.port"
                min="0"
                max="65535"
                placeholder="0 = 不启用独立端口"
                :disabled="savingMCP"
              >
            </div>
            <p class="mcp-hint">设为 0 时 Agent 只能通过网关路径访问（需本机/登录态）。设置端口后外部 Agent 可直连，修改端口需重启应用生效。</p>
          </div>

          <div class="mcp-field" v-if="mcp.port > 0">
            <label class="mcp-label">绑定地址</label>
            <input
              class="mcp-input"
              type="text"
              v-model="mcp.bindAddr"
              placeholder="0.0.0.0"
              :disabled="savingMCP"
            >
            <p class="mcp-hint">默认为 <code>0.0.0.0</code>（允许局域网访问，受 API Key 保护）。</p>
          </div>

          <div class="mcp-field">
            <label class="mcp-label">接入地址</label>
            <div class="mcp-endpoint">{{ endpointLabel }}</div>
            <button class="action-btn" :disabled="savingMCP || savingKey" @click="copyEndpoint">
              {{ copied ? '已复制' : '复制接入地址' }}
            </button>
          </div>

          <button class="action-btn mcp-save-btn" :disabled="savingMCP" @click="saveMCPConfig">
            <span v-if="savingMCP" class="checking-spinner"></span>
            {{ savingMCP ? '保存中...' : '保存 MCP 配置' }}
          </button>
          <div v-if="mcpMessage" class="check-result" :class="mcpMessage.type">
            {{ mcpMessage.message }}
          </div>
        </template>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { mcpApi } from '../services/api'
import { applyThemeColor } from '../composables/useThemeColor'
import { useUpdate } from '../composables/useUpdate'

interface ThemeSettings {
  fontSize: number
  theme: string
  primaryColor: string
}

interface ColorOption {
  name: string
  value: string
  gradient: string
}

const emit = defineEmits<{
  close: []
  update: [settings: ThemeSettings]
  showAudit: []
}>()

const { appVersion, updateInfo, checkForUpdates, installUpdate } = useUpdate()

const checking = ref(false)
const checkResult = ref<{ type: 'success' | 'error' | 'info'; message: string } | null>(null)

async function manualCheck() {
  if (checking.value) return
  // 手动检查时清除忽略记录和关闭时间，确保能看到最新结果
  try { localStorage.removeItem('logmanager_ignore_version') } catch {}
  try { localStorage.removeItem('logmanager_update_close_time') } catch {}
  checking.value = true
  checkResult.value = null
  try {
    const result = await checkForUpdates()
    if (result) {
      checkResult.value = { type: 'success', message: `发现新版本 v${result.version}` }
    } else {
      checkResult.value = { type: 'info', message: '已是最新版本' }
    }
  } catch {
    checkResult.value = { type: 'error', message: '检查更新失败，请稍后重试' }
  } finally {
    checking.value = false
  }
}

async function startUpdate() {
  try {
    await installUpdate()
  } catch {
    checkResult.value = { type: 'error', message: '安装更新失败' }
  }
}

const fontSize = ref<number>(16)
const theme = ref<'light' | 'dark' | 'auto'>('light')
const primaryColor = ref<string>('#0A59F7')

const colors: ColorOption[] = [
  { name: '鸿蒙蓝', value: '#0A59F7', gradient: 'linear-gradient(135deg, #0A59F7 0%, #317AF7 50%, #54B2F7 100%)' },
  { name: '天青', value: '#317AF7', gradient: 'linear-gradient(135deg, #317AF7 0%, #54B2F7 100%)' },
  { name: '翠青', value: '#64BB5A', gradient: 'linear-gradient(135deg, #64BB5A 0%, #86CD7E 100%)' },
  { name: '琥珀金', value: '#E8B339', gradient: 'linear-gradient(135deg, #E8B339 0%, #F0C96A 100%)' },
  { name: '丹霞橙', value: '#E84026', gradient: 'linear-gradient(135deg, #E84026 0%, #F06048 100%)' },
  { name: '莫兰迪渐变', value: '#8ec5fc', gradient: 'linear-gradient(135deg, #e0c3fc 0%, #8ec5fc 33%, #a8edea 66%, #fed6e3 100%)' },
]

function increaseFontSize(): void {
  if (fontSize.value < 24) {
    fontSize.value += 2
    applyFontSize()
    saveSettings()
  }
}

function decreaseFontSize(): void {
  if (fontSize.value > 12) {
    fontSize.value -= 2
    applyFontSize()
    saveSettings()
  }
}

function setTheme(newTheme: 'light' | 'dark' | 'auto'): void {
  theme.value = newTheme
  applyTheme()
  saveSettings()
}

function setColor(color: string): void {
  primaryColor.value = color
  applyColor()
  saveSettings()
}

function applyTheme() {
  const root = document.documentElement
  const isDark = theme.value === 'dark' || 
    (theme.value === 'auto' && window.matchMedia('(prefers-color-scheme: dark)').matches)
  
  if (isDark) {
    root.classList.add('dark-theme')
  } else {
    root.classList.remove('dark-theme')
  }
}

function applyColor() {
  applyThemeColor(primaryColor.value)
}

function applyFontSize(): void {
  const root = document.documentElement
  root.style.setProperty('--base-font-size', `${fontSize.value}px`)
}

function applyAll(): void {
  applyTheme()
  applyColor()
  applyFontSize()
}

function saveSettings(): void {
  const settings: ThemeSettings = {
    fontSize: fontSize.value,
    theme: theme.value,
    primaryColor: primaryColor.value
  }
  localStorage.setItem('logmanager_settings', JSON.stringify(settings))
  emit('update', settings)
}

function loadSettings(): void {
  try {
    const saved = localStorage.getItem('logmanager_settings')
    if (saved) {
      const settings = JSON.parse(saved) as ThemeSettings
      fontSize.value = settings.fontSize || 16
      theme.value = (settings.theme as 'light' | 'dark' | 'auto') || 'light'
      primaryColor.value = settings.primaryColor || '#0A59F7'
    }
  } catch (e) {
    console.warn('Failed to load settings:', e)
    localStorage.removeItem('logmanager_settings')
  }
  
  applyAll()
}

// ==================== MCP 配置 ====================

const savingMCP = ref(false)
const savingKey = ref(false)
const copied = ref(false)
const mcpMessage = ref<{ type: 'success' | 'error' | 'info'; message: string } | null>(null)

const mcp = ref({
  enabled: false,
  apiKey: '',
  hasKey: false,
  appName: 'fnos-logmanager',
  port: 0,
  bindAddr: '0.0.0.0',
  endpoint: '',
  hostIp: ''
})

async function loadMCPConfig(): Promise<void> {
  try {
    const cfg = await mcpApi.getConfig()
    mcp.value.enabled = cfg.enabled
    mcp.value.hasKey = !!cfg.apiKey
    mcp.value.apiKey = ''
    mcp.value.appName = cfg.appName || 'fnos-logmanager'
    mcp.value.port = cfg.port || 0
    mcp.value.bindAddr = cfg.bindAddr || '0.0.0.0'
    mcp.value.endpoint = cfg.endpoint || ''
    mcp.value.hostIp = cfg.hostIp || ''
  } catch {
    // 读取失败时保持默认，静默处理
  }
}

const endpointLabel = computed<string>(() => {
  if (mcp.value.port > 0) {
    // 使用后端返回的本机局域网 IP 替换 <NAS-IP> 占位符；无法获取时才显示占位符
    const host = mcp.value.bindAddr !== '0.0.0.0' && mcp.value.bindAddr !== '::'
      ? mcp.value.bindAddr
      : (mcp.value.hostIp || '<NAS-IP>')
    return `http://${host}:${mcp.value.port}/mcp`
  }
  // 网关路径已被移除/不可用（fnOS 网关拦截），必须配置独立端口才能接入
  return '请配置独立端口后获取接入地址'
})

async function saveMCPConfig(): Promise<void> {
  if (savingMCP.value) return
  savingMCP.value = true
  mcpMessage.value = null
  try {
    const res = await mcpApi.updateConfig({
      enabled: mcp.value.enabled,
      apiKey: mcp.value.apiKey.trim(),
      appName: mcp.value.appName || 'fnos-logmanager',
      port: mcp.value.port || 0,
      bindAddr: mcp.value.bindAddr || '0.0.0.0'
    })
    // 保存成功后清空密钥输入
    if (mcp.value.apiKey.trim()) {
      mcp.value.hasKey = true
      mcp.value.apiKey = ''
    }
    mcpMessage.value = {
      type: 'success',
      message: res.requiresRestart
        ? '配置已保存，端口变更需重启应用后生效'
        : '配置已保存'
    }
  } catch (e) {
    mcpMessage.value = {
      type: 'error',
      message: (e instanceof Error ? e.message : '保存失败') as string
    }
  } finally {
    savingMCP.value = false
  }
}

function generateKey(): void {
  const arr = new Uint8Array(24)
  crypto.getRandomValues(arr)
  mcp.value.apiKey = Array.from(arr, b => b.toString(16).padStart(2, '0')).join('')
}

async function copyEndpoint(): Promise<void> {
  const text = endpointLabel.value
  // navigator.clipboard 需要安全上下文（HTTPS 或 localhost）。通过 HTTP 内网
  // 访问（如 http://192.168.0.2:8666）时该 API 不可用，需回退到
  // document.execCommand('copy') 方案。
  try {
    if (navigator.clipboard && window.isSecureContext) {
      await navigator.clipboard.writeText(text)
      copied.value = true
      setTimeout(() => { copied.value = false }, 2000)
      return
    }
    throw new Error('clipboard API unavailable')
  } catch {
    try {
      const ta = document.createElement('textarea')
      ta.value = text
      ta.style.position = 'fixed'
      ta.style.opacity = '0'
      document.body.appendChild(ta)
      ta.select()
      const ok = document.execCommand('copy')
      document.body.removeChild(ta)
      if (ok) {
        copied.value = true
        setTimeout(() => { copied.value = false }, 2000)
        return
      }
      throw new Error('execCommand copy failed')
    } catch {
      mcpMessage.value = { type: 'error', message: '复制失败，请手动复制' }
    }
  }
}

onMounted(() => {
  loadSettings()
  loadMCPConfig()
  
  window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', () => {
    if (theme.value === 'auto') {
      applyTheme()
    }
  })
})
</script>

<style scoped>
.settings-panel {
  max-width: 400px;
  width: 100%;
  position: relative;
  overflow: hidden;
}

.settings-panel::before {
  content: '';
  position: absolute;
  inset: 0;
  background: var(--glass-shine);
  pointer-events: none;
  border-radius: inherit;
  z-index: 1;
}

.settings-panel::after {
  content: '';
  position: absolute;
  inset: 0;
  box-shadow: var(--glass-edge-light);
  pointer-events: none;
  border-radius: inherit;
  z-index: 2;
}

.settings-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: var(--spacing-md) var(--spacing-xl);
  border-bottom: 1px solid var(--border-color);
  gap: var(--spacing-md);
}

.settings-header h3 {
  margin: 0;
  font-size: var(--font-size-xl);
  font-weight: 500;
  color: var(--text-color-1);
  flex: 1;
  white-space: nowrap;
}

.close-btn {
  background: none;
  border: none;
  font-size: var(--font-size-5xl);
  cursor: pointer;
  color: var(--text-color-2);
  padding: 0;
  line-height: 1;
  flex: none;
  flex-shrink: 0;
  transition: color var(--transition-fast);
}

.close-btn:hover {
  color: var(--text-color-1);
}

.settings-body {
  padding: var(--spacing-xl);
  max-height: 70vh;
  overflow-y: auto;
}

.setting-item {
  margin-bottom: var(--spacing-xl);
}

.setting-item:last-child {
  margin-bottom: 0;
}

.setting-item label {
  display: block;
  margin-bottom: var(--spacing-sm);
  font-weight: 500;
  font-size: var(--font-size-md);
  color: var(--text-color-1);
}

.font-size-controls {
  display: flex;
  align-items: center;
  gap: var(--spacing-md);
}

.font-size-controls button {
  width: 40px;
  height: 40px;
  border: 1px solid var(--border-color);
  border-radius: var(--radius-sm);
  background: var(--bg-color-2);
  cursor: pointer;
  font-size: var(--font-size-md);
  font-weight: 600;
  color: var(--text-color-1);
  transition: all var(--transition-fast);
}

.font-size-controls button:hover {
  background: var(--primary-color);
  color: var(--text-color-on-primary);
  border-color: var(--primary-color);
}

.font-size-controls button:active {
  transform: scale(0.95);
}

.font-size-value {
  font-size: var(--font-size-xl);
  font-weight: 600;
  min-width: 50px;
  text-align: center;
  color: var(--text-color-1);
}

.theme-buttons {
  display: flex;
  gap: var(--spacing-sm);
}

.theme-buttons button {
  flex: 1;
  padding: var(--spacing-sm);
  border: 2px solid var(--border-color);
  border-radius: var(--radius-sm);
  background: var(--bg-color-2);
  cursor: pointer;
  font-size: var(--font-size-base);
  transition: all var(--transition-fast);
  color: var(--text-color-1);
}

.theme-buttons button.active {
  border-color: var(--primary-color);
  background: var(--primary-color);
  color: var(--text-color-on-primary);
}

.theme-buttons button:hover {
  border-color: var(--primary-hover);
}

.theme-buttons button:active {
  transform: scale(0.98);
}

.color-options {
  display: flex;
  gap: var(--spacing-sm);
  flex-wrap: wrap;
}

.color-btn {
  width: 36px;
  height: 36px;
  min-width: 36px;
  max-width: 36px;
  border-radius: 50%;
  border: 3px solid transparent;
  cursor: pointer;
  transition: all var(--transition-fast);
  flex-shrink: 0;
  padding: 0;
  box-sizing: border-box;
  display: block;
}

.color-btn:hover {
  transform: scale(1.1);
}

.color-btn.active {
  border-color: var(--text-color-1);
  box-shadow: var(--focus-ring);
}

.divider {
  height: 1px;
  background: var(--divider-color);
  margin: var(--spacing-xl) 0;
}

.version-display {
  font-size: var(--font-size-md);
  color: var(--text-color-2);
  margin-bottom: var(--spacing-sm);
}

.version-display strong {
  color: var(--text-color-1);
  font-weight: 600;
}

.action-btn {
  width: 100%;
  padding: var(--spacing-sm);
  background: var(--bg-color-2);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-sm);
  cursor: pointer;
  font-size: var(--font-size-md);
  color: var(--text-color-1);
  transition: all var(--transition-fast);
  display: flex;
  align-items: center;
  justify-content: center;
  gap: var(--spacing-sm);
}

.action-btn:hover:not(:disabled) {
  background: var(--primary-color);
  color: var(--text-color-on-primary);
  border-color: var(--primary-color);
}

.action-btn:active:not(:disabled) {
  transform: scale(0.98);
}

.action-btn:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.checking-spinner {
  width: 14px;
  height: 14px;
  border: 2px solid var(--border-color);
  border-top-color: var(--primary-color);
  border-radius: 50%;
  animation: settings-spin 0.6s linear infinite;
  display: inline-block;
}

@keyframes settings-spin {
  to { transform: rotate(360deg); }
}

.check-result {
  margin-top: var(--spacing-sm);
  padding: var(--spacing-xs) var(--spacing-sm);
  border-radius: var(--radius-2xs);
  font-size: var(--font-size-sm);
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--spacing-sm);
}

.check-result.success {
  color: var(--success-color);
  background: color-mix(in srgb, var(--success-color) 10%, transparent);
}

.check-result.error {
  color: var(--error-color);
  background: color-mix(in srgb, var(--error-color) 10%, transparent);
}

.check-result.info {
  color: var(--text-color-2);
  background: var(--bg-color-3);
}

.result-update-btn {
  color: var(--primary-color);
  cursor: pointer;
  font-weight: 500;
  white-space: nowrap;
  text-decoration: underline;
  text-underline-offset: 2px;
}

.result-update-btn:hover {
  color: var(--primary-hover);
}

.notification-btn {
  width: 100%;
  padding: var(--spacing-sm);
  background: var(--bg-color-2);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-sm);
  cursor: pointer;
  font-size: var(--font-size-md);
  color: var(--text-color-1);
  transition: all var(--transition-fast);
}

.notification-btn:hover {
  background: var(--primary-color);
  color: var(--text-color-on-primary);
  border-color: var(--primary-color);
}

.notification-btn:active {
  transform: scale(0.98);
}

.mcp-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: var(--spacing-sm);
}

.mcp-header label:first-child {
  margin-bottom: 0;
}

.mcp-desc {
  font-size: var(--font-size-sm);
  color: var(--text-color-2);
  margin: 0 0 var(--spacing-md);
  line-height: 1.5;
}

.mcp-field {
  margin-bottom: var(--spacing-md);
}

.mcp-label {
  font-size: var(--font-size-sm);
  color: var(--text-color-2);
  margin-bottom: var(--spacing-xs);
}

.mcp-input {
  width: 100%;
  padding: var(--spacing-sm);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-sm);
  background: var(--bg-color-2);
  color: var(--text-color-1);
  font-size: var(--font-size-md);
  box-sizing: border-box;
  transition: border-color var(--transition-fast);
}

.mcp-input:focus {
  outline: none;
  border-color: var(--primary-color);
}

.mcp-key-row {
  display: flex;
  gap: var(--spacing-sm);
  align-items: center;
}

.mcp-key-row .mcp-input {
  flex: 1;
}

.mcp-gen-btn {
  padding: var(--spacing-sm) var(--spacing-md);
  background: var(--bg-color-2);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-sm);
  cursor: pointer;
  font-size: var(--font-size-sm);
  color: var(--text-color-1);
  white-space: nowrap;
  transition: all var(--transition-fast);
}

.mcp-gen-btn:hover:not(:disabled) {
  background: var(--primary-color);
  color: var(--text-color-on-primary);
  border-color: var(--primary-color);
}

.mcp-gen-btn:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.mcp-hint {
  font-size: var(--font-size-sm);
  color: var(--text-color-3);
  margin: var(--spacing-xs) 0 0;
  line-height: 1.5;
}

.mcp-hint code {
  background: var(--bg-color-3);
  padding: 1px 4px;
  border-radius: var(--radius-2xs);
  font-size: var(--font-size-xs);
}

.mcp-endpoint {
  padding: var(--spacing-sm);
  background: var(--bg-color-3);
  border-radius: var(--radius-sm);
  font-size: var(--font-size-sm);
  font-family: monospace;
  color: var(--primary-color);
  word-break: break-all;
  margin-bottom: var(--spacing-sm);
}

.mcp-save-btn {
  margin-top: var(--spacing-xs);
}

@media (max-width: 768px) {
  .settings-panel {
    max-width: 100%;
    border-radius: var(--radius-xl) var(--radius-xl) 0 0;
  }

  .settings-body {
    padding: var(--spacing-lg);
  }

  .theme-buttons {
    flex-direction: row;
    flex-wrap: wrap;
  }

  .theme-buttons button {
    flex: 1;
    min-width: 80px;
    padding: var(--spacing-xs) var(--spacing-sm);
    font-size: var(--font-size-sm);
  }

  .color-options {
    justify-content: space-between;
  }

  .color-btn {
    width: 32px;
    height: 32px;
    min-width: 32px;
    max-width: 32px;
  }
}

@media (max-width: 480px) {
  .settings-panel {
    max-width: 100%;
    border-radius: 0;
  }

  .settings-header {
    padding: var(--spacing-sm) var(--spacing-md);
  }

  .settings-header h3 {
    font-size: var(--font-size-lg);
  }

  .close-btn {
    font-size: var(--font-size-2xl);
    width: 18px;
    padding: 0;
    margin-left: auto;
  }

  .settings-body {
    padding: var(--spacing-md);
  }

  .setting-item {
    margin-bottom: var(--spacing-lg);
  }

  .setting-item label {
    font-size: var(--font-size-base);
  }

  .font-size-controls {
    gap: var(--spacing-sm);
  }

  .font-size-controls button {
    width: 36px;
    height: 36px;
    font-size: var(--font-size-base);
  }

  .font-size-value {
    font-size: var(--font-size-lg);
  }

  .theme-buttons {
    gap: var(--spacing-xs);
  }

  .theme-buttons button {
    padding: var(--spacing-xs);
    font-size: var(--font-size-sm);
  }

  .color-options {
    gap: var(--spacing-xs);
  }

  .color-btn {
    width: 30px;
    height: 30px;
    min-width: 30px;
    max-width: 30px;
  }
}
</style>
