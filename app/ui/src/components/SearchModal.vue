<template>
  <div class="modal active" @click.self="$emit('close')">
    <div class="modal-content">
      <div class="modal-header">🔍 查找日志</div>
      <div class="modal-body">
        <div class="form-group">
          <label>查找方式</label>
          <select v-model="searchType">
            <option value="size">按文件大小</option>
            <option value="name">按文件名称</option>
          </select>
        </div>
        
        <div class="form-group" v-if="searchType === 'size'">
          <label>文件大小阈值</label>
          <input type="text" v-model="threshold" placeholder="例如: 10M, 100M, 1G">
          <div class="hint">查找超过指定大小的日志文件</div>
        </div>
        
        <div class="form-group" v-if="searchType === 'name'">
          <label>文件名包含</label>
          <input type="text" v-model="pattern" placeholder="例如: error, access, app">
          <div class="hint">查找文件名包含关键字的日志文件</div>
        </div>
      </div>
      <div class="modal-footer">
        <button class="secondary" @click="$emit('close')">取消</button>
        <button @click="execute">开始查找</button>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref } from 'vue'

const emit = defineEmits(['close', 'execute'])

const searchType = ref('size')
const threshold = ref('10M')
const pattern = ref('')

function execute() {
  emit('execute', searchType.value, threshold.value, pattern.value)
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
  z-index: 1000;
}

.modal-content {
  background: var(--card-bg, white);
  padding: 30px;
  border-radius: 12px;
  max-width: 400px;
  width: 90%;
}

.modal-header {
  font-size: 18px;
  font-weight: 600;
  margin-bottom: 20px;
  color: var(--text-color, #333);
}

.form-group {
  margin-bottom: 15px;
}

.form-group label {
  display: block;
  margin-bottom: 8px;
  font-weight: 500;
  color: var(--text-color, #333);
}

.form-group input,
.form-group select {
  width: 100%;
  padding: 10px;
  border: 1px solid var(--border-color, #ddd);
  border-radius: 6px;
  font-size: 14px;
  background: var(--card-bg, white);
  color: var(--text-color, #333);
}

.hint {
  margin-top: 6px;
  font-size: 12px;
  color: var(--text-secondary, #666);
}

.modal-footer {
  margin-top: 20px;
  display: flex;
  gap: 10px;
  justify-content: flex-end;
}

.modal-footer button {
  padding: 10px 20px;
  border: none;
  border-radius: 6px;
  cursor: pointer;
}

.modal-footer button.secondary {
  background: var(--border-color, #f0f2f5);
  color: var(--text-color, #333);
}

.modal-footer button:not(.secondary) {
  background: linear-gradient(135deg, var(--primary-color, #667eea) 0%, #764ba2 100%);
  color: white;
}
</style>
