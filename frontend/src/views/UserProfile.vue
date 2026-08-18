<script setup lang="ts">
// 用户主页（v2 §4：GET /users/:id 公开资料 + 帖子流 + 关注/私信按钮）
import { computed, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { showToast } from 'vant'
import { useAuthStore } from '../stores/auth'
import * as api from '../api/content'
import type { UserProfile } from '../api/types'
import { goLoginWithReturn } from '../router'
import { formatTime } from '../utils/format'
import Avatar from '../components/Avatar.vue'
import PostList from '../components/PostList.vue'

const route = useRoute()
const auth = useAuthStore()
const id = computed(() => Number(route.params.id))
const profile = ref<UserProfile | null>(null)
const following = ref(false)
const fetcher = (page: number, pageSize: number) => api.listPosts({ authorId: id.value, page, pageSize })

onMounted(async () => {
  try {
    profile.value = await api.getUserProfile(id.value)
    following.value = !!profile.value.viewer?.following
  } catch (e) {
    showToast((e as Error).message || '用户不存在')
  }
})

const isSelf = computed(() => auth.user?.id === id.value)

async function toggleFollow() {
  if (!auth.isLoggedIn) {
    goLoginWithReturn(route.fullPath)
    return
  }
  try {
    if (following.value) await api.unfollow({ target_type: 'user', target_id: id.value })
    else await api.follow({ target_type: 'user', target_id: id.value })
    following.value = !following.value
  } catch (e) {
    showToast((e as Error).message || '操作失败')
  }
}
function onMessage() {
  showToast('私信即将上线')
}
</script>

<template>
  <div class="page">
    <template v-if="profile">
      <div class="pcard">
        <div class="prow">
          <Avatar :name="profile.username" :size="56" />
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
        <div class="pacts" v-if="!isSelf">
          <button class="btn" :class="{ on: following }" @click="toggleFollow">
            {{ following ? '已关注' : '关注' }}
          </button>
          <button class="btn ghost" @click="onMessage">私信</button>
        </div>
      </div>

      <PostList :fetcher="fetcher" empty-title="TA 还没有发布过帖子" />
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
.pacts {
  display: flex;
  gap: 10px;
}
.btn {
  flex: 1;
  height: 40px;
  border-radius: 12px;
  font-size: 14px;
  font-weight: 600;
  background: var(--brand);
  color: #fff;
}
.btn.on {
  background: var(--surface-2);
  color: var(--ink-2);
}
.btn.ghost {
  background: var(--surface-2);
  color: var(--ink);
}
</style>
