import { z } from 'zod'
import { registerCommand } from '../registry'
import type { ApplyContext, CommandIssue, ProjectStateView } from '../types'
import type { AudioFxEffect } from '@editor/types/audio'
import { audioFxEffectSchema, audioFxParamsPatchSchema, noiseProfileSchema } from '@editor/features/audio-fx/schema'

const targetSchema = z.union([
  z.object({ itemId: z.string().min(1) }).strict(),
  z.object({ trackId: z.string().min(1) }).strict(),
])
type Target = z.infer<typeof targetSchema>

function effects(state: ProjectStateView, target: Target): AudioFxEffect[] | undefined {
  return 'itemId' in target
    ? state.items.find(item => item.id === target.itemId)?.audioFx
    : state.tracks.find(track => track.id === target.trackId)?.audioFx
}
function targetIssues(state: ProjectStateView, target: Target, effectId?: string): CommandIssue[] {
  const owner = 'itemId' in target
    ? state.items.find(item => item.id === target.itemId)
    : state.tracks.find(track => track.id === target.trackId)
  if (!owner) return [{ code: 'not-found', message: 'Không tìm thấy clip hoặc lớp âm thanh' }]
  if ('itemId' in target && 'type' in owner && owner.type !== 'audio' && owner.type !== 'video') {
    return [{ code: 'invalid-target', message: 'Hiệu ứng âm thanh chỉ áp dụng cho clip âm thanh hoặc video' }]
  }
  if (effectId && !effects(state, target)?.some(effect => effect.id === effectId)) {
    return [{ code: 'not-found', message: 'Không tìm thấy hiệu ứng âm thanh' }]
  }
  return []
}
function assign(ctx: ApplyContext, target: Target, audioFx: AudioFxEffect[]): void {
  if ('itemId' in target) {
    ctx.draft.items = ctx.draft.items.map(item => item.id === target.itemId ? { ...item, audioFx } : item)
  } else {
    ctx.draft.tracks = ctx.draft.tracks.map(track => track.id === target.trackId ? { ...track, audioFx } : track)
  }
}
const inverse = () => ({ kind: 'patch' as const, patch: {} })

export function registerAudioFxCommands(): void {
  registerCommand({
    type: 'audioFx.add',
    params: z.object({ target: targetSchema, effect: audioFxEffectSchema }).strict(),
    validate(state, params) {
      const issues = targetIssues(state, params.target)
      if (effects(state, params.target)?.some(effect => effect.id === params.effect.id)) {
        issues.push({ code: 'duplicate-id', message: 'Mã hiệu ứng âm thanh đã tồn tại' })
      }
      return issues
    },
    apply(ctx, params) {
      assign(ctx, params.target, [...(effects(ctx.draft, params.target) ?? []), params.effect])
      return inverse()
    },
  })
  registerCommand({
    type: 'audioFx.remove',
    params: z.object({ target: targetSchema, effectId: z.string().min(1) }).strict(),
    validate: (state, params) => targetIssues(state, params.target, params.effectId),
    apply(ctx, params) {
      assign(ctx, params.target, effects(ctx.draft, params.target)!.filter(effect => effect.id !== params.effectId))
      return inverse()
    },
  })
  registerCommand({
    type: 'audioFx.update',
    params: z.object({
      target: targetSchema, effectId: z.string().min(1),
      params: audioFxParamsPatchSchema.optional(), enabled: z.boolean().optional(),
    }).strict(),
    validate(state, params) {
      const issues = targetIssues(state, params.target, params.effectId)
      const current = effects(state, params.target)?.find(effect => effect.id === params.effectId)
      if (!current) return issues
      const parsed = audioFxEffectSchema.safeParse({
        ...current, enabled: params.enabled ?? current.enabled, params: { ...current.params, ...params.params },
      })
      if (!parsed.success) issues.push(...parsed.error.issues.map(issue => ({ code: 'invalid-effect', message: issue.message })))
      return issues
    },
    apply(ctx, params) {
      assign(ctx, params.target, effects(ctx.draft, params.target)!.map(effect => effect.id !== params.effectId ? effect
        : audioFxEffectSchema.parse({ ...effect, enabled: params.enabled ?? effect.enabled, params: { ...effect.params, ...params.params } })))
      return inverse()
    },
    mergeKey: params => `audio-fx:${JSON.stringify(params.target)}:${params.effectId}`,
  })
  registerCommand({
    type: 'audioFx.reorder',
    params: z.object({ target: targetSchema, effectIds: z.array(z.string().min(1)) }).strict(),
    validate(state, params) {
      const issues = targetIssues(state, params.target)
      const current = effects(state, params.target) ?? []
      if (params.effectIds.length !== current.length || new Set(params.effectIds).size !== current.length ||
          params.effectIds.some(id => !current.some(effect => effect.id === id))) {
        issues.push({ code: 'invalid-order', message: 'Thứ tự phải chứa mỗi hiệu ứng đúng một lần' })
      }
      return issues
    },
    apply(ctx, params) {
      const current = effects(ctx.draft, params.target)!
      assign(ctx, params.target, params.effectIds.map(id => current.find(effect => effect.id === id)!))
      return inverse()
    },
  })
  registerCommand({
    type: 'audioFx.learnNoiseProfile',
    params: z.object({
      target: targetSchema, effectId: z.string().min(1), profile: noiseProfileSchema,
      range: z.object({ startSeconds: z.number().finite().nonnegative(), endSeconds: z.number().finite().positive() })
        .strict().refine(range => range.endSeconds > range.startSeconds),
    }).strict(),
    validate(state, params) {
      const issues = targetIssues(state, params.target, params.effectId)
      const current = effects(state, params.target)?.find(effect => effect.id === params.effectId)
      if (current && current.type !== 'noiseReduction') {
        issues.push({ code: 'invalid-effect', message: 'Mẫu nhiễu chỉ áp dụng cho hiệu ứng giảm nhiễu' })
      }
      if ((params.range.endSeconds - params.range.startSeconds) * params.profile.sampleRate < params.profile.fftSize) {
        issues.push({ code: 'invalid-range', message: 'Khoảng nhiễu phải chứa ít nhất một khung FFT' })
      }
      return issues
    },
    apply(ctx, params) {
      assign(ctx, params.target, effects(ctx.draft, params.target)!.map(effect => effect.id === params.effectId && effect.type === 'noiseReduction'
        ? { ...effect, params: { ...effect.params, profile: params.profile } } : effect))
      return inverse()
    },
  })
}
