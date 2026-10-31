/**
 * Source-monitor insert / overwrite mutations. Run inside the `edit.sourceInsert` /
 * `edit.sourceOverwrite` command handlers with fully-resolved params (tracks and new
 * items are built by the async wrapper in `source-edit-actions.ts`).
 */

import type { TimelineItem, TimelineTrack } from '@editor/types/timeline'
import { useItemsStore } from '../items-store'
import { applyTransitionRepairs } from './shared'

export interface SourceInsertMutationParams {
  insertFrame: number
  clipDurationFrames: number
  tracks: TimelineTrack[]
  items: TimelineItem[]
}

export interface SourceOverwriteMutationParams {
  overwriteStart: number
  overwriteEnd: number
  tracks: TimelineTrack[]
  items: TimelineItem[]
}

export function applySourceInsertMutation(params: SourceInsertMutationParams): string[] {
  const { insertFrame, clipDurationFrames, tracks, items: newItems } = params
  const store = useItemsStore.getState()
  store.setTracks(tracks)
  const targetTrackIds = Array.from(new Set(newItems.map((item) => item.trackId)))
  const splitIds: string[] = []
  const shiftedIds: string[] = []

  for (const targetTrackId of targetTrackIds) {
    const straddleItem = useItemsStore
      .getState()
      .items.find(
        (item) =>
          item.trackId === targetTrackId &&
          item.from < insertFrame &&
          item.from + item.durationInFrames > insertFrame,
      )

    if (straddleItem) {
      const splitResult = store._splitItem(straddleItem.id, insertFrame)
      if (splitResult) {
        splitIds.push(splitResult.leftItem.id, splitResult.rightItem.id)
      }
    }

    const itemsToShift = useItemsStore
      .getState()
      .items.filter((item) => item.trackId === targetTrackId && item.from >= insertFrame)
    for (const item of itemsToShift) {
      store._moveItem(item.id, item.from + clipDurationFrames)
      shiftedIds.push(item.id)
    }
  }

  for (const newItem of newItems) {
    store._addItem(newItem)
  }

  const affectedIds = [...newItems.map((item) => item.id), ...shiftedIds, ...splitIds]
  applyTransitionRepairs(affectedIds)
  return affectedIds
}

export function applySourceOverwriteMutation(params: SourceOverwriteMutationParams): string[] {
  const { overwriteStart, overwriteEnd, tracks, items: newItems } = params
  const store = useItemsStore.getState()
  store.setTracks(tracks)
  const targetTrackIds = Array.from(new Set(newItems.map((item) => item.trackId)))
  const affectedIds: string[] = []

  for (const targetTrackId of targetTrackIds) {
    const overlapping = useItemsStore
      .getState()
      .items.filter(
        (item) =>
          item.trackId === targetTrackId &&
          item.from < overwriteEnd &&
          item.from + item.durationInFrames > overwriteStart,
      )

    for (const item of overlapping) {
      const itemEnd = item.from + item.durationInFrames
      const startsBeforeRegion = item.from < overwriteStart
      const endsAfterRegion = itemEnd > overwriteEnd

      if (!startsBeforeRegion && !endsAfterRegion) {
        store._removeItems([item.id])
      } else if (startsBeforeRegion && endsAfterRegion) {
        const splitResult = store._splitItem(item.id, overwriteStart)
        if (splitResult) {
          affectedIds.push(splitResult.leftItem.id)
          const splitResult2 = useItemsStore
            .getState()
            ._splitItem(splitResult.rightItem.id, overwriteEnd)
          if (splitResult2) {
            store._removeItems([splitResult2.leftItem.id])
            affectedIds.push(splitResult2.rightItem.id)
          }
        }
      } else if (startsBeforeRegion) {
        const splitResult = store._splitItem(item.id, overwriteStart)
        if (splitResult) {
          store._removeItems([splitResult.rightItem.id])
          affectedIds.push(splitResult.leftItem.id)
        }
      } else {
        const splitResult = store._splitItem(item.id, overwriteEnd)
        if (splitResult) {
          store._removeItems([splitResult.leftItem.id])
          affectedIds.push(splitResult.rightItem.id)
        }
      }
    }
  }

  for (const newItem of newItems) {
    store._addItem(newItem)
    affectedIds.push(newItem.id)
  }

  applyTransitionRepairs(affectedIds)
  return affectedIds
}
