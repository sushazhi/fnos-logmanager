<template>
  <div class="section">
    <div class="section-header">
      <h4>监控状态</h4>
      <span :class="['status-badge', status.running ? 'running' : 'stopped']">
        {{ status.running ? '运行中' : '已停止' }}
      </span>
    </div>
    <div class="status-info">
      <div class="info-item">
        <span class="label">监控文件数</span>
        <span class="value">{{ status.watchedFiles }}</span>
      </div>
      <div class="info-item">
        <span class="label">活跃规则数</span>
        <span class="value">{{ status.activeRules }}</span>
      </div>
    </div>
    <div class="btn-row">
      <button class="control-btn" @click="$emit('start')" :disabled="status.running || !enabled">启动</button>
      <button class="control-btn danger" @click="$emit('stop')" :disabled="!status.running">停止</button>
      <button class="control-btn" @click="$emit('check')" :disabled="!enabled">立即检查</button>
    </div>
  </div>
</template>

<script setup lang="ts">
interface MonitorStatus {
  running: boolean
  watchedFiles: number
  activeRules: number
}

defineProps<{
  status: MonitorStatus
  enabled: boolean
}>()

defineEmits<{
  start: []
  stop: []
  check: []
}>()
</script>
