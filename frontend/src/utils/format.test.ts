import { afterEach, describe, expect, it, vi } from 'vitest'
import { formatTime } from './format'

describe('formatTime', () => {
  afterEach(() => vi.useRealTimers())
  it('刚刚', () => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-08-18T10:00:00Z'))
    expect(formatTime('2026-08-18T09:59:30Z')).toBe('刚刚')
  })
  it('N 分钟前', () => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-08-18T10:00:00Z'))
    expect(formatTime('2026-08-18T09:40:00Z')).toBe('20 分钟前')
  })
  it('N 小时前', () => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-08-18T10:00:00Z'))
    expect(formatTime('2026-08-18T02:00:00Z')).toBe('8 小时前')
  })
  it('N 天前', () => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-08-18T10:00:00Z'))
    expect(formatTime('2026-08-15T10:00:00Z')).toBe('3 天前')
  })
  it('同年更早显示 MM-DD HH:mm（本地时区）', () => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-08-18T10:00:00Z'))
    // 展示用本地时间，断言格式形状即可（避免时区依赖）
    expect(formatTime('2026-07-01T08:05:00Z')).toMatch(/^\d{2}-\d{2} \d{2}:\d{2}$/)
  })
  it('非法输入原样返回', () => {
    expect(formatTime('not-a-date')).toBe('not-a-date')
  })
})
