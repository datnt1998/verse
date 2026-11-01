// @vitest-environment happy-dom
// Hồ sơ kênh › vùng xoá: số video đang làm lấy từ GET /api/clone?channel= (cùng
// khoá ChannelScreen đã tải). Có video đang làm → card hiện dòng cảnh báo, nút
// xoá bị khoá kèm lý do; hộp xác nhận cũng khoá nút chính vì cùng con số.
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cache } from '@noverse/ui/data'
import type { Channel } from '../../api/channels'
import { confirmDeleteChannel } from '../../components/confirm'
import { SettingsTab } from './SettingsTab'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })
vi.mock('@noverse/ui', async (importOriginal) => ({ ...(await importOriginal<object>()), toast: { ok: vi.fn(), err: vi.fn() } }))
vi.mock('../../api/channelVideos', () => ({
  getChannelJobs: async () => ({
    jobs: [{ id: 'a', stageGroup: 'working' }, { id: 'b', stageGroup: 'working' }, { id: 'c', stageGroup: 'ready' }],
    queuedTotal: 0,
    bulkEligibility: { retry: [], approve: [], bundle: [], markPosted: [], cancel: [], delete: [] },
  }),
}))

const channel = {
  id: 'c1', name: 'Kênh thử', youtubeUrl: null, language: 'en', alwaysTags: [], styleNote: null,
  descriptionFooter: null, sources: [], templates: [], pacePerDay: 2, bannerUrl: null, avatarUrl: null,
  identity: { state: 'ready' }, branding: { brief: '', look: '' },
} as unknown as Channel

let root: Root
let host: HTMLDivElement

beforeEach(() => {
  host = document.createElement('div')
  document.body.append(host)
  root = createRoot(host)
  vi.stubGlobal('fetch', vi.fn((input: RequestInfo | URL) => {
    const url = String(input)
    if (url.includes('/api/ai/providers')) return Promise.resolve(Response.json([]))
    if (url.includes('/api/ai/agents')) return Promise.resolve(Response.json({ director: { model: { provider: 'p', model: 'm' } }, artist: { draw: { provider: 'p', model: 'm' } } }))
    return Promise.resolve(Response.json({}))
  }))
})

afterEach(() => {
  act(() => root.unmount())
  host.remove()
  document.body.innerHTML = ''
  vi.unstubAllGlobals()
  cache.clear()
})

const settle = () => act(async () => {
  const { promise, resolve } = Promise.withResolvers<void>()
  setTimeout(resolve, 60)
  await promise
})

const button = (label: string) => [...document.querySelectorAll('button')].find((node) => node.textContent?.includes(label))

describe('Hồ sơ kênh › vùng xoá kênh', () => {
  it('đếm video đang làm từ dữ liệu kênh: hiện cảnh báo, khoá nút xoá và lối xem việc đang làm', async () => {
    await act(async () => { root.render(<SettingsTab channel={channel} />) })
    await settle()
    await settle()

    const card = document.querySelector('[aria-label="Xoá kênh khỏi Viderse"]') as HTMLElement
    expect(card.textContent).toContain('Đang có 2 video đang làm')
    expect(button('Xem 2 video đang làm')).toBeTruthy()

    const del = button('Xoá kênh')!
    expect(del.getAttribute('aria-disabled')).toBe('true')
    expect(document.body.textContent).toContain('Chờ 2 video đang làm xong')
  })

  it('hộp xác nhận cùng con số: nút chính khoá kèm lý do', async () => {
    await act(async () => { void confirmDeleteChannel({ name: 'Kênh thử', runningCount: 2, onConfirm: () => undefined }) })
    await settle()
    const confirmButton = [...document.querySelectorAll('button')].filter((node) => node.textContent?.trim() === 'Xoá kênh').pop()!
    expect(confirmButton.hasAttribute('disabled') || confirmButton.getAttribute('aria-disabled') === 'true').toBe(true)
    expect(document.body.textContent).toContain('Kênh có 2 video đang làm')
  })
})
