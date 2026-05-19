<template>
  <div class="bookmark-bar" v-if="bookmarks.length > 0 || showAddForm">
    <div class="bookmark-list" ref="listRef">
      <div
        v-for="bookmark in bookmarks"
        :key="bookmark.id"
        class="bookmark-tag"
        :class="{ docker: bookmark.isDocker }"
        @click="$emit('open-bookmark', bookmark)"
      >
        <span class="bookmark-icon">{{ bookmark.isDocker ? '🐳' : '📄' }}</span>
        <span class="bookmark-name" :title="bookmark.path">{{ bookmark.name }}</span>
        <button
          class="bookmark-delete"
          @click.stop="$emit('delete-bookmark', bookmark.id)"
          title="删除书签"
        >×</button>
      </div>
    </div>
    <button class="add-btn" @click="showAddForm = !showAddForm" title="添加书签">
      <span class="add-icon">{{ showAddForm ? '−' : '+' }}</span>
    </button>
    <div class="add-form" v-if="showAddForm">
      <input
        type="text"
        v-model="newPath"
        placeholder="日志文件路径"
        class="add-input"
      >
      <input
        type="text"
        v-model="newName"
        placeholder="显示名称（可选）"
        class="add-input"
      >
      <label class="docker-toggle">
        <input type="checkbox" v-model="newIsDocker">
        Docker
      </label>
      <button class="add-confirm" @click="handleAdd" :disabled="!newPath">添加</button>
      <button class="add-cancel" @click="cancelAdd">取消</button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import type { Bookmark } from '../services/api'

defineProps<{
  bookmarks: Bookmark[]
}>()

const emit = defineEmits<{
  'open-bookmark': [bookmark: Bookmark]
  'delete-bookmark': [id: string]
  'add-bookmark': [data: { path: string; name?: string; isDocker?: boolean }]
}>()

const listRef = ref<HTMLElement | null>(null)
const showAddForm = ref(false)
const newPath = ref('')
const newName = ref('')
const newIsDocker = ref(false)

function handleAdd() {
  if (!newPath.value) return
  emit('add-bookmark', {
    path: newPath.value,
    name: newName.value || undefined,
    isDocker: newIsDocker.value || undefined
  })
  cancelAdd()
}

function cancelAdd() {
  showAddForm.value = false
  newPath.value = ''
  newName.value = ''
  newIsDocker.value = false
}

defineExpose({
  prefill: (path: string, name?: string, isDocker?: boolean) => {
    showAddForm.value = true
    newPath.value = path
    newName.value = name || ''
    newIsDocker.value = !!isDocker
  }
})
</script>

<style scoped>
.bookmark-bar {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
  padding: var(--spacing-sm) var(--spacing-md);
  margin-top: var(--spacing-sm);
  background: var(--bg-color-2);
  border-radius: var(--radius-md);
  box-shadow: var(--shadow-xs);
}

.bookmark-list {
  display: flex;
  flex-wrap: wrap;
  gap: var(--spacing-xs);
  flex: 1;
  min-width: 0;
  overflow-x: auto;
}

.bookmark-tag {
  display: inline-flex;
  align-items: center;
  gap: var(--spacing-xs);
  padding: var(--spacing-xs) var(--spacing-sm);
  background: var(--bg-color-1);
  border: 1px solid var(--bg-color-4);
  border-radius: var(--radius-xs);
  font-size: var(--font-size-base);
  color: var(--text-color-2);
  cursor: pointer;
  transition: all var(--transition-fast);
  white-space: nowrap;
  max-width: 200px;
}

.bookmark-tag:hover {
  background: var(--info-bg);
  border-color: var(--primary-color);
  color: var(--primary-color);
  box-shadow: var(--shadow-xs);
}

.bookmark-tag.docker {
  background: var(--info-bg);
  border-color: var(--info-color);
  color: var(--info-color);
}

.bookmark-tag.docker:hover {
  border-color: var(--primary-color);
  color: var(--primary-color);
}

.bookmark-icon {
  flex-shrink: 0;
  font-size: var(--font-size-sm);
}

.bookmark-name {
  overflow: hidden;
  text-overflow: ellipsis;
}

.bookmark-delete {
  display: none;
  border: none;
  background: none;
  color: var(--text-color-3);
  cursor: pointer;
  font-size: var(--font-size-sm);
  width: 16px;
  height: 16px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  flex-shrink: 0;
  transition: all var(--transition-fast);
}

.bookmark-tag:hover .bookmark-delete {
  display: inline-flex;
}

.bookmark-delete:hover {
  color: var(--error-color);
  background: var(--error-bg);
}

.add-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  min-width: 28px;
  max-width: 28px;
  border: 1px dashed var(--bg-color-4);
  border-radius: var(--radius-xs);
  background: transparent;
  color: var(--text-color-3);
  cursor: pointer;
  transition: all var(--transition-fast);
  flex-shrink: 0;
}

.add-btn:hover {
  border-color: var(--primary-color);
  color: var(--primary-color);
  background: var(--info-bg);
  border-style: solid;
}

.add-icon {
  font-size: var(--font-size-xl);
  font-weight: 600;
  line-height: 1;
}

.add-form {
  display: flex;
  align-items: center;
  gap: var(--spacing-xs);
  flex-wrap: wrap;
}

.add-input {
  padding: var(--spacing-xs) var(--spacing-sm);
  border: 1px solid var(--bg-color-4);
  border-radius: var(--radius-xs);
  font-size: var(--font-size-base);
  background: var(--bg-color-1);
  color: var(--text-color);
  max-width: 200px;
  transition: border-color var(--transition-fast), box-shadow var(--transition-fast);
}

.add-input:focus {
  outline: none;
  border-color: var(--primary-color);
  box-shadow: 0 0 0 2px var(--info-bg);
}

.docker-toggle {
  display: inline-flex;
  align-items: center;
  gap: var(--spacing-xs);
  font-size: var(--font-size-base);
  color: var(--text-color-2);
  cursor: pointer;
  white-space: nowrap;
}

.add-confirm,
.add-cancel {
  padding: var(--spacing-xs) var(--spacing-sm);
  border: 1px solid var(--bg-color-4);
  border-radius: var(--radius-xs);
  font-size: var(--font-size-base);
  cursor: pointer;
  transition: all var(--transition-fast);
  font-weight: 500;
}

.add-confirm {
  background: var(--primary-color);
  color: white;
  border-color: var(--primary-color);
}

.add-confirm:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.add-confirm:not(:disabled):hover {
  background: var(--primary-hover);
}

.add-cancel {
  background: transparent;
  color: var(--text-color-2);
}

.add-cancel:hover {
  background: var(--bg-color-2);
}

@media (max-width: 768px) {
  .add-input {
    max-width: 140px;
  }

  .bookmark-tag {
    max-width: 150px;
  }
}
</style>
