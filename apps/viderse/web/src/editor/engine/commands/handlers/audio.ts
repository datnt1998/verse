import { z } from 'zod'
import type { AudioEqSettings } from '@editor/types/audio'
import type { TimelineItem } from '@editor/types/timeline'
import { useItemsStore } from '@editor/features/timeline/stores/items-store'
import { normalizeFrameFields } from '@editor/features/timeline/stores/items-store-normalize'
import { registerCommand } from '../registry'
import type { CommandIssue, Inverse, ProjectStateView } from '../types'

const number = z.number().finite()
const flag = z.boolean().optional()
const value = number.optional()
const slope = z.union([z.literal(6), z.literal(12), z.literal(18), z.literal(24)]).optional()
const innerType = z.enum(['low-shelf', 'peaking', 'high-shelf', 'notch']).optional()
export const audioEqSchema = z.object({
  enabled: flag, outputGainDb: value,
  band1Enabled: flag, band1Type: z.enum(['low-shelf', 'peaking', 'high-shelf', 'high-pass']).optional(),
  band1FrequencyHz: value, band1GainDb: value, band1Q: value, band1SlopeDbPerOct: slope,
  lowCutEnabled: flag, lowCutFrequencyHz: value, lowCutSlopeDbPerOct: slope,
  lowEnabled: flag, lowType: innerType, lowGainDb: value, lowFrequencyHz: value, lowQ: value,
  lowMidEnabled: flag, lowMidType: innerType, lowMidGainDb: value, lowMidFrequencyHz: value, lowMidQ: value,
  midGainDb: value,
  highMidEnabled: flag, highMidType: innerType, highMidGainDb: value, highMidFrequencyHz: value, highMidQ: value,
  highEnabled: flag, highType: innerType, highGainDb: value, highFrequencyHz: value, highQ: value,
  band6Enabled: flag, band6Type: z.enum(['low-pass', 'low-shelf', 'peaking', 'high-shelf']).optional(),
  band6FrequencyHz: value, band6GainDb: value, band6Q: value, band6SlopeDbPerOct: slope,
  highCutEnabled: flag, highCutFrequencyHz: value, highCutSlopeDbPerOct: slope,
}).strict()

const clipEqShape = Object.fromEntries(Object.entries(audioEqSchema.shape).map(([key, schema]) => [
  `audioEq${key[0]!.toUpperCase()}${key.slice(1)}`, schema,
]))
export const clipAudioPatchSchema = z.object({
  ...clipEqShape,
  volume: value,
  audioFadeIn: value, audioFadeOut: value,
  audioFadeInCurve: value, audioFadeOutCurve: value,
  audioFadeInCurveX: value, audioFadeOutCurveX: value,
  audioPitchSemitones: value, audioPitchCents: value,
  embeddedAudioMuted: flag,
  audioDucking: z.object({
    duckOthersDb: number,
    attackSec: value,
    releaseSec: value,
    targetTrackIds: z.array(z.string().min(1)).optional(),
  }).strict().nullable().optional(),
}).strict()

const id = z.string().min(1)
const inverse = (): Inverse => ({ kind: 'patch', patch: {} })
function validateClip(state: ProjectStateView, params: { itemId: string }): CommandIssue[] {
  const item = state.items.find((candidate) => candidate.id === params.itemId)
  return !item ? [{ message: `Không tìm thấy clip ${params.itemId}`, path: 'itemId' }]
    : item.type !== 'audio' && item.type !== 'video' && item.type !== 'composition'
      ? [{ message: 'Clip này không có âm thanh', path: 'itemId' }] : []
}
function validateTrack(state: ProjectStateView, params: { trackId: string }): CommandIssue[] {
  return state.tracks.some((track) => track.id === params.trackId) ? [] : [{ message: `Không tìm thấy rãnh ${params.trackId}`, path: 'trackId' }]
}
function eqFields(eq: AudioEqSettings | null): Partial<TimelineItem> {
  return Object.fromEntries(Object.keys(audioEqSchema.shape).map((key) => [
    `audioEq${key[0]!.toUpperCase()}${key.slice(1)}`, eq?.[key as keyof AudioEqSettings],
  ])) as Partial<TimelineItem>
}

export function registerAudioCommands(): void {
  registerCommand({
    type: 'audio.setClip',
    params: z.object({ itemId: id, updates: clipAudioPatchSchema }).strict(),
    validate: (state, params) => validateClip(state, params),
    mergeKey: (params) => `audio:clip:${params.itemId}:${Object.keys(params.updates).sort().join(',')}`,
    apply(ctx, params) {
      // The patch is a union of per-type fields; index access needs a record view.
      const updates = { ...params.updates } as Record<string, unknown>
      if ('audioDucking' in params.updates) updates.audioDucking = params.updates.audioDucking ?? undefined
      // Looping is picture-only; unmuting the embedded audio stops the loop.
      if (updates.embeddedAudioMuted === false) updates.loop = undefined
      ctx.draft.items = ctx.draft.items.map((item) => item.id === params.itemId ? normalizeFrameFields({ ...item, ...updates } as TimelineItem) : item)
      return inverse()
    },
  })
  registerCommand({
    type: 'audio.setClipEq',
    params: z.object({ itemId: id, eq: audioEqSchema.nullable() }).strict(),
    validate: (state, params) => validateClip(state, params),
    mergeKey: (params) => `audio:clip-eq:${params.itemId}`,
    apply(ctx, params) {
      const patch = eqFields(params.eq)
      ctx.draft.items = ctx.draft.items.map((item) => item.id === params.itemId ? normalizeFrameFields({ ...item, ...patch } as TimelineItem) : item)
      return inverse()
    },
  })
  registerCommand({
    type: 'audio.setTrackVolume',
    params: z.object({ trackId: id, volume: number }).strict(),
    validate: (state, params) => validateTrack(state, params),
    mergeKey: (params) => `audio:track-volume:${params.trackId}`,
    apply(ctx, params) {
      useItemsStore.getState().setTracks(ctx.draft.tracks.map((track) => track.id === params.trackId ? { ...track, volume: params.volume } : track))
      return inverse()
    },
  })
  registerCommand({
    type: 'audio.setTrackMute',
    params: z.object({ trackId: id, muted: z.boolean() }).strict(), validate: (state, params) => validateTrack(state, params),
    apply(ctx, params) {
      useItemsStore.getState().setTracks(ctx.draft.tracks.map((track) => track.id === params.trackId ? { ...track, muted: params.muted } : track))
      return inverse()
    },
  })
  registerCommand({
    type: 'audio.setTrackSolo',
    params: z.object({ trackId: id, solo: z.boolean() }).strict(), validate: (state, params) => validateTrack(state, params),
    apply(ctx, params) {
      useItemsStore.getState().setTracks(ctx.draft.tracks.map((track) => track.id === params.trackId ? { ...track, solo: params.solo } : track))
      return inverse()
    },
  })
  registerCommand({
    type: 'audio.setTrackEq',
    params: z.object({ trackId: id, eq: audioEqSchema.nullable() }).strict(), validate: (state, params) => validateTrack(state, params),
    mergeKey: (params) => `audio:track-eq:${params.trackId}`,
    apply(ctx, params) {
      useItemsStore.getState().setTracks(ctx.draft.tracks.map((track) => track.id === params.trackId ? { ...track, audioEq: params.eq ?? undefined } : track))
      return inverse()
    },
  })
  registerCommand({
    type: 'audio.setBusEq', params: z.object({ eq: audioEqSchema.nullable() }).strict(),
    mergeKey: () => 'audio:bus-eq',
    apply(ctx, params) { ctx.draft.busAudioEq = params.eq ?? undefined; return inverse() },
  })
  registerCommand({
    type: 'audio.setMasterGain', params: z.object({ db: number }).strict(),
    mergeKey: () => 'audio:master-gain',
    apply(ctx, params) { ctx.draft.masterBusDb = Math.max(-60, Math.min(12, params.db)); return inverse() },
  })
}
