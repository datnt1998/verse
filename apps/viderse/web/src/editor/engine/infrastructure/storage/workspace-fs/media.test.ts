// @vitest-environment node

import { beforeEach, describe, expect, it, vi } from 'vitest'
import { StorageClientError } from '@editor/storage/errors'

vi.mock('@editor/storage/storage-backend', async () =>
  (await import('./__tests__/fake-editor-storage')).fakeStorageBackendMock,
)

import { fakeServer } from './__tests__/fake-editor-storage'
import { memoryOpfs } from './__tests__/in-memory-opfs'
import { deleteMedia, getAllMedia, getMedia, updateMedia, type MediaMetadataUpdate } from './media'
import { readDerivedMedia } from './media-derived'

beforeEach(() => {
  fakeServer.reset()
})

describe('workspace-fs media (server-backed)', () => {
  it('getMedia maps the server record and never carries a file handle', async () => {
    fakeServer.seed({ id: 'm1', fileName: 'clip.mp4', size: 4096 })

    const media = await getMedia('m1')

    expect(media).toMatchObject({
      id: 'm1',
      storageType: 'workspace',
      fileName: 'clip.mp4',
      fileSize: 4096,
      mimeType: 'video/mp4',
      contentHash: 'sha256:seed-m1',
      duration: 10,
      width: 1920,
      height: 1080,
      fps: 30,
      codec: 'h264',
      tags: [],
    })
    expect(media).not.toHaveProperty('fileHandle')
    expect(media).not.toHaveProperty('opfsPath')
  })

  it('getMedia returns undefined for a missing media but rethrows other failures', async () => {
    expect(await getMedia('missing')).toBeUndefined()

    fakeServer.seed({ id: 'm1' })
    const failure = new StorageClientError('internal_error', 'boom', 500)
    vi.spyOn(fakeServer, 'requireRecord').mockImplementationOnce(() => {
      throw failure
    })
    await expect(getMedia('m1')).rejects.toBe(failure)
  })

  it('getAllMedia lists every server media', async () => {
    fakeServer.seed({ id: 'a' })
    fakeServer.seed({ id: 'b' })

    const all = await getAllMedia()

    expect(new Set(all.map((media) => media.id))).toEqual(new Set(['a', 'b']))
  })

  it('layers browser-derived fields over the server probe', async () => {
    fakeServer.seed({ id: 'm1' })

    await updateMedia('m1', { fps: 120, keyframeTimestamps: [0, 2], gopInterval: 2 })

    expect(await getMedia('m1')).toMatchObject({
      fps: 120,
      keyframeTimestamps: [0, 2],
      gopInterval: 2,
      width: 1920,
    })
  })

  it('keys the derived overlay by content hash, so replaced bytes start clean', async () => {
    fakeServer.seed({ id: 'm1', contentHash: 'sha256:old' })
    await updateMedia('m1', { fps: 120 })

    fakeServer.seed({ id: 'm1', contentHash: 'sha256:new' })

    expect((await getMedia('m1'))?.fps).toBe(30)
    expect(await readDerivedMedia('m1', 'sha256:old')).toEqual({ fps: 120 })
  })

  it('persists tags and attribution on the server, not in the overlay', async () => {
    fakeServer.seed({ id: 'm1' })

    const updated = await updateMedia('m1', {
      tags: ['voiceover'],
      attribution: { provider: 'LottieFiles', author: 'Ada' },
    })

    expect(fakeServer.requireRecord('m1')).toMatchObject({
      tags: ['voiceover'],
      attribution: { provider: 'LottieFiles', author: 'Ada' },
    })
    expect(updated.tags).toEqual(['voiceover'])
    expect(await readDerivedMedia('m1', 'sha256:seed-m1')).toEqual({})
  })

  it('clears attribution with null and keeps tags that were not part of the update', async () => {
    fakeServer.seed({ id: 'm1', tags: ['keep'], attribution: { provider: 'X' } })

    const updated = await updateMedia('m1', { attribution: null })

    expect(updated.attribution).toBeUndefined()
    expect(updated.tags).toEqual(['keep'])
  })

  it('does not call the server when only derived fields change', async () => {
    fakeServer.seed({ id: 'm1' })
    const spy = vi.spyOn(fakeServer, 'updateMediaMetadata')

    await updateMedia('m1', { thumbnailId: 'thumb-1' })

    expect(spy).not.toHaveBeenCalled()
    expect((await getMedia('m1'))?.thumbnailId).toBe('thumb-1')
  })

  it('rejects fields that only a byte replacement may change', async () => {
    fakeServer.seed({ id: 'm1' })

    await expect(
      updateMedia('m1', { fileName: 'renamed.mp4' } as unknown as MediaMetadataUpdate),
    ).rejects.toThrow(/Cannot update media field "fileName"/)
  })

  it('updateMedia on a missing media fails with not_found', async () => {
    await expect(updateMedia('nope', { fps: 1 })).rejects.toMatchObject({ code: 'not_found' })
  })

  it('deleteMedia removes the server record and every cached version of it', async () => {
    fakeServer.seed({ id: 'm1', contentHash: 'sha256:v1' })
    await updateMedia('m1', { fps: 60 })
    fakeServer.seed({ id: 'm1', contentHash: 'sha256:v2' })
    await updateMedia('m1', { fps: 24 })
    expect(await memoryOpfs.list(memoryOpfs.pathFor('media', 'm1'))).toHaveLength(2)

    await deleteMedia('m1')

    expect(await getMedia('m1')).toBeUndefined()
    expect(await memoryOpfs.list(memoryOpfs.pathFor('media', 'm1'))).toEqual([])
  })
})
