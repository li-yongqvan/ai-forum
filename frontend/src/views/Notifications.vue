<script setup lang="ts">
// 通知/私信（IA §4：游客原地引导；数据待 notify 包，先「即将上线」）
import { useAuthStore } from '../stores/auth'
import LoginGuide from '../components/LoginGuide.vue'
import Empty from '../components/Empty.vue'

const auth = useAuthStore()
</script>

<template>
  <div class="page">
    <LoginGuide v-if="!auth.isLoggedIn" />
    <Empty v-else title="通知功能即将上线" desc="点赞、评论、回复与关注通知将在后续开放" />
  </div>
</template>
