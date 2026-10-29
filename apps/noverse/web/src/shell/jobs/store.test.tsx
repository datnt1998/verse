// @vitest-environment happy-dom
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { Job } from '../../api/types'
import type { EventHandler } from '../../data/events'
import type { JobsState } from './store'
Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

const mocks = vi.hoisted(() => ({ getJobs: vi.fn(), handler: null as EventHandler | null }))
vi.mock('../../api/client', () => ({ ApiError: class extends Error {}, getJobs: mocks.getJobs }))
vi.mock('../../data/events', () => ({
  events: { onReconnect: () => () => {} },
  subscribeEvents: (_: unknown, handler: EventHandler) => { mocks.handler = handler; return () => { mocks.handler = null } },
}))
vi.mock('../../mobile/pwa', () => ({ notificationsEnabled: () => false, setNotificationUser: () => {} }))
vi.mock('@noverse/ui', async (importOriginal) => ({ ...(await importOriginal<object>()),  toast: {} }))

const job = (id = 'p/s/voice/1', finishedAt = '2026-10-27T10:00:00Z'): Job => ({
  id, finishedAt, kind: 'voice', title: 'Tập 1', status: 'done', progress: 1,
  href: '/', project: 'p', story: 's', startedAt: '2026-10-27T09:00:00Z',
})
let root: Root
let host: HTMLDivElement
let release: (() => void) | undefined
let snapshot: JobsState
async function mount(user = 'dat') {
  // Reload boundary: re-import a fresh store while retaining localStorage.
  const store = await import('./store')
  function Probe() { snapshot = store.useJobsState(); return null }
  await act(async () => { root.render(<Probe />); release = store.startJobs(user) })
  return store
}
async function emit(value: Job, type: 'job.done' | 'job.progress' = 'job.done') {
  await act(async () => { mocks.handler?.(value, { type, id: 'event' }) })
}
beforeEach(() => {
  vi.resetModules(); vi.useFakeTimers(); localStorage.clear()
  mocks.getJobs.mockResolvedValue({ jobs: [] })
  host = document.createElement('div'); document.body.append(host); root = createRoot(host)
})
afterEach(async () => {
  release?.(); await act(async () => root.unmount()); host.remove(); vi.clearAllTimers(); vi.useRealTimers()
})
describe('completed job announcements', () => {
  it('dismisses once per job + finishedAt, including replay and reload; a new completion still announces', async () => {
    let store = await mount()
    await emit(job())
    expect(snapshot.toasts).toHaveLength(1)
    await act(async () => { store.dismissToast(snapshot.toasts[0].id); await vi.advanceTimersByTimeAsync(600) })
    await emit({ ...job(), status: 'running', finishedAt: undefined }, 'job.progress')
    await emit(job())
    expect(snapshot.toasts).toHaveLength(0)
    release?.(); await act(async () => root.unmount()); root = createRoot(host)
    vi.resetModules(); store = await mount()
    await emit(job())
    expect(snapshot.toasts).toHaveLength(0)
    await emit(job('p/s/voice/1', '2026-10-27T11:00:00Z'))
    expect(snapshot.toasts).toHaveLength(1)
    await emit(job('p/s/voice/2'))
    expect(snapshot.toasts).toHaveLength(2)
  })
  it('keeps a bounded, user-scoped completion history and does not announce initial completed jobs', async () => {
    mocks.getJobs.mockResolvedValueOnce({ jobs: [job()] })
    await mount()
    await emit({ ...job(), status: 'running' }, 'job.progress'); await emit(job())
    expect(snapshot.toasts).toHaveLength(0)
    for (let n = 0; n < 110; n++) await emit(job(`p/s/voice/${n}`))
    const entries = JSON.parse(localStorage.getItem('ainovel.jobsAnnouncements.dat') ?? '[]')
    expect(entries).toHaveLength(100)
    release?.(); await act(async () => root.unmount()); root = createRoot(host)
    await mount('other'); await emit(job())
    expect(snapshot.toasts).toHaveLength(1)
  })
})

it('unrelated job SSE frames do not commit a narrow mobile job slice', async () => {
  const store = await import('./store') // Fresh-store boundary is intentional in this suite.
  let renders = 0
  const selectAudio = (state: JobsState) => state.jobs.filter((item) => item.kind === 'audio')
  function Probe() { store.useJobsSelector(selectAudio); renders++; return null }
  await act(async () => { root.render(<Probe />); release = store.startJobs('dat') })
  const baseline = renders
  await emit({ ...job(), status: 'running', finishedAt: undefined }, 'job.progress')
  expect(renders).toBe(baseline)
  await emit({ ...job('p/s/audio/1'), kind: 'audio', status: 'running', finishedAt: undefined }, 'job.progress')
  expect(renders).toBe(baseline + 1)
})

it('closed job sheets stop observing job changes while preserving exit content', async () => {
  const store = await import('./store') // Fresh-store boundary is intentional in this suite.
  let renders = 0
  function Probe({ open }: { open: boolean }) { store.useJobsState(open); renders++; return null }
  await act(async () => { root.render(<Probe open />); release = store.startJobs('dat') })
  await act(async () => root.render(<Probe open={false} />))
  const baseline = renders
  await emit({ ...job(), status: 'running', finishedAt: undefined }, 'job.progress')
  expect(renders).toBe(baseline)
})
