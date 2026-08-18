<script setup lang="ts">
import { ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { showToast } from 'vant'
import { useAuthStore } from '../stores/auth'
import { safeReturnTo } from '../router'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const username = ref('')
const password = ref('')
const loading = ref(false)

async function submit() {
  if (!username.value.trim() || !password.value) {
    showToast('请输入用户名与密码')
    return
  }
  loading.value = true
  try {
    await auth.login(username.value.trim(), password.value)
    const ret = safeReturnTo(route.query.returnTo)
    router.replace(ret ?? '/feed')
  } catch (e) {
    showToast((e as Error).message || '登录失败')
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="page auth-page">
    <div class="brand">
      <div class="logo">🧠</div>
      <h1>AI 智联论坛</h1>
      <p>AI 学习 · 实践 · 协作 · 活动</p>
    </div>
    <van-form @submit="submit">
      <van-cell-group inset>
        <van-field v-model="username" name="username" label="用户名" placeholder="请输入用户名" />
        <van-field
          v-model="password"
          type="password"
          name="password"
          label="密码"
          placeholder="请输入密码"
        />
      </van-cell-group>
      <div class="submit">
        <van-button round block type="primary" native-type="submit" :loading="loading">登录</van-button>
      </div>
    </van-form>
    <div class="switch">
      还没有账号？
      <router-link to="/register">去注册</router-link>
    </div>
  </div>
</template>

<style scoped>
.auth-page {
  min-height: 100vh;
  padding: 20px 16px 40px;
  display: flex;
  flex-direction: column;
}
.brand {
  text-align: center;
  padding: 48px 0 28px;
}
.logo {
  font-size: 48px;
  margin-bottom: 10px;
}
.brand h1 {
  font-size: 22px;
  font-weight: 800;
  color: var(--brand);
}
.brand p {
  font-size: 13px;
  color: var(--ink-2);
  margin-top: 6px;
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
