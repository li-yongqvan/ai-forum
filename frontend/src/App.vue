<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useThemeStore } from './stores/theme'
import { useAuthStore } from './stores/auth'
import AppIcon from './components/AppIcon.vue'

const route = useRoute()
const router = useRouter()
const theme = useThemeStore()
const auth = useAuthStore()

onMounted(() => {
  theme.init()
  auth.fetchMe()
})

const L1 = ['/feed', '/boards', '/notifications', '/me']
const AUTH_PAGES = ['/login', '/register']

const isAuthPage = computed(() => AUTH_PAGES.includes(route.path))
const isL1 = computed(() => L1.includes(route.path))
const title = computed(() => (route.meta.title as string) || 'AI 智联论坛')
// FAB 显隐（IA §5.1）：auth 页隐藏；/write、/post/:id 隐藏（避免遮挡输入栏）；其余 L2 保留
const showFab = computed(() => {
  if (isAuthPage.value) return false
  if (route.path.startsWith('/write') || route.path.startsWith('/post/')) return false
  return true
})
</script>

<template>
  <div class="app-shell">
    <van-nav-bar
      v-if="!isAuthPage"
      :title="title"
      :left-arrow="!isL1"
      fixed
      placeholder
      @click-left="router.back()"
    />

    <main class="view">
      <RouterView />
    </main>

    <van-tabbar v-if="isL1" route fixed placeholder safe-area-inset-bottom>
      <van-tabbar-item replace to="/feed">
        <span>首页</span>
        <template #icon><AppIcon name="home" :size="22" /></template>
      </van-tabbar-item>
      <van-tabbar-item replace to="/boards">
        <span>板块</span>
        <template #icon><AppIcon name="grid" :size="22" /></template>
      </van-tabbar-item>
      <van-tabbar-item replace to="/notifications">
        <span>通知</span>
        <template #icon><AppIcon name="bell" :size="22" /></template>
      </van-tabbar-item>
      <van-tabbar-item replace to="/me">
        <span>我的</span>
        <template #icon><AppIcon name="user" :size="22" /></template>
      </van-tabbar-item>
    </van-tabbar>

    <button v-if="showFab" class="fab" aria-label="发帖" @click="router.push('/write')">
      <AppIcon name="plus" :size="26" />
    </button>
  </div>
</template>

<style scoped>
.app-shell {
  min-height: 100vh;
  min-height: 100dvh;
}
.view {
  padding-top: 0;
  overflow-x: hidden;
}
/* FAB：右下角，避让 Tab 与 Home 条（IA §1.4） */
.fab {
  position: fixed;
  right: 18px;
  bottom: calc(76px + env(safe-area-inset-bottom));
  z-index: 30;
  width: 54px;
  height: 54px;
  border-radius: 50%;
  background: var(--accent-strong);
  color: #fff;
  display: flex;
  align-items: center;
  justify-content: center;
  box-shadow: 0 10px 24px -6px rgba(192, 122, 16, 0.55);
}
.fab:active {
  transform: scale(0.92);
}
</style>
