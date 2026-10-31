// @vitest-environment node

import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { ThumbnailData } from '@editor/types/storage'
import '../test-utils/logger-test-mocks'

// vi.mock is hoisted above static imports, so the shared factory can only be
// loaded lazily from inside it.
vi.mock('@editor/storage/storage-backend', async () =>
  (await import('./__tests__/in-memory-opfs')).storageBackendMock,
)

import { memoryOpfs } from './__tests__/in-memory-opfs'
import { resetMediaIdentityTracking } from './cache-identity'
import {
  deleteThumbnailsByMediaId,
  getThumbnail,
  getThumbnailByMediaId,
  getThumbnailsByMediaIds,
  saveThumbnail,
} from './thumbnails'

function makeThumbnail(mediaId: string, id = `t-${mediaId}`): ThumbnailData {
  const bytes = new Uint8Array([0xff, 0xd8, 0xff]) // JPEG SOI marker
  return {
    id,
    mediaId,
    blob: new Blob([bytes], { type: 'image/jpeg' }),
    timestamp: 0,
    width: 320,
    height: 180,
  }
}

beforeEach(() => {
  memoryOpfs.reset()
  resetMediaIdentityTracking()
  for (const id of ['m1', 'm2', 'm3']) memoryOpfs.setMedia(id, `hash-${id}`)
})

describe('workspace-fs thumbnails', () => {
  it('saveThumbnail writes the jpeg blob under the media content hash', async () => {
    await saveThumbnail(makeThumbnail('m1'))

    const text = await memoryOpfs.readText(memoryOpfs.mediaPathFor('m1', 'hash-m1', 'thumbnail.jpg'))
    expect(text).not.toBeNull()
    // No sidecar metadata file is written.
    expect(
      await memoryOpfs.readText(memoryOpfs.mediaPathFor('m1', 'hash-m1', 'thumbnail.meta.json')),
    ).toBeNull()
  })

  it('getThumbnailByMediaId returns saved thumbnail with id derived from mediaId', async () => {
    await saveThumbnail(makeThumbnail('m1'))
    const t = await getThumbnailByMediaId('m1')
    expect(t).toBeDefined()
    // The id is derived from mediaId; the caller-supplied id is ignored.
    expect(t!.id).toBe('m1')
    expect(t!.mediaId).toBe('m1')
  })

  it('getThumbnailByMediaId returns undefined when missing', async () => {
    expect(await getThumbnailByMediaId('m2')).toBeUndefined()
  })

  it('getThumbnail(id) treats id as mediaId', async () => {
    await saveThumbnail(makeThumbnail('m1'))
    const t = await getThumbnail('m1')
    expect(t).toBeDefined()
    expect(t!.mediaId).toBe('m1')
  })

  it('deleteThumbnailsByMediaId removes the jpeg', async () => {
    await saveThumbnail(makeThumbnail('m1'))
    await deleteThumbnailsByMediaId('m1')
    expect(await getThumbnailByMediaId('m1')).toBeUndefined()
    expect(
      await memoryOpfs.readText(memoryOpfs.mediaPathFor('m1', 'hash-m1', 'thumbnail.jpg')),
    ).toBeNull()
  })

  it('getThumbnailsByMediaIds batch-reads present thumbnails and skips missing ones', async () => {
    await saveThumbnail(makeThumbnail('m1'))
    await saveThumbnail(makeThumbnail('m3'))

    // 'm2' has no thumbnail — it must be absent from the result, not throw.
    const result = await getThumbnailsByMediaIds(['m1', 'm2', 'm3'])

    expect([...result.keys()].sort()).toEqual(['m1', 'm3'])
    expect(result.get('m1')!.size).toBeGreaterThan(0)
    expect(result.get('m2')).toBeUndefined()
  })

  it('getThumbnailsByMediaIds returns an empty map for empty input or unknown media', async () => {
    expect((await getThumbnailsByMediaIds([])).size).toBe(0)
    expect((await getThumbnailsByMediaIds(['nope'])).size).toBe(0)
  })

  it('saveThumbnail overwrites prior thumbnail for the same media', async () => {
    await saveThumbnail(makeThumbnail('m1', 'first'))
    await saveThumbnail(makeThumbnail('m1', 'second'))
    const t = await getThumbnailByMediaId('m1')
    expect(t).toBeDefined()
    expect(t!.mediaId).toBe('m1')
    expect(await memoryOpfs.list(memoryOpfs.mediaPathFor('m1', 'hash-m1'))).toEqual(['thumbnail.jpg'])
  })

  it('does not serve the thumbnail of the previous bytes after the source is replaced', async () => {
    await saveThumbnail(makeThumbnail('m1'))
    memoryOpfs.setMedia('m1', 'hash-m1-replaced')

    expect(await getThumbnailByMediaId('m1')).toBeUndefined()
    expect((await getThumbnailsByMediaIds(['m1'])).size).toBe(0)
  })

  it('treats a thumbnail evicted mid-read as a miss', async () => {
    await saveThumbnail(makeThumbnail('m1'))
    memoryOpfs.failReads('NotReadableError')
    expect(await getThumbnailByMediaId('m1')).toBeUndefined()
    expect((await getThumbnailsByMediaIds(['m1'])).size).toBe(0)
  })

  it('drops a thumbnail write that hits the storage quota', async () => {
    memoryOpfs.failWrites('QuotaExceededError')
    await expect(saveThumbnail(makeThumbnail('m1'))).resolves.toBeUndefined()
    memoryOpfs.failWrites(null)
    expect(await getThumbnailByMediaId('m1')).toBeUndefined()
  })

  it('drops a thumbnail write for a media the server no longer has', async () => {
    memoryOpfs.deleteMedia('m1')
    await expect(saveThumbnail(makeThumbnail('m1'))).resolves.toBeUndefined()
    expect(await memoryOpfs.list(memoryOpfs.pathFor('media'))).toEqual([])
  })
})
