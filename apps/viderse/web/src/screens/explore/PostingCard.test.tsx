// @vitest-environment happy-dom
// Thẻ Lịch đăng khi chưa có khung lặp lại: câu dưới tóm tắt phải đúng trạng
// thái — tạm nghỉ, còn video chưa đọc giờ, chưa đọc được video nào, hay đã đọc
// đủ mẫu mà không có khung nào lặp lại.
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, describe, expect, it } from 'vitest'
import { PostingCard } from './PostingCard'
import type { PostingSchedule } from '../../api/rivals'
import { t } from '../../i18n/all'

const tt = t.screens.explore
const roots: Root[] = []

const schedule = (patch: Partial<PostingSchedule>): PostingSchedule => ({
  channelId: 'r1', zone: 'vn', sample: 30, total: 30, pending: 0, pausedUntil: null,
  perWeek: 22, weeksTotal: 2, offSlot: 30,
  grid: Array.from({ length: 7 }, () => Array.from({ length: 12 }, () => 0)),
  slots: [],
  ...patch,
})

async function mount(s: PostingSchedule) {
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)
  roots.push(root)
  await act(async () => {
    root.render(<PostingCard schedule={s} zone="vn" onZone={() => {}} />)
  })
  return container.querySelector('.vx-scantext')?.textContent
}

afterEach(async () => {
  for (const root of roots.splice(0)) await act(async () => { root.unmount() })
  document.body.innerHTML = ''
})

describe('Lịch đăng chưa có khung', () => {
  it('đã đọc đủ mẫu nhưng không khung nào lặp lại → không nói "đang đọc"', async () => {
    expect(await mount(schedule({ total: 30, pending: 0 }))).toBe(tt.postingNoSlots(30))
  })

  it('còn video chưa đọc giờ → đang đọc', async () => {
    expect(await mount(schedule({ total: 4, pending: 26 }))).toBe(tt.postingPending)
  })

  it('chưa đọc được video nào và không còn gì chờ → chưa có giờ đăng', async () => {
    expect(await mount(schedule({ total: 0, pending: 0, perWeek: null, offSlot: 0 }))).toBe(tt.postingEmpty)
  })

  it('tạm nghỉ thắng mọi trạng thái khác, kèm giờ đọc tiếp', async () => {
    const at = '2026-10-30T15:00:00Z'
    const text = await mount(schedule({ total: 0, pending: 30, pausedUntil: at }))
    expect(text).toBe(tt.postingPaused(new Date(at).toLocaleTimeString('vi-VN', { hour: '2-digit', minute: '2-digit' })))
  })
})
