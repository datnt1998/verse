import type { ItemEffect, VisualEffect } from '@editor/types/effects'
import { executeCommands } from '@editor/commands'
import type { SerializedCommand } from '@editor/commands/types'
import { useItemsStore } from '../items-store'
import { emitUiSound } from '@editor/shared/ui/ui-sound'
import { CHROMA_KEY_EFFECT, detectChromaKeyColorForItem } from '@editor/features/timeline/utils/chroma-key-detect'

function isVisualItem(itemId: string): boolean {
  const item = useItemsStore.getState().itemById[itemId]
  return !!item && item.type !== 'audio'
}

function run(command: SerializedCommand): void {
  const result = executeCommands([command])
  if (!result.ok) throw new Error(result.issues.map((issue) => issue.message).join('; '))
}

export function addEffect(itemId: string, effect: VisualEffect): void {
  if (!isVisualItem(itemId)) return
  run({ type: 'effect.add', params: { itemId, effect } })
  if (effect.type === 'gpu-effect' && effect.gpuEffectType === CHROMA_KEY_EFFECT) {
    const item = useItemsStore.getState().itemById[itemId]
    if (!item?.mediaId) return
    // Best-effort auto colour pick; a failure just leaves the default green.
    void detectChromaKeyColorForItem(item)
      .then((keyColor) => {
        if (!keyColor) return
        const entry = useItemsStore.getState().itemById[itemId]?.effects?.find(candidate => candidate.effect === effect)
        if (!entry) return
        updateEffect(itemId, entry.id, { effect: { ...entry.effect, params: { ...entry.effect.params, keyColor } } })
      })
      .catch(() => undefined)
  }
}

export function addEffects(updates: Array<{ itemId: string; effects: VisualEffect[] }>): void {
  const visualUpdates = updates.filter((update) => isVisualItem(update.itemId))
  if (visualUpdates.length) run({ type: 'effect.addMany', params: { updates: visualUpdates } })
}

export function updateEffect(itemId: string, effectId: string, updates: Partial<{ effect: VisualEffect; enabled: boolean }>): void {
  if (!useItemsStore.getState().itemById[itemId]?.effects?.some((effect) => effect.id === effectId)) return
  run({ type: 'effect.update', params: { itemId, effectId, updates } })
}

export function removeEffect(itemId: string, effectId: string): void {
  if (!useItemsStore.getState().itemById[itemId]?.effects?.some((effect) => effect.id === effectId)) return
  run({ type: 'effect.remove', params: { itemId, effectId } })
}

/** Replace retained effect instances without changing their ids. */
export function setItemEffects(updates: Array<{ itemId: string; effects: ItemEffect[] }>): void {
  const visualUpdates = updates.filter((update) => isVisualItem(update.itemId))
  if (visualUpdates.length) run({ type: 'effect.setItems', params: { updates: visualUpdates } })
}

export function toggleEffect(itemId: string, effectId: string): void {
  const effect = useItemsStore.getState().itemById[itemId]?.effects?.find((entry) => entry.id === effectId)
  if (effect && isVisualItem(itemId)) run({ type: 'effect.toggle', params: { itemId, effectId } })
  emitUiSound(effect?.enabled ? 'toggleOff' : 'toggleOn')
}
