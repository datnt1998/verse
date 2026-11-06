// @vitest-environment happy-dom
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { cache, getData } from '@noverse/ui/data'
import { keys } from '../../data/keys'
import { useReview, type ReviewImage, type ReviewImages, type ReviewState } from './useReview'
import { ReviewScreen } from './ReviewScreen'
import type * as UI from '@noverse/ui'
import { dismissToasts, ToastHost } from '@noverse/ui'

const notices = vi.hoisted(() => ({
  toast: vi.fn<(text: string, tone: Parameters<typeof UI.toast>[1], action: { label: string; onClick: () => Promise<void> }, options: { undo: boolean; duration: number }) => void>(),
  error: vi.fn(),
}))
vi.mock('@noverse/ui', async importOriginal => {
  const actual = await importOriginal<typeof UI>()
  notices.toast.mockImplementation(actual.toast)
  notices.error.mockImplementation(actual.toast.err)
  return { ...actual, toast: Object.assign(notices.toast, { err: notices.error }) }
})
vi.mock('../../data/status', () => ({ useOnline: () => true }))
vi.mock('../../data/events', () => ({ subscribeEvents: () => () => {}, events: { onReconnect: () => () => {} } }))

const jobId = 'job/one'
const imageId = 'scene/one'
const endpoint = '/api/clone/job%2Fone/images'
const initial: ReviewImage = {
  id: imageId, kind: 'scene', status: 'review', prompt: 'Giữ nguyên chỉ dẫn',
  currentVersion: 1, reviewVersion: 3, skipReason: 'previous-reason',
  versions: [{ version: 1, cutout: null }, { version: 3, cutout: null }],
}
let root: Root
let host: HTMLDivElement
let latest: ReviewState
let server: ReviewImages
let beforeSkip: ReviewImage | undefined
let rejectSkip: boolean
let rejectUndo: boolean
let releasePost: (() => void) | undefined
let deferOp: string | undefined
let posts: Array<{ path: string; body: Record<string, unknown> }>
let reads: number
const json = (data: unknown, status = 200) => new Response(JSON.stringify(data), { status, headers: { 'Content-Type': 'application/json' } })
const receipt = () => notices.toast.mock.calls.at(-1)!
const undo = () => receipt()[2].onClick
function Harness() {
  latest = useReview()
  const item = latest.images[jobId]?.items[0]
  return <output>{JSON.stringify({ item, pending: !!latest.pending[jobId], revision: latest.images[jobId]?.manifest?.revision })}</output>
}
const skip = async () => {
  let accepted = false
  await act(async () => { accepted = await latest.mutate(jobId, latest.images[jobId].items[0], 'skip') })
  return accepted
}

beforeEach(async () => {
  vi.stubGlobal('IS_REACT_ACT_ENVIRONMENT', true)
  const actual = await vi.importActual<typeof UI>('@noverse/ui')
  notices.toast.mockImplementation(actual.toast)
  notices.error.mockImplementation(actual.toast.err)
  dismissToasts()
  cache.clear(); notices.toast.mockClear(); notices.error.mockClear()
  server = { manifest: { revision: 7 }, approved: false, items: [structuredClone(initial)] }
  beforeSkip = undefined; rejectSkip = false; rejectUndo = false; releasePost = undefined; deferOp = undefined; posts = []; reads = 0
  vi.stubGlobal('matchMedia', vi.fn((query: string) => ({ matches: false, media: query, addEventListener: vi.fn(), removeEventListener: vi.fn() })))
  vi.stubGlobal('fetch', vi.fn(async (path: string, init?: RequestInit) => {
    if (path === '/api/clone?channel=all') return json({ jobs: [{ id: jobId, channelId: 'c', stageGroup: 'review', subline: '', createdAt: '2026-11-05', source: { title: 'Video', thumbnailUrl: null } }] })
    if (path === '/api/owner-channels') return json({ channels: [{ id: 'c', name: 'Kênh', autoApprove: false }] })
    if (path === '/api/clone/counts') return json({ approvedToday: null })
    if (path === '/api/ai/providers') return json([])
    if (init?.method === 'POST') {
      const body = JSON.parse(String(init.body)) as Record<string, unknown>
      posts.push({ path, body })
      if (body.op === deferOp) {
        const { promise, resolve } = Promise.withResolvers<void>()
        releasePost = resolve
        await promise
      }
      if (body.expectedRevision !== server.manifest!.revision || body.op === 'skip' && rejectSkip || body.op === 'unskip' && rejectUndo) return json({ error: 'Danh sách ảnh đã thay đổi. Vui lòng tải lại.' }, 409)
      if (body.op === 'skip') {
        beforeSkip = structuredClone(server.items[0])
        server = { ...server, manifest: { revision: 12 }, items: [{ ...server.items[0], status: 'skipped', skipReason: 'user' }, ...server.items.slice(1)] }
      } else if (body.op === 'unskip') {
        if (!beforeSkip) return json({ error: 'Không có trạng thái trước khi bỏ ảnh.' }, 409)
        server = { ...server, manifest: { revision: 13 }, items: [beforeSkip, ...server.items.slice(1)] }
        beforeSkip = undefined
      } else throw new Error(`Unexpected operation ${String(body.op)}`)
      // Single-image route returns manifest; it need not be old revision + 1.
      return json({ manifest: server.manifest })
    }
    if (path === endpoint) { reads++; return json(server) }
    if (path === `${endpoint}/scene%2Fone`) return json({ item: server.items[0], revision: server.manifest!.revision, approved: server.approved })
    throw new Error(`Unexpected HTTP request ${path}`)
  }))
  host = document.createElement('div'); document.body.append(host); root = createRoot(host)
  await act(async () => root.render(<Harness />))
  await act(async () => { await latest.loadImages(jobId) })
})
afterEach(async () => {
  await act(async () => root.unmount())
  dismissToasts()
  host.remove(); cache.clear(); vi.restoreAllMocks(); vi.unstubAllGlobals()
})

it('restores the response state and shared cache, not an approved/latest-version guess', async () => {
  expect(await skip()).toBe(true)
  expect(notices.toast).toHaveBeenCalledTimes(1)
  expect(latest.images[jobId].items[0].status).toBe('skipped')
  const restore = undo()
  await act(async () => { await restore() })
  expect(posts).toEqual([
    { path: `${endpoint}/scene%2Fone`, body: { op: 'skip', expectedRevision: 7 } },
    { path: `${endpoint}/scene%2Fone`, body: { op: 'unskip', expectedRevision: 12 } },
  ])
  expect(latest.images[jobId].items[0]).toEqual(initial)
  expect(getData<ReviewImages>(keys.reviewImages(jobId))).toEqual(latest.images[jobId])
  expect(host.textContent).toContain('"reviewVersion":3')
  expect(host.textContent).toContain('"currentVersion":1')
  expect(host.textContent).toContain('"revision":13')
  expect(notices.error).not.toHaveBeenCalled()
  const count = posts.length
  await act(async () => { await restore() })
  expect(posts).toHaveLength(count)
  expect(notices.error).toHaveBeenLastCalledWith('Ảnh này đã được hoàn tác.')
})

it('holds the same pending lock through unskip and server readback without optimistic approved/running state', async () => {
  await skip()
  const restore = undo()
  deferOp = 'unskip'
  let restoring: Promise<void>
  await act(async () => { restoring = restore(); await Promise.resolve() })
  expect(latest.pending[jobId]).toBe(true)
  expect(latest.images[jobId].items[0].status).toBe('skipped')
  let accepted = true
  await act(async () => { accepted = await latest.mutate(jobId, latest.images[jobId].items[0], 'skip') })
  expect(accepted).toBe(false)
  expect(posts).toHaveLength(2)
  await act(async () => { releasePost!(); await restoring! })
  expect(latest.pending[jobId]).toBe(false)
  expect(latest.images[jobId].items[0]).toEqual(initial)
})

it('does not emit an undo receipt for a failed skip, and refreshes a conflict', async () => {
  rejectSkip = true
  server = { ...server, manifest: { revision: 8 } }
  expect(await skip()).toBe(false)
  expect(notices.toast).not.toHaveBeenCalled()
  expect(notices.error).toHaveBeenCalledWith('Danh sách ảnh đã thay đổi. Vui lòng tải lại.')
  expect(latest.images[jobId].items[0]).toEqual(initial)
  expect(latest.images[jobId].manifest!.revision).toBe(8)
})

it('deduplicates a pending skip and emits one undo receipt only after success', async () => {
  deferOp = 'skip'
  let first: Promise<boolean>
  await act(async () => { first = latest.mutate(jobId, latest.images[jobId].items[0], 'skip'); await Promise.resolve() })
  expect(latest.pending[jobId]).toBe(true)
  expect(notices.toast).not.toHaveBeenCalled()
  let duplicate = true
  await act(async () => { duplicate = await latest.mutate(jobId, latest.images[jobId].items[0], 'skip') })
  expect(duplicate).toBe(false)
  expect(posts).toHaveLength(1)
  await act(async () => { releasePost!(); expect(await first!).toBe(true) })
  expect(notices.toast).toHaveBeenCalledTimes(1)
  expect(latest.pending[jobId]).toBe(false)
})

it('keeps the skip response revision when a later server change is read and rejects stale undo truthfully', async () => {
  await skip()
  const restore = undo()
  server = { ...server, manifest: { revision: 20 }, items: [{ ...initial, status: 'approved', currentVersion: 3, reviewVersion: null }] }
  await act(async () => { await latest.loadImages(jobId, true) })
  const before = reads
  await act(async () => { await restore() })
  expect(posts.at(-1)!.body).toEqual({ op: 'unskip', expectedRevision: 12 })
  expect(reads).toBeGreaterThan(before)
  expect(latest.images[jobId].items[0]).toEqual(server.items[0])
  expect(getData<ReviewImages>(keys.reviewImages(jobId))!.manifest!.revision).toBe(20)
  expect(notices.error).toHaveBeenCalledTimes(1)
  expect(latest.pending[jobId]).toBe(false)
})

it('refreshes a rejected unskip without fabricating restoration', async () => {
  await skip(); rejectUndo = true
  const before = reads
  await act(async () => { await undo()() })
  expect(reads).toBeGreaterThan(before)
  expect(latest.images[jobId].items[0].status).toBe('skipped')
  expect(notices.error).toHaveBeenCalledTimes(1)
})

it('keeps visible toast undo functional after leaving review and refreshes the shared cache', async () => {
  await act(async () => root.render(<><Harness /><ToastHost /></>))
  await act(async () => { await latest.loadImages(jobId) })
  await skip()
  await act(async () => root.render(<><p>Đã chuyển sang màn khác</p><ToastHost /></>))
  expect(host.textContent).toContain('Đã bỏ ảnh này.')
  const control = [...host.querySelectorAll('button')].find(button => button.textContent?.trim().startsWith('Hoàn tác'))
  expect(control).toBeDefined()
  const before = reads
  await act(async () => { control!.click() })
  expect(posts.at(-1)).toEqual({ path: `${endpoint}/scene%2Fone`, body: { op: 'unskip', expectedRevision: 12 } })
  expect(reads).toBeGreaterThan(before)
  expect(getData<ReviewImages>(keys.reviewImages(jobId))!.items[0]).toEqual(initial)
  expect(getData<ReviewImages>(keys.reviewImages(jobId))!.manifest!.revision).toBe(13)
  expect(host.textContent).toContain('Đã chuyển sang màn khác')
  expect(notices.error).not.toHaveBeenCalled()
})

it.each([false, true])('uses the real %s mobile review consumer with one receipt, shared pending controls and server-backed restoration', async mobile => {
  server = { ...server, items: [structuredClone(initial), { ...structuredClone(initial), id: 'scene/two', sceneId: '2' }] }
  vi.stubGlobal('matchMedia', vi.fn((query: string) => ({ matches: mobile && query.includes('max-width'), media: query, addEventListener: vi.fn(), removeEventListener: vi.fn() })))
  await act(async () => { await latest.loadImages(jobId, true) })
  await act(async () => root.render(<ReviewScreen route={{ name: 'review', video: jobId, img: imageId }} />))
  const skipLabel = mobile ? 'Bỏ ảnh này' : 'Bỏ ảnh'
  const findSkip = () => [...host.querySelectorAll('button')].find(button => button.textContent?.trim() === skipLabel)
  expect(findSkip()).toBeDefined()
  await act(async () => { findSkip()!.click() })
  expect(notices.toast).toHaveBeenCalledTimes(1)
  expect(getData<ReviewImages>(keys.reviewImages(jobId))!.items[0].status).toBe('skipped')
  const restore = undo()
  deferOp = 'unskip'
  let restoring: Promise<void>
  await act(async () => { restoring = restore(); await Promise.resolve() })
  expect(findSkip()?.disabled).toBe(true)
  expect(posts.at(-1)!.body).toEqual({ op: 'unskip', expectedRevision: 12 })
  await act(async () => { releasePost!(); await restoring! })
  expect(getData<ReviewImages>(keys.reviewImages(jobId))!.items[0]).toEqual(initial)
  expect(findSkip()?.disabled).toBe(false)
  expect(notices.error).not.toHaveBeenCalled()
})
