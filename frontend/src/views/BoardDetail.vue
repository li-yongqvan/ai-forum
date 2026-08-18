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
const name = ref('板块')
const following = ref(false)
const fetcher = (page: number, pageSize: number) =>
  api.listPosts({ boardId: id.value, page, pageSize })

onMounted(async () => {
  try {
    const res = await api.listBoards()
    const b = res.items.find((x) => x.id === id.value)
    if (b) {
      name.value = b.name
      following.value = !!b.viewer?.following
    }
  } catch {
    /* 忽略：仅头部展示 */
  }
})

async function toggleFollow() {
  if (!auth.isLoggedIn) {
    goLoginWithReturn(route.fullPath)
    return
  }
  try {
    if (following.value) await api.unfollow({ target_type: 'board', target_id: id.value })
    else await api.follow({ target_type: 'board', target_id: id.value })
    following.value = !following.value
  } catch (e) {
    showToast((e as Error).message || '操作失败')
  }
}
</script>

<template>
  <div class="page">
    <div class="bhead">
      <span class="bname">{{ name }}</span>
      <button class="follow-btn" :class="{ on: following }" @click="toggleFollow">
        {{ following ? '已关注' : '关注' }}
      </button>
    </div>
    <PostList :fetcher="fetcher" empty-title="这个板块还很安静" />
  </div>
</template>

<style scoped>
.bhead {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 16px 0;
}
.bname {
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
