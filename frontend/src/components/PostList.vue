<script setup lang="ts">
// 通用帖子列表：无限滚动 + 乐观赞藏（失败回滚）+ 更多菜单（Feed/板块/话题/用户主页复用）
// 注意：fetcher 需为稳定引用（setup 内定义一次，内部读取参数）；参数变化时由调用方用 :key 重挂载。
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { showConfirmDialog, showToast } from 'vant'
import { useAuthStore } from '../stores/auth'
import * as api from '../api/content'
import type { Post } from '../api/types'
import { goLoginWithReturn } from '../router'
import PostCard from './PostCard.vue'
import Empty from './Empty.vue'

const props = defineProps<{
  fetcher: (page: number, pageSize: number) => Promise<{ items: Post[] }>
  emptyTitle?: string
}>()

const route = useRoute()
const auth = useAuthStore()

const posts = ref<Post[]>([])
const page = ref(1)
const loading = ref(false)
const finished = ref(false)
const PAGE_SIZE = 20

async function load(reset = false) {
  if (loading.value) return
  if (reset) {
    posts.value = []
    page.value = 1
    finished.value = false
  }
  if (finished.value) return
  loading.value = true
  try {
    const data = await props.fetcher(page.value, PAGE_SIZE)
    posts.value.push(...data.items)
    if (data.items.length < PAGE_SIZE) finished.value = true
    page.value++
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
  window.addEventListener('scroll', onScroll)
})
onUnmounted(() => window.removeEventListener('scroll', onScroll))

// ---- 更多菜单 ----
const moreShow = ref(false)
const morePost = ref<Post | null>(null)
const moreActions = computed(() => {
  const p = morePost.value
  if (!p) return []
  const mine = auth.user?.id === p.author_id
  const acts: { name: string; color?: string; key: string }[] = [
    { name: '分享', key: 'share' },
    { name: '举报', key: 'report' },
  ]
  if (mine) acts.push({ name: '删除帖子', color: 'var(--danger)', key: 'delete' })
  if (auth.isMod) {
    if (!mine) acts.push({ name: '管理删除', color: 'var(--danger)', key: 'modDelete' })
    acts.push({ name: p.is_pinned ? '取消置顶' : '设为置顶', key: 'pin' })
    acts.push({ name: p.is_featured ? '取消精华' : '设为精华', key: 'feature' })
  }
  return acts
})

async function onMoreSelect(a: { key: string }) {
  const p = morePost.value
  if (!p) return
  switch (a.key) {
    case 'share':
      try {
        await navigator.clipboard.writeText(`${location.origin}/#/post/${p.id}`)
        showToast('链接已复制')
      } catch {
        showToast(`链接：${location.origin}/#/post/${p.id}`)
      }
      break
    case 'report':
      showToast('举报功能即将上线')
      break
    case 'delete':
    case 'modDelete':
      await confirmDelete(p)
      break
    case 'pin':
      try {
        await api.togglePin(p.id)
        p.is_pinned = !p.is_pinned
      } catch (e) {
        showToast((e as Error).message || '操作失败')
      }
      break
    case 'feature':
      try {
        await api.toggleFeature(p.id)
        p.is_featured = !p.is_featured
      } catch (e) {
        showToast((e as Error).message || '操作失败')
      }
      break
  }
}

async function confirmDelete(p: Post) {
  try {
    await showConfirmDialog({ title: '删除帖子？', message: '删除后不可恢复' })
  } catch {
    return
  }
  try {
    await api.deletePost(p.id)
    posts.value = posts.value.filter((x) => x.id !== p.id)
    showToast('已删除')
  } catch (e) {
    showToast((e as Error).message || '删除失败')
  }
}

// ---- 乐观赞/藏 ----
async function onLike(id: number) {
  if (!auth.isLoggedIn) {
    goLoginWithReturn(route.fullPath)
    return
  }
  const p = posts.value.find((x) => x.id === id)
  if (!p) return
  const was = p.viewer?.liked ?? false
  applyLocal(p, 'liked', !was)
  try {
    if (was) await api.unlike('post', id)
    else await api.like('post', id)
  } catch (e) {
    applyLocal(p, 'liked', was)
    showToast((e as Error).message || '操作失败')
  }
}

async function onFav(id: number) {
  if (!auth.isLoggedIn) {
    goLoginWithReturn(route.fullPath)
    return
  }
  const p = posts.value.find((x) => x.id === id)
  if (!p) return
  const was = p.viewer?.favorited ?? false
  applyLocal(p, 'favorited', !was)
  try {
    if (was) await api.unfavorite(id)
    else await api.favorite(id)
  } catch (e) {
    applyLocal(p, 'favorited', was)
    showToast((e as Error).message || '操作失败')
  }
}

function applyLocal(p: Post, kind: 'liked' | 'favorited', v: boolean) {
  if (!p.viewer) p.viewer = { liked: false, favorited: false, following_author: false }
  p.viewer[kind] = v
  if (kind === 'liked') p.like_count += v ? 1 : -1
  else p.favorite_count += v ? 1 : -1
}

function onMore(p: Post) {
  morePost.value = p
  moreShow.value = true
}
</script>

<template>
  <div>
    <PostCard
      v-for="p in posts"
      :key="p.id"
      :post="p"
      @like="onLike"
      @fav="onFav"
      @more="onMore"
    />
    <Empty v-if="!loading && !posts.length" :title="emptyTitle ?? '还没有帖子'" />
    <div v-if="finished && posts.length" class="listend">到底啦</div>
    <div v-if="loading" class="listend">加载中…</div>

    <van-action-sheet
      v-model:show="moreShow"
      :actions="moreActions"
      cancel-text="取消"
      @select="onMoreSelect"
    />
  </div>
</template>

<style scoped>
.listend {
  text-align: center;
  font-size: 11.5px;
  color: var(--ink-3);
  padding: 18px 0 6px;
}
</style>
