import { describe, expect, it } from 'vitest'
import type { CompositionItem, TextItem, TimelineItem } from '@editor/types/timeline'
import type { ItemRenderContext, ItemTransform, SubCompRenderData } from './canvas-item-renderer/types'
import { CaptionCoverageCollector, observeCaptionSurface } from './caption-coverage'
import { CanvasPool, TextMeasurementCache } from './canvas-pool'
import { renderItem } from './canvas-item-renderer/render-item'

const caption: TextItem = {
  id: 'caption', type: 'text', trackId: 'captions', label: 'Caption',
  from: 0, durationInFrames: 60, text: 'Ordinary karaoke caption', color: '#ffffff',
  fontSize: 24, fontFamily: 'sans-serif', captionPreset: 'karaoke',
  words: [
    { text: 'Ordinary', startFrame: 0, endFrame: 15 },
    { text: 'karaoke', startFrame: 15, endFrame: 30 },
    { text: 'caption', startFrame: 30, endFrame: 60 },
  ],
}

function context(): ItemRenderContext {
  // Seeding needs only composition structure; no canvas or GPU allocation.
  return {
    fps: 30, canvasSettings: { width: 640, height: 360, fps: 30 },
    subCompRenderData: new Map<string, SubCompRenderData>(),
    instanceSubCompRenderDataCache: new Map(), keyframesMap: new Map(),
  } as ItemRenderContext
}

function wrapper(id: string): CompositionItem {
  return { id, type: 'composition', label: id, trackId: 'outer', from: 0,
    durationInFrames: 60, compositionId: 'nested', compositionWidth: 320, compositionHeight: 180 }
}

describe('caption coverage export lifecycle', () => {
  it('seeds hidden, out-of-range and blank instances explicitly without allocating surfaces', () => {
    const collector = new CaptionCoverageCollector({ width: 640, height: 360, fps: 30 }, 1)
    const rctx = context()
    const tracks = [{ visible: false, items: [caption] }, { visible: true, items: [
      { ...caption, id: 'later', from: 900 }, { ...caption, id: 'blank', text: '', words: [] },
    ] }]
    const scope = collector.seed(tracks, rctx)
    expect([...scope.entries.keys()]).toEqual(['caption', 'later', 'blank'])
    collector.finishFrame(0)
    expect(collector.finalize()?.entries.map((entry) => [entry.itemId, entry.bounds]))
      .toEqual([['caption', null], ['later', null], ['blank', null]])
  })

  it('distinguishes repeated nested wrapper paths, including hidden subtracks', () => {
    const collector = new CaptionCoverageCollector({ width: 640, height: 360, fps: 30 }, 1)
    const rctx = context()
    rctx.subCompRenderData.set('nested', { fps: 30, durationInFrames: 60,
      sortedTracks: [{ order: 0, visible: false, items: [caption] }], keyframesMap: new Map() })
    collector.seed([{ items: [wrapper('first'), wrapper('second')] }], rctx)
    collector.finishFrame(0)
    expect(collector.finalize()?.entries).toEqual(['first', 'second'].map((id) => ({
      instancePath: [id], itemId: 'caption', space: { width: 320, height: 180 }, bounds: null,
    })))
  })

  it('never emits a successful manifest for partial renders or unsupported observations', () => {
    const partial = new CaptionCoverageCollector({ width: 640, height: 360, fps: 30 }, 2)
    partial.seed([{ items: [caption] }], context())
    partial.finishFrame(0)
    expect(partial.finalize()).toBeUndefined()
    expect(partial.failureReason).toMatch(/incomplete/)
    const unsupported = new CaptionCoverageCollector({ width: 640, height: 360, fps: 30 }, 1)
    unsupported.seed([{ items: [caption] }], context())
    unsupported.fail('GPU-only caption effect')
    unsupported.finishFrame(0)
    expect(unsupported.finalize()).toBeUndefined()
    expect(unsupported.failureReason).toBe('GPU-only caption effect')
  })

  it('rejects unresolved nested structure rather than reporting its captions empty', () => {
    const collector = new CaptionCoverageCollector({ width: 640, height: 360, fps: 30 }, 1)
    collector.seed([{ items: [wrapper('unresolved')] }], context())
    collector.finishFrame(0)
    expect(collector.finalize()).toBeUndefined()
    expect(collector.failureReason).toMatch(/cannot resolve composition/)
  })
})

// jsdom in this repository does not implement OffscreenCanvas. These tests use
// real Canvas2D when the runner supplies it, never a mocked text/alpha painter.
describe.runIf(typeof OffscreenCanvas !== 'undefined')('painted caption alpha coverage', () => {
  function renderContext(collector: CaptionCoverageCollector, items: TimelineItem[]): ItemRenderContext {
    const rctx = context()
    rctx.canvasPool = new CanvasPool(640, 360, 0)
    rctx.textMeasureCache = new TextMeasurementCache()
    rctx.renderMode = 'export'
    rctx.renderItem = renderItem
    rctx.captionCoverage = collector
    rctx.captionCoverageScope = collector.seed([{ items }], rctx)
    return rctx
  }

  it('unions real karaoke overflow, stroke, shadow and transformed ink without full-frame coverage', async () => {
    const frames = [0, 15, 30, 45]
    const collector = new CaptionCoverageCollector({ width: 640, height: 360, fps: 30 }, frames.length)
    const item: TextItem = { ...caption, textShadow: { color: '#000000', blur: 5, offsetX: 8, offsetY: 9 },
      stroke: { color: '#ff0000', width: 2 }, fontStyle: 'italic', letterSpacing: 2,
      captionPreset: 'word-highlight', fontSize: 36 }
    const rctx = renderContext(collector, [item])
    const source = new OffscreenCanvas(640, 360)
    const ctx = source.getContext('2d')!
    const transform: ItemTransform = { x: 0, y: 40, width: 160, height: 20,
      rotation: 12, opacity: 1, cornerRadius: 0 }
    const painted: Array<{ x: number; y: number }> = []
    try {
      for (const frame of frames) {
        ctx.clearRect(0, 0, 640, 360)
        await renderItem(ctx, item, transform, frame, rctx)
        const before = ctx.getImageData(0, 0, 640, 360).data
        observeCaptionSurface(rctx, item, source, [], rctx.canvasSettings)
        expect(ctx.getImageData(0, 0, 640, 360).data).toEqual(before)
        for (let y = 0; y < 360; y++) for (let x = 0; x < 640; x++) {
          if (before[(y * 640 + x) * 4 + 3]) painted.push({ x, y })
        }
        collector.finishFrame(frame)
      }
      const coverage = collector.finalize()!
      const bounds = coverage.entries[0]!.bounds!
      expect(painted.length).toBeGreaterThan(0)
      expect(bounds.width * bounds.height).toBeLessThan(640 * 360 / 2)
      expect(bounds.height).toBeGreaterThan(transform.height)
      for (const point of painted) {
        expect(point.x).toBeGreaterThanOrEqual(bounds.x)
        expect(point.x).toBeLessThan(bounds.x + bounds.width)
        expect(point.y).toBeGreaterThanOrEqual(bounds.y)
        expect(point.y).toBeLessThan(bounds.y + bounds.height)
      }
      expect(coverage.renderedFrames).toBe(frames.length)
    } finally {
      collector.dispose()
      rctx.canvasPool.dispose()
    }
  })

  it('observes actual auto-expanded static text rather than the authored text box', async () => {
    const collector = new CaptionCoverageCollector({ width: 640, height: 360, fps: 30 }, 1)
    const item: TextItem = { ...caption, words: [], text: 'First line\nSecond line\nThird line', fontSize: 30 }
    const rctx = renderContext(collector, [item])
    const source = new OffscreenCanvas(640, 360)
    try {
      await renderItem(source.getContext('2d')!, item,
        { x: 0, y: 0, width: 180, height: 10, rotation: 0, opacity: 1, cornerRadius: 0 }, 0, rctx)
      observeCaptionSurface(rctx, item, source, [], rctx.canvasSettings)
      collector.finishFrame(0)
      expect(collector.finalize()!.entries[0]!.bounds!.height).toBeGreaterThan(10)
    } finally {
      collector.dispose()
      rctx.canvasPool.dispose()
    }
  })

  it('releases finished surfaces and starts a discarded render retry fresh', () => {
    const source = new OffscreenCanvas(640, 360)
    const ctx = source.getContext('2d')!
    ctx.fillRect(10, 20, 30, 40)
    const first = new CaptionCoverageCollector({ width: 640, height: 360, fps: 30 }, 2)
    const firstScope = first.seed([{ items: [{ ...caption, durationInFrames: 1 }] }], context())
    first.observe(firstScope, caption.id, source)
    first.finishFrame(2)
    expect(firstScope.entries.get(caption.id)?.canvas).toBeUndefined()
    first.dispose()
    const retry = new CaptionCoverageCollector({ width: 640, height: 360, fps: 30 }, 1)
    const scope = retry.seed([{ items: [caption] }], context())
    ctx.clearRect(0, 0, 640, 360)
    ctx.fillRect(100, 120, 4, 5)
    retry.observe(scope, caption.id, source)
    retry.finishFrame(0)
    expect(retry.finalize()!.entries[0]!.bounds).toEqual({ x: 100, y: 120, width: 4, height: 5 })
    expect(scope.entries.get(caption.id)?.canvas).toBeUndefined()
  })

  it('isolates nested captions from an opaque background and keeps wrapper instances distinct', async () => {
    const collector = new CaptionCoverageCollector({ width: 640, height: 360, fps: 30 }, 1)
    const first = wrapper('first'), second = wrapper('second')
    const rctx = renderContext(collector, [])
    const child: TextItem = { ...caption, fontSize: 20,
      transform: { x: 0, y: 30, width: 240, height: 35 } }
    const background: TimelineItem = { id: 'background', type: 'shape', label: 'Background',
      trackId: 'background', from: 0, durationInFrames: 60, shapeType: 'rectangle',
      fillColor: '#225588', transform: { x: 0, y: 0, width: 320, height: 180 } }
    rctx.subCompRenderData.set('nested', { fps: 30, durationInFrames: 60,
      sortedTracks: [{ order: 1, visible: true, items: [background] },
        { order: 0, visible: true, items: [child] }], keyframesMap: new Map() })
    rctx.captionCoverageScope = collector.seed([{ items: [first, second] }], rctx)
    const source = new OffscreenCanvas(640, 360)
    try {
      for (const [index, item] of [first, second].entries()) {
        await renderItem(source.getContext('2d')!, item,
          { x: index ? 160 : -160, y: 0, width: 320, height: 180,
            rotation: 0, opacity: 1, cornerRadius: 0 }, 15, rctx)
      }
      collector.finishFrame(15)
      const entries = collector.finalize()!.entries
      expect(entries.map((entry) => entry.instancePath)).toEqual([['first'], ['second']])
      expect(entries[0]!.bounds).toEqual(entries[1]!.bounds)
      for (const entry of entries) {
        expect(entry.space).toEqual({ width: 320, height: 180 })
        expect(entry.bounds).not.toBeNull()
        expect(entry.bounds!.width * entry.bounds!.height).toBeLessThan(320 * 180 / 2)
      }
    } finally {
      collector.dispose()
      rctx.canvasPool.dispose()
    }
  })
})
