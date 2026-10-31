import { describe, expect, it, vi } from 'vitest'
import { WaveformTestCanvas, voiceShape, waveformRenderContext } from '@editor/test/waveform-fixture'
import type { ActiveTransition } from '../canvas-transitions'
import { renderTransitionToGpuTexture, resolveTransitionParticipantRenderState } from './transition'
import { renderItem } from './render-item'

describe('transition waveform GPU-to-canvas participants', () => {
  it('rasterizes animated participants before GPU effects/compositing at start, midpoint and end', async () => {
    const left = voiceShape({ id: 'left', durationInFrames: 60, sourceEnd: 60, effects: [{ id: 'voice-blur', enabled: true, effect: { type: 'gpu-effect', gpuEffectType: 'gpu-blur', params: { amount: 2 } } }] })
    const right = voiceShape({ id: 'right', from: 60, durationInFrames: 60, sourceStart: 60, sourceEnd: 120 })
    const active: ActiveTransition = { transition: { id: 'fade', type: 'crossfade', presentation: 'fade', timing: 'linear', leftClipId: left.id, rightClipId: right.id, trackId: 'visual', durationInFrames: 20 }, leftClip: left, rightClip: right, progress: 0.5, transitionStart: 50, transitionEnd: 70, durationInFrames: 20, leftPortion: 10, rightPortion: 10, cutPoint: 60 }
    const context = waveformRenderContext()
    const uploaded: Uint8Array[] = []
    const uploadedEffects: unknown[][] = []
    const upload = vi.fn((canvas: WaveformTestCanvas, effects: unknown[]) => { uploaded.push(canvas.pixels.slice()); uploadedEffects.push(effects); return true })
    context.gpuPipeline = { applyEffectsToTexture: upload } as unknown as NonNullable<typeof context.gpuPipeline>
    context.gpuShapePipeline = { render: vi.fn(() => { throw new Error('waveform must not draw GPU rectangle') }) } as unknown as NonNullable<typeof context.gpuShapePipeline>
    const composite = vi.fn(() => true)
    context.gpuTransitionPipeline = { has: () => true, renderTexturesToTexture: composite } as unknown as NonNullable<typeof context.gpuTransitionPipeline>
    const textures = { acquire: () => ({} as GPUTexture), release: vi.fn() }
    for (const frame of [50, 60, 69]) {
      uploaded.length = 0
      uploadedEffects.length = 0
      expect(await renderTransitionToGpuTexture({} as GPUTexture, active, frame, context, 0, textures)).toBe(true)
      expect(uploaded).toHaveLength(2)
      expect(uploadedEffects[0]).toEqual([expect.objectContaining({ id: 'voice-blur', type: 'gpu-blur', params: { amount: 2 } })])
      expect(uploadedEffects[1]).toEqual([])
      for (const [index, item] of [left, right].entries()) {
        const participant = resolveTransitionParticipantRenderState(item, active, frame, 0, context)
        const expected = new WaveformTestCanvas()
        await renderItem(expected.getContext(), item, participant.transform, frame, waveformRenderContext(), 0, participant.renderSpan)
        expect(uploaded[index]).toEqual(expected.pixels)
      }
    }
    expect(composite).toHaveBeenCalledTimes(3)
    expect(textures.release).toHaveBeenCalledTimes(6)
    expect(context.isRenderingTransition).toBeUndefined()
  })
})
