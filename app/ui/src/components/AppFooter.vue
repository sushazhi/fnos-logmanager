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
  color: var(--text-color-3);
  font-size: 0.875rem;
  border-top: 1px solid var(--border-color);
}

.footer a {
  color: var(--primary-color);
  text-decoration: none;
  transition: color var(--transition-fast);
}

.footer a:hover {
  color: var(--primary-hover);
}
</style>
