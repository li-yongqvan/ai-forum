// #50 图片点击放大：事件委托处理器单测（utils/imagePreview.ts）。
// 纯函数 + 真实 DOM：mock 'vant'（showImagePreview/showToast），容器 addEventListener + 派发 MouseEvent。
import { afterEach, describe, expect, it, vi } from 'vitest'

vi.mock('vant', () => ({ showImagePreview: vi.fn(), showToast: vi.fn() }))

import { showImagePreview, showToast } from 'vant'
import { handleContentImageClick } from './imagePreview'

function mountContainer(html: string): HTMLElement {
  const container = document.createElement('div')
  container.innerHTML = html
  document.body.appendChild(container)
  container.addEventListener('click', handleContentImageClick)
  return container
}
/** jsdom 不真实加载图片（complete 恒 false、naturalWidth 恒 0）：显式定属性模拟终态，排除环境干扰。 */
function setImgState(el: Element, complete: boolean, naturalWidth: number) {
  Object.defineProperty(el, 'complete', { configurable: true, value: complete })
  Object.defineProperty(el, 'naturalWidth', { configurable: true, value: naturalWidth })
}
function click(el: Element) {
  el.dispatchEvent(new MouseEvent('click', { bubbles: true }))
}

afterEach(() => {
  vi.clearAllMocks()
  document.body.innerHTML = ''
})

describe('handleContentImageClick（#50 图片点击放大）', () => {
  it('点第 2 张 → 收集容器全部图 + startPosition=1 + showIndex:true', () => {
    const container = mountContainer(
      '<img class="content-img" src="https://a.com/1.png"><img class="content-img" src="https://a.com/2.png">',
    )
    const imgs = container.querySelectorAll('img')
    setImgState(imgs[0], true, 100)
    setImgState(imgs[1], true, 100)
    click(imgs[1])
    expect(showImagePreview).toHaveBeenCalledWith({
      images: ['https://a.com/1.png', 'https://a.com/2.png'],
      startPosition: 1,
      showIndex: true,
    })
  })

  it('单图 → showIndex:false（不显示"1/1"页码）', () => {
    const container = mountContainer('<img class="content-img" src="https://a.com/1.png">')
    const img = container.querySelector('img')!
    setImgState(img, true, 100)
    click(img)
    expect(showImagePreview).toHaveBeenCalledWith({
      images: ['https://a.com/1.png'],
      startPosition: 0,
      showIndex: false,
    })
  })

  it('点非图片元素 → 不调用 showImagePreview/showToast', () => {
    mountContainer('<p>正文文字</p>')
    const p = document.querySelector('p')!
    click(p)
    expect(showImagePreview).not.toHaveBeenCalled()
    expect(showToast).not.toHaveBeenCalled()
  })

  it('坏图（complete && naturalWidth===0）→ toast「图片加载失败」、不打开预览', () => {
    const container = mountContainer('<img class="content-img" src="https://a.com/broken.png">')
    const img = container.querySelector('img')!
    setImgState(img, true, 0)
    click(img)
    expect(showToast).toHaveBeenCalledWith('图片加载失败')
    expect(showImagePreview).not.toHaveBeenCalled()
  })

  it('正常图（complete && naturalWidth>0）→ 打开预览', () => {
    const container = mountContainer('<img class="content-img" src="https://a.com/1.png">')
    const img = container.querySelector('img')!
    setImgState(img, true, 100)
    click(img)
    expect(showImagePreview).toHaveBeenCalledTimes(1)
  })

  it('未加载完成图（complete=false）→ 不误判失败，放行进预览', () => {
    const container = mountContainer('<img class="content-img" src="https://a.com/1.png">')
    const img = container.querySelector('img')!
    setImgState(img, false, 0) // 仍在加载：complete=false
    click(img)
    expect(showImagePreview).toHaveBeenCalledTimes(1)
    expect(showToast).not.toHaveBeenCalled()
  })
})
