import { describe, expect, it, vi } from 'vitest'
import { cacheSegments, mediaCacheKey, readCacheBlob, writeCacheBlob, type CacheIdentity } from './opfs-cache'

const identity: CacheIdentity = { workspaceId: 'workspace-a', mediaId: 'media-a', contentHash: 'sha256:abc' }
describe('OPFS media cache identity', () => {
  it('invalidates on workspace, media or authoritative content hash changes', () => {
    const initial = mediaCacheKey(identity)
    expect(mediaCacheKey({ ...identity, workspaceId: 'workspace-b' })).not.toBe(initial)
    expect(mediaCacheKey({ ...identity, mediaId: 'media-b' })).not.toBe(initial)
    expect(mediaCacheKey({ ...identity, contentHash: 'sha256:def' })).not.toBe(initial)
    expect(mediaCacheKey({ ...identity })).toBe(initial)
  })
  it('pins worker writes to decoded content rather than a later server version', async () => {
    const path = ['media', identity.mediaId, 'cache', 'filmstrip', '0.jpg']
    expect(await cacheSegments(path, identity)).toEqual(['ytf-editor-cache', 'workspace-a', 'media', 'media-a', 'sha256%3Aabc', 'cache', 'filmstrip', '0.jpg'])
    expect(await cacheSegments(path, { ...identity, contentHash: 'sha256:def' })).not.toEqual(await cacheSegments(path, identity))
    await expect(cacheSegments(path, { ...identity, mediaId: 'other' })).rejects.toThrow('Media không khớp')
  })
  it('treats quota failures and cache eviction as misses', async () => {
    const getDirectory = vi.fn().mockRejectedValue(new DOMException('quota', 'QuotaExceededError'))
    vi.stubGlobal('navigator', { storage: { getDirectory } })
    try {
      expect(await readCacheBlob(['media', identity.mediaId, 'cache', 'waveform.bin'], identity)).toBeNull()
      await expect(writeCacheBlob(['media', identity.mediaId, 'cache', 'waveform.bin'], 'bins', identity)).resolves.toBeUndefined()
    } finally { vi.unstubAllGlobals() }
  })
})
