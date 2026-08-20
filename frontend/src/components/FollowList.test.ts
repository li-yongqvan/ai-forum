// FollowList.vue #23 通用关注列表：无限滚动 + 乐观取关（成功移除/失败回滚）+ 空态 + in-flight 保护（M4）。
// 行模板由 slot 提供，测试以 render function slot 注入。
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { h } from 'vue'
import { showToast } from 'vant'
import Empty from './Empty.vue'
import FollowList from './FollowList.vue'

vi.mock('vant', () => ({ showToast: vi.fn() }))

type Row = { id: number; name: string }
type PageOf = { items: Row[]; page: number; page_size: number }

// FollowList 是泛型 SFC：test-utils 的 mount/findComponent 对其泛型推导不完整（T 退回约束 {id:number}），
// 本测试验证行为而非类型，mount 与 slot 转 any（同 Me.test.ts 的 findComponent 处理）。
function mountList(
  fetcher: (page: number, pageSize: number) => Promise<PageOf>,
  unfollow?: (row: Row) => Promise<unknown>,
) {
  return mount(FollowList as any, {
    props: { fetcher, unfollow, emptyTitle: '还没有关注' },
    slots: {
      default: ((scope: any) =>
        h('div', { class: 'row' }, [
          h('span', scope.row.name),
          h('button', { class: 'unf', onClick: scope.unfollow }, '取关'),
        ])) as any,
    },
  })
}

describe('FollowList.vue #23', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('挂载加载并渲染 slot 行', async () => {
    const fetcher = vi.fn().mockResolvedValue({
      items: [
        { id: 1, name: 'alice' },
        { id: 2, name: 'bob' },
      ],
      page: 1,
      page_size: 20,
    })
    const wrapper = mountList(fetcher)
    await flushPromises()
    expect(fetcher).toHaveBeenCalledWith(1, 20)
    const rows = wrapper.findAll('.row')
    expect(rows).toHaveLength(2)
    expect(rows[0].text()).toContain('alice')
    expect(rows[1].text()).toContain('bob')
  })

  it('空列表 → Empty 空态（非 null）', async () => {
    const fetcher = vi.fn().mockResolvedValue({ items: [], page: 1, page_size: 20 })
    const wrapper = mountList(fetcher)
    await flushPromises()
    const empty = wrapper.findComponent(Empty)
    expect(empty.exists()).toBe(true)
    expect(empty.props('title')).toBe('还没有关注')
  })

  it('乐观取消成功：移除行 + 调用 unfollow', async () => {
    const unfollow = vi.fn().mockResolvedValue(undefined)
    const fetcher = vi.fn().mockResolvedValue({ items: [{ id: 1, name: 'alice' }], page: 1, page_size: 20 })
    const wrapper = mountList(fetcher, unfollow)
    await flushPromises()
    await wrapper.find('.unf').trigger('click')
    await flushPromises()
    expect(unfollow).toHaveBeenCalledWith({ id: 1, name: 'alice' })
    expect(wrapper.findAll('.row')).toHaveLength(0)
  })

  it('乐观取消失败：回滚行 + toast（失败分支唯一所在，D4）', async () => {
    const unfollow = vi.fn().mockRejectedValue(new Error('取消失败'))
    const fetcher = vi.fn().mockResolvedValue({ items: [{ id: 1, name: 'alice' }], page: 1, page_size: 20 })
    const wrapper = mountList(fetcher, unfollow)
    await flushPromises()
    await wrapper.find('.unf').trigger('click')
    await flushPromises()
    expect(wrapper.findAll('.row')).toHaveLength(1) // 回滚恢复
    expect(showToast).toHaveBeenCalledWith('取消失败')
  })

  it('in-flight 保护：pending 期间重复触发不产生第二次调用（M4）', async () => {
    let resolveFn!: (v: unknown) => void
    const unfollow = vi
      .fn()
      .mockImplementation(() => new Promise((r) => { resolveFn = r }))
    const fetcher = vi.fn().mockResolvedValue({ items: [{ id: 1, name: 'alice' }], page: 1, page_size: 20 })
    const wrapper = mountList(fetcher, unfollow)
    await flushPromises()

    const btn = wrapper.find('.unf')
    btn.trigger('click') // 第一次：pending（未 resolve）
    btn.trigger('click') // 第二次：in-flight 保护应忽略
    await flushPromises()
    expect(unfollow).toHaveBeenCalledTimes(1)

    resolveFn(undefined)
    await flushPromises()
    expect(wrapper.findAll('.row')).toHaveLength(0) // settle 后行已移除
  })

  it('短页 → 到底啦', async () => {
    const fetcher = vi.fn().mockResolvedValue({ items: [{ id: 1, name: 'alice' }], page: 1, page_size: 20 })
    const wrapper = mountList(fetcher)
    await flushPromises()
    expect(wrapper.text()).toContain('到底啦')
  })
})
