import { test, expect } from '@playwright/test'

// 核心循环冒烟：注册 → 发帖 → 详情 → 评论 → 点赞 → 关注 → 关注流
// 前置：后端运行于 8080；邀请码 E2E1 需在跑测前重新武装（见跑测命令）
const uniq = Date.now().toString(36)

// 1×1 透明 PNG（#12：setInputFiles 上传用；MIME sniff 识别为 image/png）
const PNG_1X1_HEX =
  '89504e470d0a1a0a0000000d49484452000000010000000108060000001f15c4890000000a49444154789c6360010000050001d69f4c240000000049454e44ae426082'

test('注册 → 发帖 → 详情 → 评论 → 点赞 → 关注板块 → 关注流', async ({ page }) => {
  // 1. 注册（自动登录 → /feed）
  await page.goto('/#/register')
  await page.getByPlaceholder('登录名 = 展示名').fill(`e2e_${uniq}`)
  await page.getByPlaceholder('用于账号标识').fill(`e2e_${uniq}@x.edu`)
  await page.getByPlaceholder('至少 8 位').fill('secret123')
  await page.getByPlaceholder('管理员发放的一次性邀请码').fill('E2E1')
  await page.getByRole('button', { name: '注册' }).click()
  await page.waitForURL(/#\/feed/, { timeout: 15_000 })
  await expect(page).toHaveURL(/#\/feed/)

  // 2. 发帖（FAB → /write → 选板块 → 发布 → /post/:id）
  await page.locator('button[aria-label="发帖"]').click()
  await page.waitForURL(/#\/write/)
  await page.getByPlaceholder('起个清晰的标题').fill(`E2E 冒烟帖 ${uniq}`)
  await page.getByPlaceholder(/正文/).fill('这是 **冒烟** 正文')

  // 2b. 图片上传（#12：选图 → 上传 → URL 自动插入正文光标处）
  await page.locator('.editor input[type="file"]').setInputFiles({
    name: 'e2e.png',
    mimeType: 'image/png',
    buffer: Buffer.from(PNG_1X1_HEX, 'hex'),
  })
  // 等上传成功：textarea 中出现 /uploads/ 图片 URL
  await expect(page.locator('.content-input')).toHaveValue(/\/uploads\/[0-9a-f]+\.png/)

  await page.getByPlaceholder('选择板块（必选）').click()
  await page.getByRole('button', { name: '学习讨论' }).click()
  await page.getByRole('button', { name: '发布' }).click()
  await page.waitForURL(/#\/post\//, { timeout: 15_000 })
  await expect(page.getByRole('heading', { name: `E2E 冒烟帖 ${uniq}` })).toBeVisible()
  // 详情页应渲染出上传的图片（md 外链图，src 指向 /uploads/）
  const img = page.locator('.detail img[src*="/uploads/"]')
  await expect(img).toBeVisible()

  // 3. 评论（底部输入栏）
  await page.getByPlaceholder('写下你的评论…').fill('第一条评论')
  await page.locator('.sbtn').click()
  await expect(page.getByText('第一条评论')).toBeVisible()

  // 4. 点赞 → 计数与态变化
  const likeBtn = page.locator('.dacts .dact').first()
  await likeBtn.click()
  // 点赞后 refresh 状态保持（viewer 字段，v2 §5 验证项）
  await page.reload()
  await expect(page.getByText(`E2E 冒烟帖 ${uniq}`)).toBeVisible()

  // 5. 返回 feed → 关注板块1 → 关注流出现刚发的帖
  await page.goto('/#/boards')
  const board = page.locator('.board-card', { hasText: '学习讨论' })
  await board.locator('.follow-btn').click()
  // 等异步关注完成（按钮变「已关注」后再导航，避免请求被导航中断）
  await expect(board.locator('.follow-btn')).toHaveText('已关注')
  await page.goto('/#/feed')
  await page.waitForURL(/#\/feed/)
  // hash-only 导航后 SPA 需时间重渲染：等 SegTabs 出现
  await expect(page.getByRole('button', { name: '全部', exact: true })).toBeVisible()
  await page.getByRole('button', { name: '关注', exact: true }).click()
  await expect(page.getByText(`E2E 冒烟帖 ${uniq}`)).toBeVisible()
})
