import type { ShapeItem } from '@editor/types/timeline'
import { getEnvelopeBins, getWaveformGeometryBuffer } from '@editor/features/waveform-visualizer/envelope'
import { waveformReadiness, waveformSourceSeconds, writeWaveformGeometry } from '@editor/features/waveform-visualizer/geometry'
import type { RenderTimelineSpan } from '../render-span'
import type { ItemRenderContext, ItemTransform } from './types'

export function renderWaveformVisualizer(ctx: OffscreenCanvasRenderingContext2D, item: ShapeItem, transform: ItemTransform, frame: number, rctx: ItemRenderContext, sourceFrameOffset = 0, span?: RenderTimelineSpan): void {
  const payload = item.waveformVisualizer!
  const error = waveformReadiness(item)
  const left = rctx.canvasSettings.width / 2 + transform.x - transform.width / 2
  const top = rctx.canvasSettings.height / 2 + transform.y - transform.height / 2
  if (error) {
    if (rctx.renderMode === 'export') throw new Error(error)
    ctx.fillStyle = '#a1a1aa'
    ctx.font = '14px sans-serif'
    ctx.fillText(error, left + 8, top + transform.height / 2, Math.max(0, transform.width - 16))
    return
  }
  if (!payload.visible) return
  const bins = getEnvelopeBins(payload)!
  const geometry = getWaveformGeometryBuffer(payload)
  writeWaveformGeometry(bins, payload, waveformSourceSeconds(item, frame, rctx.canvasSettings.fps, sourceFrameOffset, span), transform.width, transform.height, geometry)
  const scale = rctx.canvasSettings.width / (rctx.canvasSettings.logicalWidth ?? rctx.canvasSettings.width)
  ctx.strokeStyle = payload.color
  ctx.lineWidth = payload.strokeWidth * scale
  ctx.lineCap = 'round'
  ctx.beginPath()
  for (let i = 0; i < payload.bars; i++) {
    ctx.moveTo(left + geometry[i * 3]!, top + geometry[i * 3 + 1]!)
    ctx.lineTo(left + geometry[i * 3]!, top + geometry[i * 3 + 2]!)
  }
  ctx.stroke()
}
