import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { test } from 'node:test'
import { createNodeSession, createSeededIdFactory, getCapabilityManifest, type ProjectState } from './index'
import type { Project } from '../types/project'
import type { TimelineItem } from '../types/timeline'
import { useTimelineSettingsStore } from '../features/timeline/stores/timeline-settings-store'
import { useItemsStore } from '../features/timeline/stores/items-store'
import { waveformStateFixture } from '@editor/test/waveform-state-fixture'
import { voiceShape, VOICE_HASH } from '@editor/test/waveform-fixture'
import { collectWaveformIssues } from '@editor/features/waveform-visualizer/readiness'

test('source refresh is headless, non-dirty and absent from undo history', () => {
  assert.equal(typeof window, 'undefined')
  const project = JSON.parse(readFileSync(new URL('./fixtures/project.json', import.meta.url), 'utf8')) as Project
  const timeline = project.timeline!
  const items = timeline.items as TimelineItem[]
  const video = items.find(item => item.type === 'video')!
  video.mediaId = 'clip'
  video.src = '/old'
  const media = { id: 'clip', storageType: 'workspace' as const, contentHash: `sha256:${'a'.repeat(64)}`, fileName: 'clip.mp4', fileSize: 10, mimeType: 'video/mp4', duration: 5, width: 640, height: 360, fps: 30, codec: 'h264', bitrate: 0, tags: [], createdAt: 1, updatedAt: 1 }
  const state: ProjectState = {
    items, tracks: timeline.tracks.map(track => ({ ...track, items: [] })), transitions: timeline.transitions ?? [], keyframes: timeline.keyframes ?? [], markers: timeline.markers ?? [], compositions: [], topLevelSequenceIds: timeline.topLevelSequenceIds ?? [], inPoint: null, outPoint: null, fps: project.metadata.fps, scrollPosition: 0, snapEnabled: true, currentFrame: 0, masterBusDb: 0, projectId: project.id, projectName: project.name, projectMetadata: project.metadata, mediaItems: [media], stashStack: [], mainHolder: null,
  }
  const session = createNodeSession(state, createSeededIdFactory('source'))
  try {
    const dirty = useTimelineSettingsStore.getState().isDirty
    const previousItems = useItemsStore.getState().items
    const previousVideo = previousItems.find(item => item.type === 'video')
    let notified = false
    const unsubscribe = useItemsStore.subscribe((next, previous) => { if (next.items !== previous.items) notified = true })
    const fresh = { ...media, contentHash: `sha256:${'b'.repeat(64)}` }
    const result = session.executor.executeCommands([{ type: 'media.refreshSource', params: { media: fresh, src: '/new' } }])
    assert.equal(result.ok, true, result.ok ? undefined : JSON.stringify(result.issues))
    unsubscribe()
    assert.equal(notified, true, 'live preview subscribers receive a new item collection')
    assert.notEqual(useItemsStore.getState().items, previousItems)
    assert.equal(previousVideo?.src, '/old', 'refresh never mutates the previous render snapshot')
    const refreshedVideo = session.getState().items.find(item => item.type === 'video')
    assert.ok(refreshedVideo && refreshedVideo.type === 'video')
    assert.equal(refreshedVideo.src, '/new')
    assert.equal(session.getState().mediaItems?.[0].contentHash, fresh.contentHash)
    assert.equal(session.executor.canUndo, false)
    assert.equal(useTimelineSettingsStore.getState().isDirty, dirty)
    assert.ok(getCapabilityManifest().commands.some(command => command.type === 'media.refreshSource'))
    assert.equal(typeof document, 'undefined')
  } finally { session.dispose() }
})

test('hash refresh preserves unchanged waveform bytes and invalidates derived data across every editorial collection without mutating snapshots', () => {
  const state = waveformStateFixture()
  const voice = voiceShape({ from: 10, sourceStart: 20, speed: 2 })
  state.items = [voice]
  state.compositions = [{ id: 'nested', name: 'Nested', items: [{ ...voice, id: 'nested-voice' }], tracks: state.tracks, transitions: [], keyframes: [], fps: 30, width: 128, height: 64, durationInFrames: 120 }]
  state.stashStack = [{ compositionId: null, currentFrame: 0, items: [{ ...voice, id: 'stash-voice' }], tracks: state.tracks, transitions: [], keyframes: [], markers: [], inPoint: null, outPoint: null, scrollPosition: 0 }]
  state.mainHolder = { compositionId: null, currentFrame: 0, items: [{ ...voice, id: 'holder-voice' }], tracks: state.tracks, transitions: [], keyframes: [], markers: [], inPoint: null, outPoint: null, scrollPosition: 0 }
  const session = createNodeSession(state)
  try {
    const media = state.mediaItems![0]
    const initial = session.getState()
    const same = session.executor.executeCommands([{ type: 'media.refreshSource', params: { media, src: '/same' } }])
    assert.equal(same.ok, true)
    assert.deepEqual(session.getState().items, initial.items)
    const previous = useItemsStore.getState().items[0]
    const dirty = useTimelineSettingsStore.getState().isDirty
    const result = session.executor.executeCommands([{ type: 'media.refreshSource', params: { media: { ...media, contentHash: `sha256:${'b'.repeat(64)}` }, src: '/new' } }])
    assert.equal(result.ok, true, result.ok ? '' : JSON.stringify(result.issues))
    const refreshed = session.getState()
    const scopes = [refreshed.items, ...refreshed.compositions.map(composition => composition.items), ...(refreshed.stashStack ?? []).map(stash => stash.items), ...(refreshed.mainHolder ? [refreshed.mainHolder.items] : [])]
    for (const items of scopes) for (const item of items) {
      assert.equal(item.type, 'shape')
      if (item.type !== 'shape') throw new Error('Expected shape')
      const payload = item.waveformVisualizer!
      assert.equal(payload.envelopeBase64, undefined); assert.equal(payload.durationSeconds, undefined); assert.equal(payload.normalization, undefined)
      assert.equal(payload.sourceHash, VOICE_HASH); assert.equal(payload.color, voice.waveformVisualizer!.color); assert.equal(payload.bars, voice.waveformVisualizer!.bars)
      assert.equal(item.from, 10); assert.equal(item.sourceStart, 20); assert.equal(item.speed, 2)
      assert.equal(collectWaveformIssues([item]).length, 1)
    }
    assert.ok(previous.type === 'shape' && previous.waveformVisualizer?.envelopeBase64)
    assert.equal(session.executor.canUndo, false)
    assert.equal(useTimelineSettingsStore.getState().isDirty, dirty)
  } finally { session.dispose() }
})
