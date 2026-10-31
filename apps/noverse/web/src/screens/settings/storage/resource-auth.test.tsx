// @vitest-environment happy-dom
import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { expect, it, vi } from 'vitest'
import { cache } from '@noverse/ui/data'
import { useStorage } from './resource'

vi.mock('../../../api/session', () => ({ useSession: () => ({ user: { role: 'member' } }) }))
Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })
it('never fetches system storage for non-admin page, header, or navigation consumers', async () => {
  vi.useFakeTimers()
  const fetcher = vi.spyOn(globalThis, 'fetch')
  const root = createRoot(document.createElement('div'))
  function Probe() { useStorage(true); useStorage(false, true, true); useStorage(); return null }
  try {
    await act(async () => root.render(<Probe />))
    await act(async () => { await vi.advanceTimersByTimeAsync(60000) })
    document.dispatchEvent(new Event('visibilitychange'))
    await act(async () => { await vi.advanceTimersByTimeAsync(30000) })
    expect(fetcher.mock.calls.filter(([url]) => String(url).includes('/api/system/storage'))).toHaveLength(0)
  } finally { await act(async () => root.unmount()); cache.clear(); vi.useRealTimers(); vi.restoreAllMocks() }
})
