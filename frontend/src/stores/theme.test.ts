import { beforeEach, describe, expect, it } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useThemeStore } from './theme'

describe('theme store（三态，IA §5.8）', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    localStorage.clear()
    document.documentElement.removeAttribute('data-theme')
  })

  it('默认跟随系统：无 data-theme 属性', () => {
    const s = useThemeStore()
    expect(s.mode).toBe('system')
    expect(document.documentElement.hasAttribute('data-theme')).toBe(false)
  })

  it('深色：写属性与 localStorage', () => {
    const s = useThemeStore()
    s.set('dark')
    expect(document.documentElement.getAttribute('data-theme')).toBe('dark')
    expect(localStorage.getItem('af_theme')).toBe('dark')
  })

  it('浅色：写属性', () => {
    const s = useThemeStore()
    s.set('light')
    expect(document.documentElement.getAttribute('data-theme')).toBe('light')
  })

  it('回到跟随系统：移除属性与存储', () => {
    const s = useThemeStore()
    s.set('dark')
    s.set('system')
    expect(document.documentElement.hasAttribute('data-theme')).toBe(false)
    expect(localStorage.getItem('af_theme')).toBeNull()
  })
})
