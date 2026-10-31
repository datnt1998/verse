/**
 * Browser-side helpers for item edit actions: link-selection toggle, post-edit warm-frame
 * scheduling and the post-command finish step. Internal to the edit/ subdirectory.
 */

import type { TimelineItem } from '@editor/types/timeline'
import { useEditorStore } from '@editor/shared/state/editor'
import { usePlaybackStore } from '@editor/shared/state/playback'
import { usePreviewBridgeStore } from '@editor/shared/state/preview-bridge'
import { useItemsStore } from '../../items-store'
import { useTimelineSettingsStore } from '../../timeline-settings-store'

export function isLinkedSelectionEnabled(): boolean {
  return useEditorStore.getState().linkedSelectionEnabled
}

const POST_EDIT_WARM_MAX_FRAMES = 32

function appendWarmFrame(target: number[], seen: Set<number>, frame: number): void {
  if (!Number.isFinite(frame)) return
  const normalizedFrame = Math.max(0, Math.round(frame))
  if (seen.has(normalizedFrame)) return
  seen.add(normalizedFrame)
  target.push(normalizedFrame)
}

function appendItemWarmFrames(
  target: number[],
  seen: Set<number>,
  item: TimelineItem | undefined,
): void {
  if (!item) return
  const startFrame = Math.max(0, Math.trunc(item.from))
  const endFrame = Math.max(startFrame, Math.trunc(item.from + item.durationInFrames) - 1)
  appendWarmFrame(target, seen, startFrame)
  appendWarmFrame(target, seen, Math.min(endFrame, startFrame + 1))
  appendWarmFrame(target, seen, Math.max(startFrame, endFrame - 1))
  appendWarmFrame(target, seen, endFrame)
}

function collectPostEditWarmFrames(
  itemIds: Iterable<string>,
  preferredFrames: number[] = [],
): number[] {
  const frames: number[] = []
  const seen = new Set<number>()

  for (const frame of preferredFrames) {
    appendWarmFrame(frames, seen, frame)
  }

  const itemById = useItemsStore.getState().itemById
  for (const itemId of itemIds) {
    appendItemWarmFrames(frames, seen, itemById[itemId])
    if (frames.length >= POST_EDIT_WARM_MAX_FRAMES) {
      break
    }
  }

  return frames.slice(0, POST_EDIT_WARM_MAX_FRAMES)
}

export function requestPostEditWarmForItems(
  itemIds: Iterable<string>,
  preferredFrames: number[] = [],
): void {
  const playbackState = usePlaybackStore.getState()
  if (playbackState.isPlaying) return

  const uniqueItemIds = Array.from(new Set(itemIds))
  if (uniqueItemIds.length === 0) return

  const primaryFrame = playbackState.currentFrame
  const warmFrames = collectPostEditWarmFrames(uniqueItemIds, [primaryFrame, ...preferredFrames])
  usePreviewBridgeStore.getState().requestPostEditWarm(primaryFrame, uniqueItemIds, warmFrames)
}

/**
 * Post-command side effects shared by every edit wrapper: schedule the preview
 * warm-up for the touched items and flag the project dirty. Runs after
 * `executeCommands`, never inside a command handler.
 */
export function finishEdit(affectedIds: Iterable<string>, preferredFrames: number[] = []): void {
  requestPostEditWarmForItems(affectedIds, preferredFrames)
  useTimelineSettingsStore.getState().markDirty()
}
