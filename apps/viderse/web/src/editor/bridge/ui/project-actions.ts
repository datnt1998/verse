import { getEditorStorage } from '@editor/storage/storage-backend'
import { StorageClientError, StorageConflictError } from '@editor/storage/errors'

export type ProjectActionResult = { ok: true; id?: string } | { ok: false; error: string }

export async function createEditorProject(name: string, template: string): Promise<ProjectActionResult> {
  try {
    const { data } = await getEditorStorage().createProject({ name, template })
    return { ok: true, id: data.id }
  } catch {
    return { ok: false, error: 'Chưa tạo được project. Kiểm tra tên rồi thử lại.' }
  }
}

export async function renameEditorProject(id: string, name: string, revision: string): Promise<ProjectActionResult> {
  try {
    await getEditorStorage().renameProject(id, name, revision)
    return { ok: true }
  } catch (error) {
    return { ok: false, error: error instanceof StorageConflictError ? 'Project đã đổi từ nơi khác. Tải lại danh sách rồi thử lại.' : 'Chưa đổi được tên. Thử lại.' }
  }
}

export async function duplicateEditorProject(id: string, name: string): Promise<ProjectActionResult> {
  try {
    const { data } = await getEditorStorage().duplicateProject(id, `${name} — Bản sao`)
    return { ok: true, id: data.id }
  } catch {
    return { ok: false, error: 'Chưa nhân bản được project. Thử lại.' }
  }
}

export async function deleteEditorProject(id: string): Promise<ProjectActionResult> {
  try {
    await getEditorStorage().deleteProject(id)
    return { ok: true }
  } catch (error) {
    if (error instanceof StorageClientError && error.code === 'not_found') return { ok: true }
    if (error instanceof StorageConflictError) return { ok: false, error: error.message }
    return { ok: false, error: 'Chưa xoá được project. Thử lại.' }
  }
}
