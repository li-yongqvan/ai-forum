<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { showToast } from 'vant'
import { useAuthStore } from '../stores/auth'
import * as api from '../api/content'
import { goLoginWithReturn } from '../router'
import PostList from '../components/PostList.vue'

const route = useRoute()
const auth = useAuthStore()
const id = computed(() => Number(route.params.id))
const name = ref('话题')
const following = ref(false)
const fetcher = (page: number, pageSize: number) =>
  api.listPosts({ topicId: id.value, page, pageSize })

onMounted(async () => {
  try {
    const res = await api.listTopics()
    const t = res.items.find((x) => x.id === id.value)
    if (t) {
      name.value = t.name
      following.value = !!t.viewer?.following
    }
  } catch {
    /* 忽略 */
  }
})

async function toggleFollow() {
  if (!auth.isLoggedIn) {
    goLoginWithReturn(route.fullPath)
    return
  }
  try {
    if (following.value) await api.unfollow({ target_type: 'topic', target_id: id.value })
    else await api.follow({ target_type: 'topic', target_id: id.value })
    following.value = !following.value
  } catch (e) {
    showToast((e as Error).message || '操作失败')
  }
}
</script>

<template>
  <div class="page">
    <div class="thead">
      <span class="tname"># {{ name }}</span>
      <button class="follow-btn" :class="{ on: following }" @click="toggleFollow">
        {{ following ? '已关注' : '关注' }}
      </button>
    </div>
    <PostList :fetcher="fetcher" empty-title="这个话题还没有讨论" />
  </div>
</template>

<style scoped>
.thead {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 16px 0;
}
.tname {
  font-size: 17px;
  font-weight: 800;
}
.follow-btn {
  font-size: 12.5px;
  font-weight: 600;
  color: var(--brand-2);
  padding: 0 14px;
  height: 32px;
  border-radius: 16px;
  background: var(--brand-soft);
}
.follow-btn.on {
  color: var(--ink-3);
  background: var(--surface-2);
}
</style>
