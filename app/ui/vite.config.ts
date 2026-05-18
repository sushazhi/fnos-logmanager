import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { resolve } from 'path'

export default defineConfig({
  plugins: [vue()],
  base: './',
  resolve: {
    alias: {
      '@': resolve(__dirname, 'src')
    }
  },
  build: {
    outDir: 'dist',
    assetsDir: 'assets',
    sourcemap: false,
    minify: 'esbuild',
    chunkSizeWarningLimit: 1000,
    rollupOptions: {
      output: {
        manualChunks(id) {
          if (id.includes('node_modules/vue')) return 'vendor-vue'
          if (id.includes('node_modules/pinia')) return 'vendor-pinia'
          if (id.includes('node_modules/dompurify')) return 'vendor-dompurify'
        }
      }
    },
    target: 'es2020',
    cssCodeSplit: true,
    reportCompressedSize: true
  },
  server: {
    port: 3000,
    proxy: {
      '/api': {
        target: 'http://localhost:8090',
        changeOrigin: true
      }
    }
  },
  optimizeDeps: {
    include: ['vue', 'pinia', 'dompurify'],
    exclude: []
  }
})
