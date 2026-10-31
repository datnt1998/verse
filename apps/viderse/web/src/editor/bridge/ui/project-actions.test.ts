import { beforeEach, describe, expect, it, vi } from 'vitest'
import { StorageClientError, StorageConflictError } from '@editor/storage/errors'
import { createEditorProject, deleteEditorProject, duplicateEditorProject, renameEditorProject } from './project-actions'

const storage = vi.hoisted(() => ({
  createProject: vi.fn(), renameProject: vi.fn(), duplicateProject: vi.fn(), deleteProject: vi.fn(),
}))
vi.mock('@editor/storage/storage-backend', () => ({ getEditorStorage: () => storage }))
beforeEach(() => vi.resetAllMocks())

describe('browser editor project actions', () => {
  it('creates the selected project template through HTTP storage', async () => {
    storage.createProject.mockResolvedValue({ data: { id: 'created' }, revision: 'r1' })
    expect(await createEditorProject('Tên mới', 'blank')).toEqual({ ok: true, id: 'created' })
    expect(storage.createProject).toHaveBeenCalledWith({ name: 'Tên mới', template: 'blank' })
  })

  it('returns creation failures without an unhandled rejection', async () => {
    storage.createProject.mockRejectedValue(new Error('offline'))
    expect(await createEditorProject('', 'blank')).toEqual({ ok: false, error: 'Chưa tạo được project. Kiểm tra tên rồi thử lại.' })
  })

  it('passes rename revisions and explains conflicts separately from transport failures', async () => {
    storage.renameProject.mockResolvedValueOnce('r2')
    expect(await renameEditorProject('project', 'Đã đổi', 'r1')).toEqual({ ok: true })
    expect(storage.renameProject).toHaveBeenCalledWith('project', 'Đã đổi', 'r1')
    storage.renameProject.mockRejectedValueOnce(new StorageConflictError('conflict', 'r3'))
    expect(await renameEditorProject('project', 'Cũ', 'r1')).toEqual({ ok: false, error: 'Project đã đổi từ nơi khác. Tải lại danh sách rồi thử lại.' })
    storage.renameProject.mockRejectedValueOnce(new Error('offline'))
    expect(await renameEditorProject('project', 'Cũ', 'r1')).toEqual({ ok: false, error: 'Chưa đổi được tên. Thử lại.' })
  })

  it('duplicates with the upstream copy suffix and returns the new id', async () => {
    storage.duplicateProject.mockResolvedValue({ data: { id: 'copy' }, revision: 'copy-1' })
    expect(await duplicateEditorProject('project', 'Tên')).toEqual({ ok: true, id: 'copy' })
    expect(storage.duplicateProject).toHaveBeenCalledWith('project', 'Tên — Bản sao')
    storage.duplicateProject.mockRejectedValue(new Error('offline'))
    expect(await duplicateEditorProject('project', 'Tên')).toEqual({ ok: false, error: 'Chưa nhân bản được project. Thử lại.' })
  })

  it('makes deletion idempotent only for an authoritative not-found response', async () => {
    storage.deleteProject.mockResolvedValueOnce(undefined)
    expect(await deleteEditorProject('project')).toEqual({ ok: true })
    expect(storage.deleteProject).toHaveBeenCalledWith('project')
    storage.deleteProject.mockRejectedValueOnce(new StorageClientError('not_found', 'Không còn project', 404))
    expect(await deleteEditorProject('project')).toEqual({ ok: true })
    storage.deleteProject.mockRejectedValueOnce(new StorageConflictError('Project đang được dùng', 'r2'))
    expect(await deleteEditorProject('project')).toEqual({ ok: false, error: 'Project đang được dùng' })
    storage.deleteProject.mockRejectedValueOnce(new Error('offline'))
    expect(await deleteEditorProject('project')).toEqual({ ok: false, error: 'Chưa xoá được project. Thử lại.' })
  })
})
