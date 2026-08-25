<script setup lang="ts">
// 帖子详情 + 评论树 + 赞藏/分享/举报(stub) + 底部评论栏（回复 @ 态）
import { computed, nextTick, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { showConfirmDialog, showToast } from 'vant'
import { useAuthStore } from '../stores/auth'
import * as api from '../api/content'
import type { CommentNode, Post } from '../api/types'
import { goLoginWithReturn } from '../router'
import { formatTime } from '../utils/format'
import { md } from '../utils/md'
import { handleContentImageClick } from '../utils/imagePreview'
import { insertAtCursor } from '../utils/editor'
import AppIcon from '../components/AppIcon.vue'
import Avatar from '../components/Avatar.vue'
import CommentTree from '../components/CommentTree.vue'
import Empty from '../components/Empty.vue'
import ImagePicker from '../components/ImagePicker.vue'
import ReportSheet from '../components/ReportSheet.vue'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const id = computed(() => Number(route.params.id))

const post = ref<Post | null>(null)
const tree = ref<CommentNode[]>([])
const commentCount = ref(0)

const inputText = ref('')
const replyTo = ref<{ id: number; name: string } | null>(null)
const inputEl = ref<HTMLInputElement | null>(null)
const expandRootId = ref<number | null>(null)

function countNodes(nodes: CommentNode[]): number {
  return nodes.reduce((n, c) => n + 1 + countNodes(c.replies ?? []), 0)
}
/** 在树中定位 targetId 所属一级分支的根 id（沿祖先链到 parent_id 为 null 的节点）。 */
function findBranchRoot(nodes: CommentNode[], targetId: number): number | null {
  for (const c of nodes) {
    if (c.id === targetId) return c.id
    if (findBranchRoot(c.replies ?? [], targetId) != null) return c.id
  }
  return null
}
/** 在树中按 id 找节点（兜底取父节点末子用）。 */
function findNode(nodes: CommentNode[], targetId: number): CommentNode | null {
  for (const c of nodes) {
    if (c.id === targetId) return c
    const hit = findNode(c.replies ?? [], targetId)
    if (hit) return hit
  }
  return null
}

async function loadComments() {
  try {
    const t = await api.getComments(id.value)
    // 后端契约修复后空树恒为 []；此处 `?? []` 兜底旧后端/异常返回 null，防 v-if="tree.length" 与 reduce 崩溃
    tree.value = t.comments ?? []
    commentCount.value = countNodes(t.comments ?? [])
  } catch (e) {
    showToast((e as Error).message || '评论加载失败')
  }
}

onMounted(async () => {
  try {
    post.value = await api.getPost(id.value)
  } catch (e) {
    showToast((e as Error).message || '帖子不存在')
    router.replace('/feed')
    return
  }
  loadComments()
})

// ---- 赞 / 藏 ----
function setViewer(kind: 'liked' | 'favorited', v: boolean) {
  const p = post.value
  if (!p) return
  if (!p.viewer) p.viewer = { liked: false, favorited: false, following_author: false }
  p.viewer[kind] = v
}
async function toggleLike() {
  if (!auth.isLoggedIn) {
    goLoginWithReturn(route.fullPath)
    return
  }
  const p = post.value
  if (!p) return
  const was = p.viewer?.liked ?? false
  setViewer('liked', !was)
  p.like_count += was ? -1 : 1
  try {
    if (was) await api.unlike('post', p.id)
    else await api.like('post', p.id)
  } catch (e) {
    setViewer('liked', was)
    p.like_count += was ? 1 : -1
    showToast((e as Error).message || '操作失败')
  }
}
async function toggleFav() {
  if (!auth.isLoggedIn) {
    goLoginWithReturn(route.fullPath)
    return
  }
  const p = post.value
  if (!p) return
  const was = p.viewer?.favorited ?? false
  setViewer('favorited', !was)
  p.favorite_count += was ? -1 : 1
  try {
    if (was) await api.unfavorite(p.id)
    else await api.favorite(p.id)
  } catch (e) {
    setViewer('favorited', was)
    p.favorite_count += was ? 1 : -1
    showToast((e as Error).message || '操作失败')
  }
}
async function share() {
  const url = `${location.origin}/#/post/${id.value}`
  // 原生分享优先（移动端系统分享面板）；不支持或失败回退复制链接
  if (navigator.share) {
    try {
      await navigator.share({ title: post.value?.title ?? document.title, url })
      return
    } catch (err) {
      // 用户取消分享（AbortError）不算失败：静默结束，不弹提示
      if ((err as DOMException)?.name === 'AbortError') return
    }
  }
  try {
    navigator.clipboard.writeText(url)
    showToast('链接已复制')
  } catch {
    showToast(`链接：${url}`)
  }
}

// ---- 管理操作（角色显隐，v2 §3.1 轻量治理） ----
const moreShow = ref(false)
// ---- 举报（#33：帖子/评论入口） ----
const reportShow = ref(false)
const reportTarget = ref<{ type: 'post' | 'comment'; id: number } | null>(null)
const moreActions = computed(() => {
  const p = post.value
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
function onMore() {
  moreShow.value = true
}
async function handleMore(a: { key: string }) {
  const p = post.value
  if (!p) return
  switch (a.key) {
    case 'share':
      share()
      break
    case 'report':
      if (!auth.isLoggedIn) {
        goLoginWithReturn(route.fullPath)
        break
      }
      moreShow.value = false
      reportTarget.value = { type: 'post', id: p.id }
      reportShow.value = true
      break
    case 'delete':
    case 'modDelete':
      try {
        await showConfirmDialog({ title: '删除帖子？', message: '删除后不可恢复' })
      } catch {
        return
      }
      try {
        await api.deletePost(p.id)
        showToast('已删除')
        router.replace('/feed')
      } catch (e) {
        showToast((e as Error).message || '删除失败')
      }
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

// ---- 评论 ----
function onReply(cid: number, name: string) {
  if (!auth.isLoggedIn) {
    goLoginWithReturn(route.fullPath)
    return
  }
  replyTo.value = { id: cid, name }
  inputText.value = `回复 @${name}：`
  nextTick(() => inputEl.value?.focus())
}
// 评论举报入口（#33）
function onReportComment(cid: number) {
  if (!auth.isLoggedIn) {
    goLoginWithReturn(route.fullPath)
    return
  }
  reportTarget.value = { type: 'comment', id: cid }
  reportShow.value = true
}
async function onDeleteComment(cid: number) {
  try {
    await showConfirmDialog({ title: '删除评论？', message: '将以「评论已删除」占位保留回复链' })
  } catch {
    return
  }
  try {
    await api.deleteComment(cid)
    loadComments()
  } catch (e) {
    showToast((e as Error).message || '删除失败')
  }
}
async function sendComment() {
  if (!auth.isLoggedIn) {
    goLoginWithReturn(route.fullPath)
    return
  }
  const text = inputText.value.trim()
  if (!text) {
    showToast('先说点什么吧')
    return
  }
  const parentId = replyTo.value?.id ?? null
  try {
    const res = (await api.createComment({
      post_id: id.value,
      parent_id: parentId,
      content: text,
    })) as { id?: number } | null
    let newId = res?.id
    inputText.value = ''
    replyTo.value = null
    showToast('评论已发布')
    await loadComments()
    // 双路径兜底（评审 F1）：res.id 缺失时——顶层取 tree 末元素（floor 升序，最新在末尾）、
    // 回复取父节点末子（created_at 升序，最新在末尾）
    if (newId == null && tree.value.length) {
      if (parentId == null) {
        newId = tree.value[tree.value.length - 1].id
      } else {
        const kids = findNode(tree.value, parentId)?.replies ?? []
        if (kids.length) newId = kids[kids.length - 1].id
      }
    }
    await revealComment(newId, parentId)
  } catch (e) {
    showToast((e as Error).message || '评论失败')
  }
}

// 回复后：展开所在分支 + 滚动定位新回复（D4）。
// 依赖 expandRootId 的 pre-flush watcher + 两拍 nextTick 后 query DOM——勿改 flush 时机。
async function revealComment(newId: number | undefined, parentId: number | null) {
  if (newId == null) return
  await nextTick() // 渲染 reload 后的新树
  if (parentId != null) {
    const rootId = findBranchRoot(tree.value, parentId)
    if (rootId != null) {
      expandRootId.value = null // 先复位：保证 null→rootId 变更必触发 watcher（同分支二次回复）
      expandRootId.value = rootId
    }
  }
  await nextTick() // 渲染展开后的分支
  const el = document.querySelector(`[data-comment-id="${newId}"]`)
  if (el) {
    el.scrollIntoView({ block: 'center' })
    el.classList.add('comment-flash')
    setTimeout(() => el.classList.remove('comment-flash'), 1600)
  } else {
    console.warn(`[PostDetail] 未找到新评论元素 data-comment-id=${newId}`)
  }
}

function onExpandRootHandled() {
  expandRootId.value = null
}

// 图片按钮的登录墙守卫（#9 §4：游客点击跳登录，返回后回到本帖）
function guardUpload(): boolean {
  if (auth.isLoggedIn) return true
  goLoginWithReturn(route.fullPath)
  return false
}

// 图片上传成功：把 URL 插入评论光标处（评论为单行输入，不补换行）
function onImage(url: string) {
  const el = inputEl.value
  const start = el?.selectionStart ?? inputText.value.length
  const end = el?.selectionEnd ?? start
  const { value, cursor } = insertAtCursor(inputText.value, start, end, url)
  inputText.value = value
  nextTick(() => {
    el?.focus()
    el?.setSelectionRange(cursor, cursor)
  })
}
</script>

<template>
  <div class="page detail" v-if="post">
    <div class="dhead">
      <Avatar :name="post.author_name" :size="38" />
      <div class="dauthor">
        <div class="daname">{{ post.author_name }}</div>
        <div class="dmeta">{{ formatTime(post.created_at) }} · {{ post.board_name }}</div>
      </div>
      <button class="dmore" @click="onMore"><AppIcon name="more" :size="20" /></button>
    </div>

    <div class="chips">
      <span v-if="post.is_pinned" class="chip pin">置顶</span>
      <span v-if="post.is_featured" class="chip ess">精华</span>
      <span class="chip">{{ post.board_name }}</span>
      <span v-if="post.topic_name" class="chip"># {{ post.topic_name }}</span>
    </div>

    <h1 class="dtitle">{{ post.title }}</h1>
    <!-- eslint-disable-next-line vue/no-v-html -->
    <div v-html="md(post.content)" @click="handleContentImageClick"></div>

    <div class="dacts">
      <button class="dact" :class="{ on: post.viewer?.liked }" @click="toggleLike">
        <AppIcon name="heart" :size="18" />
        <span>{{ post.like_count }}</span>
      </button>
      <button class="dact" :class="{ on: post.viewer?.favorited }" @click="toggleFav">
        <AppIcon name="star" :size="18" />
        <span>{{ post.favorite_count }}</span>
      </button>
      <button class="dact" @click="share"><AppIcon name="share" :size="18" /><span>分享</span></button>
    </div>

    <div class="chead">评论 {{ commentCount }}</div>
    <CommentTree
      v-if="tree.length"
      :comments="tree"
      :post-id="post.id"
      :expand-root-id="expandRootId"
      @reply="onReply"
      @report="onReportComment"
      @delete="onDeleteComment"
      @expand-root-handled="onExpandRootHandled"
    />
    <Empty v-else title="还没有评论" desc="来抢沙发" />

    <van-action-sheet
      v-model:show="moreShow"
      :actions="moreActions"
      cancel-text="取消"
      @select="handleMore"
    />
    <ReportSheet
      v-model:show="reportShow"
      :target-type="reportTarget?.type ?? 'post'"
      :target-id="reportTarget?.id ?? 0"
    />

    <!-- 底部评论栏 -->
    <div class="dbar">
      <div class="reply-tag" v-if="replyTo">
        回复 @{{ replyTo.name }}
        <button @click="replyTo = null">✕</button>
      </div>
      <ImagePicker :before-open="guardUpload" @uploaded="onImage" />
      <input
        ref="inputEl"
        v-model="inputText"
        class="cinput"
        :placeholder="auth.isLoggedIn ? '写下你的评论…' : '登录后参与评论'"
        @focus="!auth.isLoggedIn && goLoginWithReturn(route.fullPath)"
      />
      <button class="sbtn" :class="{ disabled: !inputText.trim() }" @click="sendComment">
        <AppIcon name="send" :size="18" />
      </button>
    </div>
  </div>
</template>

<style scoped>
.detail {
  padding: 14px 16px 90px;
}
.dhead {
  display: flex;
  align-items: center;
  gap: 10px;
}
.dauthor {
  flex: 1;
}
.daname {
  font-size: 13.5px;
  font-weight: 600;
}
.dmeta {
  font-size: 11.5px;
  color: var(--ink-2);
}
.dmore {
  width: 44px;
  height: 44px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--ink-2);
  border-radius: 12px;
}
.chips {
  display: flex;
  gap: 6px;
  flex-wrap: wrap;
  margin: 12px 0 0;
}
.chip {
  font-size: 10.5px;
  font-weight: 600;
  padding: 3px 8px;
  border-radius: 7px;
  background: var(--brand-soft);
  color: var(--brand);
}
.chip.pin {
  background: var(--accent-soft);
  color: var(--accent-ink);
}
.chip.ess {
  background: #f3e4f0;
  color: #8a3b7e;
}
.dtitle {
  font-size: 20px;
  font-weight: 800;
  line-height: 1.4;
  margin: 10px 0 4px;
}
.dacts {
  display: flex;
  gap: 10px;
  margin: 16px 0 4px;
}
.dact {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  height: 42px;
  border-radius: 12px;
  background: var(--surface-2);
  font-size: 13px;
  font-weight: 600;
  color: var(--ink-2);
}
.dact.on {
  color: var(--accent-strong);
}
.dact.on:has(svg) {
  color: var(--danger);
}
.chead {
  font-size: 14px;
  font-weight: 700;
  margin: 18px 0 4px;
  font-variant-numeric: tabular-nums;
}
.dbar {
  position: fixed;
  left: 0;
  right: 0;
  bottom: 0;
  margin: 0 auto;
  max-width: 430px;
  z-index: 20;
  background: var(--surface);
  border-top: 1px solid var(--line);
  padding: 8px 12px calc(8px + env(safe-area-inset-bottom));
  display: flex;
  gap: 8px;
  align-items: center;
  flex-wrap: wrap;
}
.reply-tag {
  width: 100%;
  font-size: 12px;
  color: var(--brand-2);
  font-weight: 600;
  display: flex;
  align-items: center;
  gap: 6px;
}
.reply-tag button {
  color: var(--ink-3);
}
.cinput {
  flex: 1;
  min-width: 0;
  height: 40px;
  background: var(--surface-2);
  border-radius: 12px;
  padding: 0 14px;
  font-size: 13.5px;
  border: none;
  outline: none;
  color: var(--ink);
}
.cinput::placeholder {
  color: var(--ink-3);
}
.sbtn {
  width: 40px;
  height: 40px;
  border-radius: 12px;
  background: var(--brand);
  color: #fff;
  display: flex;
  align-items: center;
  justify-content: center;
  flex: none;
}
.sbtn.disabled {
  opacity: 0.4;
}
</style>
