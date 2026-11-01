import { describe, expect, it } from 'vitest'
import type { CloneJobView } from '../../api/workbench'
import { editorProjectId, restartTarget } from './workbench-actions'

describe('thao tác video hoàn tất', () => {
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
