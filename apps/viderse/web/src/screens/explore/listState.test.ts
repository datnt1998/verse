import { describe, expect, it } from 'vitest'
import type { Rival } from '../../api/rivals'
import { exploreListState, scanProgress, scanningRival } from './listState'

const rival = (patch: Partial<Rival> = {}): Rival => ({
  id: 'r1', handle: '@motphim', title: 'Một Phim', avatarUrl: null,
  url: 'https://youtube.com/@motphim', subscriberCount: 1_000,
  videoCount: 0, medianViews: null, outliers: 0, newOutliers: 0,
  addedAt: '2026-10-20T00:00:00Z', scannedAt: null,
  scan: { state: 'idle', discoveryRunning: false },
  ...patch,
})

const base = { count: 0, loading: false, error: null, offline: false, scanning: undefined, q: '' }

describe('trạng thái danh sách Khám phá', () => {
  it('đang quét thì không bao giờ trả lọc rỗng', () => {
    const scanning = rival({ videoCount: 12, scan: { state: 'running', discoveryRunning: false } })
    expect(exploreListState({ ...base, scanning })).toBe('firstScan')
    expect(exploreListState({ ...base, scanning, q: 'floor' })).toBe('firstScan')
  })

  it('quét khám phá (discoveryRunning) cũng là đang quét', () => {
    const scanning = rival({ scan: { state: 'idle', discoveryRunning: true } })
    expect(exploreListState({ ...base, scanning })).toBe('firstScan')
  })

  it('mất kết nối đứng trước lỗi tải', () => {
    expect(exploreListState({ ...base, offline: true, error: new Error('x') })).toBe('offline')
    expect(exploreListState({ ...base, error: new Error('x') })).toBe('error')
  })

  it('còn video thì giữ lưới kể cả khi mất kết nối', () => {
    expect(exploreListState({ ...base, count: 3, offline: true })).toBe('ok')
  })

  it('không lọc, không tìm, không quét → khối lọc rỗng (nêu bộ lọc mặc định)', () => {
    expect(exploreListState(base)).toBe('filterEmpty')
    expect(exploreListState({ ...base, q: 'floor' })).toBe('nomatch')
  })

  it('chọn đối thủ đang quét trong phạm vi; bỏ qua đối thủ ngoài phạm vi', () => {
    const running = rival({ id: 'r2', scan: { state: 'queued', discoveryRunning: false } })
    expect(scanningRival([rival(), running], 'all')?.id).toBe('r2')
    expect(scanningRival([rival(), running], 'r1')).toBeUndefined()
    expect(scanningRival([rival(), running], 'r2')?.id).toBe('r2')
  })

  it('tiến độ quét lần đầu: số video đã có trên 30 video mới nhất', () => {
    expect(scanProgress(rival({ videoCount: 12 }))).toEqual({ done: 12, total: 30 })
    expect(scanProgress(rival({ videoCount: 900 }))).toEqual({ done: 30, total: 30 })
  })
})
