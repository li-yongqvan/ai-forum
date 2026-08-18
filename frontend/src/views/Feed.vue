<script setup lang="ts">
import { ref } from 'vue'
import { useAuthStore } from '../stores/auth'
import * as api from '../api/content'
import SegTabs from '../components/SegTabs.vue'
import PostList from '../components/PostList.vue'
import LoginGuide from '../components/LoginGuide.vue'

const auth = useAuthStore()
const tab = ref<'all' | 'follow'>('all')
// 稳定引用（内部读 tab.value）；tab 切换用 :key 重挂载
const fetcher = (page: number, pageSize: number) =>
  api.listPosts({ tab: tab.value, page, pageSize })
</script>

<template>
  <div class="page">
    <SegTabs
      :options="[
        { key: 'all', label: '全部' },
        { key: 'follow', label: '关注' },
      ]"
      :model-value="tab"
      @update:model-value="(v: string) => (tab = v as 'all' | 'follow')"
    />

    <!-- 游客点关注 tab：原地登录引导（IA §4） -->
    <LoginGuide v-if="tab === 'follow' && !auth.isLoggedIn" />
    <PostList v-else :key="tab" :fetcher="fetcher" empty-title="还没有帖子" />
  </div>
</template>
