<script setup lang="ts">
// 用户主页（v2 §4：GET /users/:id 公开资料 + 帖子流 + 关注/私信按钮；
// #34：admin 加封禁/解封入口 + 封禁徽标 + 用户举报入口，兑现 IA §5.5 / #33 D3）
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { showToast } from 'vant'
import { useAuthStore } from '../stores/auth'
import * as api from '../api/content'
import { banUser, unbanUser } from '../api/report'
import type { UserProfile } from '../api/types'
import { goLoginWithReturn } from '../router'
import { formatTime } from '../utils/format'
import Avatar from '../components/Avatar.vue'
import PostList from '../components/PostList.vue'
import ReportSheet from '../components/ReportSheet.vue'

const route = useRoute()
const router = useRouter()
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
  if (!auth.isLoggedIn) {
    goLoginWithReturn(route.fullPath)
    return
  }
  router.push(`/messages/${id.value}`)
}

// ---- #34 用户举报入口（复用 ReportSheet，target_type=user；未登录引导登录） ----
const reportShow = ref(false)
function onReport() {
  if (!auth.isLoggedIn) {
    goLoginWithReturn(route.fullPath)
    return
  }
  reportShow.value = true
}

// ---- #34 封禁/解封（仅 admin；原因必填 ≤500） ----
const banReason = ref('')
const banDialog = ref({ show: false, isUnban: false, title: '' })
function openBan(isUnban: boolean) {
  banReason.value = ''
  banDialog.value = { show: true, isUnban, title: isUnban ? '解封用户' : '封禁用户' }
}
/** van-dialog before-close：confirm 时校验原因并提交；空原因/失败时保持弹窗。 */
function beforeBanClose(action: string): boolean | Promise<boolean> {
  if (action !== 'confirm') return true
  const reason = banReason.value.trim()
  if (!reason) {
    showToast('请填写封禁原因')
    return false
  }
  return submitBan(reason)
}
async function submitBan(reason: string): Promise<boolean> {
  try {
    if (banDialog.value.isUnban) await unbanUser(id.value, reason)
    else await banUser(id.value, reason)
    if (profile.value) profile.value.banned = !banDialog.value.isUnban
    showToast(banDialog.value.isUnban ? '已解封' : '已封禁')
    return true
  } catch (e) {
    showToast((e as Error).message || '操作失败')
    return false
  }
}
</script>

<template>
  <div class="page">
    <template v-if="profile">
      <div class="pcard">
        <div class="prow">
          <Avatar :name="profile.username" :size="56" />
          <div class="pinfo">
            <div class="pname">
              {{ profile.username }}
              <span v-if="profile.banned" class="bbadge">该用户已封禁</span>
            </div>
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
          <!-- 举报登录墙：按钮始终显示（非本人），未登录点击引导登录（IA §4 登录墙） -->
          <button class="btn ghost" @click="onReport">举报</button>
          <button v-if="auth.isAdmin" class="btn danger" @click="openBan(!!profile.banned)">
            {{ profile.banned ? '解封' : '封禁' }}
          </button>
        </div>
      </div>

      <PostList :fetcher="fetcher" empty-title="TA 还没有发布过帖子" />

      <ReportSheet v-model:show="reportShow" target-type="user" :target-id="id" />
      <van-dialog
        v-model:show="banDialog.show"
        :title="banDialog.title"
        show-cancel-button
        :before-close="beforeBanClose"
      >
        <div class="ban-reason-wrap">
          <textarea
            v-model="banReason"
            class="ban-reason"
            rows="3"
            maxlength="500"
            placeholder="请输入封禁原因（必填，≤500 字）"
          />
        </div>
      </van-dialog>
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
.bbadge {
  display: inline-block;
  margin-left: 6px;
  padding: 1px 8px;
  border-radius: 8px;
  font-size: 11px;
  font-weight: 600;
  vertical-align: middle;
  color: #fff;
  background: #e64545;
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
  flex-wrap: wrap;
}
.btn {
  flex: 1;
  min-width: 70px;
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
.btn.danger {
  background: #e64545;
  color: #fff;
}
.ban-reason-wrap {
  padding: 12px 16px;
}
.ban-reason {
  width: 100%;
  box-sizing: border-box;
  padding: 10px 12px;
  border: 1px solid var(--line);
  border-radius: 10px;
  background: transparent;
  color: var(--ink);
  font-size: 14px;
  resize: none;
}
</style>
