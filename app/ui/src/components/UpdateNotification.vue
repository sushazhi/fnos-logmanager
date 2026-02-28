<template>
  <div class="update-notification">
    <div class="content">
      <h4>🎉 发现新版本</h4>
      <p>飞牛日志管理 v{{ updateInfo.version }} 已发布</p>
    </div>
    <div class="actions">
      <a :href="updateInfo.url" target="_blank" class="btn">查看更新</a>
      <button class="close" @click="$emit('close')">×</button>
    </div>
  </div>
</template>

<script setup>
defineProps({
  updateInfo: {
    type: Object,
    required: true
  }
})

defineEmits(['close'])
</script>

<style scoped>
.update-notification {
  position: fixed;
  top: 20px;
  right: 20px;
  background: linear-gradient(135deg, #4CAF50 0%, #45a049 100%);
  color: white;
  padding: 15px 20px;
  border-radius: 10px;
  box-shadow: 0 4px 15px rgba(0, 0, 0, 0.2);
  z-index: 2000;
  display: flex;
  align-items: center;
  gap: 20px;
  max-width: 350px;
  animation: slideIn 0.3s ease;
}

@keyframes slideIn {
  from {
    transform: translateX(100%);
    opacity: 0;
  }
  to {
    transform: translateX(0);
    opacity: 1;
  }
}

.content h4 {
  margin: 0 0 5px 0;
  font-size: 14px;
}

.content p {
  margin: 0;
  font-size: 12px;
  opacity: 0.9;
}

.actions {
  display: flex;
  align-items: center;
  gap: 10px;
}

.btn {
  background: white;
  color: #4CAF50;
  padding: 6px 12px;
  border-radius: 5px;
  text-decoration: none;
  font-size: 12px;
  font-weight: 600;
}

.close {
  background: none;
  border: none;
  color: white;
  font-size: 18px;
  cursor: pointer;
  padding: 0;
  line-height: 1;
  opacity: 0.8;
}

.close:hover {
  opacity: 1;
}

@media (max-width: 768px) {
  .update-notification {
    left: 10px;
    right: 10px;
    max-width: none;
    top: 10px;
  }
}
</style>
