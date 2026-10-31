import type { ShapeItem } from '@editor/types/timeline'
import type { ItemRenderContext, ItemTransform } from '@editor/features/export/utils/canvas-item-renderer/types'
import { DEFAULT_WAVEFORM_STYLE } from '@editor/features/waveform-visualizer/types'
import { encodeEnvelope } from '@editor/features/waveform-visualizer/envelope'
import { renderItem } from '@editor/features/export/utils/canvas-item-renderer/render-item'

export const VOICE_HASH = `sha256:${'a'.repeat(64)}`
export function voiceShape(overrides: Partial<ShapeItem> = {}): ShapeItem {
  const bins = Uint8Array.from({ length: 200 }, (_, i) => i < 50 ? 0 : i < 100 ? 255 : i % 8 * 30)
  return { id: 'voice', type: 'shape', shapeType: 'rectangle', label: 'Sóng giọng', trackId: 'visual', mediaId: 'voice-audio', from: 0, durationInFrames: 120, sourceFps: 30, sourceStart: 0, sourceEnd: 120, fillColor: '#ffffff', transform: { width: 128, height: 64, x: 0, y: 0 }, waveformVisualizer: { version: 1, ...DEFAULT_WAVEFORM_STYLE, bars: 16, strokeWidth: 2, sourceHash: VOICE_HASH, samplesPerSecond: 50, durationSeconds: 4, normalization: 0.8, envelopeBase64: encodeEnvelope(bins) }, ...overrides }
}
export const voiceTransform: ItemTransform = { x: 0, y: 0, width: 128, height: 64, opacity: 1, rotation: 0, cornerRadius: 0 }

/** A deterministic coverage raster for line geometry, not a browser/GPU mock.
 * Frame assertions use exact 8-bit alpha masks; real GPU/MP4 evidence is separate. */
export class WaveformTestCanvas {
  width = 128
  height = 64
  pixels = new Uint8Array(this.width * this.height)
  segments: number[][] = []
  lineWidth = 2
  globalAlpha = 1
  globalCompositeOperation = 'source-over'
  fillStyle = ''
  strokeStyle = ''
  lineCap = 'round'
  font = ''
  private point = [0, 0]
  private stack: { alpha: number; width: number }[] = []
  getContext() { return this as unknown as OffscreenCanvasRenderingContext2D }
  save() { this.stack.push({ alpha: this.globalAlpha, width: this.lineWidth }) }
  restore() { const saved = this.stack.pop(); if (saved) { this.globalAlpha = saved.alpha; this.lineWidth = saved.width } }
  translate() {}
  rotate() {}
  scale() {}
  setTransform() {}
  resetTransform() {}
  beginPath() { this.segments = [] }
  rect() {}
  clip() {}
  moveTo(x: number, y: number) { this.point = [x, y] }
  lineTo(x: number, y: number) { this.segments.push([...this.point, x, y]); this.point = [x, y] }
  clearRect() { this.pixels = new Uint8Array(this.width * this.height); this.segments = [] }
  stroke() {
    for (const [x1, y1, x2, y2] of this.segments) {
      const dx = x2! - x1!, dy = y2! - y1!, squared = dx * dx + dy * dy
      for (let y = 0; y < this.height; y++) for (let x = 0; x < this.width; x++) {
        const t = squared ? Math.min(1, Math.max(0, ((x + 0.5 - x1!) * dx + (y + 0.5 - y1!) * dy) / squared)) : 0
        if (Math.hypot(x + 0.5 - x1! - t * dx, y + 0.5 - y1! - t * dy) <= this.lineWidth / 2) this.pixels[y * this.width + x] = Math.round(255 * this.globalAlpha)
      }
    }
  }
  drawImage(source: WaveformTestCanvas) { for (let i = 0; i < this.pixels.length; i++) this.pixels[i] = Math.max(this.pixels[i]!, source.pixels[i] ?? 0) }
  fillText() {}
  fillRect() {}
}
export function waveformRenderContext(): ItemRenderContext {
  const context = {
    fps: 30, canvasSettings: { width: 128, height: 64, fps: 30 }, renderMode: 'export', renderItem,
    canvasPool: { acquire: () => { const canvas = new WaveformTestCanvas(); return { canvas, ctx: canvas.getContext() } }, release: () => {} },
    keyframesMap: new Map(), subCompRenderData: new Map(), adjustmentLayers: [], imageElements: new Map(), gifFramesMap: new Map(), videoExtractors: new Map(), useMediabunny: new Set(), mediabunnyDisabledItems: new Set(),
  }
  return context as unknown as ItemRenderContext
}
