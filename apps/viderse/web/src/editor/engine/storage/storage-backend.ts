import type { Project } from '@editor/types/project'
import type { ExportCommitMetadata } from '@editor/types/export-caption-coverage'
import { StorageClientError, StorageConflictError, type StorageErrorCode } from './errors'
import type { ServerExportRecord, ServerMediaRecord, ServerProjectSummary } from './types'
import { cancelJob, waitForJob } from './jobs'
import type { MediaAttribution } from '@editor/types/storage'
import { z } from 'zod'

export type SidecarName = 'animation-presets' | 'render-queue'
export interface Revisioned<T> { data: T; revision: string }
export type UploadBody = Blob | ReadableStream<Uint8Array>
export interface UploadOptions { signal?: AbortSignal; mime?: string; onProgress?: (sent: number, total: number) => void }
export interface MediaImportOptions extends UploadOptions { fileName: string; projectId?: string }
export interface ExportOptions { fileName: string; mime: string; expectedSize?: number; clone?: { projectRevision?: string }; requestId?: string }

/** Durable project state and source media live exclusively on the server. */
export interface EditorStorage {
  workspaceId(): Promise<string>
  listProjects(): Promise<ServerProjectSummary[]>
  createProject(input: { name: string; template?: string }): Promise<Revisioned<Project>>
  loadProject(id: string): Promise<Revisioned<Project>>
  saveProject(project: Project, baseRevision: string): Promise<string>
  renameProject(id: string, name: string, baseRevision: string): Promise<string>
  duplicateProject(id: string, name?: string): Promise<Revisioned<Project>>
  deleteProject(id: string): Promise<void>
  loadSidecar<T>(projectId: string, name: SidecarName): Promise<Revisioned<T>>
  saveSidecar(projectId: string, name: SidecarName, data: unknown, baseRevision: string): Promise<string>
  loadWorkspaceEffectPresets<T>(): Promise<Revisioned<T>>
  saveWorkspaceEffectPresets(data: unknown, baseRevision: string): Promise<string>
  listMedia(projectId?: string): Promise<ServerMediaRecord[]>
  getMedia(id: string): Promise<ServerMediaRecord>
  updateMediaMetadata(id: string, patch: { tags?: string[]; attribution?: MediaAttribution | null; overlay?: boolean }): Promise<ServerMediaRecord>
  detectMediaBackground(id: string, options?: { refresh?: boolean; overlay?: boolean }): Promise<ServerMediaRecord>
  importMedia(body: UploadBody, meta: MediaImportOptions): Promise<ServerMediaRecord>
  importMediaFromLink(opts: { projectId: string; url: string; from?: string; to?: string; overlay?: boolean; signal?: AbortSignal }): Promise<{ media: ServerMediaRecord; reused: boolean }>
  replaceMedia(id: string, body: UploadBody, meta?: Omit<MediaImportOptions, 'projectId'>): Promise<ServerMediaRecord>
  mediaUrl(id: string, contentHash: string): string
  deleteMedia(id: string): Promise<void>
  linkMedia(projectId: string, mediaId: string): Promise<void>
  unlinkMedia(projectId: string, mediaId: string): Promise<void>
  /** Every overlay in the workspace, one entry per distinct file, newest first. */
  listOverlayLibrary(): Promise<ServerMediaRecord[]>
  /** Drops the overlay tag from every copy of the file; projects keep using it. Returns the remaining library. */
  removeFromOverlayLibrary(mediaId: string): Promise<ServerMediaRecord[]>
  beginExport(projectId: string, options: ExportOptions): Promise<string>
  uploadExport(projectId: string, exportId: string, body: UploadBody, options?: UploadOptions): Promise<void>
  commitExport(projectId: string, exportId: string, metadata?: ExportCommitMetadata): Promise<ServerExportRecord>
  abortExport(projectId: string, exportId: string): Promise<void>
  listExports(projectId: string): Promise<ServerExportRecord[]>
  exportUrl(projectId: string, exportId: string): string
  deleteExport(projectId: string, exportId: string): Promise<void>
}

export type StorageFetch = (input: RequestInfo | URL, init?: RequestInit) => Promise<Response>
const component = encodeURIComponent
/** Export uploads larger than this go through offset-chunked PUTs. */
const EXPORT_CHUNK_SIZE = 192 * 1024 * 1024

/** Loose shape check for media records returned by `/api/editor/media/*`. */
const mediaRecordSchema = z.object({
  id: z.string(),
  background: z.object({
    kind: z.enum(['black', 'green', 'none']),
    keyColor: z.string().optional(),
    version: z.number(),
    detectedAt: z.number(),
    contentHash: z.string(),
  }).optional(),
  overlay: z.boolean().optional(),
}).passthrough()

export class HttpEditorStorage implements EditorStorage {
  private workspacePromise: Promise<string> | null = null
  constructor(private readonly baseUrl = '/api/editor', private readonly request: StorageFetch = fetch) {}

  private async response(path: string, init?: RequestInit): Promise<Response> {
    const response = await this.request.call(globalThis, `${this.baseUrl}${path}`, { cache: 'no-store', ...init })
    if (response.ok) return response
    const body: Record<string, unknown> = await response.json().catch(() => ({}))
    const message = typeof body.error === 'string' ? body.error : `Lỗi lưu trữ (${response.status})`
    if (response.status === 409 && body.code === 'revision_conflict') {
      throw new StorageConflictError(message, typeof body.revision === 'string' ? body.revision : null, body)
    }
    throw new StorageClientError((typeof body.code === 'string' ? body.code : 'internal_error') as StorageErrorCode, message, response.status, body)
  }
  private async json<T>(path: string, init?: RequestInit): Promise<T> {
    return (await this.response(path, init)).json() as Promise<T>
  }
  private jsonWrite(method: string, data: unknown, revision?: string): RequestInit {
    return { method, headers: { 'content-type': 'application/json', ...(revision !== undefined && { 'if-match': `"${revision}"` }) }, body: JSON.stringify(data) }
  }
  private revision(response: Response): string {
    const revision = response.headers.get('X-Editor-Revision')
    if (!revision) throw new StorageClientError('internal_error', 'Không thể xác định phiên bản dữ liệu từ máy chủ. Hãy tải lại trang; nếu vẫn lỗi, liên hệ hỗ trợ.', response.status)
    return revision
  }
  private upload(body: UploadBody, options: UploadOptions = {}): RequestInit {
    // Fetch streams directly from OPFS File.stream(); never buffers a large export.
    const init: RequestInit & { duplex?: 'half' } = { method: 'PUT', body, signal: options.signal, headers: { 'content-type': (options.mime ?? (body instanceof Blob ? body.type : '')) || 'application/octet-stream' } }
    if (!(body instanceof Blob)) init.duplex = 'half'
    return init
  }
  private async uploadResponse(path: string, method: string, body: UploadBody, options: UploadOptions = {}): Promise<Response> {
    if (!(body instanceof Blob) || this.request !== fetch || typeof XMLHttpRequest === 'undefined') {
      return this.response(path, { ...this.upload(body, options), method })
    }
    // Chromium rejects ReadableStream requests over HTTP/1.1. Sending a
    // disk-backed OPFS File/Blob streams without a whole-file memory copy.
    if (options.signal?.aborted) throw new DOMException('Đã hủy tải lên', 'AbortError')
    const { promise, resolve, reject } = Promise.withResolvers<Response>()
    const xhr = new XMLHttpRequest()
    const abort = () => xhr.abort()
    const cleanup = () => options.signal?.removeEventListener('abort', abort)
    xhr.open(method, `${this.baseUrl}${path}`)
    xhr.setRequestHeader('content-type', options.mime || body.type || 'application/octet-stream')
    xhr.upload.onprogress = event => options.onProgress?.(event.loaded, event.lengthComputable ? event.total : body.size)
    xhr.onload = () => {
      cleanup()
      resolve(new Response(xhr.status === 204 ? null : xhr.responseText, { status: xhr.status, headers: { 'content-type': xhr.getResponseHeader('content-type') || 'application/json' } }))
    }
    xhr.onerror = () => { cleanup(); reject(new StorageClientError('network', 'Không thể tải dữ liệu lên máy chủ', 0)) }
    xhr.onabort = () => { cleanup(); reject(new DOMException('Đã hủy tải lên', 'AbortError')) }
    options.signal?.addEventListener('abort', abort, { once: true })
    xhr.send(body)
    const response = await promise
    if (!response.ok) {
      const error: Record<string, unknown> = await response.json().catch(() => ({}))
      const message = typeof error.error === 'string' ? error.error : `Lỗi tải lên (${response.status})`
      if (response.status === 409 && error.code === 'revision_conflict') throw new StorageConflictError(message, typeof error.revision === 'string' ? error.revision : null, error)
      throw new StorageClientError((typeof error.code === 'string' ? error.code : 'internal_error') as StorageErrorCode, message, response.status, error)
    }
    options.onProgress?.(body.size, body.size)
    return response
  }
  workspaceId(): Promise<string> {
    if (!this.workspacePromise) this.workspacePromise = this.json<{ workspaceId: string }>('/workspace').then(result => result.workspaceId).catch(error => { this.workspacePromise = null; throw error })
    return this.workspacePromise
  }
  async listProjects(): Promise<ServerProjectSummary[]> { return (await this.json<{ projects: ServerProjectSummary[] }>('/projects')).projects }
  async createProject(input: { name: string; template?: string }): Promise<Revisioned<Project>> {
    const result = await this.json<{ project: Project; revision: string }>('/projects', this.jsonWrite('POST', input))
    return { data: result.project, revision: result.revision }
  }
  async loadProject(id: string): Promise<Revisioned<Project>> {
    const response = await this.response(`/projects/${component(id)}`)
    return { data: await response.json() as Project, revision: this.revision(response) }
  }
  async saveProject(project: Project, baseRevision: string): Promise<string> { return (await this.json<{ revision: string }>(`/projects/${component(project.id)}`, this.jsonWrite('PUT', project, baseRevision))).revision }
  async renameProject(id: string, name: string, baseRevision: string): Promise<string> { return (await this.json<{ revision: string }>(`/projects/${component(id)}`, this.jsonWrite('PATCH', { name }, baseRevision))).revision }
  async duplicateProject(id: string, name?: string): Promise<Revisioned<Project>> {
    const result = await this.json<{ project: Project; revision: string }>(`/projects/${component(id)}/duplicate`, this.jsonWrite('POST', { name }))
    return { data: result.project, revision: result.revision }
  }
  async deleteProject(id: string): Promise<void> { await this.response(`/projects/${component(id)}`, { method: 'DELETE' }) }
  async loadSidecar<T>(projectId: string, name: SidecarName): Promise<Revisioned<T>> {
    const response = await this.response(`/projects/${component(projectId)}/sidecars/${name}`)
    return { data: await response.json() as T, revision: this.revision(response) }
  }
  async saveSidecar(projectId: string, name: SidecarName, data: unknown, baseRevision: string): Promise<string> { return (await this.json<{ revision: string }>(`/projects/${component(projectId)}/sidecars/${name}`, this.jsonWrite('PUT', data, baseRevision))).revision }
  async loadWorkspaceEffectPresets<T>(): Promise<Revisioned<T>> {
    const response = await this.response('/workspace/effect-presets')
    return { data: await response.json() as T, revision: this.revision(response) }
  }
  async saveWorkspaceEffectPresets(data: unknown, baseRevision: string): Promise<string> {
    return (await this.json<{ revision: string }>('/workspace/effect-presets', this.jsonWrite('PUT', data, baseRevision))).revision
  }
  async listMedia(projectId?: string): Promise<ServerMediaRecord[]> { return (await this.json<{ media: ServerMediaRecord[] }>(`/media${projectId ? `?projectId=${component(projectId)}` : ''}`)).media }
  getMedia(id: string): Promise<ServerMediaRecord> { return this.json(`/media/${component(id)}/metadata`) }
  async updateMediaMetadata(id: string, patch: { tags?: string[]; attribution?: MediaAttribution | null; overlay?: boolean }): Promise<ServerMediaRecord> {
    return (await this.json<{ media: ServerMediaRecord }>(`/media/${component(id)}/metadata`, this.jsonWrite('PATCH', patch))).media
  }
  async detectMediaBackground(id: string, options: { refresh?: boolean; overlay?: boolean } = {}): Promise<ServerMediaRecord> {
    const query = options.refresh ? '?refresh=1' : ''
    const body: Record<string, unknown> = {}
    if (options.overlay !== undefined) body.overlay = options.overlay
    const response = await this.json<{ media: unknown }>(`/media/${component(id)}/background${query}`, this.jsonWrite('POST', body))
    // Passthrough schema validated the fields we extend the record with; the
    // rest of the record shape is what the server already sends.
    const media = mediaRecordSchema.parse(response.media) as unknown as ServerMediaRecord
    return media
  }
  async importMedia(body: UploadBody, meta: MediaImportOptions): Promise<ServerMediaRecord> {
    const query = new URLSearchParams({ fileName: meta.fileName, ...(meta.projectId && { projectId: meta.projectId }) })
    const response = await this.uploadResponse(`/media?${query}`, 'POST', body, meta)
    return (await response.json() as { media: ServerMediaRecord }).media
  }
  async importMediaFromLink(opts: { projectId: string; url: string; from?: string; to?: string; overlay?: boolean; signal?: AbortSignal }): Promise<{ media: ServerMediaRecord; reused: boolean }> {
    const { signal, ...body } = opts
    // Server queues the download (202) so no request is held open past the
    // proxy's idle limit; the job is polled until it settles.
    const started = await this.json<{ job: { id: string } }>('/media/link', { ...this.jsonWrite('POST', body), signal })
    try {
      const result = await waitForJob(started.job.id, { signal, request: this.request, baseUrl: this.baseUrl })
      return { media: mediaRecordSchema.parse(result.media) as unknown as ServerMediaRecord, reused: result.reused === true }
    } catch (error) {
      // Huỷ giữa chừng phải dừng cả job phía server, không chỉ bỏ theo dõi.
      if (signal?.aborted) await cancelJob(started.job.id, { request: this.request, baseUrl: this.baseUrl })
      throw error
    }
  }
  async replaceMedia(id: string, body: UploadBody, meta?: Omit<MediaImportOptions, 'projectId'>): Promise<ServerMediaRecord> {
    const response = await this.uploadResponse(`/media/${component(id)}${meta?.fileName ? `?fileName=${component(meta.fileName)}` : ''}`, 'PUT', body, meta)
    return (await response.json() as { media: ServerMediaRecord }).media
  }
  mediaUrl(id: string, contentHash: string): string { return `${this.baseUrl}/media/${component(id)}?v=${component(contentHash)}` }
  async deleteMedia(id: string): Promise<void> { await this.response(`/media/${component(id)}`, { method: 'DELETE' }) }
  async linkMedia(projectId: string, mediaId: string): Promise<void> { await this.response(`/projects/${component(projectId)}/media/${component(mediaId)}`, { method: 'PUT' }) }
  async unlinkMedia(projectId: string, mediaId: string): Promise<void> { await this.response(`/projects/${component(projectId)}/media/${component(mediaId)}`, { method: 'DELETE' }) }
  async listOverlayLibrary(): Promise<ServerMediaRecord[]> { return (await this.json<{ media: ServerMediaRecord[] }>('/overlays')).media }
  async removeFromOverlayLibrary(mediaId: string): Promise<ServerMediaRecord[]> { return (await this.json<{ media: ServerMediaRecord[] }>(`/overlays/${component(mediaId)}`, { method: 'DELETE' })).media }
  async beginExport(projectId: string, options: ExportOptions): Promise<string> { return (await this.json<{ exportId: string }>(`/projects/${component(projectId)}/exports`, this.jsonWrite('POST', options))).exportId }
  async uploadExport(projectId: string, exportId: string, body: UploadBody, options?: UploadOptions): Promise<void> {
    // Large exports stream as offset chunks so no single request approaches
    // the server's request timeout (dev returns 408 at ~300 s; a 5.7 GB
    // single PUT was observed dying mid-body). Chunks upload sequentially —
    // the server enforces contiguity.
    if (body instanceof Blob && body.size > EXPORT_CHUNK_SIZE && this.request === fetch && typeof XMLHttpRequest !== 'undefined') {
      for (let offset = 0; offset < body.size; offset += EXPORT_CHUNK_SIZE) {
        const chunk = body.slice(offset, Math.min(offset + EXPORT_CHUNK_SIZE, body.size), body.type)
        await this.uploadResponse(
          `/projects/${component(projectId)}/exports/${component(exportId)}?offset=${offset}`,
          'PUT',
          chunk,
          {
            ...options,
            onProgress: options?.onProgress
              ? (sent) => options.onProgress!(offset + sent, body.size)
              : undefined,
          },
        )
      }
      options?.onProgress?.(body.size, body.size)
      return
    }
    await this.uploadResponse(`/projects/${component(projectId)}/exports/${component(exportId)}`, 'PUT', body, options)
  }
  async commitExport(projectId: string, exportId: string, metadata?: ExportCommitMetadata): Promise<ServerExportRecord> { return (await this.json<{ export: ServerExportRecord }>(`/projects/${component(projectId)}/exports/${component(exportId)}?commit`, metadata === undefined ? { method: 'POST' } : this.jsonWrite('POST', metadata))).export }
  abortExport(projectId: string, exportId: string): Promise<void> { return this.deleteExport(projectId, exportId) }
  async listExports(projectId: string): Promise<ServerExportRecord[]> { return (await this.json<{ exports: ServerExportRecord[] }>(`/projects/${component(projectId)}/exports`)).exports }
  exportUrl(projectId: string, exportId: string): string { return `${this.baseUrl}/projects/${component(projectId)}/exports/${component(exportId)}` }
  async deleteExport(projectId: string, exportId: string): Promise<void> { await this.response(`/projects/${component(projectId)}/exports/${component(exportId)}`, { method: 'DELETE' }) }
}

let backend: EditorStorage = new HttpEditorStorage()
export function getEditorStorage(): EditorStorage { return backend }
/** Hosts/tests may supply a backend; callers always resolve it at operation time. */
export function setEditorStorage(storage: EditorStorage): void { backend = storage }
