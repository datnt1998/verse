/**
 * Motion modifier actions — attach/remove procedural motion modifiers on items.
 *
 * Modifiers live on the timeline item (like effects), so these wrap
 * `_updateItem` in a single undo block. Applying a modifier replaces any
 * existing modifier of the same type on that item (apply == set, not stack).
 */

import type {
  MotionAnimationLayer,
  MotionModifier,
  MotionModifierType,
} from '@editor/types/motion'
import type { AnimatableProperty } from '@editor/types/keyframe'
import type { TimelineItem } from '@editor/types/timeline'
import { useItemsStore } from '../../features/timeline/stores/items-store'
import { useKeyframesStore, type KeyframeAddPayload } from '../../features/timeline/stores/keyframes-store'
import { canAddKeyframeAtFrame } from '../../features/timeline/stores/actions/shared'
import { createLogger, createOperationId } from '@editor/shared/logging/logger'

// Function declaration (not a module-scope const) to avoid temporal dead zone
// errors in production chunk ordering — see CLAUDE.md gotchas / shared.ts.
function getLog() {
  return createLogger('MotionModifierActions')
}

export interface MotionModifierAssignment {
  itemId: string
  modifier: MotionModifier
}

export interface MotionLayerAssignment {
  itemId: string
  layer: MotionAnimationLayer
}

/** Attach one independently removable additive animation layer per item. */
export function applyMotionLayersToItems(assignments: MotionLayerAssignment[]): number {
  if (assignments.length === 0) return 0
  return (() => {
      const store = useItemsStore.getState()
      let count = 0
      for (const { itemId, layer } of assignments) {
        const item = store.itemById[itemId]
        if (!item) continue
        store._updateItem(itemId, { motionLayers: [...(item.motionLayers ?? []), layer] })
        count += 1
      }
      
      return count
    })()
}

/** Remove a named additive layer without touching base keyframes or modifiers. */
export function removeMotionLayerFromItems(itemIds: string[], layerId: string): number {
  if (itemIds.length === 0) return 0
  return (() => {
      const store = useItemsStore.getState()
      let count = 0
      for (const itemId of itemIds) {
        const item = store.itemById[itemId]
        if (!item?.motionLayers?.some((layer) => layer.id === layerId)) continue
        store._updateItem(itemId, {
          motionLayers: item.motionLayers.filter((layer) => layer.id !== layerId),
        })
        count += 1
      }
      
      return count
    })()
}

function withModifier(
  existing: MotionModifier[] | undefined,
  modifier: MotionModifier,
): MotionModifier[] {
  const kept = (existing ?? []).filter((entry) => entry.type !== modifier.type)
  return [...kept, modifier]
}

/**
 * Apply one modifier to each listed item (single undo entry). Replaces any
 * existing modifier of the same type. Returns the number of items updated.
 */
export function applyMotionModifierToItems(assignments: MotionModifierAssignment[]): number {
  if (assignments.length === 0) return 0

  const event = getLog().startEvent('applyMotionModifiers', createOperationId())
  event.merge({
    requested: assignments.length,
    modifierTypes: [...new Set(assignments.map((a) => a.modifier.type))],
  })

  try {
    const updated = (() => {
        const store = useItemsStore.getState()
        let count = 0
        for (const { itemId, modifier } of assignments) {
          const item = store.itemById[itemId]
          if (!item) continue
          store._updateItem(itemId, {
            motionModifiers: withModifier(item.motionModifiers, modifier),
          })
          count += 1
        }
        
        return count
      })()
    event.success({ updated })
    return updated
  } catch (error) {
    event.failure(error)
    throw error
  }
}


export interface BakeMotionPlanEntry {
  itemId: string
  /** Baked keyframes to add (already item-scoped). */
  keyframes: KeyframeAddPayload[]
  /** Properties whose existing keyframes are wiped before adding the baked set. */
  clearProperties: AnimatableProperty[]
  /** Drop all transform motion modifiers from the item. */
  clearMotionModifiers: boolean
  /** Drop all non-destructive preset animation layers from the item. */
  clearMotionLayers: boolean
  /** Effect ids whose audio-pulse modulation should be removed. */
  clearAudioPulseEffectIds: string[]
}

/**
 * Bake procedural motion into keyframes: replace the baked properties' keyframes
 * with the sampled set and drop the procedural sources — all in one undo entry.
 * Returns the number of items baked.
 */
export function bakeMotionToKeyframes(plan: BakeMotionPlanEntry[]): number {
  if (plan.length === 0) return 0

  const event = getLog().startEvent('bakeMotionToKeyframes', createOperationId())
  event.merge({
    plannedItems: plan.length,
    plannedKeyframes: plan.reduce((sum, entry) => sum + entry.keyframes.length, 0),
    clearedProperties: plan.reduce((sum, entry) => sum + entry.clearProperties.length, 0),
  })

  try {
    let addedKeyframes = 0
    let blockedKeyframes = 0
    const baked = (() => {
        const itemsStore = useItemsStore.getState()
        const keyframesStore = useKeyframesStore.getState()
        let count = 0

        for (const entry of plan) {
          const item = itemsStore.itemById[entry.itemId]
          if (!item) continue

          for (const property of entry.clearProperties) {
            keyframesStore._removeKeyframesForProperty(entry.itemId, property)
          }

          const valid = entry.keyframes.filter((payload) =>
            canAddKeyframeAtFrame(payload.itemId, payload.frame),
          )
          blockedKeyframes += entry.keyframes.length - valid.length
          if (valid.length > 0) {
            keyframesStore._addKeyframes(valid)
            addedKeyframes += valid.length
          }

          const updates: Partial<TimelineItem> = {}
          if (entry.clearMotionModifiers) {
            updates.motionModifiers = []
          }
          if (entry.clearMotionLayers) {
            updates.motionLayers = []
          }
          if (entry.clearAudioPulseEffectIds.length > 0 && item.effects) {
            const ids = new Set(entry.clearAudioPulseEffectIds)
            updates.effects = item.effects.map((effect) =>
              ids.has(effect.id) ? { ...effect, audioPulse: undefined } : effect,
            )
          }
          if (Object.keys(updates).length > 0) {
            itemsStore._updateItem(entry.itemId, updates)
          }

          count += 1
        }

        
        return count
      })()
    event.success({ baked, addedKeyframes, blockedKeyframes })
    return baked
  } catch (error) {
    event.failure(error)
    throw error
  }
}

/**
 * Remove a modifier type from each listed item (single undo entry). Returns the
 * number of items that actually had the modifier removed.
 */
export function removeMotionModifierFromItems(itemIds: string[], type: MotionModifierType): number {
  if (itemIds.length === 0) return 0

  return (() => {
      const store = useItemsStore.getState()
      let updated = 0
      for (const itemId of itemIds) {
        const item = store.itemById[itemId]
        if (!item?.motionModifiers?.some((entry) => entry.type === type)) continue
        store._updateItem(itemId, {
          motionModifiers: item.motionModifiers.filter((entry) => entry.type !== type),
        })
        updated += 1
      }
      
      return updated
    })()
}

/** Remove live audio-pulse modulation while leaving the visual effect itself intact. */
export function removeAudioPulseFromItems(itemIds: string[]): number {
  if (itemIds.length === 0) return 0

  return (() => {
      const store = useItemsStore.getState()
      let updated = 0
      for (const itemId of itemIds) {
        const item = store.itemById[itemId]
        if (!item?.effects?.some((effect) => effect.audioPulse?.enabled)) continue
        store._updateItem(itemId, {
          effects: item.effects.map((effect) =>
            effect.audioPulse ? { ...effect, audioPulse: undefined } : effect,
          ),
        })
        updated += 1
      }
      
      return updated
    })()
}
