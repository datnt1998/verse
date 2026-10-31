// @vitest-environment node

import { beforeEach, describe, expect, it, vi } from 'vitest'
import '../test-utils/logger-test-mocks'

// vi.mock is hoisted above static imports, so the shared factory can only be
// loaded lazily from inside it.
vi.mock('@editor/storage/storage-backend', async () =>
  (await import('./__tests__/in-memory-opfs')).storageBackendMock,
)

import { memoryOpfs } from './__tests__/in-memory-opfs'
import { resetMediaIdentityTracking } from './cache-identity'

import {
  adoptCaptionsFromCache,
  deleteCaptions,
  getCaptionEmbeddings,
  getCaptionsByContentHash,
  getCaptionsEmbeddingsMeta,
  saveCaptionEmbeddings,
  saveCaptions,
} from './captions'
import { contentCaptionThumbRelPath } from './paths'
import { __resetKeyLocksForTesting } from './with-key-lock'

beforeEach(() => {
  memoryOpfs.reset()
  resetMediaIdentityTracking()
  for (const id of ['m1', 'm2', 'm3']) memoryOpfs.setMedia(id, `hash-${id}`)
  __resetKeyLocksForTesting()
})

describe('workspace-fs captions', () => {
  it('stores shared caption assets in separate cache variants per sample interval', async () => {
    const hash = 'a'.repeat(64)

    await saveCaptions({
      mediaId: 'm1',
      captions: [{ timeSec: 0, text: 'three-second' }],
      service: 'lfm-captioning',
      model: 'lfm-2.5-vl',
      sampleIntervalSec: 3,
      contentHash: hash,
    })
    await saveCaptionEmbeddings('m1', [Float32Array.from([1, 2])], 2, {
      contentHash: hash,
      sampleIntervalSec: 3,
    })
    const shortThumb = contentCaptionThumbRelPath(hash, 0, 3)

    await saveCaptions({
      mediaId: 'm2',
      captions: [{ timeSec: 0, text: 'ten-second' }],
      service: 'lfm-captioning',
      model: 'lfm-2.5-vl',
      sampleIntervalSec: 10,
      contentHash: hash,
    })
    await saveCaptionEmbeddings('m2', [Float32Array.from([3, 4])], 2, {
      contentHash: hash,
      sampleIntervalSec: 10,
    })
    const longThumb = contentCaptionThumbRelPath(hash, 0, 10)

    const shortCache = await getCaptionsByContentHash(hash, 3)
    const longCache = await getCaptionsByContentHash(hash, 10)
    const shortVectors = await getCaptionEmbeddings('m1', 2, 1, {
      contentHash: hash,
      sampleIntervalSec: 3,
    })
    const longVectors = await getCaptionEmbeddings('m2', 2, 1, {
      contentHash: hash,
      sampleIntervalSec: 10,
    })

    expect(shortCache?.data.captions[0]?.text).toBe('three-second')
    expect(longCache?.data.captions[0]?.text).toBe('ten-second')
    expect(shortThumb).toContain('/si-300/')
    expect(longThumb).toContain('/si-1000/')
    expect(shortThumb).not.toBe(longThumb)
    expect(Array.from(shortVectors?.[0] ?? [])).toEqual([1, 2])
    expect(Array.from(longVectors?.[0] ?? [])).toEqual([3, 4])
  })

  it('keeps other interval variants alive when deleting one shared caption cache entry', async () => {
    const hash = 'b'.repeat(64)

    await saveCaptions({
      mediaId: 'm1',
      captions: [{ timeSec: 0, text: 'three-second' }],
      service: 'lfm-captioning',
      model: 'lfm-2.5-vl',
      sampleIntervalSec: 3,
      contentHash: hash,
    })
    await saveCaptions({
      mediaId: 'm2',
      captions: [{ timeSec: 0, text: 'ten-second' }],
      service: 'lfm-captioning',
      model: 'lfm-2.5-vl',
      sampleIntervalSec: 10,
      contentHash: hash,
    })

    const adopted = await adoptCaptionsFromCache('m3', hash, 3)
    expect(adopted?.data.captions[0]?.text).toBe('three-second')
    expect((await getCaptionsEmbeddingsMeta('m3'))?.sampleIntervalSec).toBe(3)

    await deleteCaptions('m1')

    expect(await getCaptionsByContentHash(hash, 3)).toBeDefined()
    expect(await getCaptionsByContentHash(hash, 10)).toBeDefined()

    await deleteCaptions('m3')

    expect(await getCaptionsByContentHash(hash, 3)).toBeUndefined()
    expect(await getCaptionsByContentHash(hash, 10)).toBeDefined()
  })
})
