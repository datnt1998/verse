import { getEditorStorage, type UploadOptions } from '@editor/storage/storage-backend'
import type { ServerExportRecord } from '@editor/storage/types'
import type { TemporaryExportOutput } from '@editor/features/export/utils/export-output-target'
import { releaseTemporaryExportOutput } from '@editor/features/export/utils/export-output-target'

export interface SavedExport { fileName: string; relPath: string; exportId: string }
export interface ExportFileEntry { name: string; size: number; lastModified: number; path: string[] }

/** Scratch remains available after an upload failure, so the caller can retry. */
export async function saveRenderedExport(
  projectId: string,
  fileName: string,
  output: { blob: Blob; temporaryOutput?: TemporaryExportOutput },
  options: UploadOptions = {},
): Promise<ServerExportRecord> {
  const storage = getEditorStorage()
  const exportId = await storage.beginExport(projectId, { fileName, mime: output.blob.type || 'video/mp4', expectedSize: output.blob.size })
  try {
    if (options.signal?.aborted) throw new DOMException('Đã hủy xuất video', 'AbortError')
    await storage.uploadExport(projectId, exportId, output.blob, options)
    if (options.signal?.aborted) throw new DOMException('Đã hủy xuất video', 'AbortError')
    const record = await storage.commitExport(projectId, exportId)
    await releaseTemporaryExportOutput(output)
    return record
  } catch (error) {
    // A session is disposable; the OPFS source is not removed until commit.
    await storage.abortExport(projectId, exportId).catch(() => undefined)
    throw error
  }
}

export async function saveExportFile(projectId: string | undefined, fileName: string, data: Blob): Promise<SavedExport> {
  if (!projectId) throw new Error('Cần mở project trước khi lưu bản xuất')
  const record = await saveRenderedExport(projectId, fileName, { blob: data })
  return { fileName: record.fileName, exportId: record.id, relPath: `projects/${projectId}/exports/${record.id}/${record.fileName}` }
}
export async function listExportFiles(projectId: string): Promise<ExportFileEntry[]> {
  return (await getEditorStorage().listExports(projectId)).filter(record => record.status === 'ready').map(record => ({ name: record.fileName, size: record.size, lastModified: record.committedAt ?? record.createdAt, path: [projectId, record.id] }))
}
export async function readExportFile(path: string[]): Promise<Blob | null> {
  const response = await fetch(getEditorStorage().exportUrl(path[0], path[1]))
  if (response.status === 404) return null
  if (!response.ok) throw new Error(`Không thể đọc bản xuất (${response.status})`)
  return response.blob()
}
export async function deleteExportFile(path: string[]): Promise<void> {
  await getEditorStorage().deleteExport(path[0], path[1])
}
export function workspaceFolderName(): string { return 'data/editor-workspace' }
