import { describe, expect, it } from 'vitest'
import type { ChannelJob } from '../../api/channelVideos'
import { channelStats } from './videosLogic'

const job = (id: string, stageGroup: string, extra: Record<string, unknown> = {}) => ({
  id, stageGroup, subline: '', steps: null, exportBytes: null, source: { title: id, channelTitle: null, durationSec: null },
  createdAt: '2026-10-12T09:00:00Z', updatedAt: '2026-10-12T09:00:00Z', postedAt: null, ...extra,
}) as unknown as ChannelJob

describe('số liệu đầu kênh', () => {
  const now = Date.parse('2026-10-27T10:00:00Z')
  const base = { createdAt: '2026-10-12T09:00:00Z', youtubeUrl: 'https://youtube.com/@x', autoFetch: true, nextEta: null, now }

  it('Chờ bạn đếm Chờ duyệt + Cần xử lý + Tạm dừng và nêu từng phần', () => {
    const stats = channelStats([job('b', 'review'), job('c', 'attention'), job('p', 'posted', { postedAt: '2026-10-26T12:00:00Z' })], base)
    expect(stats.map((s) => [s.key, s.value])).toEqual([['made', 3], ['posted', 1], ['waiting', 2], ['queued', 0]])
  })

})
