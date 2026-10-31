import { z } from 'zod'
import type { TimelineItem, TimelineTrack } from '@editor/types/timeline'
import { registerCommand } from '../registry'
import type { ApplyOutcome, CommandIssue, ProjectStateView } from '../types'
import {
  applyRippleTrimMutation,
  applyRollingTrimMutation,
  applySlideMutation,
  applySlipMutation,
  applyTrimMutation,
  type AffectedIds,
} from '../../features/timeline/stores/actions/edit/trim-mutations'
import {
  applySplitAllMutation,
  applySplitMultiMutation,
  applySplitMutation,
  getSplitBlockReason,
} from '../../features/timeline/stores/actions/edit/split-mutations'
import { applyJoinMutation } from '../../features/timeline/stores/actions/edit/join-mutations'
import {
  applyRateStretchMutation,
  applyResetSpeedMutation,
} from '../../features/timeline/stores/actions/edit/rate-stretch-mutations'
import { applyRemoveRangesMutation } from '../../features/timeline/stores/actions/edit/range-removal-mutations'
import {
  applyFreezeFrameMutation,
  getFreezeFrameBlockReason,
} from '../../features/timeline/stores/actions/edit/freeze-frame-mutations'
import {
  applySourceInsertMutation,
  applySourceOverwriteMutation,
} from '../../features/timeline/stores/actions/source-edit-mutations'
import { applyLegacyAvRepairMutation } from '../../features/timeline/stores/actions/legacy-av-mutations'

const id = z.string().min(1)
const frame = z.number().finite()
const ids = z.array(id)
const linkedSelection = z.boolean()
const handle = z.enum(['start', 'end'])

/**
 * Edit mutations read and write the canonical stores directly (Freecut `_` mutators),
 * so the inverse is the empty patch that the executor auto-diffs against the
 * canonical state touched by `apply`.
 */
function recorded(result: unknown): ApplyOutcome {
  return { inverse: { kind: 'patch', patch: {} }, result }
}

function recordedAffected(affectedIds: AffectedIds): ApplyOutcome {
  return recorded(affectedIds ? { affectedIds } : null)
}

function missingItem(state: ProjectStateView, itemId: string, path: string): CommandIssue[] {
  return state.items.some((item) => item.id === itemId)
    ? []
    : [{ message: `Không tìm thấy clip ${itemId}`, code: 'missing-item', path }]
}

const trimBase = {
  id,
  amount: frame,
  itemIds: ids.optional(),
  linkedSelection,
}

const trimStartParams = z.object({ ...trimBase, handle: z.literal('start') }).strict()
const trimEndParams = z.object({ ...trimBase, handle: z.literal('end') }).strict()
const trimBreakingParams = z
  .object({ ...trimBase, handle, removeTransitionIds: ids })
  .strict()
type TrimParams = z.infer<typeof trimBreakingParams>

function registerTrimCommands(): void {
  registerCommand<z.infer<typeof trimStartParams>>({
    type: 'edit.trimItemStart',
    params: trimStartParams,
    validate: (state, params) => missingItem(state, params.id, 'id'),
    apply: (_ctx, params) => recordedAffected(applyTrimMutation(params)),
  })
  registerCommand<z.infer<typeof trimEndParams>>({
    type: 'edit.trimItemEnd',
    params: trimEndParams,
    validate: (state, params) => missingItem(state, params.id, 'id'),
    apply: (_ctx, params) => recordedAffected(applyTrimMutation(params)),
  })
  registerCommand<TrimParams>({
    type: 'edit.trimItemBreakingTransition',
    params: trimBreakingParams,
    validate: (state, params) => {
      const known = new Set(state.transitions.map((transition) => transition.id))
      return [
        ...missingItem(state, params.id, 'id'),
        ...params.removeTransitionIds
          .filter((transitionId) => !known.has(transitionId))
          .map((transitionId) => ({
            message: `Không tìm thấy transition ${transitionId}`,
            code: 'missing-transition',
            path: 'removeTransitionIds',
          })),
      ]
    },
    apply: (_ctx, params) => recordedAffected(applyTrimMutation(params)),
  })

  const rippleParams = z.object({ id, handle, delta: frame, linkedSelection }).strict()
  registerCommand<z.infer<typeof rippleParams>>({
    type: 'edit.rippleTrim',
    params: rippleParams,
    validate: (state, params) => missingItem(state, params.id, 'id'),
    apply: (_ctx, params) => recordedAffected(applyRippleTrimMutation(params)),
  })

  const rollingParams = z.object({ leftId: id, rightId: id, delta: frame, linkedSelection }).strict()
  registerCommand<z.infer<typeof rollingParams>>({
    type: 'edit.rollingTrim',
    params: rollingParams,
    validate: (state, params) => [
      ...missingItem(state, params.leftId, 'leftId'),
      ...missingItem(state, params.rightId, 'rightId'),
    ],
    apply: (_ctx, params) => recordedAffected(applyRollingTrimMutation(params)),
  })

  const slipParams = z.object({ id, delta: frame, linkedSelection }).strict()
  registerCommand<z.infer<typeof slipParams>>({
    type: 'edit.slip',
    params: slipParams,
    validate: (state, params) => missingItem(state, params.id, 'id'),
    apply: (_ctx, params) => recordedAffected(applySlipMutation(params)),
  })

  const slideParams = z
    .object({
      id,
      delta: frame,
      leftNeighborId: id.nullable(),
      rightNeighborId: id.nullable(),
      linkedSelection,
    })
    .strict()
  registerCommand<z.infer<typeof slideParams>>({
    type: 'edit.slide',
    params: slideParams,
    validate: (state, params) => missingItem(state, params.id, 'id'),
    apply: (_ctx, params) => recordedAffected(applySlideMutation(params)),
  })
}

function registerSplitCommands(): void {
  const splitParams = z.object({ id, frame, linkedSelection }).strict()
  registerCommand<z.infer<typeof splitParams>>({
    type: 'edit.splitItem',
    params: splitParams,
    validate: (state, params) => {
      const reason = getSplitBlockReason(
        state.items,
        state.transitions,
        params.id,
        params.frame,
        params.linkedSelection,
      )
      if (!reason) return []
      const message =
        reason === 'transition-overlap'
          ? 'Không thể cắt bên trong vùng chuyển cảnh'
          : reason === 'out-of-range'
            ? 'Điểm cắt nằm ngoài clip'
            : `Không tìm thấy clip ${params.id}`
      return [{ message, code: reason, path: 'frame' }]
    },
    apply: (_ctx, params) => recorded(applySplitMutation(params)),
  })

  const splitAllParams = z.object({ frame, linkedSelection }).strict()
  registerCommand<z.infer<typeof splitAllParams>>({
    type: 'edit.splitAllAtFrame',
    params: splitAllParams,
    apply: (_ctx, params) => recorded(applySplitAllMutation(params)),
  })

  const splitMultiParams = z
    .object({ id, frames: z.array(frame).min(1), linkedSelection })
    .strict()
  registerCommand<z.infer<typeof splitMultiParams>>({
    type: 'edit.splitItemAtFrames',
    params: splitMultiParams,
    validate: (state, params) => missingItem(state, params.id, 'id'),
    apply: (_ctx, params) => recorded(applySplitMultiMutation(params)),
  })

  const joinParams = z.object({ itemIds: ids.min(2), linkedSelection }).strict()
  registerCommand<z.infer<typeof joinParams>>({
    type: 'edit.joinItems',
    params: joinParams,
    apply: (_ctx, params) => recordedAffected(applyJoinMutation(params)),
  })
}

function registerSpeedCommands(): void {
  const rateStretchParams = z
    .object({
      id,
      newFrom: frame,
      newDuration: frame,
      newSpeed: frame.positive(),
      linkedSelection,
      /** Consecutive commands sharing a key merge into one undo entry (live drags). */
      mergeKey: z.string().min(1).optional(),
    })
    .strict()
  registerCommand<z.infer<typeof rateStretchParams>>({
    type: 'edit.rateStretch',
    params: rateStretchParams,
    validate: (state, params) => missingItem(state, params.id, 'id'),
    apply: (_ctx, params) => recordedAffected(applyRateStretchMutation(params)),
    mergeKey: (params) => params.mergeKey ?? null,
  })

  const resetSpeedParams = z.object({ itemIds: ids, linkedSelection }).strict()
  registerCommand<z.infer<typeof resetSpeedParams>>({
    type: 'edit.resetSpeedWithRipple',
    params: resetSpeedParams,
    apply: (_ctx, params) => recordedAffected(applyResetSpeedMutation(params)),
  })
}

const range = z.object({ start: frame, end: frame }).strict()
const removeRangesParams = z
  .object({
    itemIds: ids,
    rangesByMediaId: z.record(z.string(), z.array(range)),
    linkedSelection,
  })
  .strict()

function registerRangeRemovalCommands(): void {
  for (const type of [
    'edit.removeSilence',
    'edit.removeFillerWords',
    'edit.removeTranscriptRanges',
  ]) {
    registerCommand<z.infer<typeof removeRangesParams>>({
      type,
      params: removeRangesParams,
      apply: (_ctx, params) => recorded(applyRemoveRangesMutation(params)),
    })
  }
}

function registerFreezeFrameCommand(): void {
  const freezeParams = z
    .object({
      itemId: id,
      playheadFrame: frame,
      freezeDurationFrames: frame.min(1),
      image: z
        .object({
          label: z.string(),
          mediaId: id,
          src: z.string().min(1),
          sourceWidth: frame.positive(),
          sourceHeight: frame.positive(),
        })
        .strict(),
    })
    .strict()
  registerCommand<z.infer<typeof freezeParams>>({
    type: 'edit.insertFreezeFrame',
    params: freezeParams,
    validate: (state, params) => {
      const reason = getFreezeFrameBlockReason(
        state.items,
        state.transitions,
        params.itemId,
        params.playheadFrame,
      )
      return reason
        ? [{ message: `Không thể chèn khung hình tĩnh (${reason})`, code: reason, path: 'playheadFrame' }]
        : []
    },
    apply: (_ctx, params) => recorded(applyFreezeFrameMutation(params)),
  })
}

const timelineItemSchema = z.custom<TimelineItem>(
  (value) => {
    if (typeof value !== 'object' || value === null) return false
    const item = value as Record<string, unknown>
    return (
      typeof item.id === 'string' &&
      item.id.length > 0 &&
      typeof item.type === 'string' &&
      typeof item.trackId === 'string' &&
      typeof item.from === 'number' &&
      Number.isFinite(item.from) &&
      typeof item.durationInFrames === 'number' &&
      Number.isFinite(item.durationInFrames) &&
      item.durationInFrames > 0
    )
  },
  { message: 'Clip không hợp lệ' },
)

const timelineTrackSchema = z.custom<TimelineTrack>(
  (value) =>
    typeof value === 'object' &&
    value !== null &&
    typeof (value as Record<string, unknown>).id === 'string',
  { message: 'Track không hợp lệ' },
)

/** Shared guard for source insert/overwrite: unique ids, known tracks, sane source bounds. */
function validateSourceEditItems(
  state: ProjectStateView,
  tracks: TimelineTrack[],
  items: TimelineItem[],
): CommandIssue[] {
  const issues: CommandIssue[] = []
  const trackIds = new Set(tracks.map((track) => track.id))
  const existingIds = new Set(state.items.map((item) => item.id))
  const seen = new Set<string>()
  for (const item of items) {
    if (existingIds.has(item.id) || seen.has(item.id)) {
      issues.push({ message: `Clip ${item.id} đã tồn tại`, code: 'duplicate-id', path: 'items' })
    }
    seen.add(item.id)
    if (!trackIds.has(item.trackId)) {
      issues.push({ message: `Không tìm thấy track ${item.trackId}`, code: 'missing-track', path: 'items' })
    }
    if (item.type === 'video' || item.type === 'audio') {
      const { sourceStart, sourceEnd } = item
      if (
        (sourceStart !== undefined && (!Number.isFinite(sourceStart) || sourceStart < 0)) ||
        (sourceEnd !== undefined &&
          (!Number.isFinite(sourceEnd) || sourceEnd <= (sourceStart ?? 0)))
      ) {
        issues.push({ message: `Giới hạn nguồn của clip ${item.id} không hợp lệ`, code: 'source-bounds', path: 'items' })
      }
    }
  }
  return issues
}

function registerSourceEditCommands(): void {
  const insertParams = z
    .object({
      insertFrame: frame.min(0),
      clipDurationFrames: frame.min(1),
      tracks: z.array(timelineTrackSchema),
      items: z.array(timelineItemSchema).min(1),
    })
    .strict()
  registerCommand<z.infer<typeof insertParams>>({
    type: 'edit.sourceInsert',
    params: insertParams,
    validate: (state, params) => validateSourceEditItems(state, params.tracks, params.items),
    apply: (_ctx, params) => recorded(applySourceInsertMutation(params)),
  })

  const overwriteParams = z
    .object({
      overwriteStart: frame.min(0),
      overwriteEnd: frame.min(1),
      tracks: z.array(timelineTrackSchema),
      items: z.array(timelineItemSchema).min(1),
    })
    .strict()
  registerCommand<z.infer<typeof overwriteParams>>({
    type: 'edit.sourceOverwrite',
    params: overwriteParams,
    validate: (state, params) => [
      ...(params.overwriteEnd > params.overwriteStart
        ? []
        : [{ message: 'Vùng ghi đè phải có độ dài dương', code: 'empty-range', path: 'overwriteEnd' }]),
      ...validateSourceEditItems(state, params.tracks, params.items),
    ],
    apply: (_ctx, params) => recorded(applySourceOverwriteMutation(params)),
  })

  const repairParams = z.object({ videoHasAudioByMediaId: z.record(z.string(), z.boolean()) }).strict()
  registerCommand<z.infer<typeof repairParams>>({
    type: 'edit.repairLegacyAvTracks',
    params: repairParams,
    apply: (_ctx, params) => recorded(applyLegacyAvRepairMutation(params)),
  })
}

export function registerEditCommands(): void {
  registerTrimCommands()
  registerSplitCommands()
  registerSpeedCommands()
  registerRangeRemovalCommands()
  registerFreezeFrameCommand()
  registerSourceEditCommands()
}
