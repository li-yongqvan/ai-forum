import { defineStore } from 'pinia'
import * as notifyApi from '../api/notify'

// 通知未读数：底部 Tab badge 的单一来源（进入通知页/标记已读后刷新）。
export const useNotifyStore = defineStore('notify', {
  state: () => ({
    unread: 0,
  }),
  actions: {
    async refreshUnread() {
      try {
        this.unread = (await notifyApi.unreadCount()).count
      } catch {
        this.unread = 0
      }
    },
    clearUnread() {
      this.unread = 0
    },
    decrementUnread() {
      this.unread = Math.max(0, this.unread - 1)
    },
  },
})
