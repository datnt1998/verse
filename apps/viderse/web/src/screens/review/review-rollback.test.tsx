import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { useReview } from './useReview'
Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

const api = vi.hoisted(() => ({ request: vi.fn(), error: vi.fn() }))
vi.mock('../../api/request', () => ({ request: api.request, ApiError: class extends Error { constructor(public status: number, message: string) { super(message) } } }))
vi.mock('@noverse/ui', () => ({ toast: { err: api.error } }))
vi.mock('@noverse/ui/data', () => ({ getData: () => undefined, setData: vi.fn(), invalidate: vi.fn() }))
const job = { id: 'j', channelId: 'c', stageGroup: 'review', subline: '', createdAt: '2026-01-01', source: { title: 'Video', thumbnailUrl: null } }
const image = { id: 'i', kind: 'scene', status: 'review', prompt: 'p', currentVersion: null, reviewVersion: 1, versions: [] }
let root: Root | undefined
afterEach(() => { if (root) act(() => root!.unmount()); root = undefined; document.body.innerHTML = ''; vi.clearAllMocks() })

describe('duyệt lạc quan', () => {
  it('ẩn video ngay rồi khôi phục và báo lỗi khi máy từ chối', async () => {
    let reject!: (reason: Error) => void
    const pending = new Promise((_resolve, fail) => { reject = fail })
    api.request.mockImplementation((url: string, options?: { body?: object }) => options?.body ? pending : url.includes('?channel=') ? Promise.resolve({ jobs: [job] }) : Promise.resolve({ manifest: { revision: 2 }, items: [image], approved: false }))
    function Harness() { const review = useReview(); return <><span>{review.jobs?.length ?? 'loading'}</span><button onClick={() => void review.loadImages('j')}>tải</button><button onClick={() => void review.mutate('j', null, 'approveAll')}>duyệt</button></> }
    const node = document.createElement('div'); document.body.append(node); root = createRoot(node)
    await act(async () => { root!.render(<Harness />) })
    const buttons = node.querySelectorAll('button')
    await act(async () => { buttons[0].click() })
    await act(async () => { buttons[1].click() })
    expect(node.querySelector('span')?.textContent).toBe('0')
    await act(async () => { reject(new Error('Không duyệt được')); await pending.catch(() => {}) })
    expect(node.querySelector('span')?.textContent).toBe('1')
    expect(api.error).toHaveBeenCalledWith('Không duyệt được')
  })
})
