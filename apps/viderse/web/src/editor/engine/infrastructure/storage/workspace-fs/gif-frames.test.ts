// @vitest-environment node

import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { GifFrameData } from '@editor/types/storage'
import '../test-utils/logger-test-mocks'

// vi.mock is hoisted above static imports, so the shared factory can only be
// loaded lazily from inside it.
vi.mock('@editor/storage/storage-backend', async () =>
  (await import('./__tests__/in-memory-opfs')).storageBackendMock,
)

import { memoryOpfs } from './__tests__/in-memory-opfs'
import { resetMediaIdentityTracking } from './cache-identity'
import { clearAllGifFrames, deleteGifFrames, getGifFrames, saveGifFrames } from './gif-frames'

function makeData(mediaId: string, frames = 3): GifFrameData {
  return {
    id: mediaId,
    mediaId,
    frames: Array.from(
      { length: frames },
      (_, i) => new Blob([new Uint8Array([0x89, 0x50, 0x4e, 0x47, i])], { type: 'image/png' }),
    ),
    durations: Array.from({ length: frames }, () => 100),
    totalDuration: frames * 100,
    width: 100,
    height: 100,
    frameCount: frames,
    createdAt: 0,
  }
}

beforeEach(() => {
  memoryOpfs.reset()
  resetMediaIdentityTracking()
  memoryOpfs.setMedia('m1', 'h1')
})

describe('workspace-fs gif-frames', () => {
  it('saveGifFrames writes meta + per-frame binaries under the media content hash', async () => {
    await saveGifFrames(makeData('m1', 3))

    const meta = await memoryOpfs.readText(
      memoryOpfs.mediaPathFor('m1', 'h1', 'cache', 'gif-frames', 'meta.json'),
    )
    expect(meta).not.toBeNull()
    const parsed = JSON.parse(meta!)
    expect(parsed.frameCount).toBe(3)
    expect(parsed.frames).toBeUndefined()
  })

  it('getGifFrames round-trips', async () => {
    await saveGifFrames(makeData('m1', 2))
    const got = await getGifFrames('m1')
    expect(got).toBeDefined()
    expect(got!.frameCount).toBe(2)
    expect(got!.frames.length).toBe(2)
  })

  it('getGifFrames returns undefined when missing', async () => {
    expect(await getGifFrames('m1')).toBeUndefined()
  })

  it('getGifFrames returns undefined for a media the server no longer has', async () => {
    await saveGifFrames(makeData('m1'))
    memoryOpfs.deleteMedia('m1')
    expect(await getGifFrames('m1')).toBeUndefined()
  })

  it('saveGifFrames drops the write for a media the server no longer has', async () => {
    await expect(saveGifFrames(makeData('ghost'))).resolves.toBeUndefined()
    expect(await memoryOpfs.list(memoryOpfs.pathFor('media'))).toEqual([])
  })

  it('does not serve frames cached for the previous bytes after the source is replaced', async () => {
    await saveGifFrames(makeData('m1', 2))
    memoryOpfs.setMedia('m1', 'h2')
    expect(await getGifFrames('m1')).toBeUndefined()
  })

  it('treats a quota failure while saving as a dropped write, not an error', async () => {
    memoryOpfs.failWrites('QuotaExceededError')
    await expect(saveGifFrames(makeData('m1'))).resolves.toBeUndefined()
    memoryOpfs.failWrites(null)
    expect(await getGifFrames('m1')).toBeUndefined()
  })

  it('deleteGifFrames removes the whole cache dir', async () => {
    await saveGifFrames(makeData('m1'))
    await deleteGifFrames('m1')
    expect(await getGifFrames('m1')).toBeUndefined()
  })

  it('clearAllGifFrames removes across all media', async () => {
    memoryOpfs.setMedia('m2', 'h2')
    await saveGifFrames(makeData('m1'))
    await saveGifFrames(makeData('m2'))
    await clearAllGifFrames()
    expect(await getGifFrames('m1')).toBeUndefined()
    expect(await getGifFrames('m2')).toBeUndefined()
  })
})
