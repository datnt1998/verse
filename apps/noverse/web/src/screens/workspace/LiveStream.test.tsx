// @vitest-environment happy-dom
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { LiveStream } from './LiveStream'

vi.mock('../../api/client', () => ({ getLiveHistory: vi.fn().mockResolvedValue({ rounds: [{ round: 1, role: 'writer', title: 'Viết chương 2', startedAt: '2026-01-01T10:00:00Z', operations: [{ id: 'one', role: 'writer', round: 1, startedAt: '2026-01-01T10:00:00Z', durationMs: 0, status: 'running', error: '', key: 'draft_chapter', params: '{"chapter":2}', result: '' }] }], nextBeforeRound: null }) }))
vi.mock('../../api/useStoryEvents', () => ({ useStoryEvents: () => ({ events: [] }) }))

let host: HTMLDivElement | undefined
let root: Root | undefined
afterEach(async () => { if (root) await act(async () => root?.unmount()); host?.remove(); root = undefined; host = undefined })
const mount = async (isRunning: boolean) => {
  ;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true
  host = document.createElement('div'); document.body.append(host); root = createRoot(host)
  await act(async () => { root?.render(<LiveStream project="p" story="s" streamText={'✻ draft_chapter\n  chapter: 2'} isRunning={isRunning} snapshot={null} />); await Promise.resolve() })
}
describe('luồng viết', () => {
  it('dùng lịch sử làm nguồn thẻ mặc định, không vẽ lại transcript công cụ', async () => {
    await mount(true)
    expect(host!.textContent).toContain('Vòng 1')
    expect([...host!.querySelectorAll('[data-run-card]')]).toHaveLength(1)
    expect(host!.textContent).not.toContain('draft_chapter')
  })
  it('trạng thái đang chạy gói trong viên thuốc có đồng hồ, không chạy thì không có', async () => {
    await mount(true)
    const pill = host!.querySelector('[role="status"]')
    expect(pill).toBeTruthy()
    expect(pill!.querySelector('[role="timer"]')?.getAttribute('aria-label')).toContain('Đã chạy')
    await act(async () => root?.unmount())
    root = undefined
    host!.remove()
    await mount(false)
    expect(host!.querySelector('[role="status"]')).toBeNull()
  })
})
