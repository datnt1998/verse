// @vitest-environment happy-dom
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { cache, setData } from '@noverse/ui/data'
import type { Channel } from '../../api/channels'
import type { ChannelJob, ChannelJobs } from '../../api/channelVideos'
import { keys } from '../../data/keys'
import { ChannelScreen } from './ChannelScreen'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })
vi.mock('../../data/status', () => ({ useStatus: () => ({ data: undefined }), useOnline: () => true }))
const channel = (id: string): Channel => ({
  id, name: id, youtubeUrl: null, language: 'vi', styleNote: null, descriptionFooter: null,
  alwaysTags: [], autoFetch: false, autoApprove: false, perDay: 5,
  defaultTemplateId: 'standard', defaultTemplateName: 'Mẫu chuẩn', agentInstructions: {},
  branding: { brief: '', summary: '', look: '', avatar: '', banner: '' },
  avatarUrl: null, bannerUrl: null, sources: [], identity: { state: 'ready' },
  createdAt: '2026-11-08T00:00:00Z', updatedAt: '2026-11-08T00:00:00Z',
})
const job = (id: string, owner: string): ChannelJob => ({
  id, channelId: owner, sourceKey: id, generation: 1, templateId: 'standard',
  status: 'succeeded', stageGroup: 'ready', activeStep: null, version: 1, noImages: false,
  source: { id, title: `Video ${id}`, thumbnailUrl: null, url: 'https://example.com/video', channelId: null, channelTitle: null, durationSec: 60 },
  subline: 'Sẵn sàng đăng', agentView: null, steps: null,
  createdAt: '2026-11-08T00:00:00Z', updatedAt: '2026-11-08T01:00:00Z',
})
const jobsResponse = (jobs: ChannelJob[]): ChannelJobs => ({
  jobs, queuedTotal: 0,
  bulkEligibility: { retry: [], approve: [], bundle: [], markPosted: [], cancel: [], delete: [] },
})
let root: Root
let host: HTMLDivElement
let loadJobs: (id: string) => Promise<ChannelJobs>
const render = async (id: string) => {
  await act(async () => root.render(<ChannelScreen route={{ name: 'channel', channelId: id }} />))
}
const flush = async () => { await act(async () => { await new Promise(resolve => setTimeout(resolve, 20)) }) }
const list = () => host.querySelector('section[aria-label="Video của kênh"]')
beforeEach(() => {
  cache.clear()
  vi.stubGlobal('matchMedia', vi.fn(() => ({ matches: false, addEventListener() {}, removeEventListener() {} })))
  loadJobs = async () => jobsResponse([])
  vi.stubGlobal('fetch', vi.fn(async (input: string) => {
    const url = new URL(input, 'http://localhost')
    if (url.pathname === '/api/clone') {
      try { return Response.json(await loadJobs(url.searchParams.get('channel') ?? '')) }
      catch { return Response.json({ error: 'Không tải được video' }, { status: 503 }) }
    }
    return Response.json({ error: 'Not used in this test' }, { status: 404 })
  }))
  const channels = [channel('one'), channel('two')]
  setData(keys.channels(), { channels })
  for (const item of channels) setData(keys.channel(item.id), item)
  host = document.createElement('div'); document.body.append(host); root = createRoot(host)
})
afterEach(async () => {
  await act(async () => root.unmount())
  host.remove(); cache.clear(); vi.unstubAllGlobals()
})
it('opens videos by default and never carries video links into another channel during loading', async () => {
  setData(keys.channelJobs('one'), jobsResponse([job('first', 'one')]))
  let resolve!: (data: ChannelJobs) => void
  loadJobs = () => new Promise<ChannelJobs>(r => { resolve = r })
  await render('one')
  expect(list()?.querySelector('a')?.getAttribute('href')).toBe('#/?channel=one&video=first')
  await render('two')
  expect(list()?.querySelector('a') ?? null).toBeNull()
  expect(host.textContent).not.toContain('Video first')
  await act(async () => resolve(jobsResponse([job('second', 'two')])))
  await flush()
  expect(list()?.querySelector('a')?.getAttribute('href')).toBe('#/?channel=two&video=second')
  expect(host.textContent).not.toContain('Video first')
})
it('distinguishes a failed read from an empty channel and retries successfully', async () => {
  let failed = true
  loadJobs = async () => {
    if (failed) throw new Error('unavailable')
    return jobsResponse([])
  }
  await render('one')
  await flush()
  expect(host.textContent).toContain('Chưa tải được video của kênh')
  expect(host.textContent).not.toContain('Kênh chưa có video')
  failed = false
  const retry = [...host.querySelectorAll<HTMLButtonElement>('button')].find(b => b.textContent === 'Thử lại')!
  await act(async () => retry.click())
  await flush()
  expect(host.textContent).toContain('Kênh chưa có video')
  expect(host.textContent).not.toContain('Chưa tải được video của kênh')
})
