import { describe, expect, it } from 'vitest'
import { groupJobs, readScope } from './workbench-logic'

describe('Bàn dựng', () => {
  it('gộp việc cần người, giới hạn hàng chờ và ẩn đã đăng ở mọi kênh', () => {
    const jobs = ['review', 'paused', 'attention', 'working', 'queued', 'queued', 'queued', 'queued', 'posted'].map((stageGroup, i) => ({ id: String(i), stageGroup }))
    const groups = groupJobs(jobs, true)
    expect(groups.map(g => [g.key, g.items.length])).toEqual([['waiting', 3], ['working', 1], ['queued', 3]])
    expect(groupJobs(jobs, false).find(g => g.key === 'queued')?.items).toHaveLength(2)
    expect(groupJobs(jobs, false).find(g => g.key === 'posted')?.items).toHaveLength(1)
  })
  it('phạm vi URL ưu tiên lựa chọn đã lưu; mặc định mọi kênh', () => {
    expect(readScope(new URLSearchParams('channel=a'), 'b')).toBe('a')
    expect(readScope(new URLSearchParams('scope=all'), 'b')).toBe('all')
    expect(readScope(new URLSearchParams(), 'b')).toBe('b')
    expect(readScope(new URLSearchParams(), null)).toBe('all')
  })
})
