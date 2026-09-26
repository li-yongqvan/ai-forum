<script setup lang="ts">
import { ref, computed } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import * as api from '../api/content'
import SegTabs from '../components/SegTabs.vue'
import PostList from '../components/PostList.vue'
import LoginGuide from '../components/LoginGuide.vue'
import AppIcon from '../components/AppIcon.vue'

const router = useRouter()
const auth = useAuthStore()
const tab = ref<'all' | 'hot' | 'follow'>('all')
// 稳定引用（内部读 tab.value）；tab 切换用 :key 重挂载
const fetcher = (page: number, pageSize: number) =>
  api.listPosts({ tab: tab.value, page, pageSize })

const emptyTitle = computed(() =>
  tab.value === 'hot' ? '暂无热门帖子' : '还没有帖子'
)
</script>

<template>
  <div class="page">
    <!-- #78 搜索入口（D8）：静态壳，不可输入，点击进入 /search；置于 SegTabs 之上 -->
    <button class="sentry" @click="router.push('/search')">
      <AppIcon name="search" :size="16" />
      <span>搜索帖子标题或正文</span>
    </button>

    <SegTabs
      :options="[
        { key: 'all', label: '全部' },
        { key: 'hot', label: '热门' },
        { key: 'follow', label: '关注' },
      ]"
      :model-value="tab"
      @update:model-value="(v: string) => (tab = v as 'all' | 'hot' | 'follow')"
    />

    <!-- 游客点关注 tab：原地登录引导（IA §4） -->
    <LoginGuide v-if="tab === 'follow' && !auth.isLoggedIn" />
    <PostList v-else :key="tab" :fetcher="fetcher" :empty-title="emptyTitle" />
  </div>
</template>

<style scoped>
.sentry {
  display: flex;
  align-items: center;
  gap: 7px;
  width: calc(100% - 24px);
  margin: 10px 12px 0;
  padding: 9px 12px;
  border: 1px solid var(--line);
  border-radius: 10px;
  background: var(--surface-2);
  color: var(--ink-3);
  font-size: 13px;
  text-align: left;
}
</style>
