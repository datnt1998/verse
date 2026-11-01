// @vitest-environment happy-dom
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { getLiveHistory } from '../../api/client'
import type { LiveHistoryOperation, LiveHistoryRound } from '../../api/types'
import { RunHistory } from './RunHistory'

vi.mock('../../api/client', () => ({ getLiveHistory: vi.fn() }))
vi.mock('../../api/useStoryEvents', () => ({ useStoryEvents: () => ({ events: [] }) }))

const AT = '2026-01-01T10:00:00Z'
const op = (id: string, round: number, role: string, key: string, extra: Partial<LiveHistoryOperation> = {}): LiveHistoryOperation =>
  ({ id, role, round, startedAt: AT, durationMs: 1000, status: 'done', error: '', key, params: '{}', result: '{}', ...extra })
const round = (n: number, role: string, title: string, operations: LiveHistoryOperation[]): LiveHistoryRound =>
  ({ round: n, roundId: `round-${n}`, role, title, startedAt: AT, operations })

// Vòng 10–11 là hai vòng gần nhất (mở sẵn); 8–9 cũ hơn (thu gọn).
const rounds: LiveHistoryRound[] = [
  round(8, 'architect', 'Soát nền truyện', [op('a1', 8, 'architect', 'audit_foundation'), op('a2', 8, 'architect', 'read_chapter')]),
  round(9, 'arbiter', 'Cân nhắc hướng đi', [op('e1', 9, 'arbiter', 'read_chapter', { durationMs: 0 }), op('e2', 9, 'arbiter', 'save_review', { durationMs: 40 })]),
  round(10, 'architect', 'Xem góp ý dàn ý', [op('c1', 10, 'architect', 'revise_outline')]),
  round(11, 'writer', 'Viết lại chương 4', [op('w1', 11, 'writer', 'novel_context', { durationMs: 1200 }), op('w2', 11, 'writer', 'commit_chapter', { status: 'running', durationMs: 0, startedAt: new Date().toISOString() })]),
]

let host: HTMLDivElement
let root: Root
const cards = (n: number) => host.querySelector(`[data-round="${n}"]`)?.querySelectorAll('[data-run-card]') ?? []
const head = (n: number) => host.querySelector(`[data-round="${n}"] > button`) as HTMLButtonElement | null
const click = async (node: Element | null) => { await act(async () => { (node as HTMLElement).click() }) }

beforeEach(async () => {
  ;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true
  vi.useFakeTimers()
  vi.mocked(getLiveHistory).mockReset().mockResolvedValue({ rounds, nextBeforeRound: 8 })
  host = document.createElement('div'); document.body.append(host); root = createRoot(host)
  await act(async () => { root.render(<RunHistory project="p" story="s" embedded />); await Promise.resolve() })
})
afterEach(async () => { await act(async () => root.unmount()); host.remove(); vi.useRealTimers() })

describe('lịch sử luồng viết trong khung', () => {
  it('chỉ hai vòng gần nhất mở sẵn, vòng cũ hơn thu gọn', () => {
    expect(cards(8)).toHaveLength(0)
    expect(cards(9)).toHaveLength(0)
    expect(cards(10)).toHaveLength(1)
    expect(cards(11)).toHaveLength(2)
    expect(head(10)?.getAttribute('aria-expanded')).toBe('true')
    expect(head(8)?.getAttribute('aria-expanded')).toBe('false')
  })
  it('vòng thu gọn ghi số thao tác và tổng thời gian, bấm để mở', async () => {
    expect(head(8)?.textContent).toContain('2 thao tác')
    expect(head(8)?.textContent).toContain('2 giây')
    await click(head(8))
    expect(cards(8)).toHaveLength(2)
    await click(head(8))
    expect(cards(8)).toHaveLength(0)
  })
  it('lọc theo vai chỉ còn vòng của vai đó', async () => {
    const writer = [...host.querySelectorAll('button[aria-pressed]')].find(node => node.textContent === 'Người viết')!
    await click(writer)
    expect(host.querySelector('[data-round="11"]')).toBeTruthy()
    expect(host.querySelector('[data-round="10"]')).toBeNull()
    expect(writer.getAttribute('aria-pressed')).toBe('true')
  })
  it('lọc không có vòng thì nói rõ và gợi ý tải thêm', async () => {
    const editor = [...host.querySelectorAll('button[aria-pressed]')].find(node => node.textContent === 'Biên tập viên')!
    await click(editor)
    expect(host.textContent).toContain('Chưa có vòng nào của Biên tập viên')
    expect(host.textContent).toContain('Trong 20 vòng gần nhất')
  })
  it('thời lượng dưới 0,1 giây ghi “<0,1 giây”, không đo được thì ẩn', async () => {
    await click(head(9))
    const [zero, short] = [...cards(9)] as HTMLElement[]
    expect(short.textContent).toContain('<0,1 giây')
    expect(zero.textContent).not.toContain('giây')
    expect(host.textContent).not.toContain('0 giây')
  })
  it('thẻ đang chạy đứng cuối vòng và chạy đồng hồ thay thời lượng', async () => {
    const running = cards(11)[1] as HTMLElement
    expect(running.getAttribute('data-status')).toBe('running')
    expect(running.querySelector('[role="timer"]')?.textContent).toBe('0:00')
    await act(async () => { vi.advanceTimersByTime(2000) })
    expect(running.querySelector('[role="timer"]')?.textContent).toBe('0:02')
    expect(running.textContent).not.toContain('giây')
  })
  it('“Lịch sử ›” mở trang lịch sử chứ không cuộn lên đầu', () => {
    expect(host.querySelector('a[href="#/p/p/s/s/work?view=run"]')?.textContent).toContain('Lịch sử')
  })
  it('mở thẻ không tải lại; chi tiết kỹ thuật đóng sẵn', async () => {
    expect(getLiveHistory).toHaveBeenCalledTimes(1)
    const card = cards(11)[0] as HTMLElement
    const button = card.querySelector(':scope > button')!
    await click(button)
    expect(card.textContent).toContain('Chi tiết kỹ thuật')
    expect(host.textContent).not.toContain('novel_context')
    await click(button)
    expect(getLiveHistory).toHaveBeenCalledTimes(1)
  })
  it('tải vòng cũ hơn với con trỏ độc quyền', async () => {
    const button = [...host.querySelectorAll('button')].find(node => node.textContent?.includes('Tải thêm vòng cũ hơn'))!
    await click(button)
    expect(getLiveHistory).toHaveBeenLastCalledWith('p', 's', { beforeRound: 8, limit: 20 })
  })
})
