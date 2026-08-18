<script setup lang="ts">
// 递归评论树（IA §5.2：默认全展开、>3 回复折叠、软删占位、楼层号）
import { reactive } from 'vue'
import { useAuthStore } from '../stores/auth'
import type { CommentNode } from '../api/types'
import { formatTime } from '../utils/format'
import { md } from '../utils/md'
import Avatar from './Avatar.vue'

const props = defineProps<{ comments: CommentNode[]; postId: number }>()
const emit = defineEmits<{
  (e: 'reply', commentId: number, author: string): void
  (e: 'report', commentId: number): void
  (e: 'delete', commentId: number): void
}>()
const auth = useAuthStore()
const expanded = reactive(new Set<number>())

function visibleReplies(c: CommentNode): CommentNode[] {
  const r = c.replies ?? []
  return expanded.has(c.id) ? r : r.slice(0, 3)
}
function hiddenCount(c: CommentNode): number {
  const r = c.replies ?? []
  return expanded.has(c.id) ? 0 : Math.max(0, r.length - 3)
}
function expand(id: number) {
  expanded.add(id)
}
function canDelete(c: CommentNode): boolean {
  return auth.user?.id === c.author_id || auth.isMod
}
</script>

<template>
  <div class="comments">
    <template v-for="c in comments" :key="c.id">
      <div class="comment" :class="{ 'is-deleted': c.deleted }">
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
                <button @click="emit('reply', c.id, c.author_name)">回复</button>
                <button @click="emit('report', c.id)">举报</button>
                <button v-if="canDelete(c)" class="danger" @click="emit('delete', c.id)">删除</button>
              </div>
            </div>
          </div>
        </template>

        <div v-if="c.replies && c.replies.length" class="replies">
          <CommentTree
            :comments="visibleReplies(c)"
            :post-id="postId"
            @reply="(id, name) => emit('reply', id, name)"
            @report="(id) => emit('report', id)"
            @delete="(id) => emit('delete', id)"
          />
          <button v-if="hiddenCount(c) > 0" class="expand" @click="expand(c.id)">
            展开 {{ hiddenCount(c) }} 条回复
          </button>
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
.expand {
  display: flex;
  align-items: center;
  font-size: 12px;
  color: var(--brand-2);
  font-weight: 600;
  min-height: 36px;
  margin-left: 14px;
}
</style>
