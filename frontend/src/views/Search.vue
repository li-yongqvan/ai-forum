<script setup lang="ts">
// #78 搜索页（L2）：搜索词放 URL query（`#/search?q=…`），"返回保留查询词"即靠此。
// 游客可搜（D4）：本页无登录守卫。服务端是搜索词规则的唯一权威（§6-1），
// 这里的长度/词数判断只为把"参数不合法"翻译成可读提示，不替代校验。
import { computed, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { showToast } from 'vant'
import * as api from '../api/content'
import PostList from '../components/PostList.vue'
import { deriveTerms } from '../utils/highlight'

const route = useRoute()
const router = useRouter()

const MIN_LEN = 2
const MAX_LEN = 64
const MAX_WORDS = 4

const q = computed(() => String(route.query.q ?? '').trim())
const input = ref(q.value)
const terms = computed(() => deriveTerms(q.value))

// 非空即不合规时的提示（空 q 是"还没搜"，不算错误）
const invalidTip = computed(() => {
  const n = [...q.value].length
  if (!n) return ''
  if (n < MIN_LEN) return '搜索词至少 2 个字'
  if (n > MAX_LEN) return `搜索词最长 ${MAX_LEN} 个字`
  if (terms.value.length > MAX_WORDS) return `最多 ${MAX_WORDS} 个搜索词`
  return ''
})

const emptyTitle = computed(() => invalidTip.value || `没有找到「${q.value}」的相关帖子`)

const total = ref<number | null>(null)

// 稳定引用（内部读 q.value）；搜索词变化由调用方 :key 重挂载（§7-12）
const fetcher = (page: number, pageSize: number) => {
  if (!q.value || invalidTip.value) {
    total.value = null
    return Promise.resolve({ items: [] })
  }
  return api.listPosts({ q: q.value, page, pageSize }).then((res) => {
    if (page === 1) total.value = res.total ?? null
    return res
  })
}

function search() {
  const v = input.value.trim()
  if (!v) {
    showToast('请输入搜索词')
    return
  }
  router.replace({ path: '/search', query: { q: v } })
}
</script>

<template>
  <div class="page">
    <van-search
      v-model="input"
      shape="round"
      placeholder="搜索帖子标题或正文"
      enterkeyhint="search"
      @search="search"
    >
      <template #action>
        <div class="gobtn" @click="search">搜索</div>
      </template>
    </van-search>

    <div v-if="total !== null && total > 0" class="count">找到 {{ total }} 条结果</div>
    <PostList :key="q" :fetcher="fetcher" :terms="terms" :empty-title="emptyTitle" />
  </div>
</template>

<style scoped>
.count {
  font-size: 12.5px;
  color: var(--ink-2);
  padding: 0 16px 2px;
  font-variant-numeric: tabular-nums;
}
.gobtn {
  padding: 0 12px;
  font-size: 14px;
  color: var(--brand);
}
</style>
