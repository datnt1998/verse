import { z } from 'zod'
import type { SerializedCommand } from '@editor/commands/types'
import type { ShapeItem, TextItem } from '@editor/types/timeline'

/**
 * "Subscribe" overlay preset: a rounded-rectangle backdrop, a play triangle and
 * a text label, built only from shape/text items plus keyframes — no Lottie or
 * third-party asset. The three items share one `linkedGroupId` so the editor
 * keeps them together while every item stays independently editable.
 */

/** ~0.4 s in + ~3 s hold + ~0.3 s out at any project fps. */
export const SUBSCRIBE_TIMING = { introSec: 0.4, holdSec: 3, outroSec: 0.3 } as const

export const subscribePresetContextSchema = z.object({
  idPrefix: z.string().min(1),
  /** Three video tracks, bottom→top: backdrop, play icon, label. */
  trackIds: z.tuple([z.string().min(1), z.string().min(1), z.string().min(1)]),
  from: z.number().int().nonnegative(),
  durationInFrames: z.number().int().positive(),
  fps: z.number().positive(),
  width: z.number().positive(),
  height: z.number().positive(),
  label: z.string().optional(),
  accentColor: z.string().regex(/^#[a-fA-F0-9]{6}$/).optional(),
  textColor: z.string().regex(/^#[a-fA-F0-9]{6}$/).optional(),
  x: z.number().finite().optional(),
  y: z.number().finite().optional(),
}).strict()
export type SubscribePresetContext = z.infer<typeof subscribePresetContextSchema>

export interface SubscribePresetResult {
  commands: SerializedCommand[]
  /** Order matches trackIds: backdrop, play icon, label. */
  itemIds: [string, string, string]
  groupId: string
  timing: { introFrames: number; holdFrames: number; outroFrames: number }
}
/** Split `durationInFrames` into intro/hold/outro; hold absorbs leftover frames and short clips clamp proportionally. */
export function subscribeTiming(durationInFrames: number, fps: number): { introFrames: number; holdFrames: number; outroFrames: number } {
  const intro = Math.max(1, Math.min(Math.round(SUBSCRIBE_TIMING.introSec * fps), Math.max(1, durationInFrames - 1)))
  const outro = Math.max(durationInFrames > intro ? 1 : 0, Math.min(Math.round(SUBSCRIBE_TIMING.outroSec * fps), durationInFrames - intro))
  const hold = Math.max(0, durationInFrames - intro - outro)
  return { introFrames: intro, holdFrames: hold, outroFrames: outro }
}

/** Geometry of the three preset items in canvas pixels. Exported for tests. */
export function subscribeGeometry(context: Pick<SubscribePresetContext, 'width' | 'height' | 'x' | 'y'>) {
  const buttonHeight = Math.round(context.height * 0.064)
  const buttonWidth = Math.round(context.width * 0.156)
  const margin = Math.round(context.width * 0.033)
  const x = context.x ?? context.width / 2 - margin - buttonWidth / 2
  const y = context.y ?? -(context.height / 2) + margin + buttonHeight / 2
  const pad = Math.round(buttonHeight * 0.32)
  return { buttonWidth, buttonHeight, margin, x, y, pad }
}

export function buildSubscribeCommands(context: SubscribePresetContext): SubscribePresetResult {
  const ctx = subscribePresetContextSchema.parse(context)
  const prefix = ctx.idPrefix
  const geo = subscribeGeometry(ctx)
  const timing = subscribeTiming(ctx.durationInFrames, ctx.fps)
  const accent = ctx.accentColor ?? '#cc3327'
  const labelColor = ctx.textColor ?? '#ffffff'
  const label = ctx.label ?? 'Subscribe'
  const groupId = `${prefix}-subscribe-group`
  const base = {
    from: ctx.from,
    durationInFrames: ctx.durationInFrames,
    linkedGroupId: groupId,
  }

  const backdrop: ShapeItem = {
    ...base,
    id: `${prefix}-subscribe-backdrop`,
    type: 'shape',
    shapeType: 'rectangle',
    label: 'Nền nút Subscribe',
    trackId: ctx.trackIds[0],
    fillColor: accent,
    cornerRadius: geo.buttonHeight / 2,
    transform: { x: geo.x, y: geo.y, width: geo.buttonWidth, height: geo.buttonHeight },
  }
  const iconSide = Math.round(geo.buttonHeight * 0.46)
  const icon: ShapeItem = {
    ...base,
    id: `${prefix}-subscribe-play`,
    type: 'shape',
    shapeType: 'triangle',
    direction: 'right',
    label: 'Biểu tượng phát',
    trackId: ctx.trackIds[1],
    fillColor: labelColor,
    transform: {
      x: geo.x - geo.buttonWidth / 2 + geo.pad + iconSide / 2,
      y: geo.y,
      width: iconSide,
      height: iconSide,
    },
  }
  const text: TextItem = {
    ...base,
    id: `${prefix}-subscribe-text`,
    type: 'text',
    label: 'Chữ Subscribe',
    trackId: ctx.trackIds[2],
    text: label,
    color: labelColor,
    fontSize: Math.round(geo.buttonHeight * 0.5),
    fontWeight: 'semibold',
    textAlign: 'center',
    verticalAlign: 'middle',
    transform: {
      x: geo.x + (geo.pad + iconSide) / 2,
      y: geo.y,
      width: geo.buttonWidth - geo.pad - iconSide,
      height: geo.buttonHeight,
    },
  }

  const commands: SerializedCommand[] = [
    { type: 'item.add', params: { item: backdrop } },
    { type: 'item.add', params: { item: icon } },
    { type: 'item.add', params: { item: text } },
  ]
  const { holdFrames: hold, outroFrames: outro } = timing
  const slide = geo.buttonWidth + geo.margin
  const last = ctx.durationInFrames - 1
  const intro = Math.min(timing.introFrames, last)
  const holdEnd = Math.min(last, ctx.durationInFrames - outro)
  for (const item of [backdrop, icon, text]) {
    const restX = item.transform!.x!
    const opacityKeys: [number, number][] = [[0, 0], [intro, 1]]
    if (hold > 0) opacityKeys.push([holdEnd, 1])
    opacityKeys.push([last, 0])
    const xKeys: [number, number][] = [[0, restX + slide], [intro, restX]]
    if (hold > 0) xKeys.push([holdEnd, restX])
    xKeys.push([last, restX])
    const addKey = (property: 'opacity' | 'x', keys: [number, number][]) => {
      const deduped: [number, number][] = []
      for (const key of keys) {
        const previous = deduped[deduped.length - 1]
        if (previous && previous[0] === key[0]) previous[1] = key[1]
        else deduped.push([...key])
      }
      for (const [frame, value] of deduped) {
        commands.push({ type: 'keyframe.add', params: { itemId: item.id, property, frame, value, easing: frame === intro ? 'ease-out' : 'linear' } })
      }
    }
    addKey('opacity', opacityKeys)
    addKey('x', xKeys)
  }
  return { commands, itemIds: [backdrop.id, icon.id, text.id], groupId, timing }
}
