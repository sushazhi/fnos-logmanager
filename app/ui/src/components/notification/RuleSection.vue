<template>
  <div class="section">
    <div class="section-header">
      <h4>通知规则</h4>
      <button class="add-btn" @click="$emit('add')">+ 添加规则</button>
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
          <button class="toggle-btn" @click="$emit('toggle', rule.id)">
            {{ rule.status === 'enabled' ? '禁用' : '启用' }}
          </button>
          <button class="edit-btn" @click="$emit('edit', rule)">编辑</button>
          <button class="delete-btn" @click="$emit('delete', rule.id)">删除</button>
        </div>
      </div>
    </div>
    <div class="empty-hint" v-else>
      暂无通知规则，请先添加
    </div>
  </div>
</template>

<script setup lang="ts">
interface NotificationRule {
  id: string
  name: string
  status: string
  appName: string
  logLevel: string
  triggerCount: number
}

defineProps<{
  rules: NotificationRule[]
}>()

defineEmits<{
  add: []
  edit: [rule: NotificationRule]
  toggle: [id: string]
  delete: [id: string]
}>()
</script>
