// @vitest-environment node
import { describe, expect, it } from 'vitest'
import type { TimelineItem } from '@editor/types/timeline'
import type { MediaMetadata } from '@editor/types/storage'
import { makeTimelineAudioItem, makeTimelineTrack, makeTimelineVideoItem } from '@editor/features/timeline/test-helpers'
import { useItemsStore } from '@editor/features/timeline/stores/items-store'
import { usePlaybackStore } from '@editor/shared/state/playback'
import { useMediaLibraryStore } from '@editor/features/media-library/stores/media-library-store'
import { buildSubtitleTextItems } from '@editor/features/media-library/utils/caption-items'
import { placeItemsWithoutTimelineOverlap } from '@editor/features/timeline/stores/actions/item-placement'
import { parseSrt } from '@editor/shared/utils/subtitles'
import { captureProjectState, createNodeSession, createSeededIdFactory, listCommands, withIdFactory, type ProjectState, type SerializedCommand } from '../index'
import { audioEqSchema } from './audio'

const brightness = { type: 'gpu-effect' as const, gpuEffectType: 'gpu-brightness', params: { brightness: 0.4 } }
const fxA = { id: 'fx-a', effect: brightness, enabled: true }
const fxB = { id: 'fx-b', effect: { ...brightness, params: { brightness: 0.8 } }, enabled: false }
const srt = '1\n00:00:01,000 --> 00:00:02,500\nMột dòng\n\n2\n00:00:03,000 --> 00:00:04,000\nHai dòng'
const media: MediaMetadata = { id: 'media-1', storageType: 'workspace', fileName: 'clip.mp4', fileSize: 50, mimeType: 'video/mp4', duration: 4, width: 1920, height: 1080, fps: 30, codec: 'h264', bitrate: 4000, tags: [], createdAt: 1, updatedAt: 1 }

function fixture(): ProjectState {
  const base = { trackId: 'track-v1', from: 120, durationInFrames: 120, label: 'Layer' }
  const items: TimelineItem[] = [
    makeTimelineVideoItem({ effects: [fxA, fxB] }),
    makeTimelineVideoItem({ id: 'video-2', from: 60, mediaId: 'media-2' }),
    makeTimelineAudioItem(),
    { ...base, id: 'text-1', type: 'text', text: 'Hello', color: '#ffffff', fontSize: 40, backgroundColor: '#000000' },
    { ...base, id: 'shape-1', type: 'shape', shapeType: 'rectangle', fillColor: '#ff0000', blendMode: 'multiply' },
    { ...base, id: 'lottie-1', type: 'lottie', src: '/animation.json', frameRate: 30, totalFrames: 90 },
    { ...base, id: 'subtitle-1', type: 'subtitle', color: '#ffffff', source: { type: 'subtitle-import', fileName: 'sub.srt', format: 'srt', importedAt: 1 }, cues: [{ id: 'cue-1', startSeconds: 0, endSeconds: 1, text: 'Old' }, { id: 'cue-2', startSeconds: 2, endSeconds: 3, text: 'Second' }] },
  ]
  return { ...captureProjectState(), projectId: null, projectName: undefined, projectMetadata: null, mediaItems: [media], items, tracks: [makeTimelineTrack({ id: 'track-v1', name: 'V1', kind: 'video', order: 0 }), makeTimelineTrack({ id: 'track-a1', name: 'A1', kind: 'audio', order: 1 }), makeTimelineTrack({ id: 'track-c', name: 'V2', kind: 'video', order: 2 })], transitions: [], keyframes: [], compositions: [], markers: [], topLevelSequenceIds: [], stashStack: [], mainHolder: null, inPoint: null, outPoint: null, fps: 30, currentFrame: 0, busAudioEq: undefined, masterBusDb: 0 }
}

function json(value: unknown): unknown { return JSON.parse(JSON.stringify(value)) }
function patchItem(id: string, updates: Partial<TimelineItem>): void { useItemsStore.getState()._updateItem(id, updates) }
function patchTrack(updates: Partial<ProjectState['tracks'][number]>): void {
  useItemsStore.getState().setTracks(useItemsStore.getState().tracks.map((track) => track.id === 'track-a1' ? { ...track, ...updates } : track))
}

const eq = { enabled: true, lowGainDb: 7, lowCutFrequencyHz: 50 }
const cues = [{ id: 'replacement', startSeconds: 0.5, endSeconds: 1.5, text: 'New cue' }]
const audioPatch = { volume: 80, audioFadeIn: 0.3, audioFadeOut: 0.6, audioFadeInCurve: 9, audioFadeOutCurveX: -1, audioPitchSemitones: 40, audioPitchCents: -200, audioDucking: { duckOthersDb: -8, attackSec: 0.1, releaseSec: 0.4, targetTrackIds: ['track-v1'] } }

const cases: Array<{ command: SerializedCommand; reference: () => void }> = [
  { command: { type: 'effect.add', params: { itemId: 'video-1', effect: brightness } }, reference: () => useItemsStore.getState()._addEffect('video-1', brightness) },
  { command: { type: 'effect.addMany', params: { updates: [{ itemId: 'video-1', effects: [brightness, brightness] }, { itemId: 'video-2', effects: [brightness] }] } }, reference: () => useItemsStore.getState()._addEffects([{ itemId: 'video-1', effects: [brightness, brightness] }, { itemId: 'video-2', effects: [brightness] }]) },
  { command: { type: 'effect.update', params: { itemId: 'video-1', effectId: 'fx-a', updates: { effect: { ...brightness, params: { brightness: 0.9 } }, enabled: false } } }, reference: () => useItemsStore.getState()._updateEffect('video-1', 'fx-a', { effect: { ...brightness, params: { brightness: 0.9 } }, enabled: false }) },
  { command: { type: 'effect.remove', params: { itemId: 'video-1', effectId: 'fx-a' } }, reference: () => useItemsStore.getState()._removeEffect('video-1', 'fx-a') },
  { command: { type: 'effect.toggle', params: { itemId: 'video-1', effectId: 'fx-a' } }, reference: () => useItemsStore.getState()._toggleEffect('video-1', 'fx-a') },
  { command: { type: 'effect.setItems', params: { updates: [{ itemId: 'video-1', effects: [fxB, fxA] }] } }, reference: () => useItemsStore.getState()._setItemEffects([{ itemId: 'video-1', effects: [fxB, fxA] }]) },
  { command: { type: 'effect.setBlendMode', params: { itemId: 'video-1', blendMode: 'screen' } }, reference: () => patchItem('video-1', { blendMode: 'screen' }) },
  { command: { type: 'audio.setClip', params: { itemId: 'audio-1', updates: audioPatch } }, reference: () => patchItem('audio-1', audioPatch) },
  { command: { type: 'audio.setClipEq', params: { itemId: 'audio-1', eq } }, reference: () => patchItem('audio-1', Object.fromEntries(Object.keys(audioEqSchema.shape).map((key) => [`audioEq${key[0]!.toUpperCase()}${key.slice(1)}`, eq[key as keyof typeof eq]]))) },
  { command: { type: 'audio.setTrackVolume', params: { trackId: 'track-a1', volume: 20 } }, reference: () => patchTrack({ volume: 20 }) },
  { command: { type: 'audio.setTrackMute', params: { trackId: 'track-a1', muted: true } }, reference: () => patchTrack({ muted: true }) },
  { command: { type: 'audio.setTrackSolo', params: { trackId: 'track-a1', solo: true } }, reference: () => patchTrack({ solo: true }) },
  { command: { type: 'audio.setTrackEq', params: { trackId: 'track-a1', eq } }, reference: () => patchTrack({ audioEq: eq }) },
  { command: { type: 'audio.setBusEq', params: { eq } }, reference: () => usePlaybackStore.getState().setBusAudioEq(eq) },
  { command: { type: 'audio.setMasterGain', params: { db: -80 } }, reference: () => usePlaybackStore.getState().setMasterBusDb(-80) },
  { command: { type: 'text.update', params: { itemId: 'text-1', updates: { text: 'Nội dung', fontSize: 60, textSpans: [{ text: 'Một', color: '#ff0000' }, { text: 'Hai' }], spanLayout: 'inline', textLayoutDrafts: { single: { text: 'Nội dung' } } }, clearFields: ['backgroundColor'] } }, reference: () => patchItem('text-1', { text: 'Nội dung', fontSize: 60, textSpans: [{ text: 'Một', color: '#ff0000' }, { text: 'Hai' }], spanLayout: 'inline', textLayoutDrafts: { single: { text: 'Nội dung' } }, backgroundColor: undefined } as Partial<TimelineItem>) },
  { command: { type: 'text.updateShape', params: { itemId: 'shape-1', updates: { isMask: true, maskFeather: 12, maskOpacity: 50, fillType: 'linear', gradientStartColor: '#ff0000', gradientEndColor: '#0000ff', strokeEnabled: true, strokeWidth: 0, taperStartWidth: 400 } } }, reference: () => patchItem('shape-1', { isMask: true, maskFeather: 12, maskOpacity: 50, fillType: 'linear', gradientStartColor: '#ff0000', gradientEndColor: '#0000ff', strokeEnabled: true, strokeWidth: 0, taperStartWidth: 400 } as Partial<TimelineItem>) },
  { command: { type: 'text.updateLottie', params: { itemId: 'lottie-1', updates: { loopMode: 'pingpong', reversed: true, segmentStart: 10, segmentEnd: 80, textOverrides: { title: 'Title' }, colorOverrides: { fill: '#ff0000' }, slotOverrides: { size: [10, 20], speed: 2 } } } }, reference: () => patchItem('lottie-1', { loopMode: 'pingpong', reversed: true, segmentStart: 10, segmentEnd: 80, textOverrides: { title: 'Title' }, colorOverrides: { fill: '#ff0000' }, slotOverrides: { size: [10, 20], speed: 2 } } as Partial<TimelineItem>) },
  { command: { type: 'subtitle.import', params: { trackId: 'track-c', fileName: 'sub.srt', format: 'srt', content: srt, importedAt: 7 } }, reference: () => useItemsStore.getState()._addItems(placeItemsWithoutTimelineOverlap(buildSubtitleTextItems({ trackId: 'track-c', fileName: 'sub.srt', format: 'srt', cues: parseSrt(srt).cues, timelineFps: 30, canvasWidth: 1920, canvasHeight: 1080, importedAt: 7, idFactory: createSeededIdFactory('parity').mint }))) },
  { command: { type: 'subtitle.setCues', params: { itemId: 'subtitle-1', cues } }, reference: () => patchItem('subtitle-1', { cues } as Partial<TimelineItem>) },
  { command: { type: 'subtitle.updateCue', params: { itemId: 'subtitle-1', cueId: 'cue-1', updates: { text: 'Edited', startSeconds: 0.2, endSeconds: 1.2 } } }, reference: () => { const item = useItemsStore.getState().itemById['subtitle-1']; if (item?.type === 'subtitle') patchItem(item.id, { cues: item.cues.map((cue) => cue.id === 'cue-1' ? { ...cue, text: 'Edited', startSeconds: 0.2, endSeconds: 1.2 } : cue) } as Partial<TimelineItem>) } },
  { command: { type: 'subtitle.setStyle', params: { itemId: 'subtitle-1', style: { fontSize: 60, color: '#ff0000', textAlign: 'left' } } }, reference: () => patchItem('subtitle-1', { fontSize: 60, color: '#ff0000', textAlign: 'left' } as Partial<TimelineItem>) },
  { command: { type: 'media.add', params: { media: { ...media, id: 'media-2' } } }, reference: () => useMediaLibraryStore.setState((state) => ({ mediaItems: [{ ...media, id: 'media-2' }, ...state.mediaItems] })) },
  { command: { type: 'media.refreshSource', params: { media: { ...media, contentHash: `sha256:${'b'.repeat(64)}`, width: 640, height: 360 }, src: '/replacement.mp4' } }, reference: () => {
    useMediaLibraryStore.setState(state => ({ mediaItems: state.mediaItems.map(record => record.id === media.id ? { ...media, contentHash: `sha256:${'b'.repeat(64)}`, width: 640, height: 360 } : record) }))
    for (const item of useItemsStore.getState().items) {
      if (item.mediaId !== media.id) continue
      patchItem(item.id, { src: '/replacement.mp4', ...(item.type === 'video' || item.type === 'audio' ? { audioSrc: '/replacement.mp4' } : {}), sourceWidth: 640, sourceHeight: 360, sourceFps: 30, sourceDuration: 120 } as Partial<TimelineItem>)
    }
  } },
  { command: { type: 'media.update', params: { mediaId: 'media-1', updates: { tags: ['travel'], attribution: { provider: 'Source', author: 'Dat' } } } }, reference: () => useMediaLibraryStore.setState((state) => ({ mediaItems: state.mediaItems.map((record) => record.id === 'media-1' ? { ...record, tags: ['travel'], attribution: { provider: 'Source', author: 'Dat' } } : record) })) },
  { command: { type: 'media.setRecords', params: { mediaItems: [{ ...media, id: 'media-2' }] } }, reference: () => useMediaLibraryStore.setState({ mediaItems: [{ ...media, id: 'media-2' }] }) },
  { command: { type: 'media.removeRecords', params: { mediaIds: ['media-1'] } }, reference: () => useMediaLibraryStore.setState({ mediaItems: [] }) },
  { command: { type: 'media.remove', params: { mediaIds: ['media-1'] } }, reference: () => { useItemsStore.getState().setItems(useItemsStore.getState().items.filter((item) => item.mediaId !== 'media-1')); useMediaLibraryStore.setState({ mediaItems: [] }) } },
  { command: { type: 'media.relinkItems', params: { itemIds: ['video-1', 'audio-1'], updates: { mediaId: 'media-2', label: 'Replacement' }, clearFields: ['src'] } }, reference: () => { patchItem('video-1', { mediaId: 'media-2', label: 'Replacement', src: undefined } as Partial<TimelineItem>); patchItem('audio-1', { mediaId: 'media-2', label: 'Replacement', src: undefined } as Partial<TimelineItem>) } },
]

for (const { command, reference } of cases) {
  describe(command.type, () => {
    it('matches Freecut mutation, snapshot undo, redo and JSON replay', () => {
      const initial = fixture()
      let expected: unknown
      const baseline = createNodeSession(initial, createSeededIdFactory('parity'))
      try {
        withIdFactory(createSeededIdFactory('parity'), reference)
        expected = json(baseline.getState())
      } finally { baseline.dispose() }
      const session = createNodeSession(initial, createSeededIdFactory('parity'))
      try {
        const before = json(session.getState())
        const result = session.executor.executeCommands([JSON.parse(JSON.stringify(command)) as SerializedCommand])
        expect(result.ok, JSON.stringify(result)).toBe(true)
        expect(json(session.getState())).toEqual(expected)
        if (command.type === 'media.refreshSource') {
          const definition = listCommands().find(entry => entry.type === command.type)
          expect(definition?.history).toBe('none')
          expect(definition?.dirty).toBe(false)
        }
        const after = json(session.getState())
        if (listCommands().find((entry) => entry.type === command.type)?.history !== 'none') {
          expect(session.executor.undo().ok).toBe(true)
          expect(json(session.getState())).toEqual(before)
          expect(session.executor.redo().ok).toBe(true)
          expect(json(session.getState())).toEqual(after)
        } else expect(session.executor.canUndo).toBe(false)
      } finally { session.dispose() }
    })
  })
}

describe('media command boundaries', () => {
  it('covers every registered command in the five domains', () => {
    const covered = new Set(cases.map((entry) => entry.command.type))
    const missing = listCommands().filter((entry) => /^(effect|audio|text|subtitle|media)\./.test(entry.type)).filter((entry) => !covered.has(entry.type)).map((entry) => entry.type)
    expect(missing).toEqual([])
  })

  it('validates all commands before committing any state', () => {
    const session = createNodeSession(fixture(), createSeededIdFactory('atomic'))
    try {
      const before = session.getState()
      const result = session.executor.executeCommands([{ type: 'audio.setMasterGain', params: { db: 5 } }, { type: 'effect.add', params: { itemId: 'audio-1', effect: brightness } }])
      expect(result.ok).toBe(false)
      expect(session.getState()).toEqual(before)
    } finally { session.dispose() }
  })

  it('rejects invalid cue timing and immutable metadata ids without mutating', () => {
    const session = createNodeSession(fixture())
    try {
      const before = session.getState()
      expect(session.executor.executeCommands([{ type: 'subtitle.updateCue', params: { itemId: 'subtitle-1', cueId: 'cue-1', updates: { endSeconds: 0 } } }]).ok).toBe(false)
      expect(session.executor.executeCommands([{ type: 'media.update', params: { mediaId: 'media-1', updates: { id: 'other' } } }]).ok).toBe(false)
      expect(session.executor.executeCommands([{ type: 'text.update', params: { itemId: 'text-1', updates: {}, clearFields: ['text'] } }]).ok).toBe(false)
      expect(session.getState()).toEqual(before)
    } finally { session.dispose() }
  })

  it('imports VTT and creates its captions track in one reversible step', () => {
    const session = createNodeSession(fixture(), createSeededIdFactory('vtt'))
    try {
      const before = json(session.getState())
      expect(session.executor.executeCommands([{ type: 'subtitle.import', params: { trackId: 'new-captions', createTrack: true, fileName: 'sub.vtt', format: 'vtt', content: 'WEBVTT\n\n00:01.000 --> 00:02.000\n<b>Xin chào</b>' } }]).ok).toBe(true)
      expect(session.getState().tracks.some((track) => track.id === 'new-captions')).toBe(true)
      expect(session.getState().items.find((item) => item.id === 'vtt-1')).toMatchObject({ type: 'text', text: 'Xin chào', from: 30, durationInFrames: 30 })
      expect(session.executor.undo().ok).toBe(true)
      expect(json(session.getState())).toEqual(before)
    } finally { session.dispose() }
  })

  it('merges slider updates while retaining the first inverse and latest redo', () => {
    const session = createNodeSession(fixture())
    try {
      const before = json(session.getState())
      expect(session.executor.executeCommands([{ type: 'audio.setMasterGain', params: { db: 2 } }]).ok).toBe(true)
      expect(session.executor.executeCommands([{ type: 'audio.setMasterGain', params: { db: 4 } }]).ok).toBe(true)
      expect(session.executor.undo().ok).toBe(true)
      expect(json(session.getState())).toEqual(before)
      expect(session.executor.canUndo).toBe(false)
      expect(session.executor.redo().ok).toBe(true)
      expect(session.getState().masterBusDb).toBe(4)
    } finally { session.dispose() }
  })

  it('removes media dependents and animations from nested and parked timelines', () => {
    const initial = fixture()
    const linked = makeTimelineVideoItem({ id: 'nested-video' })
    const animation = { itemId: linked.id, properties: [{ property: 'opacity' as const, keyframes: [{ id: 'key-1', frame: 0, value: 0.5, easing: 'linear' as const }] }] }
    initial.compositions = [{ id: 'nested', name: 'Nested', items: [linked], tracks: initial.tracks, transitions: [], keyframes: [animation], fps: 30, width: 1920, height: 1080, durationInFrames: 60 }]
    initial.stashStack = [{ compositionId: null, items: [linked], tracks: initial.tracks, transitions: [], keyframes: [animation], currentFrame: 0, markers: [], inPoint: null, outPoint: null }]
    initial.mainHolder = { ...initial.stashStack[0]! }
    initial.keyframes = [{ ...animation, itemId: 'video-1' }]
    initial.transitions = [{ id: 'transition-1', type: 'crossfade', presentation: 'fade', timing: 'linear', leftClipId: 'video-1', rightClipId: 'video-2', trackId: 'track-v1', durationInFrames: 10 }]
    const session = createNodeSession(initial)
    try {
      const before = json(captureProjectState())
      const result = session.executor.executeCommands([{ type: 'media.remove', params: { mediaIds: ['media-1'] } }])
      expect(result.ok, JSON.stringify(result)).toBe(true)
      const after = captureProjectState()
      expect(after.items.some((item) => item.mediaId === 'media-1')).toBe(false)
      expect(after.transitions).toEqual([])
      expect(after.keyframes).toEqual([])
      expect(after.compositions[0]?.items).toEqual([])
      expect(after.compositions[0]?.keyframes).toEqual([])
      expect(after.stashStack?.[0]?.items).toEqual([])
      expect(after.mainHolder?.items).toEqual([])
      expect(session.executor.undo().ok).toBe(true)
      expect(json(captureProjectState())).toEqual(before)
    } finally { session.dispose() }
  })

  it('keeps provisional imports out of canonical state while preserving their UI records', () => {
    const previous = useMediaLibraryStore.getState()
    const session = createNodeSession(fixture())
    try {
      const placeholder = { ...media, id: 'pending-import' }
      useMediaLibraryStore.setState((state) => ({ importingIds: [placeholder.id], mediaItems: [placeholder, ...state.mediaItems] }))
      expect(captureProjectState().mediaItems?.map((record) => record.id)).toEqual(['media-1'])
      expect(session.executor.executeCommands([{ type: 'media.add', params: { media: { ...media, id: 'media-2' } } }]).ok).toBe(true)
      expect(captureProjectState().mediaItems?.map((record) => record.id)).toEqual(['media-2', 'media-1'])
      expect(useMediaLibraryStore.getState().mediaItems.some((record) => record.id === placeholder.id)).toBe(true)
      expect(session.executor.executeCommands([{ type: 'media.setRecords', params: { mediaItems: [], resetImporting: true } }]).ok).toBe(true)
      expect(useMediaLibraryStore.getState().mediaItems).toEqual([])
      expect(useMediaLibraryStore.getState().importingIds).toEqual([])
    } finally {
      session.dispose()
      useMediaLibraryStore.setState({ importingIds: previous.importingIds, mediaItems: previous.mediaItems })
    }
  })

  it('does not merge edits of different GPU parameter names', () => {
    const session = createNodeSession(fixture())
    try {
      const command = (key: string, value: number): SerializedCommand => ({ type: 'effect.update', params: { itemId: 'video-1', effectId: 'fx-a', updates: { effect: { ...brightness, params: { [key]: value } } } } })
      expect(session.executor.executeCommands([command('brightness', 0.6)]).ok).toBe(true)
      const first = json(session.getState())
      expect(session.executor.executeCommands([command('contrast', 0.8)]).ok).toBe(true)
      expect(session.executor.undo().ok).toBe(true)
      expect(json(session.getState())).toEqual(first)
      expect(session.executor.canUndo).toBe(true)
    } finally { session.dispose() }
  })
})

describe('audio.setClip loop side effect', () => {
  it('unmuting the embedded audio clears the loop flag', () => {
    const session = createNodeSession(fixture())
    try {
      const looping = makeTimelineVideoItem({ id: 'video-1', embeddedAudioMuted: true, loop: true })
      useItemsStore.getState().setItems(useItemsStore.getState().items.map((item) => (item.id === 'video-1' ? looping : item)))
      const result = session.executor.executeCommands([{ type: 'audio.setClip', params: { itemId: 'video-1', updates: { embeddedAudioMuted: false } } }])
      expect(result.ok).toBe(true)
      const item = useItemsStore.getState().itemById['video-1'] as { embeddedAudioMuted?: boolean; loop?: boolean }
      expect(item.embeddedAudioMuted).toBe(false)
      expect(item.loop).toBeUndefined()
    } finally { session.dispose() }
  })
})
