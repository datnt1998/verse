import { z } from 'zod'
import { registerCommand } from '../registry'
import { useItemsStore } from '@editor/features/timeline/stores/items-store'
import { useTransitionsStore } from '@editor/features/timeline/stores/transitions-store'
import { useTimelineSettingsStore } from '@editor/features/timeline/stores/timeline-settings-store'
import { repairTransitions } from '@editor/features/timeline/utils/transition-auto-repair'
import * as mutations from './transition-mutations'

const id = z.string().min(1)
const number = z.number().finite()
const direction = z.enum(['from-left', 'from-right', 'from-top', 'from-bottom'])
const alignment = number.min(0).max(1)
const updates = z.object({
  durationInFrames: z.number().int().positive().optional(), type: z.literal('crossfade').optional(), presentation: id.optional(), direction: direction.optional(), timing: z.enum(['linear', 'ease-in', 'ease-out', 'ease-in-out', 'cubic-bezier']).optional(), alignment: alignment.optional(),
  bezierPoints: z.object({ x1: number.min(0).max(1), y1: number, x2: number.min(0).max(1), y2: number }).optional(), presetId: id.optional(), properties: z.record(z.string(), z.json()).optional(),
})
function register<P>(type: string, params: z.ZodType<P>, run: (input: P) => unknown, mergeKey?: (input: P) => string | null): void {
  registerCommand({ type, params, mergeKey, apply: (_ctx, input) => ({ inverse: { kind: 'patch', patch: {} }, result: run(input) }) })
}
export function registerTransitionCommands(): void {
  register('transition.add', z.object({ leftClipId: id, rightClipId: id, type: z.literal('crossfade').default('crossfade'), durationInFrames: z.number().int().positive().optional(), presentation: id.optional(), direction: direction.optional(), alignment: alignment.default(0.5), timing: updates.shape.timing }), p => {
    const added = mutations.addTransition(p.leftClipId, p.rightClipId, p.type, p.durationInFrames, p.presentation, p.direction, p.alignment)
    if (added && p.timing) {
      const transition = useTransitionsStore.getState().transitions.find(t => t.leftClipId === p.leftClipId && t.rightClipId === p.rightClipId)
      if (transition) mutations.updateTransition(transition.id, { timing: p.timing })
    }
    return added
  })
  register('transition.update', z.object({ id, updates }), p => mutations.updateTransition(p.id, p.updates), p => `transition:${p.id}`)
  register('transition.updateMany', z.object({ updates: z.array(z.object({ id, updates })) }), p => mutations.updateTransitions(p.updates), p => `transitions:${p.updates.map(u => u.id).sort().join(',')}`)
  register('transition.remove', z.object({ id }), p => mutations.removeTransition(p.id))
  register('transition.removeMany', z.object({ ids: z.array(id) }), p => useTransitionsStore.getState()._removeTransitions(p.ids))
  register('transition.repair', z.object({ changedClipIds: z.array(id), deletedClipIds: z.array(id).optional() }), p => {
    const { valid, repaired, broken } = repairTransitions(p.changedClipIds, useItemsStore.getState().items, useTransitionsStore.getState().transitions, p.deletedClipIds ? new Set(p.deletedClipIds) : undefined, useTimelineSettingsStore.getState().fps)
    useTransitionsStore.getState().setTransitions([...valid, ...repaired.map(entry => entry.repaired)])
    return { repaired, broken }
  })
}
