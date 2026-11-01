// @vitest-environment happy-dom
// Hợp đồng pane điện thoại: đổi tab chỉ ẩn màn cũ (state + vị trí cuộn còn
// nguyên, không gọi lại API khi dữ liệu còn mới), màn đẩy phủ lên trên mà các
// pane vẫn mounted, và sau khung hình đầu thì nạp trước dữ liệu của tab chưa mở.
// Test dựng khung thật với màn giả: điều cần chốt là khung giữ-alive + lớp dữ
// liệu SWR, không phải nội dung từng màn.
import { act, useState } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cache } from '@noverse/ui/data'
import { useResource } from '../data/useResource'
import { parseHash, type Route } from '../router'
import { MobileRouteStage, paneKeyOf } from './MobileRouteStage'

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
  const [state, setState] = useState('a')
  return <div data-marker="workbench" data-state={state}>
    <button type="button" onClick={() => setState('b')}>đổi</button>
    <span>{resource.data ? 'có dữ liệu' : 'chờ'}</span>
  </div>
}
const ReviewPane = () => <div data-marker="review">Duyệt</div>
const renderScreen = (route: Route) => {
  if (route.name === 'workbench') return <WorkPane />
  if (route.name === 'review') return <ReviewPane />
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

    await draw({ name: 'review' })
    expect(getComputedStyle(paneOf(container, 'workbench')).display).toBe('none')
    expect(getComputedStyle(paneOf(container, 'review')).display).toBe('flex')
    // Vị trí cuộn của tab đang ẩn không bị thanh tab khác ghi đè.
    paneOf(container, 'workbench').scrollTop = 500
    await draw({ name: 'workbench' })

    expect(container.querySelector('[data-marker="workbench"]')).toBe(node)
    expect(node.dataset.state).toBe('b')
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
})

describe('route → pane', () => {
  it('5 tab có pane riêng, màn đẩy không chiếm pane', () => {
    expect(paneKeyOf(parseHash('#/').route)).toBe('workbench')
    expect(paneKeyOf(parseHash('#/review').route)).toBe('review')
    expect(paneKeyOf(parseHash('#/new').route)).toBe('new')
    expect(paneKeyOf(parseHash('#/channels/new').route)).toBe('channels')
    expect(paneKeyOf(parseHash('#/channels/yt-1').route)).toBe('channels')
    expect(paneKeyOf(parseHash('#/explore').route)).toBe('explore')
    expect(paneKeyOf(parseHash('#/settings').route)).toBeNull()
    expect(paneKeyOf(parseHash('#/queue').route)).toBeNull()
    expect(paneKeyOf(parseHash('#/library').route)).toBeNull()
    expect(paneKeyOf(parseHash('#/templates/t-1').route)).toBeNull()
  })
})

describe('nạp trước tab chưa mở', () => {
  it('sau khung hình đầu, dữ liệu của các tab còn lại được nạp ở nhịp rảnh', async () => {
    const fetchMock = vi.fn((path: string) => Promise.resolve(Response.json(path.includes('clone/counts') ? { approvedToday: null } : {})))
    vi.stubGlobal('fetch', fetchMock)
    const { draw } = await render({ name: 'workbench' })
    // Nhịp rảnh: requestIdleCallback (timeout 1,5 s) hoặc hẹn giờ 250 ms.
    const idle = Promise.withResolvers<void>()
    setTimeout(idle.resolve, 400)
    await act(async () => { await idle.promise })
    const paths = fetchMock.mock.calls.map(([path]) => String(path))
    // Bàn dựng (danh sách video + tóm tắt kênh), Duyệt (số đếm), Khám phá (đối thủ + lưới mặc định).
    expect(paths).toContain('/api/clone?channel=all')
    expect(paths).toContain('/api/channels/summary')
    expect(paths).toContain('/api/clone/counts')
    expect(paths).toContain('/api/rivals')
    expect(paths.some((path) => path.startsWith('/api/rivals/all/videos'))).toBe(true)
    expect(paths).toContain('/api/owner-channels')
    await draw({ name: 'review' })
  })
})
