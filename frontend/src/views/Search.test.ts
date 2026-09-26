// #78 搜索页单测（设计文档 §5.2-7）。mock 范式仿 TagDetail.test.ts / Feed.test.ts。
// vant 的 van-search 换成最小假组件：真实输入框 DOM 不在本票断言范围，
// 这里要钉的是"事件 → 路由 query / 请求参数 / 文案"这条装配线。
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { reactive } from 'vue'
import PostList from '../components/PostList.vue'

type Route = { query: Record<string, string> }

// 真路由对象是响应式的；这里用同一个 reactive 替身，否则"换词"不会触发重算
const routeBox = vi.hoisted(() => ({ store: null as Route | null }))
const routerState = vi.hoisted(() => ({ push: vi.fn(), replace: vi.fn() }))
const mountCount = vi.hoisted(() => ({ n: 0 }))
// PostList 换成带挂载计数的替身：既读得到 props，又看得到"换词是否重挂载"（:key="q" 的效果）
const listStub = vi.hoisted(() => ({
  name: 'PostListStub',
  props: ['fetcher', 'emptyTitle', 'terms'],
  mounted() {
    mountCount.n++
  },
  template: '<div class="postlist-stub"></div>',
}))
const vantState = vi.hoisted(() => ({
  showToast: vi.fn(),
  Search: {
    name: 'VanSearch',
    props: ['modelValue'],
    emits: ['update:modelValue', 'search'],
    template: '<div class="van-search-stub"></div>',
  },
}))

vi.mock('vue-router', () => ({
  useRoute: () => (routeBox.store ??= reactive({ query: {} }) as Route),
  useRouter: () => routerState,
}))
vi.mock('../router', () => ({ goLoginWithReturn: vi.fn() }))
vi.mock('../stores/auth', () => ({ useAuthStore: vi.fn() }))
vi.mock('../api/content', () => ({ listPosts: vi.fn() }))
vi.mock('vant', () => vantState)

import * as api from '../api/content'
import Search from './Search.vue'
import { showToast } from 'vant'

type Wrapper = VueWrapper<Record<string, unknown>>

// 路由替身是全文件共享的响应式对象 ⇒ 历史 wrapper 仍订阅着它，改 query 会把它们一起重挂。
// 所以每个用例结束就卸载，让挂载计数只反映"当前这一个实例"的行为。
const alive: Wrapper[] = []

function mountSearch(q: string | undefined) {
  const route = (routeBox.store ??= reactive({ query: {} }) as Route)
  route.query = q === undefined ? {} : { q }
  const wrapper = mount(Search, { global: { stubs: { PostList: listStub } } }) as Wrapper
  alive.push(wrapper)
  return wrapper
}

const list = (wrapper: Wrapper) => wrapper.findComponent(PostList)

beforeEach(() => {
  while (alive.length) alive.pop()!.unmount()
  mountCount.n = 0
  vi.mocked(api.listPosts).mockResolvedValue({ items: [], page: 1, page_size: 20 })
})

afterEach(() => {
  vi.clearAllMocks()
})

describe('Search.vue（#78 搜索页）', () => {
  it('fetcher 带 q 请求，且不传 tab（搜索是叠加在默认流上的筛选）', async () => {
    const wrapper = mountSearch('论坛')
    await flushPromises()
    await list(wrapper).props('fetcher')(2, 20)
    expect(api.listPosts).toHaveBeenCalledWith({ q: '论坛', page: 2, pageSize: 20 })
  })

  it('空 q / 纯空白 q 不发请求（守卫）', async () => {
    for (const q of [undefined, '', '   ']) {
      vi.mocked(api.listPosts).mockClear()
      const wrapper = mountSearch(q)
      await flushPromises()
      const res = await list(wrapper).props('fetcher')(1, 20)
      expect(res).toEqual({ items: [] })
      expect(api.listPosts).not.toHaveBeenCalled()
    }
  })

  it('非法 q 本地拦截：不发请求，空态文案给可读提示（§6-3 的前端提示）', async () => {
    const cases: [string, string][] = [
      ['论', '搜索词至少 2 个字'],
      ['论'.repeat(65), '搜索词最长 64 个字'],
      ['a b c d e', '最多 4 个搜索词'],
    ]
    for (const [q, tip] of cases) {
      vi.mocked(api.listPosts).mockClear()
      const wrapper = mountSearch(q)
      await flushPromises()
      const res = await list(wrapper).props('fetcher')(1, 20)
      expect(res).toEqual({ items: [] })
      expect(api.listPosts).not.toHaveBeenCalled()
      expect(list(wrapper).props('emptyTitle')).toBe(tip)
    }
  })

  it('合规边界不拦：2 字、64 字、4 词照常请求，空态文案是"没找到"', async () => {
    for (const q of ['论坛', '论'.repeat(64), 'a b c d']) {
      const wrapper = mountSearch(q)
      await flushPromises()
      await list(wrapper).props('fetcher')(1, 20)
      expect(api.listPosts).toHaveBeenCalledWith({ q, page: 1, pageSize: 20 })
      expect(list(wrapper).props('emptyTitle')).toBe(`没有找到「${q}」的相关帖子`)
    }
  })

  it('terms 按空白切词透传给卡片层（高亮在 PostCard，本页只送词）', () => {
    const wrapper = mountSearch(' 论坛  搜索 ')
    expect(list(wrapper).props('terms')).toEqual(['论坛', '搜索'])
  })

  it('total 有值时显示"找到 N 条结果"', async () => {
    vi.mocked(api.listPosts).mockResolvedValue({ items: [], page: 1, page_size: 20, total: 7 })
    const wrapper = mountSearch('论坛')
    await flushPromises()
    await list(wrapper).props('fetcher')(1, 20)
    await flushPromises()
    expect(wrapper.find('.count').text()).toBe('找到 7 条结果')
  })

  it('无 total（未搜索）与 total=0 都不占条数行', async () => {
    const never = mountSearch(undefined)
    await flushPromises()
    expect(never.find('.count').exists()).toBe(false)

    vi.mocked(api.listPosts).mockResolvedValue({ items: [], page: 1, page_size: 20, total: 0 })
    const zero = mountSearch('论坛')
    await flushPromises()
    await list(zero).props('fetcher')(1, 20)
    await flushPromises()
    expect(zero.find('.count').exists()).toBe(false)
  })

  it('fetcher 是稳定引用；换搜索词则 PostList 重挂载（§7-12：:key="q"）', async () => {
    const wrapper = mountSearch('论坛')
    await flushPromises()
    expect(list(wrapper).props('fetcher')).toBe(list(wrapper).props('fetcher'))
    expect(mountCount.n).toBe(1)

    routeBox.store!.query = { q: '搜索' }
    await wrapper.vm.$nextTick()
    await flushPromises()
    expect(mountCount.n).toBe(2) // key 变了 ⇒ 卸载重挂，而不是复用旧列表继续翻页
  })

  it('检索 → router.replace 带 q（换词不堆历史）', async () => {
    const wrapper = mountSearch('论坛')
    const vs = wrapper.findComponent({ name: 'VanSearch' })
    vs.vm.$emit('update:modelValue', '  全文 搜索  ')
    vs.vm.$emit('search')
    expect(routerState.replace).toHaveBeenCalledWith({
      path: '/search',
      query: { q: '全文 搜索' },
    })
    expect(routerState.push).not.toHaveBeenCalled()
  })

  it('输入为空 → 只提示，不导航、不发请求', async () => {
    const wrapper = mountSearch('论坛')
    const vs = wrapper.findComponent({ name: 'VanSearch' })
    vi.mocked(api.listPosts).mockClear()
    vs.vm.$emit('update:modelValue', '   ')
    vs.vm.$emit('search')
    expect(showToast).toHaveBeenCalledWith('请输入搜索词')
    expect(routerState.replace).not.toHaveBeenCalled()
    expect(api.listPosts).not.toHaveBeenCalled()
  })
})
