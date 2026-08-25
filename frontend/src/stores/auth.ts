import { defineStore } from 'pinia'
import { getToken, setToken, setAuthFailureHandler } from '../api/client'
import * as authApi from '../api/auth'
import type { AuthUser } from '../api/types'

// #34（评审 F1）：注册会话失效钩子——401 / 403 封禁统一经 client 触发，清 Pinia store，
// 实现「封禁即登出」（仅清 localStorage 不清 store 会导致 isLoggedIn 仍 true）。
setAuthFailureHandler(() => useAuthStore().clear())

export const useAuthStore = defineStore('auth', {
  state: () => ({
    token: getToken() as string | null,
    user: null as AuthUser | null,
  }),
  getters: {
    isLoggedIn: (s) => !!s.token,
    isMod: (s) => s.user?.role === 'moderator' || s.user?.role === 'admin',
    isAdmin: (s) => s.user?.role === 'admin',
  },
  actions: {
    async login(username: string, password: string) {
      const res = await authApi.login(username, password)
      this.setSession(res.token, res.user)
      return res
    },
    async register(payload: {
      username: string
      email: string
      password: string
      invite_code: string
    }) {
      const res = await authApi.register(payload)
      this.setSession(res.token, res.user)
      return res
    },
    /** 启动/路由守卫时拉取当前用户（token 失效则清除并视为游客）。 */
    async fetchMe(): Promise<AuthUser | null> {
      if (!this.token) return null
      try {
        this.user = await authApi.me()
        return this.user
      } catch {
        this.clear()
        return null
      }
    },
    setSession(token: string, user: AuthUser) {
      this.token = token
      this.user = user
      setToken(token)
    },
    async logout() {
      try {
        await authApi.logout()
      } catch {
        /* JWT 无状态，注销空实现；本地清 token 为主 */
      }
      this.clear()
    },
    clear() {
      this.token = null
      this.user = null
      setToken(null)
    },
  },
})
