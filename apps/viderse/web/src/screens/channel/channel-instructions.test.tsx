// @vitest-environment happy-dom
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { cache, setData } from '@noverse/ui/data'
import type { Channel } from '../../api/channels'
import { keys } from '../../data/keys'
import { ChannelScreen } from './ChannelScreen'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })
vi.mock('../../data/status', () => ({ useStatus: () => ({ data: undefined }), useOnline: () => true }))

const channel = (id: string, scout: string): Channel => ({
  id, name: id, youtubeUrl: null, language: 'vi', styleNote: null, descriptionFooter: null,
  alwaysTags: [], autoFetch: false, autoApprove: false, perDay: 5,
  defaultTemplateId: 'standard', defaultTemplateName: 'Mẫu chuẩn',
  agentInstructions: { scout, director: 'Giữ tiêu đề ngắn' },
  branding: { brief: '', summary: '', look: '', avatar: '', banner: '' },
  avatarUrl: null, bannerUrl: null, sources: [], identity: { state: 'ready' },
  createdAt: '2026-10-01T00:00:00Z', updatedAt: '2026-10-01T00:00:00Z',
})
let root: Root
let host: HTMLDivElement
const render = async (id: string) => {
  await act(async () => { root.render(<ChannelScreen route={{ name: 'channel', channelId: id, tab: 'templates' }} />) })
}
const input = () => host.querySelector<HTMLInputElement>('#vch-composer')!
const type = async (value: string) => {
  await act(async () => {
    Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')!.set!.call(input(), value)
    input().dispatchEvent(new Event('input', { bubbles: true }))
  })
}
beforeEach(() => {
  cache.clear()
  vi.stubGlobal('matchMedia', vi.fn(() => ({ matches: false, addEventListener() {}, removeEventListener() {} })))
  const channels = [channel('c1', 'Chỉ lấy video dài'), channel('c2', 'Chỉ lấy nguồn của kênh hai')]
  vi.stubGlobal('fetch', vi.fn(async (url: string) => {
    if (url === '/api/owner-channels/c1' || url === '/api/owner-channels/c2') return Response.json({ error: 'Không lưu được chỉ dẫn' }, { status: 500 })
    return Response.json({ error: 'Endpoint không thuộc bài test chỉ dẫn' }, { status: 404 })
  }))
  setData(keys.channels(), { channels })
  setData(keys.templates(), { items: [] })
  for (const item of channels) {
    setData(keys.channel(item.id), item)
    setData(keys.channelJobs(item.id), { jobs: [] })
  }
  host = document.createElement('div'); document.body.append(host); root = createRoot(host)
})
afterEach(async () => {
  await act(async () => root.unmount())
  host.remove(); cache.clear(); vi.unstubAllGlobals()
})

it('loads saved instructions for the initial recipient', async () => {
  await render('c1')
  expect(input().value).toBe('Chỉ lấy video dài')
})
it('never carries a draft into another channel', async () => {
  await render('c1')
  await type('Bản nháp riêng kênh một')
  await render('c2')
  expect(input().value).toBe('Chỉ lấy nguồn của kênh hai')
})
it('keeps an editable draft when saving fails', async () => {
  await render('c1')
  await type('Bản nháp cần thử lại')
  await act(async () => { host.querySelector<HTMLButtonElement>('button[type="submit"]')!.click() })
  expect(host.querySelector('[role="alert"]')).not.toBeNull()
  expect(input().value).toBe('Bản nháp cần thử lại')
  expect(input().disabled).toBe(false)
})
