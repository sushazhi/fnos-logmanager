<template>
  <div class="modal-overlay" @click.self="$emit('close')">
    <div class="notification-panel">
      <div class="panel-header">
        <h3>通知设置</h3>
        <button class="close-btn" @click="$emit('close')">×</button>
      </div>

      <div class="panel-body">
        <!-- 全局设置 -->
        <GlobalSettings
          v-model:enabled="settings.enabled"
          v-model:checkInterval="settings.checkInterval"
          @update:enabled="updateSettings"
          @update:checkInterval="updateSettings"
        />

        <div class="divider"></div>

        <!-- 监控状态 -->
        <MonitorStatus
          :status="monitorStatus"
          :enabled="settings.enabled"
          @start="startMonitor"
          @stop="stopMonitor"
          @check="triggerCheck"
        />

        <div class="divider"></div>

        <!-- 通知渠道 -->
        <ChannelList
          :channels="channels"
          @add="showAddChannel = true"
          @edit="editChannel"
          @toggle="toggleChannel"
          @test="testChannel"
          @delete="confirmDeleteChannel"
        />

        <div class="divider"></div>

        <!-- 通知规则 -->
        <RuleList
          :rules="rules"
          @add="showAddRule = true"
          @edit="editRule"
          @toggle="toggleRule"
          @delete="confirmDeleteRule"
        />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue';
import GlobalSettings from './notification/GlobalSettings.vue';
import MonitorStatus from './notification/MonitorStatus.vue';
import ChannelList from './notification/ChannelList.vue';
import RuleList from './notification/RuleList.vue';
import { ChannelItem, RuleItem, MonitorStatusProps } from './notification/types';

const emit = defineEmits<{
  (e: 'close'): void;
}>();

// 状态
const settings = ref({
  enabled: false,
  checkInterval: 30000
});

const monitorStatus = ref<MonitorStatusProps>({
  running: false,
  watchedFiles: 0,
  activeRules: 0,
  lastCheckTime: null,
  errors: []
});

const channels = ref<ChannelItem[]>([]);
const rules = ref<RuleItem[]>([]);

const showAddChannel = ref(false);
const showAddRule = ref(false);

// 加载数据
onMounted(async () => {
  await loadSettings();
  await loadChannels();
  await loadRules();
  await loadStatus();
});

async function loadSettings() {
  try {
    const response = await fetch('/api/notifications/settings');
    if (response.ok) {
      settings.value = await response.json();
    }
  } catch (err) {
    console.error('Failed to load settings:', err);
  }
}

async function loadChannels() {
  try {
    const response = await fetch('/api/notifications/channels');
    if (response.ok) {
      channels.value = await response.json();
    }
  } catch (err) {
    console.error('Failed to load channels:', err);
  }
}

async function loadRules() {
  try {
    const response = await fetch('/api/notifications/rules');
    if (response.ok) {
      rules.value = await response.json();
    }
  } catch (err) {
    console.error('Failed to load rules:', err);
  }
}

async function loadStatus() {
  try {
    const response = await fetch('/api/notifications/status');
    if (response.ok) {
      monitorStatus.value = await response.json();
    }
  } catch (err) {
    console.error('Failed to load status:', err);
  }
}

async function updateSettings() {
  try {
    await fetch('/api/notifications/settings', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(settings.value)
    });
  } catch (err) {
    console.error('Failed to update settings:', err);
  }
}

async function startMonitor() {
  try {
    await fetch('/api/notifications/start', { method: 'POST' });
    await loadStatus();
  } catch (err) {
    console.error('Failed to start monitor:', err);
  }
}

async function stopMonitor() {
  try {
    await fetch('/api/notifications/stop', { method: 'POST' });
    await loadStatus();
  } catch (err) {
    console.error('Failed to stop monitor:', err);
  }
}

async function triggerCheck() {
  try {
    await fetch('/api/notifications/check', { method: 'POST' });
    await loadStatus();
  } catch (err) {
    console.error('Failed to trigger check:', err);
  }
}

function editChannel(channel: ChannelItem) {
  // TODO: 打开渠道编辑对话框
  // 编辑渠道功能待实现
}

async function toggleChannel(name: string, enabled: boolean) {
  try {
    await fetch(`/api/notifications/channels/${name}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ enabled })
    });
    await loadChannels();
  } catch (err) {
    console.error('Failed to toggle channel:', err);
  }
}

async function testChannel(name: string) {
  try {
    await fetch(`/api/notifications/channels/${name}/test`, { method: 'POST' });
  } catch (err) {
    console.error('Failed to test channel:', err);
  }
}

async function confirmDeleteChannel(name: string) {
  if (confirm(`确定要删除渠道 "${name}" 吗？`)) {
    try {
      await fetch(`/api/notifications/channels/${name}`, { method: 'DELETE' });
      await loadChannels();
    } catch (err) {
      console.error('Failed to delete channel:', err);
    }
  }
}

function editRule(rule: RuleItem) {
  // TODO: 打开规则编辑对话框
  // 编辑规则功能待实现
}

async function toggleRule(id: string, enabled: boolean) {
  try {
    await fetch(`/api/notifications/rules/${id}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ enabled })
    });
    await loadRules();
  } catch (err) {
    console.error('Failed to toggle rule:', err);
  }
}

async function confirmDeleteRule(id: string) {
  if (confirm('确定要删除此规则吗？')) {
    try {
      await fetch(`/api/notifications/rules/${id}`, { method: 'DELETE' });
      await loadRules();
    } catch (err) {
      console.error('Failed to delete rule:', err);
    }
  }
}
</script>

<style scoped>
.modal-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(0, 0, 0, 0.5);
  display: flex;
  justify-content: center;
  align-items: center;
  z-index: 1000;
}

.notification-panel {
  width: 600px;
  max-width: 90vw;
  max-height: 80vh;
  background: var(--bg-primary);
  border-radius: 8px;
  overflow: hidden;
  display: flex;
  flex-direction: column;
}

.panel-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 16px 20px;
  border-bottom: 1px solid var(--border-color);
}

.panel-header h3 {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
}

.close-btn {
  width: 32px;
  height: 32px;
  border: none;
  border-radius: 4px;
  background: transparent;
  color: var(--text-secondary);
  font-size: 20px;
  cursor: pointer;
}

.close-btn:hover {
  background: var(--bg-secondary);
}

.panel-body {
  flex: 1;
  overflow-y: auto;
  padding: 16px 20px;
}

.divider {
  height: 1px;
  background: var(--border-color);
  margin: 16px 0;
}
</style>
