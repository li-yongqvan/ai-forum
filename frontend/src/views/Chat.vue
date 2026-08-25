<script setup lang="ts">
// 私信聊天视图（#59）：双方往来消息 + 底部输入栏。
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { showToast } from 'vant'
import { useAuthStore } from '../stores/auth'
import { useNotifyStore } from '../stores/notify'
import * as api from '../api/notify'
import type { AppMessage } from '../api/types'
import { formatTime } from '../utils/format'
import Avatar from '../components/Avatar.vue'
import LoginGuide from '../components/LoginGuide.vue'
import Empty from '../components/Empty.vue'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const notify = useNotifyStore()

const peerId = computed(() => Number(route.params.peerId))
const messages = ref<AppMessage[]>([])
const peerName = ref('')
const input = ref('')
const loading = ref(false)
const sending = ref(false)
const hasMore = ref(true)
const PAGE_SIZE = 20

async function loadMore(reset = false) {
  if (loading.value || (!hasMore.value && !reset)) return
  if (reset) {
    messages.value = []
    hasMore.value = true
  }
  loading.value = true
  try {
    const page = reset ? 1 : Math.floor(messages.value.length / PAGE_SIZE) + 1
    const data = await api.listConversationMessages(peerId.value, { page, pageSize: PAGE_SIZE })
    if (reset) {
      messages.value = data.items
    } else {
      messages.value.push(...data.items)
    }
    if (data.items.length < PAGE_SIZE) hasMore.value = false
  } catch (e) {
    showToast((e as Error).message || '加载失败')
  } finally {
    loading.value = false
  }
}

async function markRead() {
  try {
    await api.markConversationRead(peerId.value)
    notify.refreshUnread()
  } catch (e) {
    showToast((e as Error).message || '标记已读失败')
  }
}

async function send() {
  const content = input.value.trim()
  if (!content || sending.value) return
  sending.value = true
  try {
    const msg = await api.sendMessage(peerId.value, content)
    messages.value.unshift(msg)
    input.value = ''
  } catch (e) {
    showToast((e as Error).message || '发送失败')
  } finally {
    sending.value = false
  }
}

const groupedMessages = computed(() => {
  return [...messages.value].reverse()
})

function isMine(m: AppMessage) {
  return m.from_user_id === auth.user?.id
}

onMounted(async () => {
  if (!auth.isLoggedIn) return
  await loadMore(true)
  await markRead()
})

const onVisibility = () => {
  if (!document.hidden) markRead()
}
onMounted(() => document.addEventListener('visibilitychange', onVisibility))
onUnmounted(() => document.removeEventListener('visibilitychange', onVisibility))

function onBack() {
  router.back()
}
</script>

<template>
  <div class="page chat">
    <LoginGuide v-if="!auth.isLoggedIn" />

    <template v-else>
      <div class="header">
        <button class="back" @click="onBack">‹</button>
        <span class="title">{{ peerName || '私信' }}</span>
        <span class="spacer" />
      </div>

      <div class="messages">
        <button v-if="hasMore && messages.length" class="more" :disabled="loading" @click="loadMore()">
          {{ loading ? '加载中…' : '查看更早消息' }}
        </button>

        <div
          v-for="m in groupedMessages"
          :key="m.id"
          class="bubble-row"
          :class="{ mine: isMine(m) }"
        >
          <Avatar v-if="!isMine(m)" :name="peerName || '?'" :size="34" />
          <div class="bubble-wrap">
            <div class="bubble">{{ m.content }}</div>
            <div class="meta">{{ formatTime(m.created_at) }}</div>
          </div>
        </div>

        <Empty v-if="!loading && !messages.length" title="还没有消息" desc="发条消息打个招呼吧" />
      </div>

      <div class="composer">
        <input
          v-model="input"
          class="input"
          type="text"
          placeholder="写点什么…"
          maxlength="2000"
          @keyup.enter="send"
        />
        <button class="send" :disabled="!input.trim() || sending" @click="send">发送</button>
      </div>
    </template>
  </div>
</template>

<style scoped>
.chat {
  display: flex;
  flex-direction: column;
  height: 100vh;
  height: 100dvh;
  padding-bottom: env(safe-area-inset-bottom);
}
.header {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 16px;
  border-bottom: 1px solid var(--line);
  background: var(--surface);
}
.back {
  font-size: 26px;
  color: var(--ink);
  background: transparent;
  padding: 0 4px;
}
.title {
  flex: 1;
  text-align: center;
  font-size: 16px;
  font-weight: 600;
  color: var(--ink);
}
.spacer {
  width: 28px;
}
.messages {
  flex: 1;
  overflow-y: auto;
  padding: 12px 16px;
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.more {
  align-self: center;
  font-size: 12px;
  color: var(--accent);
  background: transparent;
  padding: 6px 12px;
}
.bubble-row {
  display: flex;
  align-items: flex-end;
  gap: 8px;
}
.bubble-row.mine {
  flex-direction: row-reverse;
}
.bubble-wrap {
  max-width: 70%;
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.bubble-row.mine .bubble-wrap {
  align-items: flex-end;
}
.bubble {
  padding: 10px 14px;
  border-radius: 16px;
  font-size: 14px;
  line-height: 1.45;
  background: var(--surface-2);
  color: var(--ink);
  word-break: break-word;
}
.bubble-row.mine .bubble {
  background: var(--accent);
  color: #fff;
}
.meta {
  font-size: 10.5px;
  color: var(--ink-3);
}
.composer {
  display: flex;
  gap: 10px;
  padding: 10px 16px calc(10px + env(safe-area-inset-bottom));
  border-top: 1px solid var(--line);
  background: var(--surface);
}
.input {
  flex: 1;
  height: 40px;
  border-radius: 20px;
  padding: 0 16px;
  font-size: 14px;
  background: var(--surface-2);
  color: var(--ink);
  border: none;
}
.send {
  height: 40px;
  padding: 0 18px;
  border-radius: 20px;
  font-size: 14px;
  font-weight: 600;
  color: #fff;
  background: var(--accent);
}
.send:disabled {
  opacity: 0.5;
}
</style>
