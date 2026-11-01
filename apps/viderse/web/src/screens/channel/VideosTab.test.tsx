// @vitest-environment happy-dom
// Kênh › Video: lọc theo nhóm của GET /api/clone?channel=, Chọn nhiều + thanh
// thao tác đếm đúng theo `bulkEligibility`, và Tải gói đã xong chỉ gói video đủ điều kiện.
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cache } from '@noverse/ui/data'
import type { Channel } from '../../api/channels'
import type { ChannelJobs } from '../../api/channelVideos'
import type { Resource } from '../../data/useResource'
import { VideosTab } from './VideosTab'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })
vi.mock('@noverse/ui', async (importOriginal) => ({ ...(await importOriginal<object>()), toast: { ok: vi.fn(), err: vi.fn() } }))

const job = (id: string, stageGroup: string) => ({
  id, channelId: 'c1', stageGroup, templateId: 't1', subline: '', steps: null, exportBytes: null,
  source: { title: `Video ${id}`, channelTitle: 'Nguồn', durationSec: 600, url: 'https://youtube.com/watch?v=x' },
  createdAt: '2026-10-12T09:00:00Z', updatedAt: '2026-10-12T09:00:00Z', postedAt: null,
})
const jobs = (list: Record<string, unknown>[], bundle: string[]) => ({
  data: {
    jobs: list, queuedTotal: 0,
    bulkEligibility: { retry: ['attention'], approve: ['review'], bundle, markPosted: ['ready'], cancel: ['working'], delete: ['review', 'attention', 'ready'] },
  } as unknown as ChannelJobs,
  error: undefined, status: 'ready', key: ['channels', 'jobs', 'c1'], initialLoading: false, placeholder: false, refresh: vi.fn(async () => undefined),
}) as unknown as Resource<ChannelJobs>
const channel = { id: 'c1', name: 'Kênh thử', youtubeUrl: null, autoPick: false, createdAt: '2026-10-12T09:00:00Z' } as unknown as Channel

let root: Root
let host: HTMLDivElement
let dock: HTMLDivElement
beforeEach(() => {
  host = document.createElement('div')
  dock = document.createElement('div')
  document.body.append(host, dock)
  root = createRoot(host)
  vi.stubGlobal('fetch', vi.fn(() => Promise.resolve(Response.json({ items: [], posted: [], insights: [], needMore: 3, counted: 0 }))))
})
afterEach(() => { act(() => root.unmount()); host.remove(); dock.remove(); vi.unstubAllGlobals(); cache.clear?.() })

const settle = () => act(async () => {
  const { promise, resolve } = Promise.withResolvers<void>()
  setTimeout(resolve, 20)
  await promise
})
const mount = async (resource: Resource<ChannelJobs>) => {
  await act(async () => { root.render(<VideosTab channel={channel} rival={undefined} jobs={resource} dockSlot={dock} onSelecting={() => {}} />) })
  await settle()
}
const titles = () => [...host.querySelectorAll('[role=row] a[role=cell]')].map((cell) => cell.querySelector('span[title]')?.getAttribute('title') ?? '')
const chip = (name: string) => [...host.querySelectorAll<HTMLElement>('[role=radio]')].find((el) => el.textContent?.startsWith(name))!
const click = (el: Element) => act(async () => { el.dispatchEvent(new MouseEvent('click', { bubbles: true, cancelable: true })) })
const all = [job('working', 'working'), job('review', 'review'), job('attention', 'attention'), job('queued', 'queued'), job('ready', 'ready'), job('posted', 'posted')]

describe('Kênh › Video', () => {
  it('chip lọc theo nhóm của máy chủ: Chờ bạn = Chờ duyệt + Cần xử lý; Tất cả xếp đang làm trước, đã đăng cuối', async () => {
    await mount(jobs([all[5], all[4], all[2], all[0], all[3], all[1]], []))
    expect(titles()).toEqual(['Video working', 'Video review', 'Video attention', 'Video queued', 'Video ready', 'Video posted'])
    await click(chip('Chờ bạn'))
    expect(titles().join('|')).toContain('Video review')
    expect(titles().join('|')).toContain('Video attention')
    expect(titles()).toHaveLength(2)
    await click(chip('Đã đăng'))
    expect(titles()).toHaveLength(1)
    expect(chip('Chờ bạn').textContent).toContain('2')
  })

  it('Chọn nhiều: chọn 1 Chờ duyệt + 1 Cần xử lý + 1 Sẵn sàng → Thử lại 1 · Duyệt & dựng 1 · Tải gói 1 · Đánh dấu đã đăng 1; Esc thoát', async () => {
    await mount(jobs(all, ['ready']))
    const toggle = [...host.querySelectorAll('button')].find((el) => el.textContent?.trim() === 'Chọn nhiều')!
    await click(toggle)
    for (const id of ['review', 'attention', 'ready']) await click(host.querySelector(`[role=checkbox][aria-label="Chọn Video ${id}"]`)!)
    const bar = dock.querySelector('[role=toolbar]')!
    expect(bar.textContent).toContain('Đã chọn 3')
    const counts = Object.fromEntries([...bar.querySelectorAll('button[title]')].map((el) => [el.textContent?.replace(/\d+$/, ''), el.textContent?.match(/\d+$/)?.[0]]))
    expect(counts).toMatchObject({ 'Thử lại': '1', 'Duyệt & dựng': '1', 'Tải gói': '1', 'Đánh dấu đã đăng': '1' })
    await act(async () => { window.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' })) })
    expect(dock.querySelector('[role=toolbar]')).toBeNull()
  })

  it('Shift-click chọn cả khoảng theo thứ tự đang hiện', async () => {
    await mount(jobs(all, []))
    await click([...host.querySelectorAll('button')].find((el) => el.textContent?.trim() === 'Chọn nhiều')!)
    await click(host.querySelector('[role=checkbox][aria-label="Chọn Video working"]')!)
    await act(async () => { host.querySelector('[role=checkbox][aria-label="Chọn Video attention"]')!.dispatchEvent(new MouseEvent('click', { bubbles: true, shiftKey: true })) })
    expect(dock.querySelector('[role=toolbar]')!.textContent).toContain('Đã chọn 3')
  })

  it('Tải gói đã xong gói đúng các video đủ điều kiện; không có video nào thì khoá', async () => {
    const clicked: string[] = []
    const real = HTMLAnchorElement.prototype.click
    HTMLAnchorElement.prototype.click = function click() { clicked.push(this.getAttribute('href') ?? '') }
    try {
      await mount(jobs(all, ['ready', 'posted']))
      const bundle = () => [...host.querySelectorAll('button')].find((el) => el.textContent?.trim() === 'Tải gói đã xong')!
      expect(bundle().getAttribute('aria-disabled')).toBe('false')
      await click(bundle())
      await act(async () => { await new Promise((resolve) => setTimeout(resolve, 10)) })
      expect(clicked).toEqual(['/api/clone/download?ids=ready,posted'])
      await act(async () => { root.unmount() }); root = createRoot(host)
      await mount(jobs(all, []))
      expect(bundle().getAttribute('aria-disabled')).toBe('true')
    } finally { HTMLAnchorElement.prototype.click = real }
  })
})
