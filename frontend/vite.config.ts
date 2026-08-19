import { defineConfig } from 'vitest/config'
import vue from '@vitejs/plugin-vue'
import Components from 'unplugin-vue-components/vite'
import { VantResolver } from '@vant/auto-import-resolver'
import { VitePWA } from 'vite-plugin-pwa'

// dev/e2e 代理目标（#12：图片 URL 带前端 Host，/uploads 也须代理到本地 Go；e2e 可用环境变量指隔离后端）
const apiTarget = process.env.VITE_API_PROXY_TARGET ?? 'http://localhost:8080'

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
    // 绑定所有网卡（手机经局域网 IP 访问，如 http://192.168.0.115:5173）
    host: true,
    proxy: {
      // changeOrigin:false 透传前端 Host（与生产 nginx 的 proxy_set_header Host $host 一致）：
      // 后端拼图片 URL 用 Host，必须保持为访问入口（localhost:5173 / 192.168.0.115:5173），
      // 否则会拼成后端地址（localhost:8080）导致手机端图片 404
      '/api': { target: apiTarget, changeOrigin: false },
      // #12 dev：上传图片 URL 带前端 Host，须一并代理到本地 Go（开发态由 Go 托管静态）
      '/uploads': { target: apiTarget, changeOrigin: false },
    },
  },
  test: {
    environment: 'jsdom',
    globals: true,
    // e2e/ 是 Playwright 冒烟（testing.md §3.2），不得被 vitest 当作单测收集
    exclude: ['e2e/**', 'node_modules/**'],
    // 组件单测会经 unplugin-vue-components 按需引入 Vant 组件样式：
    // 让 Vant 走 Vite 转换（其 es 模块内含 .css import），否则 vite-node 直接载 .css 报 Unknown file extension
    server: { deps: { inline: [/vant/] } },
  },
})
