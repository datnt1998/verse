import assert from 'node:assert/strict'
import { test } from 'node:test'
import type { MediaMetadata } from '@editor/types/storage'
import type { TimelineTrack } from '@editor/types/timeline'
import { buildOverlayCommands } from './overlay-commands'

const media = (overrides: Partial<MediaMetadata> = {}): MediaMetadata => ({
  id: 'media-1', fileName: 'snow.mp4', mimeType: 'video/mp4', duration: 5, width: 320, height: 180, fps: 30, ...overrides,
} as MediaMetadata)

const track = (id: string, order: number): TimelineTrack => ({ id, name: id, kind: 'video', height: 56, locked: false, visible: true, muted: false, solo: false, order, items: [] })

function build(mediaOverrides: Partial<MediaMetadata> = {}, extra: { contentEnd?: number } = {}) {
  let next = 0
  const result = buildOverlayCommands({
    media: media(mediaOverrides), src: '/media/1', fps: 30,
    tracks: [track('v1', 0), track('v2', 1)], items: [{ from: 0, durationInFrames: extra.contentEnd ?? 300 }],
    createId: () => `id-${next++}`,
  })
  const [command] = result.commands as [{ type: string; params: { items: Record<string, unknown>[]; tracks: { id: string; order: number }[] } }]
  return { command, item: command.params.items[0]!, tracks: command.params.tracks, itemId: result.itemId }
}

test('a short video loops, is muted and spans the content on a new top track', () => {
  const { command, item, tracks, itemId } = build()
  assert.equal(command.type, 'item.addOnNewTracks')
  assert.deepEqual(tracks.map((entry) => [entry.id, entry.order]), [['id-0', 0], ['v1', 1], ['v2', 2]])
  assert.equal(itemId, item.id)
  assert.equal(item.trackId, 'id-0')
  assert.equal(item.durationInFrames, 300)
  assert.equal(item.sourceEnd, 150)
  assert.equal(item.loop, true)
  assert.equal(item.embeddedAudioMuted, true)
  assert.equal(item.templateAnchor, 'full')
})

test('every overlay is anchored to the whole video and a media longer than the span does not loop', () => {
  const { item } = build({ duration: 600 })
  assert.equal(item.templateAnchor, 'full')
  assert.equal(item.loop, undefined)
  assert.equal(item.sourceEnd, 300)
  assert.equal(build({ mimeType: 'image/png', fileName: 'sparkle.png', duration: 0 }).item.templateAnchor, 'full')
})

test('detected backgrounds pick a blend or a chroma key', () => {
  const detected = { version: 1, detectedAt: 1, contentHash: 'x' }
  assert.equal(build({ background: { kind: 'black', ...detected } }).item.blendMode, 'screen')
  const green = build({ background: { kind: 'green', keyColor: '#00ff44', ...detected } }).item
  const effect = (green.effects as { effect: { gpuEffectType: string; params: Record<string, unknown> } }[])[0]!.effect
  assert.equal(effect.gpuEffectType, 'gpu-chroma-key')
  assert.equal(effect.params.keyColor, '#00ff44')
})

test('images are stills and audio is refused', () => {
  const still = build({ mimeType: 'image/png', fileName: 'sparkle.png', duration: 0 }).item
  assert.equal(still.type, 'image')
  assert.equal(still.durationInFrames, 300)
  assert.equal(still.loop, undefined)
  assert.equal(still.embeddedAudioMuted, undefined)
  assert.throws(() => build({ mimeType: 'audio/mpeg' }), /video hoặc ảnh/)
})
