import { describe, expect, it } from 'vitest'
import type { AudioItem, TextItem, TimelineItem, VideoItem } from '@editor/types/timeline'
import type { Transition } from '@editor/types/transition'
import { makeTimelineAudioItem, makeTimelineTrack, makeTimelineVideoItem } from '../../features/timeline/test-helpers'
import {
  rateStretchItem,
  rateStretchItemWithoutHistory,
  trimItemEnd,
  joinItems,
} from '../../features/timeline/stores/actions/item-edit-actions'
import { applyRateStretchMutation } from '../../features/timeline/stores/actions/edit/rate-stretch-mutations'
import {
  applyRippleTrimMutation,
  applyRollingTrimMutation,
  applySlideMutation,
  applySlipMutation,
  applyTrimMutation,
} from '../../features/timeline/stores/actions/edit/trim-mutations'
import { createNodeSession, type NodeCommandSession } from '../node-session'
import { captureProjectState, commitProjectState, type ProjectState } from '../project-state'
import { createSeededIdFactory } from '../id-factory'
import { deserializeCommands, serializeCommands } from '../serializer'
import { listCommandTypes } from '../registry'
import type { ExecuteResult, SerializedCommand } from '../types'
import { registerEditCommands } from './edit'

registerEditCommands()

const V = 'track-v1'
const A = 'track-a1'

function tracks(syncLock = false) {
  return [
    makeTimelineTrack({ id: V, name: 'V1', order: 0, kind: 'video', syncLock }),
    makeTimelineTrack({ id: A, name: 'A1', order: 1, kind: 'audio', syncLock }),
  ]
}

function vid(id: string, from: number, duration: number, overrides: Partial<VideoItem> = {}): VideoItem {
  return makeTimelineVideoItem({
    id,
    trackId: V,
    from,
    durationInFrames: duration,
    sourceStart: 100,
    sourceEnd: 100 + duration,
    sourceDuration: 2000,
    ...overrides,
  })
}

function aud(id: string, from: number, duration: number, overrides: Partial<AudioItem> = {}): AudioItem {
  return makeTimelineAudioItem({
    id,
    trackId: A,
    from,
    durationInFrames: duration,
    sourceStart: 100,
    sourceEnd: 100 + duration,
    sourceDuration: 2000,
    ...overrides,
  })
}

function fixture(
  items: TimelineItem[],
  options: { syncLock?: boolean; transitions?: Transition[] } = {},
): ProjectState {
  return {
    ...captureProjectState(),
    fps: 30,
    items,
    tracks: tracks(options.syncLock),
    transitions: options.transitions ?? [],
    keyframes: [],
  }
}

function open(state: ProjectState): NodeCommandSession {
  return createNodeSession(state, createSeededIdFactory('seed'))
}

function snapshot(session: NodeCommandSession) {
  const state = session.getState()
  return {
    items: state.items,
    tracks: state.tracks,
    transitions: state.transitions,
    keyframes: state.keyframes,
  }
}

function expectOk(result: ExecuteResult): Extract<ExecuteResult, { ok: true }> {
  if (!result.ok) throw new Error(result.issues.map((issue) => issue.message).join('; '))
  return result
}

function byId(session: NodeCommandSession, id: string): TimelineItem {
  const item = session.getState().items.find((candidate) => candidate.id === id)
  if (!item) throw new Error(`missing item ${id}`)
  return item
}

/** Execute, then prove undo/redo/serde: undo restores canonical `before`, redo `after`. */
function expectReversible(session: NodeCommandSession, commands: SerializedCommand[]) {
  const before = snapshot(session)
  const result = expectOk(session.executor.executeCommands(commands))
  const after = snapshot(session)
  expect(after).not.toEqual(before)
  expect(deserializeCommands(serializeCommands(result.commands))).toEqual(result.commands)
  expect(session.executor.undo().ok).toBe(true)
  expect(snapshot(session)).toEqual(before)
  expect(session.executor.redo().ok).toBe(true)
  expect(snapshot(session)).toEqual(after)
  return { result, before, after }
}

function issueCodes(result: ExecuteResult): string[] {
  return result.ok ? [] : result.issues.map((issue) => issue.code ?? 'schema')
}

describe('edit command registry', () => {
  it('registers every edit entry as a typed command', () => {
    expect(listCommandTypes().filter((type) => type.startsWith('edit.'))).toEqual(
      [
        'edit.joinItems',
        'edit.rateStretch',
        'edit.removeFillerWords',
        'edit.removeSilence',
        'edit.removeTranscriptRanges',
        'edit.repairLegacyAvTracks',
        'edit.resetSpeedWithRipple',
        'edit.rippleTrim',
        'edit.rollingTrim',
        'edit.insertFreezeFrame',
        'edit.slide',
        'edit.slip',
        'edit.sourceInsert',
        'edit.sourceOverwrite',
        'edit.splitAllAtFrame',
        'edit.splitItem',
        'edit.splitItemAtFrames',
        'edit.trimItemBreakingTransition',
        'edit.trimItemEnd',
        'edit.trimItemStart',
      ].sort((left, right) => left.localeCompare(right)),
    )
  })

  it('rejects non-finite frames, empty ids and malformed params without mutating', () => {
    const session = open(fixture([vid('a', 0, 60)]))
    try {
      const before = snapshot(session)
      for (const command of [
        { type: 'edit.trimItemEnd', params: { id: 'a', handle: 'end', amount: Number.NaN, linkedSelection: true } },
        { type: 'edit.trimItemEnd', params: { id: 'a', handle: 'end', amount: Infinity, linkedSelection: true } },
        { type: 'edit.trimItemEnd', params: { id: '', handle: 'end', amount: 1, linkedSelection: true } },
        { type: 'edit.trimItemEnd', params: { id: 'a', handle: 'start', amount: 1, linkedSelection: true } },
        { type: 'edit.trimItemEnd', params: { id: 'a', handle: 'end', amount: 1 } },
        { type: 'edit.joinItems', params: { itemIds: ['a'], linkedSelection: true } },
        { type: 'edit.rateStretch', params: { id: 'a', newFrom: 0, newDuration: 30, newSpeed: 0, linkedSelection: true } },
      ]) {
        expect(session.executor.executeCommands([command]).ok).toBe(false)
      }
      expect(snapshot(session)).toEqual(before)
      expect(session.executor.canUndo).toBe(false)
    } finally {
      session.dispose()
    }
  })
})

describe('trim commands', () => {
  it('trims start and end with explicit linked selection', () => {
    const items = [
      vid('v', 10, 60, { linkedGroupId: 'g' }),
      aud('a', 10, 60, { linkedGroupId: 'g' }),
    ]
    const linked = open(fixture(items))
    try {
      expectReversible(linked, [
        { type: 'edit.trimItemStart', params: { id: 'v', handle: 'start', amount: 5, linkedSelection: true } },
      ])
      expectOk(linked.executor.executeCommands([
        { type: 'edit.trimItemEnd', params: { id: 'v', handle: 'end', amount: -10, linkedSelection: true } },
      ]))
      for (const id of ['v', 'a']) {
        expect(byId(linked, id)).toMatchObject({ from: 15, durationInFrames: 45, sourceStart: 105, sourceEnd: 150 })
      }
    } finally {
      linked.dispose()
    }

    const solo = open(fixture(items))
    try {
      expectOk(solo.executor.executeCommands([
        { type: 'edit.trimItemStart', params: { id: 'v', handle: 'start', amount: 5, linkedSelection: false } },
      ]))
      expect(byId(solo, 'v')).toMatchObject({ from: 15, durationInFrames: 55 })
      expect(byId(solo, 'a')).toMatchObject({ from: 10, durationInFrames: 60 })
    } finally {
      solo.dispose()
    }
  })

  it('honours explicit participant ids instead of linked expansion', () => {
    const session = open(fixture([
      vid('v', 0, 60, { linkedGroupId: 'g' }),
      aud('a', 0, 60, { linkedGroupId: 'g' }),
      vid('other', 100, 60),
    ]))
    try {
      expectOk(session.executor.executeCommands([
        { type: 'edit.trimItemEnd', params: { id: 'v', handle: 'end', amount: -10, itemIds: ['other'], linkedSelection: true } },
      ]))
      expect(byId(session, 'v').durationInFrames).toBe(50)
      expect(byId(session, 'other').durationInFrames).toBe(50)
      expect(byId(session, 'a').durationInFrames).toBe(60)
    } finally {
      session.dispose()
    }
  })

  it('clamps to source bounds and records nothing when the clamp zeroes the trim', () => {
    const session = open(fixture([vid('a', 30, 60, { sourceStart: 0, sourceEnd: 60 })]))
    try {
      expectOk(session.executor.executeCommands([
        { type: 'edit.trimItemStart', params: { id: 'a', handle: 'start', amount: -10, linkedSelection: true } },
      ]))
      expect(byId(session, 'a')).toMatchObject({ from: 30, durationInFrames: 60, sourceStart: 0 })
      expect(session.executor.canUndo).toBe(false)
    } finally {
      session.dispose()
    }
  })

  it('never extends a trim into the adjacent clip', () => {
    const session = open(fixture([vid('a', 0, 60), vid('b', 60, 60)]))
    try {
      expectOk(session.executor.executeCommands([
        { type: 'edit.trimItemEnd', params: { id: 'a', handle: 'end', amount: 10, linkedSelection: false } },
      ]))
      expect(byId(session, 'a').durationInFrames).toBe(60)
      expect(session.executor.canUndo).toBe(false)
    } finally {
      session.dispose()
    }
  })

  it('removes the named transitions when breaking, and rejects unknown transitions', () => {
    const transition: Transition = {
      id: 't1',
      type: 'crossfade',
      presentation: 'fade',
      timing: 'linear',
      leftClipId: 'a',
      rightClipId: 'b',
      trackId: V,
      durationInFrames: 12,
    }
    const session = open(fixture([vid('a', 0, 60), vid('b', 60, 60)], { transitions: [transition] }))
    try {
      const rejected = session.executor.executeCommands([
        { type: 'edit.trimItemBreakingTransition', params: { id: 'b', handle: 'start', amount: 5, removeTransitionIds: ['ghost'], linkedSelection: false } },
      ])
      expect(issueCodes(rejected)).toContain('missing-transition')
      expect(session.getState().transitions).toHaveLength(1)

      const { after } = expectReversible(session, [
        { type: 'edit.trimItemBreakingTransition', params: { id: 'b', handle: 'start', amount: 5, removeTransitionIds: ['t1'], linkedSelection: false } },
      ])
      expect(after.transitions).toEqual([])
      expect(after.items.find((item) => item.id === 'b')).toMatchObject({ from: 65, durationInFrames: 55 })
    } finally {
      session.dispose()
    }
  })

  it('ripple trims linked clips and shifts downstream items on every synchronized track', () => {
    const items = [
      vid('v0', 0, 60, { linkedGroupId: 'g0' }),
      vid('v1', 60, 60, { linkedGroupId: 'g1' }),
      aud('a0', 0, 60, { linkedGroupId: 'g0' }),
      aud('a1', 60, 60, { linkedGroupId: 'g1' }),
    ]
    const linked = open(fixture(items))
    try {
      expectReversible(linked, [
        { type: 'edit.rippleTrim', params: { id: 'v0', handle: 'end', delta: -10, linkedSelection: true } },
      ])
      expect(byId(linked, 'v0').durationInFrames).toBe(50)
      expect(byId(linked, 'a0').durationInFrames).toBe(50)
      expect(byId(linked, 'v1').from).toBe(50)
      expect(byId(linked, 'a1').from).toBe(50)
    } finally {
      linked.dispose()
    }

    const unlinked = open(fixture(items))
    try {
      expectOk(unlinked.executor.executeCommands([
        { type: 'edit.rippleTrim', params: { id: 'v0', handle: 'end', delta: -10, linkedSelection: false } },
      ]))
      expect(byId(unlinked, 'v0').durationInFrames).toBe(50)
      expect(byId(unlinked, 'v1').from).toBe(50)
      expect(byId(unlinked, 'a0').durationInFrames).toBe(60)
      expect(byId(unlinked, 'a1').from).toBe(60)
    } finally {
      unlinked.dispose()
    }
  })

  it('re-anchors caption word timings when a text item is trimmed', () => {
    const words = [
      { text: 'a', startFrame: 0, endFrame: 30 },
      { text: 'b', startFrame: 30, endFrame: 60 },
      { text: 'c', startFrame: 60, endFrame: 90 },
    ]
    const caption: TextItem = {
      id: 'cap',
      type: 'text',
      trackId: V,
      from: 0,
      durationInFrames: 90,
      label: 'Caption',
      text: 'a b c',
      color: '#ffffff',
      words,
    }
    const session = open(fixture([caption]))
    try {
      expectReversible(session, [
        { type: 'edit.trimItemStart', params: { id: 'cap', handle: 'start', amount: 30, linkedSelection: false } },
      ])
      expect(byId(session, 'cap')).toMatchObject({
        from: 30,
        durationInFrames: 60,
        words: [
          { text: 'b', startFrame: 0, endFrame: 30 },
          { text: 'c', startFrame: 30, endFrame: 60 },
        ],
      })
    } finally {
      session.dispose()
    }
  })

  it('ripple trim propagates the removed interval to sync-locked tracks', () => {
    const session = open(fixture([
      vid('v0', 0, 60),
      vid('v1', 60, 60),
      aud('a0', 0, 60),
      aud('a1', 60, 60),
    ], { syncLock: true }))
    try {
      expectReversible(session, [
        { type: 'edit.rippleTrim', params: { id: 'v0', handle: 'end', delta: -10, linkedSelection: false } },
      ])
      expect(byId(session, 'v1').from).toBe(50)
      expect(byId(session, 'a0').durationInFrames).toBe(50)
      expect(byId(session, 'a1').from).toBe(50)
      expect(session.getState().items).toHaveLength(4)
    } finally {
      session.dispose()
    }
  })

  it('ripple trim of the start handle keeps the clip anchored and pulls downstream left', () => {
    const session = open(fixture([vid('a', 0, 60), vid('b', 60, 60)]))
    try {
      expectReversible(session, [
        { type: 'edit.rippleTrim', params: { id: 'a', handle: 'start', delta: 10, linkedSelection: false } },
      ])
      expect(byId(session, 'a')).toMatchObject({ from: 0, durationInFrames: 50, sourceStart: 110 })
      expect(byId(session, 'b').from).toBe(50)
    } finally {
      session.dispose()
    }
  })

  it('rolling trim moves the shared edit point and keeps total length', () => {
    const session = open(fixture([vid('a', 0, 60), vid('b', 60, 60)]))
    try {
      expectReversible(session, [
        { type: 'edit.rollingTrim', params: { leftId: 'a', rightId: 'b', delta: 10, linkedSelection: false } },
      ])
      expect(byId(session, 'a').durationInFrames).toBe(70)
      expect(byId(session, 'b')).toMatchObject({ from: 70, durationInFrames: 50 })
    } finally {
      session.dispose()
    }
  })

  it('slip shifts the source window only and clamps at the source start', () => {
    const session = open(fixture([vid('a', 0, 60, { sourceStart: 10, sourceEnd: 70 })]))
    try {
      expectReversible(session, [
        { type: 'edit.slip', params: { id: 'a', delta: 5, linkedSelection: false } },
      ])
      expectOk(session.executor.executeCommands([
        { type: 'edit.slip', params: { id: 'a', delta: -100, linkedSelection: false } },
      ]))
      expect(byId(session, 'a')).toMatchObject({ from: 0, durationInFrames: 60, sourceStart: 0, sourceEnd: 60 })
    } finally {
      session.dispose()
    }
  })

  it('slide moves the clip and absorbs the change in both neighbours', () => {
    const session = open(fixture([vid('a', 0, 60), vid('b', 60, 60), vid('c', 120, 60)]))
    try {
      expectReversible(session, [
        { type: 'edit.slide', params: { id: 'b', delta: 10, leftNeighborId: 'a', rightNeighborId: 'c', linkedSelection: false } },
      ])
      expect(byId(session, 'a').durationInFrames).toBe(70)
      expect(byId(session, 'b').from).toBe(70)
      expect(byId(session, 'c')).toMatchObject({ from: 130, durationInFrames: 50 })
    } finally {
      session.dispose()
    }
  })
})

describe('split commands', () => {
  it('splits a linked pair into two fresh groups with injected ids', () => {
    const session = open(fixture([
      vid('v', 0, 60, { linkedGroupId: 'g' }),
      aud('a', 0, 60, { linkedGroupId: 'g' }),
    ]))
    try {
      const { result } = expectReversible(session, [
        { type: 'edit.splitItem', params: { id: 'v', frame: 30, linkedSelection: true } },
      ])
      expect(result.results[0]).toMatchObject({ leftItem: { id: 'v' } })
      const items = session.getState().items
      expect(items).toHaveLength(4)
      const left = items.filter((item) => item.from === 0)
      const right = items.filter((item) => item.from === 30)
      expect(new Set(left.map((item) => item.linkedGroupId)).size).toBe(1)
      expect(new Set(right.map((item) => item.linkedGroupId)).size).toBe(1)
      expect(left[0]!.linkedGroupId).not.toBe(right[0]!.linkedGroupId)
      for (const item of items) expect(item.linkedGroupId).toMatch(/^seed-\d+$/)
    } finally {
      session.dispose()
    }
  })

  it('splits only the anchor when linked selection is off', () => {
    const session = open(fixture([
      vid('v', 0, 60, { linkedGroupId: 'g' }),
      aud('a', 0, 60, { linkedGroupId: 'g' }),
    ]))
    try {
      expectOk(session.executor.executeCommands([
        { type: 'edit.splitItem', params: { id: 'v', frame: 30, linkedSelection: false } },
      ]))
      expect(session.getState().items).toHaveLength(3)
      expect(byId(session, 'a').durationInFrames).toBe(60)
    } finally {
      session.dispose()
    }
  })

  it('classifies blocked splits: bounds, missing clip and transition zone', () => {
    const transition: Transition = {
      id: 't1',
      type: 'crossfade',
      presentation: 'fade',
      timing: 'linear',
      leftClipId: 'a',
      rightClipId: 'b',
      trackId: V,
      durationInFrames: 12,
    }
    const session = open(fixture([vid('a', 0, 60), vid('b', 60, 60)], { transitions: [transition] }))
    try {
      const split = (id: string, frame: number) =>
        session.executor.executeCommands([{ type: 'edit.splitItem', params: { id, frame, linkedSelection: true } }])
      expect(issueCodes(split('a', 0))).toEqual(['out-of-range'])
      expect(issueCodes(split('a', 60))).toEqual(['out-of-range'])
      expect(issueCodes(split('ghost', 10))).toEqual(['missing-item'])
      expect(issueCodes(split('a', 59))).toEqual(['transition-overlap'])
      expect(issueCodes(split('b', 61))).toEqual(['transition-overlap'])
      expect(session.getState().items).toHaveLength(2)
      expect(session.executor.canUndo).toBe(false)
      expect(split('a', 20).ok).toBe(true)
    } finally {
      session.dispose()
    }
  })

  it('splits every crossing group at a frame in one undo step and reports blocked groups', () => {
    const session = open(fixture([
      vid('v', 0, 60, { linkedGroupId: 'g' }),
      aud('a', 0, 60, { linkedGroupId: 'g' }),
      vid('other', 10, 40, { trackId: 'track-v2' }),
      vid('outside', 100, 20),
    ]))
    try {
      const { result } = expectReversible(session, [
        { type: 'edit.splitAllAtFrame', params: { frame: 30, linkedSelection: true } },
      ])
      expect(result.results[0]).toMatchObject({ splitCount: 2, blockedByTransitionCount: 0 })
      expect(session.getState().items).toHaveLength(7)
    } finally {
      session.dispose()
    }
  })

  it('splits at several frames and keeps fades only on the outer edges', () => {
    const session = open(fixture([vid('a', 0, 60, { fadeIn: 5, fadeOut: 5 })]))
    try {
      const { result } = expectReversible(session, [
        { type: 'edit.splitItemAtFrames', params: { id: 'a', frames: [40, 20, -5, 999], linkedSelection: false } },
      ])
      expect(result.results[0]).toEqual({ splitCount: 2 })
      const pieces = session.getState().items.toSorted((left, right) => left.from - right.from) as VideoItem[]
      expect(pieces.map((piece) => [piece.from, piece.durationInFrames])).toEqual([[0, 20], [20, 20], [40, 20]])
      expect(pieces.map((piece) => [piece.fadeIn ?? 0, piece.fadeOut ?? 0])).toEqual([[5, 0], [0, 0], [0, 5]])
    } finally {
      session.dispose()
    }
  })

  it('joins contiguous clips and undoes to the original pieces', () => {
    const session = open(fixture([vid('a', 0, 30), vid('b', 30, 30, { sourceStart: 130, sourceEnd: 160 })]))
    try {
      expectReversible(session, [{ type: 'edit.joinItems', params: { itemIds: ['a', 'b'], linkedSelection: false } }])
      expect(session.getState().items).toHaveLength(1)
      expect(byId(session, 'a').durationInFrames).toBe(60)
    } finally {
      session.dispose()
    }
  })
})

describe('speed commands', () => {
  const clip = () => vid('a', 0, 60, { sourceStart: 0, sourceEnd: 60, speed: 1 })

  it('rate stretch retimes the clip and ripples downstream clips', () => {
    const session = open(fixture([clip(), vid('b', 60, 30)]))
    try {
      expectReversible(session, [
        { type: 'edit.rateStretch', params: { id: 'a', newFrom: 0, newDuration: 30, newSpeed: 2, linkedSelection: false } },
      ])
      expect(byId(session, 'a')).toMatchObject({ durationInFrames: 30, speed: 2 })
      expect(byId(session, 'b').from).toBe(30)
    } finally {
      session.dispose()
    }
  })

  it('merges drag updates that share a merge key into one undo entry', () => {
    const session = open(fixture([clip()]))
    try {
      for (const duration of [48, 40, 30]) {
        expectOk(session.executor.executeCommands([
          { type: 'edit.rateStretch', params: { id: 'a', newFrom: 0, newDuration: duration, newSpeed: 60 / duration, linkedSelection: false, mergeKey: 'edit.rateStretch:a' } },
        ]))
      }
      expect(byId(session, 'a').durationInFrames).toBe(30)
      expect(session.executor.undo().ok).toBe(true)
      expect(byId(session, 'a')).toMatchObject({ durationInFrames: 60, speed: 1 })
      expect(session.executor.canUndo).toBe(false)

      for (const duration of [48, 40]) {
        expectOk(session.executor.executeCommands([
          { type: 'edit.rateStretch', params: { id: 'a', newFrom: 0, newDuration: duration, newSpeed: 60 / duration, linkedSelection: false } },
        ]))
      }
      expect(session.executor.undo().ok).toBe(true)
      expect(byId(session, 'a').durationInFrames).toBe(48)
    } finally {
      session.dispose()
    }
  })

  it('reset speed grows the clip back to 1x and pushes later clips right', () => {
    const session = open(fixture([
      vid('a', 0, 30, { sourceStart: 0, sourceEnd: 60, speed: 2 }),
      vid('b', 30, 30),
    ]))
    try {
      expectReversible(session, [
        { type: 'edit.resetSpeedWithRipple', params: { itemIds: ['a'], linkedSelection: false } },
      ])
      expect(byId(session, 'a')).toMatchObject({ durationInFrames: 60, speed: 1 })
      expect(byId(session, 'b').from).toBe(60)
    } finally {
      session.dispose()
    }
  })

  it('rate stretch retimes attached captions and scales their word timings', () => {
    const caption: TextItem = {
      id: 'cap',
      type: 'text',
      trackId: A,
      from: 30,
      durationInFrames: 20,
      label: 'Caption',
      text: 'xin chào',
      color: '#ffffff',
      textRole: 'caption',
      captionSource: { type: 'transcript', clipId: 'a', mediaId: 'media-1' },
      words: [
        { text: 'xin', startFrame: 0, endFrame: 10 },
        { text: 'chào', startFrame: 10, endFrame: 20 },
      ],
    }
    const session = open(fixture([vid('a', 0, 60), caption]))
    try {
      // 60 → 30 at speed 2: caption span scales by 0.5, words halve.
      expectReversible(session, [
        { type: 'edit.rateStretch', params: { id: 'a', newFrom: 0, newDuration: 30, newSpeed: 2, linkedSelection: false } },
      ])
      expect(byId(session, 'cap')).toMatchObject({
        from: 15,
        durationInFrames: 10,
        words: [
          { text: 'xin', startFrame: 0, endFrame: 5 },
          { text: 'chào', startFrame: 5, endFrame: 10 },
        ],
      })

      const session2 = open(fixture([
        vid('a', 0, 30, { sourceStart: 0, sourceEnd: 60, speed: 2 }),
        {
          ...caption,
          from: 10,
          durationInFrames: 10,
          words: [
            { text: 'xin', startFrame: 0, endFrame: 5 },
            { text: 'chào', startFrame: 5, endFrame: 10 },
          ],
        },
      ]))
      try {
        expectReversible(session2, [
          { type: 'edit.resetSpeedWithRipple', params: { itemIds: ['a'], linkedSelection: false } },
        ])
        expect(byId(session2, 'cap')).toMatchObject({
          from: 20,
          durationInFrames: 20,
          words: [
            { text: 'xin', startFrame: 0, endFrame: 10 },
            { text: 'chào', startFrame: 10, endFrame: 20 },
          ],
        })
      } finally {
        session2.dispose()
      }
    } finally {
      session.dispose()
    }
  })
})

describe('range removal commands', () => {
  it.each(['edit.removeSilence', 'edit.removeFillerWords', 'edit.removeTranscriptRanges'])(
    '%s splits at the range edges and ripples the covered segment away',
    (type) => {
      const session = open(fixture([vid('a', 0, 300, { sourceStart: 0, sourceEnd: 300, sourceFps: 30 })]))
      try {
        const params = {
          itemIds: ['a'],
          rangesByMediaId: { 'media-1': [{ start: 3, end: 5 }] },
          linkedSelection: false,
        }
        const { result } = expectReversible(session, [{ type, params }])
        expect(result.results[0]).toMatchObject({
          summary: { analyzedItemCount: 1, removedRangeCount: 1, removedItemCount: 1, splitCount: 2 },
        })
        const items = session.getState().items.toSorted((left, right) => left.from - right.from)
        expect(items).toHaveLength(2)
        expect(items.reduce((sum, item) => sum + item.durationInFrames, 0)).toBe(240)
        expect(items[1]!.from).toBe(items[0]!.from + items[0]!.durationInFrames)
      } finally {
        session.dispose()
      }
    },
  )

  it('is a recorded-nothing no-op when no range matches the media', () => {
    const session = open(fixture([vid('a', 0, 60)]))
    try {
      const result = expectOk(session.executor.executeCommands([
        { type: 'edit.removeSilence', params: { itemIds: ['a'], rangesByMediaId: { other: [{ start: 0, end: 1 }] }, linkedSelection: false } },
      ]))
      expect(result.results[0]).toMatchObject({ summary: { analyzedItemCount: 0 } })
      expect(session.executor.canUndo).toBe(false)
    } finally {
      session.dispose()
    }
  })
})

describe('freeze frame commit', () => {
  const image = { label: 'freeze.png', mediaId: 'frame-1', src: 'blob:frame', sourceWidth: 1920, sourceHeight: 1080 }

  it('splits, inserts the persisted still and shifts everything after it', () => {
    const session = open(fixture([vid('a', 0, 90), vid('b', 100, 30)]))
    try {
      const { result } = expectReversible(session, [
        { type: 'edit.insertFreezeFrame', params: { itemId: 'a', playheadFrame: 30, freezeDurationFrames: 60, image } },
      ])
      const { freezeFrameItemId } = result.results[0] as { freezeFrameItemId: string }
      expect(freezeFrameItemId).toMatch(/^seed-\d+$/)
      const items = session.getState().items.toSorted((left, right) => left.from - right.from)
      expect(items.map((item) => [item.type, item.from, item.durationInFrames])).toEqual([
        ['video', 0, 30],
        ['image', 30, 60],
        ['video', 90, 60],
        ['video', 160, 30],
      ])
      expect(items[1]).toMatchObject({ mediaId: 'frame-1', src: 'blob:frame', trackId: V })
    } finally {
      session.dispose()
    }
  })

  it('refuses edges, non-video clips and transition zones', () => {
    const transition: Transition = {
      id: 't1',
      type: 'crossfade',
      presentation: 'fade',
      timing: 'linear',
      leftClipId: 'a',
      rightClipId: 'b',
      trackId: V,
      durationInFrames: 12,
    }
    const session = open(fixture([vid('a', 0, 60), vid('b', 60, 60), aud('x', 0, 60)], { transitions: [transition] }))
    try {
      const freeze = (itemId: string, playheadFrame: number) =>
        session.executor.executeCommands([
          { type: 'edit.insertFreezeFrame', params: { itemId, playheadFrame, freezeDurationFrames: 60, image } },
        ])
      expect(issueCodes(freeze('a', 0))).toEqual(['out-of-range'])
      expect(issueCodes(freeze('x', 30))).toEqual(['not-video'])
      expect(issueCodes(freeze('ghost', 30))).toEqual(['not-video'])
      expect(issueCodes(freeze('a', 59))).toEqual(['transition-overlap'])
      expect(session.getState().items).toHaveLength(3)
    } finally {
      session.dispose()
    }
  })
})

describe('source edit commands', () => {
  const newClip = (overrides: Partial<VideoItem> = {}) =>
    makeTimelineVideoItem({
      id: 'new-1',
      trackId: V,
      from: 30,
      durationInFrames: 20,
      sourceStart: 0,
      sourceEnd: 20,
      ...overrides,
    })

  it('insert splits the straddling clip and ripples the remainder', () => {
    const session = open(fixture([vid('a', 0, 60)]))
    try {
      const params = { insertFrame: 30, clipDurationFrames: 20, tracks: tracks(), items: [newClip()] }
      expectReversible(session, [{ type: 'edit.sourceInsert', params }])
      const items = session.getState().items.toSorted((left, right) => left.from - right.from)
      expect(items.map((item) => [item.from, item.durationInFrames])).toEqual([[0, 30], [30, 20], [50, 30]])
      expect(items[1]!.id).toBe('new-1')
    } finally {
      session.dispose()
    }
  })

  it('overwrite replaces the covered range in place', () => {
    const session = open(fixture([vid('a', 0, 60)]))
    try {
      const params = { overwriteStart: 20, overwriteEnd: 40, tracks: tracks(), items: [newClip({ from: 20 })] }
      expectReversible(session, [{ type: 'edit.sourceOverwrite', params }])
      const items = session.getState().items.toSorted((left, right) => left.from - right.from)
      expect(items.map((item) => [item.from, item.durationInFrames])).toEqual([[0, 20], [20, 20], [40, 20]])
    } finally {
      session.dispose()
    }
  })

  it('validates duplicate ids, unknown tracks, source bounds and empty ranges', () => {
    const session = open(fixture([vid('a', 0, 60)]))
    try {
      const insert = (item: TimelineItem) =>
        session.executor.executeCommands([
          { type: 'edit.sourceInsert', params: { insertFrame: 30, clipDurationFrames: 20, tracks: tracks(), items: [item] } },
        ])
      expect(issueCodes(insert(newClip({ id: 'a' })))).toEqual(['duplicate-id'])
      expect(issueCodes(insert(newClip({ trackId: 'nope' })))).toEqual(['missing-track'])
      expect(issueCodes(insert(newClip({ sourceStart: 10, sourceEnd: 10 })))).toEqual(['source-bounds'])
      expect(issueCodes(insert(newClip({ sourceStart: -1 })))).toEqual(['source-bounds'])
      expect(
        issueCodes(
          session.executor.executeCommands([
            { type: 'edit.sourceOverwrite', params: { overwriteStart: 20, overwriteEnd: 20, tracks: tracks(), items: [newClip()] } },
          ]),
        ),
      ).toEqual(['empty-range'])
      expect(session.getState().items).toHaveLength(1)
      expect(session.executor.canUndo).toBe(false)
    } finally {
      session.dispose()
    }
  })
})

describe('legacy A/V repair command', () => {
  it('recomputes the layout from canonical state, mints ids from the injected factory and is idempotent', () => {
    const state: ProjectState = {
      ...fixture([vid('video-1', 0, 90, { mediaId: 'media-1' })]),
      tracks: [makeTimelineTrack({ id: V, name: 'Track 1', order: 0 })],
    }
    const session = open(state)
    try {
      const params = { videoHasAudioByMediaId: { 'media-1': true } }
      const { result } = expectReversible(session, [{ type: 'edit.repairLegacyAvTracks', params }])
      expect(result.results[0]).toBe(true)
      const audio = session.getState().items.filter((item) => item.type === 'audio')
      expect(audio).toHaveLength(1)
      expect(audio[0]!.id).toMatch(/^seed-\d+$/)

      const again = expectOk(session.executor.executeCommands([{ type: 'edit.repairLegacyAvTracks', params }]))
      expect(again.results[0]).toBe(false)
      expect(session.executor.undo().ok).toBe(true)
      expect(session.getState().items.filter((item) => item.type === 'audio')).toHaveLength(0)
    } finally {
      session.dispose()
    }
  })
})

describe('exported edit actions execute typed commands', () => {
  function lastCommandTypes(session: NodeCommandSession): string[] {
    const entry = session.executor.history.stacks(session.executor.contextKey).undo.at(-1)
    return entry ? entry.commands.map((command) => command.type) : []
  }

  it('routes trim, rate stretch and join wrappers through edit.* commands', () => {
    const session = open(fixture([vid('a', 0, 60, { sourceStart: 0, sourceEnd: 60, speed: 1 }), vid('b', 100, 30)]))
    try {
      trimItemEnd('a', -10)
      expect(lastCommandTypes(session)).toEqual(['edit.trimItemEnd'])
      expect(byId(session, 'a').durationInFrames).toBe(50)

      rateStretchItem('a', 0, 25, 2)
      expect(lastCommandTypes(session)).toEqual(['edit.rateStretch'])

      rateStretchItemWithoutHistory('b', 100, 20, 1.5)
      rateStretchItemWithoutHistory('b', 100, 15, 2)
      expect(byId(session, 'b').durationInFrames).toBe(15)
      const stack = session.executor.history.stacks(session.executor.contextKey).undo
      expect(stack.map((entry) => entry.commands[0]!.type)).toEqual([
        'edit.trimItemEnd',
        'edit.rateStretch',
        'edit.rateStretch',
      ])

      joinItems(['a'])
      expect(stack).toHaveLength(3)
    } finally {
      session.dispose()
    }
  })

  it('wrapper inputs that fail validation leave the project untouched', () => {
    const session = open(fixture([vid('a', 0, 60)]))
    try {
      const before = snapshot(session)
      trimItemEnd('a', Number.NaN)
      trimItemEnd('missing', 5)
      expect(snapshot(session)).toEqual(before)
      expect(session.executor.canUndo).toBe(false)
    } finally {
      session.dispose()
    }
  })
})

/** Mulberry32: deterministic across runs so failures reproduce from the seed alone. */
function seededRandom(seed: number): () => number {
  let state = seed >>> 0
  return () => {
    state = (state + 0x6d2b79f5) >>> 0
    let t = state
    t = Math.imul(t ^ (t >>> 15), t | 1)
    t ^= t + Math.imul(t ^ (t >>> 7), t | 61)
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296
  }
}

function randomFixture(random: () => number): ProjectState {
  const items: TimelineItem[] = []
  let cursor = Math.floor(random() * 20)
  const count = 3 + Math.floor(random() * 3)
  for (let index = 0; index < count; index += 1) {
    const duration = 40 + Math.floor(random() * 50)
    const sourceStart = 100 + Math.floor(random() * 300)
    const linked = random() < 0.6
    const shared = { from: cursor, durationInFrames: duration, sourceStart, sourceEnd: sourceStart + duration }
    items.push(vid(`v${index}`, cursor, duration, { ...shared, ...(linked ? { linkedGroupId: `g${index}` } : {}) }))
    if (linked) items.push(aud(`a${index}`, cursor, duration, { ...shared, linkedGroupId: `g${index}` }))
    cursor += duration + (random() < 0.3 ? Math.floor(random() * 15) : 0)
  }
  return fixture(items)
}

function randomCommand(random: () => number, state: ProjectState): SerializedCommand | null {
  const pick = <T>(list: T[]): T | undefined => list[Math.floor(random() * list.length)]
  const videos = state.items.filter((item) => item.trackId === V).toSorted((left, right) => left.from - right.from)
  const target = pick(videos)
  if (!target) return null
  const linkedSelection = random() < 0.5
  const amount = (random() < 0.5 ? -1 : 1) * (1 + Math.floor(random() * 15))
  switch (Math.floor(random() * 7)) {
    case 0:
      return { type: 'edit.trimItemStart', params: { id: target.id, handle: 'start', amount, linkedSelection } }
    case 1:
      return { type: 'edit.trimItemEnd', params: { id: target.id, handle: 'end', amount, linkedSelection } }
    case 2:
      return { type: 'edit.rippleTrim', params: { id: target.id, handle: random() < 0.5 ? 'start' : 'end', delta: amount, linkedSelection } }
    case 3:
      return { type: 'edit.slip', params: { id: target.id, delta: amount * 2, linkedSelection } }
    case 4:
      return {
        type: 'edit.splitItem',
        params: { id: target.id, frame: target.from + 1 + Math.floor(random() * Math.max(1, target.durationInFrames - 1)), linkedSelection },
      }
    case 5: {
      const index = videos.indexOf(target)
      const right = videos[index + 1]
      if (!right || right.from !== target.from + target.durationInFrames) return null
      return { type: 'edit.rollingTrim', params: { leftId: target.id, rightId: right.id, delta: amount, linkedSelection } }
    }
    default: {
      const index = videos.indexOf(target)
      const left = videos[index - 1]
      const right = videos[index + 1]
      return {
        type: 'edit.slide',
        params: { id: target.id, delta: amount, leftNeighborId: left?.id ?? null, rightNeighborId: right?.id ?? null, linkedSelection },
      }
    }
  }
}

describe('seeded edit sessions', () => {
  it.each([1, 2, 3, 5, 8, 13, 21, 34, 55, 89])(
    'seed %i: undo restores the start, redo and wire replay reproduce the end',
    (seed) => {
      const random = seededRandom(seed)
      const initial = randomFixture(random)
      const applied: SerializedCommand[] = []
      let initialSnapshot: ReturnType<typeof snapshot>
      let finalSnapshot: ReturnType<typeof snapshot>

      const session = open(initial)
      try {
        initialSnapshot = snapshot(session)
        for (let step = 0; step < 10; step += 1) {
          const command = randomCommand(random, session.getState())
          if (!command) continue
          const before = snapshot(session)
          const result = session.executor.executeCommands([command])
          if (!result.ok) {
            expect(snapshot(session)).toEqual(before)
            continue
          }
          applied.push(...result.commands)
          for (const item of session.getState().items) {
            expect(item.durationInFrames).toBeGreaterThanOrEqual(1)
            expect(item.from).toBeGreaterThanOrEqual(0)
          }
        }
        finalSnapshot = snapshot(session)

        while (session.executor.canUndo) expect(session.executor.undo().ok).toBe(true)
        expect(snapshot(session)).toEqual(initialSnapshot)
        while (session.executor.canRedo) expect(session.executor.redo().ok).toBe(true)
        expect(snapshot(session)).toEqual(finalSnapshot)
      } finally {
        session.dispose()
      }

      const replay = open(initial)
      try {
        const wire = deserializeCommands(serializeCommands(applied))
        for (const command of wire) expectOk(replay.executor.executeCommands([command]))
        expect(snapshot(replay)).toEqual(finalSnapshot)
      } finally {
        replay.dispose()
      }
    },
  )

  it.each([4, 6, 10, 16, 40])(
    'seed %i: command execution matches the direct store mutation',
    (seed) => {
      const random = seededRandom(seed)
      const initial = randomFixture(random)
      const session = open(initial)
      try {
        for (let step = 0; step < 8; step += 1) {
          const command = randomCommand(random, session.getState())
          if (!command || command.type === 'edit.splitItem') continue
          const start = session.getState()

          commitProjectState(start)
          const direct = runDirect(command)
          const directState = snapshot(session)

          commitProjectState(start)
          const outcome = session.executor.executeCommands([command])
          if (!outcome.ok) continue
          expect(snapshot(session)).toEqual(directState)
          expect(direct === null ? null : { affectedIds: direct }).toEqual(outcome.results[0])
        }
      } finally {
        session.dispose()
      }
    },
  )
})

function runDirect(command: SerializedCommand): string[] | null {
  const params = command.params as Record<string, never>
  switch (command.type) {
    case 'edit.trimItemStart':
    case 'edit.trimItemEnd':
      return applyTrimMutation(params as never)
    case 'edit.rippleTrim':
      return applyRippleTrimMutation(params as never)
    case 'edit.rollingTrim':
      return applyRollingTrimMutation(params as never)
    case 'edit.slip':
      return applySlipMutation(params as never)
    case 'edit.slide':
      return applySlideMutation(params as never)
    case 'edit.rateStretch':
      return applyRateStretchMutation(params as never)
    default:
      return null
  }
}
