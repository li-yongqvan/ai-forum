import { showImagePreview, showToast } from 'vant'
// 函数式组件样式不随按需引入加载（函数式调用无模板组件被 resolver 识别，toast 此前一直无样式）：
// 显式引入保证预览（overlay/swipe/页码）与提示渲染正常；CSS 全局生效，一并修复全站 toast 样式。
import 'vant/es/image-preview/style'
import 'vant/es/toast/style'

/** #50 图片点击放大：md 渲染容器的事件委托处理器（@click）。 */
export function handleContentImageClick(e: MouseEvent): void {
  const container = e.currentTarget as HTMLElement | null
  const target = e.target as Element | null
  if (!container || !target) return
  const img = target.closest('.content-img') as HTMLImageElement | null
  if (!img) return
  e.preventDefault() // 命中即拦默认行为：防御性保留（当前 md() 不产出 <a><img></a>，无行为变更），面向未来
  const images = Array.from(container.querySelectorAll<HTMLImageElement>('img.content-img'))
  const startPosition = images.indexOf(img) // 按元素引用定位，同 URL 重复图也准
  if (startPosition < 0) return
  if (img.complete && img.naturalWidth === 0) {
    showToast('图片加载失败')
    return
  }
  showImagePreview({ images: images.map((i) => i.src), startPosition, showIndex: images.length > 1 })
}
