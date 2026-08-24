<script setup lang="ts">
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import type { Post } from '../api/types'
import { formatTime } from '../utils/format'
import { plainText } from '../utils/md'
import AppIcon from './AppIcon.vue'
import Avatar from './Avatar.vue'

const props = defineProps<{ post: Post }>()
const emit = defineEmits<{
  (e: 'like', id: number): void
  (e: 'fav', id: number): void
  (e: 'more', post: Post): void
}>()
const router = useRouter()

const liked = computed(() => props.post.viewer?.liked ?? false)
const faved = computed(() => props.post.viewer?.favorited ?? false)
const go = () => router.push(`/post/${props.post.id}`)
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
    <h3 class="ptitle">{{ post.title }}</h3>
    <p class="pabstract">{{ plainText(post.content) }}</p>
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
</style>
