import { afterEach, describe, expect, it, vi } from 'vitest'
import { applyPrompt, getJobImages, jobAction, jobBulkAction, planPrompt, previewRestart, readCloneJob, restartJob } from './workbench'

afterEach(() => vi.unstubAllGlobals())
describe('hợp đồng Bàn dựng', () => {
  it('đọc tên từ library Snapshot thật và cho phép snapshot cũ chưa lưu tên', () => {
    const job = { id: 'j', channelId: 'c', source: { id: 's', url: 'https://example.test', title: 'Nguồn', channelId: null, channelTitle: null, thumbnailUrl: null, durationSec: null }, sourceKey: 's', generation: 1, templateId: 't', status: 'waiting', stageGroup: 'review', activeStep: 'images', version: 1, noImages: false, subline: '' }
    expect(readCloneJob({ ...job, templateSnapshot: { templateName: 'Mẫu thực', revision: 3 } }, 'job').templateName).toBe('Mẫu thực')
    expect(readCloneJob({ ...job, templateSnapshot: { revision: 1 } }, 'job').templateName).toBeUndefined()
    expect(() => readCloneJob({ ...job, templateSnapshot: { templateName: 7 } }, 'job')).toThrow()
  })
  it('gửi thao tác job tới endpoint gốc', async () => {
    const fetcher = vi.fn(() => Promise.resolve(Response.json({})))
    vi.stubGlobal('fetch', fetcher)
    await jobAction('job 1', 'pause')
    const [path, init] = fetcher.mock.calls[0] as unknown as [string, RequestInit]
    expect(path).toBe('/api/clone/job%201')
    expect(JSON.parse(init.body as string)).toEqual({ action: 'pause' })
  })
  it('làm lại dùng id hàng chờ mới từ phản hồi thật', async () => {
    const fetcher = vi.fn(() => Promise.resolve(Response.json({ restartedFrom: 'old-job', queueItem: { id: 'new-item' } }, { status: 202 })))
    vi.stubGlobal('fetch', fetcher)
    const result = await restartJob('old-job')
    const [path, init] = fetcher.mock.calls[0] as unknown as [string, RequestInit]
    expect(path).toBe('/api/clone/old-job')
    expect(JSON.parse(init.body as string)).toEqual({ action: 'restart' })
    expect(result.queueItem.id).toBe('new-item')
  })
  it('xem trước template và hạn mức trước khi tạo lượt mới', async () => {
    const fetcher = vi.fn(() => Promise.resolve(Response.json({ template: { id: 't', name: 'Mẫu tiếp theo' }, imageEstimate: 4, position: 'front', etaStart: null })))
    vi.stubGlobal('fetch', fetcher)
    const preview = await previewRestart('old-job')
    const [path, init] = fetcher.mock.calls[0] as unknown as [string, RequestInit]
    expect(path).toBe('/api/clone/old-job')
    expect(JSON.parse(init.body as string)).toEqual({ action: 'restart', dryRun: true })
    expect(preview.template.name).toBe('Mẫu tiếp theo')
    expect(preview.imageEstimate).toBe(4)
  })
  it('gửi bản kế hoạch và xác nhận đúng số ảnh, revision', async () => {
    const fetcher = vi.fn(() => Promise.resolve(Response.json({ redraw: { scenes: 4, cover: false }, cost: { images: 4 }, quotaLeft: 8, rebuild: false, enablesImages: false })))
    vi.stubGlobal('fetch', fetcher)
    const choice = { scenePromptId: 'lantern' }
    const plan = await planPrompt('j', choice)
    expect(plan.cost.images).toBe(4)
    await applyPrompt('j', choice, 7, plan.cost.images)
    const [path, init] = fetcher.mock.calls[1] as unknown as [string, RequestInit]
    expect(path).toBe('/api/clone/j/prompts')
    expect(JSON.parse(init.body as string)).toEqual({ ...choice, expectedRevision: 7, confirmCount: 4 })
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
