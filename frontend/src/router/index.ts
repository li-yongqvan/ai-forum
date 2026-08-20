import { createRouter, createWebHashHistory, type RouteLocationRaw } from 'vue-router'
import { useAuthStore } from '../stores/auth'

const router = createRouter({
  history: createWebHashHistory(),
  routes: [
    { path: '/login', component: () => import('../views/Login.vue'), meta: { title: '登录' } },
    { path: '/register', component: () => import('../views/Register.vue'), meta: { title: '注册' } },
    { path: '/feed', component: () => import('../views/Feed.vue'), meta: { title: 'AI 智联论坛' } },
    { path: '/boards', component: () => import('../views/Boards.vue'), meta: { title: '板块' } },
    { path: '/board/:id', component: () => import('../views/BoardDetail.vue'), meta: { title: '板块' } },
    { path: '/topic/:id', component: () => import('../views/TopicDetail.vue'), meta: { title: '话题' } },
    { path: '/post/:id', component: () => import('../views/PostDetail.vue'), meta: { title: '帖子详情' } },
    { path: '/write', component: () => import('../views/Write.vue'), meta: { title: '发帖', requireAuth: true } },
    { path: '/user/:id', component: () => import('../views/UserProfile.vue'), meta: { title: '用户主页' } },
    { path: '/me', component: () => import('../views/Me.vue'), meta: { title: '我的' } },
    { path: '/notifications', component: () => import('../views/Notifications.vue'), meta: { title: '通知' } },
    { path: '/reports', component: () => import('../views/Reports.vue'), meta: { title: '举报管理', requireAuth: true } },
    { path: '/settings', component: () => import('../views/Settings.vue'), meta: { title: '设置' } },
    { path: '/', redirect: '/feed' },
    { path: '/:pathMatch(.*)*', redirect: '/feed' },
  ],
})

// 登录墙（D2/v2）：二级页需登录 → 跳登录带 returnTo；底 Tab 页（/me、/notifications）由页面内原地引导，不入此守卫
router.beforeEach((to) => {
  if (to.meta.requireAuth && !useAuthStore().token) {
    const ret = to.fullPath.startsWith('/') ? to.fullPath : '/feed'
    return { path: '/login', query: { returnTo: ret } }
  }
  return true
})

/** returnTo 仅接受站内路径（v2：防开放重定向）。 */
export function safeReturnTo(raw: unknown): string | null {
  if (typeof raw !== 'string' || !raw.startsWith('/') || raw.startsWith('//')) return null
  return raw
}

/** 跳转登录并携带当前页作为 returnTo。 */
export function goLoginWithReturn(current: RouteLocationRaw): void {
  const to = typeof current === 'string' ? current : (current as { fullPath?: string }).fullPath ?? '/feed'
  router.push({ path: '/login', query: { returnTo: to } })
}

export default router
