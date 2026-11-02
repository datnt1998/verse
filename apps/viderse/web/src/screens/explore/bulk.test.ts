// Hành vi của kết quả xếp hàng loạt: server tách `items` (mới vào hàng chờ) và
// `duplicates` (đã có job/hàng chờ). Caller phải đếm đúng nhóm nào, giữ lựa
// chọn cho nhóm chưa xếp được và mở đúng job có sẵn.
import { describe, expect, it } from 'vitest'
import { bulkEnqueueOutcome } from './bulk'

const picked = (id: string) => ({ id, url: `https://youtu.be/${id}` })

describe('kết quả xếp hàng loạt', () => {
  it('mọi video đều mới: đếm đủ items, không có nhóm trùng', () => {
    const outcome = bulkEnqueueOutcome(
      { items: [{ id: 'q1', jobId: null }, { id: 'q2', jobId: null }] },
      [picked('v0'), picked('v1')],
    )
    expect(outcome).toEqual({ added: 2, duplicateCount: 0, duplicateVideoIds: [], duplicateJobId: null })
  })

  it('một phần đã có sẵn: chỉ đếm items thật, giữ đúng id của nhóm trùng', () => {
    const outcome = bulkEnqueueOutcome(
      {
        items: [{ id: 'q1', jobId: null }],
        duplicates: [
          { url: 'https://youtu.be/v1', jobId: 'j9', state: 'working' },
          { url: 'https://youtu.be/v2', jobId: null, state: 'queued' },
        ],
      },
      [picked('v0'), picked('v1'), picked('v2')],
    )
    expect(outcome.added).toBe(1)
    expect(outcome.duplicateCount).toBe(2)
    expect(outcome.duplicateVideoIds).toEqual(['v1', 'v2'])
    expect(outcome.duplicateJobId).toBe('j9')
  })

  it('toàn bộ đã có sẵn: 0 item, mở job của video trùng đầu tiên có job', () => {
    const outcome = bulkEnqueueOutcome(
      {
        items: [],
        duplicates: [
          { url: 'https://youtu.be/v0', jobId: null, state: 'queued' },
          { url: 'https://youtu.be/v1', jobId: 'j7', state: 'ready' },
        ],
      },
      [picked('v0'), picked('v1')],
    )
    expect(outcome.added).toBe(0)
    expect(outcome.duplicateCount).toBe(2)
    expect(outcome.duplicateVideoIds).toEqual(['v0', 'v1'])
    expect(outcome.duplicateJobId).toBe('j7')
  })

  it('URL trùng không nằm trong lựa chọn thì không kéo id lạ vào lựa chọn', () => {
    const outcome = bulkEnqueueOutcome(
      { items: [], duplicates: [{ url: 'https://youtu.be/khac', jobId: 'j1', state: 'queued' }] },
      [picked('v0')],
    )
    expect(outcome.duplicateVideoIds).toEqual([])
    expect(outcome.duplicateJobId).toBe('j1')
  })
})
