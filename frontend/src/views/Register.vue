<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { showToast } from 'vant'
import { useAuthStore } from '../stores/auth'

const router = useRouter()
const auth = useAuthStore()
const form = ref({ username: '', email: '', password: '', invite_code: '' })
const loading = ref(false)

async function submit() {
  if (!form.value.username || !form.value.email || !form.value.password || !form.value.invite_code) {
    showToast('请填写全部字段')
    return
  }
  loading.value = true
  try {
    await auth.register(form.value)
    showToast('注册成功')
    router.replace('/feed')
  } catch (e) {
    showToast((e as Error).message || '注册失败')
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="page auth-page">
    <div class="brand">
      <div class="logo">✍️</div>
      <h1>加入智联</h1>
      <p>需要邀请码，由社团管理员发放</p>
    </div>
    <van-form @submit="submit">
      <van-cell-group inset>
        <van-field v-model="form.username" label="用户名" placeholder="登录名 = 展示名" />
        <van-field v-model="form.email" label="邮箱" placeholder="用于账号标识" />
        <van-field v-model="form.password" type="password" label="密码" placeholder="至少 8 位" />
        <van-field v-model="form.invite_code" label="邀请码" placeholder="管理员发放的一次性邀请码" />
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
