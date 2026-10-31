/**
 * Fake server-side `EditorStorage` for the media/project-media unit tests, layered on the
 * in-memory OPFS from `./in-memory-opfs` (the derived-metadata overlay lives there).
 *
 * Usage:
 *
 *   vi.mock('@editor/storage/storage-backend', async () =>
 *     (await import('./__tests__/fake-editor-storage')).fakeStorageBackendMock,
 *   )
 *   import { fakeServer } from './__tests__/fake-editor-storage'
 *   beforeEach(() => fakeServer.reset())
 *
 * It models what the real routes guarantee: import hashes the bytes (so equal bytes give an
 * equal `sha256:` hash), replace changes the hash and keeps the id and links, delete drops the
 * media from every project, and unknown ids fail with `not_found`.
 */

import { StorageClientError } from '@editor/storage/errors'
import type { MediaAttribution } from '@editor/types/storage'
import type { ServerMediaRecord } from '@editor/storage/types'

import { memoryOpfs } from './in-memory-opfs'

export interface FakeImportMeta {
  fileName: string
  mime?: string
  projectId?: string
  signal?: AbortSignal
  onProgress?: (sent: number, total: number) => void
}

async function sha256(blob: Blob): Promise<string> {
  const digest = await crypto.subtle.digest('SHA-256', await blob.arrayBuffer())
  const hex = [...new Uint8Array(digest)].map((byte) => byte.toString(16).padStart(2, '0')).join('')
  return `sha256:${hex}`
}

function notFound(what: string): StorageClientError {
  return new StorageClientError('not_found', `${what} not found`, 404)
}

class FakeServer {
  records = new Map<string, ServerMediaRecord>()
  links = new Map<string, Set<string>>()
  projects: string[] = []
  uploadCalls: Array<{ kind: 'import' | 'replace'; body: Blob; meta: FakeImportMeta }> = []
  private nextId = 1

  reset(): void {
    memoryOpfs.reset()
    this.records.clear()
    this.links.clear()
    this.projects = []
    this.uploadCalls = []
    this.nextId = 1
  }

  addProject(projectId: string): void {
    this.projects.push(projectId)
    this.links.set(projectId, new Set())
  }

  /** Seed a record directly, as if imported earlier. */
  seed(record: Partial<ServerMediaRecord> & { id: string }): ServerMediaRecord {
    const full: ServerMediaRecord = {
      fileName: `${record.id}.mp4`,
      mime: 'video/mp4',
      size: 1000,
      contentHash: `sha256:seed-${record.id}`,
      createdAt: 1,
      probe: {
        kind: 'video',
        mime: 'video/mp4',
        durationSec: 10,
        width: 1920,
        height: 1080,
        fps: 30,
        videoCodec: 'h264',
        audioCodec: 'aac',
        sampleRate: 48000,
        channels: 2,
      },
      ...record,
    }
    this.records.set(full.id, full)
    return full
  }

  requireRecord(id: string): ServerMediaRecord {
    const record = this.records.get(id)
    if (!record) throw notFound(`Media ${id}`)
    return record
  }

  private async store(
    id: string,
    body: Blob,
    meta: Pick<FakeImportMeta, 'fileName' | 'mime'>,
    previous?: ServerMediaRecord,
  ): Promise<ServerMediaRecord> {
    const mime = meta.mime || body.type || 'application/octet-stream'
    const record: ServerMediaRecord = {
      id,
      fileName: meta.fileName,
      mime,
      size: body.size,
      contentHash: await sha256(body),
      createdAt: previous?.createdAt ?? Date.now(),
      probe: {
        kind: mime.startsWith('audio/') ? 'audio' : mime.startsWith('image/') ? 'image' : 'video',
        mime,
        durationSec: 2,
        width: 640,
        height: 360,
        fps: 24,
        videoCodec: 'h264',
        audioCodec: null,
        sampleRate: null,
        channels: null,
      },
      ...(previous?.tags && { tags: previous.tags }),
      ...(previous?.attribution && { attribution: previous.attribution }),
    }
    this.records.set(id, record)
    return record
  }

  async importMedia(body: Blob, meta: FakeImportMeta): Promise<ServerMediaRecord> {
    this.uploadCalls.push({ kind: 'import', body, meta })
    meta.onProgress?.(body.size, body.size)
    const record = await this.store(`media-${this.nextId++}`, body, meta)
    if (meta.projectId) this.links.get(meta.projectId)?.add(record.id)
    return record
  }

  async replaceMedia(id: string, body: Blob, meta: FakeImportMeta): Promise<ServerMediaRecord> {
    this.uploadCalls.push({ kind: 'replace', body, meta })
    meta.onProgress?.(body.size, body.size)
    return this.store(id, body, meta, this.requireRecord(id))
  }

  async updateMediaMetadata(
    id: string,
    patch: { tags?: string[]; attribution?: MediaAttribution | null },
  ): Promise<ServerMediaRecord> {
    const previous = this.requireRecord(id)
    const record: ServerMediaRecord = { ...previous }
    if (patch.tags !== undefined) record.tags = [...patch.tags]
    if (patch.attribution === null) delete record.attribution
    else if (patch.attribution !== undefined) record.attribution = patch.attribution
    this.records.set(id, record)
    return record
  }

  deleteMedia(id: string): void {
    this.requireRecord(id)
    this.records.delete(id)
    for (const linked of this.links.values()) linked.delete(id)
  }

  listMedia(projectId?: string): ServerMediaRecord[] {
    if (!projectId) return [...this.records.values()]
    const linked = this.links.get(projectId)
    if (!linked) throw notFound(`Project ${projectId}`)
    return [...linked].flatMap((id) => {
      const record = this.records.get(id)
      return record ? [record] : []
    })
  }

  link(projectId: string, mediaId: string): void {
    this.requireRecord(mediaId)
    const linked = this.links.get(projectId)
    if (!linked) throw notFound(`Project ${projectId}`)
    linked.add(mediaId)
  }

  unlink(projectId: string, mediaId: string): void {
    const linked = this.links.get(projectId)
    if (!linked) throw notFound(`Project ${projectId}`)
    linked.delete(mediaId)
  }
}

export const fakeServer = new FakeServer()

/** Module factory for `vi.mock('@editor/storage/storage-backend', ...)`. */
export const fakeStorageBackendMock = {
  getEditorStorage: () => ({
    workspaceId: async () => memoryOpfs.workspaceId,
    listProjects: async () => fakeServer.projects.map((id) => ({ id, name: id, updatedAt: 1 })),
    listMedia: async (projectId?: string) => fakeServer.listMedia(projectId),
    getMedia: async (id: string) => fakeServer.requireRecord(id),
    updateMediaMetadata: (id: string, patch: { tags?: string[]; attribution?: MediaAttribution | null }) =>
      fakeServer.updateMediaMetadata(id, patch),
    importMedia: (body: Blob, meta: FakeImportMeta) => fakeServer.importMedia(body, meta),
    replaceMedia: (id: string, body: Blob, meta: FakeImportMeta) =>
      fakeServer.replaceMedia(id, body, meta),
    mediaUrl: (id: string, contentHash: string) => `/api/editor/media/${id}/file?v=${contentHash}`,
    deleteMedia: async (id: string) => fakeServer.deleteMedia(id),
    linkMedia: async (projectId: string, mediaId: string) => fakeServer.link(projectId, mediaId),
    unlinkMedia: async (projectId: string, mediaId: string) => fakeServer.unlink(projectId, mediaId),
  }),
}
