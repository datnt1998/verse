// @vitest-environment happy-dom
// Danh sách video trên điện thoại: cửa sổ DOM bám khung nhìn (không dựng cả
// danh sách), hàng muộn xuất hiện khi cuộn, và chỉ xin trang kế khi cửa sổ đã
// chạm vùng cuối của dữ liệu đã nạp. Dùng virtualizer thật; chỉ stub kích thước
// layout vì happy-dom không có layout engine.
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { Video } from '../../../api/rivals'
import { MobileVideoList } from './MobileVideoList'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

const VIEWPORT = 844
const ROW_GAP = 18

const video = (i: number): Video => ({
  id: `v${i}`, channelId: 'r1', title: `Video số ${i}`, durationSec: 2391,
  viewCount: 1_200_000, publishedAt: '2026-10-28T04:00:00Z', exactTime: true,
  score: 2.1, hasCaptions: true,
  thumbnailUrl: `https://i.ytimg.com/vi/v${i}/mqdefault.jpg`, url: `https://youtu.be/v${i}`,
})

const originalOffsetHeight = Object.getOwnPropertyDescriptor(HTMLElement.prototype, 'offsetHeight')!
const roots: Root[] = []

const flush = async (ms = 30) => {
  const { promise, resolve } = Promise.withResolvers<void>()
  setTimeout(resolve, ms)
  await act(async () => { await promise })
}

const mount = async (items: Video[], hasMore: boolean, needMore: () => void) => {
  const scroller = document.createElement('div')
  scroller.style.overflowY = 'auto'
  Object.defineProperty(scroller, 'clientHeight', { configurable: true, value: VIEWPORT })
  Object.defineProperty(scroller, 'clientWidth', { configurable: true, value: 390 })
  const host = document.createElement('div')
  scroller.append(host)
  document.body.append(scroller)
  const root = createRoot(host)
  roots.push(root)
  await act(async () => {
    root.render(
      <MobileVideoList items={items} hasMore={hasMore} needMore={needMore}>
        {(item) => <div>{item.title}</div>}
      </MobileVideoList>,
    )
  })
  await flush()
  return { host, scroller }
}

const rowIndexes = (host: ParentNode) =>
  [...host.querySelectorAll<HTMLElement>('[data-index]')].map((node) => Number(node.dataset.index))

const until = async (fn: () => boolean, tries = 40) => {
  for (let i = 0; i < tries; i++) { if (fn()) return; await flush() }
  expect(fn()).toBe(true)
}

const scrollTo = async (scroller: HTMLElement, top: number) => {
  await act(async () => {
    scroller.scrollTop = top
    scroller.dispatchEvent(new Event('scroll', { bubbles: true }))
  })
  await flush()
}

beforeEach(() => {
  // happy-dom không đo layout: mỗi hàng có chiều cao thật, thay đổi theo index,
  // để virtualizer thật đo và tính lại vị trí như trình duyệt.
  Object.defineProperty(HTMLElement.prototype, 'offsetHeight', {
    configurable: true,
    get(this: HTMLElement) {
      const index = this.dataset.index
      return index == null ? 0 : 300 + (Number(index) % 4) * 22
    },
  })
})

afterEach(async () => {
  for (const root of roots.splice(0)) await act(async () => { root.unmount() })
  document.body.innerHTML = ''
  Object.defineProperty(HTMLElement.prototype, 'offsetHeight', originalOffsetHeight)
})

describe('MobileVideoList — cửa sổ DOM và nạp trang kế', () => {
  it('chỉ dựng cửa sổ quanh khung nhìn; cuộn xuống dựng hàng muộn mà không dựng cả danh sách', async () => {
    const needMore = vi.fn()
    const { host, scroller } = await mount(
      Array.from({ length: 1000 }, (_, i) => video(i)), true, needMore,
    )

    const initial = rowIndexes(host)
    expect(initial.length).toBeGreaterThan(0)
    expect(initial.length).toBeLessThan(20)
    expect(initial[0]).toBe(0)
    expect(initial.at(-1)).toBeLessThan(100)
    expect(needMore).not.toHaveBeenCalled()

    // Hàng thứ hai bắt đầu sau chiều cao ĐO ĐƯỢC của hàng đầu (300 + gap 18),
    // không phải sau ước lượng 358×9/16+90.
    await until(() => host.querySelector<HTMLElement>('[data-index="1"]')?.style.transform === 'translateY(318px)')
    expect(host.querySelector<HTMLElement>('[data-index="0"]')?.style.transform).toBe('translateY(0px)')
    expect(host.querySelector<HTMLElement>('[data-index="1"]')?.style.transform).toBe('translateY(318px)')

    const total = Number.parseFloat(
      host.querySelector<HTMLElement>('[data-index]')?.parentElement?.style.height ?? '0',
    )
    expect(total).toBeGreaterThan(100_000)
    await scrollTo(scroller, total * 0.6)

    const later = rowIndexes(host)
    expect(later.length).toBeGreaterThan(0)
    expect(later.length).toBeLessThan(20)
    expect(Math.min(...later)).toBeGreaterThan(500)
    expect(later).not.toContain(999)
    expect(needMore).not.toHaveBeenCalled()
  })

  it('chạm vùng cuối dữ liệu đã nạp thì gọi needMore, không gọi lại trong cùng cửa sổ', async () => {
    const needMore = vi.fn()
    const { host, scroller } = await mount(
      Array.from({ length: 200 }, (_, i) => video(i)), true, needMore,
    )

    const total = Number.parseFloat(
      host.querySelector<HTMLElement>('[data-index]')?.parentElement?.style.height ?? '0',
    )
    expect(needMore).not.toHaveBeenCalled()
    await scrollTo(scroller, Math.max(0, total - VIEWPORT))

    expect(needMore).toHaveBeenCalled()
    const callsAtEnd = needMore.mock.calls.length
    expect(rowIndexes(host).at(-1)).toBeGreaterThanOrEqual(196)
    expect(rowIndexes(host).length).toBeLessThan(20)

    // Cùng cửa sổ cuối: cuộn thêm không xin trang nữa.
    await scrollTo(scroller, Math.max(0, total - VIEWPORT + ROW_GAP * 2))
    expect(needMore).toHaveBeenCalledTimes(callsAtEnd)
    expect(rowIndexes(host).length).toBeLessThan(20)
  })
})
