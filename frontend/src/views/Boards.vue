<script setup lang="ts">
// 板块宫格 / 话题列表（IA §3 Q3）；关注按钮初始态取 viewer.following（v2 §4）
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { showToast } from 'vant'
import { useAuthStore } from '../stores/auth'
import * as api from '../api/content'
import type { Board, Topic } from '../api/types'
import { goLoginWithReturn } from '../router'
import SegTabs from '../components/SegTabs.vue'
import AppIcon from '../components/AppIcon.vue'
import Empty from '../components/Empty.vue'

const router = useRouter()
const auth = useAuthStore()
const tab = ref<'boards' | 'topics'>('boards')
const boards = ref<Board[]>([])
const topics = ref<Topic[]>([])
const loading = ref(false)

onMounted(load)
async function load() {
  loading.value = true
  try {
    const [b, t] = await Promise.all([api.listBoards(), api.listTopics()])
    boards.value = b.items
    topics.value = t.items
  } catch (e) {
    showToast((e as Error).message || '加载失败')
  } finally {
    loading.value = false
  }
}

async function toggleFollow(type: 'board' | 'topic', id: number) {
  if (!auth.isLoggedIn) {
    goLoginWithReturn(router.currentRoute.value.fullPath)
    return
  }
  try {
    await api.follow({ target_type: type, target_id: id })
    if (type === 'board') {
      const b = boards.value.find((x) => x.id === id)
      if (b) b.viewer = { following: true }
    } else {
      const t = topics.value.find((x) => x.id === id)
      if (t) t.viewer = { following: true }
    }
  } catch (e) {
    showToast((e as Error).message || '关注失败')
  }
}
</script>

<template>
  <div class="page">
    <SegTabs
      :options="[
        { key: 'boards', label: '板块' },
        { key: 'topics', label: '话题' },
      ]"
      :model-value="tab"
      @update:model-value="(v: string) => (tab = v as 'boards' | 'topics')"
    />

    <div v-if="tab === 'boards'" class="board-grid">
      <button
        v-for="b in boards"
        :key="b.id"
        class="board-card"
        @click="router.push(`/board/${b.id}`)"
      >
        <div class="bicon"><AppIcon name="chat" :size="20" /></div>
        <div class="binfo">
          <div class="bname">{{ b.name }}</div>
          <div class="bdesc">{{ b.description }}</div>
        </div>
        <span
          class="follow-btn"
          :class="{ on: b.viewer?.following }"
          role="button"
          @click.stop="b.viewer?.following ? void 0 : toggleFollow('board', b.id)"
          >{{ b.viewer?.following ? '已关注' : '关注' }}</span
        >
      </button>
      <Empty v-if="!loading && !boards.length" title="暂无板块" />
    </div>

    <div v-else class="topic-list">
      <button
        v-for="t in topics"
        :key="t.id"
        class="topic-row"
        @click="router.push(`/topic/${t.id}`)"
      >
        <span class="tname"># {{ t.name }}</span>
        <span
          class="follow-chip"
          :class="{ on: t.viewer?.following }"
          @click.stop="t.viewer?.following ? void 0 : toggleFollow('topic', t.id)"
        >
          {{ t.viewer?.following ? '已关注' : '关注' }}
        </span>
      </button>
      <Empty v-if="!loading && !topics.length" title="暂无话题" />
    </div>
  </div>
</template>

<style scoped>
.board-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 10px;
  padding: 12px;
}
.board-card {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 14px;
  background: var(--surface);
  border: 1px solid var(--line);
  border-radius: 16px;
  text-align: left;
  align-items: flex-start;
}
.bicon {
  width: 36px;
  height: 36px;
  border-radius: 12px;
  background: var(--brand-soft);
  color: var(--brand);
  display: flex;
  align-items: center;
  justify-content: center;
}
.bname {
  font-size: 15px;
  font-weight: 700;
}
.bdesc {
  font-size: 11.5px;
  color: var(--ink-2);
  line-height: 1.4;
  min-height: 32px;
}
.follow-btn {
  font-size: 12px;
  font-weight: 600;
  color: var(--brand-2);
  padding: 0 12px;
  height: 32px;
  border-radius: 16px;
  background: var(--brand-soft);
  display: inline-flex;
  align-items: center;
  justify-content: center;
}
.follow-btn.on {
  color: var(--ink-3);
  background: var(--surface-2);
}
.topic-list {
  padding: 12px;
}
.topic-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;
  padding: 14px 16px;
  background: var(--surface);
  border: 1px solid var(--line);
  border-radius: 14px;
  margin-bottom: 8px;
}
.tname {
  font-size: 14.5px;
  font-weight: 600;
}
.follow-chip {
  font-size: 12px;
  font-weight: 600;
  color: var(--brand-2);
  padding: 0 12px;
  height: 30px;
  border-radius: 15px;
  background: var(--brand-soft);
  display: inline-flex;
  align-items: center;
}
.follow-chip.on {
  color: var(--ink-3);
  background: var(--surface-2);
}
</style>
