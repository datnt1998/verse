import { describe, expect, it } from 'vitest'
import { getGpuShapeUnsupportedReason, prepareGpuMediaParticipant } from './gpu'
import { voiceShape, voiceTransform, waveformRenderContext } from '@editor/test/waveform-fixture'
import { getItemRenderTimelineSpan } from '../render-span'

describe('procedural shape GPU eligibility', () => {
  it('rejects voice waveform before ordinary shape eligibility even without a GPU pipeline', async () => {
    const item = voiceShape(), context = waveformRenderContext()
    expect(getGpuShapeUnsupportedReason(item, voiceTransform, [], context)).toBe('waveform-visualizer-requires-canvas')
    expect(await prepareGpuMediaParticipant({ item, transform: voiceTransform, effects: [], renderSpan: getItemRenderTimelineSpan(item) }, 45, context)).toBeNull()
  })
  it('continues preparing supported ordinary rectangles on the direct GPU path', async () => {
    const item = voiceShape({ waveformVisualizer: undefined }), context = waveformRenderContext()
    context.gpuShapePipeline = {} as NonNullable<typeof context.gpuShapePipeline>
    expect(getGpuShapeUnsupportedReason(item, voiceTransform, [], context)).toBeNull()
    expect((await prepareGpuMediaParticipant({ item, transform: voiceTransform, effects: [], renderSpan: getItemRenderTimelineSpan(item) }, 45, context))?.media.kind).toBe('shape')
  })
})
