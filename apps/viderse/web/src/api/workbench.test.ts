import { afterEach, describe, expect, it, vi } from 'vitest'
import { getJobImages, jobBulkAction, readCloneJob } from './workbench'

afterEach(() => vi.unstubAllGlobals())
describe('hợp đồng Bàn dựng', () => {
  it('đọc tên từ library Snapshot thật và cho phép snapshot cũ chưa lưu tên', () => {
    const job = { id: 'j', channelId: 'c', source: { id: 's', url: 'https://example.test', title: 'Nguồn', channelId: null, channelTitle: null, thumbnailUrl: null, durationSec: null }, sourceKey: 's', generation: 1, templateId: 't', status: 'waiting', stageGroup: 'review', activeStep: 'images', version: 1, noImages: false, subline: '' }
    expect(readCloneJob({ ...job, templateSnapshot: { templateName: 'Mẫu thực', revision: 3 } }, 'job').templateName).toBe('Mẫu thực')
    expect(readCloneJob({ ...job, templateSnapshot: { revision: 1 } }, 'job').templateName).toBeUndefined()
    expect(() => readCloneJob({ ...job, templateSnapshot: { templateName: 7 } }, 'job')).toThrow()
  })
  it('preserves wait causes through the workbench decoder, including stored errors without metadata', () => {
    const job = { id: 'j', channelId: 'c', source: { id: 's', url: 'https://example.test', title: 'Nguồn', channelId: null, channelTitle: null, thumbnailUrl: null, durationSec: null }, sourceKey: 's', generation: 1, templateId: 't', status: 'running', stageGroup: 'working', activeStep: 'download', version: 1, noImages: false, subline: '' }
    const step = { step: 'download', status: 'waiting', progress: { unitsDone: 0, unitsTotal: null, phase: '' }, output: null, startedAt: null, finishedAt: null }
    for (const waitReason of [undefined, 'source_auth', 'storage', 'transient']) {
      const error = { code: 'quota', message: 'stored diagnostic', ...(waitReason ? { waitReason } : {}) }
      const decoded = readCloneJob({ ...job, steps: [{ ...step, error }] }, 'job')
      expect(decoded.steps?.[0].error).toEqual(error)
    }
  })
  it('giữ mốc cảnh bắt đầu từ 0 và không bịa khoảng thời gian khi DTO không có', async () => {
    const item = { id: 'scene', kind: 'scene', status: 'review', currentVersion: null, reviewVersion: null, versions: [] }
    vi.stubGlobal('fetch', vi.fn(() => Promise.resolve(Response.json({ manifest: null, items: [{ ...item, startSec: 0, endSec: 3 }, item] }))))
    const images = await getJobImages('job')
    expect(images.items?.[0]).toMatchObject({ startSec: 0, endSec: 3 })
    expect(images.items?.[1]).not.toHaveProperty('startSec')
    expect(images.items?.[1]).not.toHaveProperty('endSec')
  })
  it('không báo thành công khi server trả lỗi từng video', async () => {
    vi.stubGlobal('fetch', vi.fn(() => Promise.resolve(Response.json({ results: [{ ok: false, error: 'Không còn file' }] }))))
    await expect(jobBulkAction('j', 'deleteExport')).rejects.toThrow('Không còn file')
  })
})
