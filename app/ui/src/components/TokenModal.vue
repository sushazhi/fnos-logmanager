<template>
  <div class="modal active" @click.self="$emit('close')">
    <div class="modal-content">
      <div class="modal-header">Token认证</div>
      <div class="modal-body">
        <p>请输入访问Token:</p>
        <div class="form-group">
          <input 
            type="password" 
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
  background: var(--overlay);
  display: flex;
  justify-content: center;
  align-items: center;
  z-index: 1000;
}

.modal-content {
  background: var(--card-bg);
  color: var(--text-color-1);
  padding: var(--spacing-3xl);
  border-radius: var(--radius-sm);
  max-width: 400px;
  width: 90%;
}

.modal-header {
  font-size: var(--font-size-2xl);
  font-weight: 600;
  margin-bottom: var(--spacing-xl);
}

.modal-body p {
  margin-bottom: var(--spacing-lg);
}

.form-group input {
  width: 100%;
  padding: var(--spacing-sm) var(--spacing-md);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-xs);
  font-size: var(--font-size-md);
  background: var(--bg-color-1);
  color: var(--text-color-1);
}

.modal-footer {
  margin-top: var(--spacing-xl);
  text-align: right;
}

.modal-footer button {
  padding: var(--spacing-sm) var(--spacing-xl);
  background: var(--primary-gradient);
  color: white;
  border: none;
  border-radius: var(--radius-xs);
  cursor: pointer;
}
</style>
