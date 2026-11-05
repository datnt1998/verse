// @vitest-environment node
import { afterEach, beforeEach, describe, expect, it } from 'vitest'
import { createServer, type IncomingMessage, type Server, type ServerResponse } from 'node:http'
import { getEditorStorage, HttpEditorStorage, setEditorStorage, type StorageFetch } from './storage-backend'
import { StorageClientError, StorageConflictError } from './errors'
import { getProjectRevision, getProjectSnapshot, loadProject, resetProjectPersistenceForTesting, updateProject } from './project-persistence'
import { getStorageConflict, useStorageConflictStore } from './conflict-store'
import type { Project } from '@editor/types/project'
import type { ServerExportRecord, ServerMediaRecord } from './types'

// These are client transport contract tests, not a replacement server. A real
// loopback HTTP peer checks each request and supplies the API response. Server
// persistence/probe/CLI semantics belong to the Go editor service tests.
interface RequestExpectation {
  method: string
  path: string
  body?: unknown
  bytes?: Uint8Array
  headers?: Record<string, string>
  status?: number
  response?: unknown
  responseHeaders?: Record<string, string>
  /** Fires once the peer has answered; tests await it instead of sleeping. */
  onRequest?: () => void
}
let server: Server
let baseUrl: string
let storage: HttpEditorStorage
let expected: RequestExpectation[]
let peerErrors: unknown[]
const project: Project = {
  id: 'project', name: 'Lưu project', description: '', createdAt: 1, updatedAt: 1,
  duration: 1, metadata: { width: 1920, height: 1080, fps: 30 },
}
const media: ServerMediaRecord = {
  id: 'media', fileName: 'clip.mp4', mime: 'video/mp4', size: 4, contentHash: 'sha256:original', createdAt: 1,
  probe: { kind: 'video', mime: 'video/mp4', durationSec: 1, width: 1920, height: 1080, fps: 30, videoCodec: 'h264', audioCodec: null, sampleRate: null, channels: null },
}
const staged: ServerExportRecord = { id: 'export', fileName: 'video.mp4', mime: 'video/mp4', size: 0, status: 'staged', createdAt: 1, committedAt: null }

async function handleRequest(request: IncomingMessage, response: ServerResponse): Promise<void> {
  const next = expected.shift()
  if (!next) throw new Error(`Unexpected request: ${request.method} ${request.url}`)
  expect(request.method).toBe(next.method)
  expect(request.url).toBe(`/api/editor${next.path}`)
  for (const [name, value] of Object.entries(next.headers ?? {})) expect(request.headers[name]).toBe(value)
  const chunks: Buffer[] = []
  for await (const chunk of request) chunks.push(Buffer.isBuffer(chunk) ? chunk : Buffer.from(chunk))
  const body = Buffer.concat(chunks)
  if (next.body !== undefined) {
    expect(request.headers['content-type']).toBe('application/json')
    expect(JSON.parse(body.toString('utf8'))).toEqual(next.body)
  }
  if (next.bytes !== undefined) expect(new Uint8Array(body)).toEqual(next.bytes)
  response.writeHead(next.status ?? 200, { 'content-type': 'application/json', ...next.responseHeaders })
  response.end(next.response === undefined ? undefined : JSON.stringify(next.response))
  next.onRequest?.()
}

beforeEach(async () => {
  expected = []
  peerErrors = []
  server = createServer((request, response) => {
    void handleRequest(request, response).catch(error => {
      peerErrors.push(error)
      if (!response.headersSent) response.writeHead(500, { 'content-type': 'application/json' })
      response.end(JSON.stringify({ error: 'HTTP contract mismatch', code: 'internal_error' }))
    })
  })
  const listening = Promise.withResolvers<void>()
  server.once('error', listening.reject)
  server.listen(0, '127.0.0.1', () => { server.removeListener('error', listening.reject); listening.resolve() })
  await listening.promise
  const address = server.address()
  if (address === null || typeof address === 'string') throw new Error('No loopback HTTP address')
  baseUrl = `http://127.0.0.1:${address.port}/api/editor`
  storage = new HttpEditorStorage(baseUrl)
})
afterEach(async () => {
  server.closeAllConnections()
  const closing = Promise.withResolvers<void>()
  server.close(error => error ? closing.reject(error) : closing.resolve())
  await closing.promise
  expect(peerErrors).toEqual([])
  expect(expected).toEqual([])
})

describe('HTTP editor storage transport', () => {
  it('binds browser fetch to its global receiver and caches the workspace response', async () => {
    expected.push({ method: 'GET', path: '/workspace', response: { workspaceId: 'workspace' } })
    const receiverAware: StorageFetch = function (this: typeof globalThis, input, init) {
      expect(this).toBe(globalThis)
      expect(init?.cache).toBe('no-store')
      return fetch(input, init)
    }
    const client = new HttpEditorStorage(baseUrl, receiverAware)
    expect(await Promise.all([client.workspaceId(), client.workspaceId()])).toEqual(['workspace', 'workspace'])
    expect(await client.workspaceId()).toBe('workspace')
  })

  it('does not cache a failed workspace lookup', async () => {
    expected.push(
      { method: 'GET', path: '/workspace', status: 503, response: { code: 'locked', error: 'Đang bận' } },
      { method: 'GET', path: '/workspace', response: { workspaceId: 'workspace' } },
    )
    await expect(storage.workspaceId()).rejects.toMatchObject({ code: 'locked', status: 503, message: 'Đang bận' })
    expect(await storage.workspaceId()).toBe('workspace')
  })

  const revisionLoaders = [
    { label: 'project', path: '/projects/project', data: project, load: (client: HttpEditorStorage) => client.loadProject(project.id) },
    { label: 'sidecar', path: '/projects/project/sidecars/animation-presets', data: { version: 1, presets: [] }, load: (client: HttpEditorStorage) => client.loadSidecar(project.id, 'animation-presets') },
    { label: 'workspace effects', path: '/workspace/effect-presets', data: { version: 1, presets: [] }, load: (client: HttpEditorStorage) => client.loadWorkspaceEffectPresets() },
  ]
  for (const loader of revisionLoaders) {
    it.each([undefined, 'W/"representation-only"'])(`loads ${loader.label} using the application revision with ETag %s`, async etag => {
      expected.push({ method: 'GET', path: loader.path, response: loader.data, responseHeaders: { 'x-editor-revision': 'authoritative', ...(etag && { etag }) } })
      expect(await loader.load(storage)).toEqual({ data: loader.data, revision: 'authoritative' })
    })

    it.each([undefined, ''])(`rejects ${loader.label} without an application revision (%s), even with an ETag`, async revision => {
      expected.push({ method: 'GET', path: loader.path, response: loader.data, responseHeaders: { etag: '"not-an-application-revision"', ...(revision !== undefined && { 'x-editor-revision': revision }) } })
      const error = await loader.load(storage).catch((cause: unknown) => cause)
      expect(error).toBeInstanceOf(StorageClientError)
      expect(error).not.toBeInstanceOf(StorageConflictError)
      expect(error).toMatchObject({ code: 'internal_error', status: 200 })
    })
  }

  it.each(['revision_conflict', 'conflict', undefined])('retains the loaded snapshot and records only a revision_conflict 409 (%s)', async code => {
    const previous = getEditorStorage()
    resetProjectPersistenceForTesting()
    useStorageConflictStore.setState({ conflicts: {} })
    setEditorStorage(storage)
    try {
      expected.push(
        { method: 'GET', path: '/projects/project', response: project, responseHeaders: { 'x-editor-revision': 'r1', etag: 'W/"representation-only"' } },
        { method: 'PUT', path: '/projects/project', headers: { 'if-match': '"r1"' }, status: 409, response: { code, error: 'Yêu cầu bị từ chối', revision: 'r2' } },
      )
      await loadProject(project.id)
      await expect(updateProject(project.id, { description: 'Chưa lưu' })).rejects.toMatchObject({ name: code === 'revision_conflict' ? 'StorageConflictError' : 'StorageClientError', status: 409 })
      expect(getProjectSnapshot(project.id)).toEqual(project)
      expect(getProjectRevision(project.id)).toBe('r1')
      if (code === 'revision_conflict') {
        expect(getStorageConflict(project.id)).toMatchObject({ baseRevision: 'r1', serverRevision: 'r2' })
        await expect(updateProject(project.id, { description: 'Không ghi đè' })).rejects.toBeInstanceOf(StorageConflictError)
      } else {
        expect(getStorageConflict(project.id)).toBeUndefined()
      }
    } finally {
      setEditorStorage(previous)
      resetProjectPersistenceForTesting()
      useStorageConflictStore.setState({ conflicts: {} })
    }
  })

  it('sends optimistic project revisions and exposes the winning revision on stale saves', async () => {
    const edited = { ...project, description: 'Đã sửa' }
    expected.push(
      { method: 'POST', path: '/projects', body: { name: project.name }, response: { project, revision: 'r1' } },
      { method: 'GET', path: '/projects/project', response: project, responseHeaders: { 'x-editor-revision': 'r1', etag: 'W/"compressed-r1"' } },
      { method: 'PUT', path: '/projects/project', headers: { 'if-match': '"r1"' }, body: edited, response: { revision: 'r2' } },
      { method: 'PUT', path: '/projects/project', headers: { 'if-match': '"r1"' }, body: project, status: 409, response: { error: 'Đã thay đổi', code: 'revision_conflict', revision: 'r2', project: edited } },
      { method: 'GET', path: '/projects/project', response: edited, responseHeaders: { 'x-editor-revision': 'r2' } },
      { method: 'PATCH', path: '/projects/project', body: { name: 'Tên mới' }, headers: { 'if-match': '"r2"' }, response: { revision: 'r3' } },
      { method: 'GET', path: '/projects', response: { projects: [{ id: project.id, name: 'Tên mới', updatedAt: 2 }] } },
      { method: 'DELETE', path: '/projects/project', status: 204 },
    )
    expect(await storage.createProject({ name: project.name })).toEqual({ data: project, revision: 'r1' })
    const second = new HttpEditorStorage(baseUrl)
    const stale = await second.loadProject(project.id)
    expect(await storage.saveProject(edited, 'r1')).toBe('r2')
    await expect(second.saveProject(stale.data, stale.revision)).rejects.toMatchObject({ name: 'StorageConflictError', serverRevision: 'r2', extra: { project: edited } })
    expect(await second.loadProject(project.id)).toEqual({ data: edited, revision: 'r2' })
    expect(await storage.renameProject(project.id, 'Tên mới', 'r2')).toBe('r3')
    expect((await storage.listProjects())[0].name).toBe('Tên mới')
    await storage.deleteProject(project.id)
  })

  it('keeps sidecar revisions independent and sends duplicate names', async () => {
    const data = { version: 1, presets: [{ id: 'zoom', name: 'Phóng to' }] }
    expected.push(
      { method: 'GET', path: '/projects/project/sidecars/animation-presets', response: { version: 1, presets: [] }, responseHeaders: { 'x-editor-revision': 'empty' } },
      { method: 'PUT', path: '/projects/project/sidecars/animation-presets', body: data, headers: { 'if-match': '"empty"' }, response: { revision: 'sidecar-1' } },
      { method: 'GET', path: '/projects/project/sidecars/animation-presets', response: data, responseHeaders: { 'x-editor-revision': 'sidecar-1', etag: 'W/"compressed-sidecar"' } },
      { method: 'PUT', path: '/projects/project/sidecars/animation-presets', body: data, headers: { 'if-match': '"empty"' }, status: 409, response: { code: 'revision_conflict', revision: 'sidecar-1' } },
      { method: 'POST', path: '/projects/project/duplicate', body: { name: 'Bản sao' }, response: { project: { ...project, id: 'copy' }, revision: 'copy-1' } },
    )
    expect((await storage.loadSidecar(project.id, 'animation-presets')).revision).toBe('empty')
    expect(await storage.saveSidecar(project.id, 'animation-presets', data, 'empty')).toBe('sidecar-1')
    expect(await storage.loadSidecar(project.id, 'animation-presets')).toEqual({ data, revision: 'sidecar-1' })
    await expect(storage.saveSidecar(project.id, 'animation-presets', data, 'empty')).rejects.toBeInstanceOf(StorageConflictError)
    expect((await storage.duplicateProject(project.id, 'Bản sao')).data.id).toBe('copy')
  })

  it('streams exact media bytes and versions the URL on replacement', async () => {
    const bytes = new Uint8Array([0, 1, 254, 255])
    const image = new Uint8Array([137, 80, 78, 71])
    const replacement = { ...media, fileName: 'ảnh.png', contentHash: 'sha256:replacement', probe: { ...media.probe, kind: 'image' } }
    expected.push(
      { method: 'POST', path: '/media?fileName=clip+1.mp4&projectId=project', bytes, headers: { 'content-type': 'video/mp4' }, response: { media } },
      { method: 'GET', path: '/media/media/metadata', response: media },
      { method: 'PUT', path: '/media/media?fileName=%E1%BA%A3nh.png', bytes: image, headers: { 'content-type': 'image/png' }, response: { media: replacement } },
      { method: 'DELETE', path: '/media/media', status: 204 },
      { method: 'GET', path: '/media/media/metadata', status: 404, response: { code: 'not_found', error: 'Không còn media' } },
    )
    const original = await storage.importMedia(new Blob([bytes]).stream(), { fileName: 'clip 1.mp4', projectId: project.id, mime: 'video/mp4' })
    expect(await storage.getMedia(original.id)).toEqual(media)
    const replaced = await storage.replaceMedia(original.id, new Blob([image]).stream(), { fileName: 'ảnh.png', mime: 'image/png' })
    expect(replaced.probe.kind).toBe('image')
    expect(storage.mediaUrl(original.id, original.contentHash)).toBe(`${baseUrl}/media/media?v=sha256%3Aoriginal`)
    expect(storage.mediaUrl(replaced.id, replaced.contentHash)).toBe(`${baseUrl}/media/media?v=sha256%3Areplacement`)
    await storage.deleteMedia(media.id)
    await expect(storage.getMedia(media.id)).rejects.toMatchObject({ code: 'not_found', status: 404 })
  })

  it('streams export upload, commits, lists, deletes and aborts staged exports', async () => {
    const output = new Blob(['encoded-video-output'], { type: 'video/mp4' })
    const ready = { ...staged, status: 'ready', size: output.size, committedAt: 2 }
    expected.push(
      { method: 'POST', path: '/projects/project/exports', body: { fileName: 'video.mp4', mime: output.type, expectedSize: output.size }, response: { exportId: 'export' } },
      { method: 'PUT', path: '/projects/project/exports/export', bytes: new Uint8Array(await output.arrayBuffer()), headers: { 'content-type': 'application/octet-stream' }, status: 204 },
      { method: 'POST', path: '/projects/project/exports/export?commit', bytes: new Uint8Array(), response: { export: ready } },
      { method: 'GET', path: '/projects/project/exports', response: { exports: [ready] } },
      { method: 'DELETE', path: '/projects/project/exports/export', status: 204 },
      { method: 'POST', path: '/projects/project/exports', body: { fileName: 'cancel.mp4', mime: output.type }, response: { exportId: 'cancel' } },
      { method: 'DELETE', path: '/projects/project/exports/cancel', status: 204 },
    )
    const id = await storage.beginExport(project.id, { fileName: 'video.mp4', mime: output.type, expectedSize: output.size })
    await storage.uploadExport(project.id, id, output.stream())
    expect(await storage.commitExport(project.id, id)).toEqual(ready)
    expect(storage.exportUrl(project.id, id)).toBe(`${baseUrl}/projects/project/exports/export`)
    expect(await storage.listExports(project.id)).toEqual([ready])
    await storage.deleteExport(project.id, id)
    await storage.abortExport(project.id, await storage.beginExport(project.id, { fileName: 'cancel.mp4', mime: output.type }))
  })

  it('forwards commit metadata and preserves a rejected session for a subsequent commit', async () => {
    const metadata = { captionCoverage: { projectRevision: 'r1', coverage: { version: 1 as const, output: { width: 1920, height: 1080, fps: 30 }, renderedFrames: 30, entries: [] } } }
    expected.push(
      { method: 'POST', path: '/projects/project/exports/export?commit', body: metadata, status: 400, response: { code: 'invalid_request', error: 'Không hợp lệ' } },
      { method: 'GET', path: '/projects/project/exports', response: { exports: [staged] } },
      { method: 'POST', path: '/projects/project/exports/export?commit', response: { export: { ...staged, status: 'ready' } } },
    )
    await expect(storage.commitExport(project.id, staged.id, metadata)).rejects.toMatchObject({ code: 'invalid_request', status: 400 })
    expect((await storage.listExports(project.id))[0].status).toBe('staged')
    expect((await storage.commitExport(project.id, staged.id)).status).toBe('ready')
  })

  it('lists, links, unlinks and patches metadata without writing project JSON', async () => {
    const patch = { tags: ['ảnh'], attribution: { provider: 'local', author: 'Tác giả' } }
    const tagged = { ...media, ...patch }
    expected.push(
      { method: 'GET', path: '/media?projectId=project', response: { media: [media] } },
      { method: 'PUT', path: '/projects/second/media/media', status: 204 },
      { method: 'PATCH', path: '/media/media/metadata', body: patch, response: { media: tagged } },
      { method: 'GET', path: '/media/media/metadata', response: tagged },
      { method: 'DELETE', path: '/projects/second/media/media', status: 204 },
      { method: 'GET', path: '/media', response: { media: [tagged] } },
    )
    expect(await storage.listMedia(project.id)).toEqual([media])
    await storage.linkMedia('second', media.id)
    expect(await storage.updateMediaMetadata(media.id, patch)).toEqual(tagged)
    expect((await storage.getMedia(media.id)).attribution?.provider).toBe('local')
    await storage.unlinkMedia('second', media.id)
    expect(await storage.listMedia()).toEqual([tagged])
  })

  it('revision-checks workspace effect presets independently', async () => {
    const data = { version: 1, presets: [{ id: 'grade', name: 'Màu phim' }] }
    expected.push(
      { method: 'GET', path: '/workspace/effect-presets', response: { version: 1, presets: [] }, responseHeaders: { 'x-editor-revision': 'empty' } },
      { method: 'PUT', path: '/workspace/effect-presets', body: data, headers: { 'if-match': '"empty"' }, response: { revision: 'effects-1' } },
      { method: 'GET', path: '/workspace/effect-presets', response: data, responseHeaders: { 'x-editor-revision': 'effects-1', etag: 'W/"compressed-effects"' } },
      { method: 'PUT', path: '/workspace/effect-presets', body: data, headers: { 'if-match': '"empty"' }, status: 409, response: { code: 'revision_conflict', revision: 'effects-1' } },
    )
    expect((await storage.loadWorkspaceEffectPresets()).revision).toBe('empty')
    expect(await storage.saveWorkspaceEffectPresets(data, 'empty')).toBe('effects-1')
    expect(await storage.loadWorkspaceEffectPresets()).toEqual({ data, revision: 'effects-1' })
    await expect(storage.saveWorkspaceEffectPresets(data, 'empty')).rejects.toBeInstanceOf(StorageConflictError)
  })

  it('keeps an authoritative unchanged project revision when loading refreshed source URLs', async () => {
    const refreshed: Project = { ...project, timeline: { tracks: [], items: [{ id: 'item', trackId: 'video', type: 'video', from: 0, durationInFrames: 30, label: '', mediaId: media.id, src: '/api/editor/media/media?v=sha256%3Areplacement' }] } }
    expected.push(
      { method: 'GET', path: '/projects/project', response: refreshed, responseHeaders: { 'x-editor-revision': 'r1', etag: '"representation-only"' } },
      { method: 'PUT', path: '/projects/project', body: { ...refreshed, description: 'Sửa ở tab B' }, headers: { 'if-match': '"r1"' }, response: { revision: 'r2' } },
    )
    const loaded = await storage.loadProject(project.id)
    expect(loaded.revision).toBe('r1')
    expect(loaded.data.timeline?.items[0].src).toContain('replacement')
    expect(await storage.saveProject({ ...loaded.data, description: 'Sửa ở tab B' }, loaded.revision)).toBe('r2')
  })

  it('escapes path identifiers and passes abort signals to the upload transport', async () => {
    expected.push({ method: 'GET', path: '/projects/project%2Fwith%20space', response: project, responseHeaders: { 'x-editor-revision': 'r1' } })
    await storage.loadProject('project/with space')
    expect(storage.mediaUrl('media/id', 'sha256:a+b')).toBe(`${baseUrl}/media/media%2Fid?v=sha256%3Aa%2Bb`)
    const controller = new AbortController()
    controller.abort()
    await expect(storage.importMedia(new Blob(['cancel']), { fileName: 'cancel.mp4', signal: controller.signal })).rejects.toMatchObject({ name: 'AbortError' })
  })

  it('queues the link import, polls its job and cancels the job when the caller aborts', async () => {
    const running = { id: 'link-1', kind: 'media.link', state: 'running', createdAt: 1 }
    const link = { projectId: project.id, url: 'https://www.twitch.tv/videos/2899740602', from: '1:05', to: '1:35', overlay: true }
    const polled = Promise.withResolvers<void>()
    expected.push(
      { method: 'POST', path: '/media/link', body: link, status: 202, response: { job: running } },
      { method: 'GET', path: '/jobs/link-1', response: { job: { ...running, state: 'succeeded', result: { media, reused: true } } } },
      { method: 'POST', path: '/media/link', body: { projectId: project.id, url: link.url }, status: 202, response: { job: { ...running, id: 'link-2' } } },
      { method: 'GET', path: '/jobs/link-2', response: { job: { ...running, id: 'link-2', state: 'failed', error: { code: 'invalid_request', error: 'Đoạn cắt tối đa 10 phút' } } } },
      { method: 'POST', path: '/media/link', body: { projectId: project.id, url: link.url }, status: 202, response: { job: { ...running, id: 'link-3' } } },
      { method: 'GET', path: '/jobs/link-3', response: { job: { ...running, id: 'link-3' } }, onRequest: () => polled.resolve() },
      { method: 'DELETE', path: '/jobs/link-3', response: { job: { ...running, id: 'link-3', state: 'cancelled' } } },
    )
    expect(await storage.importMediaFromLink(link)).toEqual({ media, reused: true })
    await expect(storage.importMediaFromLink({ projectId: project.id, url: link.url })).rejects.toThrow('Đoạn cắt tối đa 10 phút')
    const controller = new AbortController()
    const pending = storage.importMediaFromLink({ projectId: project.id, url: link.url, signal: controller.signal })
    // Huỷ sau khi job đã được hỏi một lần: job phía server phải bị dừng theo.
    await polled.promise
    controller.abort()
    await expect(pending).rejects.toMatchObject({ name: 'AbortError' })
  })
})
