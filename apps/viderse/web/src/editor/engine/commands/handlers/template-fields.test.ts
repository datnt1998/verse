// @vitest-environment node
import { describe, expect, it } from 'vitest'
import { captureProjectState, createNodeSession } from '@editor/commands'
import { makeTimelineTrack, makeTimelineVideoItem } from '@editor/features/timeline/test-helpers'
import type { SerializedCommand } from '@editor/commands/types'
import { DEFAULT_BACKGROUND_APPEARANCE } from '@editor/features/editing-templates/template-recipe'
import { TEMPLATE_LIMITS } from '@editor/features/editing-templates/template-limits'

describe('template slot and anchor commands', () => {
  it('enforces slot type and uniqueness, clears roles and preserves roles on ordinary updates', () => {
    const tracks = [makeTimelineTrack({ id: 'v1', name: 'Video', kind: 'video', order: 0 }), makeTimelineTrack({ id: 'v2', name: 'Video 2', kind: 'video', order: 1 }), makeTimelineTrack({ id: 'a1', name: 'Audio', kind: 'audio', order: 2 })]
    const session = createNodeSession({ ...captureProjectState(), tracks, items: [], compositions: [], stashStack: [], mainHolder: null })
    try {
      expect(session.executor.executeCommands([{ type: 'track.update', params: { id: 'a1', updates: { templateSlot: 'scenes' } } }]).ok).toBe(false)
      expect(session.executor.executeCommands([{ type: 'track.update', params: { id: 'v1', updates: { templateSlot: 'source-audio' } } }]).ok).toBe(false)
      expect(session.executor.executeCommands([{ type: 'track.update', params: { id: 'v1', updates: { templateSlot: 'scenes' } } }]).ok).toBe(true)
      expect(session.executor.executeCommands([{ type: 'track.update', params: { id: 'v2', updates: { templateSlot: 'scenes' } } }]).ok).toBe(false)
      expect(session.executor.executeCommands([{ type: 'track.update', params: { id: 'v1', updates: { name: 'Đổi tên' } } }]).ok).toBe(true)
      expect(session.getState().tracks[0]?.templateSlot).toBe('scenes')
      expect(session.executor.executeCommands([{ type: 'track.update', params: { id: 'v1', updates: { kind: 'audio' } } }]).ok).toBe(false)
      expect(session.executor.executeCommands([{ type: 'track.update', params: { id: 'v1', updates: { templateSlot: null } } }, { type: 'track.update', params: { id: 'v2', updates: { templateSlot: 'scenes' } } }]).ok).toBe(true)
      expect(session.executor.undo().ok).toBe(true)
      expect(session.getState().tracks[0]?.templateSlot).toBe('scenes')
    } finally { session.dispose() }
  })
  it('accepts start/end/full anchors but refuses invalid values and videos that cannot loop', () => {
    const track = makeTimelineTrack({ id: 'video', name: 'Video', kind: 'video', order: 0 })
    const item = makeTimelineVideoItem({ id: 'overlay', trackId: 'video', sourceStart: 0, sourceEnd: 60, embeddedAudioMuted: false })
    const session = createNodeSession({ ...captureProjectState(), tracks: [track], items: [item], compositions: [], stashStack: [], mainHolder: null })
    try {
      expect(session.executor.executeCommands([{ type: 'item.update', params: { id: item.id, updates: { templateAnchor: 'wrong' } } }]).ok).toBe(false)
      expect(session.executor.executeCommands([{ type: 'item.update', params: { id: item.id, updates: { templateAnchor: 'full' } } }]).ok).toBe(false)
      expect(session.executor.executeCommands([{ type: 'item.update', params: { id: item.id, updates: { templateAnchor: 'end' } } }]).ok).toBe(true)
      expect(session.executor.executeCommands([{ type: 'item.update', params: { id: item.id, updates: { templateAnchor: 'start' } } }]).ok).toBe(true)
      expect(session.executor.executeCommands([{ type: 'item.update', params: { id: item.id, updates: { embeddedAudioMuted: true, templateAnchor: 'full' } } }]).ok).toBe(true)
      expect(session.getState().items[0]?.templateAnchor).toBe('full')
      expect(session.executor.executeCommands([{ type: 'item.update', params: { id: item.id, updates: { embeddedAudioMuted: false } } }]).ok).toBe(false)
    } finally { session.dispose() }
  })
  it('accepts the background slot on video tracks only, once per template', () => {
    const tracks = [makeTimelineTrack({ id: 'v1', name: 'Video', kind: 'video', order: 0 }), makeTimelineTrack({ id: 'v2', name: 'Video 2', kind: 'video', order: 1 }), makeTimelineTrack({ id: 'a1', name: 'Audio', kind: 'audio', order: 2 })]
    const session = createNodeSession({ ...captureProjectState(), tracks, items: [], compositions: [], stashStack: [], mainHolder: null })
    try {
      expect(session.executor.executeCommands([{ type: 'track.update', params: { id: 'a1', updates: { templateSlot: 'background-video' } } }]).ok).toBe(false)
      expect(session.executor.executeCommands([{ type: 'track.update', params: { id: 'v1', updates: { templateSlot: 'background-video' } } }]).ok).toBe(true)
      expect(session.executor.executeCommands([{ type: 'track.update', params: { id: 'v2', updates: { templateSlot: 'background-video' } } }]).ok).toBe(false)
      expect(session.getState().tracks[0]?.templateSlot).toBe('background-video')
    } finally { session.dispose() }
  })
  it('sets and clears the scenes recipe, rejects it off the scenes slot and out of range', () => {
    const recipe = { style: 'cutout' as const, images: { mode: 'per-minutes' as const, minutes: 4 }, motion: { kind: 'wander' as const, intensity: 1.5, speed: 2 } }
    const tracks = [makeTimelineTrack({ id: 'v1', name: 'Video', kind: 'video', order: 0 }), makeTimelineTrack({ id: 'v2', name: 'Video 2', kind: 'video', order: 1 })]
    const session = createNodeSession({ ...captureProjectState(), tracks, items: [], compositions: [], stashStack: [], mainHolder: null })
    try {
      const exec = (updates: Record<string, unknown>, id = 'v1') => session.executor.executeCommands([{ type: 'track.update', params: { id, updates } }])
      expect(exec({ templateScenes: recipe }).ok).toBe(false)
      expect(exec({ templateSlot: 'scenes' }).ok).toBe(true)
      expect(exec({ templateScenes: recipe }).ok).toBe(true)
      expect(session.getState().tracks[0]?.templateScenes).toEqual(recipe)
      expect(exec({ templateScenes: { ...recipe, images: { mode: 'fixed', count: 1 } } }).ok).toBe(true)
      expect(exec({ templateScenes: { ...recipe, images: { mode: 'fixed', count: 0 } } }).ok).toBe(false)
      expect(exec({ templateScenes: { ...recipe, images: { mode: 'fixed', count: 61 } } }).ok).toBe(false)
      expect(exec({ templateScenes: { ...recipe, images: { mode: 'per-minutes', minutes: 60 } } }).ok).toBe(true)
      expect(exec({ templateScenes: { ...recipe, images: { mode: 'per-minutes', minutes: 61 } } }).ok).toBe(false)
      expect(exec({ templateScenes: { ...recipe, motion: { kind: 'wander', intensity: 0.1, speed: 1 } } }).ok).toBe(false)
      // Moving the role to another track drops the recipe with the slot.
      expect(exec({ templateSlot: null }).ok).toBe(true)
      expect(session.getState().tracks[0]?.templateScenes).toBeUndefined()
      expect(exec({ templateSlot: 'scenes' }, 'v2').ok).toBe(true)
      // Setting the recipe in the same update that assigns the slot works;
      // moving the role off v2 first keeps one holder per slot.
      expect(session.executor.executeCommands([{ type: 'track.update', params: { id: 'v2', updates: { templateSlot: null } } }, { type: 'track.update', params: { id: 'v1', updates: { templateSlot: 'scenes', templateScenes: recipe } } }]).ok).toBe(true)
      expect(session.getState().tracks[0]?.templateScenes).toEqual(recipe)
      expect(exec({ templateScenes: null }).ok).toBe(true)
      expect(session.getState().tracks[0]?.templateScenes).toBeUndefined()
      // Switching to another role clears the recipe too.
      expect(exec({ templateScenes: recipe }).ok).toBe(true)
      expect(exec({ templateSlot: 'captions' }).ok).toBe(true)
      expect(session.getState().tracks[0]?.templateScenes).toBeUndefined()
    } finally { session.dispose() }
  })
  it('rejects verbatim tracks carrying a recipe outside the scenes slot', () => {
    const session = createNodeSession({ ...captureProjectState(), tracks: [], items: [], compositions: [], stashStack: [], mainHolder: null })
    const base = { name: 'T', height: 80, locked: false, visible: true, muted: false, solo: false, order: 0, kind: 'video' }
    const recipe = { style: 'full', images: { mode: 'fixed', count: 5 }, motion: { kind: 'ken-burns', intensity: 1, speed: 1 } }
    try {
      expect(session.executor.executeCommands([{ type: 'track.set', params: { tracks: [{ ...base, id: 't1', templateScenes: recipe }] } }]).ok).toBe(false)
      expect(session.executor.executeCommands([{ type: 'track.set', params: { tracks: [{ ...base, id: 't1', templateSlot: 'scenes', templateScenes: recipe }] } }]).ok).toBe(true)
    } finally { session.dispose() }
  })
})

describe('template backdrop settings', () => {
  const backdrop = { channels: [{ channelId: 'twitch:xqc', kinds: ['clip', 'highlight'], pinned: [] }, { channelId: 'bilibili:12345', kinds: ['video'], pinned: ['BV1xx411c7mD'] }], clipCount: 10, clipSec: 60 }
  const tracks = () => [makeTimelineTrack({ id: 'v1', name: 'Video', kind: 'video', order: 0 }), makeTimelineTrack({ id: 'v2', name: 'Video 2', kind: 'video', order: 1 })]
  it('sets and clears the backdrop, rejects it off the background slot, duplicates and out-of-range values', () => {
    const session = createNodeSession({ ...captureProjectState(), tracks: tracks(), items: [], compositions: [], stashStack: [], mainHolder: null })
    try {
      const exec = (updates: Record<string, unknown>, id = 'v1') => session.executor.executeCommands([{ type: 'track.update', params: { id, updates } }])
      expect(exec({ templateBackdrop: backdrop }).ok).toBe(false)
      expect(exec({ templateSlot: 'scenes' }).ok).toBe(true)
      expect(exec({ templateBackdrop: backdrop }).ok).toBe(false)
      expect(exec({ templateSlot: 'background-video' }).ok).toBe(true)
      expect(exec({ templateBackdrop: backdrop }).ok).toBe(true)
      expect(session.getState().tracks[0]?.templateBackdrop).toEqual(backdrop)
      expect(exec({ templateBackdrop: { ...backdrop, channels: [backdrop.channels[0], backdrop.channels[0]] } }).ok).toBe(false)
      expect(exec({ templateBackdrop: { ...backdrop, channels: [] } }).ok).toBe(false)
      expect(exec({ templateBackdrop: { ...backdrop, channels: [{ ...backdrop.channels[0], channelId: 'youtube:abc' }] } }).ok).toBe(false)
      expect(exec({ templateBackdrop: { ...backdrop, channels: [{ ...backdrop.channels[0], kinds: [] }] } }).ok).toBe(false)
      expect(exec({ templateBackdrop: { ...backdrop, clipCount: 41 } }).ok).toBe(false)
      // No clip count means the job fills the video itself.
      expect(exec({ templateBackdrop: { channels: backdrop.channels, clipSec: 60 } }).ok).toBe(true)
      expect(session.getState().tracks[0]?.templateBackdrop).toEqual({ channels: backdrop.channels, clipSec: 60 })
      expect(exec({ templateBackdrop: { ...backdrop, clipCount: 0 } }).ok).toBe(false)
      expect(exec({ templateBackdrop: { ...backdrop, clipSec: 9 } }).ok).toBe(false)
      expect(exec({ templateBackdrop: { ...backdrop, clipSec: 600 } }).ok).toBe(true)
      expect(session.getState().tracks[0]?.templateBackdrop?.clipSec).toBe(600)
      // An unrelated update keeps the setting.
      expect(exec({ name: 'Nền' }).ok).toBe(true)
      expect(session.getState().tracks[0]?.templateBackdrop?.clipSec).toBe(600)
      expect(exec({ templateBackdrop: null }).ok).toBe(true)
      expect(session.getState().tracks[0]?.templateBackdrop).toBeUndefined()
      // Moving the role to another slot or track drops the backdrop with the slot.
      expect(exec({ templateBackdrop: backdrop }).ok).toBe(true)
      expect(exec({ templateSlot: 'captions' }).ok).toBe(true)
      expect(session.getState().tracks[0]?.templateBackdrop).toBeUndefined()
      expect(exec({ templateSlot: 'background-video', templateBackdrop: backdrop }).ok).toBe(true)
      expect(session.getState().tracks[0]?.templateBackdrop).toEqual(backdrop)
      expect(exec({ templateSlot: null }).ok).toBe(true)
      expect(session.getState().tracks[0]?.templateBackdrop).toBeUndefined()
    } finally { session.dispose() }
  })
  it('reports the issue codes for a misplaced and a duplicated backdrop', () => {
    const session = createNodeSession({ ...captureProjectState(), tracks: tracks(), items: [], compositions: [], stashStack: [], mainHolder: null })
    try {
      const misplaced = session.executor.executeCommands([{ type: 'track.update', params: { id: 'v1', updates: { templateBackdrop: backdrop } } }])
      expect(misplaced.ok).toBe(false)
      expect(JSON.stringify(misplaced)).toContain('track.backdropOnly')
      const duplicated = session.executor.executeCommands([{ type: 'track.update', params: { id: 'v1', updates: { templateSlot: 'background-video', templateBackdrop: { ...backdrop, channels: [backdrop.channels[0], backdrop.channels[0]] } } } }])
      expect(duplicated.ok).toBe(false)
      expect(JSON.stringify(duplicated)).toContain('track.backdropDuplicate')
    } finally { session.dispose() }
  })
  it('adds a background track carrying the backdrop and refuses it on other roles', () => {
    const session = createNodeSession({ ...captureProjectState(), tracks: [], items: [], compositions: [], stashStack: [], mainHolder: null })
    try {
      expect(session.executor.executeCommands([{ type: 'track.add', params: { id: 't1', kind: 'video', templateSlot: 'scenes', templateBackdrop: backdrop } }]).ok).toBe(false)
      expect(session.executor.executeCommands([{ type: 'track.add', params: { id: 't1', kind: 'video', templateBackdrop: backdrop } }]).ok).toBe(false)
      expect(session.executor.executeCommands([{ type: 'track.add', params: { id: 't1', kind: 'video', templateSlot: 'background-video', templateBackdrop: backdrop } }]).ok).toBe(true)
      expect(session.getState().tracks[0]?.templateBackdrop).toEqual(backdrop)
    } finally { session.dispose() }
  })
  it('rejects verbatim tracks carrying a backdrop outside the background slot', () => {
    const session = createNodeSession({ ...captureProjectState(), tracks: [], items: [], compositions: [], stashStack: [], mainHolder: null })
    const base = { name: 'T', height: 80, locked: false, visible: true, muted: false, solo: false, order: 0, kind: 'video' }
    try {
      expect(session.executor.executeCommands([{ type: 'track.set', params: { tracks: [{ ...base, id: 't1', templateBackdrop: backdrop }] } }]).ok).toBe(false)
      expect(session.executor.executeCommands([{ type: 'track.set', params: { tracks: [{ ...base, id: 't1', templateSlot: 'background-video', templateBackdrop: { ...backdrop, channels: [backdrop.channels[0], backdrop.channels[0]] } }] } }]).ok).toBe(false)
      expect(session.executor.executeCommands([{ type: 'track.set', params: { tracks: [{ ...base, id: 't1', templateSlot: 'background-video', templateBackdrop: backdrop }] } }]).ok).toBe(true)
      expect(session.getState().tracks[0]?.templateBackdrop).toEqual(backdrop)
    } finally { session.dispose() }
  })
})

describe('common background appearance', () => {
  const appearance = { ...DEFAULT_BACKGROUND_APPEARANCE, blurRadius: 12, speed: 1.5, flipX: true }
  const tracks = () => [
    makeTimelineTrack({ id: 'bg', name: 'Background', kind: 'video', order: 0, templateSlot: 'background-video' }),
    makeTimelineTrack({ id: 'other', name: 'Other', kind: 'video', order: 1 }),
  ]

  it('persists complete settings, clears on removal/reassignment and rejects off-role edits atomically', () => {
    const session = createNodeSession({ ...captureProjectState(), tracks: tracks(), items: [], compositions: [], stashStack: [], mainHolder: null })
    const update = (id: string, updates: Record<string, unknown>): SerializedCommand => ({ type: 'track.update', params: { id, updates } })
    try {
      expect(session.executor.executeCommands([update('bg', { templateBackgroundAppearance: appearance })]).ok).toBe(true)
      expect(session.executor.executeCommands([update('bg', { name: 'Styled background' })]).ok).toBe(true)
      expect(session.getState().tracks[0]?.templateBackgroundAppearance).toEqual(appearance)
      const before = session.getState()
      expect(session.executor.executeCommands([
        update('bg', { name: 'Must roll back' }),
        update('other', { templateBackgroundAppearance: appearance }),
      ]).ok).toBe(false)
      expect(session.getState()).toEqual(before)
      expect(session.executor.executeCommands([
        update('bg', { templateSlot: null }),
        update('other', { templateSlot: 'background-video', templateBackgroundAppearance: appearance }),
      ]).ok).toBe(true)
      expect(session.getState().tracks.find(track => track.id === 'bg')?.templateBackgroundAppearance).toBeUndefined()
      expect(session.getState().tracks.find(track => track.id === 'other')?.templateBackgroundAppearance).toEqual(appearance)
      expect(session.executor.executeCommands([update('other', { templateSlot: 'captions' })]).ok).toBe(true)
      expect(session.getState().tracks.find(track => track.id === 'other')?.templateBackgroundAppearance).toBeUndefined()
      expect(session.executor.executeCommands([update('other', { templateSlot: 'background-video', templateBackgroundAppearance: appearance })]).ok).toBe(true)
      expect(session.executor.executeCommands([update('other', { templateBackgroundAppearance: null })]).ok).toBe(true)
      expect(session.getState().tracks.find(track => track.id === 'other')?.templateBackgroundAppearance).toBeUndefined()
    } finally { session.dispose() }
  })

  it('enforces every numeric range, boolean type, completeness and strict keys through canonical updates', () => {
    const session = createNodeSession({ ...captureProjectState(), tracks: tracks(), items: [], compositions: [], stashStack: [], mainHolder: null })
    const set = (value: unknown) => session.executor.executeCommands([{ type: 'track.update', params: { id: 'bg', updates: { templateBackgroundAppearance: value } } }])
    try {
      for (const [field, range] of Object.entries(TEMPLATE_LIMITS.backgroundAppearance)) {
        for (const value of [range.min, range.max]) expect(set({ ...appearance, [field]: value }).ok).toBe(true)
        for (const value of [range.min - 0.01, range.max + 0.01, '1', null]) {
          const before = session.getState()
          expect(set({ ...appearance, [field]: value }).ok).toBe(false)
          expect(session.getState()).toEqual(before)
        }
      }
      for (const value of [{ ...appearance, flipX: 1 }, { ...appearance, flipY: 'false' }, { ...appearance, surprise: true }, { opacity: 1 }]) {
        expect(set(value).ok).toBe(false)
      }
    } finally { session.dispose() }
  })

  it('validates full tracks in every canonical ingestion path, including composition tracks', () => {
    const base = tracks()[0]!
    const item = makeTimelineVideoItem({ id: 'moving', trackId: 'bg', from: 0 })
    const routes = (track: unknown): SerializedCommand[] => [
      { type: 'track.set', params: { tracks: [track] } },
      { type: 'project.patch', params: { patch: { tracks: [track] } } },
      { type: 'project.patch', params: { patch: { compositions: [{
        id: 'nested', name: 'Nested', items: [], tracks: [track], transitions: [], keyframes: [],
        fps: 30, width: 1920, height: 1080, durationInFrames: 60,
      }] } } },
      { type: 'item.moveMany', params: { updates: [{ id: item.id, from: 10 }], tracks: [track] } },
      { type: 'item.duplicate', params: { itemIds: [item.id], positions: [{ from: 100, trackId: 'bg' }], tracks: [track] } },
      { type: 'item.addOnNewTracks', params: { items: [{ ...item, id: 'new' }], tracks: [track] } },
    ]
    for (const [index, valid] of routes({ ...base, templateBackgroundAppearance: appearance }).entries()) {
      const session = createNodeSession({ ...captureProjectState(), tracks: tracks(), items: [item], compositions: [], transitions: [], keyframes: [], stashStack: [], mainHolder: null })
      try {
        for (const invalid of [
          { ...base, templateSlot: undefined, templateBackgroundAppearance: appearance },
          { ...base, kind: 'audio', templateBackgroundAppearance: appearance },
          { ...base, templateBackgroundAppearance: { ...appearance, speed: 0.1 } },
          { ...base, templateBackgroundAppearance: { ...appearance, extra: true } },
        ]) {
          const before = session.getState()
          expect(session.executor.executeCommands([
            { type: 'track.update', params: { id: 'other', updates: { name: 'Must roll back' } } },
            routes(invalid)[index]!,
          ]).ok).toBe(false)
          expect(session.getState()).toEqual(before)
        }
        expect(session.executor.executeCommands([valid]).ok).toBe(true)
        const state = session.getState()
        const stored = index === 2 ? state.compositions[0]?.tracks[0] : state.tracks.find(track => track.id === 'bg')
        expect(stored?.templateBackgroundAppearance).toEqual(appearance)
      } finally { session.dispose() }
    }
  })

  it('validates appearance when creating tracks and retains old serialization when absent', () => {
    const session = createNodeSession({ ...captureProjectState(), tracks: [], items: [], compositions: [], stashStack: [], mainHolder: null })
    try {
      expect(session.executor.executeCommands([{ type: 'track.add', params: { id: 'wrong', templateBackgroundAppearance: appearance } }]).ok).toBe(false)
      expect(session.executor.executeCommands([{ type: 'track.add', params: { id: 'bg', templateSlot: 'background-video', templateBackgroundAppearance: appearance } }]).ok).toBe(true)
      expect(session.getState().tracks[0]?.templateBackgroundAppearance).toEqual(appearance)
      expect(session.executor.executeCommands([{ type: 'track.add', params: { id: 'plain' } }]).ok).toBe(true)
      const plain = session.getState().tracks.find(track => track.id === 'plain')
      expect(JSON.stringify(plain)).not.toContain('templateBackgroundAppearance')
    } finally { session.dispose() }
  })
})
