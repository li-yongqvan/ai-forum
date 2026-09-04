<script setup lang="ts">
// #76 open 版（一次性改动，随 revert 还原）：免码注册，字段收敛为 用户名+密码。
// 撞名 409（code=username_taken）→ 内联建议名一键采用 / 我自己改（D4）；
// suggestion 每次以响应最新值为准重渲染（Q4 铁律）。
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { showToast } from 'vant'
import { useAuthStore } from '../stores/auth'
import { ApiError } from '../api/client'
import type { RegisterPayload } from '../api/types'

const router = useRouter()
const auth = useAuthStore()
const form = ref<RegisterPayload>({ username: '', password: '' })
const loading = ref(false)
const suggestion = ref('')

async function submit() {
  if (loading.value) return // 重入守卫：封死 :loading 生效前的极短双击窗（D8）
  if (!form.value.username || !form.value.password) {
    showToast('请填写用户名和密码')
    return
  }
  loading.value = true
  try {
    await auth.register({ username: form.value.username, password: form.value.password })
    showToast('注册成功')
    router.replace('/feed')
  } catch (e) {
    if (e instanceof ApiError && e.status === 409 && e.code === 'username_taken' && e.suggestion) {
      suggestion.value = e.suggestion // 内联提示，不打 toast（其余错误仍走 toast）
      return
    }
    showToast((e as Error).message || '注册失败')
  } finally {
    loading.value = false
  }
}

// 一键采用：换名直接重提；若再撞名，以最新响应 suggestion 重渲染
function adoptSuggestion() {
  form.value.username = suggestion.value
  suggestion.value = ''
  submit()
}

// 我自己改：仅收起建议，不强制采纳（D4 退出路径）
function dismissSuggestion() {
  suggestion.value = ''
}
</script>

<template>
  <div class="page auth-page">
    <div class="brand">
      <div class="logo">✍️</div>
      <h1>加入智联</h1>
      <p>免邀请码开放注册，填好即可加入</p>
    </div>
    <van-form @submit="submit">
      <van-cell-group inset>
        <van-field v-model="form.username" label="用户名" placeholder="登录名 = 展示名" />
        <div v-if="suggestion" class="suggest">
          <div class="suggest-text">「{{ form.username }}」已被占用，试试「{{ suggestion }}」？</div>
          <div class="suggest-actions">
            <van-button size="small" type="primary" @click="adoptSuggestion">就用这个名字</van-button>
            <van-button size="small" @click="dismissSuggestion">我自己改</van-button>
          </div>
        </div>
        <van-field v-model="form.password" type="password" label="密码" placeholder="至少 8 位" />
      </van-cell-group>
      <div class="submit">
        <van-button round block type="primary" native-type="submit" :loading="loading">注册</van-button>
      </div>
    </van-form>
    <div class="switch">
      已有账号？
      <router-link to="/login">去登录</router-link>
    </div>
  </div>
</template>

<style scoped>
.auth-page {
  min-height: 100vh;
  padding: 20px 16px 40px;
}
.brand {
  text-align: center;
  padding: 36px 0 24px;
}
.logo {
  font-size: 44px;
  margin-bottom: 8px;
}
.brand h1 {
  font-size: 20px;
  font-weight: 800;
  color: var(--brand);
}
.brand p {
  font-size: 13px;
  color: var(--ink-2);
  margin-top: 6px;
}
.suggest {
  padding: 8px 16px 4px;
}
.suggest-text {
  font-size: 13px;
  color: var(--ink-2);
  line-height: 1.5;
}
.suggest-actions {
  display: flex;
  gap: 8px;
  margin-top: 8px;
}
.submit {
  margin: 24px 16px 0;
}
.switch {
  text-align: center;
  font-size: 13px;
  color: var(--ink-2);
  margin-top: 18px;
}
.switch a {
  color: var(--brand-2);
  font-weight: 600;
}
</style>
