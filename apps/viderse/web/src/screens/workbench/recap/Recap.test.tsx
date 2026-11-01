import { describe, expect, it } from 'vitest'
import { groupJobs } from '../workbench-logic'
import { recapLead, shouldAutoOpenRecap } from './recap-logic'

describe('Bản tin khi trở lại', () => {
  it('chỉ tự mở khi có điều mới, đủ thời gian và được bật', () => {
    const from = '2026-10-29T08:00:00'
    const to = '2026-10-29T11:01:00'
    expect(shouldAutoOpenRecap(from, to, true, true)).toBe(true)
    expect(shouldAutoOpenRecap(from, to, false, true)).toBe(false)
    expect(shouldAutoOpenRecap(from, to, true, false)).toBe(false)
    expect(shouldAutoOpenRecap(from, '2026-10-29T09:00:00', true, true)).toBe(false)
    expect(shouldAutoOpenRecap('2026-10-29T04:30:00', '2026-10-29T05:01:00', true, true)).toBe(true)
    expect(shouldAutoOpenRecap('invalid', to, true, true)).toBe(false)
  })
  it('câu đầu đếm cùng nhóm Chờ bạn, kể cả video tạm dừng', () => {
    const jobs = ['review', 'attention', 'paused', 'working', 'ready'].map(stageGroup => ({ stageGroup }))
    const groups = groupJobs(jobs, true)
    const waiting = groups.find(group => group.key === 'waiting')?.count ?? 0
    const ready = groups.find(group => group.key === 'ready')?.count ?? 0
    expect(recapLead(waiting, ready)).toBe('Trong lúc bạn vắng: 1 video sẵn sàng đăng, 3 video chờ bạn.')
  })
})
