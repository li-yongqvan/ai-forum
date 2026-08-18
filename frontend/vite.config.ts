import { defineConfig } from 'vitest/config'
import vue from '@vitejs/plugin-vue'
import Components from 'unplugin-vue-components/vite'
import { VantResolver } from '@vant/auto-import-resolver'
import { VitePWA } from 'vite-plugin-pwa'

export default defineConfig({
  plugins: [
    vue(),
    // Vant 按需引入（D3：控制包体）
    Components({ resolvers: [VantResolver()] }),
    VitePWA({
      registerType: 'autoUpdate', // D8：SW autoUpdate，避免旧页引用失效 chunk
      manifest: {
        name: 'AI 智联论坛',
        short_name: '智联论坛',
        description: '面向学院师生的 AI 主题社区',
        theme_color: '#1F3A5F',
        background_color: '#F6F4EF',
        display: 'standalone',
        start_url: '/',
        icons: [
          { src: '/icons/pwa-192x192.png', sizes: '192x192', type: 'image/png' },
          { src: '/icons/pwa-512x512.png', sizes: '512x512', type: 'image/png' },
          { src: '/icons/maskable-icon-512x512.png', sizes: '512x512', type: 'image/png', purpose: 'maskable' },
        ],
      },
      workbox: {
        globPatterns: ['**/*.{js,css,html,svg,png,ico}'],
      },
    }),
  ],
  server: {
    proxy: {
      // dev 环境 /api → 本地 Go 后端（D6）
      '/api': 'http://localhost:8080',
    },
  },
  test: {
    environment: 'jsdom',
    globals: true,
  },
})
