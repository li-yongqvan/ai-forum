<script setup lang="ts">
// 底 Tab 页（/me、/notifications）游客原地登录引导（IA §4：非跳转）
import { useRouter } from 'vue-router'

const router = useRouter()
</script>

<template>
  <div class="guide">
    <div class="gicon">🔒</div>
    <div class="gtitle">登录后查看</div>
    <div class="gdesc">登录即可查看通知与个人中心</div>
    <button class="btn pri" @click="router.push('/login')">去登录</button>
  </div>
</template>

<style scoped>
.guide {
  padding: 64px 32px;
  text-align: center;
  color: var(--ink-2);
}
.gicon {
  font-size: 44px;
  margin-bottom: 14px;
}
.gtitle {
  font-size: 15px;
  font-weight: 700;
  color: var(--ink);
  margin-bottom: 6px;
}
.gdesc {
  font-size: 12.5px;
  margin-bottom: 20px;
}
.btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  height: 44px;
  padding: 0 24px;
  border-radius: 12px;
  font-size: 14px;
  font-weight: 600;
}
.btn.pri {
  background: var(--brand);
  color: #fff;
}
</style>
