import { afterEach, describe, expect, it, vi } from 'vitest'
import { applyPrompt, jobAction, jobBulkAction, planPrompt } from './workbench'

afterEach(() => vi.unstubAllGlobals())
describe('hợp đồng Bàn dựng', () => {
  it('gửi thao tác job tới endpoint gốc', async () => {
    const fetcher = vi.fn(() => Promise.resolve(Response.json({})))
    vi.stubGlobal('fetch', fetcher)
    await jobAction('job 1', 'pause')
    const [path, init] = fetcher.mock.calls[0] as unknown as [string, RequestInit]
    expect(path).toBe('/api/clone/job%201')
    expect(JSON.parse(init.body as string)).toEqual({ action: 'pause' })
  })
  it('gửi bản kế hoạch và xác nhận đúng số ảnh, revision', async () => {
    const fetcher = vi.fn(() => Promise.resolve(Response.json({ redraw: { scenes: 4, cover: false }, cost: { images: 4 }, quotaLeft: 8, rebuild: false })))
    vi.stubGlobal('fetch', fetcher)
    const choice = { scenePromptId: 'lantern' }
    const plan = await planPrompt('j', choice)
    expect(plan.cost.images).toBe(4)
    await applyPrompt('j', choice, 7, plan.cost.images)
    const [path, init] = fetcher.mock.calls[1] as unknown as [string, RequestInit]
    expect(path).toBe('/api/clone/j/prompts')
    expect(JSON.parse(init.body as string)).toEqual({ ...choice, expectedRevision: 7, confirmCount: 4 })
  })
  it('không báo thành công khi server trả lỗi từng video', async () => {
    vi.stubGlobal('fetch', vi.fn(() => Promise.resolve(Response.json({ results: [{ ok: false, error: 'Không còn file' }] }))))
    await expect(jobBulkAction('j', 'deleteExport')).rejects.toThrow('Không còn file')
  })
})
