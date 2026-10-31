import type { TimelineItem } from '@editor/types/timeline'
import type { RenderedCaptionCoverage } from '@editor/types/export-caption-coverage'
import type { ItemRenderContext, CanvasSettings } from './canvas-item-renderer/types'
import { resolveSubCompRenderDataForInstance } from './canvas-item-renderer/composition-instance'
import { getRenderTimelineSourceStart } from './render-span'
import { applyMasks, type MaskCanvasSettings } from './canvas-masks'
import type { EffectSourceMask } from './canvas-effects'

type CoverageEntry = RenderedCaptionCoverage['entries'][number]
interface Accumulation {
  entry: CoverageEntry
  expiresAfter: number
  canvas?: OffscreenCanvas
  ctx?: OffscreenCanvasRenderingContext2D
}

/** One scope per wrapper instance, including repeated uses of the same subcomposition. */
export interface CaptionCoverageScope {
  entries: Map<string, Accumulation>
  children: Map<string, CaptionCoverageScope>
  hasCaptions: boolean
}

export class CaptionCoverageCollector {
  private readonly entries: Accumulation[] = []
  private readonly active = new Set<Accumulation>()
  private renderedFrames = 0
  private failure?: string
  private finished = false

  constructor(
    private readonly output: RenderedCaptionCoverage['output'],
    private readonly expectedFrames: number,
  ) {}

  get failureReason(): string | undefined { return this.failure }

  seed(tracks: Array<{ items?: TimelineItem[] }>, rctx: ItemRenderContext): CaptionCoverageScope {
    // The affine clock is used only for releasing finished accumulation surfaces.
    // Propagate half-frame rounding uncertainty at every nested clock conversion;
    // the renderer remains the sole authority on which pixels are observed.
    const visit = (
      items: TimelineItem[], path: string[], width: number, height: number,
      fps: number, rate: number, offset: number, error: number,
      ancestorEnd: number, ancestors: Set<string>,
    ): CaptionCoverageScope => {
      const scope: CaptionCoverageScope = { entries: new Map(), children: new Map(), hasCaptions: false }
      for (const item of items) {
        const end = rate > 0
          ? (item.from + item.durationInFrames - offset + error) / rate
          : rate < 0 ? (item.from - offset - error) / rate : ancestorEnd
        const expiresAfter = Math.min(ancestorEnd, end)
        if (item.type === 'text' || item.type === 'subtitle') {
          if (this.entries.length >= 10_000) {
            this.fail('Caption coverage exceeds the instance limit')
            break
          }
          const accumulation: Accumulation = {
            entry: { instancePath: path, itemId: item.id, space: { width, height }, bounds: null },
            expiresAfter,
          }
          scope.entries.set(item.id, accumulation)
          this.entries.push(accumulation)
        } else if (item.type === 'composition') {
          if (path.length >= 32 || ancestors.has(item.compositionId)) {
            this.fail('Caption coverage encountered cyclic or excessively deep compositions')
            continue
          }
          const data = resolveSubCompRenderDataForInstance(item, rctx)
          if (!data) {
            this.fail(`Caption coverage cannot resolve composition ${item.compositionId}`)
            continue
          }
          const speed = (item.speed ?? 1) * (item.sourceFps ?? data.fps) / fps
          if (!Number.isFinite(speed)) {
            this.fail('Caption coverage encountered an invalid composition clock')
            continue
          }
          const nextAncestors = new Set(ancestors)
          nextAncestors.add(item.compositionId)
          scope.children.set(item.id, visit(
            data.sortedTracks.flatMap((track) => track.items), [...path, item.id],
            item.compositionWidth, item.compositionHeight, data.fps,
            rate * speed, getRenderTimelineSourceStart(item) + (offset - item.from) * speed,
            error * Math.abs(speed) + 0.5, expiresAfter, nextAncestors,
          ))
        }
      }
      scope.hasCaptions = scope.entries.size > 0
      if (!scope.hasCaptions) {
        for (const child of scope.children.values()) {
          if (child.hasCaptions) {
            scope.hasCaptions = true
            break
          }
        }
      }
      return scope
    }
    return visit(tracks.flatMap((track) => track.items ?? []), [],
      rctx.canvasSettings.logicalWidth ?? rctx.canvasSettings.width,
      rctx.canvasSettings.logicalHeight ?? rctx.canvasSettings.height,
      rctx.fps, 1, 0, 0, Infinity, new Set())
  }

  observe(scope: CaptionCoverageScope | undefined, itemId: string, source: OffscreenCanvas): void {
    if (this.failure || this.finished) return
    const accumulation = scope?.entries.get(itemId)
    if (!accumulation) {
      this.fail(`Caption coverage did not seed rendered item ${itemId}`)
      return
    }
    try {
      if (!accumulation.canvas) {
        const canvas = new OffscreenCanvas(source.width, source.height)
        const ctx = canvas.getContext('2d')
        if (!ctx) {
          canvas.width = 0
          canvas.height = 0
          throw new Error('Cannot create caption coverage canvas')
        }
        accumulation.canvas = canvas
        accumulation.ctx = ctx
        this.active.add(accumulation)
      }
      if (accumulation.canvas.width !== source.width || accumulation.canvas.height !== source.height) {
        throw new Error('Caption coverage surface dimensions changed during export')
      }
      // This context is private and always has identity transform, source-over,
      // full opacity and no filters/shadows. Never read pixels in the frame loop.
      accumulation.ctx!.drawImage(source, 0, 0)
    } catch (error) {
      this.fail(error instanceof Error ? error.message : 'Cannot observe caption coverage')
    }
  }

  finishFrame(rootFrame: number): void {
    if (this.failure || this.finished) return
    this.renderedFrames++
    for (const accumulation of this.active) {
      if (rootFrame > accumulation.expiresAfter) this.flush(accumulation)
    }
  }

  finalize(): RenderedCaptionCoverage | undefined {
    if (this.finished) return undefined
    if (this.renderedFrames !== this.expectedFrames) this.fail('Caption coverage render is incomplete')
    for (const accumulation of this.active) this.flush(accumulation)
    this.finished = true
    if (this.failure) return undefined
    return { version: 1, output: this.output, renderedFrames: this.renderedFrames,
      entries: this.entries.map((accumulation) => accumulation.entry) }
  }

  fail(reason: string): void {
    this.failure ??= reason
    this.dispose()
  }

  dispose(): void {
    for (const accumulation of this.active) this.release(accumulation)
    this.finished = true
  }

  private release(accumulation: Accumulation): void {
    if (accumulation.canvas) {
      accumulation.canvas.width = 0
      accumulation.canvas.height = 0
    }
    accumulation.canvas = undefined
    accumulation.ctx = undefined
    this.active.delete(accumulation)
  }

  private flush(accumulation: Accumulation): void {
    const { canvas, ctx, entry } = accumulation
    if (!canvas || !ctx) return
    try {
      const { data } = ctx.getImageData(0, 0, canvas.width, canvas.height)
      let left = canvas.width, top = canvas.height, right = -1, bottom = -1
      for (let y = 0; y < canvas.height; y++) {
        for (let x = 0; x < canvas.width; x++) {
          if (data[(y * canvas.width + x) * 4 + 3] === 0) continue
          left = Math.min(left, x)
          top = Math.min(top, y)
          right = Math.max(right, x)
          bottom = Math.max(bottom, y)
        }
      }
      if (right >= left && bottom >= top) {
        const sx = entry.space.width / canvas.width
        const sy = entry.space.height / canvas.height
        const previous = entry.bounds
        const x = Math.min(previous?.x ?? Infinity, left * sx)
        const y = Math.min(previous?.y ?? Infinity, top * sy)
        entry.bounds = { x, y,
          width: Math.max(previous ? previous.x + previous.width : -Infinity, (right + 1) * sx) - x,
          height: Math.max(previous ? previous.y + previous.height : -Infinity, (bottom + 1) * sy) - y }
      }
    } catch (error) {
      this.fail(error instanceof Error ? error.message : 'Cannot read caption coverage alpha')
    } finally {
      this.release(accumulation)
    }
  }
}

/** Observe only an isolated caption surface, after the same final masks as compositing. */
export function observeCaptionSurface(
  rctx: ItemRenderContext, item: TimelineItem, source: OffscreenCanvas,
  masks: EffectSourceMask[], settings: CanvasSettings & MaskCanvasSettings,
): void {
  const collector = rctx.captionCoverage
  if (!collector || collector.failureReason || (item.type !== 'text' && item.type !== 'subtitle')) return
  if (masks.length === 0) {
    collector.observe(rctx.captionCoverageScope, item.id, source)
    return
  }
  const { canvas, ctx } = rctx.canvasPool.acquire()
  try {
    canvas.width = source.width
    canvas.height = source.height
    applyMasks(ctx, source, masks, settings)
    collector.observe(rctx.captionCoverageScope, item.id, canvas)
  } catch (error) {
    collector.fail(error instanceof Error ? error.message : 'Cannot observe masked caption coverage')
  } finally {
    rctx.canvasPool.release(canvas)
  }
}
