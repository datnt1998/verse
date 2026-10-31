// @vitest-environment happy-dom
// Lớp dữ liệu Viderse (src/data/events.ts + status + request):
//  · ≤5 s kể từ mất kết nối — luồng treo sau hello (read() không bao giờ trả)
//    và kết nối headers không bao giờ về đều phải rơi về retrying.
//  · 401 ở bất kỳ request nào → chạy đường hết phiên (notifySessionExpired).
//  · Poll /api/status chỉ khi tab hiện + SSE xuống; làm mới khi tab trở lại.
//  · Nối lại sau rớt: invalidate qua cache — không có fetch thô nào ghi đè.
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cache } from '@noverse/ui/data'
import { events, startDataEvents, POLL_MS } from './events'
import { keys } from './keys'
import { request } from '../api/request'
import { onSessionExpired } from '../api/sessionSignal'

const USER = { user: { id: 'u1', username: 'dat', name: 'Dat', role: 'admin' } }
const STATUS = { running: 1, queued: 2, paused: false, reviewCount: 0, attention: [] }

/** lib tsconfig chưa có Promise.withResolvers — deferred tối thiểu cho test. */
function deferred<T>() {
  let resolve!: (v: T | PromiseLike<T>) => void
  const promise = new Promise<T>((r) => { resolve = r })
  return { promise, resolve }
}

const json = (body: unknown, status = 200) =>
  new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } })

type FetchMock = (input: RequestInfo | URL, init?: RequestInit) => Promise<Response>

function mockFetch(routes: { me?: () => Promise<Response>; status?: () => Promise<Response>; events: (init?: RequestInit) => Promise<Response> }) {
  const calls: string[] = []
  const fn: FetchMock = (input, init) => {
    const url = typeof input === 'string' ? input : input instanceof URL ? input.href : input.url
    calls.push(url)
    if (url.startsWith('/api/me') || url.includes('/api/me')) return routes.me?.() ?? Promise.resolve(json(USER))
    if (url.includes('/api/status')) return routes.status?.() ?? Promise.resolve(json(STATUS))
    if (url.includes('/api/events')) return routes.events(init)
    return Promise.resolve(json({}))
  }
  vi.stubGlobal('fetch', vi.fn(fn))
  return calls
}

/** Luồng SSE "mở": trả headers ngay + frame hello, rồi read() treo vĩnh viễn. */
function hangingStream(firstReads: Uint8Array[]) {
  const encoder = new TextEncoder()
  const reads = [...firstReads]
  return Promise.resolve(new Response(
    new ReadableStream<Uint8Array>({
      pull(controller) {
        const next = reads.shift()
        if (next) controller.enqueue(next)
        // hết frame đầu: không đóng stream — read() kế treo như kết nối chết
      },
    }),
    { status: 200, headers: { 'Content-Type': 'text/event-stream' } },
  ))
}

const HELLO = new TextEncoder().encode('event: hello\ndata: {"subscriber":"t1"}\n\nid: 1\nevent: status\ndata: {"rev":1}\n\n')

describe('mất kết nối trong ≤5 giây', () => {
  beforeEach(() => { vi.useFakeTimers() })
  afterEach(() => { events.stop(); vi.useRealTimers(); vi.unstubAllGlobals() })

  it('luồng nhận hello rồi im hẳn (read treo) → retrying trong ≤5 s, nối lại được', async () => {
    mockFetch({ events: () => hangingStream([HELLO]) })
    events.start()
    await vi.advanceTimersByTimeAsync(10)
    expect(events.status().state).toBe('open')
    // Read tiếp treo mãi: quá 3,5 s không byte → client phải rớt về retrying.
    await vi.advanceTimersByTimeAsync(4200)
    expect(events.status().state).toBe('retrying')
    // Nối lại: sau backoff, hello mới đến → open trở lại (dải mất kết nối tắt).
    await vi.advanceTimersByTimeAsync(1200)
    expect(events.status().state).toBe('open')
  })

  it('headers không bao giờ về (fetch treo) → retrying trong ≤5 s', async () => {
    // Fetch chỉ trả lời khi bị huỷ — giống kết nối chết trước response.
    mockFetch({
      events: (init) => {
        const gate = deferred<Response>()
        init?.signal?.addEventListener('abort', () => gate.resolve(new Response(null, { status: 599 })))
        return gate.promise
      },
    })
    events.start()
    await vi.advanceTimersByTimeAsync(10)
    expect(events.status().state).toBe('connecting')
    await vi.advanceTimersByTimeAsync(4000)
    expect(events.status().state).toBe('retrying')
  })
})

describe('hết phiên (401)', () => {
  beforeEach(() => { vi.useFakeTimers() })
  afterEach(() => { events.stop(); cache.clear(); vi.useRealTimers(); vi.unstubAllGlobals() })

  it('401 trên request thường báo hết phiên cho app', async () => {
    vi.stubGlobal('fetch', vi.fn(() => Promise.resolve(json({ error: 'hết phiên' }, 401))))
    const expired = vi.fn()
    const off = onSessionExpired(expired)
    await expect(request('/api/status')).rejects.toMatchObject({ status: 401 })
    expect(expired).toHaveBeenCalledTimes(1)
    off()
  })

  it('401 khi mở luồng SSE: báo hết phiên + client dừng hẳn', async () => {
    mockFetch({ events: () => Promise.resolve(json({ error: 'hết phiên' }, 401)) })
    const expired = vi.fn()
    const off = onSessionExpired(expired)
    events.start()
    await vi.advanceTimersByTimeAsync(20)
    expect(expired).toHaveBeenCalledTimes(1)
    expect(events.status().state).toBe('stopped')
    off()
    events.stop()
  })

  it('sau khi tháo lớp dữ liệu: không request nào còn ghi cache', async () => {
    const gate = deferred<Response>()
    mockFetch({
      events: () => hangingStream([HELLO]),
      status: () => gate.promise,
    })
    const release = cache.retain(keys.status(), ({ signal }) => request('/api/status', { signal }).then(() => STATUS))
    const stop = startDataEvents()
    await vi.advanceTimersByTimeAsync(30)
    // Giả lập đường hết phiên: App unmount shell → teardown chạy giữa request.
    stop()
    release()
    gate.resolve(json(STATUS))
    await vi.advanceTimersByTimeAsync(10)
    expect(cache.get(keys.status()).data).toBeUndefined()
    expect(events.status().state).toBe('stopped')
  })
})

describe('vòng poll dự phòng', () => {
  beforeEach(() => { vi.useFakeTimers() })
  afterEach(() => { events.stop(); cache.clear(); vi.useRealTimers(); vi.unstubAllGlobals(); vi.restoreAllMocks() })

  it('chỉ poll khi tab hiện + SSE xuống; tab ẩn dừng, trở lại làm mới ngay', async () => {
    let statusCalls = 0
    mockFetch({ events: () => deferred<Response>().promise, status: () => { statusCalls++; return Promise.resolve(json(STATUS)) } })
    const release = cache.retain(keys.status(), () => request('/api/status'))
    const stop = startDataEvents()
    await vi.advanceTimersByTimeAsync(30)
    const whileDown = statusCalls
    expect(whileDown).toBeGreaterThan(0)
    await vi.advanceTimersByTimeAsync(POLL_MS + 100)
    expect(statusCalls).toBeGreaterThan(whileDown)
    // Tab ẩn → poll dừng hẳn.
    vi.spyOn(document, 'visibilityState', 'get').mockReturnValue('hidden')
    document.dispatchEvent(new Event('visibilitychange'))
    const hidden = statusCalls
    await vi.advanceTimersByTimeAsync(POLL_MS * 3)
    expect(statusCalls).toBe(hidden)
    // Trở lại tab → làm mới ngay, không chờ nhịp poll tiếp theo.
    vi.spyOn(document, 'visibilityState', 'get').mockReturnValue('visible')
    document.dispatchEvent(new Event('visibilitychange'))
    await vi.advanceTimersByTimeAsync(10)
    expect(statusCalls).toBeGreaterThan(hidden)
    stop(); release()
  })

  it('refreshes the visible queue alongside status while the event stream is down', async () => {
    mockFetch({ events: () => deferred<Response>().promise })
    let revision = 1
    const release = cache.retain(keys.queue(), async () => ({ revision }))
    await vi.advanceTimersByTimeAsync(10)
    const stop = startDataEvents()
    await vi.advanceTimersByTimeAsync(10)
    revision = 2
    await vi.advanceTimersByTimeAsync(3000)
    expect(cache.get<{ revision: number }>(keys.queue()).data?.revision).toBe(2)
    stop(); release()
  })

  it('nối lại sau rớt: chỉ invalidate qua cache, không setData thô', async () => {
    mockFetch({ events: () => hangingStream([HELLO]), status: () => Promise.resolve(json(STATUS)) })
    const release = cache.retain(keys.status(), () => request('/api/status'))
    const stop = startDataEvents()
    await vi.advanceTimersByTimeAsync(30)
    expect(events.status().state).toBe('open')
    const setData = vi.spyOn(cache, 'setData')
    const setDataCalls = setData.mock.calls.length
    // Luồng chết → backoff → hello mới → onReconnect reset(): chỉ invalidate,
    // cache tự fetch lại; không fetchStatus thô ghi thẳng vào cache.
    await vi.advanceTimersByTimeAsync(4200)
    expect(events.status().state).toBe('retrying')
    await vi.advanceTimersByTimeAsync(1200)
    expect(events.status().state).toBe('open')
    await vi.advanceTimersByTimeAsync(50)
    expect(setData.mock.calls.length).toBe(setDataCalls)
    stop(); release()
  })
})
