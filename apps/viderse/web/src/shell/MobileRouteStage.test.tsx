// @vitest-environment happy-dom
// Hợp đồng pane điện thoại: đổi tab chỉ ẩn màn cũ (state + vị trí cuộn còn
// nguyên, không gọi lại API khi dữ liệu còn mới), màn đẩy phủ lên trên mà các
// pane vẫn mounted. Tab chưa mở không bị gọi API đầu cơ trong lượt tải đầu.
// Test dựng khung thật với màn giả: điều cần chốt là khung giữ-alive + lớp dữ
// liệu SWR, không phải nội dung từng màn.
import { act, useContext, useState } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cache } from '@noverse/ui/data'
import { PANE_STALE_MS, PaneStaleContext } from '../data/paneStale'
import { useResource } from '../data/useResource'
import type { Route } from '../router'
import { MobileRouteStage } from './MobileRouteStage'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

const roots: Root[] = []
afterEach(async () => {
  for (const root of roots.splice(0)) await act(() => root.unmount())
  document.body.replaceChildren()
  vi.useRealTimers()
  vi.unstubAllGlobals()
})

let workbenchFetches = 0
const WorkPane = () => {
  const resource = useResource(['test', 'workbench'], async () => { workbenchFetches++; return { count: workbenchFetches } })
  // Mốc "còn mới" mà hook trong pane nhận được: 30 s thay vì 2 s mặc định.
  const stale = useContext(PaneStaleContext)
  const [state, setState] = useState('a')
  return <div data-marker="workbench" data-state={state} data-stale={String(stale)}>
    <button type="button" onClick={() => setState('b')}>đổi</button>
    <span>{resource.data ? 'có dữ liệu' : 'chờ'}</span>
  </div>
}
const ReviewPane = () => <div data-marker="review">Duyệt</div>
const ExplorePane = () => {
  const [filter, setFilter] = useState('all')
  return <div data-marker="explore" data-filter={filter}>
    <button type="button" onClick={() => setFilter('long')}>lọc</button>
  </div>
}
const ChannelPane = () => {
  const [filter, setFilter] = useState('all')
  return <div data-marker="channel" data-filter={filter}>
    <button type="button" onClick={() => setFilter('ready')}>lọc kênh</button>
  </div>
}
const renderScreen = (route: Route) => {
  if (route.name === 'workbench') return <WorkPane />
  if (route.name === 'review') return <ReviewPane />
  if (route.name === 'explore' && !route.video) return <ExplorePane />
  if (route.name === 'channel') return <ChannelPane />
  return <div data-marker="pushed" data-screen={route.name}>màn đẩy</div>
}

const render = async (route: Route) => {
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)
  roots.push(root)
  const draw = async (next: Route) => { await act(async () => { root.render(<MobileRouteStage route={next} renderScreen={renderScreen} />) }) }
  await draw(route)
  return { container, draw }
}

const paneOf = (container: HTMLElement, id: string) => container.querySelector(`[data-pane="${id}"]`) as HTMLElement

beforeEach(() => {
  workbenchFetches = 0
  // Bộ đệm là singleton của kit: dọn để mỗi test bắt đầu từ dữ liệu trống.
  cache.clear()
  vi.stubGlobal('fetch', vi.fn((path: string) => Promise.resolve(Response.json(path.includes('clone/counts') ? { approvedToday: null } : {}))))
})


describe('giữ pane khi đổi tab', () => {
  it('đổi tab rồi quay lại: state, DOM và dữ liệu còn nguyên, không gọi lại API', async () => {
    const { container, draw } = await render({ name: 'workbench' })
    await act(async () => {})
    expect(workbenchFetches).toBe(1)
    const node = container.querySelector('[data-marker="workbench"]') as HTMLElement
    await act(async () => { node.querySelector('button')!.click() })
    expect(node.dataset.state).toBe('b')
    expect(node.dataset.stale).toBe(String(PANE_STALE_MS))

    await draw({ name: 'review' })
    expect(getComputedStyle(paneOf(container, 'workbench')).display).toBe('none')
    expect(getComputedStyle(paneOf(container, 'review')).display).toBe('flex')
    // Vị trí cuộn của tab đang ẩn không bị thanh tab khác ghi đè.
    paneOf(container, 'workbench').scrollTop = 500
    await draw({ name: 'workbench' })

    expect(container.querySelector('[data-marker="workbench"]')).toBe(node)
    expect(node.dataset.state).toBe('b')
    expect(node.dataset.stale).toBe(String(PANE_STALE_MS))
    expect(paneOf(container, 'workbench').scrollTop).toBe(500)
    expect(workbenchFetches).toBe(1)
  })

  it('màn đẩy phủ lên trên nhưng pane đã mở vẫn nằm dưới (không gỡ, không tải lại)', async () => {
    const { container, draw } = await render({ name: 'workbench' })
    await act(async () => {})
    const node = container.querySelector('[data-marker="workbench"]')
    await draw({ name: 'settings' })
    expect(container.querySelector('[data-marker="pushed"]')).toBeTruthy()
    expect(container.querySelector('[data-marker="workbench"]')).toBe(node)
    expect(paneOf(container, 'workbench').getAttribute('aria-hidden')).toBe('true')
    await draw({ name: 'workbench' })
    expect(container.querySelector('[data-marker="pushed"]')).toBeNull()
    expect(container.querySelector('[data-marker="workbench"]')).toBe(node)
    expect(workbenchFetches).toBe(1)
  })

  it('mở video đối thủ rồi quay lại giữ bộ lọc và vị trí của danh sách', async () => {
    const { container, draw } = await render({ name: 'explore' })
    const list = container.querySelector('[data-marker="explore"]') as HTMLElement
    await act(async () => { list.querySelector('button')!.click() })
    const pane = paneOf(container, 'explore')
    pane.scrollTop = 320

    await draw({ name: 'explore', video: 'rival-video-1' })
    expect(container.querySelector('[data-marker="pushed"]')).toBeTruthy()
    expect(pane.getAttribute('aria-hidden')).toBe('true')

    await draw({ name: 'explore' })
    expect(container.querySelector('[data-marker="pushed"]')).toBeNull()
    expect(container.querySelector('[data-marker="explore"]')).toBe(list)
    expect(list.dataset.filter).toBe('long')
    expect(pane.scrollTop).toBe(320)
  })

  it('tạo kênh là màn con, quay lại giữ trạng thái trang kênh đang mở', async () => {
    const { container, draw } = await render({ name: 'channel', channelId: 'channel-1' })
    const channel = container.querySelector('[data-marker="channel"]') as HTMLElement
    await act(async () => { channel.querySelector('button')!.click() })
    await draw({ name: 'channel-new' })
    expect(container.querySelector('[data-marker="channel"]')).toBe(channel)
    await draw({ name: 'channel', channelId: 'channel-1' })
    expect(container.querySelector('[data-marker="channel"]')).toBe(channel)
    expect(channel.dataset.filter).toBe('ready')
  })
})

