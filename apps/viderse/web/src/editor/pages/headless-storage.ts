import { z } from 'zod'
import { HttpEditorStorage, type ExportOptions, type UploadBody, type UploadOptions } from '@editor/storage/storage-backend'
import type { ExportCommitMetadata } from '@editor/types/export-caption-coverage'
import { HeadlessClient, exportRecordSchema, type HeadlessSnapshot } from './headless-client'

/** The inherited editor mutation methods are deliberately denied, never sent with a user session. */
export class HeadlessEditorStorage extends HttpEditorStorage {
  constructor(readonly client: HeadlessClient, readonly snapshot: HeadlessSnapshot) {
    super(client.baseUrl, async () => { throw new Error('Yêu cầu ngầm không được phép sửa dự án') })
  }

  private project(projectId: string): void {
    if (projectId !== this.snapshot.project.id) throw new Error('Dự án nằm ngoài phạm vi yêu cầu')
  }
  override async workspaceId() { return this.snapshot.runtime.workspaceId }
  override async loadProject(projectId: string) { this.project(projectId); return { data: this.snapshot.project, revision: this.snapshot.revision } }
  override async listMedia(projectId?: string) { if (projectId) this.project(projectId); return this.snapshot.media }
  override async getMedia(mediaId: string) {
    const media = this.snapshot.media.find(record => record.id === mediaId)
    if (!media) throw new Error('Tệp nguồn nằm ngoài phạm vi yêu cầu')
    return media
  }
  override mediaUrl(mediaId: string, contentHash: string) {
    if (!this.snapshot.media.some(record => record.id === mediaId)) throw new Error('Tệp nguồn nằm ngoài phạm vi yêu cầu')
    return this.client.mediaUrl(mediaId, contentHash)
  }
  override async beginExport(projectId: string, options: ExportOptions) {
    this.project(projectId)
    const response = await this.client.post('/exports', { fence: this.client.requireFence(), options })
    return z.object({ exportId: z.string().min(1) }).parse(await response.json()).exportId
  }
  override async uploadExport(projectId: string, exportId: string, body: UploadBody, options: UploadOptions = {}) {
    this.project(projectId)
    if (!(body instanceof Blob)) throw new Error('Xuất ngầm cần tệp video đã dựng')
    const chunkSize = 16 * 1024 ** 2
    for (let offset = 0; offset < body.size; offset += chunkSize) {
      options.signal?.throwIfAborted()
      const end = Math.min(offset + chunkSize, body.size)
      await this.client.response(`/exports/${encodeURIComponent(exportId)}?offset=${offset}`, {
        method: 'PUT', signal: options.signal,
        headers: { 'Content-Type': options.mime ?? body.type ?? 'application/octet-stream', 'X-Render-Fence': this.client.requireFence() },
        body: body.slice(offset, end),
      })
      options.onProgress?.(end, body.size)
    }
  }
  override async commitExport(projectId: string, exportId: string, metadata?: ExportCommitMetadata) {
    this.project(projectId)
    const response = await this.client.post(`/exports/${encodeURIComponent(exportId)}/commit`, { fence: this.client.requireFence(), metadata })
    return z.object({ export: exportRecordSchema }).parse(await response.json()).export
  }
  override async abortExport(projectId: string, exportId: string) {
    this.project(projectId)
    await this.client.response(`/exports/${encodeURIComponent(exportId)}`, { method: 'DELETE', headers: { 'X-Render-Fence': this.client.requireFence() } })
  }
}
