import { describe, expect, it } from 'vitest'
import type { HostEvent, LiveHistoryOperation, LiveHistoryRound } from '../api/types'
import { mergeRunHistory, mergeRunEvents } from './runHistoryMerge'

const op: LiveHistoryOperation = { id: 'op-1', round: 7, role: 'writer', startedAt: '2026-01-01T10:00:00Z', durationMs: 0, status: 'running', error: '', key: 'draft_chapter', params: '{"chapter":2}', result: '' }
const round: LiveHistoryRound = { round: 7, roundId: 'round-7', role: 'writer', title: 'Viết chương 2', startedAt: op.startedAt, operations: [op] }
describe('gộp lịch sử và luồng trực tiếp', () => {
  it('cùng id chỉ một thẻ, chuyển đang chạy sang hoàn tất', () => {
    const completed = { ...op, status: 'done' as const, durationMs: 1200, result: '{"words":123}' }
    const merged = mergeRunHistory([round], [{ ...round, operations: [completed] }])
    expect(merged).toHaveLength(1)
    expect(merged[0].operations).toEqual([completed])
  })
  it('không để sự kiện bắt đầu cũ thay kết quả đã hoàn tất', () => {
    const completed = { ...op, status: 'done' as const, result: '{"words":123}' }
    expect(mergeRunHistory([{ ...round, operations: [completed] }], [round])[0].operations).toEqual([completed])
  })
  it('gộp SSE cùng roundId và operation id, chuyển running thành done', () => {
    const event: HostEvent = { id: 'op-1', time: op.startedAt, finishedAt: op.startedAt, failed: false, category: 'TOOL', agent: 'writer', role: 'writer', roundId: 'round-7', roundStartedAt: op.startedAt, summary: '', kind: '', level: 'info', depth: 0, durationNs: 0, retryAt: '', sessionId: '', canRetryNow: false, tool: 'draft_chapter', args: op.params, result: '{\"words\":123}', durationMs: 1200, status: 'ok' }
    const merged = mergeRunEvents([round], [event])
    expect(merged).toHaveLength(1)
    expect(merged[0].operations).toHaveLength(1)
    expect(merged[0].operations[0].status).toBe('done')
    expect(merged[0].operations[0].result).toBe('{\"words\":123}')
  })
})
