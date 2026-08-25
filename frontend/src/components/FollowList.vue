<script setup lang="ts" generic="T extends { id: number }">
// 通用关注列表：无限滚动 + 乐观取消关注（行内移除、失败回滚）+ 空态 + per-row in-flight 保护（M4）。
// 行模板由调用方以默认 slot 提供（slot props: row, unfollow, pending）；T 由 fetcher 推导。
// 注意：fetcher 需为稳定引用（setup 内定义一次，内部读取参数）；子分段切换由调用方 v-if 重挂载。
import { onMounted, onUnmounted, reactive, ref, type Ref } from 'vue'
import { showToast } from 'vant'
import type { Page } from '../api/types'
import Empty from './Empty.vue'

const props = defineProps<{
  fetcher: (page: number, pageSize: number) => Promise<Page<T>>
  emptyTitle?: string
  /** 取消关注：返回 promise；乐观移除，失败回滚。 */
  unfollow?: (row: T) => Promise<unknown>
}>()
defineSlots<{ default(props: { row: T; unfollow: () => void; pending: boolean }): any }>()

// 泛型 T 在 ref 中会被 UnwrapRef 重写，用 as Ref<T[]> 保住原类型（rows.value 即 T[]）。
const rows = ref([]) as Ref<T[]>
const page = ref(1)
const loading = ref(false)
const finished = ref(false)
const pending = reactive(new Set<number>()) // in-flight 行 id（M4）
const PAGE_SIZE = 20

async function load(reset = false) {
  if (loading.value) return
  if (reset) {
    rows.value = []
    page.value = 1
    finished.value = false
  }
  if (finished.value) return
  loading.value = true
  try {
    const data = await props.fetcher(page.value, PAGE_SIZE)
    rows.value.push(...data.items)
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

async function unfollowRow(row: T) {
  if (!props.unfollow) return
  if (pending.has(row.id)) return // in-flight：pending 期间忽略重复触发（M4）
  const idx = rows.value.indexOf(row)
  if (idx === -1) return
  pending.add(row.id)
  rows.value.splice(idx, 1) // 乐观移除
  try {
    await props.unfollow(row)
  } catch (e) {
    rows.value.splice(idx, 0, row) // 失败回滚
    showToast((e as Error).message || '取消失败')
  } finally {
    pending.delete(row.id)
  }
}

function unfollowFor(row: T) {
  return () => unfollowRow(row)
}
</script>

<template>
  <div>
    <slot v-for="r in rows" name="default" :row="r" :unfollow="unfollowFor(r)" :pending="pending.has(r.id)" />
    <Empty v-if="!loading && !rows.length" :title="emptyTitle ?? '还没有关注'" />
    <div v-if="finished && rows.length" class="listend">到底啦</div>
    <div v-if="loading" class="listend">加载中…</div>
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
