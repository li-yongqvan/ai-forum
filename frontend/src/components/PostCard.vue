<script setup lang="ts">
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import type { Post } from '../api/types'
import { formatTime } from '../utils/format'
import { hitWindow, splitByTerms } from '../utils/highlight'
import { plainText } from '../utils/md'
import AppIcon from './AppIcon.vue'
import Avatar from './Avatar.vue'

// terms（#78，可选）：搜索词表，由结果页透传，仅用于展示层高亮/命中窗口。
// 未传 ⇒ 分段数组退化为"整段非命中"单一节点，渲染结果与加这个功能之前一致（§7-5）。
const props = defineProps<{ post: Post; terms?: string[] }>()
const emit = defineEmits<{
  (e: 'like', id: number): void
  (e: 'fav', id: number): void
  (e: 'more', post: Post): void
}>()
const router = useRouter()

const liked = computed(() => props.post.viewer?.liked ?? false)
const faved = computed(() => props.post.viewer?.favorited ?? false)
const go = () => router.push(`/post/${props.post.id}`)

const titleSegs = computed(() => splitByTerms(props.post.title, props.terms ?? []))
const abstractSegs = computed(() => {
  const plain = plainText(props.post.content)
  // 短路而非交给 hitWindow + splitByTerms 兜底：那两个函数对空正文返回 **0 段**（splitByTerms 首行），
  // 而改造前的实现是 `{{ plainText(...) }}`，恒有一个（可能为空的）文本节点 ⇒ 少一个节点即违反 §7-5。
  if (!props.terms?.length) return [{ text: plain, hit: false }]
  // X6：正文里定位最先出现的命中词，前后各约 40 字；仅标题命中时窗口不切（回落两行截断）
  const w = hitWindow(plain, props.terms)
  const segs = splitByTerms(w.text, props.terms)
  if (w.headCut) segs.unshift({ text: '…', hit: false })
  if (w.tailCut) segs.push({ text: '…', hit: false })
  return segs
})
</script>

<template>
  <div class="card" @click="go">
    <div class="prow">
      <Avatar :name="post.author_name" :size="34" />
      <div class="pmain">
        <div class="pname">{{ post.author_name }}</div>
        <div class="pmeta">{{ formatTime(post.created_at) }} · {{ post.board_name }}</div>
      </div>
    </div>
    <div class="chips">
      <span v-if="post.is_pinned" class="chip pin">置顶</span>
      <span v-if="post.is_featured" class="chip ess">精华</span>
      <span class="chip">{{ post.board_name }}</span>
      <span v-if="post.topic_name" class="chip"># {{ post.topic_name }}</span>
      <!-- #54 标签胶囊：hash 原生导航 + @click.stop 挡卡片 go（评审 §6.5 用 <a> 非 <button>） -->
      <a
        v-for="t in post.tags ?? []"
        :key="t"
        class="chip tag"
        :href="`#/tag/${encodeURIComponent(t)}`"
        @click.stop
      >#{{ t }}</a>
    </div>
    <!-- #78 高亮：分段数组渲染（禁 v-html，§6-5）。段间不留源码空白以外的字符，
         Vue 的 whitespace:condense 会吃掉换行缩进 ⇒ 无 terms 时就是一个文本节点，与改造前一致。 -->
    <h3 class="ptitle">
      <template v-for="(seg, i) in titleSegs" :key="i">
        <mark v-if="seg.hit">{{ seg.text }}</mark>
        <template v-else>{{ seg.text }}</template>
      </template>
    </h3>
    <p class="pabstract">
      <template v-for="(seg, i) in abstractSegs" :key="i">
        <mark v-if="seg.hit">{{ seg.text }}</mark>
        <template v-else>{{ seg.text }}</template>
      </template>
    </p>
    <div class="pacts" @click.stop>
      <button class="pact" :class="{ on: liked }" @click="emit('like', post.id)">
        <AppIcon name="heart" :size="18" />
        <span class="cnt">{{ post.like_count }}</span>
      </button>
      <button class="pact" @click="go">
        <AppIcon name="comment" :size="18" />
        <span class="cnt">{{ post.comment_count }}</span>
      </button>
      <button class="pact" :class="{ on: faved }" @click="emit('fav', post.id)">
        <AppIcon name="star" :size="18" />
        <span class="cnt">{{ post.favorite_count }}</span>
      </button>
      <button class="pact" @click="emit('more', post)">
        <AppIcon name="more" :size="18" />
      </button>
    </div>
  </div>
</template>

<style scoped>
.card {
  background: var(--surface);
  border-radius: 16px;
  margin: 10px 12px;
  padding: 14px 14px 8px;
  border: 1px solid var(--line);
  transition: transform 0.12s;
}
.card:active {
  transform: scale(0.985);
}
.prow {
  display: flex;
  align-items: center;
  gap: 10px;
}
.pmain {
  flex: 1;
  min-width: 0;
}
.pname {
  font-size: 13.5px;
  font-weight: 600;
}
.pmeta {
  font-size: 11.5px;
  color: var(--ink-2);
  font-variant-numeric: tabular-nums;
}
.chips {
  display: flex;
  gap: 6px;
  flex-wrap: wrap;
  margin: 8px 0 0;
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
.chip.tag {
  cursor: pointer;
}
.ptitle {
  font-size: 15.5px;
  font-weight: 700;
  line-height: 1.42;
  margin: 8px 0 4px;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}
.pabstract {
  font-size: 13px;
  color: var(--ink-2);
  line-height: 1.55;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
  word-break: break-all;
}
.pacts {
  display: flex;
  align-items: center;
  margin-top: 8px;
  border-top: 1px solid var(--line);
  padding-top: 2px;
}
.pact {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 5px;
  height: 44px;
  font-size: 12.5px;
  color: var(--ink-2);
  border-radius: 10px;
}
.pact.on {
  color: var(--accent-strong);
  font-weight: 600;
}
.pact.on:has(svg:first-child) {
  color: var(--danger);
}
.cnt {
  font-variant-numeric: tabular-nums;
}
/* #78 命中段：底色高亮不改字号行高，避免卡片高度抖动 */
.ptitle mark,
.pabstract mark {
  background: var(--accent-soft);
  color: var(--ink);
  border-radius: 3px;
  padding: 0 1px;
}
</style>
