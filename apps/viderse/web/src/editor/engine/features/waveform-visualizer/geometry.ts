import type { ShapeItem } from '@editor/types/timeline'
import type { RenderTimelineSpan } from '@editor/features/export/utils/render-span'
import { getSourceFrameRampOffset } from '@editor/features/export/utils/render-span'
import type { WaveformVisualizer } from './types'
import { getEnvelopeBins } from './envelope'

export function waveformSourceSeconds(item: ShapeItem, frame: number, fps: number, sourceFrameOffset = 0, span?: RenderTimelineSpan): number {
  const sourceFps = item.sourceFps ?? fps
  const speed = item.speed ?? 1
  const start = span?.sourceStart ?? item.sourceStart ?? item.trimStart ?? 0
  const local = frame - (span?.from ?? item.from)
  if (item.isReversed) {
    const end = item.sourceEnd ?? start + item.durationInFrames / fps * speed * sourceFps
    return (end - sourceFrameOffset - 1) / sourceFps - local / fps * speed
  }
  const ramp = span?.sourceTimeRamp ? getSourceFrameRampOffset(span.sourceTimeRamp, frame) : 0
  return (start + sourceFrameOffset + ramp) / sourceFps + local / fps * speed
}

/** Writes x/top/bottom triples into a caller-owned small buffer. SVG and canvas
 * use identical geometry; empty source windows are a real flat baseline. */
export function writeWaveformGeometry(bins: Uint8Array, payload: WaveformVisualizer, seconds: number, width: number, height: number, output: Float32Array): void {
  const center = height * payload.baseline
  const halfHeight = Math.min(center, height - center) * payload.amplitude
  for (let bar = 0; bar < payload.bars; bar++) {
    const at = seconds + ((bar + 0.5) / payload.bars - 0.5) * payload.windowSeconds
    const index = Math.floor(at * payload.samplesPerSecond + 1e-8)
    const magnitude = index < 0 || index >= bins.length ? 0 : bins[index]! / 255
    const amplitude = Math.min(Math.min(center, height - center), magnitude * halfHeight)
    output[bar * 3] = (bar + 0.5) * width / payload.bars
    output[bar * 3 + 1] = center - amplitude
    output[bar * 3 + 2] = center + amplitude
  }
}

export function waveformReadiness(item: ShapeItem, currentHash?: string): string | null {
  const payload = item.waveformVisualizer
  if (!payload) return null
  if (!item.mediaId || (currentHash !== undefined && currentHash !== payload.sourceHash)) return 'Sóng giọng đã cũ. Tạo lại từ nguồn giọng hiện tại.'
  if (!getEnvelopeBins(payload)) return 'Thiếu dữ liệu sóng giọng. Tạo lại từ nguồn giọng.'
  return null
}
