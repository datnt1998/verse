import { z } from 'zod'
import type { TimelineItem } from '@editor/types/timeline'
import { BLEND_MODE_INDEX } from '@editor/types/blend-modes'
import { registerCommand } from '../registry'
import type { ApplyContext, CommandIssue, Inverse, ProjectStateView } from '../types'

export const visualEffectSchema = z.object({
  type: z.literal('gpu-effect'),
  gpuEffectType: z.string().min(1),
  params: z.record(z.string(), z.union([z.number().finite(), z.boolean(), z.string()])),
}).strict()

export const itemEffectSchema = z.object({
  id: z.string().min(1),
  effect: visualEffectSchema,
  enabled: z.boolean(),
  audioPulse: z.object({
    enabled: z.boolean(),
    beats: z.array(z.object({ frame: z.number().finite(), amplitude: z.number().finite() }).strict()),
    durationFrames: z.number().finite(),
    strength: z.number().finite(),
    chroma: z.number().finite(),
    glowColorBase: z.number().finite(),
  }).strict().optional(),
}).strict()

const id = z.string().min(1)
const itemParams = z.object({ itemId: id }).strict()
const effectParams = itemParams.extend({ effectId: id })
const emptyInverse = (): Inverse => ({ kind: 'patch', patch: {} })

function validateVisualItems(state: ProjectStateView, itemIds: string[]): CommandIssue[] {
  return itemIds.flatMap((itemId) => {
    const item = state.items.find((candidate) => candidate.id === itemId)
    return !item ? [{ message: `Không tìm thấy clip ${itemId}`, path: 'itemId' }]
      : item.type === 'audio' ? [{ message: 'Clip âm thanh không hỗ trợ hiệu ứng hình ảnh', path: 'itemId' }]
        : []
  })
}

function validateEffect(state: ProjectStateView, params: { itemId: string; effectId: string }): CommandIssue[] {
  const issues = validateVisualItems(state, [params.itemId])
  if (issues.length) return issues
  return state.items.find((item) => item.id === params.itemId)?.effects?.some((effect) => effect.id === params.effectId)
    ? [] : [{ message: `Không tìm thấy hiệu ứng ${params.effectId}`, path: 'effectId' }]
}

function updateVisualItem(ctx: ApplyContext, itemId: string, update: (item: TimelineItem) => TimelineItem): void {
  ctx.draft.items = ctx.draft.items.map((item) => item.id === itemId && item.type !== 'audio' ? update(item) : item)
}

export function registerEffectCommands(): void {
  registerCommand({
    type: 'effect.add',
    params: itemParams.extend({ effect: visualEffectSchema, effectId: id.optional() }),
    validate: (state, params) => validateVisualItems(state, [params.itemId]),
    apply(ctx, params) {
      const effectId = params.effectId ?? ctx.ids.mint()
      updateVisualItem(ctx, params.itemId, (item) => ({ ...item, effects: [...(item.effects ?? []), { id: effectId, effect: params.effect, enabled: true }] }))
      return { inverse: emptyInverse(), result: effectId }
    },
  })
  registerCommand({
    type: 'effect.addMany',
    params: z.object({ updates: z.array(z.object({ itemId: id, effects: z.array(visualEffectSchema) }).strict()) }).strict(),
    validate: (state, params) => validateVisualItems(state, params.updates.map((entry) => entry.itemId)),
    apply(ctx, params) {
      const updates = new Map(params.updates.map((entry) => [entry.itemId, entry.effects]))
      ctx.draft.items = ctx.draft.items.map((item) => {
        const effects = updates.get(item.id)
        return !effects || item.type === 'audio' ? item : { ...item, effects: [...(item.effects ?? []), ...effects.map((effect) => ({ id: ctx.ids.mint(), effect, enabled: true }))] }
      })
      return emptyInverse()
    },
  })
  registerCommand({
    type: 'effect.update',
    params: effectParams.extend({ updates: z.object({ effect: visualEffectSchema.optional(), enabled: z.boolean().optional() }).strict() }),
    validate: (state, params) => validateEffect(state, params),
    mergeKey: (params) => `effect:${params.itemId}:${params.effectId}:${[
      ...Object.keys(params.updates).filter((key) => key !== 'effect'),
      ...Object.keys(params.updates.effect?.params ?? {}).map((key) => `${params.updates.effect?.gpuEffectType}:${key}`),
    ].sort().join(',')}`,
    apply(ctx, params) {
      updateVisualItem(ctx, params.itemId, (item) => ({ ...item, effects: (item.effects ?? []).map((effect) => effect.id === params.effectId ? { ...effect, ...params.updates } : effect) }))
      return emptyInverse()
    },
  })
  registerCommand({
    type: 'effect.remove', params: effectParams, validate: validateEffect,
    apply(ctx, params) {
      updateVisualItem(ctx, params.itemId, (item) => ({ ...item, effects: (item.effects ?? []).filter((effect) => effect.id !== params.effectId) }))
      return emptyInverse()
    },
  })
  registerCommand({
    type: 'effect.toggle', params: effectParams, validate: validateEffect,
    apply(ctx, params) {
      updateVisualItem(ctx, params.itemId, (item) => ({ ...item, effects: (item.effects ?? []).map((effect) => effect.id === params.effectId ? { ...effect, enabled: !effect.enabled } : effect) }))
      return emptyInverse()
    },
  })
  registerCommand({
    type: 'effect.setItems',
    params: z.object({ updates: z.array(z.object({ itemId: id, effects: z.array(itemEffectSchema) }).strict()) }).strict(),
    validate: (state, params) => validateVisualItems(state, params.updates.map((entry) => entry.itemId)),
    apply(ctx, params) {
      const updates = new Map(params.updates.map((entry) => [entry.itemId, entry.effects]))
      ctx.draft.items = ctx.draft.items.map((item) => updates.has(item.id) && item.type !== 'audio' ? { ...item, effects: updates.get(item.id) } : item)
      return emptyInverse()
    },
  })
  registerCommand({
    type: 'effect.setBlendMode',
    params: itemParams.extend({ blendMode: z.enum(Object.keys(BLEND_MODE_INDEX) as [keyof typeof BLEND_MODE_INDEX, ...(keyof typeof BLEND_MODE_INDEX)[]]) }),
    validate: (state, params) => validateVisualItems(state, [params.itemId]),
    apply(ctx, params) {
      updateVisualItem(ctx, params.itemId, (item) => ({ ...item, blendMode: item.type === 'shape' && item.isMask ? 'normal' : params.blendMode }))
      return emptyInverse()
    },
  })
}
