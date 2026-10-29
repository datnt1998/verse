// @vitest-environment happy-dom
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, expect, it, vi } from 'vitest'
import { cache } from '../../data'
import type { HostEvent } from '../../api/types'
import { ActivityScreen } from './index'
import fixture from './history.fixture.json'

const live = vi.hoisted(() => ({ events: [] as HostEvent[] }))
vi.mock('../../api/useStoryEvents', () => ({ useStoryEvents: () => ({ ...live, jobs: new Map(), snapshot: null, blocked: null, loading: false, refreshing: false, connected: true, retryNow() {}, refresh: async () => {} }) }))
vi.mock('../../api/session', () => ({ useSession: () => ({ user: { username: 'admin', role: 'admin', projects: {} } }) }))
vi.mock('../cmdresults', () => ({ useCommandResults: () => ({}), CommandResults: () => null }))
Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })
let root: Root | undefined
let intersect: IntersectionObserverCallback
let observed: Element

afterEach(async () => {
  await act(() => root?.unmount())
  cache.clear()
  document.body.replaceChildren()
  vi.unstubAllGlobals()
})

it('renders persisted desktop history, automatically pages older rows, and upserts live events by ID', async () => {
  vi.stubGlobal('IntersectionObserver', class {
    constructor(callback: IntersectionObserverCallback) { intersect = callback }
    observe(target: Element) { observed = target }
    disconnect() {}
  })
  const older = { ...fixture.events[1], id: 'line-467300', time: '2026-10-24T23:00:28.273+07:00', summary: '«Trà Xanh Cướp Kịch Bản Của Tôi, Tôi Lên Show Thực Tế Đọc Bản Gốc Cho Cả Nước Nghe» hoàn thành: 8 chương, 61.693 chữ' }
  const fetcher = vi.fn(async (path: string) => {
    const url = new URL(path, 'http://localhost')
    if (url.pathname.endsWith('/events')) return Response.json(url.searchParams.has('before') ? { ...fixture, events: [older], nextCursor: null } : fixture)
    if (url.pathname === '/api/system') return Response.json({ canReveal: false })
    if (url.pathname === '/api/reveal') return Response.json({ path: 'nhật ký' })
    return Response.json({ chapters: [] })
  })
  vi.stubGlobal('fetch', fetcher)
  const container = document.createElement('div')
  document.body.append(container)
  root = createRoot(container)
  await act(async () => { root!.render(<ActivityScreen project="dit-dang-audio" story="01-tra-xanh-cuop-kich-ban" />) })
  expect(container.textContent).toContain('Đang tiến hành xuất bản...')
  expect(container.textContent).not.toContain('Chưa có hoạt động nào')
  expect(container.querySelector('[role="radiogroup"]')?.textContent).toContain('368')
  expect(fetcher.mock.calls.some(([path]) => path.endsWith('/events?limit=500'))).toBe(true)
  expect(container.textContent).not.toContain('Xem các chương trước')
  await act(async () => { intersect([{ isIntersecting: true, target: observed } as IntersectionObserverEntry], {} as IntersectionObserver) })
  expect(container.textContent).toContain('hoàn thành: 8 chương, 61.693 chữ')
  expect(fetcher.mock.calls.some(([path]) => new URL(path, 'http://localhost').searchParams.get('before') === fixture.nextCursor)).toBe(true)
  live.events = [{ ...fixture.events[1], summary: 'Đã cập nhật xuất bản' }]
  await act(async () => { root!.render(<ActivityScreen project="dit-dang-audio" story="01-tra-xanh-cuop-kich-ban" />) })
  expect(container.textContent).toContain('Đã cập nhật xuất bản')
  expect(container.textContent).not.toContain('Đang tiến hành xuất bản...')
  expect(container.querySelectorAll('ol li')).toHaveLength(3)
})
