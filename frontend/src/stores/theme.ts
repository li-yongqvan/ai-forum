import { defineStore } from 'pinia'

export type ThemeMode = 'system' | 'light' | 'dark'
const KEY = 'af_theme'

/** 应用主题到 <html data-theme>：system 时移除属性，由 prefers-color-scheme 接管（IA §5.8 三态）。 */
function apply(mode: ThemeMode) {
  const root = document.documentElement
  root.removeAttribute('data-theme')
  if (mode === 'light') root.setAttribute('data-theme', 'light')
  if (mode === 'dark') root.setAttribute('data-theme', 'dark')
}

export const useThemeStore = defineStore('theme', {
  state: () => ({
    mode: (localStorage.getItem(KEY) as ThemeMode) || ('system' as ThemeMode),
  }),
  actions: {
    set(mode: ThemeMode) {
      this.mode = mode
      if (mode === 'system') localStorage.removeItem(KEY)
      else localStorage.setItem(KEY, mode)
      apply(mode)
    },
    init() {
      apply(this.mode)
    },
  },
})
