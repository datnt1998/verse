// @vitest-environment happy-dom
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, expect, it, vi } from 'vitest'
import type { ActivityEvent, ActivityHistory } from '../../../api/types'
import { cache } from '../../../data'
import { MobileActivity } from './index'
import { groupChapters } from './logic'

const state = vi.hoisted(() => ({ events: [] as ActivityEvent[], snapshot: { currentChapter: 41, inProgressChapter: 41, isRunning: true, totalChapters: 64 } }))
vi.mock('../../../api/useStoryEvents', () => ({ useStoryEvents: () => ({ ...state, jobs: new Map(), blocked: null, connected: true, retryNow() {} }) }))
vi.mock('../../../api/session', () => ({ useSession: () => ({ user: { username: 'dat', name: 'Đạt' } }) }))
Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })
const row = (id: string, chapter: number, summary: string, patch = {}): ActivityEvent => ({ id, chapter, summary, time: `2026-10-27T10:00:${id.padStart(2, '0')}Z`, finishedAt: '2026-10-27T10:01:00Z', failed: false, category: 'SYSTEM', agent: 'writer', kind: '', level: 'info', depth: 0, durationNs: 0, retryAt: '', sessionId: 'x', canRetryNow: false, ...patch })
// Every host event category, including legacy dispatches with inline tool parameters.
const events = [
  row('9', 40, 'writer (Viết lại chương 40)', { category: 'DISPATCH', finishedAt: '' }),
  row('8', 40, 'editor (Thẩm duyệt cung 2 của tập 1 · chương 33-40: gọi novel_context, save_review dùng scope=arc, chapter=40)', { category: 'DISPATCH', agent: 'editor', scope: 'arc', arc: 2, volume: 1, chapterFrom: 33, chapterTo: 40 }),
  row('7', 39, 'Viết lại chương 39', { category: 'DISPATCH' }),
  row('6', 38, 'save_review(chương 38)', { category: 'TOOL', tool: 'save_review', args: '{"chapter":38,"scope":"chapter"}', status: 'ok' }),
  row('5', 38, 'Đã duyệt chương 38', { category: 'DECISION', agent: 'arbiter' }),
  row('4', 38, 'Đã nén bối cảnh', { category: 'CONTEXT' }),
  row('3', 38, 'Kết nối bị ngắt', { category: 'ERROR', failed: true, level: 'error' }),
  row('2', 38, 'Giữ nhịp kể', { category: 'USER', agent: 'user', username: 'dat', kind: 'user' }),
  row('1', 41, 'Đang chờ duyệt'),
]
const chapters = [{ chapter: 40, title: 'Mỗi giọng nói đều có người giữ', eventCount: 2 }, { chapter: 39, title: '', eventCount: 1 }, { chapter: 38, title: 'Bản gốc', score: 80, eventCount: 24 }, { chapter: 41, title: '', eventCount: 1 }]
let root: Root | undefined
async function mount() {
  vi.stubGlobal('fetch', vi.fn(async () => Response.json({ events, chapters, nextCursor: null, counts: { total: events.length, errors: 1, mine: 1, byAgent: {} } } satisfies ActivityHistory)))
  const container = document.createElement('div'); document.body.append(container); root = createRoot(container)
  await act(async () => { root!.render(<MobileActivity project="x" story="x" />) })
  return container
}
afterEach(async () => { await act(() => root?.unmount()); root = undefined; cache.clear(); document.body.replaceChildren(); vi.unstubAllGlobals() })
it('X5 groups by event chapter, gives arc reviews their own chronological group, and follows the real running rewrite', () => {
  const groups = groupChapters(events, chapters, 41, true)
  expect(groups.map(group => group.events.map(event => event.id))).toEqual([['9'], ['8'], ['7'], ['6', '5', '4', '3', '2'], ['1']])
  expect(groups[0].live).toBe(true)
  expect(groups.at(-1)?.live).toBe(false)
})
it('X2 chapter arrows point down collapsed and up expanded with matching aria-expanded', async () => {
  const container = await mount()
  const header = [...container.querySelectorAll<HTMLButtonElement>('.mlog-chapter')].find(button => button.textContent?.includes('Chương 38'))!
  expect(header.getAttribute('aria-expanded')).toBe('false')
  expect((header.lastElementChild as HTMLElement).style.maskImage).toContain('arrow-down-2')
  await act(() => header.click())
  expect(header.getAttribute('aria-expanded')).toBe('true')
  expect((header.lastElementChild as HTMLElement).style.maskImage).toContain('arrow-up-2')
})
it('X4 uses en dashes for arc ranges', async () => {
  const container = await mount()
  expect(container.textContent).toContain('Cung 2 · chương 33–40')
  expect(container.textContent).not.toContain('33-40')
})
it('X6 always names untitled chapters explicitly', async () => {
  const container = await mount()
  expect(container.textContent).toContain('Chương 39 · chưa đặt tên')
  expect(container.textContent).toContain('Chương 41 · chưa đặt tên')
})
it('X8 identifies editor score with one Vietnamese decimal', async () => {
  const container = await mount()
  expect(container.textContent).toContain('Biên tập 8,0 · 24 sự kiện')
  expect(container.textContent).toContain('Đang viết lại')
})
it('X3 renders every event kind without equals or tool names; each referenced tool has its own disclosure and Vietnamese parameters', async () => {
  const container = await mount()
  for (const header of container.querySelectorAll<HTMLButtonElement>('.mlog-chapter[aria-expanded="false"]')) await act(() => header.click())
  expect(container.textContent).not.toContain('=')
  expect(container.textContent).not.toMatch(/novel_context|save_review/)
  const arc = [...container.querySelectorAll('section')].find(section => section.getAttribute('aria-label') === 'Cung 2 · chương 33–40')!
  expect(arc.querySelector('.mlog-summary')?.textContent).toBe('Thẩm duyệt cung 2 của tập 1 · chương 33–40')
  expect(arc.querySelectorAll('.mlog-tool')).toHaveLength(2)
  expect(arc.querySelector('.mlog-io')).toBeNull()
  for (const tool of arc.querySelectorAll<HTMLButtonElement>('.mlog-tool')) await act(() => tool.click())
  expect(arc.textContent).toContain('Phạm vi')
  expect(arc.textContent).toContain('cả cung')
  expect(arc.textContent).toContain('Đến chương')
  expect(container.textContent).not.toContain('=')
})
