<template>
  <div class="card glass-card">
    <h2>操作功能</h2>
    
    <!-- 入口按语义分三组。原先 14 个按钮平铺成 4 行、「系统空间清理」还独占一行，
         用户难以判断该点哪个；分组后认知负担从「14 个并列」降到「3 组各 4-5 个」。 -->
    <div class="action-group">
      <div class="group-label">日志查看</div>
      <div class="actions-row">
        <button class="btn-primary" @click="$emit('listLogs')" title="列出所有日志文件">
          列出日志
        </button>
        <button class="btn-primary" @click="$emit('showSearch')" title="按大小或名称查找日志文件">
          查找日志
        </button>
        <button class="btn-primary" @click="$emit('listArchives')" title="查看压缩归档的日志文件">
          归档日志
        </button>
        <button class="btn-primary" @click="$emit('listDocker')" title="查看Docker容器日志">
          Docker日志
        </button>
      </div>
    </div>

    <div class="action-group">
      <div class="group-label">空间与清理</div>
      <div class="actions-row">
        <button class="danger" @click="$emit('showClean')" title="清空或删除旧日志文件">
          清理日志
        </button>
        <button
          class="space-clean-btn"
          @click="$emit('showSpaceClean')"
          title="分析系统空间占用，按分类清理可再生缓存（包管理器缓存、浏览器缓存、缩略图、容器日志、安装包残留等），并可视化展示可释放空间"
        >
          系统空间清理
        </button>
        <button class="warning" @click="$emit('showAutoClean')" title="配置定时自动清理策略">
          自动清理
        </button>
        <button class="warning" @click="$emit('showUninstalledClean')" title="清理已卸载应用的残留：空文件夹直接删除，其余残留移入回收站，保留时长可设置，到期自动清空">
          清理已卸载应用
        </button>
        <button class="btn-primary" @click="$emit('backup')" title="打开备份管理：创建备份、预览内容并恢复到原位置、删除备份">
          备份
        </button>
      </div>
    </div>

    <div class="action-group">
      <div class="group-label">系统与设置</div>
      <div class="actions-row">
        <button class="btn-primary" @click="$emit('showProcesses')" title="查看系统中运行的进程并支持结束进程">
          进程管理
        </button>
        <button class="btn-primary" @click="$emit('showKernelModules')" title="查看已安装的 Linux 内核版本">
          内核版本
        </button>
        <button class="btn-primary" @click="$emit('showEventLogger')" title="配置系统日志监控">
          系统日志
        </button>
        <button class="btn-primary" @click="$emit('showNotification')" title="配置日志通知规则">
          通知设置
        </button>
        <button class="btn-primary" @click="$emit('openSettings')" title="显示设置">
          设置
        </button>
      </div>
    </div>
    
    <div class="filter-section">
      <div class="filter-toggle">
        <span class="filter-label">敏感信息过滤</span>
        <label class="switch">
          <input 
            type="checkbox" 
            :checked="filterEnabled"
            @change="$emit('toggleFilter')"
          >
          <span class="slider"></span>
        </label>
        <span class="filter-status">{{ filterEnabled ? '已启用' : '已禁用' }}</span>
      </div>
    </div>
    
    <div class="status" :class="status.type">
      <span class="status-icon">{{ statusIcons[status.type] }}</span>
      <span>{{ status.message }}</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import type { Status, StatusType } from '../types'

defineProps<{
  status: Status
  filterEnabled: boolean
}>()

defineEmits<{
  refresh: []
  listLogs: []
  showSearch: []
  showClean: []
  showAutoClean: []
  showUninstalledClean: []
  backup: []
  listArchives: []
  listDocker: []
  toggleFilter: []
  openSettings: []
  showNotification: []
  showEventLogger: []
  showKernelModules: []
  showProcesses: []
  showSpaceClean: []
}>()

const statusIcons: Record<StatusType, string> = {
  success: '✓',
  error: '✗',
  warning: '⚠',
  loading: '⟳',
  info: 'ℹ'
}
</script>

<style scoped>
/* 分组容器：三组之间留白，组内按钮紧凑 */
.action-group {
  margin-top: var(--spacing-lg);
}

.action-group:first-of-type {
  margin-top: var(--spacing-lg);
}

/* 组标题：弱化的视觉层级，只做归类提示，不与按钮标题争夺注意力 */
.group-label {
  font-size: var(--font-size-xs);
  font-weight: var(--font-weight-medium);
  color: var(--text-color-3);
  letter-spacing: 0.5px;
  margin-bottom: var(--spacing-sm);
  padding-left: 2px;
}

.actions-row {
  display: flex;
  flex-wrap: wrap;
  gap: var(--spacing-sm);
}

.actions-row:first-of-type {
  margin-top: 0;
}

button {
  /* 允许换行后不能再无条件 flex:1，否则 5 个按钮会被压得过窄 */
  flex: 1 1 auto;
  min-width: 104px;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: var(--spacing-xs);
  padding: var(--spacing-md) var(--spacing-sm);
  background: linear-gradient(135deg,
    var(--card-color-1) 0%,
    var(--card-color-1-light) 50%,
    var(--card-color-2) 100%
  );
  color: var(--text-color);
  font-size: var(--font-size-md);
  font-weight: var(--font-weight-semibold);
  border-radius: var(--radius-sm);
  cursor: pointer;
  transition: all var(--transition-spring);
  box-shadow: var(--depth-1);
  position: relative;
  overflow: hidden;
}

button::after {
  content: '';
  position: absolute;
  inset: 0;
  background: linear-gradient(180deg, rgba(255,255,255,0.1) 0%, transparent 50%);
  pointer-events: none;
  border-radius: inherit;
}

button:hover {
  transform: translateY(-2px) perspective(var(--perspective-near)) rotateX(0.5deg);
  box-shadow: var(--depth-2), 0 0 20px var(--glow-primary);
}

button:active {
  transform: scale(var(--scale-active));
  box-shadow: var(--depth-1), 0 0 28px var(--glow-primary-strong);
}

button.danger {
  background: linear-gradient(135deg,
    var(--card-color-4) 0%,
    var(--card-color-4-light) 50%,
    var(--card-color-3) 100%
  );
  color: var(--text-color);
}

button.danger:hover {
  background: linear-gradient(135deg,
    var(--card-color-4-light) 0%,
    var(--card-color-4) 50%,
    var(--card-color-3-light) 100%
  );
  box-shadow: var(--depth-2), 0 0 20px var(--glow-error);
}

button.danger:active {
  box-shadow: var(--depth-1), 0 0 28px var(--glow-error);
}

button.warning {
  background: linear-gradient(135deg,
    var(--warning-color) 0%,
    var(--warning-color) 50%,
    var(--error-color) 100%
  );
  color: var(--text-color);
}

button.warning:hover {
  background: linear-gradient(135deg,
    var(--warning-color) 0%,
    var(--warning-color) 50%,
    var(--error-color) 100%
  );
  box-shadow: var(--depth-2), 0 0 20px var(--glow-warning);
}

.filter-section {
  margin-top: var(--spacing-lg);
  padding: var(--spacing-md);
  background: var(--glass-bg);
  backdrop-filter: blur(var(--glass-blur));
  -webkit-backdrop-filter: blur(var(--glass-blur));
  border-radius: var(--radius-sm);
  border: 1px solid var(--glass-border);
}

.filter-toggle {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
}

.filter-label {
  font-weight: var(--font-weight-medium);
  font-size: var(--font-size-md);
  color: var(--text-color-1);
}

.switch {
  position: relative;
  display: inline-block;
  width: 44px;
  height: 22px;
}

.switch input {
  opacity: 0;
  width: 0;
  height: 0;
}

.slider {
  position: absolute;
  cursor: pointer;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background-color: var(--bg-color-4);
  transition: var(--transition-base);
  border-radius: var(--radius-xl);
}

.slider::before {
  position: absolute;
  content: "";
  height: 18px;
  width: 18px;
  left: 2px;
  bottom: 2px;
  background-color: white;
  transition: var(--transition-base);
  border-radius: 50%;
  box-shadow: var(--depth-0);
}

input:checked + .slider {
  background: var(--primary-color);
  box-shadow: 0 0 8px var(--glow-primary);
}

input:checked + .slider::before {
  transform: translateX(22px);
}

.filter-status {
  font-size: var(--font-size-sm);
  font-weight: var(--font-weight-regular);
  color: var(--text-color-2);
}

.status {
  margin-top: var(--spacing-lg);
  padding: var(--spacing-sm) var(--spacing-md);
  border-radius: var(--radius-sm);
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
  font-size: var(--font-size-md);
  font-weight: var(--font-weight-regular);
  background: var(--glass-bg);
  backdrop-filter: blur(var(--glass-blur));
  -webkit-backdrop-filter: blur(var(--glass-blur));
  border: 1px solid var(--glass-border);
}

.status.success {
  background: var(--success-bg);
  color: var(--success-color);
  border-color: rgba(100, 187, 90, 0.2);
}

.status.error {
  background: var(--error-bg);
  color: var(--error-color);
  border-color: rgba(232, 64, 38, 0.2);
}

.status.warning {
  background: var(--warning-bg);
  color: var(--warning-color);
  border-color: rgba(232, 179, 57, 0.2);
}

.status.loading {
  background: var(--info-bg);
  color: var(--info-color);
  border-color: rgba(49, 122, 247, 0.2);
}

.status-icon {
  font-size: var(--font-size-md);
}

.btn-primary:hover {
  box-shadow: var(--depth-2), 0 0 20px var(--glow-primary);
}

/* 系统空间清理入口：使用成功色渐变，与普通操作区分开 */
.space-clean-btn {
  background: linear-gradient(135deg,
    var(--card-color-3) 0%,
    var(--card-color-3-light) 50%,
    var(--card-color-2-light) 100%
  ) !important;
}

.space-clean-btn:hover {
  box-shadow: var(--depth-2), 0 0 20px rgba(100, 187, 90, 0.45) !important;
}

.space-clean-btn:active {
  transform: scale(0.97);
}

.btn-primary:active {
  transform: scale(0.97);
  box-shadow: var(--depth-1), 0 0 28px var(--glow-primary-strong);
}

.glass-card {
  border: 1px solid var(--glass-border);
  position: relative;
}

.glass-card::before {
  content: '';
  position: absolute;
  inset: 0;
  background: var(--glass-shine);
  pointer-events: none;
  border-radius: inherit;
}

@media (max-width: 768px) {
  .actions-row {
    flex-wrap: wrap;
  }

  button {
    padding: var(--spacing-sm) var(--spacing-xs);
    font-size: var(--font-size-base);
  }

  .filter-label {
    font-size: var(--font-size-base);
  }
}

@media (max-width: 480px) {
  .actions-row {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: var(--spacing-xs);
  }

  button {
    width: 100%;
    /* 网格已经限定了列宽，这里的 min-width 只会造成溢出 */
    min-width: 0;
  }
}
</style>
