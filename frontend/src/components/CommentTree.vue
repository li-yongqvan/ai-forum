<script setup lang="ts">
// 递归评论树（#47 深度截断：默认只显示一级；一级分支可展开/收起；显示深度 3 封顶、更深平铺；
// 软删占位、楼层号；第 3 层及更深无「回复」按钮——源头保证评论 ≤3 层）
import { reactive, watch } from 'vue'
import { useAuthStore } from '../stores/auth'
import type { CommentNode } from '../api/types'
import { formatTime } from '../utils/format'
import { md } from '../utils/md'
import Avatar from './Avatar.vue'

const props = withDefaults(
  defineProps<{
    comments: CommentNode[]
    postId: number
    /** 显示深度：PostDetail 传的根数组为 1，每递归 +1，封顶 3（更深压平到第 3 层）。 */
    depth?: number
    /** 一次性信号：驱动展开指定一级分支（仅根实例接收；父组件收到 handled 后复位）。 */
    expandRootId?: number | null
  }>(),
  { depth: 1, expandRootId: null },
)
const emit = defineEmits<{
  (e: 'reply', commentId: number, author: string): void
  (e: 'report', commentId: number): void
  (e: 'delete', commentId: number): void
  (e: 'expand-root-handled'): void
}>()
const auth = useAuthStore()
// 按一级评论 id 为 key：各一级分支独立展开/收起（D3）
const expanded = reactive(new Set<number>())

function hasReplies(c: CommentNode): boolean {
  return (c.replies?.length ?? 0) > 0
}
/** 全部后代总数（任意深度；软删占位也计，与评论总数 countNodes 口径一致）。 */
function countDesc(c: CommentNode): number {
  return (c.replies ?? []).reduce((n, r) => n + 1 + countDesc(r), 0)
}
/** 一级分支展开/收起切换（仅一级调用）。 */
function toggleBranch(id: number) {
  if (expanded.has(id)) expanded.delete(id)
  else expanded.add(id)
}
function canDelete(c: CommentNode): boolean {
  return auth.user?.id === c.author_id || auth.isMod
}
// 回复/未来通知锚点：驱动展开指定一级分支（一次性；父组件收到 handled 后复位 prop）。
// 依赖 null→rootId 两拍触发（无 immediate，挂载时 expandRootId 为 null 不触发）——勿删父组件的 null 复位步。
// 显式 flush:'pre'（默认即 pre）：revealComment 依赖 pre-flush + 两拍 nextTick 后 query DOM，勿改。
watch(
  () => props.expandRootId,
  (id) => {
    if (id != null) {
      expanded.add(id)
      emit('expand-root-handled')
    }
  },
  { flush: 'pre' },
)
</script>

<template>
  <div class="comments">
    <template v-for="c in comments" :key="c.id">
      <div class="comment" :class="{ 'is-deleted': c.deleted }" :data-comment-id="c.id">
        <template v-if="c.deleted">
          <span class="cdeleted">评论已删除</span>
        </template>
        <template v-else>
          <div class="prow2">
            <Avatar :name="c.author_name" :size="28" />
            <div class="cmain">
              <div class="cmeta-top">
                <span class="cauthor">{{ c.author_name }}</span>
                <span class="ctime">{{ formatTime(c.created_at) }}</span>
                <span v-if="c.floor" class="cfloor">{{ c.floor }}F</span>
              </div>
              <!-- 评论内容走 md 渲染（#12：与正文一致，图片 URL 可渲染；md 已先转义防 XSS） -->
              <!-- eslint-disable-next-line vue/no-v-html -->
              <div class="cbody" v-html="md(c.content)"></div>
              <div class="cactions">
                <button v-if="depth < 3" @click="emit('reply', c.id, c.author_name)">回复</button>
                <button @click="emit('report', c.id)">举报</button>
                <button v-if="canDelete(c)" class="danger" @click="emit('delete', c.id)">删除</button>
              </div>
            </div>
          </div>
        </template>

        <!-- 一级评论：分支展开/收起（默认折叠，按钮显示全部后代数 N） -->
        <template v-if="depth === 1 && hasReplies(c)">
          <!-- 已知限制（评审 F3）：展开态一次性全渲染该分支全部后代（D2/D5 用户确认）；如需「分支内加载更多」在此接缝追加 -->
          <div v-if="expanded.has(c.id)" class="replies">
            <CommentTree
              :comments="c.replies ?? []"
              :post-id="postId"
              :depth="2"
              @reply="(id, name) => emit('reply', id, name)"
              @report="(id) => emit('report', id)"
              @delete="(id) => emit('delete', id)"
            />
          </div>
          <button class="expand" :aria-expanded="expanded.has(c.id)" @click="toggleBranch(c.id)">
            {{ expanded.has(c.id) ? '收起' : `展开 ${countDesc(c)} 条回复` }}
          </button>
        </template>

        <!-- 二级及更深：恒渲染子树；显示深度 3 封顶，第 4 层+ 平铺到第 3 层 -->
        <div v-else-if="hasReplies(c)" :class="depth >= 3 ? 'replies flat' : 'replies'">
          <CommentTree
            :comments="c.replies ?? []"
            :post-id="postId"
            :depth="Math.min(depth + 1, 3)"
            @reply="(id, name) => emit('reply', id, name)"
            @report="(id) => emit('report', id)"
            @delete="(id) => emit('delete', id)"
          />
        </div>
      </div>
    </template>
  </div>
</template>

<style scoped>
.comment {
  padding: 12px 0;
  border-top: 1px solid var(--line);
}
.prow2 {
  display: flex;
  gap: 8px;
}
.cmain {
  flex: 1;
  min-width: 0;
}
.cmeta-top {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 11.5px;
  color: var(--ink-2);
}
.cauthor {
  color: var(--ink);
  font-weight: 600;
}
.cfloor {
  color: var(--ink-3);
  font-variant-numeric: tabular-nums;
}
.cbody {
  font-size: 13.5px;
  line-height: 1.65;
  margin: 4px 0;
  word-break: break-word;
}
.cactions {
  display: flex;
  gap: 14px;
}
.cactions button {
  font-size: 11.5px;
  color: var(--ink-2);
  min-height: 28px;
  display: inline-flex;
  align-items: center;
}
.cactions button.danger {
  color: var(--danger);
}
.cdeleted {
  color: var(--ink-3);
  font-size: 12.5px;
  background: var(--surface-2);
  border-radius: 8px;
  padding: 6px 10px;
  display: inline-block;
}
.replies {
  margin: 4px 0 0 14px;
  padding-left: 12px;
  border-left: 2px solid var(--surface-3);
}
.replies .comment {
  border-top: none;
  padding: 8px 0;
}
/* 压平：第 4 层+ 与第 3 层同缩进（显式写全覆盖 .replies 的 border-left/margin/padding） */
.replies.flat {
  margin: 0;
  padding-left: 0;
  border-left: none;
}
.expand {
  display: flex;
  align-items: center;
  font-size: 12px;
  color: var(--brand-2);
  font-weight: 600;
  min-height: 36px;
  margin-left: 14px;
}
/* 新评论锚点高亮 1.6s（IA §5.2：accent-soft 渐隐） */
.comment-flash {
  animation: comment-flash 1.6s ease;
  border-radius: 10px;
}
@keyframes comment-flash {
  0% {
    background: var(--accent-soft);
  }
  100% {
    background: transparent;
  }
}
</style>
