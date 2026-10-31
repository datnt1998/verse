// @vitest-environment node

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

vi.mock('@editor/storage/storage-backend', async () =>
  (await import('./__tests__/fake-editor-storage')).fakeStorageBackendMock,
)

import { fakeServer } from './__tests__/fake-editor-storage'
import { memoryOpfs } from './__tests__/in-memory-opfs'
import { getMedia, updateMedia } from './media'
import {
  hasMediaSource,
  importMediaSource,
  mediaSourceUrl,
  readMediaSource,
  replaceMediaSource,
} from './media-source'

function makeFile(name: string, content: string, type = 'video/mp4'): File {
  return new File([content], name, { type })
}

beforeEach(() => {
  fakeServer.reset()
})

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('importMediaSource', () => {
  it('uploads the File itself (a disk-backed Blob), not a stream', async () => {
    const file = makeFile('clip.mp4', 'bytes')

    await importMediaSource(file)

    expect(fakeServer.uploadCalls).toHaveLength(1)
    expect(fakeServer.uploadCalls[0]?.body).toBe(file)
  })

  it('forwards file name, project link, abort signal and progress to the server', async () => {
    fakeServer.addProject('p1')
    const controller = new AbortController()
    const onProgress = vi.fn()

    const media = await importMediaSource(makeFile('clip.mp4', 'abcd'), {
      projectId: 'p1',
      signal: controller.signal,
      onProgress,
    })

    expect(fakeServer.uploadCalls[0]?.meta).toMatchObject({
      fileName: 'clip.mp4',
      mime: 'video/mp4',
      projectId: 'p1',
      signal: controller.signal,
    })
    expect(onProgress).toHaveBeenCalledWith(4, 4)
    expect([...(fakeServer.links.get('p1') ?? [])]).toEqual([media.id])
  })

  it('lets the caller override a missing or generic MIME type', async () => {
    await importMediaSource(makeFile('take.webm', 'x', ''), { mimeType: 'audio/webm' })

    expect(fakeServer.uploadCalls[0]?.meta.mime).toBe('audio/webm')
  })

  it('returns the server-hashed media with the library view of it', async () => {
    const media = await importMediaSource(makeFile('clip.mp4', 'same-bytes'))
    const again = await importMediaSource(makeFile('other.mp4', 'same-bytes'))

    expect(media.contentHash).toMatch(/^sha256:[0-9a-f]{64}$/)
    expect(again.contentHash).toBe(media.contentHash)
    expect(again.id).not.toBe(media.id)
    expect(media.storageType).toBe('workspace')
  })
})

describe('replaceMediaSource', () => {
  it('keeps the id, changes the hash and discards every cached derivative', async () => {
    const original = await importMediaSource(makeFile('clip.mp4', 'v1'))
    await updateMedia(original.id, { fps: 120 })

    const replaced = await replaceMediaSource(original.id, makeFile('clip.mp4', 'v2'))

    expect(replaced.id).toBe(original.id)
    expect(replaced.contentHash).not.toBe(original.contentHash)
    expect(await memoryOpfs.list(memoryOpfs.pathFor('media', original.id))).toEqual([])
    expect((await getMedia(original.id))?.fps).toBe(24)
  })

  it('preserves durable tags and attribution across the replacement', async () => {
    const original = await importMediaSource(makeFile('clip.mp4', 'v1'))
    await updateMedia(original.id, { tags: ['b-roll'], attribution: { provider: 'Pexels' } })

    const replaced = await replaceMediaSource(original.id, makeFile('clip.mp4', 'v2'))

    expect(replaced.tags).toEqual(['b-roll'])
    expect(replaced.attribution).toEqual({ provider: 'Pexels' })
  })

  it('uploads the File itself and reports progress', async () => {
    const original = await importMediaSource(makeFile('clip.mp4', 'v1'))
    const file = makeFile('clip.mp4', 'v2')
    const onProgress = vi.fn()

    await replaceMediaSource(original.id, file, { onProgress })

    expect(fakeServer.uploadCalls.at(-1)).toMatchObject({ kind: 'replace', body: file })
    expect(onProgress).toHaveBeenCalledWith(2, 2)
  })

  it('does not drop the cache when the server rejects the replacement', async () => {
    const original = await importMediaSource(makeFile('clip.mp4', 'v1'))
    await updateMedia(original.id, { fps: 120 })

    await expect(replaceMediaSource('missing', makeFile('x.mp4', 'v2'))).rejects.toMatchObject({
      code: 'not_found',
    })

    expect((await getMedia(original.id))?.fps).toBe(120)
  })
})

describe('mediaSourceUrl', () => {
  it('is the immutable server URL for the media version', () => {
    expect(mediaSourceUrl({ id: 'm1', contentHash: 'sha256:abc' })).toBe(
      '/api/editor/media/m1/file?v=sha256:abc',
    )
  })

  it('refuses a media without a content hash instead of building an unversioned URL', () => {
    expect(() => mediaSourceUrl({ id: 'm1', contentHash: undefined })).toThrow(/no content hash/)
  })
})

describe('readMediaSource', () => {
  it('fetches the versioned URL and returns the bytes', async () => {
    fakeServer.seed({ id: 'm1', contentHash: 'sha256:v1' })
    const fetchMock = vi.fn(async () => new Response('payload', { status: 200 }))
    vi.stubGlobal('fetch', fetchMock)

    const blob = await readMediaSource('m1')

    expect(fetchMock).toHaveBeenCalledWith('/api/editor/media/m1/file?v=sha256:v1')
    expect(await blob?.text()).toBe('payload')
  })

  it('returns null when the media is unknown or its bytes are gone', async () => {
    expect(await readMediaSource('missing')).toBeNull()

    fakeServer.seed({ id: 'm1' })
    vi.stubGlobal('fetch', vi.fn(async () => new Response('', { status: 404 })))
    expect(await readMediaSource('m1')).toBeNull()
  })

  it('throws on a server failure rather than reporting the media as missing', async () => {
    fakeServer.seed({ id: 'm1' })
    vi.stubGlobal('fetch', vi.fn(async () => new Response('', { status: 500 })))

    await expect(readMediaSource('m1')).rejects.toMatchObject({ status: 500 })
  })
})

describe('hasMediaSource', () => {
  it('reflects whether the server knows the media', async () => {
    fakeServer.seed({ id: 'm1' })

    expect(await hasMediaSource('m1')).toBe(true)
    expect(await hasMediaSource('m2')).toBe(false)
  })
})
