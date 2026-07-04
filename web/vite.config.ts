import { fileURLToPath, URL } from 'node:url'
import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

// https://vite.dev/config/
export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  server: {
    port: 5173,
    proxy: {
      // 开发环境把 /api 代理到后端服务。
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
    },
  },
  build: {
    chunkSizeWarningLimit: 600,
    rollupOptions: {
      output: {
        manualChunks(id) {
          if (!id.includes('node_modules')) return
          if (/[\\/]node_modules[\\/](vue|vue-router|pinia)[\\/]/.test(id)) return 'vue-core'
          if (id.includes('node_modules/naive-ui')) return 'naive-ui'
          if (
            id.includes('node_modules/@css-render') ||
            id.includes('node_modules/css-render') ||
            id.includes('node_modules/seemly') ||
            id.includes('node_modules/treemate') ||
            id.includes('node_modules/vooks') ||
            id.includes('node_modules/vueuc')
          ) {
            return 'naive-ui-vendor'
          }
          if (
            id.includes('node_modules/marked') ||
            id.includes('node_modules/dompurify') ||
            id.includes('node_modules/qrcode')
          ) {
            return 'content-tools'
          }
          if (id.includes('node_modules/axios')) return 'http-client'
          return 'vendor'
        },
      },
    },
  },
})
