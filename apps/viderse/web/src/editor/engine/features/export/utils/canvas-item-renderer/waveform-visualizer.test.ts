import { describe, expect, it } from 'vitest'
import { WaveformTestCanvas, voiceShape, voiceTransform, waveformRenderContext } from '@editor/test/waveform-fixture'
import { renderItem } from './render-item'
import { renderWaveformVisualizer } from './waveform-visualizer'
import { getEnvelopeBins, getWaveformGeometryBuffer } from '@editor/features/waveform-visualizer/envelope'
import { writeWaveformGeometry } from '@editor/features/waveform-visualizer/geometry'
import { assessExportPreflight } from '../export-preflight'

describe('canvas voice waveform frames', () => {
  it('matches shared SVG geometry with exact alpha-mask pixels at silence, peak, midpoint and seek', async () => {
    const item = voiceShape(), context = waveformRenderContext()
    const frames: Uint8Array[] = []
    for (const frame of [15, 45, 60, 90, 45]) {
      const canvas = new WaveformTestCanvas()
      await renderItem(canvas.getContext(), item, voiceTransform, frame, context)
      const geometry = new Float32Array(item.waveformVisualizer!.bars * 3)
      writeWaveformGeometry(getEnvelopeBins(item.waveformVisualizer!)!, item.waveformVisualizer!, frame / 30, 128, 64, geometry)
      expect(canvas.segments.flat()).toEqual(Array.from({ length: item.waveformVisualizer!.bars }, (_, i) => [geometry[i * 3], geometry[i * 3 + 1], geometry[i * 3], geometry[i * 3 + 2]]).flat())
      frames.push(canvas.pixels)
    }
    expect(frames[0]).not.toEqual(frames[1]); expect(frames[1]).toEqual(frames[4]); expect(frames[2]).not.toEqual(frames[3])
  })
  it('preserves surrounding opacity and caller buffer ownership; export fails closed', async () => {
    const item = voiceShape(), context = waveformRenderContext(), canvas = new WaveformTestCanvas()
    await renderItem(canvas.getContext(), item, { ...voiceTransform, opacity: 0.5 }, 45, context)
    expect(Math.max(...canvas.pixels)).toBe(128)
    const buffer = getWaveformGeometryBuffer(item.waveformVisualizer!)
    renderWaveformVisualizer(canvas.getContext(), item, voiceTransform, 60, context)
    expect(getWaveformGeometryBuffer(item.waveformVisualizer!)).toBe(buffer)
    const missing = voiceShape(); delete missing.waveformVisualizer!.envelopeBase64
    expect(() => renderWaveformVisualizer(canvas.getContext(), missing, voiceTransform, 0, context)).toThrow(/Thiếu dữ liệu/)
  })
  it('blocks preflight with a missing envelope, while a ready source is exportable', async () => {
    const item = voiceShape()
    const options = { settings: { mode: 'video' as const, codec: 'h264' as const, quality: 'high' as const, resolution: { width: 1920, height: 1080 } }, fps: 30, durationFrames: 120, supportedVideoCodecs: ['avc' as const], workerAvailable: true, audioEncoderSupported: true, composition: { fps: 30, tracks: [{ id: 'visual', name: 'Visual', height: 60, order: 0, visible: true, locked: false, muted: false, solo: false, items: [item] }] } }
    expect((await assessExportPreflight(options)).canExport).toBe(true)
    delete item.waveformVisualizer!.envelopeBase64
    const blocked = await assessExportPreflight(options)
    expect(blocked.canExport).toBe(false)
    expect(blocked.checks).toContainEqual(expect.objectContaining({ id: 'waveform-envelope-blocks-export', severity: 'error' }))
  })
})
