// @vitest-environment node
import { describe, expect, it } from 'vitest'
import type { ProjectState, SerializedCommand } from '@editor/commands'
import { createNodeSession, createSeededIdFactory, withIdFactory, serializeCommand, deserializeCommand } from '@editor/commands'
import { makeTimelineTrack, makeTimelineVideoItem } from '@editor/features/timeline/test-helpers'
import { getPresetTrackHeight } from '@editor/features/timeline/utils/track-heights'
import { captureSnapshot, restoreSnapshot } from '@editor/test/freecut-snapshot'
import { useTransitionsStore } from '@editor/features/timeline/stores/transitions-store'
import { useItemsStore } from '@editor/features/timeline/stores/items-store'
import { repairTransitions } from '@editor/features/timeline/utils/transition-auto-repair'
import * as mutation from './transition-mutations'

function fixture(seed: number, withTransition = true): ProjectState {
  const duration = 60 + ((seed * 1103515245 + 12345) >>> 0) % 60
  return {
    items: [makeTimelineVideoItem({ id: 'left', durationInFrames: duration, sourceStart: 30, sourceEnd: 30 + duration, sourceDuration: 360 }), makeTimelineVideoItem({ id: 'right', from: duration, durationInFrames: duration, sourceStart: 30, sourceEnd: 30 + duration, sourceDuration: 360 })],
    tracks: [makeTimelineTrack({ id: 'track-v1', name: 'V1', kind: 'video', order: 0, height: getPresetTrackHeight() })],
    transitions: withTransition ? [{ id: 'fade', type: 'crossfade', presentation: 'fade', timing: 'linear', leftClipId: 'left', rightClipId: 'right', trackId: 'track-v1', durationInFrames: 12, alignment: 0.5 }] : [],
    keyframes: [], markers: [], compositions: [], topLevelSequenceIds: [], inPoint: null, outPoint: null, fps: 30, scrollPosition: 0, snapEnabled: true, currentFrame: 0, masterBusDb: 0, projectId: null, projectMetadata: null, stashStack: [], mainHolder: null, mediaItems: [],
  }
}
function normalized(value: unknown): unknown { return JSON.parse(JSON.stringify(value)) }
const scenarios = [
  { type: 'transition.add', params: { leftClipId: 'left', rightClipId: 'right', durationInFrames: 24, alignment: 0.25 }, hasTransition: false, reference: () => mutation.addTransition('left', 'right', 'crossfade', 24, undefined, undefined, 0.25) },
  { type: 'transition.update', params: { id: 'fade', updates: { durationInFrames: 18, alignment: 0.3, timing: 'cubic-bezier', bezierPoints: { x1: 0.2, y1: -0.1, x2: 0.8, y2: 1.2 } } }, hasTransition: true, reference: () => mutation.updateTransition('fade', { durationInFrames: 18, alignment: 0.3, timing: 'cubic-bezier', bezierPoints: { x1: 0.2, y1: -0.1, x2: 0.8, y2: 1.2 } }) },
  { type: 'transition.updateMany', params: { updates: [{ id: 'fade', updates: { presentation: 'wipe', direction: 'from-left' } }, { id: 'fade', updates: { alignment: 0.7 } }] }, hasTransition: true, reference: () => mutation.updateTransitions([{ id: 'fade', updates: { presentation: 'wipe', direction: 'from-left' } }, { id: 'fade', updates: { alignment: 0.7 } }]) },
  { type: 'transition.remove', params: { id: 'fade' }, hasTransition: true, reference: () => mutation.removeTransition('fade') },
  { type: 'transition.removeMany', params: { ids: ['fade'] }, hasTransition: true, reference: () => useTransitionsStore.getState()._removeTransitions(['fade']) },
  { type: 'transition.repair', params: { changedClipIds: ['right'] }, hasTransition: true, reference: () => {
    const result = repairTransitions(['right'], useItemsStore.getState().items, useTransitionsStore.getState().transitions, undefined, 30)
    useTransitionsStore.getState().setTransitions([...result.valid, ...result.repaired.map(entry => entry.repaired)])
  } },
]
describe('transition command parity', () => {
  for (const scenario of scenarios) it(`${scenario.type} matches Freecut repair, undo, redo and serialization`, () => {
    for (const seed of [1, 23, 719]) {
      const initial = fixture(seed, scenario.hasTransition)
      if (scenario.type === 'transition.repair') initial.items[1] = { ...initial.items[1], from: initial.items[1].from + 10 }
      const ids = `${scenario.type}-${seed}`
      const session = createNodeSession(initial, createSeededIdFactory(ids))
      let after: ProjectState
      try {
        const before = session.getState()
        const snapshot = captureSnapshot()
        withIdFactory(createSeededIdFactory(ids), () => { scenario.reference() })
        const reference = session.getState()
        restoreSnapshot(snapshot)
        expect(normalized(session.getState())).toEqual(normalized(before))
        expect(session.executor.executeCommands([{ type: scenario.type, params: scenario.params }])).toMatchObject({ ok: true })
        after = session.getState()
        expect(normalized(after)).toEqual(normalized(reference))
        expect(session.executor.undo()).toMatchObject({ ok: true })
        expect(normalized(session.getState())).toEqual(normalized(before))
        expect(session.executor.redo()).toMatchObject({ ok: true })
        expect(normalized(session.getState())).toEqual(normalized(after))
      } finally { session.dispose() }
      const replay = createNodeSession(initial, createSeededIdFactory(ids))
      try {
        const serialized: SerializedCommand = { type: scenario.type, params: scenario.params }
        expect(replay.executor.executeCommands([deserializeCommand(serializeCommand(serialized))])).toMatchObject({ ok: true })
        expect(normalized(replay.getState())).toEqual(normalized(after!))
      } finally { replay.dispose() }
    }
  })
  it('clamps an add to available source handles without moving either clip', () => {
    const initial = fixture(1, false)
    initial.items[0] = { ...initial.items[0], sourceDuration: (initial.items[0].sourceEnd ?? 0) + 3 }
    const session = createNodeSession(initial)
    try {
      const positions = session.getState().items.map(item => item.from)
      expect(session.executor.executeCommands([{ type: 'transition.add', params: { leftClipId: 'left', rightClipId: 'right', durationInFrames: 50 } }])).toMatchObject({ ok: true, results: [true] })
      expect(session.getState().transitions[0].durationInFrames).toBeLessThanOrEqual(7)
      expect(session.getState().items.map(item => item.from)).toEqual(positions)
    } finally { session.dispose() }
  })
  it('preserves explicit add timing and rejects unknown timing', () => {
    const session = createNodeSession(fixture(1, false))
    try {
      const params = { leftClipId: 'left', rightClipId: 'right', durationInFrames: 15, presentation: 'fade', alignment: 0.5 }
      expect(session.executor.executeCommands([{ type: 'transition.add', params: { ...params, timing: 'broken' } }])).toMatchObject({ ok: false })
      expect(session.getState().transitions).toEqual([])
      expect(session.executor.executeCommands([{ type: 'transition.add', params: { ...params, timing: 'ease-in-out' } }])).toMatchObject({ ok: true })
      expect(session.getState().transitions[0]).toMatchObject({ timing: 'ease-in-out', presentation: 'fade', alignment: 0.5, durationInFrames: 15 })
    } finally { session.dispose() }
  })
  it('rejects non-finite frames and invalid alignment atomically', () => {
    const session = createNodeSession(fixture(1))
    try {
      const before = session.getState()
      expect(session.executor.executeCommands([{ type: 'transition.update', params: { id: 'fade', updates: { alignment: 2, durationInFrames: Infinity } } }])).toMatchObject({ ok: false })
      expect(normalized(session.getState())).toEqual(normalized(before))
    } finally { session.dispose() }
  })
})
