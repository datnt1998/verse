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
describe('luồng viết', () => {
  it('dùng lịch sử làm nguồn thẻ mặc định, không vẽ lại transcript công cụ', async () => {
    ;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true
    host = document.createElement('div'); document.body.append(host); root = createRoot(host)
    await act(async () => { root?.render(<LiveStream project="p" story="s" streamText={'✻ draft_chapter\n  chapter: 2'} isRunning snapshot={null} />); await Promise.resolve() })
    expect(host.textContent).toContain('Vòng 1')
    expect([...host.querySelectorAll('details')].filter(node => !node.parentElement?.closest('details'))).toHaveLength(1)
    expect(host.textContent).not.toContain('draft_chapter')
  })
})
