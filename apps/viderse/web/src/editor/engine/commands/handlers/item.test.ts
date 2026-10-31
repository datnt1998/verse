// @vitest-environment node

import { describe, expect, it } from 'vitest'
import type { TimelineItem } from '@editor/types/timeline'
import type { ItemKeyframes } from '@editor/types/keyframe'
import type { Transition } from '@editor/types/transition'
import {
  makeTimelineAudioItem,
  makeTimelineTrack,
  makeTimelineVideoItem,
} from '../../features/timeline/test-helpers'
import { useCompositionNavigationStore } from '../../features/timeline/stores/composition-navigation-store'
import { captureProjectState, type ProjectState } from '../project-state'
import { createNodeSession, type NodeCommandSession } from '../node-session'
import { createSeededIdFactory } from '../id-factory'
import { deserializeCommands, serializeCommands } from '../serializer'
import type { ExecuteResult } from '../types'
import { registerItemCommands } from './item'
import { getPresetTrackHeight } from '@editor/features/timeline/utils/track-heights'
import { normalizeClassicTrackNames } from '@editor/features/timeline/utils/classic-tracks'
import { captureSnapshot, restoreSnapshot } from '@editor/test/freecut-snapshot'
import { updateItemTransform } from '@editor/features/timeline/stores/actions/transform-actions'

registerItemCommands()

const CANVAS = { width: 1920, height: 1080, fps: 30 }

const tracks = [
  makeTimelineTrack({ id: 'v1', name: 'V1', kind: 'video', order: 0 }),
  makeTimelineTrack({ id: 'v2', name: 'V2', kind: 'video', order: 1 }),
  makeTimelineTrack({ id: 'a1', name: 'A1', kind: 'audio', order: 2 }),
]

function video(id: string, from = 0, overrides: Partial<TimelineItem> = {}): TimelineItem {
  return makeTimelineVideoItem({
    id,
    trackId: 'v1',
    from,
    durationInFrames: 60,
    sourceStart: 30,
    sourceEnd: 90,
    sourceDuration: 300,
    ...overrides,
  } as never)
}

function audio(id: string, from = 0, overrides: Partial<TimelineItem> = {}): TimelineItem {
  return makeTimelineAudioItem({ id, trackId: 'a1', from, ...overrides } as never)
}

function transition(id: string, leftClipId: string, rightClipId: string): Transition {
  return {
    id,
    type: 'crossfade',
    presentation: 'fade',
    timing: 'linear',
    leftClipId,
    rightClipId,
    trackId: 'v1',
    durationInFrames: 10,
  } as Transition
}

function session(overrides: Partial<ProjectState> = {}): NodeCommandSession {
  return createNodeSession(
    {
      ...captureProjectState(),
      items: [],
      tracks,
      transitions: [],
      keyframes: [],
      markers: [],
      compositions: [],
      topLevelSequenceIds: [],
      ...overrides,
    },
    createSeededIdFactory('t'),
  )
}

function run(current: NodeCommandSession, type: string, params: unknown): ExecuteResult {
  return current.executor.executeCommands([{ type, params }])
}

function ok(current: NodeCommandSession, type: string, params: unknown): unknown {
  const outcome = run(current, type, params)
  if (!outcome.ok) throw new Error(JSON.stringify(outcome.issues))
  return outcome.results[0]
}

function byId(items: TimelineItem[], id: string): TimelineItem {
  const found = items.find((item) => item.id === id)
  if (!found) throw new Error(`missing item ${id}`)
  return found
}

describe('item add commands', () => {
  it('places collision-safe, mints ids, undoes/redoes and replays from serialized commands', () => {
    const current = session({ items: [video('existing', 0)] })
    let wire = ''
    let afterItems: TimelineItem[] = []
    try {
      const before = current.getState()
      const outcome = run(current, 'item.add', {
        item: { type: 'text', trackId: 'v1', from: 30, durationInFrames: 30, label: 'Tiêu đề', text: 'Xin chào', color: '#fff' },
      })
      expect(outcome.ok).toBe(true)
      if (!outcome.ok) return
      const placed = outcome.results[0] as TimelineItem
      expect(placed.id).toBe('t-1')
      expect(placed.from).toBe(60)
      afterItems = current.getState().items
      expect(afterItems.map((item) => item.id)).toEqual(['existing', 't-1'])

      current.executor.undo()
      expect(current.getState().items).toEqual(before.items)
      current.executor.redo()
      expect(current.getState().items).toEqual(afterItems)
      wire = serializeCommands(outcome.commands)
    } finally {
      current.dispose()
    }

    const replay = session({ items: [video('existing', 0)] })
    try {
      const replayed = replay.executor.executeCommands(deserializeCommands(wire))
      expect(replayed.ok).toBe(true)
      expect(replay.getState().items).toEqual(afterItems)
    } finally {
      replay.dispose()
    }
  })

  it('rejects unknown tracks and duplicate ids atomically', () => {
    const current = session({ items: [video('existing', 0)] })
    try {
      const before = current.getState()
      expect(run(current, 'item.add', { item: { type: 'text', trackId: 'nope', from: 0, durationInFrames: 10, label: 'x', text: 'x', color: '#fff' } }).ok).toBe(false)
      expect(run(current, 'item.addMany', { items: [{ id: 'existing', type: 'text', trackId: 'v1', from: 200, durationInFrames: 10, label: 'x', text: 'x', color: '#fff' }] }).ok).toBe(false)
      expect(run(current, 'item.add', { item: { type: 'text', trackId: 'v1', from: 0, durationInFrames: 0, label: 'x' } }).ok).toBe(false)
      expect(current.getState().items).toEqual(before.items)
      expect(current.executor.canUndo).toBe(false)
    } finally {
      current.dispose()
    }
  })

  it('adds a video with a linked audio companion on a new audio track in one step', () => {
    const current = session({ tracks: [tracks[0]!] })
    try {
      const result = ok(current, 'item.addWithLinkedAudio', {
        video: { id: 'clip', type: 'video', trackId: 'v1', from: 0, durationInFrames: 60, label: 'clip', src: 'blob:v', mediaId: 'm1' },
      }) as { video: TimelineItem; audio: TimelineItem; track: { id: string } | null }
      const state = current.getState()
      expect(result.track?.id).toMatch(/^track-t-/)
      expect(state.tracks.map((track) => track.id)).toContain(result.track!.id)
      expect(result.audio.type).toBe('audio')
      expect(result.audio.linkedGroupId).toBe(result.video.linkedGroupId)
      expect(state.items).toHaveLength(2)
      current.executor.undo()
      expect(current.getState().items).toEqual([])
      expect(current.getState().tracks).toHaveLength(1)
    } finally {
      current.dispose()
    }
  })

  it('adds a video with its linked audio onto an existing audio track and keeps both after commit', () => {
    const current = session()
    try {
      const result = ok(current, 'item.addWithLinkedAudio', {
        video: {
          id: 'clip', type: 'video', trackId: 'v1', from: 0, durationInFrames: 600, label: 'clip.mp4',
          src: 'blob:v', mediaId: 'm1', sourceWidth: 1920, sourceHeight: 1080, sourceFps: 60, sourceDuration: 600, sourceStart: 0,
        },
      }) as { track: unknown }
      const state = current.getState()
      expect(result.track).toBeNull()
      expect(state.tracks).toHaveLength(3)
      expect(state.items.map((item) => [item.id, item.trackId])).toEqual([
        ['clip', 'v1'],
        [expect.any(String), 'a1'],
      ])
      expect(state.items[1]!.linkedGroupId).toBe(state.items[0]!.linkedGroupId)
      current.executor.undo()
      expect(current.getState().items).toEqual([])
      current.executor.redo()
      expect(current.getState().items).toHaveLength(2)
    } finally {
      current.dispose()
    }
  })

  it('adds items together with planned new tracks atomically', () => {
    const current = session()
    try {
      const newTrack = { id: 'v3', name: 'V3', height: 80, locked: false, visible: true, muted: false, solo: false, order: 3, kind: 'video', items: [] }
      ok(current, 'item.addOnNewTracks', {
        items: [{ id: 'overlay', type: 'text', trackId: 'v3', from: 0, durationInFrames: 30, label: 'o', text: 'o', color: '#fff' }],
        tracks: [...tracks, newTrack],
      })
      expect(current.getState().tracks.map((track) => track.id)).toContain('v3')
      expect(byId(current.getState().items, 'overlay').trackId).toBe('v3')
      expect(run(current, 'item.addOnNewTracks', { items: [{ id: 'bad', type: 'text', trackId: 'missing', from: 0, durationInFrames: 5, label: 'b' }], tracks }).ok).toBe(false)
      current.executor.undo()
      expect(current.getState().tracks).toHaveLength(3)
      expect(current.getState().items).toEqual([])
    } finally {
      current.dispose()
    }
  })
})

describe('item update/move commands', () => {
  it('updates fields, clears via clearFields and merges repeated updates into one undo step', () => {
    const current = session({ items: [video('a', 0, { isReversed: true })] })
    try {
      ok(current, 'item.update', { id: 'a', updates: { label: 'Một' }, clearFields: ['isReversed'] })
      ok(current, 'item.update', { id: 'a', updates: { label: 'Hai' }, clearFields: ['isReversed'] })
      const item = byId(current.getState().items, 'a')
      expect(item.label).toBe('Hai')
      expect(item.isReversed).toBeUndefined()
      current.executor.undo()
      expect(current.executor.canUndo).toBe(false)
      expect(byId(current.getState().items, 'a')).toMatchObject({ label: 'clip.mp4', isReversed: true })
      expect(run(current, 'item.update', { id: 'missing', updates: { label: 'x' } }).ok).toBe(false)
      expect(run(current, 'item.update', { id: 'a', updates: { id: 'other' } }).ok).toBe(false)
    } finally {
      current.dispose()
    }
  })

  it('moves a transition pair onto another track and restores it on undo', () => {
    const current = session({
      items: [video('a', 0), video('b', 60)],
      transitions: [transition('tr1', 'a', 'b')],
    })
    try {
      const before = current.getState()
      ok(current, 'item.moveMany', {
        updates: [
          { id: 'a', from: 0, trackId: 'v2' },
          { id: 'b', from: 60, trackId: 'v2' },
        ],
      })
      const state = current.getState()
      expect(state.items.every((item) => item.trackId === 'v2')).toBe(true)
      expect(state.transitions.map((entry) => entry.trackId)).toEqual(['v2'])
      current.executor.undo()
      expect(current.getState().items).toEqual(before.items)
      expect(current.getState().transitions).toEqual(before.transitions)
      expect(run(current, 'item.moveMany', { updates: [{ id: 'a', from: 0, trackId: 'nope' }] }).ok).toBe(false)
    } finally {
      current.dispose()
    }
  })

  it('merges drag moves of one item into a single history entry', () => {
    const current = session({ items: [video('a', 0)] })
    try {
      ok(current, 'item.move', { id: 'a', from: 10 })
      ok(current, 'item.move', { id: 'a', from: 25 })
      expect(byId(current.getState().items, 'a').from).toBe(25)
      current.executor.undo()
      expect(byId(current.getState().items, 'a').from).toBe(0)
      expect(current.executor.canUndo).toBe(false)
    } finally {
      current.dispose()
    }
  })

  it('closes gaps and pushes downstream items across tracks', () => {
    const current = session({ items: [video('x', 0), video('y', 100), audio('z', 100)] })
    try {
      const before = current.getState()
      ok(current, 'item.closeGap', { trackId: 'v1', frame: 70 })
      expect(byId(current.getState().items, 'y').from).toBe(60)
      expect(byId(current.getState().items, 'z').from).toBe(60)
      current.executor.undo()
      expect(current.getState().items).toEqual(before.items)

      ok(current, 'item.trackPush', { anchorId: 'y', delta: 10 })
      expect(byId(current.getState().items, 'y').from).toBe(110)
      expect(byId(current.getState().items, 'z').from).toBe(110)
      expect(byId(current.getState().items, 'x').from).toBe(0)
      current.executor.undo()
      expect(current.getState().items).toEqual(before.items)

      ok(current, 'item.closeAllGaps', { trackId: 'v1', linkedSelection: false })
      expect(byId(current.getState().items, 'y').from).toBe(60)
    } finally {
      current.dispose()
    }
  })
})

describe('item duplicate command', () => {
  it('copies internal transitions, keyframes and remaps property links and parents', () => {
    const keyframes: ItemKeyframes[] = [
      {
        itemId: 'a',
        properties: [{ property: 'x', keyframes: [{ id: 'k1', frame: 0, value: 10, easing: 'linear' }] }],
        propertyLinks: [{ type: 'link', targetProperty: 'x', sourceItemId: 'b', sourceProperty: 'x', enabled: true, timeOffsetFrames: 0 }],
      },
    ]
    const current = session({
      items: [
        video('a', 0, { linkedGroupId: 'g' }),
        video('b', 60, { transformParent: { parentItemId: 'a', childLocalReference: {}, childWorldReference: {} } as never }),
      ],
      transitions: [transition('tr1', 'a', 'b')],
      keyframes,
    })
    try {
      const before = current.getState()
      const created = ok(current, 'item.duplicate', {
        itemIds: ['a', 'b'],
        positions: [
          { from: 200, trackId: 'v2' },
          { from: 260, trackId: 'v2' },
        ],
      }) as TimelineItem[]
      const [dupA, dupB] = created as [TimelineItem, TimelineItem]
      const state = current.getState()
      expect(state.items).toHaveLength(4)
      expect(dupB.transformParent?.parentItemId).toBe(dupA.id)
      expect(dupA.linkedGroupId).not.toBe('g')

      expect(state.transitions).toHaveLength(2)
      const copied = state.transitions.find((entry) => entry.id !== 'tr1')!
      expect(copied).toMatchObject({ leftClipId: dupA.id, rightClipId: dupB.id, trackId: 'v2' })
      expect(copied.id).toMatch(/^t-/)

      const copiedKeyframes = state.keyframes.find((entry) => entry.itemId === dupA.id)!
      expect(copiedKeyframes.properties[0]!.keyframes[0]!.id).toMatch(/^t-/)
      expect(copiedKeyframes.properties[0]!.keyframes[0]!.id).not.toBe('k1')
      expect(copiedKeyframes.propertyLinks?.[0]?.sourceItemId).toBe(dupB.id)
      expect(state.keyframes.find((entry) => entry.itemId === 'a')).toEqual(keyframes[0])

      current.executor.undo()
      expect(current.getState().items).toEqual(before.items)
      expect(current.getState().transitions).toEqual(before.transitions)
      expect(current.getState().keyframes).toEqual(before.keyframes)
      expect(run(current, 'item.duplicate', { itemIds: ['a'], positions: [] }).ok).toBe(false)
    } finally {
      current.dispose()
    }
  })
})

describe('item remove commands', () => {
  it('removes exactly the given items and cascades transitions, keyframes and groups', () => {
    const current = session({
      items: [video('a', 0, { linkedGroupId: 'g' }), video('b', 60), audio('aa', 0, { linkedGroupId: 'g' })],
      transitions: [transition('tr1', 'a', 'b')],
      keyframes: [{ itemId: 'a', properties: [] }],
    })
    try {
      const before = current.getState()
      expect(ok(current, 'item.removeExact', { itemIds: ['a'] })).toEqual(['a'])
      const state = current.getState()
      expect(state.items.map((item) => item.id)).toEqual(['b', 'aa'])
      expect(state.transitions).toEqual([])
      expect(state.keyframes).toEqual([])
      current.executor.undo()
      expect(current.getState().items).toEqual(before.items)
      expect(current.getState().transitions).toEqual(before.transitions)
      expect(current.getState().keyframes).toEqual(before.keyframes)
      expect(run(current, 'item.removeExact', { itemIds: ['ghost'] }).ok).toBe(false)
      expect(run(current, 'item.removeExact', { itemIds: [] }).ok).toBe(false)
    } finally {
      current.dispose()
    }
  })

  it('expands linked items only when linkedSelection is on', () => {
    const items = [video('a', 0, { linkedGroupId: 'g' }), audio('aa', 0, { linkedGroupId: 'g' })]
    const off = session({ items })
    try {
      ok(off, 'item.remove', { itemIds: ['a'], linkedSelection: false })
      expect(off.getState().items.map((item) => item.id)).toEqual(['aa'])
    } finally {
      off.dispose()
    }
    const on = session({ items })
    try {
      ok(on, 'item.remove', { itemIds: ['a'], linkedSelection: true })
      expect(on.getState().items).toEqual([])
    } finally {
      on.dispose()
    }
  })

  it('ripple deletes linked items, shifts downstream clips and undoes in one step', () => {
    const current = session({
      items: [
        video('a', 0, { linkedGroupId: 'g1' }),
        video('b', 60, { linkedGroupId: 'g2' }),
        audio('aa', 0, { linkedGroupId: 'g1' }),
        audio('bb', 60, { linkedGroupId: 'g2' }),
      ],
      transitions: [transition('tr1', 'a', 'b')],
    })
    try {
      const before = current.getState()
      const removed = ok(current, 'item.rippleDelete', { itemIds: ['a'], linkedSelection: true }) as string[]
      expect(removed).toEqual(expect.arrayContaining(['a', 'aa']))
      const state = current.getState()
      expect(state.items.map((item) => item.id).sort()).toEqual(['b', 'bb'])
      expect(byId(state.items, 'b').from).toBe(0)
      expect(byId(state.items, 'bb').from).toBe(0)
      expect(state.transitions).toEqual([])
      current.executor.undo()
      expect(current.getState().items).toEqual(before.items)
      expect(current.getState().transitions).toEqual(before.transitions)
      expect(current.executor.canUndo).toBe(false)
    } finally {
      current.dispose()
    }
  })
})

describe('item link/reverse commands', () => {
  it('unlinks with embedded audio muting and relinks with a minted group id', () => {
    const current = session({
      items: [video('v', 0, { linkedGroupId: 'g' }), audio('a', 0, { linkedGroupId: 'g' })],
    })
    try {
      const unlinked = ok(current, 'item.unlink', { itemIds: ['v'] }) as string[]
      expect(unlinked.sort()).toEqual(['a', 'v'])
      let items = current.getState().items
      expect(byId(items, 'v')).toMatchObject({ linkedGroupId: 'v', embeddedAudioMuted: true })
      expect(byId(items, 'a').linkedGroupId).toBe('a')

      const linked = ok(current, 'item.link', { itemIds: ['v', 'a'] }) as string[]
      expect(linked.sort()).toEqual(['a', 'v'])
      items = current.getState().items
      expect(byId(items, 'v').linkedGroupId).toBe('t-1')
      expect(byId(items, 'a').linkedGroupId).toBe('t-1')
      const video = byId(items, 'v')
      expect(video.type).toBe('video')
      expect(video.type === 'video' ? video.embeddedAudioMuted : undefined).toBeUndefined()
      expect(run(current, 'item.link', { itemIds: ['v', 'a'] }).ok).toBe(false)
      current.executor.undo()
      expect(byId(current.getState().items, 'v').linkedGroupId).toBe('v')
    } finally {
      current.dispose()
    }
  })

  it('toggles reverse and commits prepared reverse conform fields', () => {
    const current = session({ items: [audio('a', 0), video('v', 0)] })
    try {
      ok(current, 'item.reverse', { itemIds: ['a'] })
      expect(byId(current.getState().items, 'a').isReversed).toBe(true)
      ok(current, 'item.reverse', { itemIds: ['a'] })
      expect(byId(current.getState().items, 'a').isReversed).toBeUndefined()

      ok(current, 'item.commitPreparedReverse', {
        entries: [{ itemId: 'v', conform: { src: 'blob:r', path: 'p', key: 'k', quality: 'full', usesProxy: false, isSourceLevel: false } }],
      })
      expect(byId(current.getState().items, 'v')).toMatchObject({
        isReversed: true,
        reverseConformSrc: 'blob:r',
        reverseConformKey: 'k',
        reverseConformStatus: 'ready',
      })
      ok(current, 'item.reverse', { itemIds: ['v'] })
      const un = byId(current.getState().items, 'v')
      expect(un.isReversed).toBeUndefined()
      expect(un.reverseConformSrc).toBeUndefined()
      expect(un.reverseConformStatus).toBeUndefined()
      expect(run(current, 'item.reverse', { itemIds: ['ghost'] }).ok).toBe(false)
    } finally {
      current.dispose()
    }
  })
})

describe('item transform commands', () => {
  it('merges transform drags, resets, and writes auto keyframes in the same step', () => {
    const current = session({ items: [video('a', 0, { transform: {} } as never)] })
    try {
      ok(current, 'item.updateTransform', { id: 'a', transform: { x: 5 }, operation: 'move' })
      ok(current, 'item.updateTransform', { id: 'a', transform: { x: 9 }, operation: 'move' })
      expect(byId(current.getState().items, 'a').transform?.x).toBe(9)
      current.executor.undo()
      expect(byId(current.getState().items, 'a').transform?.x).toBeUndefined()
      expect(current.executor.canUndo).toBe(false)

      ok(current, 'item.updateTransform', {
        id: 'a',
        transform: { y: 4 },
        autoKeyframeOperations: [{ type: 'add', itemId: 'a', property: 'x', frame: 0, value: 3, easing: 'linear' }],
      })
      expect(current.getState().keyframes.find((entry) => entry.itemId === 'a')?.properties[0]?.keyframes[0]).toMatchObject({ frame: 0, value: 3 })
      current.executor.undo()
      expect(current.getState().keyframes).toEqual([])

      ok(current, 'item.updateTransforms', { ids: ['a'], transform: { opacity: 0.5 } })
      ok(current, 'item.resetTransform', { id: 'a' })
      expect(byId(current.getState().items, 'a').transform?.opacity).toBeUndefined()
      expect(run(current, 'item.updateTransform', { id: 'a', transform: { bogus: 1 } }).ok).toBe(false)
      expect(run(current, 'item.updateTransform', { id: 'ghost', transform: { x: 1 } }).ok).toBe(false)
    } finally {
      current.dispose()
    }
  })

  it('merges updates onto the resolved default transform for hydrated items without the key', () => {
    const bare = video('bare', 0, { sourceWidth: 1280, sourceHeight: 720, sourceDuration: 300, sourceStart: 30, sourceEnd: 90 })
    delete bare.transform
    const current = session({ items: [bare] })
    try {
      ok(current, 'item.updateTransform', { id: 'bare', transform: { x: 11 } })
      const written = byId(current.getState().items, 'bare').transform
      expect(written).toMatchObject({ x: 11, y: 0, rotation: 0, width: 1920, height: 1080, opacity: 1 })
      current.executor.undo()
      expect('transform' in byId(current.getState().items, 'bare')).toBe(false)
      ok(current, 'item.updateTransforms', { ids: ['bare'], transform: { rotation: 90 } })
      expect(byId(current.getState().items, 'bare').transform?.rotation).toBe(90)
      ok(current, 'item.updateTransformsMap', { entries: [{ id: 'bare', transform: { opacity: 0.2 } }] })
      expect(byId(current.getState().items, 'bare').transform?.opacity).toBe(0.2)
      ok(current, 'item.resetTransform', { id: 'bare' })
      expect(byId(current.getState().items, 'bare').transform).toMatchObject({ x: 0, y: 0, rotation: 0 })
    } finally {
      current.dispose()
    }
  })

  it('returns a command issue for unsupported audio transforms without recording history', () => {
    const current = session({ items: [audio('sound')] })
    try {
      const before = current.getState()
      const result = updateItemTransform('sound', { x: 5 })
      expect(result).toMatchObject({ ok: false, issues: [{ code: 'item.unsupportedTransform' }] })
      expect(run(current, 'item.resetTransform', { id: 'sound' })).toMatchObject({ ok: false, issues: [{ code: 'item.unsupportedTransform' }] })
      expect(run(current, 'item.updateTransforms', { ids: ['sound'], transform: { x: 5 } })).toMatchObject({ ok: false, issues: [{ code: 'item.unsupportedTransform' }] })
      expect(run(current, 'item.updateTransformsMap', { entries: [{ id: 'sound', transform: { x: 5 } }] })).toMatchObject({ ok: false, issues: [{ code: 'item.unsupportedTransform' }] })
      expect(current.getState()).toEqual(before)
      expect(current.executor.canUndo).toBe(false)
    } finally {
      current.dispose()
    }
  })

  it('applies bento layout and clears conflicting transform keyframes', () => {
    const current = session({
      items: [video('a', 0, { transform: {} } as never), video('b', 0, { trackId: 'v2', transform: {} } as never)],
      keyframes: [{ itemId: 'a', properties: [{ property: 'x', keyframes: [{ id: 'k', frame: 0, value: 1, easing: 'linear' }] }] }],
    })
    try {
      ok(current, 'item.applyBentoLayout', { itemIds: ['a', 'b'], canvasWidth: 1920, canvasHeight: 1080, config: { preset: 'row' } })
      const state = current.getState()
      expect(byId(state.items, 'a').transform?.width).toBeGreaterThan(0)
      expect(byId(state.items, 'b').transform?.x).not.toBe(byId(state.items, 'a').transform?.x)
      expect(state.keyframes.find((entry) => entry.itemId === 'a')?.properties.find((property) => property.property === 'x')).toBeUndefined()
      current.executor.undo()
      expect(current.getState().keyframes).toHaveLength(1)
      expect(run(current, 'item.applyBentoLayout', { itemIds: ['a'], canvasWidth: 1920, canvasHeight: 1080 }).ok).toBe(false)
    } finally {
      current.dispose()
    }
  })

  it('creates a null parent and reparents layers in one undo step each', () => {
    const current = session({ items: [video('a', 0), video('b', 0, { trackId: 'v2' }), audio('sound', 0)] })
    try {
      const before = current.getState()
      const parent = ok(current, 'item.createNullParent', { childItemIds: ['a'], frame: 0, canvas: CANVAS }) as TimelineItem
      expect(parent.type).toBe('controller')
      expect(parent.id).toMatch(/^t-/)
      let state = current.getState()
      expect(byId(state.items, 'a').transformParent?.parentItemId).toBe(parent.id)
      expect(state.tracks.some((track) => track.id === parent.trackId)).toBe(true)
      current.executor.undo()
      expect(current.getState().items).toEqual(before.items)
      expect(current.getState().tracks).toEqual(before.tracks)

      ok(current, 'item.createNullParent', { childItemIds: ['a'], frame: 0, canvas: CANVAS })
      const created = current.getState().items.find((item) => item.type === 'controller')!
      expect(ok(current, 'item.setTransformParents', { childItemIds: ['b'], parentItemId: created.id, frame: 0, canvas: CANVAS })).toBe(true)
      state = current.getState()
      expect(byId(state.items, 'b').transformParent?.parentItemId).toBe(created.id)
      expect(ok(current, 'item.setTransformParents', { childItemIds: ['b'], behavior: 'restore-local', frame: 0, canvas: CANVAS })).toBe(true)
      expect(byId(current.getState().items, 'b').transformParent).toBeUndefined()

      expect(run(current, 'item.setTransformParents', { childItemIds: ['b'], parentItemId: 'b', frame: 0, canvas: CANVAS }).ok).toBe(false)
      expect(run(current, 'item.createNullParent', { childItemIds: ['sound'], frame: 0, canvas: CANVAS }).ok).toBe(false)
    } finally {
      current.dispose()
    }
  })
})

describe('project-wide item commands', () => {
  function nested(label: string): TimelineItem {
    return video('nested', 0, { trackId: 'inner', label })
  }
  const innerTrack = makeTimelineTrack({ id: 'inner', name: 'Inner', kind: 'video', order: 0 })

  function seedNavigation(): void {
    const stash = {
      compositionId: 'c1',
      items: [nested('stash')],
      tracks: [innerTrack],
      transitions: [],
      keyframes: [{ itemId: 'nested', properties: [] }],
      currentFrame: 0,
      markers: [],
      inPoint: null,
      outPoint: null,
    }
    useCompositionNavigationStore.setState({ stashStack: [stash as never], mainHolder: { ...stash, compositionId: null, items: [nested('main')] } as never })
  }

  function composition() {
    return {
      id: 'c1',
      name: 'Phụ',
      items: [nested('comp')],
      tracks: [innerTrack],
      transitions: [],
      keyframes: [],
      fps: 30,
      width: 1920,
      height: 1080,
      durationInFrames: 60,
    }
  }

  it('updates one item across compositions, nav stash and mainHolder and undoes it', () => {
    const current = session({ compositions: [composition()] as never })
    try {
      seedNavigation()
      const before = captureProjectState()
      expect(ok(current, 'item.updateProjectItem', { itemId: 'nested', updates: { label: 'Mới' }, clearFields: ['mediaId'] })).toBe(true)
      const state = captureProjectState()
      expect(state.compositions[0]!.items[0]!.label).toBe('Mới')
      expect(state.compositions[0]!.items[0]!.mediaId).toBeUndefined()
      expect(state.stashStack![0]!.items[0]!.label).toBe('Mới')
      expect(state.mainHolder!.items[0]!.label).toBe('Mới')

      current.executor.undo()
      const restored = captureProjectState()
      expect(restored.compositions[0]!.items[0]!.label).toBe('comp')
      expect(restored.stashStack![0]!.items[0]!.label).toBe('stash')
      expect(restored.mainHolder!.items[0]!.label).toBe('main')
      expect(restored.compositions).toEqual(before.compositions)

      expect(ok(current, 'item.updateProjectItem', { itemId: 'ghost', updates: { label: 'x' } })).toBe(false)
      expect(current.executor.canUndo).toBe(false)
    } finally {
      current.dispose()
    }
  })

  it('removes items everywhere with their transitions and keyframes and undoes it', () => {
    const current = session({
      items: [video('keep', 0), video('nested', 60)],
      transitions: [transition('tr', 'keep', 'nested')],
      keyframes: [{ itemId: 'nested', properties: [] }],
      compositions: [composition()] as never,
    })
    try {
      seedNavigation()
      const before = captureProjectState()
      expect(ok(current, 'item.removeProjectItems', { itemIds: ['nested'] })).toBe(true)
      const state = captureProjectState()
      expect(state.items.map((item) => item.id)).toEqual(['keep'])
      expect(state.transitions).toEqual([])
      expect(state.keyframes).toEqual([])
      expect(state.compositions[0]!.items).toEqual([])
      expect(state.stashStack![0]!.items).toEqual([])
      expect(state.stashStack![0]!.keyframes).toEqual([])
      expect(state.mainHolder!.items).toEqual([])

      current.executor.undo()
      const restored = captureProjectState()
      expect(restored.items).toEqual(before.items)
      expect(restored.stashStack).toEqual(before.stashStack)
      expect(restored.mainHolder).toEqual(before.mainHolder)
      expect(restored.compositions).toEqual(before.compositions)

      expect(ok(current, 'item.removeProjectItems', { itemIds: ['ghost'] })).toBe(false)
    } finally {
      current.dispose()
    }
  })
})

describe('seeded item command reversibility', () => {
  const added = { id: 'added', type: 'text', trackId: 'v2', from: 200, durationInFrames: 30, label: 'Chữ', text: 'Chữ', color: '#fff' }
  const canonicalTracks = normalizeClassicTrackNames(tracks.map(track => ({ ...track, height: getPresetTrackHeight() })))
  const cases: Array<{ type: string; params: unknown }> = [
    { type: 'item.add', params: { item: added } },
    { type: 'item.addMany', params: { items: [added] } },
    { type: 'item.addWithLinkedAudio', params: { video: { ...video('added', 200), trackId: 'v2' } } },
    { type: 'item.addOnNewTracks', params: { items: [{ ...added, trackId: 'v3' }], tracks: [...canonicalTracks, { ...canonicalTracks[0], id: 'v3', name: 'V3', order: 3 }] } },
    { type: 'item.update', params: { id: 'a', updates: { label: 'Đổi tên' } } },
    { type: 'item.move', params: { id: 'a', from: 10 } },
    { type: 'item.moveMany', params: { updates: [{ id: 'a', from: 10, trackId: 'v2' }, { id: 'b', from: 130, trackId: 'v2' }] } },
    { type: 'item.trackPush', params: { anchorId: 'b', delta: 15 } },
    { type: 'item.duplicate', params: { itemIds: ['a'], positions: [{ from: 200, trackId: 'v2' }] } },
    { type: 'item.remove', params: { itemIds: ['a'], linkedSelection: false } },
    { type: 'item.removeExact', params: { itemIds: ['a'] } },
    { type: 'item.rippleDelete', params: { itemIds: ['a'], linkedSelection: false } },
    { type: 'item.closeGap', params: { trackId: 'v1', frame: 70 } },
    { type: 'item.closeAllGaps', params: { trackId: 'v1', linkedSelection: false } },
    { type: 'item.link', params: { itemIds: ['a', 'aa'] } },
    { type: 'item.unlink', params: { itemIds: ['a'] } },
    { type: 'item.reverse', params: { itemIds: ['a'] } },
    { type: 'item.commitPreparedReverse', params: { entries: [{ itemId: 'a', conform: { src: '/api/editor/media/reverse', path: 'reverse', key: 'key', quality: 'full', usesProxy: false, isSourceLevel: false } }] } },
    { type: 'item.setTransformParents', params: { childItemIds: ['a'], parentItemId: 'b', frame: 0, canvas: CANVAS } },
    { type: 'item.createNullParent', params: { childItemIds: ['a'], frame: 0, canvas: CANVAS } },
    { type: 'item.updateTransform', params: { id: 'a', transform: { x: 100 } } },
    { type: 'item.commitMaskEdit', params: { id: 'a', transform: { rotation: 30 } } },
    { type: 'item.resetTransform', params: { id: 'a' } },
    { type: 'item.updateTransforms', params: { ids: ['a', 'b'], transform: { rotation: 20 } } },
    { type: 'item.updateTransformsMap', params: { entries: [{ id: 'a', transform: { opacity: 0.3 } }, { id: 'b', transform: { opacity: 0.5 } }] } },
    { type: 'item.applyBentoLayout', params: { itemIds: ['a', 'b'], canvasWidth: 1920, canvasHeight: 1080, config: { preset: 'row' } } },
    { type: 'item.updateProjectItem', params: { itemId: 'a', updates: { label: 'Đổi tên' } } },
    { type: 'item.removeProjectItems', params: { itemIds: ['a'] } },
  ]
  const json = (value: unknown): unknown => JSON.parse(JSON.stringify(value))
  for (const scenario of cases) it(`${scenario.type} restores the Freecut snapshot and replays for seeded fixtures`, () => {
    for (const seed of [3, 29, 997]) {
      const x = ((seed * 1664525 + 1013904223) >>> 0) / 0x100000000 * 40
      const linked = scenario.type === 'item.unlink' ? 'av' : undefined
      const initial = { items: [video('a', 0, { transform: { x, width: 128, height: 72 }, linkedGroupId: linked }), video('b', 120, { transform: { x: -x, width: 128, height: 72 } }), audio('aa', 0, { linkedGroupId: linked })], tracks: canonicalTracks, stashStack: [], mainHolder: null }
      const current = session(initial)
      let after: ProjectState
      try {
        const before = captureProjectState()
        const snapshot = captureSnapshot()
        ok(current, scenario.type, scenario.params)
        after = captureProjectState()
        expect(json(after)).not.toEqual(json(before))
        expect(current.executor.undo()).toMatchObject({ ok: true })
        expect(json(captureProjectState())).toEqual(json(before))
        restoreSnapshot(snapshot)
        expect(json(captureProjectState())).toEqual(json(before))
        expect(current.executor.redo()).toMatchObject({ ok: true })
        expect(json(captureProjectState())).toEqual(json(after))
      } finally { current.dispose() }
      const replay = session(initial)
      try {
        expect(replay.executor.executeCommands(deserializeCommands(serializeCommands([scenario])))).toMatchObject({ ok: true })
        expect(json(captureProjectState())).toEqual(json(after!))
      } finally { replay.dispose() }
    }
  })
})

describe('video loop flag', () => {
  it('lets a muted clip loop and extend past its source; turning loop off trims back in one undo', () => {
    const current = session({ items: [video('a', 0, { embeddedAudioMuted: true })] })
    try {
      ok(current, 'item.update', { id: 'a', updates: { loop: true } })
      ok(current, 'item.update', { id: 'a', updates: { durationInFrames: 300 } })
      expect(byId(current.getState().items, 'a')).toMatchObject({ loop: true, durationInFrames: 300 })
      ok(current, 'item.update', { id: 'a', updates: { loop: false } })
      const item = byId(current.getState().items, 'a') as { loop?: boolean; durationInFrames: number }
      expect(item.loop).toBeUndefined()
      // Window is sourceStart..sourceEnd = 30..90 source frames = 60 timeline frames at 1×.
      expect(item.durationInFrames).toBe(60)
      current.executor.undo()
      expect(byId(current.getState().items, 'a')).toMatchObject({ loop: true, durationInFrames: 300 })
    } finally {
      current.dispose()
    }
  })

  it('rejects loop on non-video, unmuted or linked clips', () => {
    const current = session({ items: [video('a', 0), video('b', 60, { embeddedAudioMuted: true, linkedGroupId: 'g' }), audio('c', 60, { linkedGroupId: 'g' })] })
    try {
      expect(run(current, 'item.update', { id: 'a', updates: { loop: true } }).ok).toBe(false)
      expect(run(current, 'item.update', { id: 'b', updates: { loop: true } }).ok).toBe(false)
      expect(run(current, 'item.update', { id: 'c', updates: { loop: true } }).ok).toBe(false)
    } finally {
      current.dispose()
    }
  })

  it('unmuting a looping clip clears the loop flag', () => {
    const current = session({ items: [video('a', 0, { embeddedAudioMuted: true, loop: true })] })
    try {
      ok(current, 'item.update', { id: 'a', updates: { embeddedAudioMuted: false } })
      const item = byId(current.getState().items, 'a') as { embeddedAudioMuted?: boolean; loop?: boolean }
      expect(item.embeddedAudioMuted).toBe(false)
      expect(item.loop).toBeUndefined()
    } finally {
      current.dispose()
    }
  })

  it('addWithLinkedAudio rejects a looping video', () => {
    const current = session()
    try {
      const clip = video('new', 0, { embeddedAudioMuted: true, loop: true })
      expect(run(current, 'item.addWithLinkedAudio', { video: clip }).ok).toBe(false)
    } finally {
      current.dispose()
    }
  })
})
