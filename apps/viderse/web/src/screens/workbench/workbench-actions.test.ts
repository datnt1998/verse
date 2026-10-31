import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { confirm } from '@noverse/ui'
import type { CloneJobView } from '../../api/workbench'
import { editorProjectId, markPosted, restartTarget } from './workbench-actions'

vi.mock('@noverse/ui', () => ({ confirm: vi.fn() }))
beforeEach(() => vi.clearAllMocks())
afterEach(() => vi.unstubAllGlobals())

describe('thao tác video hoàn tất', () => {
  it('chỉ hỏi xoá MP4 sau khi đánh dấu đã đăng thành công, đúng dung lượng', async () => {
    const calls: string[] = []
    vi.stubGlobal('fetch', vi.fn((_url: string, init: RequestInit) => {
      calls.push(JSON.parse(init.body as string).action)
      return Promise.resolve(Response.json({ results: [{ ok: true }] }))
    }))
    vi.mocked(confirm).mockImplementation(async options => { expect(options.title).toBe('Xoá MP4 để lấy lại 2 MB?'); await options.onConfirm(''); return true })
    const refresh = vi.fn()
    await markPosted({ id: 'j', exportBytes: 1048577 } as CloneJobView, refresh)
    expect(calls).toEqual(['posted', 'deleteExport'])
    expect(refresh).toHaveBeenCalledTimes(2)
  })
  it('không hỏi xoá nếu không còn MP4 và không hỏi khi đánh dấu thất bại', async () => {
    vi.stubGlobal('fetch', vi.fn(() => Promise.resolve(Response.json({ results: [{ ok: true }] }))))
    await markPosted({ id: 'j', exportBytes: null } as CloneJobView, vi.fn())
    expect(confirm).not.toHaveBeenCalled()
    vi.stubGlobal('fetch', vi.fn(() => Promise.resolve(Response.json({ results: [{ ok: false, error: 'Thất bại' }] }))))
    await expect(markPosted({ id: 'j', exportBytes: 10 } as CloneJobView, vi.fn())).rejects.toThrow('Thất bại')
    expect(confirm).not.toHaveBeenCalled()
  })
  it('mở editor theo projectId của bước dựng thay vì id video', () => {
    expect(editorProjectId({ steps: [{ step: 'build', output: { data: { projectId: 'project-1' } } }] } as unknown as CloneJobView)).toBe('project-1')
    expect(editorProjectId({ steps: [] } as unknown as CloneJobView)).toBeUndefined()
  })
  it('chọn lượt làm lại: mục hàng chờ nếu chưa nhận, job thế hệ kế tiếp nếu đã nhận', () => {
    const previous = { id: 'old', sourceKey: 'src', generation: 1 } as CloneJobView
    const queued = { id: 'item', sourceKey: '', generation: 0 } as CloneJobView
    const admitted = { id: 'new', sourceKey: 'src', generation: 2 } as CloneJobView
    const other = { id: 'x', sourceKey: 'src', generation: 1 } as CloneJobView
    expect(restartTarget([other, queued], 'item', previous)).toBe('item')
    expect(restartTarget([other, admitted], 'item', previous)).toBe('new')
    expect(restartTarget([other], 'item', previous)).toBeUndefined()
  })
})
