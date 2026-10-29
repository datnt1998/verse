import 'fake-indexeddb/auto'
import { afterEach, expect, it, vi } from 'vitest'
import { me, sendInput, setUnauthorizedHandler } from '../api/client'
import { getRemembered, rememberAccount } from '../api/storage'
import { createEventsClient, events } from '../data/events'
import { getDraft, registerAccountSession, setDraft, setDraftAccount, switchAccount } from './accounts'
import { mobileBackoff, MOBILE_BACKOFF_MS, tabModeFor } from './logic'
import { drainOutbox, enqueueComment, readOutbox, setOutboxAccount } from './outbox'
import { accountTransition, sessionGate } from './session'

const storage = new Map<string, string>()
vi.stubGlobal('localStorage', { getItem: (key: string) => storage.get(key) ?? null, setItem: (key: string, value: string) => storage.set(key, value), removeItem: (key: string) => storage.delete(key) })
vi.stubGlobal('navigator', { onLine: true })
afterEach(() => { vi.useRealTimers(); vi.restoreAllMocks(); setUnauthorizedHandler(null); storage.clear(); sessionGate.complete() })

it('tabMode: bàn phím và màn ẩn thắng mini; cuộn lên trả mặc định', () => {
  expect(tabModeFor({ down: true })).toBe('mini')
  expect(tabModeFor({ down: false })).toBe('default')
  expect(tabModeFor({ keyboard: 300, down: true })).toBe('hide')
  expect(tabModeFor({ hidden: true })).toBe('hide')
  expect(tabModeFor({ immersive: true, down: true })).toBe('hide')
})

it('nối lại thật theo 1 → 2 → 4 → 5 giây và giữ trần 5 giây', () => {
  vi.useFakeTimers()
  const sources: { onerror: (() => void) | null; close: () => void; addEventListener: () => void }[] = []
  const client = createEventsClient({ backoff: () => MOBILE_BACKOFF_MS, createSource: () => { const source = { onerror: null, close() {}, addEventListener() {} }; sources.push(source); return source } })
  client.start()
  const observed: number[] = []
  for (const expected of [1000, 2000, 4000, 5000, 5000]) {
    sources.at(-1)!.onerror!()
    observed.push(client.status().reconnectAt! - Date.now())
    vi.advanceTimersByTime(expected)
  }
  expect(observed).toEqual([1000, 2000, 4000, 5000, 5000])
  expect([0, 1, 2, 3, 10].map(mobileBackoff)).toEqual(observed)
  client.stop()
})

it('IndexedDB outbox giữ góp ý theo tài khoản, giữ khi lỗi và gửi lại khi online', async () => {
  const fetcher = vi.fn().mockRejectedValueOnce(new TypeError('offline')).mockResolvedValue(new Response(JSON.stringify({ id: 'comment-1' }), { status: 200, headers: { 'Content-Type': 'application/json' } }))
  vi.stubGlobal('fetch', fetcher)
  setOutboxAccount('dat')
  const input = { paragraph: 2, quote: 'Trời đổ mưa', chips: ['Thêm thoại'], text: 'Để nhân vật trả lời.' }
  await enqueueComment({ project: 'p', story: 's', chapter: 3, input })
  expect(await readOutbox()).toHaveLength(1)
  setOutboxAccount('linh'); await drainOutbox(); expect(fetcher).not.toHaveBeenCalled()
  setOutboxAccount('dat'); await drainOutbox(); expect(await readOutbox()).toHaveLength(1)
  await drainOutbox()
  expect(fetcher).toHaveBeenCalledTimes(2)
  expect(fetcher.mock.calls[1][0]).toBe('/api/projects/p/stories/s/chapters/3/comments')
  expect(JSON.parse(fetcher.mock.calls[1][1].body)).toEqual(input)
  expect(await readOutbox()).toHaveLength(0)
})

it('401 chờ cùng một sheet rồi gọi lại đúng request/body một lần', async () => {
  const fetcher = vi.fn().mockResolvedValueOnce(new Response('{"error":"Phiên hết hạn"}', { status: 401 })).mockResolvedValueOnce(new Response('{"ok":true,"action":"steer"}', { status: 200 }))
  vi.stubGlobal('fetch', fetcher)
  setUnauthorizedHandler(() => sessionGate.request())
  const request = sendInput('p', 's', 'Giữ lời góp ý đang gõ')
  await vi.waitFor(() => expect(sessionGate.isOpen()).toBe(true))
  expect(sessionGate.request()).toBe(sessionGate.request())
  sessionGate.complete()
  expect(await request).toEqual({ ok: true, action: 'steer' })
  expect(fetcher).toHaveBeenCalledTimes(2)
  expect(fetcher.mock.calls[1]).toEqual(fetcher.mock.calls[0])
  // A second 401 is surfaced, never an infinite retry loop.
  fetcher.mockReset().mockImplementation(() => Promise.resolve(new Response('{"error":"Không có phiên"}', { status: 401 })))
  const handler = vi.fn(async () => true); setUnauthorizedHandler(handler)
  await expect(me()).rejects.toMatchObject({ status: 401 })
  expect(fetcher).toHaveBeenCalledTimes(2); expect(handler).toHaveBeenCalledTimes(1)
})

it('danh sách tài khoản chỉ giữ tên/vai trò/lần cuối; bản nháp riêng trong bộ nhớ', () => {
  storage.set('ainovel.accounts', JSON.stringify([{ username: 'dat', name: 'Dat', role: 'admin', lastLoginAt: '2026-10-25', password: 'không giữ', device: 'Safari', cookie: 'bí mật' }]))
  expect(getRemembered()).toEqual([{ username: 'dat', name: 'Dat', role: 'admin', lastLoginAt: '2026-10-25' }])
  rememberAccount({ username: 'dat', name: 'Dat mới', role: 'admin', lastLoginAt: '2026-10-26', device: 'Chrome' })
  expect(JSON.parse(storage.get('ainovel.accounts')!)).toEqual([{ username: 'dat', name: 'Dat mới', role: 'admin', lastLoginAt: '2026-10-26' }])
  setDraftAccount('dat'); setDraft('p/s', 'Bản nháp của Dat')
  setDraftAccount('linh'); expect(getDraft('p/s')).toBe(''); setDraft('p/s', 'Bản nháp của Linh')
  setDraftAccount('dat'); expect(getDraft('p/s')).toBe('Bản nháp của Dat')
  storage.set('ainovel.accounts', '{lỗi'); expect(getRemembered()).toEqual([])
})

it('chuyển tài khoản giữ handler trước logout và chặn thu hồi phiên dự kiến', async () => {
  vi.spyOn(events, 'start').mockImplementation(() => {})
  const accept = vi.fn(async () => {})
  const unregister = registerAccountSession(accept)
  const user = { username: 'linh', name: 'Linh', role: 'member', projects: { p: 'collab' } }
  const fetcher = vi.fn(async (path: string) => {
    expect(accountTransition.isActive()).toBe(true)
    if (path === '/api/logout') { unregister(); return new Response('{"ok":true}', { status: 200 }) }
    return new Response(JSON.stringify({ user }), { status: 200 })
  })
  vi.stubGlobal('fetch', fetcher)
  const result = await switchAccount({ username: 'linh', password: 'mat-khau-moi' })
  expect(result.user).toEqual(user)
  expect(fetcher.mock.calls.map(([path]) => path)).toEqual(['/api/logout', '/api/login'])
  expect(accept).toHaveBeenCalledWith(user)
  expect(accountTransition.isActive()).toBe(false)
})
