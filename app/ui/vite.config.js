import { defineConfig } from 'vite';
import vue from '@vitejs/plugin-vue';
import { resolve } from 'path';
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
                manualChunks: {
                    'vendor-vue': ['vue'],
                    'vendor-pinia': ['pinia'],
                    'vendor-dompurify': ['dompurify']
                }
            }
        },
        // 性能优化
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
    // 优化依赖预构建
    optimizeDeps: {
        include: ['vue', 'pinia', 'dompurify'],
        exclude: []
    }
});
