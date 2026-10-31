// @vitest-environment node

import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { DecodedPreviewAudioBin, DecodedPreviewAudioMeta } from '@editor/types/storage'
import '../test-utils/logger-test-mocks'

// vi.mock is hoisted above static imports, so the shared factory can only be
// loaded lazily from inside it.
vi.mock('@editor/storage/storage-backend', async () =>
  (await import('./__tests__/in-memory-opfs')).storageBackendMock,
)

import { memoryOpfs } from './__tests__/in-memory-opfs'
import { resetMediaIdentityTracking } from './cache-identity'
import {
  deleteDecodedPreviewAudio,
  getDecodedPreviewAudio,
  saveDecodedPreviewAudio,
} from './decoded-preview-audio'

function makeMeta(mediaId: string, binCount: number): DecodedPreviewAudioMeta {
  return {
    id: mediaId,
    mediaId,
    kind: 'meta',
    sampleRate: 22050,
    totalFrames: binCount * 22050 * 30,
    binCount,
    binDurationSec: 30,
    createdAt: 0,
  }
}

function makeBin(mediaId: string, binIndex: number): DecodedPreviewAudioBin {
  const leftBytes = new Uint8Array(8).fill(binIndex + 1)
  const rightBytes = new Uint8Array(8).fill((binIndex + 1) * 2)
  return {
    id: `${mediaId}:bin:${binIndex}`,
    mediaId,
    kind: 'bin',
    binIndex,
    left: leftBytes.buffer,
    right: rightBytes.buffer,
    frames: 4,
    createdAt: 0,
  }
}

beforeEach(() => {
  memoryOpfs.reset()
  resetMediaIdentityTracking()
  memoryOpfs.setMedia('m1', 'h1')
})

describe('workspace-fs decoded-preview-audio', () => {
  it('saveDecodedPreviewAudio (meta) then get round-trips', async () => {
    await saveDecodedPreviewAudio(makeMeta('m1', 3))
    const got = await getDecodedPreviewAudio('m1')
    expect(got!.kind).toBe('meta')
    expect((got as DecodedPreviewAudioMeta).binCount).toBe(3)
  })

  it('saveDecodedPreviewAudio (bin) then get via bin key round-trips', async () => {
    await saveDecodedPreviewAudio(makeBin('m1', 0))
    const got = await getDecodedPreviewAudio('m1:bin:0')
    expect(got!.kind).toBe('bin')
    const bin = got as DecodedPreviewAudioBin
    expect(bin.binIndex).toBe(0)
    expect(new Uint8Array(bin.left).every((b) => b === 1)).toBe(true)
    expect(new Uint8Array(bin.right).every((b) => b === 2)).toBe(true)
  })

  it('returns undefined when missing', async () => {
    expect(await getDecodedPreviewAudio('m1')).toBeUndefined()
    expect(await getDecodedPreviewAudio('m1:bin:0')).toBeUndefined()
  })

  it('returns undefined for a media the server no longer has', async () => {
    await saveDecodedPreviewAudio(makeMeta('m1', 1))
    memoryOpfs.deleteMedia('m1')
    expect(await getDecodedPreviewAudio('m1')).toBeUndefined()
  })

  it('deleteDecodedPreviewAudio removes meta and all bins', async () => {
    await saveDecodedPreviewAudio(makeMeta('m1', 2))
    await saveDecodedPreviewAudio(makeBin('m1', 0))
    await saveDecodedPreviewAudio(makeBin('m1', 1))
    await deleteDecodedPreviewAudio('m1')
    expect(await getDecodedPreviewAudio('m1')).toBeUndefined()
    expect(await getDecodedPreviewAudio('m1:bin:0')).toBeUndefined()
  })

  it('a decode started on old bytes writes under the old hash, never the replaced one', async () => {
    const pinned = { workspaceId: memoryOpfs.workspaceId, mediaId: 'm1', contentHash: 'h1' }

    // The source is replaced while the (slow) decode of the old bytes is running.
    memoryOpfs.setMedia('m1', 'h2')
    await saveDecodedPreviewAudio(makeBin('m1', 0), pinned)
    await saveDecodedPreviewAudio(makeMeta('m1', 1), pinned)

    expect(await getDecodedPreviewAudio('m1')).toBeUndefined()
    expect(await getDecodedPreviewAudio('m1:bin:0')).toBeUndefined()
    expect(
      await memoryOpfs.list(memoryOpfs.mediaPathFor('m1', 'h1', 'cache', 'decoded-audio')),
    ).toContain('meta.json')
    expect(await memoryOpfs.list(memoryOpfs.pathFor('media', 'm1'))).toEqual(['h1'])
  })

  it('drops bin writes that hit the storage quota instead of failing the decode', async () => {
    memoryOpfs.failWrites('QuotaExceededError')
    await expect(saveDecodedPreviewAudio(makeBin('m1', 0))).resolves.toBeUndefined()
    memoryOpfs.failWrites(null)
    expect(await getDecodedPreviewAudio('m1:bin:0')).toBeUndefined()
  })
})
