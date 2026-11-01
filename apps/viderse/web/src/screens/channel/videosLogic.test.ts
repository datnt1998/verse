import { describe, expect, it } from 'vitest'
import type { BulkEligibility, ChannelJob } from '../../api/channelVideos'
import {
  bulkCounts, channelStats, countByFilter, filterJobs, pruneSelection, rangeIds, rowAction, statusDetail, toggleSelection, NO_SELECTION,
} from './videosLogic'

const job = (id: string, stageGroup: string, extra: Record<string, unknown> = {}) => ({
  id, stageGroup, subline: '', steps: null, exportBytes: null, source: { title: id, channelTitle: null, durationSec: null },
  createdAt: '2026-10-12T09:00:00Z', updatedAt: '2026-10-12T09:00:00Z', postedAt: null, ...extra,
}) as unknown as ChannelJob

describe('nhóm bộ lọc', () => {
  const jobs = [job('a', 'working'), job('b', 'review'), job('c', 'attention'), job('d', 'paused'), job('e', 'queued'), job('f', 'ready'), job('g', 'posted'), job('h', 'posted')]

  it('Chờ bạn gộp Chờ duyệt + Cần xử lý + Tạm dừng', () => {
    expect(countByFilter(jobs)).toEqual({ all: 8, working: 1, waiting: 3, queued: 1, ready: 1, posted: 2 })
    expect(filterJobs(jobs, 'waiting').map((item) => item.id)).toEqual(['b', 'd', 'c'])
  })

  it('Tất cả xếp theo thứ tự bảng: đang làm, chờ duyệt, tạm dừng, cần xử lý, chờ lượt, sẵn sàng, đã đăng; cùng nhóm giữ thứ tự máy chủ', () => {
    const shuffled = [job('p1', 'posted'), job('f', 'ready'), job('p2', 'posted'), job('c', 'attention'), job('e', 'queued'), job('a', 'working'), job('b', 'review'), job('d', 'paused')]
    expect(filterJobs(shuffled, 'all').map((item) => item.id)).toEqual(['a', 'b', 'd', 'c', 'e', 'f', 'p1', 'p2'])
  })

  it('Tất cả giữ cả video thuộc nhóm lạ, các nhóm riêng thì không', () => {
    const odd = [...jobs, job('x', 'archived')]
    const counts = countByFilter(odd)
    expect(filterJobs(odd, 'all')).toHaveLength(9)
    expect(counts.all).toBe(9)
    expect(counts.working + counts.waiting + counts.queued + counts.ready + counts.posted).toBe(8)
  })
})

describe('đếm nút hàng loạt', () => {
  const eligibility: BulkEligibility = { retry: ['c'], approve: ['b'], bundle: ['f'], markPosted: ['f', 'f2'], cancel: ['a'], delete: ['b', 'c', 'f'] }

  it('chọn 1 Chờ duyệt, 1 Cần xử lý, 1 Sẵn sàng → mỗi nút 1 (nghiệm thu V4)', () => {
    expect(bulkCounts(new Set(['b', 'c', 'f']), eligibility)).toEqual({ retry: 1, approve: 1, bundle: 1, markPosted: 1, cancel: 0, delete: 3 })
  })

  it('video đã chọn nhưng không hợp lệ không được đếm; chưa chọn gì thì 0', () => {
    expect(bulkCounts(new Set(['e']), eligibility)).toEqual({ retry: 0, approve: 0, bundle: 0, markPosted: 0, cancel: 0, delete: 0 })
    expect(bulkCounts(new Set(), eligibility).markPosted).toBe(0)
  })
})

describe('chọn khoảng bằng Shift', () => {
  const order = ['a', 'b', 'c', 'd', 'e']

  it('lấy cả hai đầu, theo thứ tự đang hiện, bất kể chiều', () => {
    expect(rangeIds(order, 'b', 'd')).toEqual(['b', 'c', 'd'])
    expect(rangeIds(order, 'd', 'b')).toEqual(['b', 'c', 'd'])
  })

  it('không mốc hoặc mốc đã ẩn khỏi bộ lọc thì chỉ chọn đích', () => {
    expect(rangeIds(order, null, 'c')).toEqual(['c'])
    expect(rangeIds(order, 'zz', 'c')).toEqual(['c'])
    expect(rangeIds(order, 'a', 'zz')).toEqual([])
  })

  it('Shift-click chọn một khoảng rồi Shift-click lại ô trong khoảng thì bỏ cả khoảng tới ô đó', () => {
    let state = toggleSelection(NO_SELECTION, 'a', false, order)
    state = toggleSelection(state, 'd', true, order)
    expect([...state.picked]).toEqual(['a', 'b', 'c', 'd'])
    expect(state.anchor).toBe('d')
    state = toggleSelection(state, 'b', true, order)
    expect([...state.picked]).toEqual(['a'])
  })

  it('bấm thường chỉ đảo một ô và dời mốc', () => {
    const state = toggleSelection(toggleSelection(NO_SELECTION, 'c', false, order), 'c', false, order)
    expect(state.picked.size).toBe(0)
    expect(state.anchor).toBe('c')
  })

  it('bỏ khỏi lựa chọn video không còn trong danh sách', () => {
    const state = toggleSelection(toggleSelection(NO_SELECTION, 'a', false, order), 'b', false, order)
    const next = pruneSelection(state, ['b', 'c'])
    expect([...next.picked]).toEqual(['b'])
    expect(pruneSelection(next, ['b'])).toBe(next)
  })
})

describe('nút “Việc tiếp”', () => {
  it('Sẵn sàng chỉ có Tải gói đăng khi đủ điều kiện gói, không thì mở Bàn dựng', () => {
    expect(rowAction(job('f', 'ready'), new Set(['f']))).toBe('pack')
    expect(rowAction(job('f', 'ready'), new Set())).toBe('open')
  })
  it('Chờ lượt và Đã đăng không có nút', () => {
    expect(rowAction(job('e', 'queued'), new Set())).toBeNull()
    expect(rowAction(job('g', 'posted'), new Set())).toBeNull()
  })
})

describe('dòng phụ của Trạng thái', () => {
  const now = Date.parse('2026-10-27T10:00:00Z')

  it('Chờ duyệt = số ảnh · mốc xong tương đối', () => {
    const review = job('b', 'review', {
      subline: 'Chờ bạn duyệt ảnh',
      steps: [{ step: 'images', status: 'succeeded', finishedAt: '2026-10-27T09:18:00Z', output: { data: { imageCount: 7 } } }],
    })
    expect(statusDetail(review, { now })).toBe('7 ảnh · 42 phút trước')
  })

  it('Cần xử lý = câu lỗi của bước hỏng, không kèm tên trợ lý', () => {
    const failed = job('c', 'attention', {
      subline: 'Hoạ sĩ · Cần đăng nhập ChatGPT',
      steps: [{ step: 'images', status: 'failed', finishedAt: null, error: { code: 'x', message: 'Cần đăng nhập ChatGPT' }, output: null }],
    })
    expect(statusDetail(failed, { now })).toBe('Cần đăng nhập ChatGPT')
  })

  it('Sẵn sàng = MP4 thời lượng · dung lượng', () => {
    const ready = job('f', 'ready', { exportBytes: 172 * 1024 * 1024, source: { title: 'f', channelTitle: null, durationSec: 2292 } })
    expect(statusDetail(ready, { now })).toBe('MP4 38:12 · 172 MB')
  })

  it('Đã đăng ưu tiên chuỗi số liệu, không có thì “Đăng dd/MM”', () => {
    const posted = job('g', 'posted', { postedAt: '2026-10-26T12:00:00Z' })
    expect(statusDetail(posted, { now, posted: '6,2 N / 24 giờ · ×1,3' })).toBe('6,2 N / 24 giờ · ×1,3')
    expect(statusDetail(posted, { now })).toBe('Đăng 26/10')
  })
})

describe('số liệu đầu kênh', () => {
  const now = Date.parse('2026-10-27T10:00:00Z')
  const base = { createdAt: '2026-10-12T09:00:00Z', youtubeUrl: 'https://youtube.com/@x', autoPick: true, nextEta: null, now }

  it('Chờ bạn đếm Chờ duyệt + Cần xử lý + Tạm dừng và nêu từng phần', () => {
    const stats = channelStats([job('b', 'review'), job('c', 'attention'), job('p', 'posted', { postedAt: '2026-10-26T12:00:00Z' })], base)
    expect(stats.map((s) => [s.key, s.value])).toEqual([['made', 3], ['posted', 1], ['waiting', 2], ['queued', 0]])
    expect(stats[2]).toMatchObject({ sub: '1 chờ duyệt · 1 cần xử lý', tone: 'warn' })
    expect(stats[1].sub).toBe('gần nhất 26/10')
    expect(stats[0].sub).toBe('từ 12/10')
    expect(stats[3].sub).toBe('tự lấy video đang bật')
  })

  it('kênh chưa có video: chữ của bản kênh mới', () => {
    const stats = channelStats([], { ...base, youtubeUrl: null, createdAt: new Date(now).toISOString() })
    expect(stats.map((s) => s.sub)).toEqual(['tạo hôm nay', 'chưa liên kết YouTube', 'không có việc', 'tự lấy video đang bật'])
  })
})
