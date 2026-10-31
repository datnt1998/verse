import { z } from 'zod'
import type { TimelineItem } from '@editor/types/timeline'
import { normalizeFrameFields } from '@editor/features/timeline/stores/items-store-normalize'
import { registerCommand } from '../registry'
import type { ApplyContext, CommandIssue, Inverse, ProjectStateView } from '../types'

const number = z.number().finite()
const id = z.string().min(1)
const textInlineStyleSchema = z.object({
  fontSize: number.optional(), fontFamily: z.string().optional(),
  fontWeight: z.enum(['normal', 'medium', 'semibold', 'bold']).optional(),
  fontStyle: z.enum(['normal', 'italic']).optional(),
  underline: z.boolean().optional(), color: z.string().optional(), letterSpacing: number.optional(),
}).strict()
const textSpanSchema = textInlineStyleSchema.extend({ text: z.string() })
export const textStyleSchema = textInlineStyleSchema.extend({
  backgroundColor: z.string().optional(), backgroundRadius: number.optional(),
  textAlign: z.enum(['left', 'center', 'right']).optional(),
  verticalAlign: z.enum(['top', 'middle', 'bottom']).optional(),
  lineHeight: number.optional(), textPadding: number.optional(),
  textShadow: z.object({ offsetX: number, offsetY: number, blur: number, color: z.string() }).strict().optional(),
  stroke: z.object({ width: number, color: z.string() }).strict().optional(),
})
export const textPatchSchema = textStyleSchema.extend({
  text: z.string().optional(), textSpans: z.array(textSpanSchema).optional(),
  spanLayout: z.enum(['stack', 'inline']).optional(),
  textLayoutDrafts: z.object({ single: textSpanSchema.optional(), twoSpans: z.array(textSpanSchema).optional(), threeSpans: z.array(textSpanSchema).optional() }).strict().optional(),
  textStylePresetId: z.string().optional(), textStyleScale: number.optional(),
})
const vector = z.tuple([number, number])
export const shapePatchSchema = z.object({
  shapeType: z.enum(['rectangle', 'circle', 'triangle', 'ellipse', 'star', 'polygon', 'heart', 'path']).optional(),
  fillColor: z.string().optional(), fillEnabled: z.boolean().optional(), fillType: z.enum(['solid', 'linear']).optional(),
  gradientStartColor: z.string().optional(), gradientEndColor: z.string().optional(), gradientAngle: number.optional(),
  strokeColor: z.string().optional(), strokeWidth: number.optional(), strokeEnabled: z.boolean().optional(),
  strokeLineCap: z.enum(['butt', 'round', 'square']).optional(), strokeLineJoin: z.enum(['miter', 'round', 'bevel']).optional(), strokeMiterLimit: number.optional(),
  trimPathStart: number.optional(), trimPathEnd: number.optional(), trimPathOffset: number.optional(),
  taperStartWidth: number.optional(), taperEndWidth: number.optional(), taperStartLength: number.optional(), taperEndLength: number.optional(),
  cornerRadius: number.optional(), direction: z.enum(['up', 'down', 'left', 'right']).optional(), points: number.optional(), innerRadius: number.optional(),
  pathVertices: z.array(z.object({ position: vector, inHandle: vector, outHandle: vector, tangentMode: z.enum(['corner', 'smooth', 'continuous', 'broken']).optional() }).strict()).optional(),
  pathClosed: z.boolean().optional(),
  isMask: z.boolean().optional(), maskType: z.enum(['clip', 'alpha']).optional(), maskFeather: number.optional(), maskOpacity: number.optional(), maskInvert: z.boolean().optional(),
}).strict()
export const lottiePatchSchema = z.object({
  frameRate: number.optional(), totalFrames: number.optional(), loop: z.boolean().optional(),
  reversed: z.boolean().optional(), loopMode: z.enum(['loop', 'pingpong']).optional(),
  speed: number.optional(), segmentStart: number.optional(), segmentEnd: number.optional(),
  animationId: z.string().optional(), themeId: z.string().optional(),
  textOverrides: z.record(z.string(), z.string()).optional(), colorOverrides: z.record(z.string(), z.string()).optional(),
  slotOverrides: z.record(z.string(), z.union([number, vector])).optional(),
}).strict()

function validateKind(state: ProjectStateView, itemId: string, kind: TimelineItem['type']): CommandIssue[] {
  const item = state.items.find((candidate) => candidate.id === itemId)
  return !item ? [{ message: `Không tìm thấy clip ${itemId}`, path: 'itemId' }]
    : item.type !== kind ? [{ message: `Clip không phải loại ${kind}`, path: 'itemId' }] : []
}
function update(ctx: ApplyContext, itemId: string, updates: object, clearFields: string[]): Inverse {
  const patch: Record<string, unknown> = { ...updates }
  for (const key of clearFields) patch[key] = undefined
  ctx.draft.items = ctx.draft.items.map((item) => item.id === itemId ? normalizeFrameFields({ ...item, ...patch } as TimelineItem) : item)
  return { kind: 'patch', patch: {} }
}
function optionalKeys<T extends z.ZodRawShape>(schema: z.ZodObject<T>, required: readonly string[]): [string, ...string[]] {
  return Object.keys(schema.shape).filter((key) => !required.includes(key)) as [string, ...string[]]
}

export function registerTextCommands(): void {
  registerCommand({
    type: 'text.update',
    params: z.object({ itemId: id, updates: textPatchSchema, clearFields: z.array(z.enum(optionalKeys(textPatchSchema, ['text', 'color']))).default([]) }).strict(),
    validate: (state, params) => validateKind(state, params.itemId, 'text'),
    mergeKey: (params) => `text:${params.itemId}:${[...Object.keys(params.updates), ...params.clearFields].sort().join(',')}`,
    apply: (ctx, params) => update(ctx, params.itemId, params.updates, params.clearFields),
  })
  registerCommand({
    type: 'text.updateShape',
    params: z.object({ itemId: id, updates: shapePatchSchema, clearFields: z.array(z.enum(optionalKeys(shapePatchSchema, ['shapeType', 'fillColor']))).default([]) }).strict(),
    validate: (state, params) => validateKind(state, params.itemId, 'shape'),
    mergeKey: (params) => `shape:${params.itemId}:${[...Object.keys(params.updates), ...params.clearFields].sort().join(',')}`,
    apply: (ctx, params) => update(ctx, params.itemId, params.updates, params.clearFields),
  })
  registerCommand({
    type: 'text.updateLottie',
    params: z.object({ itemId: id, updates: lottiePatchSchema, clearFields: z.array(z.enum(optionalKeys(lottiePatchSchema, ['frameRate', 'totalFrames']))).default([]) }).strict(),
    validate: (state, params) => validateKind(state, params.itemId, 'lottie'),
    mergeKey: (params) => `lottie:${params.itemId}:${[...Object.keys(params.updates), ...params.clearFields].sort().join(',')}`,
    apply: (ctx, params) => update(ctx, params.itemId, params.updates, params.clearFields),
  })
}
