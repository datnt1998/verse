import { describe, expect, it, vi } from 'vitest'
import { followAdmitted, groupJobs, imageTime, issueReason, jobStage, readScope, scheduleScopeRestore } from './workbench-logic'

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
  it('hiện đủ hai mốc thời gian thực, giữ mốc 0 và bỏ qua mốc chưa có', () => {
    expect(imageTime({ startSec: 0, endSec: 3 })).toBe('0:00–0:03')
    expect(imageTime({ startSec: 61.5, endSec: 90 })).toBe('1:01–1:30')
    expect(imageTime({ startSec: 0 })).toBe('')
    expect(imageTime({})).toBe('')
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
  it('giữ lựa chọn xuất ở cổng chất lượng sau khi worker đã nhả bước đang chạy', () => {
    expect(jobStage({
      stageGroup: 'attention', activeStep: null,
      steps: [{ step: 'score', status: 'failed', error: { code: 'quality_gate', message: 'Video chưa đạt Độ hoàn thiện để xuất' } }],
    })).toBe('quality')
    expect(jobStage({
      stageGroup: 'attention', activeStep: 'score',
      steps: [{ step: 'score', status: 'waiting', error: { code: 'tool_missing', message: 'Cần Chrome for Testing' } }],
    })).toBe('issue')
    expect(jobStage({
      stageGroup: 'attention', activeStep: null,
      steps: [{ step: 'images', status: 'failed', error: { code: 'provider_auth', message: 'Hết phiên đăng nhập' } }],
    })).toBe('issue')
  })
  it('giữ đường xử lý khi bước xuất kiểm tra lại cổng chất lượng, không bỏ qua lỗi bộ mã', () => {
    expect(jobStage({
      stageGroup: 'attention', activeStep: null,
      steps: [{ step: 'export', status: 'failed', error: { code: 'quality_gate', message: 'Video chưa đạt mức hoàn thiện' } }],
    })).toBe('quality')
    expect(jobStage({
      stageGroup: 'attention', activeStep: null,
      steps: [{ step: 'export', status: 'failed', error: { code: 'export_failed', message: 'Bộ mã video bị gián đoạn' } }],
    })).toBe('issue')
  })
})

describe('lý do thật của video Cần xử lý', () => {
  const stopped = (code: string, message: string, status = 'failed') => ({
    step: 'images', status, progress: { unitsDone: 0, unitsTotal: null, phase: '' }, error: { code, message },
  })

  it('giữ phân biệt hết phiên đăng nhập / khoá bị từ chối / hạn mức thay vì một câu chung', () => {
    const expired = issueReason({ subline: 'Hoạ sĩ · ChatGPT hết hạn đăng nhập.', steps: [stopped('provider_auth', 'ChatGPT hết hạn đăng nhập.')] })
    const rejected = issueReason({ subline: 'Hoạ sĩ · invalid api key', steps: [stopped('auth_failed', 'invalid api key')] })
    const quota = issueReason({ subline: 'Chờ hạn mức ảnh · 07:00', steps: [stopped('quota', 'quota exceeded', 'waiting')] })
    expect(expired).toContain('đăng nhập')
    expect(rejected).toContain('khoá')
    expect(quota).toContain('hạn mức')
    expect(new Set([expired, rejected, quota]).size).toBe(3)
    expect(rejected).not.toContain('invalid api key')
  })

  it('chọn đúng bước đang chặn của video, bỏ qua bước đã xong; không có bước lỗi thì dùng dòng phụ', () => {
    const text = issueReason({
      subline: 'Hoạ sĩ · ChatGPT hết hạn đăng nhập.',
      steps: [
        stopped('internal_error', 'done already', 'succeeded'),
        stopped('provider_auth', 'ChatGPT hết hạn đăng nhập.'),
      ],
    })
    expect(text).toContain('đăng nhập')
    expect(issueReason({ subline: 'dòng phụ của video', steps: [] })).toBe('dòng phụ của video')
    expect(issueReason({ subline: '', steps: null })).not.toBe('')
  })
})
