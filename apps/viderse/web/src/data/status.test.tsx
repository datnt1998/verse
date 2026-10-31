// @vitest-environment happy-dom
import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { expect, it, vi } from 'vitest'
import { cache } from '@noverse/ui/data'
import { useOnline } from './status'
vi.mock('./events', () => ({ useEventsStatus: () => ({ state: 'backoff' }) }))
function Connection() { const online = useOnline(); return <div data-online={online} /> }
it('keeps a reachable server online while status polling replaces the event stream', async () => {
  cache.clear()
  vi.stubGlobal('fetch', vi.fn(async () => new Response(JSON.stringify({ running: 1, queued: 0 }))))
  const el = document.createElement('div'); document.body.append(el); const root = createRoot(el)
  try {
    await act(async () => { root.render(<Connection />) })
    expect(el.firstElementChild?.getAttribute('data-online')).toBe('true')
  } finally {
    await act(async () => root.unmount()); el.remove(); cache.clear(); vi.unstubAllGlobals()
  }
})
