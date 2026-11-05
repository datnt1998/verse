import { describe, expect, it } from 'vitest'
import type { ClonePlan } from '../../api/clone'
import { planLine, pickPriority } from './plan'

const plan = (over: Partial<ClonePlan> = {}): ClonePlan => ({
  accepted: [{ url: 'https://youtu.be/a', videoId: 'a', title: 'Đêm ở trạm', durationSec: 600, outlier: 2.1, hasCaptions: true } as ClonePlan['accepted'][number]],
  duplicates: [],
  invalid: [],
  position: 'front',
  etaStart: null,
  paceLeftToday: 2,
  overflowDays: 0,
  template: { id: 't1', name: 'Mẫu chuẩn' },
  imageCalls: 6,
  priceUSD: null,
  ...over,
})

describe('dòng kế hoạch dưới ô link', () => {
  it('máy rảnh: nói bấm là chạy ngay, không bịa giờ chờ', () => {
    const line = planLine(plan({ etaStart: '2026-11-01T03:00:00+07:00' }), { urls: ['https://youtu.be/a'], running: 0, channelName: 'Midnight Tales' })
    expect(line?.text).toBe('Bắt đầu ngay khi bấm')
  })
  it('máy bận: nêu giờ và số video đang chạy', () => {
    const line = planLine(plan({ etaStart: '2026-11-01T00:40:00+07:00' }), { urls: ['https://youtu.be/a'], running: 2, channelName: 'Midnight Tales' })
    expect(line?.text).toBe('Lên đầu hàng chờ · bắt đầu khoảng 00:40 khi một trong 2 video đang chạy xong')
  })
  it('nhiều link: xếp cuối hàng chờ theo nhịp kênh, kèm phần bỏ qua', () => {
    const line = planLine(
      plan({ position: 'back', duplicates: [{ url: 'https://youtu.be/b', jobId: null, state: 'queued' }], paceLeftToday: 2 }),
      { urls: ['https://youtu.be/a', 'https://youtu.be/b', 'https://youtu.be/c', 'https://youtu.be/d'], running: 0, channelName: 'Midnight Tales' },
    )
    expect(line?.text).toBe('Vào cuối hàng chờ, theo nhịp 2 video/ngày của Midnight Tales')
    expect(line?.detail).toBe('Hôm nay còn 2 lượt, video thứ 3 chạy ngày mai · 1 đã làm cho kênh này · sẽ bỏ qua')
  })
  it('một link chọn tay thì lên đầu hàng chờ', () => {
    expect(pickPriority(['https://youtu.be/a'])).toBe('pick')
    expect(pickPriority(['https://youtu.be/a', 'https://youtu.be/b'])).toBe('batch')
  })
})
