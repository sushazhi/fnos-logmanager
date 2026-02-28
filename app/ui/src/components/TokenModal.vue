<template>
  <div class="modal active" @click.self="$emit('close')">
    <div class="modal-content">
      <div class="modal-header">🔐 Token认证</div>
      <div class="modal-body">
        <p>请输入访问Token:</p>
        <div class="form-group">
          <input 
            type="text" 
            v-model="token" 
            placeholder="输入Token"
            @keyup.enter="submit"
          >
        </div>
      </div>
      <div class="modal-footer">
        <button @click="submit">确认</button>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref } from 'vue'

const emit = defineEmits(['close', 'save'])

const token = ref('')

function submit() {
  if (token.value.trim()) {
    emit('save', token.value.trim())
  }
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

.modal-body p {
  margin-bottom: 15px;
}

.form-group input {
  width: 100%;
  padding: 10px;
  border: 1px solid #ddd;
  border-radius: 6px;
  font-size: 14px;
}

.modal-footer {
  margin-top: 20px;
  text-align: right;
}

.modal-footer button {
  padding: 10px 20px;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  color: white;
  border: none;
  border-radius: 6px;
  cursor: pointer;
}
</style>
