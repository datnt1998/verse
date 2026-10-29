import { afterEach, describe, expect, it } from 'vitest'
import { cache, setData } from '../data'
import { applyLiveToCache, readLive } from './useStoryEvents'
import type { HostEvent } from './types'

const ev = (id: string, time: string): HostEvent => ({
  id, time, finishedAt: time, failed: false, category: 'TOOL', agent: 'writer', summary: `sự kiện ${id}`,
  kind: 'tool', level: 'info', depth: 0, durationNs: 0, retryAt: '', sessionId: 's1', canRetryNow: false,
})

const eventKey = ['story', 'p', 's', 'events']
const ids = () => (cache.get<HostEvent[]>(eventKey).data ?? []).map((item) => item.id)

afterEach(() => { cache.clear() })

describe('khung thực thể (live) khi nối lại', () => {
  it('giữ dữ liệu đang hiển thị rồi hợp nhất bản bù, không trùng', () => {
    setData(eventKey, [ev('e1', '2026-10-26T21:00:00Z'), ev('e2', '2026-10-26T21:01:00Z')])
    // Rớt mạng: không khung nào được phép xoá cache.
    applyLiveToCache('p', 's', { project: 'p', story: 's', kind: 'clear', data: {} })
    applyLiveToCache('p', 's', { project: 'p', story: 's', kind: 'reset', data: {} })
    expect(ids()).toEqual(['e1', 'e2'])
    // Nối lại: máy chủ gửi bù ring (có bản đã có + hai bản mới).
    applyLiveToCache('p', 's', {
      project: 'p', story: 's', kind: 'events',
      data: [ev('e1', '2026-10-26T21:00:00Z'), ev('e2', '2026-10-26T21:01:00Z'), ev('e3', '2026-10-26T21:02:00Z')],
    })
    applyLiveToCache('p', 's', { project: 'p', story: 's', kind: 'event', data: ev('e4', '2026-10-26T21:03:00Z') })
    expect(ids()).toEqual(['e1', 'e2', 'e3', 'e4'])
  })

  it('snapshot đến sau khi nối lại thay bản cũ, và khung lạ bị bỏ qua', () => {
    const snapshot = { bookTitle: 'Truyện', currentChapter: 9, isRunning: true }
    applyLiveToCache('p', 's', { project: 'p', story: 's', kind: 'snapshot', data: snapshot })
    expect(cache.get(['story', 'p', 's', 'snapshot']).data).toMatchObject(snapshot)
    expect(readLive({ project: 'p', story: 's', kind: 'event', data: {} })).not.toBeNull()
    for (const bad of [null, 'x', { project: 'p' }, { project: 1, story: 's', kind: 'event' }]) {
      expect(readLive(bad)).toBeNull()
    }
  })

  it('filters internal and legacy judgments from live and replayed chat events', () => {
    const publicAnswer = { ...ev('answer', '2026-10-28T22:00:00Z'), category: 'SYSTEM', summary: 'Em sẽ kiểm tra.' }
    const internal = { ...ev('internal', publicAnswer.time), category: 'SYSTEM', kind: 'internal', summary: 'private reasoning' }
    const legacy = { ...ev('legacy', publicAnswer.time), category: 'SYSTEM', kind: '', summary: 'Phán định: Người dùng hỏi khả năng xử lý lỗi' }
    setData(eventKey, [internal, legacy, publicAnswer])
    applyLiveToCache('p', 's', { project: 'p', story: 's', kind: 'events', data: [internal, legacy, publicAnswer] })
    applyLiveToCache('p', 's', { project: 'p', story: 's', kind: 'event', data: { ...internal, id: 'new-internal' } })
    expect(ids()).toEqual(['answer'])
  })
})
