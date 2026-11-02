// Màn Duyệt đọc danh sách video chờ duyệt qua cache dùng chung và làm mới bằng
// sự kiện SSE — không còn vòng poll 2,5 s. Các bài dưới kiểm tra hành vi thật:
//  · quay lại tab trong hạn dùng không gọi lại `GET /api/clone?channel=all`;
//  · đứng yên 10 s không phát request nền nào;
//  · sự kiện job (bảng `keysForEvent`) làm mới danh sách; tab ẩn gắn cờ cũ,
//    hiện lại thì tải đúng một lần;
//  · `image.progress` ghép trạng thái đang vẽ, xong/lỗi thì đọc lại ảnh.
import { Activity, act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cache, invalidate } from '@noverse/ui/data'
import { keysForEvent } from '../../data/eventKeys'
import { keys } from '../../data/keys'
import { useReview, type ReviewState } from './useReview'
import { ApiError } from '../../api/request'
Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

const api = vi.hoisted(() => ({
  request: vi.fn(),
  handlers: new Map<string, (data: unknown) => void>(),
}))
vi.mock('../../api/request', () => ({ request: api.request, ApiError: class extends Error { constructor(public status: number, message: string) { super(message) } } }))
vi.mock('@noverse/ui', () => ({ toast: { err: vi.fn() } }))
vi.mock('../../data/events', () => ({
  subscribeEvents: (type: string, handler: (data: unknown) => void) => {
    api.handlers.set(type, handler)
    return () => { api.handlers.delete(type) }
  },
  events: { onReconnect: () => () => {} },
}))

/** Video chờ duyệt mẫu: `j` có sẵn, `k` xuất hiện khi máy báo thêm. */
const reviewJob = { id: 'j', channelId: 'c', stageGroup: 'review', subline: '', createdAt: '2026-01-01', source: { title: 'j', thumbnailUrl: null } }
const nextJob = { ...reviewJob, id: 'k', createdAt: '2026-01-02', source: { title: 'k', thumbnailUrl: null } }
const image = { id: 'i', kind: 'scene', status: 'review', prompt: 'p', currentVersion: null, reviewVersion: 1, versions: [] }
const view = (status: string) => ({ manifest: { revision: imageRevision }, items: [{ ...image, status }], approved: false })

let serverJobs = [reviewJob]
let imageStatus = 'review'
let imageRevision = 2
let postConflict = false
let paneActive = true
let root: Root | undefined
let node: HTMLElement
const latest: { current?: ReviewState } = {}

const jobsCalls = () => api.request.mock.calls.filter(([url]) => url === '/api/clone?channel=all').length
const imageCalls = () => api.request.mock.calls.filter(([url, options]) => url === '/api/clone/j/images' && !options?.body).length
const flush = async () => {
  const { promise, resolve } = Promise.withResolvers<void>()
  setTimeout(resolve, 0)
  await act(async () => { await promise })
}

async function mount() {
  node = document.createElement('div'); document.body.append(node); root = createRoot(node)
  paneActive = true
  await act(async () => { root!.render(<Stage />) })
}
async function unmount() {
  await act(async () => { root!.unmount() })
  root = undefined
}
/** Ẩn/hiện pane giữ-alive: state còn, effect (SSE/timer) bị gỡ rồi gắn lại. */
async function show(active: boolean) {
  paneActive = active
  await act(async () => { root!.render(<Stage />) })
}
function Stage() {
  return <Activity mode={paneActive ? 'visible' : 'hidden'}><Harness /></Activity>
}
function Harness() {
  const review = useReview()
  latest.current = review
  return <>
    <span data-jobs>{review.jobs ? review.jobs.map(entry => entry.id).join(',') : 'loading'}</span>
    <span data-status>{review.images.j?.items[0]?.status ?? 'none'}</span>
    <button onClick={() => void review.loadImages('j')}>tải</button>
    <button onClick={() => void review.mutate('j', null, 'approveAll')}>duyệt</button>
  </>
}

beforeEach(() => {
  cache.clear()
  api.handlers.clear()
  serverJobs = [reviewJob]
  imageStatus = 'review'
  imageRevision = 2
  postConflict = false
  api.request.mockImplementation((url: string, options?: { body?: object }) => {
    if (options?.body) {
      const expectedRevision = 'expectedRevision' in options.body ? options.body.expectedRevision : undefined
      return postConflict || expectedRevision !== imageRevision
        ? Promise.reject(new ApiError(409, 'Danh sách ảnh đã thay đổi; tải lại trước khi sửa'))
        : Promise.resolve({ revision: imageRevision + 1 })
    }
    if (url === '/api/clone?channel=all') return Promise.resolve({ jobs: serverJobs })
    if (url === '/api/clone/j/images') return Promise.resolve(view(imageStatus))
    return Promise.resolve({})
  })
})
afterEach(() => {
  if (root) act(() => root!.unmount())
  root = undefined
  document.body.innerHTML = ''
  cache.clear()
  vi.clearAllMocks()
  vi.useRealTimers()
})

describe('Duyệt — cache và sự kiện', () => {
  it('quay lại tab trong hạn dùng không gọi lại danh sách video', async () => {
    await mount()
    expect(latest.current!.jobs?.map(entry => entry.id)).toEqual(['j'])
    expect(jobsCalls()).toBe(1)

    await unmount()
    await mount()

    expect(latest.current!.jobs?.map(entry => entry.id)).toEqual(['j'])
    expect(jobsCalls()).toBe(1)
  })

  it('đứng yên 10 giây không phát request nền nào', async () => {
    await mount()
    const before = jobsCalls()
    vi.useFakeTimers()
    await act(async () => { vi.advanceTimersByTime(10_000) })
    vi.useRealTimers()
    expect(jobsCalls()).toBe(before)
  })

  it('sự kiện job làm mới danh sách qua bảng sự kiện', async () => {
    await mount()
    expect(jobsCalls()).toBe(1)
    serverJobs = [reviewJob, nextJob]

    // Đúng việc `startDataEvents` làm khi nhận clone.job.updated.
    await act(async () => {
      const eventKeys = keysForEvent({ type: 'clone.job.updated', data: { jobId: 'k' } })
      if (eventKeys === '*') await invalidate()
      else for (const key of eventKeys) await invalidate(key)
    })

    expect(latest.current!.jobs?.map(entry => entry.id)).toEqual(['j', 'k'])
    expect(jobsCalls()).toBe(2)
  })

  it('pane ẩn: sự kiện đánh dấu cũ, hiện lại tải lại danh sách và ảnh đã mở', async () => {
    await mount()
    await act(async () => { node.querySelector('button')!.click() })
    await flush()
    expect(jobsCalls()).toBe(1)
    expect(imageCalls()).toBe(1)

    await show(false)
    await flush() // nhả khoá cache sau khi pane ẩn
    expect(api.handlers.size).toBe(0)

    // Trong lúc pane ẩn, cầu sự kiện toàn cục invalidate khoá danh sách.
    serverJobs = [nextJob]
    await act(async () => { await invalidate(keys.reviewJobs()) })

    await show(true)
    await flush()
    expect(latest.current!.jobs?.map(entry => entry.id)).toEqual(['k'])
    expect(jobsCalls()).toBe(2)
    expect(imageCalls()).toBe(2)
  })

  it('duyệt cả video bị 409: hoàn tác video và đọc lại ảnh để revision khớp', async () => {
    await mount()
    await act(async () => { node.querySelector('button')!.click() })
    await flush()
    const before = imageCalls()
    expect(latest.current!.jobs?.map(entry => entry.id)).toEqual(['j'])

    postConflict = true
    imageRevision = 3
    await act(async () => { [...node.querySelectorAll('button')].find(button => button.textContent === 'duyệt')!.click() })
    await flush()

    expect(latest.current!.jobs?.map(entry => entry.id)).toEqual(['j'])
    expect(imageCalls()).toBe(before + 1)
    expect(latest.current!.images.j?.manifest?.revision).toBe(3)

    postConflict = false
    let retried = false
    await act(async () => { retried = await latest.current!.mutate('j', null, 'approveAll') })
    expect(retried).toBe(true)
    expect(latest.current!.jobs).toEqual([])
  })

  it('keeps an image prefetch that finishes while the pane is hidden', async () => {
    await mount()
    const normalRequest = api.request.getMockImplementation()!
    const { promise: pendingImage, resolve: finish } = Promise.withResolvers<unknown>()
    api.request.mockImplementation((url: unknown, options?: unknown) => {
      if (String(url) === '/api/clone/j/images') return pendingImage
      return normalRequest(url, options)
    })
    let loading!: Promise<void>
    await act(async () => { loading = latest.current!.loadImages('j') })
    await show(false)
    finish(view(imageStatus))
    await act(async () => { await loading })
    await show(true)
    await flush()
    expect(node.querySelector('[data-status]')?.textContent).toBe('review')
    expect(latest.current!.images.j?.manifest?.revision).toBe(2)
    expect(imageCalls()).toBe(1)
  })

  it('catches up a hidden image changed by an event even while its TTL is fresh', async () => {
    await mount()
    await act(async () => { await latest.current!.loadImages('j') })
    await show(false)
    imageRevision = 3
    imageStatus = 'approved'
    const dirty = keysForEvent({ type: 'image.progress', data: { jobId: 'j', imageId: 'i', n: 3, phase: 'done' } })
    await act(async () => { if (dirty === '*') await invalidate(); else for (const key of dirty) await invalidate(key) })
    await show(true)
    await flush()
    expect(node.querySelector('[data-status]')?.textContent).toBe('approved')
    expect(latest.current!.images.j?.manifest?.revision).toBe(3)
  })

  it('image.progress ghép trạng thái đang vẽ, xong thì đọc lại ảnh', async () => {
    await mount()
    await act(async () => { node.querySelector('button')!.click() })
    await flush()
    expect(imageCalls()).toBe(1)

    imageStatus = 'running'
    act(() => { api.handlers.get('image.progress')!({ jobId: 'j', imageId: 'i', n: 2, phase: 'drawing' }) })
    expect(node.querySelector('[data-status]')?.textContent).toBe('running')
    expect(imageCalls()).toBe(1)

    imageStatus = 'review'
    act(() => { api.handlers.get('image.progress')!({ jobId: 'j', imageId: 'i', n: 2, phase: 'done' }) })
    await flush()
    expect(imageCalls()).toBe(2)
    expect(node.querySelector('[data-status]')?.textContent).toBe('review')
  })
})
