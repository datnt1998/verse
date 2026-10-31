import { beforeEach, describe, expect, it } from 'vitest'
import { getCommandExecutor } from '@editor/commands'
import { useItemsStore } from '@editor/features/timeline/stores/items-store'
import { useKeyframesStore } from '@editor/features/timeline/stores/keyframes-store'
import { useTimelineStore } from '@editor/features/timeline/stores/timeline-store'
import { makeTimelineTrack, makeTimelineVideoItem, resetTimelineCompositionTestState } from '@editor/features/timeline/test-helpers'
import { buildEffectAnimatableProperty } from '@editor/types/keyframe'
import { applyLibraryDrop } from '@/editor/ui/library/library-actions'
import { currentEffects, removeEffectWithKeyframes } from './effects'

const keyframeGroups = () => useKeyframesStore.getState().keyframesByItemId.clip?.properties ?? []

describe('applying and removing an effect', () => {
  beforeEach(() => {
    resetTimelineCompositionTestState()
    getCommandExecutor().clearHistory()
    useItemsStore.getState().setTracks([makeTimelineTrack({ id: 'v1', name: 'V1', kind: 'video', order: 0 })])
    useItemsStore.getState().setItems([makeTimelineVideoItem({ id: 'clip', trackId: 'v1' })])
  })

  it('adds an effect to the chosen clip and undo takes it away again', () => {
    applyLibraryDrop({ type: 'effect', effectId: 'gpu-sepia' }, 'clip')

    expect(currentEffects('clip').map((entry) => entry.effect.gpuEffectType)).toEqual(['gpu-sepia'])

    getCommandExecutor().undo()
    expect(currentEffects('clip')).toHaveLength(0)
  })

  it('refuses to apply an effect when no video or image clip is chosen', () => {
    expect(() => applyLibraryDrop({ type: 'effect', effectId: 'gpu-sepia' }, 'missing')).toThrow()
  })

  it('removing an effect also removes its keyframes, and one undo brings both back', () => {
    useTimelineStore.getState().addEffect('clip', { type: 'gpu-effect', gpuEffectType: 'gpu-brightness', params: { amount: 0.5 } })
    const entry = currentEffects('clip')[0]!
    const property = buildEffectAnimatableProperty('gpu-brightness', entry.id, 'amount')
    useTimelineStore.getState().addKeyframe('clip', property, 0, 0.5)
    getCommandExecutor().clearHistory()

    removeEffectWithKeyframes('clip', entry)

    expect(currentEffects('clip')).toHaveLength(0)
    expect(keyframeGroups().some((group) => group.property === property)).toBe(false)

    getCommandExecutor().undo()
    expect(currentEffects('clip')).toHaveLength(1)
    expect(keyframeGroups().some((group) => group.property === property)).toBe(true)
  })
})
