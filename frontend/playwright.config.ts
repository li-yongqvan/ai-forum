import { defineConfig } from '@playwright/test'

// E2E 冒烟（testing.md 已确认：不入 CI，本地跑；连本地后端 8080，经 Vite proxy）
export default defineConfig({
  testDir: './e2e',
  timeout: 60_000,
  use: {
    baseURL: 'http://localhost:5173',
    viewport: { width: 390, height: 844 }, // 移动端优先视口
    locale: 'zh-CN',
  },
  webServer: {
    command: 'npm run dev',
    url: 'http://localhost:5173',
    reuseExistingServer: true,
    timeout: 30_000,
  },
})
