// @vitest-environment node

import { beforeEach, describe, expect, it, vi } from 'vitest'

vi.mock('@editor/storage/storage-backend', async () =>
  (await import('./__tests__/fake-editor-storage')).fakeStorageBackendMock,
)

import { fakeServer } from './__tests__/fake-editor-storage'
import { memoryOpfs } from './__tests__/in-memory-opfs'
import {
  isDerivedMediaKey,
  readDerivedMedia,
  removeDerivedMedia,
  writeDerivedMedia,
} from './media-derived'

beforeEach(() => {
  fakeServer.reset()
})

const HASH = 'sha256:abc'

function derivedFile(mediaId: string, hash: string): string[] {
  return memoryOpfs.mediaPathFor(mediaId, hash, 'derived.json')
}

describe('media-derived overlay', () => {
  it('is empty for media that was never analysed', async () => {
    expect(await readDerivedMedia('m1', HASH)).toEqual({})
  })

  it('persists only whitelisted derived fields', async () => {
    await writeDerivedMedia('m1', HASH, {
      fps: 60,
      keyframeTimestamps: [0, 1],
      // @ts-expect-error — guards against non-derived keys leaking into the cache entry
      fileName: 'nope.mp4',
    })
    await writeDerivedMedia('m1', HASH, {
      // @ts-expect-error — same for server-owned fields
      tags: ['x'],
    })

    expect(await readDerivedMedia('m1', HASH)).toEqual({ fps: 60, keyframeTimestamps: [0, 1] })
  })

  it('merges into the existing entry and removes fields set to undefined', async () => {
    await writeDerivedMedia('m1', HASH, { fps: 60, gopInterval: 2 })

    await writeDerivedMedia('m1', HASH, { gopInterval: undefined })

    expect(await readDerivedMedia('m1', HASH)).toEqual({ fps: 60 })
  })

  it('keeps the overlay under the exact content hash, so other versions are unaffected', async () => {
    await writeDerivedMedia('m1', 'sha256:old', { fps: 60 })
    await writeDerivedMedia('m1', 'sha256:new', { fps: 24 })

    expect(await readDerivedMedia('m1', 'sha256:old')).toEqual({ fps: 60 })
    expect(await readDerivedMedia('m1', 'sha256:new')).toEqual({ fps: 24 })
  })

  it('treats a torn cache entry as a miss and lets the next write recover', async () => {
    await memoryOpfs.plantText(derivedFile('m1', HASH), '{torn json')

    expect(await readDerivedMedia('m1', HASH)).toEqual({})

    await writeDerivedMedia('m1', HASH, { fps: 30 })
    expect(await readDerivedMedia('m1', HASH)).toEqual({ fps: 30 })
  })

  it('treats a failed cache as a miss rather than throwing', async () => {
    memoryOpfs.failReads('NotAllowedError')

    expect(await readDerivedMedia('m1', HASH)).toEqual({})
    await expect(writeDerivedMedia('m1', HASH, { fps: 30 })).resolves.toEqual({ fps: 30 })
    await expect(removeDerivedMedia('m1')).resolves.toBeUndefined()
  })

  it('removeDerivedMedia clears every hashed version of the media', async () => {
    await writeDerivedMedia('m1', 'sha256:a', { fps: 60 })
    await writeDerivedMedia('m1', 'sha256:b', { fps: 24 })
    await writeDerivedMedia('m2', 'sha256:a', { fps: 48 })

    await removeDerivedMedia('m1')

    expect(await memoryOpfs.list(memoryOpfs.pathFor('media', 'm1'))).toEqual([])
    expect(await readDerivedMedia('m2', 'sha256:a')).toEqual({ fps: 48 })
  })

  it('serializes a concurrent read-modify-write pair on the same media', async () => {
    const slow = writeDerivedMedia('m1', HASH, { fps: 60 })
    const fast = writeDerivedMedia('m1', HASH, { gopInterval: 4 })
    await Promise.all([slow, fast])

    expect(await readDerivedMedia('m1', HASH)).toEqual({ fps: 60, gopInterval: 4 })
  })

  it('isDerivedMediaKey matches the overlay whitelist exactly', () => {
    expect(isDerivedMediaKey('fps')).toBe(true)
    expect(isDerivedMediaKey('aiCaptions')).toBe(true)
    expect(isDerivedMediaKey('tags')).toBe(false)
    expect(isDerivedMediaKey('fileName')).toBe(false)
    expect(isDerivedMediaKey('contentHash')).toBe(false)
  })
})
