<script setup lang="ts">
// 通知中心（#32，IA §4/§5.4：通知/私信分段；未读红点 + 全部已读；点击单条跳锚点）
// 当前仅 follow 通知（关注用户 → 被关注者）；like/comment/reply/report_result 类型预留给后续。
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { showToast } from 'vant'
import { useAuthStore } from '../stores/auth'
import { useNotifyStore } from '../stores/notify'
import * as api from '../api/notify'
import type { AppNotification } from '../api/types'
import { formatTime } from '../utils/format'
import Avatar from '../components/Avatar.vue'
import SegTabs from '../components/SegTabs.vue'
import LoginGuide from '../components/LoginGuide.vue'
import Empty from '../components/Empty.vue'

const router = useRouter()
const auth = useAuthStore()
const notify = useNotifyStore()

const tab = ref('notify')
const items = ref<AppNotification[]>([])
const loading = ref(false)
const finished = ref(false)
const PAGE_SIZE = 20

async function load(reset = false) {
  if (loading.value) return
  if (reset) {
    items.value = []
    finished.value = false
  }
  if (finished.value) return
  loading.value = true
  try {
    const page = Math.floor(items.value.length / PAGE_SIZE) + 1
    const data = await api.listNotifications({ page, pageSize: PAGE_SIZE })
    items.value.push(...data.items)
    if (data.items.length < PAGE_SIZE) finished.value = true
  } catch (e) {
    showToast((e as Error).message || '加载失败')
  } finally {
    loading.value = false
  }
}

function onScroll() {
  const el = document.scrollingElement
  if (el && el.scrollTop + el.clientHeight >= el.scrollHeight - 300) load()
}
onMounted(() => {
  load(true)
  notify.refreshUnread()
  window.addEventListener('scroll', onScroll)
})
onUnmounted(() => window.removeEventListener('scroll', onScroll))

// 文案（快照字段由后端算好，前端只拼装）
function textFor(n: AppNotification): string {
  const who = n.actor_name ? `${n.actor_name} ` : ''
  switch (n.type) {
    case 'follow':
      return `${who}关注了你`
    case 'like':
      return `${who}赞了你的帖子`
    case 'comment':
      return `${who}评论了你的帖子`
    case 'reply':
      return `${who}回复了你的评论`
    case 'report_result':
      return n.target_title ? `举报结果：${n.target_title}` : '举报处理结果已更新'
    default:
      return '新通知'
  }
}

// 跳转锚点（IA v2 §4）：follow→用户主页；like/comment/reply→帖子详情；report_result→提示
function linkTo(n: AppNotification): string | null {
  switch (n.type) {
    case 'follow': {
      const id = n.target_id ?? n.actor_id
      return id != null ? `/user/${id}` : null
    }
    case 'like':
    case 'comment':
    case 'reply':
      return n.target_id != null ? `/post/${n.target_id}` : null
    default:
      return null
  }
}

async function onItemClick(n: AppNotification) {
  const to = linkTo(n)
  if (to) router.push(to)
  // 未读 → 先本地置已读（乐观），后台确认
  if (!n.is_read) {
    n.is_read = true
    notify.unread = Math.max(0, notify.unread - 1)
    try {
      await api.markNotificationRead(n.id)
    } catch {
      notify.refreshUnread()
    }
  }
}

async function onMarkAllRead() {
  try {
    await api.markAllRead()
    items.value.forEach((n) => (n.is_read = true))
    notify.clearUnread()
  } catch (e) {
    showToast((e as Error).message || '操作失败')
  }
}

const hasUnread = computed(() => items.value.some((n) => !n.is_read))
</script>

<template>
  <div class="page">
    <LoginGuide v-if="!auth.isLoggedIn" />

    <template v-else>
      <SegTabs
        v-model="tab"
        :options="[
          { key: 'notify', label: '通知' },
          { key: 'msg', label: '私信' },
        ]"
      />

      <div v-if="tab === 'notify'" class="notify">
        <div class="toolbar">
          <button class="all-read" :disabled="!hasUnread" @click="onMarkAllRead">全部已读</button>
        </div>

        <div
          v-for="n in items"
          :key="n.id"
          class="item"
          :class="{ unread: !n.is_read }"
          @click="onItemClick(n)"
        >
          <Avatar :name="n.actor_name || '?'" :size="42" />
          <div class="body">
            <div class="text">{{ textFor(n) }}</div>
            <div class="time">{{ formatTime(n.created_at) }}</div>
          </div>
          <span v-if="!n.is_read" class="dot" aria-label="未读" />
        </div>

        <Empty v-if="!loading && finished && !items.length" title="暂时没有新通知" />
        <div v-if="finished && items.length" class="listend">到底啦</div>
        <div v-if="loading" class="listend">加载中…</div>
      </div>

      <div v-else class="msg">
        <Empty title="私信功能即将上线" desc="私信将在后续版本开放" />
      </div>
    </template>
  </div>
</template>

<style scoped>
.toolbar {
  display: flex;
  justify-content: flex-end;
  padding: 10px 16px 2px;
}
.all-read {
  height: 30px;
  padding: 0 12px;
  border-radius: 8px;
  font-size: 12px;
  font-weight: 600;
  color: var(--accent);
  background: var(--surface-2);
}
.all-read:disabled {
  color: var(--ink-3);
  opacity: 0.6;
}
.item {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 16px;
  border-bottom: 1px solid var(--line);
}
.item:active {
  background: var(--surface-2);
}
.body {
  flex: 1;
  min-width: 0;
}
.text {
  font-size: 14px;
  color: var(--ink);
  line-height: 1.45;
}
.time {
  font-size: 11.5px;
  color: var(--ink-3);
  margin-top: 3px;
}
.dot {
  flex: none;
  width: 9px;
  height: 9px;
  border-radius: 50%;
  background: var(--danger, #ee0a24);
}
.listend {
  text-align: center;
  font-size: 11.5px;
  color: var(--ink-3);
  padding: 18px 0 6px;
}
</style>
