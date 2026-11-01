// @vitest-environment happy-dom
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import type { CloneJobView } from '../../../api/workbench'
import { WorkbenchPanels } from './WorkbenchPanels'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

const job = { id: 'job', channelId: 'channel', activeStep: 'images', stageGroup: 'images', version: 1, source: { title: 'Nguồn', url: 'https://youtube.com', channelTitle: 'Kênh', thumbnailUrl: null, durationSec: 100 }, agentView: [], steps: [] } as unknown as CloneJobView
const metadata = { status: 'ai', revision: 3, metadata: { title: 'Bản hiện tại', description: 'Mô tả', tags: ['a'] } }
const clip = { id: 'clip', channelId: 'c', videoId: 'v', kind: 'clip', url: 'https://example.test', platform: 'twitch', title: 'Nền', thumbnailUrl: null, channelTitle: 'Kênh nền', channelHandle: null, videoDurationSec: null, startSec: 0, endSec: 30, source: 'random', status: 'ready', mediaId: 'm', error: null }
let backdrop = { revision: 'r', busy: false, randomPicks: true, wantedCount: 1, fallback: 'none', warnings: [], playbackSpeed: 1, fixedItems: [], sources: [], coverage: { mode: 'fill', fixedSec: 70, channelSec: 30, videoSec: 100 }, items: [clip], readyCount: 1 }
let el: HTMLDivElement, root: Root, calls: { path: string; body?: Record<string, unknown> }[]
function button(label: string) { return [...el.querySelectorAll('button')].find(b => b.textContent?.trim() === label)! }
async function click(label: string) { await act(async () => { button(label).click() }) }
beforeEach(async () => {
  backdrop = { revision: 'r', busy: false, randomPicks: true, wantedCount: 1, fallback: 'none', warnings: [], playbackSpeed: 1, fixedItems: [], sources: [], coverage: { mode: 'fill', fixedSec: 70, channelSec: 30, videoSec: 100 }, items: [clip], readyCount: 1 }; calls = []
  vi.stubGlobal('fetch', vi.fn(async (path: string, init?: RequestInit) => {
    const body = init?.body ? JSON.parse(String(init.body)) : undefined
    calls.push({ path, body })
    if (path.endsWith('/history')) return Response.json({ history: [{ at: '2026-01-01T00:00:00Z', by: 'director', parts: ['title'], title: 'Bản cũ', description: '', tags: [] }] })
    if (path.endsWith('/restore')) return Response.json({ ...metadata, revision: 4, metadata: { ...metadata.metadata, title: 'Bản cũ', edited: { title: true } } })
    if (path.endsWith('/rewrite')) return Response.json({ ...metadata, revision: 4 })
    if (path.endsWith('/metadata')) return Response.json(metadata)
    if (path.endsWith('/backdrop')) { if (body?.op === 'remove') backdrop = { ...backdrop, coverage: { ...backdrop.coverage, channelSec: 0 }, items: [], readyCount: 0 }; return Response.json({ backdrop }) }
    return Response.json({ descriptionFooter: '', alwaysTags: [] })
  }))
  el = document.createElement('div'); document.body.append(el); root = createRoot(el)
})
afterEach(async () => { await act(async () => root.unmount()); el.remove(); vi.unstubAllGlobals() })
it('sends only selected rewrite parts and reuses the selected history revision', async () => {
  await act(async () => root.render(<WorkbenchPanels job={job} onRefresh={() => {}} />))
  await click('Nội dung'); await click('Viết lại'); await click('Mô tả'); await click('Tiêu đề'); await click('Viết lại 1 phần')
  expect(calls.find(x => x.path.endsWith('/rewrite'))?.body).toMatchObject({ parts: ['description'], expectedRevision: 3 })
  await click('Lịch sử'); await click('Dùng lại')
  expect(calls.find(x => x.path.endsWith('/restore'))?.body).toMatchObject({ at: '2026-01-01T00:00:00Z', expectedRevision: 4 })
  expect(el.textContent).toContain('bạn đã sửa')
  expect([...el.querySelectorAll('span')].some(span => span.textContent === 'Bạn đã sửa')).toBe(true)
})
it('locks clips while building and updates coverage when removing one', async () => {
  await act(async () => root.render(<WorkbenchPanels job={{ ...job, activeStep: 'build' }} onRefresh={() => {}} />))
  await click('Nền')
  expect(el.textContent).toContain('Kiểm định đang dựng video')
  expect(el.querySelector<HTMLButtonElement>('[aria-label="Bỏ clip 1"]')?.disabled).toBe(true)
  await act(async () => root.render(<WorkbenchPanels job={job} onRefresh={() => {}} />))
  await act(async () => { el.querySelector<HTMLButtonElement>('[aria-label="Bỏ clip 1"]')!.click() })
  expect(el.textContent).toContain('Nền sẽ lặp 0:30')
  expect(calls.find(x => x.body?.op === 'remove')?.body?.revision).toBe('r')
})
