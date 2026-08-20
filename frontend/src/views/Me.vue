<script setup lang="ts">
// 我的（IA §4：游客原地引导；资料卡 + 帖子/收藏/关注 三段 + 关注内嵌 用户/板块/话题 子分段，#23）
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import * as api from '../api/content'
import * as reportApi from '../api/report'
import type { Board, FollowedUser, Topic, UserProfile } from '../api/types'
import { formatTime } from '../utils/format'
import Avatar from '../components/Avatar.vue'
import SegTabs from '../components/SegTabs.vue'
import PostList from '../components/PostList.vue'
import FollowList from '../components/FollowList.vue'
import LoginGuide from '../components/LoginGuide.vue'
import AppIcon from '../components/AppIcon.vue'

const router = useRouter()
const auth = useAuthStore()
const profile = ref<UserProfile | null>(null)
const pendingReports = ref(0)
const tab = ref<'posts' | 'favs' | 'follows'>('posts')
const followTab = ref<'users' | 'boards' | 'topics'>('users')

onMounted(async () => {
  if (!auth.isLoggedIn) return
  try {
    profile.value = await api.getUserProfile(auth.user!.id)
  } catch {
    /* 忽略 */
  }
  // 治理入口徽章（#33：mod 待处理举报数）
  if (auth.isMod) {
    try {
      pendingReports.value = (await reportApi.countReports()).count
    } catch {
      /* 忽略 */
    }
  }
})

// 稳定 fetcher（内部读 auth/固定 target_type）；tab/子分段切换由分支 v-if 重挂载
const myPosts = (page: number, pageSize: number) =>
  api.listPosts({ authorId: auth.user?.id ?? 0, page, pageSize })
const myFavs = (page: number, pageSize: number) => api.listFavorites({ page, pageSize })
const myFollowedUsers = (page: number, pageSize: number) =>
  api.listFollows<FollowedUser>('user', { page, pageSize })
const myFollowedBoards = (page: number, pageSize: number) =>
  api.listFollows<Board>('board', { page, pageSize })
const myFollowedTopics = (page: number, pageSize: number) =>
  api.listFollows<Topic>('topic', { page, pageSize })

const unfollowUser = (row: FollowedUser) => api.unfollow({ target_type: 'user', target_id: row.id })
const unfollowBoard = (row: Board) => api.unfollow({ target_type: 'board', target_id: row.id })
const unfollowTopic = (row: Topic) => api.unfollow({ target_type: 'topic', target_id: row.id })
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
          <button v-if="auth.isMod" class="entry" @click="router.push('/reports')">
            <AppIcon name="shield" :size="18" />举报处理
            <span v-if="pendingReports > 0" class="cnt">{{ pendingReports }}</span>
          </button>
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

      <PostList v-if="tab === 'posts'" :fetcher="myPosts" empty-title="还没有发布过帖子" />
      <!-- 收藏列表复用 PostList：取消收藏乐观切换 viewer.favorited；行保留至重挂载（M3 已认可的取舍） -->
      <PostList v-else-if="tab === 'favs'" :fetcher="myFavs" empty-title="还没有收藏帖子" />

      <template v-else-if="tab === 'follows'">
        <SegTabs
          :options="[
            { key: 'users', label: '用户' },
            { key: 'boards', label: '板块' },
            { key: 'topics', label: '话题' },
          ]"
          :model-value="followTab"
          @update:model-value="(v: string) => (followTab = v as 'users' | 'boards' | 'topics')"
        />

        <FollowList v-if="followTab === 'users'" :fetcher="myFollowedUsers" :unfollow="unfollowUser" empty-title="还没有关注用户">
          <template #default="{ row, unfollow, pending }">
            <div class="f-row" @click="router.push(`/user/${row.id}`)">
              <Avatar :name="row.username" :size="40" />
              <div class="finfo">
                <div class="fname">{{ row.username }}</div>
                <div class="fsub">{{ row.bio || '这个人很懒，什么都没写' }}</div>
              </div>
              <span class="fchip" :class="{ disabled: pending }" @click.stop="!pending && unfollow()">已关注</span>
            </div>
          </template>
        </FollowList>

        <FollowList v-else-if="followTab === 'boards'" :fetcher="myFollowedBoards" :unfollow="unfollowBoard" empty-title="还没有关注板块">
          <template #default="{ row, unfollow, pending }">
            <div class="f-row" @click="router.push(`/board/${row.id}`)">
              <div class="finfo">
                <div class="fname">{{ row.name }}</div>
                <div class="fsub">{{ row.description || '暂无简介' }}</div>
              </div>
              <span class="fchip" :class="{ disabled: pending }" @click.stop="!pending && unfollow()">已关注</span>
            </div>
          </template>
        </FollowList>

        <FollowList v-else :fetcher="myFollowedTopics" :unfollow="unfollowTopic" empty-title="还没有关注话题">
          <template #default="{ row, unfollow, pending }">
            <div class="f-row" @click="router.push(`/topic/${row.id}`)">
              <div class="finfo">
                <div class="fname"># {{ row.name }}</div>
              </div>
              <span class="fchip" :class="{ disabled: pending }" @click.stop="!pending && unfollow()">已关注</span>
            </div>
          </template>
        </FollowList>
      </template>
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
.cnt {
  min-width: 18px;
  height: 18px;
  padding: 0 5px;
  border-radius: 9px;
  background: var(--danger);
  color: #fff;
  font-size: 11px;
  font-weight: 700;
  display: inline-flex;
  align-items: center;
  justify-content: center;
}

/* 关注行（仿 Boards.vue topic-row/follow-chip） */
.f-row {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 12px 16px;
}
.finfo {
  flex: 1;
  min-width: 0;
}
.fname {
  font-size: 14.5px;
  font-weight: 600;
}
.fsub {
  font-size: 12px;
  color: var(--ink-2);
  margin-top: 2px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.fchip {
  flex-shrink: 0;
  height: 28px;
  padding: 0 12px;
  border-radius: 14px;
  font-size: 12px;
  font-weight: 600;
  display: inline-flex;
  align-items: center;
  color: var(--brand);
  background: color-mix(in srgb, var(--brand) 12%, transparent);
}
.fchip.disabled {
  opacity: 0.5;
  pointer-events: none;
}
</style>
