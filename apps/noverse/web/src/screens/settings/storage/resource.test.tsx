// @vitest-environment happy-dom
import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { afterEach, expect, it, vi } from 'vitest'
import type { StorageView } from '../../../api/storage-settings'
import { cache } from '@noverse/ui/data'
import { storageKey, useStorage } from './resource'

vi.mock('../../../api/session', () => ({ useSession: () => ({ user: { role: 'admin' } }) }))
const { fetchStorage } = vi.hoisted(() => ({ fetchStorage: vi.fn() }))
vi.mock('../../../api/client', () => ({ getStorage: fetchStorage }))
Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })
afterEach(() => { cache.clear(); vi.useRealTimers(); vi.restoreAllMocks() })
const view: StorageView = { mode: 'postgres', encryption: { available: true }, operations: [], migration: null }

it('starts on migration, stops idle/hidden, resumes visible, and disposes on unmount', async () => {
  vi.useFakeTimers()
  let hidden = false
  vi.spyOn(document, 'hidden', 'get').mockImplementation(() => hidden)
  let current = view
  fetchStorage.mockImplementation(async () => current)
  cache.setData(storageKey, current)
  const host = document.createElement('div'); const root = createRoot(host)
  function Probe() { useStorage(true); return null }
  try {
    await act(async () => root.render(<Probe />))
    await act(async () => { await vi.advanceTimersByTimeAsync(15000) })
    expect(fetchStorage).not.toHaveBeenCalled()
    current = { ...view, migration: { id: 'migration', state: 'running', copiedCount: 0, copiedBytes: 0, totalCount: 2, totalBytes: 20, bytesPerSecond: 0, errorCount: 0, errors: [], startedAt: new Date().toISOString() } }
    await act(async () => cache.setData(storageKey, current))
    await act(async () => { await vi.advanceTimersByTimeAsync(5000) })
    expect(fetchStorage).toHaveBeenCalledTimes(1)
    hidden = true; document.dispatchEvent(new Event('visibilitychange'))
    await act(async () => { await vi.advanceTimersByTimeAsync(15000) })
    expect(fetchStorage).toHaveBeenCalledTimes(1)
    hidden = false; document.dispatchEvent(new Event('visibilitychange'))
    await act(async () => { await vi.advanceTimersByTimeAsync(5000) })
    expect(fetchStorage).toHaveBeenCalledTimes(2)
    current = view
    await act(async () => cache.setData(storageKey, current))
    await act(async () => { await vi.advanceTimersByTimeAsync(15000) })
    expect(fetchStorage).toHaveBeenCalledTimes(2)
    await act(async () => root.unmount())
    await vi.advanceTimersByTimeAsync(15000)
    expect(fetchStorage).toHaveBeenCalledTimes(2)
  } finally { await act(async () => root.unmount()) }
})

it('shares one polling lease across page and header consumers', async () => {
  vi.useFakeTimers()
  fetchStorage.mockClear()
  vi.spyOn(document, 'hidden', 'get').mockReturnValue(false)
  const current = { ...view, migration: { id: 'migration', state: 'running' as const, copiedCount: 0, copiedBytes: 0, totalCount: 2, totalBytes: 20, bytesPerSecond: 0, errorCount: 0, errors: [], startedAt: new Date().toISOString() } }
  cache.setData(storageKey, current); fetchStorage.mockResolvedValue(current)
  const root = createRoot(document.createElement('div'))
  function Probe() { useStorage(true); useStorage(true); useStorage(false, true, true); return null }
  try {
    await act(async () => root.render(<Probe />))
    await act(async () => { await vi.advanceTimersByTimeAsync(30000) })
    expect(fetchStorage).toHaveBeenCalledTimes(6)
  } finally { await act(async () => root.unmount()) }
})
