import { describe, expect, it, vi } from 'vitest'
import { followAdmitted, groupJobs, readScope, scheduleScopeRestore } from './workbench-logic'

describe('Bàn dựng', () => {
  it('đi theo job mới khi mục hàng chờ đang chọn được nhận', () => {
    const followed = { id: 'item', sourceKey: 'src' }
    expect(followAdmitted([{ id: 'item', sourceKey: 'src', generation: 0 }], followed)).toBeUndefined()
    expect(followAdmitted([{ id: 'a', sourceKey: 'src', generation: 1 }, { id: 'b', sourceKey: 'src', generation: 2 }, { id: 'c', sourceKey: 'x', generation: 3 }], followed)).toBe('b')
    expect(followAdmitted([{ id: 'b', sourceKey: 'src', generation: 2 }], { id: 'item', sourceKey: '' })).toBeUndefined()
  })
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
  it('khôi phục phạm vi sau chuẩn hoá route mà không ghi đè điều hướng mới', async () => {
    let hash = '#/'
    const restore = vi.fn()
    scheduleScopeRestore('kenh-a', () => hash, restore)
    expect(restore).not.toHaveBeenCalled()
    await Promise.resolve()
    expect(restore).toHaveBeenCalledWith('kenh-a')
    restore.mockClear()
    scheduleScopeRestore('kenh-a', () => hash, restore)
    hash = '#/review'
    await Promise.resolve()
    expect(restore).not.toHaveBeenCalled()
  })
})
