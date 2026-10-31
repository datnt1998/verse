// @vitest-environment node

import { beforeEach, describe, expect, it, vi } from 'vitest'

vi.mock('@/editor/ui/shell/announcer', () => ({ announce: vi.fn() }))
vi.mock('@editor/features/media-library/utils/media-resolver', () => ({ resolveMediaUrl: vi.fn() }))
vi.mock('@editor/commands', () => ({ executeCommands: vi.fn(() => ({ ok: true })) }))
vi.mock('@editor/features/timeline/stores/timeline-actions', () => ({ addEffect: vi.fn(), addTransition: vi.fn() }))
vi.mock('@editor/infrastructure/gpu-effects/registry', () => ({ getGpuEffectDefaultParams: vi.fn(() => ({})) }))

import { makeTimelineTrack, makeTimelineVideoItem } from '@editor/features/timeline/test-helpers'
import { useItemsStore } from '@editor/features/timeline/stores/items-store'
import { destination } from './library-actions'

describe('destination', () => {
  beforeEach(() => {
    useItemsStore.getState().setTracks([
      makeTimelineTrack({ id: 'track-v1', name: 'V1', kind: 'video', order: 0 }),
      makeTimelineTrack({ id: 'track-v2', name: 'V2', kind: 'video', order: 1 }),
    ])
    useItemsStore.getState().setItems([])
  })

  it('reuses the asked-for track when it is free at the drop point', () => {
    const plan = destination('video', 0, 60, 'track-v2')
    expect(plan.isNew).toBe(false)
    expect(plan.track.id).toBe('track-v2')
  })

  it('skips the asked-for track when a clip already occupies the drop point', () => {
    useItemsStore.getState().setItems([makeTimelineVideoItem({ id: 'existing', trackId: 'track-v2', from: 0 })])

    const plan = destination('video', 10, 60, 'track-v2')

    // track-v2 is occupied at [10, 70); the plan must pick a free track or a new one, never overlap
    expect(plan.track.id).toBe('track-v1')
    expect(plan.isNew).toBe(false)
  })

  it('inserts a brand-new video track at the top (order 0) so the drop is never hidden behind image layers', () => {
    useItemsStore.getState().setItems([
      makeTimelineVideoItem({ id: 'a', trackId: 'track-v1', from: 0 }),
      makeTimelineVideoItem({ id: 'b', trackId: 'track-v2', from: 0 }),
    ])

    const plan = destination('video', 10, 60)

    expect(plan.isNew).toBe(true)
    expect(plan.track.order).toBe(0)
    expect(plan.tracks[0]?.id).toBe(plan.track.id)
    expect(plan.tracks.find(t => t.id === 'track-v1')?.order).toBe(1)
    expect(plan.tracks.find(t => t.id === 'track-v2')?.order).toBe(2)
  })

  it('appends a new audio track at the bottom like before', () => {
    const plan = destination('audio', 0, 60)

    expect(plan.isNew).toBe(true)
    expect(plan.track.kind).toBe('audio')
    expect(plan.tracks.at(-1)?.id).toBe(plan.track.id)
    expect(plan.track.order).toBe(2)
  })
})

import { addOverlay } from './library-actions'
import { resolveMediaUrl } from '@editor/features/media-library/utils/media-resolver'
import { executeCommands } from '@editor/commands'
import { usePlaybackStore } from '@editor/shared/state/playback'
import { useTimelineSettingsStore } from '@editor/features/timeline/stores/timeline-settings-store'
import type { MediaMetadata } from '@editor/types/storage'

function overlayMedia(overrides: Partial<MediaMetadata> = {}): MediaMetadata {
  return {
    id: 'media-1',
    fileName: 'snow.mp4',
    mimeType: 'video/mp4',
    fileSize: 1,
    size: 1,
    fileType: 'video',
    duration: 5,
    width: 320,
    height: 180,
    fps: 30,
    bitrate: 0,
    codec: 'h264',
    tags: [],
    createdAt: 0,
    updatedAt: 0,
    ...overrides,
  } as MediaMetadata
}

describe('addOverlay', () => {
  beforeEach(() => {
    vi.mocked(executeCommands).mockClear()
    vi.mocked(resolveMediaUrl).mockResolvedValue('blob:media-1')
    usePlaybackStore.setState({ currentFrame: 120 })
    useTimelineSettingsStore.setState({ fps: 30 })
    useItemsStore.getState().setTracks([makeTimelineTrack({ id: 'track-v1', name: 'V1', kind: 'video', order: 0 })])
    useItemsStore.getState().setItems([makeTimelineVideoItem({ id: 'tail', trackId: 'track-v1', from: 0, durationInFrames: 300 })])
  })

  const lastItem = () => {
    const [command] = vi.mocked(executeCommands).mock.calls[0]![0] as [{ type: string; params: { items: Record<string, unknown>[]; tracks: { kind: string; name: string; order: number }[] } }]
    return { command, item: command.params.items[0]! }
  }

  it('adds a muted, looping clip on a fresh top track spanning the whole timeline', async () => {
    await addOverlay(overlayMedia())

    const { command, item } = lastItem()
    expect(command.type).toBe('item.addOnNewTracks')
    expect(command.params.tracks[0]!.kind).toBe('video')
    expect(command.params.tracks[0]!.name).toBe('Lớp phủ: snow.mp4')
    expect(command.params.tracks[0]!.order).toBe(0)
    expect(item.type).toBe('video')
    // From frame 0 to the content tail (300 frames), never the playhead.
    expect(item.from).toBe(0)
    expect(item.durationInFrames).toBe(300)
    // 5 s media < 10 s span → loop on, window keeps the whole 150-frame source.
    expect(item.loop).toBe(true)
    expect(item.sourceEnd).toBe(150)
    expect(item.embeddedAudioMuted).toBe(true)
    expect(item.effects).toBeUndefined()
  })

  it('screens black backgrounds and keys detected greens with the detected colour', async () => {
    await addOverlay(overlayMedia({ background: { kind: 'black', version: 1, detectedAt: 1, contentHash: 'x' } }))
    expect(lastItem().item.blendMode).toBe('screen')
    expect(lastItem().item.effects).toBeUndefined()

    vi.mocked(executeCommands).mockClear()
    await addOverlay(overlayMedia({ background: { kind: 'green', keyColor: '#00ff44', version: 1, detectedAt: 1, contentHash: 'x' } }))
    const green = lastItem().item
    const effect = (green.effects as { effect: { gpuEffectType: string; params: Record<string, unknown> } }[])[0]
    expect(effect?.effect.gpuEffectType).toBe('gpu-chroma-key')
    expect(effect?.effect.params.keyColor).toBe('#00ff44')
    expect(green.blendMode).toBeUndefined()
  })

  it('trims a long source to the span instead of looping or extending the project', async () => {
    // 10-min overlay on a 10-s project: spans the project, no loop.
    await addOverlay(overlayMedia({ duration: 600 }))

    const { item } = lastItem()
    expect(item.durationInFrames).toBe(300)
    expect(item.loop).toBeUndefined()
    expect(item.sourceEnd).toBe(300)
  })

  it('lays a PNG overlay down as a still image item without loop or audio fields', async () => {
    await addOverlay(overlayMedia({ mimeType: 'image/png', fileName: 'sparkle.png', duration: 0 }))

    const { item } = lastItem()
    expect(item.type).toBe('image')
    expect(item.loop).toBeUndefined()
    expect(item.embeddedAudioMuted).toBeUndefined()
    expect(item.sourceEnd).toBeUndefined()
    expect(item.durationInFrames).toBe(300)
  })

  it('uses the media length as the span on an empty timeline', async () => {
    useItemsStore.getState().setItems([])
    await addOverlay(overlayMedia({ duration: 5 }))

    const { item } = lastItem()
    expect(item.durationInFrames).toBe(150)
    expect(item.loop).toBeUndefined()
  })
})
