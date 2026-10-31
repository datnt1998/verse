import '@editor/commands'
import type { TimelineItem } from '@editor/types/timeline'
import { useSelectionStore } from '@editor/shared/state/selection'
import { runItemCommand, splitItemUpdates } from '@editor/commands/handlers/item'
import { getLinkedItems } from '../../utils/linked-items'
import {
  getCurrentTimelineSnapshot,
  getEffectiveCompositions,
  getRootTimelineSnapshot,
} from './shared'

export interface MediaDeletionImpact {
  itemIds: string[]
  rootReferenceCount: number
  nestedReferenceCount: number
  totalReferenceCount: number
}

function isMediaReferenceItem(item: TimelineItem, mediaIds: ReadonlySet<string>): boolean {
  return !!item.mediaId && mediaIds.has(item.mediaId)
}

function collectMediaReferenceItemIds(
  items: TimelineItem[],
  mediaIds: ReadonlySet<string>,
): string[] {
  return items.filter((item) => isMediaReferenceItem(item, mediaIds)).map((item) => item.id)
}

function shouldCountLogicalMediaReference(
  item: TimelineItem,
  items: TimelineItem[],
  mediaIds: ReadonlySet<string>,
): boolean {
  if (!isMediaReferenceItem(item, mediaIds)) {
    return false
  }

  if (item.type !== 'audio') {
    return true
  }

  const linkedItems = getLinkedItems(items, item.id)
  return !linkedItems.some(
    (linkedItem) =>
      linkedItem.id !== item.id &&
      linkedItem.type !== 'audio' &&
      isMediaReferenceItem(linkedItem, mediaIds),
  )
}

function countLogicalMediaReferences(items: TimelineItem[], mediaIds: ReadonlySet<string>): number {
  return items.filter((item) => shouldCountLogicalMediaReference(item, items, mediaIds)).length
}

export function getMediaDeletionImpact(mediaIds: string[]): MediaDeletionImpact {
  const targetMediaIds = new Set(mediaIds.filter(Boolean))
  if (targetMediaIds.size === 0) {
    return {
      itemIds: [],
      rootReferenceCount: 0,
      nestedReferenceCount: 0,
      totalReferenceCount: 0,
    }
  }

  const currentSnapshot = getCurrentTimelineSnapshot()
  const rootSnapshot = getRootTimelineSnapshot(currentSnapshot)
  const rootItemIds = collectMediaReferenceItemIds(rootSnapshot.items, targetMediaIds)
  const nestedItemIds = getEffectiveCompositions(currentSnapshot).flatMap((composition) =>
    collectMediaReferenceItemIds(composition.items, targetMediaIds),
  )
  const itemIds = Array.from(new Set([...rootItemIds, ...nestedItemIds]))
  const rootReferenceCount = countLogicalMediaReferences(rootSnapshot.items, targetMediaIds)
  const nestedReferenceCount = getEffectiveCompositions(currentSnapshot).reduce(
    (count, composition) => count + countLogicalMediaReferences(composition.items, targetMediaIds),
    0,
  )

  return {
    itemIds,
    rootReferenceCount,
    nestedReferenceCount,
    totalReferenceCount: rootReferenceCount + nestedReferenceCount,
  }
}

export function removeProjectItems(itemIds: string[]): boolean {
  const targetIds = Array.from(new Set(itemIds.filter(Boolean)))
  if (targetIds.length === 0) return false

  const outcome = runItemCommand<boolean>('item.removeProjectItems', { itemIds: targetIds })
  if (!outcome.ok || outcome.result !== true) return false
  useSelectionStore.getState().clearSelection()
  return true
}

export function updateProjectItem(itemId: string, updates: Partial<TimelineItem>): boolean {
  if (!itemId) return false

  const outcome = runItemCommand<boolean>('item.updateProjectItem', {
    itemId,
    ...splitItemUpdates(updates),
  })
  return outcome.ok && outcome.result === true
}
