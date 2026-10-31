import { describe, expect, it, vi } from 'vitest'
import { createMediaSourceSync } from './media-source-sync'
import { canonicalMediaSourceUrl, sourceRefreshPlan, refreshSourceItems, visitSourceItems, SOURCE_MEMORY_CACHES } from '@editor/storage/media-source-refresh'
import type { MediaMetadata } from '@editor/types/storage'
import { voiceShape, VOICE_HASH } from '@editor/test/waveform-fixture'

const oldHash = `sha256:${'a'.repeat(64)}`
const newHash = `sha256:${'b'.repeat(64)}`
const record: MediaMetadata = { id: 'clip', storageType: 'workspace', contentHash: oldHash, fileName: 'clip.mp4', fileSize: 10, mimeType: 'video/mp4', duration: 5, width: 640, height: 360, fps: 30, codec: 'h264', bitrate: 0, tags: [], createdAt: 1, updatedAt: 1, thumbnailId: 'old' }
const event = { workspaceId: 'workspace', mediaId: 'clip', contentHash: newHash }

describe('cross-tab media source refresh', () => {
  it('plans a new immutable URL and every source memory cache eviction', () => {
    const plan = sourceRefreshPlan(event, 'workspace', record)
    expect(plan?.newUrl).toBe(canonicalMediaSourceUrl('clip', newHash))
    expect(plan?.oldUrl).toBe(canonicalMediaSourceUrl('clip', oldHash))
    expect(plan?.caches).toEqual(['blob', 'bitmap', 'thumbnail', 'filmstrip', 'waveform', 'gif', 'proxy', 'decoded-audio'])
    expect(plan?.caches).toEqual(SOURCE_MEMORY_CACHES)
  })
  it('ignores foreign workspaces, unknown media, identical hashes and malformed events', () => {
    expect(sourceRefreshPlan(event, 'different', record)).toBeNull()
    expect(sourceRefreshPlan(event, 'workspace', undefined)).toBeNull()
    expect(sourceRefreshPlan({ ...event, contentHash: oldHash }, 'workspace', record)).toBeNull()
    expect(sourceRefreshPlan({ ...event, contentHash: 'not-a-hash' }, 'workspace', record)).toBeNull()
  })
  it('fetches authoritative metadata once and invalidates before applying new source state', async () => {
    let current = record
    const fresh = { ...record, contentHash: newHash, thumbnailId: 'new' }
    const barrier = Promise.withResolvers<MediaMetadata>()
    const fetchMedia = vi.fn(() => barrier.promise)
    const invalidate = vi.fn()
    const apply = vi.fn((media: MediaMetadata) => { current = media })
    const sync = createMediaSourceSync({ workspaceId: 'workspace', current: () => current, fetchMedia, invalidate, apply })
    const first = sync.handle(event)
    const second = sync.handle(event)
    barrier.resolve(fresh)
    await Promise.all([first, second])
    expect(fetchMedia).toHaveBeenCalledTimes(1)
    expect(invalidate).toHaveBeenCalledWith('clip', canonicalMediaSourceUrl('clip', oldHash), newHash)
    expect(apply).toHaveBeenCalledWith(fresh, canonicalMediaSourceUrl('clip', newHash))
    expect(invalidate.mock.invocationCallOrder[0]).toBeLessThan(apply.mock.invocationCallOrder[0])
    await sync.handle(event)
    expect(fetchMedia).toHaveBeenCalledTimes(1)
  })
  it('uses the latest server hash when broadcast messages arrive out of order', async () => {
    const newest = { ...record, contentHash: `sha256:${'c'.repeat(64)}` }
    const apply = vi.fn()
    const sync = createMediaSourceSync({ workspaceId: 'workspace', current: () => record, fetchMedia: async () => newest, invalidate: vi.fn(), apply })
    await sync.handle(event)
    expect(apply).toHaveBeenCalledWith(newest, canonicalMediaSourceUrl('clip', newest.contentHash))
  })
  it('rewrites root, nested composition and stash sources while preserving editorial timing', () => {
    const item = { type: 'video', mediaId: 'clip', src: 'old', audioSrc: 'old', from: 10, durationInFrames: 40, reverseConformSrc: 'blob:old', thumbnailUrl: 'blob:old-thumbnail' }
    const state = { items: [{ ...item }], compositions: [{ items: [{ ...item }] }], stashStack: [{ items: [{ ...item }] }], mainHolder: { items: [{ ...item }] } }
    visitSourceItems(state, value => refreshSourceItems([value], 'clip', canonicalMediaSourceUrl('clip', newHash), newHash))
    for (const value of [state.items[0], state.compositions[0].items[0], state.stashStack[0].items[0], state.mainHolder.items[0]]) {
      expect(value.src).toBe(canonicalMediaSourceUrl('clip', newHash))
      expect(value.audioSrc).toBe(value.src)
      expect(value.from).toBe(10)
      expect(value.durationInFrames).toBe(40)
      expect(value).not.toHaveProperty('reverseConformSrc')
      expect(value).not.toHaveProperty('thumbnailUrl')
    }
  })
})

it('processes a newer replacement that arrives during an older metadata request', async () => {
  let current = record
  const firstRecord = { ...record, contentHash: newHash }
  const latestRecord = { ...record, contentHash: `sha256:${'c'.repeat(64)}` }
  const barrier = Promise.withResolvers<MediaMetadata>()
  const fetchMedia = vi.fn().mockReturnValueOnce(barrier.promise).mockResolvedValue(latestRecord)
  const sync = createMediaSourceSync({ workspaceId: 'workspace', current: () => current, fetchMedia, invalidate: vi.fn(), apply: media => { current = media } })
  const first = sync.handle(event)
  const second = sync.handle({ ...event, contentHash: latestRecord.contentHash })
  barrier.resolve(firstRecord)
  await Promise.all([first, second])
  expect(current.contentHash).toBe(latestRecord.contentHash)
  expect(fetchMedia).toHaveBeenCalledTimes(2)
})

it('visits only media-backed waveform shapes and invalidates all editorial scopes by authoritative hash', () => {
  const voice = voiceShape()
  const plain = voiceShape({ waveformVisualizer: undefined })
  const state = { items: [{ ...voice }, plain], compositions: [{ items: [{ ...voice }] }], stashStack: [{ items: [{ ...voice }] }], mainHolder: { items: [{ ...voice }] } }
  let visited = 0
  visitSourceItems(state, item => { visited++; refreshSourceItems([item], voice.mediaId!, '/unused', VOICE_HASH) })
  expect(visited).toBe(4)
  expect(state.items[0].waveformVisualizer?.envelopeBase64).toBe(voice.waveformVisualizer?.envelopeBase64)
  visitSourceItems(state, item => refreshSourceItems([item], voice.mediaId!, '/unused', newHash))
  for (const item of [state.items[0], state.compositions[0].items[0], state.stashStack[0].items[0], state.mainHolder.items[0]]) {
    expect(item.waveformVisualizer?.envelopeBase64).toBeUndefined()
    expect(item.waveformVisualizer?.durationSeconds).toBeUndefined()
    expect(item.waveformVisualizer?.normalization).toBeUndefined()
    expect(item.waveformVisualizer?.sourceHash).toBe(VOICE_HASH)
    expect(item.waveformVisualizer?.color).toBe(voice.waveformVisualizer?.color)
  }
  expect(plain).toEqual(voiceShape({ waveformVisualizer: undefined }))
  expect(voice.waveformVisualizer?.envelopeBase64).toBeDefined()
})
