// Chi tiết video trên điện thoại: chạy tiếp / thử lại đi đúng endpoint thao tác
// của video (không tạo lượt mới tốn hạn mức ảnh), video chờ lượt có lời giải
// thích + lối mở Hàng chờ, và video đã đăng xoá được riêng file MP4 khi còn file.
// @vitest-environment happy-dom
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { CloneJobView } from '../../../api/workbench'
import { MobileWorkbenchDetail } from './MobileWorkbenchDetail'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

const state = vi.hoisted(() => ({
  online: true,
  refresh: vi.fn(),
  markPosted: vi.fn(),
  confirm: vi.fn(),
}))
vi.mock('@noverse/ui', async (importOriginal) => ({ ...(await importOriginal<object>()), confirm: state.confirm }))
vi.mock('../../../data/useResource', () => ({ useResource: () => ({ data: undefined, error: null, initialLoading: false, refresh: state.refresh }) }))
vi.mock('../../../data/events', () => ({ subscribeEvents: () => () => {} }))
vi.mock('../../../data/status', () => ({ useOnline: () => state.online }))
vi.mock('../../../components/performance', async (importOriginal) => ({ ...(await importOriginal<object>()), requestMarkPosted: state.markPosted }))
vi.mock('../../../router', () => ({ navigate: vi.fn(), routePath: (route: { name: string }) => `#/${route.name}` }))
vi.mock('../../../components', () => ({ DevelopEffect: () => null }))

const base = (overrides: Partial<CloneJobView> = {}): CloneJobView => ({
  id: 'job',
  channelId: 'c1',
  source: { id: 's1', url: '', title: 'Đêm ở trạm', channelId: 'c1', channelTitle: 'Kênh', thumbnailUrl: null, durationSec: 600 },
  sourceKey: 'k1',
  generation: 1,
  templateId: 't1',
  templateName: 'Mẫu chuẩn',
  status: 'running',
  stageGroup: 'working',
  activeStep: 'images',
  version: 1,
  noImages: false,
  subline: '',
  agentView: [],
  steps: [],
  ...overrides,
})

let root: Root | undefined
let host: HTMLDivElement
const render = async (job: CloneJobView) => {
  if (!root) {
    host = document.createElement('div')
    document.body.append(host)
    root = createRoot(host)
  }
  await act(async () => root!.render(<MobileWorkbenchDetail
    job={job} images={undefined} imagesLoading={false} imagesError={null} onBack={() => {}}
    onRefresh={state.refresh} onPlan={() => {}} onNotice={() => {}} planOpen={false} />))
  return host
}
const button = (text: string) => [...host.querySelectorAll('button')].find((entry) => entry.textContent?.includes(text))
const fetcher = vi.fn()
const lastPost = () => {
  const [url, init] = fetcher.mock.calls.at(-1) as unknown as [string, RequestInit]
  return { url, body: JSON.parse(String(init.body)) as Record<string, unknown> }
}

beforeEach(() => {
  state.online = true
  state.refresh.mockReset()
  state.confirm.mockReset().mockImplementation(async (options: { onConfirm: (value: string) => Promise<void> }) => { await options.onConfirm(''); return true })
  fetcher.mockReset().mockImplementation(async () => Response.json({ results: [{ ok: true }] }))
  vi.stubGlobal('fetch', fetcher)
})
afterEach(async () => {
  if (root) await act(async () => root!.unmount())
  root = undefined
  vi.unstubAllGlobals()
  document.body.replaceChildren()
})

describe('chạy tiếp và thử lại trên điện thoại', () => {
  it('video tạm dừng chạy tiếp bằng thao tác của video, không tạo lượt mới và không tốn hạn mức ảnh', async () => {
    await render(base({ stageGroup: 'paused', status: 'paused' }))
    await act(async () => button('Chạy tiếp')!.click())
    expect(fetcher).toHaveBeenCalledTimes(1)
    expect(lastPost()).toEqual({ url: '/api/clone/job', body: { action: 'resume' } })
    expect(state.refresh).toHaveBeenCalled()
  })

  it('video cần xử lý thử lại bằng thao tác run, không làm lại từ đầu', async () => {
    await render(base({ stageGroup: 'attention', status: 'failed', activeStep: 'images', subline: 'Hoạ sĩ dừng vì thiếu hạn mức ảnh.' }))
    await act(async () => button('Thử lại')!.click())
    expect(fetcher).toHaveBeenCalledTimes(1)
    expect(lastPost()).toEqual({ url: '/api/clone/job', body: { action: 'run' } })
  })

  it('máy chủ từ chối thì hiện đúng lời lỗi tiếng Việt và mở lại nút', async () => {
    fetcher.mockImplementation(async () => Response.json({ error: 'Máy chủ chưa chạy tiếp được.' }, { status: 409 }))
    await render(base({ stageGroup: 'paused', status: 'paused' }))
    await act(async () => button('Chạy tiếp')!.click())
    expect(host.querySelector('[role="alert"]')?.textContent).toContain('Máy chủ chưa chạy tiếp được.')
    expect(state.refresh).not.toHaveBeenCalled()
    expect(button('Chạy tiếp')?.hasAttribute('disabled')).toBe(false)
  })

  it('mất kết nối thì chưa gửi thao tác', async () => {
    state.online = false
    await render(base({ stageGroup: 'paused', status: 'paused' }))
    expect(button('Chạy tiếp')?.hasAttribute('disabled')).toBe(true)
    expect(fetcher).not.toHaveBeenCalled()
  })
})

describe('video đang chờ lượt', () => {
  it('giải thích đúng trạng thái chờ và mở được Hàng chờ thay vì khung 0 / 0 ảnh', async () => {
    await render(base({ stageGroup: 'queued', status: 'pending', queueItem: { noImages: false } }))
    expect(host.textContent).toContain('Video đang chờ lượt.')
    const queue = host.querySelector('a[href="#/queue"]')
    expect(queue?.textContent).toContain('Mở Hàng chờ')
    expect(host.textContent).not.toContain('0 / 0 ảnh')
    expect(host.querySelector('[aria-label="Phòng vẽ trực tiếp"]')).toBeNull()
  })
})

describe('xoá riêng file MP4 của video đã đăng', () => {
  it('có file xuất thì xoá đúng file MP4 qua luồng xoá MP4 đã có, không xoá cả video', async () => {
    await render(base({ stageGroup: 'posted', status: 'posted', exportBytes: 1048576 }))
    const remove = button('Xoá MP4')
    expect(remove).toBeTruthy()
    await act(async () => remove!.click())
    expect(state.confirm).toHaveBeenCalledOnce()
    expect(lastPost()).toEqual({ url: '/api/clone/bulk', body: { action: 'deleteExport', jobIds: ['job'] } })
  })

  it('không còn file xuất hoặc chưa đăng thì không mời xoá', async () => {
    await render(base({ stageGroup: 'posted', status: 'posted', exportBytes: null }))
    expect(button('Xoá MP4')).toBeUndefined()
    await act(async () => root!.render(<MobileWorkbenchDetail
      job={base({ stageGroup: 'ready', status: 'done', exportBytes: 1048576 })} images={undefined} imagesLoading={false} imagesError={null}
      onBack={() => {}} onRefresh={state.refresh} onPlan={() => {}} onNotice={() => {}} planOpen={false} />))
    expect(button('Xoá MP4')).toBeUndefined()
  })
})
