// Register.vue #76 open 版：两字段免码注册 + 撞名 409 一键采用/我自己改 + loading 重入守卫（D8）。
// vant 用 partial mock（VantResolver 静态注入组件导入，整体 mock 会让 van-* 变 undefined；
// 只 mock showToast，组件保持真实实现渲染于 jsdom）。
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { ApiError } from '../api/client'

vi.mock('vue-router', () => ({ useRouter: () => ({ replace: vi.fn() }) }))
vi.mock('vant', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vant')>()
  return { ...actual, showToast: vi.fn() }
})
vi.mock('../stores/auth', () => ({ useAuthStore: vi.fn() }))

import { showToast } from 'vant'
import Register from './Register.vue'
import { useAuthStore } from '../stores/auth'

const authMock = vi.mocked(useAuthStore)
const storeRegister = vi.fn()

function mountRegister() {
  authMock.mockReturnValue({ register: storeRegister } as never)
  return mount(Register)
}

function submitOf(wrapper: ReturnType<typeof mountRegister>) {
  return (wrapper.vm as unknown as { submit: () => Promise<void> }).submit
}

function fillForm(wrapper: ReturnType<typeof mountRegister>, username: string, password: string) {
  const vm = wrapper.vm as unknown as { form: { username: string; password: string } }
  vm.form.username = username
  vm.form.password = password
}

describe('Register.vue #76 免码注册', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('两字段载荷提交：不携带 email/invite_code', async () => {
    storeRegister.mockResolvedValue({ token: 'tk', user: {} })
    const wrapper = mountRegister()
    fillForm(wrapper, 'alice', 'secret123')
    await submitOf(wrapper)()
    await flushPromises()
    expect(storeRegister).toHaveBeenCalledTimes(1)
    expect(storeRegister).toHaveBeenCalledWith({ username: 'alice', password: 'secret123' })
  })

  it('撞名 409：内联建议（code=username_taken），不打 toast', async () => {
    storeRegister.mockRejectedValueOnce(new ApiError(409, '用户名已被占用', 'username_taken', 'alice_2'))
    const wrapper = mountRegister()
    fillForm(wrapper, 'alice', 'secret123')
    await submitOf(wrapper)()
    await flushPromises()
    expect(wrapper.text()).toContain('已被占用')
    expect(wrapper.text()).toContain('alice_2')
    expect(showToast).not.toHaveBeenCalled()
  })

  it('一键采用：换名直接重提（以响应最新 suggestion 为准）', async () => {
    storeRegister
      .mockRejectedValueOnce(new ApiError(409, '用户名已被占用', 'username_taken', 'alice_2'))
      .mockResolvedValueOnce({ token: 'tk', user: {} })
    const wrapper = mountRegister()
    fillForm(wrapper, 'alice', 'secret123')
    await submitOf(wrapper)()
    await flushPromises()
    const adopt = wrapper.findAll('button').find((b) => b.text().includes('就用这个名字'))
    expect(adopt).toBeTruthy()
    await adopt!.trigger('click')
    await flushPromises()
    expect(storeRegister).toHaveBeenCalledTimes(2)
    expect(storeRegister).toHaveBeenLastCalledWith({ username: 'alice_2', password: 'secret123' })
    expect(wrapper.text()).not.toContain('就用这个名字')
  })

  it('我自己改：仅收起建议，不重提、不强制采纳', async () => {
    storeRegister.mockRejectedValueOnce(new ApiError(409, '用户名已被占用', 'username_taken', 'alice_2'))
    const wrapper = mountRegister()
    fillForm(wrapper, 'alice', 'secret123')
    await submitOf(wrapper)()
    await flushPromises()
    const dismiss = wrapper.findAll('button').find((b) => b.text().includes('我自己改'))
    expect(dismiss).toBeTruthy()
    await dismiss!.trigger('click')
    await flushPromises()
    expect(storeRegister).toHaveBeenCalledTimes(1)
    expect(wrapper.text()).not.toContain('已被占用')
  })

  it('loading 重入守卫：请求未返回前重复提交不触发第二次请求（D8）', async () => {
    storeRegister.mockReturnValue(new Promise(() => {})) // 永不返回（首个 submit 悬挂中，loading 恒 true）
    const wrapper = mountRegister()
    fillForm(wrapper, 'alice', 'secret123')
    const submit = submitOf(wrapper)
    void submit() // 首次提交：进入请求中
    await submit() // 第二次应被重入守卫拦下（不发起第二次请求）
    await flushPromises()
    expect(storeRegister).toHaveBeenCalledTimes(1)
  })

  it('非撞名错误仍走 toast', async () => {
    storeRegister.mockRejectedValueOnce(new ApiError(500, '服务器内部错误'))
    const wrapper = mountRegister()
    fillForm(wrapper, 'alice', 'secret123')
    await submitOf(wrapper)()
    await flushPromises()
    expect(showToast).toHaveBeenCalledWith('服务器内部错误')
    expect(wrapper.text()).not.toContain('就用这个名字')
  })

  it('字段为空不发起请求', async () => {
    const wrapper = mountRegister()
    fillForm(wrapper, '', '')
    await submitOf(wrapper)()
    await flushPromises()
    expect(storeRegister).not.toHaveBeenCalled()
    expect(showToast).toHaveBeenCalled()
  })
})
