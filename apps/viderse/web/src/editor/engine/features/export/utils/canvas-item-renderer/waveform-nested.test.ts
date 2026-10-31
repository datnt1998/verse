import { afterEach, describe, expect, it, vi } from 'vitest'
import type { CompositionItem } from '@editor/types/timeline'
import { WaveformTestCanvas, voiceShape, voiceTransform, waveformRenderContext } from '@editor/test/waveform-fixture'
import { prepareGpuMediaParticipant } from './gpu'
import { renderItem } from './render-item'
import { getItemRenderTimelineSpan } from '../render-span'

afterEach(() => vi.unstubAllGlobals())
describe('nested composition waveform frames', () => {
  it('rejects a GPU composition containing a waveform and rasterizes matching local frames', async () => {
    vi.stubGlobal('GPUTextureUsage', { TEXTURE_BINDING: 4, RENDER_ATTACHMENT: 16, COPY_DST: 2 })
    const child = voiceShape(), context = waveformRenderContext()
    context.fps = context.canvasSettings.fps = 60
    const wrapper: CompositionItem = { id: 'wrapper', type: 'composition', label: 'Nested voice', trackId: 'outer', compositionId: 'nested', compositionWidth: 128, compositionHeight: 64, from: 10, durationInFrames: 90, sourceStart: 15, sourceFps: 30, speed: 1, transform: child.transform }
    const track = { order: 0, visible: true, muted: true, items: [child] }
    context.subCompRenderData.set('nested', { fps: 30, durationInFrames: 120, sortedTracks: [track], keyframesMap: new Map(), itemsById: new Map([[child.id, child]]) })
    const destroy = vi.fn()
    context.gpuPipeline = { getDevice: () => ({ createTexture: () => ({ destroy }) }) } as unknown as NonNullable<typeof context.gpuPipeline>
    context.gpuMediaPipeline = {} as NonNullable<typeof context.gpuMediaPipeline>
    context.gpuShapePipeline = {} as NonNullable<typeof context.gpuShapePipeline>
    for (const frame of [10, 70, 100]) {
      expect(await prepareGpuMediaParticipant({ item: wrapper, transform: voiceTransform, effects: [], renderSpan: getItemRenderTimelineSpan(wrapper) }, frame, context)).toBeNull()
      const nested = new WaveformTestCanvas(), direct = new WaveformTestCanvas()
      await renderItem(nested.getContext(), wrapper, voiceTransform, frame, context)
      const local = (frame - 10) / 2 + 15
      const directContext = waveformRenderContext()
      await renderItem(direct.getContext(), child, voiceTransform, local, directContext)
      // Exact pixel alpha masks (0 px tolerance) under the deterministic line raster.
      expect(nested.pixels).toEqual(direct.pixels)
    }
    expect(destroy).toHaveBeenCalledTimes(3)
    track.visible = false
    const hidden = new WaveformTestCanvas()
    await renderItem(hidden.getContext(), wrapper, voiceTransform, 70, context)
    expect(hidden.pixels.some(value => value !== 0)).toBe(false)
  })
})
