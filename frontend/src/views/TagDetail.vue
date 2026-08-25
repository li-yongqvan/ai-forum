<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import PostList from '../components/PostList.vue'
import * as api from '../api/content'

const route = useRoute()
// #54 聚合页标签名（归一化小写）；.trim() 防 `/tag/%20` 解码为空格后守卫失效（评审 F3）
const name = computed(() => String(route.params.name ?? '').trim().toLowerCase())
const fetcher = (page: number, pageSize: number) => {
  if (!name.value) return Promise.resolve({ items: [] }) // 空名守卫：不发请求（评审 F3）
  return api.listPosts({ tag: name.value, page, pageSize })
}
</script>

<template>
  <div class="page">
    <div class="thead">
      <span class="tname"># {{ name }}</span>
    </div>
    <PostList :fetcher="fetcher" empty-title="还没有这个标签的帖子" />
  </div>
</template>

<style scoped>
.thead {
  display: flex;
  align-items: center;
  padding: 12px 16px 0;
}
.tname {
  font-size: 17px;
  font-weight: 800;
}
</style>
