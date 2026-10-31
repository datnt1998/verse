// @vitest-environment node
import { createHash } from 'node:crypto'
import { z } from 'zod'
import { describe, expect, it } from 'vitest'
import { captureProjectState, createNodeSession } from '@editor/commands'
import { makeTimelineTrack } from '@editor/features/timeline/test-helpers'
import { DEFAULT_WAVEFORM_STYLE, type WaveformEnvelope } from '@editor/features/waveform-visualizer/types'
import type { MediaMetadata } from '@editor/types/storage'
import type { SerializedCommand } from '@editor/commands/types'
import type { TemplateBackgroundAppearance, TimelineItem, VideoItem } from '@editor/types/timeline'
import type { TemplateProjectShape } from './channel-template'
import { buildTemplateFill, prepareCaptionData } from './template-fill'
import { DEFAULT_BACKGROUND_APPEARANCE } from './template-recipe'
import { canAddTransition } from '@editor/features/timeline/utils/transition-utils'

const OLD_HASH = `sha256:${'a'.repeat(64)}`
const HASH = `sha256:${'b'.repeat(64)}`
const envelope: WaveformEnvelope = { sourceHash: HASH, samplesPerSecond: 50, durationSeconds: 6, envelopeBase64: Buffer.alloc(300, 12).toString('base64'), normalization: 0.5 }
function media(id: string, mimeType = 'image/webp', contentHash = HASH): MediaMetadata {
  return { id, storageType: 'workspace', contentHash, fileName: id, fileSize: 12, mimeType, duration: 6, width: 1920, height: 1080, fps: 30, codec: '', bitrate: 0, tags: [], createdAt: 1, updatedAt: 1 }
}
function template(): TemplateProjectShape {
  const tracks = [makeTimelineTrack({ id: 'scenes', name: 'Hình ảnh cảnh', kind: 'video', order: 0, templateSlot: 'scenes' }), makeTimelineTrack({ id: 'captions', name: 'Phụ đề', kind: 'video', order: 1, templateSlot: 'captions' }), makeTimelineTrack({ id: 'audio', name: 'Âm thanh gốc', kind: 'audio', order: 2, templateSlot: 'source-audio' }), makeTimelineTrack({ id: 'wave', name: 'Sóng giọng', kind: 'video', order: 3 }), makeTimelineTrack({ id: 'overlay', name: 'Lớp phủ', kind: 'video', order: 4 }), makeTimelineTrack({ id: 'end', name: 'Cuối', kind: 'video', order: 5 }), makeTimelineTrack({ id: 'start', name: 'Đầu', kind: 'video', order: 6 })]
  const items: TimelineItem[] = [
    { id: 'scene-prototype', type: 'image', trackId: 'scenes', label: 'Mẫu', from: 0, durationInFrames: 300, src: '/old', mediaId: 'old-image', transform: { x: 12, y: 24, width: 1280, height: 720, opacity: 0.7 }, blendMode: 'screen' },
    { id: 'captions-prototype', type: 'subtitle', trackId: 'captions', label: 'Phụ đề', from: 0, durationInFrames: 300, color: '#ff00ff', fontSize: 72, fontFamily: 'Arial', backgroundColor: '#112233', captionPreset: 'pop', transform: { x: 2, y: 360, width: 900, height: 140 }, source: { type: 'subtitle-import', fileName: 'captions.json', format: 'vtt', importedAt: 0 }, cues: [] },
    { id: 'audio-prototype', type: 'audio', trackId: 'audio', label: 'Nguồn', from: 0, durationInFrames: 300, src: '/old-audio', mediaId: 'old-audio', volume: -3, audioEqLowGainDb: 2 },
    { id: 'waveform', type: 'shape', shapeType: 'rectangle', trackId: 'wave', label: 'Sóng', from: 30, durationInFrames: 270, mediaId: 'old-audio', fillColor: '#ffffff', waveformVisualizer: { version: 1, ...DEFAULT_WAVEFORM_STYLE, ...envelope, sourceHash: OLD_HASH } },
    { id: 'snow', type: 'video', trackId: 'overlay', label: 'Tuyết', from: 0, durationInFrames: 300, src: '/snow', mediaId: 'snow-media', sourceStart: 0, sourceEnd: 143 * 30, sourceDuration: 143 * 30, embeddedAudioMuted: true, templateAnchor: 'full' },
    { id: 'subscribe', type: 'text', trackId: 'end', label: 'Subscribe', from: 240, durationInFrames: 60, text: 'Subscribe', color: '#ffffff', templateAnchor: 'end', linkedGroupId: 'clone-subscribe-group' },
    { id: 'start-drop', type: 'text', trackId: 'start', label: 'Sau', from: 250, durationInFrames: 50, text: 'Sau', color: '#ffffff' },
  ]
  return { metadata: { width: 1920, height: 1080, fps: 30 }, tracks, items }
}
function fill(project: TemplateProjectShape, D = 180, seed = 7) {
  return buildTemplateFill({ template: project, idPrefix: 'filled', durationInFrames: D, seed, media: [media('scene-1'), media('scene-2'), media('new-audio', 'audio/mp4')], scenes: [{ sceneId: 'one', title: 'Một', startFrame: 0, endFrame: 90, mediaId: 'scene-1', src: '/one' }, { sceneId: 'two', title: 'Hai', startFrame: 90, endFrame: D, mediaId: 'scene-2', src: '/two' }], cues: [{ id: 'cue', startSec: 0, endSec: 2, text: 'Xin chào', words: [{ startMs: 0, endMs: 1000, text: 'Xin' }, { startMs: 1000, endMs: 2000, text: 'chào' }] }], audio: { mediaId: 'new-audio', src: '/new-audio', durationInFrames: 180 }, envelope })
}
function apply(project: TemplateProjectShape, D = 180) {
  const initial = { ...captureProjectState(), tracks: [...project.tracks], items: [...project.items], transitions: [], keyframes: [], compositions: [], markers: [], topLevelSequenceIds: [], mediaItems: [media('old-image'), media('old-audio', 'audio/mp4', OLD_HASH), media('snow-media', 'video/mp4')], fps: 30, projectMetadata: project.metadata, stashStack: [], mainHolder: null }
  const session = createNodeSession(initial)
  try {
    const result = session.executor.executeCommands(fill(project, D).commands)
    expect(result).toMatchObject({ ok: true })
    return session.getState()
  } finally { session.dispose() }
}

/** Volume of the audio item the fill adds, read through a validated shape. */
function audioVolume(commands: SerializedCommand[]): number | undefined {
  const shape = z.object({ item: z.object({ type: z.literal('audio'), volume: z.number().optional() }) })
  for (const command of commands) {
    const parsed = shape.safeParse(command.params)
    if (command.type === 'item.add' && parsed.success) return parsed.data.item.volume
  }
  return undefined
}

describe('template fill', () => {
  it('applies through a node session with exact scenes, prototype style, pop captions, audio mix and rebound waveform', () => {
    const project = template()
    const before = structuredClone(project)
    const state = apply(project)
    expect(project).toEqual(before)
    const scenes = state.items.filter(item => item.type === 'image')
    expect(scenes.map(item => [item.from, item.durationInFrames, item.mediaId])).toEqual([[0, 90, 'scene-1'], [90, 90, 'scene-2']])
    expect(scenes[0]?.transform).toEqual(project.items[0]?.transform)
    expect(scenes[0]?.blendMode).toBe('screen')
    const captions = state.items.find(item => item.type === 'subtitle')
    expect(captions).toMatchObject({ from: 0, durationInFrames: 180, captionPreset: 'pop', color: '#ff00ff', fontSize: 72, backgroundColor: '#112233' })
    expect(state.items.find(item => item.type === 'audio')).toMatchObject({ mediaId: 'new-audio', from: 0, durationInFrames: 180, volume: -3, audioEqLowGainDb: 2 })
    expect(state.items.find(item => item.id === 'waveform')).toMatchObject({ mediaId: 'new-audio', from: 0, durationInFrames: 180, waveformVisualizer: { sourceHash: HASH, envelopeBase64: envelope.envelopeBase64 } })
    expect(state.keyframes.map(entry => entry.itemId)).toContain('filled-scene-one')
    expect(state.items.some(item => item.id === 'start-drop')).toBe(false)
    expect(state.items.find(item => item.id === 'subscribe')?.from).toBe(120)
    expect(state.items.reduce((end, item) => Math.max(end, item.from + item.durationInFrames), 0)).toBe(180)
  })
  it('fills locked template tracks while retaining their authored locks on the clone', () => {
    const project = template()
    const state = apply({ ...project, tracks: project.tracks.map(track => ({ ...track, locked: true })) })
    expect(state.tracks.every(track => track.locked)).toBe(true)
    expect(state.items.find(item => item.id === 'snow')).toMatchObject({ durationInFrames: 180, loop: true })
  })
  it('loops a 2:23 full video across two hours and preserves end distance', () => {
    const D = 2 * 60 * 60 * 30
    const state = apply(template(), D)
    expect(state.items.find(item => item.id === 'snow')).toMatchObject({ from: 0, durationInFrames: D, loop: true, sourceEnd: 143 * 30 })
    expect(state.items.find(item => item.id === 'subscribe')).toMatchObject({ from: D - 60, durationInFrames: 60 })
  })
  it('omits captions when the template has no captions role and defaults empty scene/audio slots', () => {
    const project = template()
    const modified = { ...project, tracks: project.tracks.filter(track => track.id !== 'captions'), items: project.items.filter(item => !['captions', 'scenes', 'audio', 'wave'].includes(item.trackId)) }
    const state = apply(modified)
    expect(state.items.some(item => item.type === 'subtitle')).toBe(false)
    expect(state.items.find(item => item.type === 'image')?.transform).toMatchObject({ width: 1920, height: 1080 })
    expect(state.items.find(item => item.type === 'audio')?.volume).toBe(0)
  })
  it('clamps start anchors, drops negative end anchors, and reports estimated cue timing explicitly', () => {
    const project = template()
    const clipped = { ...project, items: project.items.map(item => item.id === 'start-drop' ? { ...item, from: 150, durationInFrames: 150 } : item.id === 'subscribe' ? { ...item, from: 30 } : item) }
    const state = apply(clipped)
    expect(state.items.find(item => item.id === 'start-drop')).toMatchObject({ from: 150, durationInFrames: 30 })
    expect(state.items.some(item => item.id === 'subscribe')).toBe(false)
    expect(prepareCaptionData([{ id: 'c', startSec: 0, endSec: 2, text: 'Chưa có mốc', words: null }], 30, 180).estimated).toBe(true)
  })
  it('repeats scene transition style at each boundary and clamps short scenes', () => {
    const project = template()
    project.items = [...project.items, { ...project.items[0]!, id: 'scene-prototype-two', from: 300 }]
    project.transitions = [{ id: 'prototype-fade', trackId: 'scenes', leftClipId: 'scene-prototype', rightClipId: 'scene-prototype-two', type: 'crossfade', presentation: 'fade', timing: 'ease-in-out', durationInFrames: 15, alignment: 0.5 }]
    const base = { template: project, idPrefix: 'three', durationInFrames: 90, seed: 3, media: [], cues: [], audio: { mediaId: 'new-audio', src: '/new', durationInFrames: 90 }, envelope }
    const scenes = [0, 1, 2].map(index => ({ sceneId: String(index), title: '', startFrame: index * 30, endFrame: (index + 1) * 30, mediaId: 'scene-1', src: '/one' }))
    const commands = buildTemplateFill({ ...base, scenes }).commands.filter(command => command.type === 'transition.add')
    expect(commands).toHaveLength(2)
    expect(commands[0]?.params).toMatchObject({ leftClipId: 'three-scene-0', rightClipId: 'three-scene-1', presentation: 'fade', timing: 'ease-in-out', alignment: 0.5, durationInFrames: 12 })
    expect(buildTemplateFill({ ...base, scenes: scenes.slice(0, 1) }).commands.filter(command => command.type === 'transition.add')).toHaveLength(0)
    expect(buildTemplateFill({ ...base, template: { ...project, transitions: [] }, scenes }).commands.filter(command => command.type === 'transition.add')).toHaveLength(0)
  })
  it('adds clamped loudness gain to the prototype mix only with a measured target', () => {
    const project = template()
    project.tracks = project.tracks.map(track => track.templateSlot === 'source-audio' ? { ...track, templateLoudnessLufs: -14 } : track)
    const base = { template: project, idPrefix: 'gain', durationInFrames: 180, seed: 11, media: [], scenes: [], cues: [], audio: { mediaId: 'new-audio', src: '/new', durationInFrames: 180 }, envelope }
    for (const [measuredLufs, appliedGainDb] of [[-20, 6], [-5, -9], [-40, 12], [5, -12]]) {
      const result = buildTemplateFill({ ...base, audio: { ...base.audio, measuredLufs } })
      expect(result.loudness).toEqual({ targetLufs: -14, appliedGainDb })
      expect(audioVolume(result.commands)).toBe(-3 + appliedGainDb)
    }
    expect(buildTemplateFill(base).loudness).toBeNull()
    const noTarget = { ...base, template: template(), audio: { ...base.audio, measuredLufs: -20 } }
    expect(audioVolume(buildTemplateFill(noTarget).commands)).toBe(-3)
  })

  it('multiplies the prototype opacity into scene items only, and leaves everything identical when unset', () => {
    const project = template()
    const opaque = fill(project, 180)
    const withScenes = (opacity?: number): TemplateProjectShape => ({
      ...project,
      tracks: project.tracks.map(track => track.templateSlot === 'scenes'
        ? { ...track, templateScenes: { style: 'full' as const, images: { mode: 'fixed' as const, count: 2 }, motion: { kind: 'ken-burns' as const, intensity: 1, speed: 1 }, ...(opacity !== undefined ? { opacity } : {}) } }
        : track),
    })

    // A stored recipe without the field fills exactly like a template that never had one.
    expect(fill(withScenes(), 180).commands).toEqual(opaque.commands)
    // An explicit 1 is the same identity, so a saved-but-untouched opacity never perturbs the fill.
    expect(fill(withScenes(1), 180).commands).toEqual(opaque.commands)

    const dimmed = fill(withScenes(0.6), 180)
    const sceneItems = addedItems(dimmed.commands, 'scenes')
    expect(sceneItems).toHaveLength(2)
    for (const item of sceneItems) {
      expect(item.transform?.opacity).toBeCloseTo(0.7 * 0.6, 10) // prototype 0.7 × layer 0.6
    }
    // Nothing else moved: captions, audio and anchored items come out exactly as without the setting.
    for (const trackId of ['captions', 'audio', 'wave', 'overlay', 'end', 'start']) {
      expect(addedItems(dimmed.commands, trackId)).toEqual(addedItems(opaque.commands, trackId))
    }
    const opaqueNonAdd = opaque.commands.filter(command => command.type !== 'item.add')
    const dimmedNonAdd = dimmed.commands.filter(command => command.type !== 'item.add')
    expect(dimmedNonAdd).toEqual(opaqueNonAdd)
  })
})

/** Template extended with a `background-video` slot holding `pool` video clips. */
function templateWithBackground(pool: number, durationEach = 60): TemplateProjectShape {
  const project = template()
  project.tracks = [makeTimelineTrack({ id: 'bg', name: 'Nền video', kind: 'video', order: -1, templateSlot: 'background-video' }), ...project.tracks]
  project.items = [
    ...Array.from({ length: pool }, (_, index) => ({
      id: `bg-${index}`, type: 'video' as const, trackId: 'bg', label: `Nền ${index}`, from: index * durationEach,
      durationInFrames: durationEach, src: `/bg-${index}.mp4`, mediaId: `bg-media-${index}`,
      sourceStart: 10 + index, sourceEnd: 10 + index + durationEach, sourceDuration: 4000, sourceFps: 30,
      transform: { x: 0, y: 0, width: 1920, height: 1080 },
    }) as TimelineItem),
    ...project.items,
  ]
  return project
}

/** `item.add` payloads landing on `trackId`, in command order. */
function addedItems(commands: SerializedCommand[], trackId: string): TimelineItem[] {
  const items: TimelineItem[] = []
  for (const command of commands) {
    if (command.type !== 'item.add') continue
    // `item.add` params serialize a full TimelineItem; the command schema owns the validation.
    const params = command.params as { item: TimelineItem }
    if (params.item.trackId === trackId) items.push(params.item)
  }
  return items
}

describe('template fill: background-video slot', () => {
  it('tiles the shuffled pool end to end for pools of 1, 2 and 5 clips over 30 s to 2 h', () => {
    for (const pool of [1, 2, 5]) {
      for (const D of [900, 30 * 60 * 30, 2 * 60 * 60 * 30]) {
        const items = addedItems(fill(templateWithBackground(pool), D).commands, 'bg')
        expect(items.length).toBeGreaterThan(0)
        expect(items[0]?.from).toBe(0)
        let end = 0
        for (const [index, item] of items.entries()) {
          expect(item.from).toBe(end)
          expect(item.id).toBe(`filled-bg-${index}`)
          expect(item.type === 'video' && item.embeddedAudioMuted).toBe(true)
          end = item.from + item.durationInFrames
        }
        expect(end).toBe(D)
        const mediaIds = items.map((item) => item.mediaId)
        if (pool > 1) {
          for (let index = 1; index < mediaIds.length; index++) {
            expect(mediaIds[index]).not.toBe(mediaIds[index - 1])
          }
        }
      }
    }
  })

  it('keeps each clip’s own source window, starting late only as far as the join crossfade needs', () => {
    const commands = fill(templateWithBackground(3), 200).commands
    const items = addedItems(commands, 'bg')
    // The first placement of each clip is a full play; the last placement may be cut short at D.
    const windows = new Map(items.toReversed().map((item) => [item.mediaId, item]))
    for (const index of [0, 1, 2]) {
      const placed = windows.get(`bg-media-${index}`)
      // A 30-frame crossfade takes 15 frames of head before the cut; the clips start at 10–12.
      expect(placed).toMatchObject({ src: `/bg-${index}.mp4`, sourceStart: 15, sourceEnd: 70 + index, sourceDuration: 4000 })
    }
    const removed = commands.find((command) => command.type === 'item.removeExact')
    // `item.removeExact` params carry the id list.
    const removal = removed?.params as { itemIds: string[] } | undefined
    for (const index of [0, 1, 2]) expect(removal?.itemIds).toContain(`bg-${index}`)
    const updatedIds = commands.flatMap((command) => {
      if (command.type !== 'item.update') return []
      // `item.update` params carry the target id.
      const params = command.params as { id: string }
      return [params.id]
    })
    expect(updatedIds.some((id) => id.startsWith('bg-'))).toBe(false)
  })

  it('is byte-identical per seed and changes the order across seeds', () => {
    const project = templateWithBackground(5, 30)
    expect(JSON.stringify(fill(project, 5400, 42).commands)).toBe(JSON.stringify(fill(templateWithBackground(5, 30), 5400, 42).commands))
    const orders = new Set([41, 42, 43, 44, 45].map((seed) => addedItems(fill(templateWithBackground(5, 30), 5400, seed).commands, 'bg').map((item) => item.mediaId).join(',')))
    expect(orders.size).toBeGreaterThan(1)
  })

  it('reuses the template transition between background clips at every join, clamped to the shorter clip', () => {
    const project = templateWithBackground(2, 30)
    project.transitions = [{ id: 'bg-fade', trackId: 'bg', leftClipId: 'bg-0', rightClipId: 'bg-1', type: 'crossfade', presentation: 'wipe', timing: 'ease-out', durationInFrames: 12, alignment: 0.25, direction: 'from-left' }]
    const commands = fill(project, 140).commands.filter((command) => command.type === 'transition.add')
    // 140 frames over 30-frame clips: four joins; the last pair is 30+20, so it clamps to 8.
    expect(commands).toHaveLength(4)
    for (const [index, command] of commands.entries()) {
      expect(command.params).toMatchObject({
        leftClipId: `filled-bg-${index}`, rightClipId: `filled-bg-${index + 1}`,
        presentation: 'wipe', timing: 'ease-out', alignment: 0.25, direction: 'from-left',
      })
    }
    expect(commands[3]?.params).toMatchObject({ durationInFrames: 8 })
  })

  it('crossfades background joins for one second when the template has no background transition', () => {
    const commands = fill(templateWithBackground(2, 60), 140).commands.filter((command) => command.type === 'transition.add')
    // 140 frames over 55- and 56-frame clips leave a 29-frame tail clip, so the second join clamps to 11.
    expect(commands.map((command) => command.params)).toMatchObject([
      { presentation: 'fade', timing: 'ease-in-out', alignment: 0.5, durationInFrames: 22 },
      { durationInFrames: 11 },
    ])
  })

  it('lands every background crossfade even when the clips use their whole source', () => {
    const project = templateWithBackground(3, 60)
    project.items = project.items.map((item) => (item.trackId === 'bg' ? { ...item, sourceStart: 0, sourceEnd: 120, sourceDuration: 120, sourceFps: 60 } as TimelineItem : item))
    const state = apply(project, 400)
    const joins = state.items.filter((item) => item.trackId === 'bg').length - 1
    expect(joins).toBeGreaterThan(1)
    expect(state.transitions.filter((transition) => transition.trackId === 'bg')).toHaveLength(joins)
  })

  it('leaves the slot empty when the track holds no video clips', () => {
    const project = template()
    project.tracks = [makeTimelineTrack({ id: 'bg', name: 'Nền video', kind: 'video', order: -1, templateSlot: 'background-video' }), ...project.tracks]
    const commands = fill(project, 180).commands
    expect(addedItems(commands, 'bg')).toHaveLength(0)
  })

  it('matches the pre-recipe output byte for byte when the template has no recipe settings', () => {
    // sha256 of JSON.stringify(commands) captured from the fixture above before
    // the recipe fields existed; the seed must not perturb a slot-free template.
    const digest = createHash('sha256').update(JSON.stringify(fill(template()).commands)).digest('hex')
    expect(digest).toBe('552d8cd1624efe62623e2333ca48616a98db7d15269ba0de7c92d1466d5b51b8')
  })
})

describe('template fill: common background appearance', () => {
  function withAppearance(project: TemplateProjectShape, appearance: TemplateBackgroundAppearance): TemplateProjectShape {
    return { ...project, tracks: project.tracks.map(track => track.templateSlot === 'background-video' ? { ...track, templateBackgroundAppearance: appearance } : track) }
  }

  it('composes all local styles with common controls once for fixed and fetched-like pool clips only', () => {
    const project = templateWithBackground(2, 90)
    const localEffect = { id: 'local-effect', enabled: false, effect: { type: 'gpu-effect' as const, gpuEffectType: 'gpu-grain', params: { amount: 0.3 } } }
    project.items = project.items.map(item => item.trackId === 'bg' ? {
      ...item, label: item.id === 'bg-0' ? 'Fixed clip' : 'Fetched clip', effects: [localEffect],
      transform: { x: 20, y: -10, width: 1000, height: 600, opacity: 0.6, rotation: 7, flipHorizontal: true, flipVertical: false, anchorX: 100, anchorY: 120 },
    } : item)
    const appearance = { opacity: 0.5, blurRadius: 8, brightness: -0.2, contrast: 1.4, saturation: 0.7, hueShift: 0.25, speed: 1, zoom: 1.5, flipX: true, flipY: true }
    const styled = withAppearance(project, appearance)
    const before = structuredClone(styled)
    const result = fill(styled, 400)
    const baseline = fill(project, 400)
    expect(styled).toEqual(before)
    expect(fill(styled, 400).commands).toEqual(result.commands)
    const items = addedItems(result.commands, 'bg')
    expect(new Set(items.map(item => item.label))).toEqual(new Set(['Fixed clip', 'Fetched clip']))
    const generatedIds: string[] = []
    for (const item of items) {
      expect(item.transform).toEqual({ x: 20, y: -10, width: 1500, height: 900, opacity: 0.3, rotation: 7, flipHorizontal: false, flipVertical: true, anchorX: 150, anchorY: 180 })
      expect(item.effects?.[0]).toEqual(localEffect)
      expect(item.effects?.slice(1).map(effect => effect.effect)).toEqual([
        { type: 'gpu-effect', gpuEffectType: 'gpu-gaussian-blur', params: { radius: 8 } },
        { type: 'gpu-effect', gpuEffectType: 'gpu-brightness', params: { amount: -0.2 } },
        { type: 'gpu-effect', gpuEffectType: 'gpu-contrast', params: { amount: 1.4 } },
        { type: 'gpu-effect', gpuEffectType: 'gpu-saturation', params: { amount: 0.7 } },
        { type: 'gpu-effect', gpuEffectType: 'gpu-hue-shift', params: { shift: 0.25 } },
      ])
      expect(item.effects?.slice(1).every(effect => effect.enabled)).toBe(true)
      generatedIds.push(...(item.effects?.slice(1).map(effect => effect.id) ?? []))
    }
    expect(new Set(generatedIds).size).toBe(generatedIds.length)
    for (const track of ['scenes', 'captions', 'audio']) {
      expect(addedItems(result.commands, track)).toEqual(addedItems(baseline.commands, track))
    }
    expect(result.commands.filter(command => command.type === 'item.update')).toEqual(baseline.commands.filter(command => command.type === 'item.update'))
    const state = apply(styled, 400)
    expect(state.items.filter(item => item.trackId === 'bg').every(item => item.type === 'video' && item.embeddedAudioMuted)).toBe(true)
    expect(state.items.find(item => item.type === 'audio')).toMatchObject({ volume: -3, sourceStart: 0, sourceEnd: 180 })
    expect(state.items.find(item => item.type === 'audio')?.speed).toBeUndefined()
  })

  it('keeps identity byte-for-byte equivalent, including legacy local speeds and arbitrary local effects', () => {
    const project = templateWithBackground(2)
    project.items = project.items.map(item => item.trackId === 'bg' ? {
      ...item, speed: 16,
      effects: [{ id: 'legacy-effect', enabled: true, effect: { type: 'gpu-effect', gpuEffectType: 'gpu-grain', params: { amount: 0.2 } } }],
    } as VideoItem : item)
    expect(JSON.stringify(fill(withAppearance(project, { ...DEFAULT_BACKGROUND_APPEARANCE }), 400).commands)).toBe(JSON.stringify(fill(project, 400).commands))
    expect(() => fill(withAppearance(project, { ...DEFAULT_BACKGROUND_APPEARANCE, brightness: 0.1 }), 400)).not.toThrow()
  })

  it.each([0.25, 3])('retimes local rates before source windows and joins at global speed %s without gaps or overruns', speed => {
    const project = templateWithBackground(2, 120)
    project.items = project.items.map(item => item.trackId === 'bg' ? {
      ...item,
      speed: item.id === 'bg-0' ? 0.5 : 2,
      sourceFps: item.id === 'bg-0' ? 60 : 24,
      sourceStart: 0,
      sourceEnd: item.id === 'bg-0' ? 120 : 192,
      sourceDuration: item.id === 'bg-0' ? 120 : 192,
    } as VideoItem : item)
    const styled = withAppearance(project, { ...DEFAULT_BACKGROUND_APPEARANCE, speed })
    const before = structuredClone(styled)
    const result = fill(styled, 509)
    const items = addedItems(result.commands, 'bg') as VideoItem[]
    let cursor = 0
    for (const item of items) {
      expect(item.from).toBe(cursor)
      expect(item.durationInFrames).toBeGreaterThan(0)
      if (item.from + item.durationInFrames < 509) {
        expect(item.durationInFrames).toBeLessThanOrEqual(Math.floor(120 / speed))
        expect(item.durationInFrames).toBeGreaterThan(Math.floor(120 / speed) / 2)
      }
      expect(item.speed).toBe((item.mediaId === 'bg-media-0' ? 0.5 : 2) * speed)
      expect(item.sourceFps).toBe(item.mediaId === 'bg-media-0' ? 60 : 24)
      expect(item.sourceStart).toBeGreaterThanOrEqual(0)
      expect(item.sourceEnd).toBeLessThanOrEqual(item.sourceDuration!)
      const lastSample = item.sourceStart! + (item.durationInFrames - 1) * item.speed! * item.sourceFps! / 30
      expect(lastSample).toBeLessThan(item.sourceEnd!)
      cursor += item.durationInFrames
    }
    expect(cursor).toBe(509)
    const state = apply(styled, 509)
    const transitions = state.transitions.filter(transition => transition.trackId === 'bg')
    expect(transitions.length).toBeGreaterThan(0)
    expect(transitions).toHaveLength(result.commands.filter(command => command.type === 'transition.add').length)
    for (const transition of transitions) {
      const left = state.items.find(item => item.id === transition.leftClipId)!
      const right = state.items.find(item => item.id === transition.rightClipId)!
      expect(canAddTransition(left, right, transition.durationInFrames, transition.alignment, 30).canAdd).toBe(true)
    }
    expect(styled).toEqual(before)
  })

  it.each([0.25, 3])('preserves looping source windows and truncates only the final timeline tile at speed %s', speed => {
    const project = templateWithBackground(1, 120)
    project.items = project.items.map(item => item.trackId === 'bg' ? {
      ...item, loop: true, embeddedAudioMuted: true, speed: 0.5,
      sourceStart: 10, sourceEnd: 30, sourceDuration: 40, sourceFps: 60,
    } as VideoItem : item)
    const state = apply(withAppearance(project, { ...DEFAULT_BACKGROUND_APPEARANCE, speed }), 509)
    const items = state.items.filter((item): item is VideoItem => item.trackId === 'bg' && item.type === 'video')
    let cursor = 0
    for (const [index, item] of items.entries()) {
      expect(item).toMatchObject({ from: cursor, speed: 0.5 * speed, sourceStart: 10, sourceEnd: 30, sourceDuration: 40, sourceFps: 60, loop: true, embeddedAudioMuted: true })
      expect(item.durationInFrames).toBe(index === items.length - 1 ? 509 - cursor : Math.floor(120 / speed))
      cursor += item.durationInFrames
    }
    expect(cursor).toBe(509)
  })

  it('handles one-frame and unmeasured clips without zero-frame tiles or a trailing gap', () => {
    for (const durationInFrames of [1, 30]) {
      const project = templateWithBackground(1, durationInFrames)
      project.items = project.items.map(item => item.trackId === 'bg' ? {
        ...item, sourceStart: undefined, sourceEnd: undefined, sourceDuration: undefined,
      } : item)
      for (const speed of [0.25, 3]) {
        const items = addedItems(fill(withAppearance(project, { ...DEFAULT_BACKGROUND_APPEARANCE, speed }), 181).commands, 'bg')
        expect(items[0]?.from).toBe(0)
        expect(items.every(item => item.durationInFrames >= 1 && item.sourceEnd! > item.sourceStart!)).toBe(true)
        expect(items.at(-1)!.from + items.at(-1)!.durationInFrames).toBe(181)
      }
    }
  })

  it('rejects effective rates outside the persisted 0.1..10 range instead of clamping runtime-valid rates', () => {
    for (const [localSpeed, speed] of [[0.2, 0.25], [4, 3], [Number.NaN, 2]]) {
      const project = templateWithBackground(1)
      project.items = project.items.map(item => item.trackId === 'bg' ? { ...item, speed: localSpeed } : item)
      const before = structuredClone(project)
      expect(() => fill(withAppearance(project, { ...DEFAULT_BACKGROUND_APPEARANCE, speed }))).toThrow('0.1–10')
      expect(project).toEqual(before)
    }
  })

  it('accepts both persisted effective-rate boundaries without altering the input pool', () => {
    for (const [localSpeed, speed, effective] of [[0.4, 0.25, 0.1], [5, 2, 10]]) {
      const project = templateWithBackground(1)
      project.items = project.items.map(item => item.trackId === 'bg' ? { ...item, speed: localSpeed } : item)
      const styled = withAppearance(project, { ...DEFAULT_BACKGROUND_APPEARANCE, speed })
      const before = structuredClone(styled)
      const state = apply(styled)
      const background = state.items.filter(item => item.trackId === 'bg')
      expect(background.length).toBeGreaterThan(0)
      expect(background.every(item => item.speed === effective)).toBe(true)
      expect(styled).toEqual(before)
    }
  })

  it('uses a half-open source end that covers fractional slow-rate samples at the final truncation', () => {
    const project = templateWithBackground(1, 300)
    project.items = project.items.map(item => item.trackId === 'bg' ? {
      ...item, speed: 0.4, sourceFps: 24, sourceStart: 40, sourceEnd: 136, sourceDuration: 1000,
    } as VideoItem : item)
    const items = addedItems(fill(withAppearance(project, { ...DEFAULT_BACKGROUND_APPEARANCE, speed: 0.25 }), 514).commands, 'bg') as VideoItem[]
    const last = items.at(-1)!
    expect(last.speed).toBe(0.1)
    expect(last.sourceStart).toBe(40)
    expect(last.sourceEnd).toBe(82)
    expect(last.sourceStart! + (last.durationInFrames - 1) * last.speed! * last.sourceFps! / 30).toBeLessThan(last.sourceEnd!)
  })

  it('keeps generated effect IDs deterministic even when a local effect uses the generated name', () => {
    const project = templateWithBackground(1)
    project.items = project.items.map(item => item.trackId === 'bg' ? {
      ...item, effects: [{ id: 'filled-bg-0-background-brightness', enabled: true, effect: { type: 'gpu-effect', gpuEffectType: 'gpu-grain', params: { amount: 0.2 } } }],
    } as VideoItem : item)
    const items = addedItems(fill(withAppearance(project, { ...DEFAULT_BACKGROUND_APPEARANCE, brightness: 0.2 })).commands, 'bg')
    expect(items[0]?.effects?.map(effect => effect.id)).toEqual(['filled-bg-0-background-brightness', 'filled-bg-0-background-brightness-1'])
    for (const item of items) expect(new Set(item.effects?.map(effect => effect.id)).size).toBe(item.effects?.length)
  })

  it('zooms implicit source-aspect defaults and avoids redundant identity effects', () => {
    const project = templateWithBackground(1)
    project.items = project.items.map(item => item.trackId === 'bg' ? { ...item, transform: undefined, sourceWidth: 1000, sourceHeight: 1000 } as VideoItem : item)
    const items = addedItems(fill(withAppearance(project, { ...DEFAULT_BACKGROUND_APPEARANCE, zoom: 2 })).commands, 'bg')
    expect(items[0]?.transform).toEqual({ width: 2160, height: 2160 })
    expect(items.every(item => item.effects === undefined)).toBe(true)
  })
})

describe('template fill: subject motion', () => {
  const wanderTemplate = () => {
    const project = template()
    project.tracks = project.tracks.map((track) => track.templateSlot === 'scenes'
      ? { ...track, templateScenes: { style: 'full' as const, images: { mode: 'fixed' as const, count: 2 }, motion: { kind: 'wander' as const, intensity: 1.5, speed: 2 } } }
      : track)
    return project
  }

  it('attaches one seeded wander modifier per scene and no Ken Burns keyframes', () => {
    const result = fill(wanderTemplate())
    const scenes = addedItems(result.commands, 'scenes')
    expect(scenes).toHaveLength(2)
    for (const [index, item] of scenes.entries()) {
      expect(item.motionModifiers).toMatchObject([{
        version: 2, id: `filled-mm-${index === 0 ? 'one' : 'two'}`, type: 'wander',
        enabled: true, amplitude: 1.5, frequency: 0.125, phaseFrames: 0,
      }])
    }
    expect(scenes[0]?.motionModifiers?.[0]?.seed).not.toBe(scenes[1]?.motionModifiers?.[0]?.seed)
    expect(result.commands.some((command) => command.type === 'keyframe.add')).toBe(false)
  })

  it('varies the wander seed across job seeds', () => {
    const first = addedItems(fill(wanderTemplate(), 180, 42).commands, 'scenes')[0]
    const second = addedItems(fill(wanderTemplate(), 180, 43).commands, 'scenes')[0]
    expect(first?.motionModifiers?.[0]?.seed).not.toBe(second?.motionModifiers?.[0]?.seed)
  })

  it('keeps the alternating Ken Burns on recipes without wander', () => {
    const project = wanderTemplate()
    project.tracks = project.tracks.map((track) => track.templateScenes ? { ...track, templateScenes: { ...track.templateScenes, motion: { kind: 'ken-burns' as const, intensity: 1, speed: 1 } } } : track)
    const result = fill(project)
    const scenes = addedItems(result.commands, 'scenes')
    expect(scenes.every((item) => item.motionModifiers === undefined)).toBe(true)
    expect(result.commands.filter((command) => command.type === 'keyframe.add')).toHaveLength(8)
  })
})

describe('template fill: close-up cut-out', () => {
  type Box = { x: number; y: number; width: number; height: number }
  const closeUp = (boxes: (Box | undefined)[]) => {
    const project = template()
    project.tracks = project.tracks.map((track) => track.templateSlot === 'scenes'
      ? { ...track, templateScenes: { style: 'cutout-close' as const, images: { mode: 'fixed' as const, count: 3 }, motion: { kind: 'wander' as const, intensity: 1, speed: 1 } } }
      : track)
    project.items = project.items.map((item) => item.id === 'scene-prototype' ? { ...item, transform: { x: 0, y: 0, width: 1920, height: 1080 } } : item)
    const scenes = boxes.map((subjectBox, index) => ({ sceneId: String(index), title: '', startFrame: index * 60, endFrame: (index + 1) * 60, mediaId: 'scene-1', src: '/one', ...(subjectBox ? { subjectBox } : {}) }))
    return () => buildTemplateFill({ template: project, idPrefix: 'filled', durationInFrames: 180, seed: 7, media: [media('scene-1'), media('new-audio', 'audio/mp4')], scenes, cues: [], audio: { mediaId: 'new-audio', src: '/new-audio', durationInFrames: 180 }, envelope })
  }
  /** Where the subject box lands on the 1920×1080 canvas, from the item's centre-relative transform. */
  const onCanvas = (transform: { x?: number; y?: number; width?: number; height?: number } | undefined, box: Box) => {
    const width = transform?.width ?? 0, height = transform?.height ?? 0
    const left = 960 + (transform?.x ?? 0) - width / 2, top = 540 + (transform?.y ?? 0) - height / 2
    return { top: top + box.y * height, centreX: left + (box.x + box.width / 2) * width, width: box.width * width, height: box.height * height, scale: width / 1920 }
  }

  it('enlarges a standing figure until head to waist fills the frame, centred, with its head just under the top edge', () => {
    const box = { x: 0.25, y: 0.05, width: 0.3, height: 0.9 }
    const [scene] = addedItems(closeUp([box, box, box])().commands, 'scenes')
    const placed = onCanvas(scene?.transform, box)
    expect(placed.top).toBeCloseTo(0.04 * 1080, 6)
    expect(placed.centreX).toBeCloseTo(960, 6)
    expect(placed.height * 0.55).toBeCloseTo(0.95 * 1080, 6)
  })

  it('frames each scene on its own subject and keeps a wide group within a small overflow', () => {
    const left = { x: 0.05, y: 0.1, width: 0.2, height: 0.85 }
    const group = { x: 0.05, y: 0.1, width: 0.9, height: 0.8 }
    const tiny = { x: 0.45, y: 0.4, width: 0.05, height: 0.1 }
    const scenes = addedItems(closeUp([left, group, tiny])().commands, 'scenes')
    expect(onCanvas(scenes[0]?.transform, left).centreX).toBeCloseTo(960, 6)
    const wide = onCanvas(scenes[1]?.transform, group)
    expect(wide.width).toBeLessThanOrEqual(1.15 * 1920 + 1e-6)
    expect(wide.scale).toBeGreaterThanOrEqual(1.25)
    expect(onCanvas(scenes[2]?.transform, tiny).scale).toBeCloseTo(2.2, 6)
  })

  it('refuses a close-up scene without a measured subject box', () => {
    expect(closeUp([{ x: 0.2, y: 0.1, width: 0.3, height: 0.8 }, undefined, undefined])).toThrow(/khung nhân vật/)
  })
})

describe('prepareCaptionData', () => {
  it('maps real word offsets, splits multi-word pieces by character length and flags wordless cues as estimated', () => {
    const cues = [
      { id: 'c1', startSec: 0.5, endSec: 2.5, text: 'Xin chào các bạn', words: [{ startMs: 500, endMs: 1200, text: 'Xin' }, { startMs: 1200, endMs: 1900, text: 'chào' }, { startMs: 1900, endMs: 2500, text: 'các bạn' }] },
      { id: 'c2', startSec: 3, endSec: 5, text: 'Câu hai không có từ', words: null },
    ]
    const data = prepareCaptionData(cues, 30, 180)
    expect(data.cues).toHaveLength(2)
    expect(data.cues[0]).toMatchObject({ startSeconds: 0.5, endSeconds: 2.5 })
    const real = data.words.slice(0, 4)
    // 'các bạn' arrives as one shared-timing piece and splits by character length.
    expect(real.map((w) => w.text)).toEqual(['Xin', 'chào', 'các', 'bạn'])
    expect(real[0]).toMatchObject({ startFrame: 15, endFrame: 36 })
    expect(real[2]).toMatchObject({ text: 'các', startFrame: 57, endFrame: 66 })
    // 'Câu hai không có từ' = 5 tokens spread over frames 90→150.
    const estimated = data.words.slice(4)
    expect(estimated).toHaveLength(5)
    expect(estimated[0]).toMatchObject({ text: 'Câu', startFrame: 90 })
    expect(estimated.at(-1)).toMatchObject({ endFrame: 150 })
    expect(data.estimated).toBe(true)
  })

  it('marks unestimated when every cue carries measured words', () => {
    const data = prepareCaptionData([{ id: 'c1', startSec: 0, endSec: 2, text: 'một hai', words: [{ startMs: 0, endMs: 1000, text: 'một' }, { startMs: 1000, endMs: 2000, text: 'hai' }] }], 30, 180)
    expect(data.estimated).toBe(false)
    expect(data.words).toHaveLength(2)
  })
  it('bounds cues to the project duration and drops invalid ranges', () => {
    const data = prepareCaptionData([
      { id: 'late', startSec: 5, endSec: 12, text: 'quá dài', words: null },
      { id: 'bad', startSec: 4, endSec: 4, text: 'vô hiệu', words: null },
      { id: 'out', startSec: 8, endSec: 9, text: 'ngoài', words: null },
    ], 30, 180)
    expect(data.cues.map((c) => c.id)).toEqual(['late'])
    expect(data.cues[0]!.endSeconds).toBe(6)
  })
  it('clips overlapping cues and paginates long phrases into tiled one-page cues', () => {
    const text = 'một hai ba bốn năm sáu bảy tám chín mười'
    const words = text.split(' ').map((token, index) => ({ startMs: index * 300, endMs: index * 300 + 280, text: token }))
    const data = prepareCaptionData([
      { id: 'a', startSec: 0, endSec: 4, text, words },
      { id: 'b', startSec: 3, endSec: 5, text: 'đuôi', words: [{ startMs: 3000, endMs: 5000, text: 'đuôi' }] },
    ], 30, 240)
    // Cue A clips to cue B's start (3 s): 10 words → pages of 7 + 3 that tile [0, 90).
    expect(data.cues.map((cue) => cue.id)).toEqual(['a', 'a.2', 'b'])
    expect(data.cues.map((cue) => [cue.startSeconds, cue.endSeconds])).toEqual([[0, 2.1], [2.1, 3], [3, 5]])
    expect(data.cues[0]!.text).toBe('một hai ba bốn năm sáu bảy')
    expect(data.estimated).toBe(false)
  })
  it('keeps every word in reading order when aligned timing runs past the next cue', () => {
    // Real shape: the last word of cue A was aligned after cue B's first words.
    const data = prepareCaptionData([
      { id: 'a', startSec: 0, endSec: 1, text: "Once the preform's done,", words: [
        { startMs: 0, endMs: 300, text: 'Once' }, { startMs: 300, endMs: 600, text: 'the' },
        { startMs: 600, endMs: 900, text: "preform's" }, { startMs: 1200, endMs: 1400, text: 'done,' },
      ] },
      { id: 'b', startSec: 1, endSec: 1.6, text: 'This is absurd.', words: [
        { startMs: 1000, endMs: 1100, text: 'This', estimated: true }, { startMs: 1100, endMs: 1200, text: 'is' },
        { startMs: 1200, endMs: 1200, text: 'absurd.', estimated: true },
      ] },
    ], 30, 120)
    expect(data.pages.map((page) => page.text)).toEqual(["Once the preform's done,", 'This is absurd.'])
    const starts = data.words.map((word) => word.startFrame)
    expect(starts).toEqual([...starts].sort((x, y) => x - y))
    expect(new Set(starts).size).toBe(starts.length)
    expect(data.words.every((word) => word.endFrame > word.startFrame)).toBe(true)
  })
  it('never interleaves two cues whose words overlap in time', () => {
    const data = prepareCaptionData([
      { id: 'a', startSec: 0, endSec: 1, text: 'right there.', words: [{ startMs: 0, endMs: 500, text: 'right' }, { startMs: 500, endMs: 1000, text: 'there.' }] },
      { id: 'b', startSec: 0.2, endSec: 1, text: '(light music)', words: [{ startMs: 200, endMs: 600, text: '(light' }, { startMs: 600, endMs: 1000, text: 'music)' }] },
    ], 30, 90)
    expect(data.pages.map((page) => page.text).join(' ')).toBe('right there. (light music)')
  })
})
