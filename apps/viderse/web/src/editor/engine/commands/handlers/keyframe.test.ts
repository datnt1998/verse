// @vitest-environment node
import { describe, expect, it } from 'vitest'
import type { ProjectState, SerializedCommand } from '@editor/commands'
import { createNodeSession, createSeededIdFactory, serializeCommands, deserializeCommands, withIdFactory } from '@editor/commands'
import { makeTimelineTrack, makeTimelineVideoItem } from '@editor/features/timeline/test-helpers'
import { getPresetTrackHeight } from '@editor/features/timeline/utils/track-heights'
import { captureSnapshot, restoreSnapshot } from '@editor/test/freecut-snapshot'
import { useKeyframesStore } from '@editor/features/timeline/stores/keyframes-store'
import { beginMotionModifierEdit, updateMotionModifiersLive, commitMotionModifierEdit } from '@editor/features/timeline/stores/actions/motion-modifier-actions'
import * as keys from './keyframe-mutations'
import * as motion from './motion-modifier-mutations'
import * as text from './text-motion-mutations'
import * as preset from './preset-mutations'
import { TEXT_MOTION_IN_PRESET_IDS } from '@editor/shared/typography/text-motion/text-motion-preset-ids'

const modifier = { id: 'spin', type: 'spin' as const, enabled: true, amplitude: 1, frequency: 0.5, phaseFrames: 0, seed: 5 }
const wander = { id: 'wander-1', type: 'wander' as const, enabled: true, amplitude: 1, frequency: 0.25, phaseFrames: 0, seed: 42, channelGains: { x: 1, y: 1, width: 1, height: 1 } }
const layer = { id: 'layer', name: 'Lớp', enabled: true, source: 'built-in-preset' as const, sourcePresetId: 'motion', tracks: [{ property: 'x' as const, blend: 'add' as const, keyframes: [{ id: 'layer-key', frame: 0, value: 2, easing: 'linear' as const }] }] }
const textEffect = { presetId: TEXT_MOTION_IN_PRESET_IDS[0], durationFrames: 20, staggerFrames: 2, intensity: 1, order: 'forward' as const, easing: 'linear' as const, seed: 2 }
const link = { type: 'link' as const, targetProperty: 'rotation' as const, sourceItemId: 'b', sourceProperty: 'rotation' as const, enabled: true, timeOffsetFrames: 0 }
const expression = { type: 'expression' as const, targetProperty: 'width' as const, source: 'value + 1', enabled: true }
const reference = { itemId: 'a', property: 'x' as const, keyframeId: 'x0' }
function fixture(seed: number): ProjectState {
  const value = ((seed * 1664525 + 1013904223) >>> 0) / 0x100000000 * 50
  return {
    items: [makeTimelineVideoItem({ id: 'a', durationInFrames: 120, sourceEnd: 120, sourceDuration: 360, transform: { x: value, y: 0, rotation: 0 }, motionModifiers: [modifier], motionLayers: [layer], effects: [{ id: 'pulse', effect: { type: 'gpu-effect', gpuEffectType: 'gpu-trigger-wave', params: {} }, enabled: true, audioPulse: { enabled: true, beats: [], durationFrames: 10, strength: 1, chroma: 1, glowColorBase: 0 } }] }), makeTimelineVideoItem({ id: 'b', from: 120, sourceStart: 30, sourceEnd: 150, sourceDuration: 360 }), { id: 'text', label: 'Chữ', type: 'text', trackId: 'track-v1', from: 240, durationInFrames: 120, text: 'Chữ', color: '#ffffff', textMotion: { in: textEffect } }],
    tracks: [makeTimelineTrack({ id: 'track-v1', name: 'V1', kind: 'video', order: 0, height: getPresetTrackHeight() })],
    keyframes: [{ itemId: 'a', animationVersion: 2, properties: [{ property: 'x', keyframes: [{ id: 'x0', frame: 0, value, easing: 'linear' }, { id: 'x1', frame: 20, value: value + 10, easing: 'ease-in' }, { id: 'x2', frame: 160, value: 30, easing: 'linear', source: { applicationId: 'app', kind: 'saved-preset', presetId: 'saved', presetName: 'Đã lưu' } }] }], vectorProperties: [{ property: 'position', keyframes: [{ id: 'v0', frame: 0, value: { x: 1, y: 2 }, easing: 'linear' }, { id: 'v1', frame: 30, value: { x: 3, y: 4 }, easing: 'linear' }] }], propertyLinks: [link], expressions: [expression] }],
    transitions: [], markers: [], compositions: [], topLevelSequenceIds: [], inPoint: null, outPoint: null, fps: 30, scrollPosition: 0, snapEnabled: true, currentFrame: 0, masterBusDb: 0, projectId: null, projectMetadata: null, stashStack: [], mainHolder: null, mediaItems: [],
  }
}
function normalized(value: unknown): unknown { return JSON.parse(JSON.stringify(value)) }
interface Scenario { type: string; params: unknown; reference(): unknown }
const scenarios: Scenario[] = [
  { type: 'keyframe.add', params: { itemId: 'a', property: 'opacity', frame: 15, value: 0.4 }, reference: () => keys.addKeyframe('a', 'opacity', 15, 0.4) },
  { type: 'keyframe.addMany', params: { payloads: [{ itemId: 'a', property: 'opacity', frame: 10, value: 1 }] }, reference: () => keys.addKeyframes([{ itemId: 'a', property: 'opacity', frame: 10, value: 1 }]) },
  { type: 'keyframe.update', params: { ...reference, updates: { value: 12, easing: 'cubic-bezier', easingConfig: { type: 'cubic-bezier', bezier: { x1: 0.1, y1: -0.2, x2: 0.8, y2: 1.2 } } } }, reference: () => keys.updateKeyframe('a', 'x', 'x0', { value: 12, easing: 'cubic-bezier', easingConfig: { type: 'cubic-bezier', bezier: { x1: 0.1, y1: -0.2, x2: 0.8, y2: 1.2 } } }) },
  { type: 'keyframe.updateMany', params: { updates: [{ ...reference, updates: { frame: 20, value: 9 } }] }, reference: () => keys.updateKeyframes([{ ...reference, updates: { frame: 20, value: 9 } }]) },
  { type: 'keyframe.move', params: { moves: [{ ref: reference, newFrame: 12 }] }, reference: () => keys.updateKeyframes([{ ...reference, updates: { frame: 12 } }]) },
  { type: 'keyframe.remove', params: reference, reference: () => keys.removeKeyframe('a', 'x', 'x0') },
  { type: 'keyframe.removeMany', params: { refs: [reference] }, reference: () => keys.removeKeyframes([reference]) },
  { type: 'keyframe.removeForItem', params: { itemId: 'a' }, reference: () => keys.removeKeyframesForItem('a') },
  { type: 'keyframe.removeForProperty', params: { itemId: 'a', property: 'x' }, reference: () => keys.removeKeyframesForProperty('a', 'x') },
  { type: 'keyframe.removePresetApplication', params: { itemId: 'a', applicationId: 'app' }, reference: () => keys.removePresetKeyframeApplication('a', 'app') },
  { type: 'keyframe.removeManual', params: { itemId: 'a' }, reference: () => keys.removeManualKeyframes('a') },
  { type: 'keyframe.trimToBounds', params: { itemId: 'a' }, reference: () => keys.trimAnimationToItemBounds('a') },
  { type: 'keyframe.auto', params: { operations: [{ type: 'add', itemId: 'a', property: 'opacity', frame: 25, value: 0.2 }] }, reference: () => keys.applyAutoKeyframeOperations([{ type: 'add', itemId: 'a', property: 'opacity', frame: 25, value: 0.2 }]) },
  { type: 'keyframe.vectorUpsert', params: { itemId: 'a', property: 'scale', input: { frame: 5, value: { x: 100, y: 110 } } }, reference: () => keys.upsertVectorKeyframe('a', 'scale', { frame: 5, value: { x: 100, y: 110 } }) },
  { type: 'keyframe.vectorUpdate', params: { itemId: 'a', property: 'position', keyframeId: 'v0', updates: { frame: 30, temporalEase: { out: { speed: 12, influence: 40 } }, spatial: { inTangent: { x: -1, y: -2 }, outTangent: { x: 1, y: 2 }, continuous: true } } }, reference: () => keys.updateVectorKeyframe('a', 'position', 'v0', { frame: 30, temporalEase: { out: { speed: 12, influence: 40 } }, spatial: { inTangent: { x: -1, y: -2 }, outTangent: { x: 1, y: 2 }, continuous: true } }) },
  { type: 'keyframe.vectorRemove', params: { itemId: 'a', property: 'position', keyframeId: 'v0' }, reference: () => keys.removeVectorKeyframe('a', 'position', 'v0') },
  { type: 'keyframe.vectorPromote', params: { itemId: 'a', vectorProperty: { property: 'position', keyframes: [{ id: 'new-vector', frame: 4, value: { x: 6, y: 7 }, easing: 'linear' }] }, removeScalarProperties: ['x', 'y'] }, reference: () => keys.promoteTransformToVector('a', { property: 'position', keyframes: [{ id: 'new-vector', frame: 4, value: { x: 6, y: 7 }, easing: 'linear' }] }, ['x', 'y']) },
  { type: 'keyframe.vectorSeparate', params: { itemId: 'a', property: 'position', separated: true, conversion: { scalarProperties: [{ property: 'x', keyframes: [{ id: 'new-x', frame: 0, value: 1, easing: 'linear' }] }] } }, reference: () => keys.setVectorDimensionsSeparated('a', 'position', true, { scalarProperties: [{ property: 'x', keyframes: [{ id: 'new-x', frame: 0, value: 1, easing: 'linear' }] }] }) },
  { type: 'keyframe.setLink', params: { itemId: 'a', link: { ...link, targetProperty: 'opacity', sourceProperty: 'opacity' } }, reference: () => keys.setDirectPropertyLink('a', { ...link, targetProperty: 'opacity', sourceProperty: 'opacity' }) },
  { type: 'keyframe.removeLink', params: { itemId: 'a', property: 'rotation' }, reference: () => keys.removeDirectPropertyLink('a', 'rotation') },
  { type: 'keyframe.setExpression', params: { itemId: 'a', expression: { ...expression, source: 'value + 2' } }, reference: () => keys.setPropertyExpression('a', { ...expression, source: 'value + 2' }) },
  { type: 'keyframe.removeExpression', params: { itemId: 'a', property: 'width' }, reference: () => keys.removePropertyExpression('a', 'width') },
  { type: 'keyframe.applyMotionPreset', params: { payloads: [{ itemId: 'a', property: 'x', frame: 0, value: 50 }], clearProperties: [{ itemId: 'a', property: 'x', fromFrame: 0, toFrame: 20 }] }, reference: () => keys.applyMotionPresetKeyframes([{ itemId: 'a', property: 'x', frame: 0, value: 50 }], [{ itemId: 'a', property: 'x', fromFrame: 0, toFrame: 20 }]) },
  { type: 'keyframe.duplicate', params: { refs: [reference], frameOffset: 12 }, reference: () => { const k = useKeyframesStore.getState().getKeyframeById('a', 'x', 'x0')!; keys.addKeyframes([{ itemId: 'a', property: 'x', frame: k.frame + 12, value: k.value, easing: k.easing }]) } },
  { type: 'keyframe.motionLayersAdd', params: { assignments: [{ itemId: 'a', layer: { ...layer, id: 'layer-new' } }] }, reference: () => motion.applyMotionLayersToItems([{ itemId: 'a', layer: { ...layer, id: 'layer-new' } }]) },
  { type: 'keyframe.motionLayerRemove', params: { itemIds: ['a'], layerId: 'layer' }, reference: () => motion.removeMotionLayerFromItems(['a'], 'layer') },
  { type: 'keyframe.motionModifiersSet', params: { assignments: [{ itemId: 'a', modifier: { ...modifier, amplitude: 1.5 } }] }, reference: () => motion.applyMotionModifierToItems([{ itemId: 'a', modifier: { ...modifier, amplitude: 1.5 } }]) },
  { type: 'keyframe.motionModifiersSet', params: { assignments: [{ itemId: 'a', modifier: wander }] }, reference: () => motion.applyMotionModifierToItems([{ itemId: 'a', modifier: wander }]) },
  { type: 'keyframe.motionModifierRemove', params: { itemIds: ['a'], type: 'spin' }, reference: () => motion.removeMotionModifierFromItems(['a'], 'spin') },
  { type: 'keyframe.audioPulseRemove', params: { itemIds: ['a'] }, reference: () => motion.removeAudioPulseFromItems(['a']) },
  { type: 'keyframe.motionBake', params: { plan: [{ itemId: 'a', keyframes: [{ itemId: 'a', property: 'x', frame: 5, value: 20 }], clearProperties: ['x'], clearMotionModifiers: true, clearMotionLayers: true, clearAudioPulseEffectIds: ['pulse'] }] }, reference: () => motion.bakeMotionToKeyframes([{ itemId: 'a', keyframes: [{ itemId: 'a', property: 'x', frame: 5, value: 20 }], clearProperties: ['x'], clearMotionModifiers: true, clearMotionLayers: true, clearAudioPulseEffectIds: ['pulse'] }]) },
  { type: 'keyframe.textMotionSet', params: { itemIds: ['text'], slot: 'in', effect: { ...textEffect, intensity: 1.5 } }, reference: () => text.applyTextMotionEffect(['text'], 'in', { ...textEffect, intensity: 1.5 }) },
  { type: 'keyframe.textMotionUpdate', params: { itemIds: ['text'], slot: 'in', partial: { intensity: 1.8 } }, reference: () => text.updateTextMotionLive(['text'], 'in', { intensity: 1.8 }) },
  { type: 'keyframe.textMotionRemove', params: { itemIds: ['text'], slot: 'in' }, reference: () => text.removeTextMotionEffect(['text'], 'in') },
  { type: 'keyframe.applyPreset', params: { targetItemId: 'a', preset: { id: 'saved', name: 'Mẫu', sourceItemType: 'video', properties: [{ property: 'opacity', keyframes: [{ id: 'saved-key', frame: 0, value: 0.5, easing: 'linear' }] }], effects: [], sourceDurationInFrames: 120, createdAt: 0 } }, reference: () => preset.applyAnimationPreset('a', { id: 'saved', name: 'Mẫu', sourceItemType: 'video', properties: [{ property: 'opacity', keyframes: [{ id: 'saved-key', frame: 0, value: 0.5, easing: 'linear' }] }], effects: [], sourceDurationInFrames: 120, createdAt: 0 }) },
]

describe('keyframe command parity', () => {
  for (const scenario of scenarios) {
    it(`${scenario.type} matches mutation, snapshot undo, redo and serialized replay`, () => {
      for (const seed of [1, 17, 503]) {
        const idsSeed = `${scenario.type}-${seed}`
        const session = createNodeSession(fixture(seed), createSeededIdFactory(idsSeed))
        let before: ProjectState
        let after: ProjectState
        let referenceAfter: ProjectState
        try {
          before = session.getState()
          const snapshot = captureSnapshot()
          withIdFactory(createSeededIdFactory(idsSeed), scenario.reference)
          referenceAfter = session.getState()
          restoreSnapshot(snapshot)
          expect(normalized(session.getState())).toEqual(normalized(before))
          const command: SerializedCommand = { type: scenario.type, params: scenario.params }
          expect(session.executor.executeCommands([command])).toMatchObject({ ok: true })
          after = session.getState()
          expect(normalized(after)).toEqual(normalized(referenceAfter))
          expect(session.executor.undo()).toMatchObject({ ok: true })
          expect(normalized(session.getState())).toEqual(normalized(before))
          expect(session.executor.redo()).toMatchObject({ ok: true })
          expect(normalized(session.getState())).toEqual(normalized(after))
        } finally { session.dispose() }
        const replay = createNodeSession(fixture(seed), createSeededIdFactory(idsSeed))
        try {
          expect(replay.executor.executeCommands(deserializeCommands(serializeCommands([{ type: scenario.type, params: scenario.params }])))).toMatchObject({ ok: true })
          expect(normalized(replay.getState())).toEqual(normalized(after!))
        } finally { replay.dispose() }
      }
    })
  }
  it('prunes empty scalar shells and restores them exactly on undo', () => {
    const initial = fixture(1)
    initial.keyframes = [{ itemId: 'a', properties: [{ property: 'opacity', keyframes: [{ id: 'only', frame: 0, value: 1, easing: 'linear' }] }] }]
    const session = createNodeSession(initial)
    try {
      const before = session.getState()
      expect(session.executor.executeCommands([{ type: 'keyframe.remove', params: { itemId: 'a', property: 'opacity', keyframeId: 'only' } }])).toMatchObject({ ok: true })
      expect(session.getState().keyframes).toEqual([])
      expect(session.executor.undo()).toMatchObject({ ok: true })
      expect(normalized(session.getState())).toEqual(normalized(before))
    } finally { session.dispose() }
  })
  it('keeps consecutive motion drags separate after their gesture commits', () => {
    const session = createNodeSession(fixture(1))
    try {
      const before = session.getState()
      const first = beginMotionModifierEdit()
      updateMotionModifiersLive([{ itemId: 'a', modifier: { ...modifier, amplitude: 1.2 } }])
      updateMotionModifiersLive([{ itemId: 'a', modifier: { ...modifier, amplitude: 1.4 } }])
      commitMotionModifierEdit(first)
      expect(session.executor.history.stacks(session.executor.contextKey).undo).toHaveLength(1)
      const second = beginMotionModifierEdit()
      updateMotionModifiersLive([{ itemId: 'a', modifier: { ...modifier, amplitude: 1.7 } }])
      commitMotionModifierEdit(second)
      expect(session.executor.history.stacks(session.executor.contextKey).undo).toHaveLength(2)
      session.executor.undo()
      expect(session.getState().items[0].motionModifiers?.[0].amplitude).toBe(1.4)
      session.executor.undo()
      expect(normalized(session.getState())).toEqual(normalized(before))
    } finally { session.dispose() }
  })
})
