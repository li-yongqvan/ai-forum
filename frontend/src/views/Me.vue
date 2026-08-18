<script setup lang="ts">
// 我的（IA §4：游客原地引导；资料卡 + 帖子/收藏/关注 分段，收藏/关注 待后端）
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import * as api from '../api/content'
import type { UserProfile } from '../api/types'
import { formatTime } from '../utils/format'
import Avatar from '../components/Avatar.vue'
import SegTabs from '../components/SegTabs.vue'
import PostList from '../components/PostList.vue'
import Empty from '../components/Empty.vue'
import LoginGuide from '../components/LoginGuide.vue'
import AppIcon from '../components/AppIcon.vue'

const router = useRouter()
const auth = useAuthStore()
const profile = ref<UserProfile | null>(null)
const tab = ref<'posts' | 'favs' | 'follows'>('posts')

onMounted(async () => {
  if (!auth.isLoggedIn) return
  try {
    profile.value = await api.getUserProfile(auth.user!.id)
  } catch {
    /* 忽略 */
  }
})

const fetcher = (page: number, pageSize: number) =>
  api.listPosts({ authorId: auth.user?.id ?? 0, page, pageSize })
</script>

<template>
  <div class="page">
    <LoginGuide v-if="!auth.isLoggedIn" />

    <template v-else>
      <div class="pcard" v-if="profile">
        <div class="prow">
          <Avatar :name="profile.username" :size="52" />
          <div class="pinfo">
            <div class="pname">{{ profile.username }}</div>
            <div class="pbio">{{ profile.bio || '这个人很懒，什么都没写' }}</div>
            <div class="pjoined">加入于 {{ formatTime(profile.joined_at) }}</div>
          </div>
        </div>
        <div class="stats">
          <div class="stat"><b>{{ profile.post_count }}</b><span>帖子</span></div>
          <div class="stat"><b>{{ profile.follower_count }}</b><span>粉丝</span></div>
          <div class="stat"><b>{{ profile.following_count }}</b><span>关注</span></div>
        </div>
        <div class="entries">
          <button class="entry" @click="router.push('/settings')">
            <AppIcon name="gear" :size="18" />设置
          </button>
        </div>
      </div>

      <SegTabs
        :options="[
          { key: 'posts', label: '帖子' },
          { key: 'favs', label: '收藏' },
          { key: 'follows', label: '关注' },
        ]"
        :model-value="tab"
        @update:model-value="(v: string) => (tab = v as 'posts' | 'favs' | 'follows')"
      />

      <PostList v-if="tab === 'posts'" :fetcher="fetcher" empty-title="还没有发布过帖子" />
      <Empty v-else :title="tab === 'favs' ? '收藏功能即将上线' : '关注列表即将上线'" />
    </template>
  </div>
</template>

<style scoped>
.pcard {
  background: var(--surface);
  border-bottom: 1px solid var(--line);
  padding: 16px;
}
.prow {
  display: flex;
  gap: 12px;
  align-items: flex-start;
}
.pinfo {
  flex: 1;
  min-width: 0;
}
.pname {
  font-size: 17px;
  font-weight: 800;
}
.pbio {
  font-size: 12.5px;
  color: var(--ink-2);
  margin-top: 4px;
}
.pjoined {
  font-size: 11px;
  color: var(--ink-3);
  margin-top: 4px;
}
.stats {
  display: flex;
  gap: 24px;
  margin: 14px 0;
}
.stat {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 2px;
}
.stat b {
  font-size: 16px;
  font-variant-numeric: tabular-nums;
}
.stat span {
  font-size: 11.5px;
  color: var(--ink-2);
}
.entries {
  display: flex;
  gap: 10px;
}
.entry {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  height: 40px;
  padding: 0 16px;
  border-radius: 12px;
  background: var(--surface-2);
  font-size: 13px;
  font-weight: 600;
  color: var(--ink);
}
</style>
