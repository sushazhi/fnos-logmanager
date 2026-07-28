<template>
  <footer class="footer">
    <p>飞牛应用日志管理 © 2026</p>
    <p>
      <a href="https://github.com/sushazhi/fnos-logmanager/issues" target="_blank" rel="noopener noreferrer">
        问题反馈
      </a>
    </p>
  </footer>
</template>

<style scoped>
.footer {
  margin-top: var(--spacing-3xl);
  padding: var(--spacing-xl);
  text-align: center;
  color: var(--text-color-on-primary);
  font-size: var(--font-size-md);
  position: relative;
  overflow: hidden;
  border-radius: var(--radius-lg);
  background: var(--primary-gradient);
  box-shadow: var(--depth-3), 0 0 30px var(--glow-primary-soft);
  transition: background var(--transition-base), box-shadow var(--transition-base);
}

.footer::before {
  content: '';
  position: absolute;
  inset: 0;
  background: var(--glass-shine);
  pointer-events: none;
  border-radius: inherit;
}

.footer a {
  color: var(--text-color-on-primary);
  text-decoration: none;
  transition: color var(--transition-fast), text-shadow var(--transition-fast);
  opacity: 0.9;
}

.footer a:hover {
  opacity: 1;
  text-shadow: 0 0 12px rgba(255, 255, 255, 0.6);
}
</style>
