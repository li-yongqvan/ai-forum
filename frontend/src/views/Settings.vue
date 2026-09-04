<script setup lang="ts">
// 设置（v2：公开页，主题三态游客可用；退出登录仅登录态显示）
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import { showConfirmDialog, showToast } from 'vant'
import { useAuthStore } from '../stores/auth'
import { useThemeStore, type ThemeMode } from '../stores/theme'
import AppIcon from '../components/AppIcon.vue'

const router = useRouter()
const auth = useAuthStore()
const theme = useThemeStore()

// #76 open 版：占位邮箱不向本人展示伪值（评审 F4/Q6，不变量 #12）——固定文案替代
const emailLabel = computed(() => {
  const email = auth.user?.email ?? ''
  return email.endsWith('@local.invalid') ? '占位邮箱' : email
})

const THEMES: { key: ThemeMode; label: string; icon: string }[] = [
  { key: 'system', label: '跟随系统', icon: 'sun' },
  { key: 'light', label: '浅色', icon: 'sun' },
  { key: 'dark', label: '深色', icon: 'moon' },
]

async function onLogout() {
  try {
    await showConfirmDialog({ title: '退出登录？' })
  } catch {
    return
  }
  await auth.logout()
  showToast('已退出登录')
  router.replace('/feed')
}
</script>

<template>
  <div class="page">
    <div class="group">
      <div class="gtitle">主题</div>
      <van-radio-group :model-value="theme.mode" @update:model-value="(v: ThemeMode) => theme.set(v)">
        <van-cell-group inset>
          <van-cell v-for="t in THEMES" :key="t.key" :title="t.label" clickable @click="theme.set(t.key)">
            <template #right-icon>
              <van-radio :name="t.key" />
            </template>
          </van-cell>
        </van-cell-group>
      </van-radio-group>
    </div>

    <div class="group" v-if="auth.isLoggedIn">
      <div class="gtitle">账号</div>
      <van-cell-group inset>
        <van-cell :title="auth.user?.username" :label="emailLabel" />
        <van-cell title="退出登录" is-link @click="onLogout">
          <template #icon><AppIcon name="lock" :size="18" /></template>
        </van-cell>
      </van-cell-group>
    </div>

    <div class="version">AI 智联论坛 · MVP</div>
  </div>
</template>

<style scoped>
.group {
  margin-bottom: 16px;
}
.gtitle {
  font-size: 12px;
  color: var(--ink-2);
  padding: 12px 16px 6px;
}
.version {
  text-align: center;
  font-size: 11.5px;
  color: var(--ink-3);
  margin-top: 32px;
}
</style>
