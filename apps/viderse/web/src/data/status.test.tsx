// @vitest-environment happy-dom
import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { afterEach, expect, it, vi } from 'vitest'
import { cache, invalidate } from '@noverse/ui/data'
import { useOnline } from './status'
import { keys } from './keys'
const events = vi.hoisted(() => ({ state: 'open' }))
vi.mock('./events', () => ({ useEventsStatus: () => events }))
function Connection() { const online = useOnline(); return <div data-online={online} /> }

afterEach(() => { events.state = 'open'; cache.clear(); vi.unstubAllGlobals(); vi.useRealTimers() })

async function mountedConnection() {
  vi.stubGlobal('IS_REACT_ACT_ENVIRONMENT', true)
  cache.clear()
  const el = document.createElement('div'); document.body.append(el); const root = createRoot(el)
  await act(async () => { root.render(<Connection />) })
  return { el, root, online: () => el.firstElementChild?.getAttribute('data-online'), close: async () => { await act(async () => root.unmount()); el.remove() } }
}

it('keeps a reachable server online during the ten-second event-stream grace period', async () => {
  events.state = 'retrying'
  vi.stubGlobal('fetch', vi.fn(async () => new Response(JSON.stringify({ running: 1, queued: 0 }))))
  const app = await mountedConnection()
  try { expect(app.online()).toBe('true') } finally { await app.close() }
})

it('requires two consecutive status failures and clears the failure streak on success', async () => {
  let fails = true
  vi.stubGlobal('fetch', vi.fn(async () => fails
    ? new Response(JSON.stringify({ error: { message: 'Unavailable' } }), { status: 503 })
    : new Response(JSON.stringify({ running: 0, queued: 0 }))))
  const app = await mountedConnection()
  try {
    expect(app.online()).toBe('true')
    await act(async () => { await invalidate(keys.status()) })
    expect(app.online()).toBe('false')
    fails = false
    await act(async () => { await invalidate(keys.status()) })
    expect(app.online()).toBe('true')
  } finally { await app.close() }
})

it('marks an SSE outage longer than ten seconds offline even while status polling succeeds', async () => {
  vi.useFakeTimers()
  events.state = 'retrying'
  vi.stubGlobal('fetch', vi.fn(async () => new Response(JSON.stringify({ running: 1, queued: 0 }))))
  const app = await mountedConnection()
  try {
    await act(async () => { await vi.advanceTimersByTimeAsync(10_000) })
    expect(app.online()).toBe('true')
    await act(async () => { await vi.advanceTimersByTimeAsync(1) })
    expect(app.online()).toBe('false')
    events.state = 'open'
    await act(async () => { app.root.render(<Connection />) })
    expect(app.online()).toBe('true')
  } finally { await app.close() }
})

it('reuses a status fetched less than a minute ago when a screen mounts again', async () => {
  vi.useFakeTimers()
  vi.stubGlobal('IS_REACT_ACT_ENVIRONMENT', true)
  const fetchMock = vi.fn(async () => new Response(JSON.stringify({ running: 1, queued: 0 })))
  vi.stubGlobal('fetch', fetchMock)
  const mount = async () => {
    const el = document.createElement('div'); document.body.append(el); const root = createRoot(el)
    await act(async () => { root.render(<Connection />) })
    return async () => { await act(async () => root.unmount()); el.remove() }
  }
  const closeFirst = await mount()
  await act(async () => { await vi.advanceTimersByTimeAsync(10) })
  const calls = fetchMock.mock.calls.length
  await closeFirst()
  await act(async () => { await vi.advanceTimersByTimeAsync(5_000) })
  const closeSecond = await mount()
  try {
    expect(calls).toBe(1)
    expect(fetchMock.mock.calls.length).toBe(calls)
  } finally { await closeSecond() }
})
