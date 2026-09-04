// Settings.vue #76 不变量 #12：占位邮箱不向本人展示伪值——@local.invalid 显示固定文案「占位邮箱」。
// vant 用 partial mock（VantResolver 静态注入组件导入，整体 mock 会让 van-* 变 undefined）。
import { mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

vi.mock('vue-router', () => ({ useRouter: () => ({ replace: vi.fn(), push: vi.fn() }) }))
vi.mock('vant', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vant')>()
  return { ...actual, showConfirmDialog: vi.fn(), showToast: vi.fn() }
})
vi.mock('../stores/auth', () => ({ useAuthStore: vi.fn() }))
vi.mock('../stores/theme', () => ({
  useThemeStore: () => ({ mode: 'system', set: vi.fn() }),
}))

import Settings from './Settings.vue'
import { useAuthStore } from '../stores/auth'

const authMock = vi.mocked(useAuthStore)

function mountSettings(email: string | null) {
  authMock.mockReturnValue({
    isLoggedIn: !!email,
    user: email ? { id: 1, username: 'alice', email, role: 'user', avatar_url: null } : null,
    logout: vi.fn(),
  } as never)
  return mount(Settings)
}

function accountCellLabel(wrapper: ReturnType<typeof mountSettings>): string | undefined {
  const cell = wrapper.findAllComponents({ name: 'van-cell' }).find((w) => w.props('title') === 'alice')
  return cell?.props('label') as string | undefined
}

describe('Settings.vue #76 占位邮箱隐藏（不变量 #12）', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('@local.invalid → label 显示「占位邮箱」而非伪值', () => {
    const wrapper = mountSettings('u_0123abcd0123abcd0123abcd0123abcd@local.invalid')
    expect(accountCellLabel(wrapper)).toBe('占位邮箱')
  })

  it('真实邮箱原样展示（原版行为不回归）', () => {
    const wrapper = mountSettings('alice@x.edu')
    expect(accountCellLabel(wrapper)).toBe('alice@x.edu')
  })
})
