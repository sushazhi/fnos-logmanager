<template>
  <div class="modal active" @click.self="$emit('close')">
    <div class="modal-content">
      <div class="modal-header">🗑️ 清理日志</div>
      <div class="modal-body">
        <div class="form-group">
          <label>清理方式</label>
          <select v-model="cleanType">
            <option value="truncate">清空大文件内容</option>
            <option value="deleteOld">删除旧归档文件</option>
          </select>
        </div>
        
        <div class="form-group" v-if="cleanType === 'truncate'">
          <label>文件大小阈值</label>
          <input type="text" v-model="threshold" placeholder="例如: 100M">
        </div>
        
        <div class="form-group" v-if="cleanType === 'deleteOld'">
          <label>删除多少天前的文件</label>
          <input type="number" v-model="days" placeholder="例如: 7">
        </div>
      </div>
      <div class="modal-footer">
        <button class="secondary" @click="$emit('close')">取消</button>
        <button class="danger" @click="execute">执行清理</button>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref } from 'vue'

const emit = defineEmits(['close', 'execute'])

const cleanType = ref('truncate')
const threshold = ref('100M')
const days = ref(7)

function execute() {
  emit('execute', cleanType.value, threshold.value, days.value)
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
  background: white;
  padding: 30px;
  border-radius: 12px;
  max-width: 400px;
  width: 90%;
}

.modal-header {
  font-size: 18px;
  font-weight: 600;
  margin-bottom: 20px;
}

.form-group {
  margin-bottom: 15px;
}

.form-group label {
  display: block;
  margin-bottom: 8px;
  font-weight: 500;
}

.form-group input,
.form-group select {
  width: 100%;
  padding: 10px;
  border: 1px solid #ddd;
  border-radius: 6px;
  font-size: 14px;
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
  background: #f0f2f5;
  color: #333;
}

.modal-footer button.danger {
  background: linear-gradient(135deg, #ff6b6b 0%, #ee5a5a 100%);
  color: white;
}
</style>
