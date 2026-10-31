import { describe, expect, it } from 'vitest'
import { voiceShape } from '@editor/test/waveform-fixture'
import { getEnvelopeBins } from './envelope'
import { waveformReadiness, waveformSourceSeconds, writeWaveformGeometry } from './geometry'
import { collectWaveformIssues } from './readiness'

describe('source-bound waveform geometry', () => {
  it('maps timeline/source fps, move, trim, speed, reverse and transition offsets', () => {
    const item = voiceShape({ from: 60, sourceStart: 30, sourceFps: 30, speed: 2 })
    expect(waveformSourceSeconds(item, 90, 30)).toBe(3)
    expect(waveformSourceSeconds({ ...item, from: 120 }, 150, 30)).toBe(3)
    expect(waveformSourceSeconds(item, 120, 60)).toBe(3)
    expect(waveformSourceSeconds({ ...item, sourceStart: undefined, trimStart: 30 }, 90, 30)).toBe(3)
    expect(waveformSourceSeconds({ ...item, isReversed: true, sourceEnd: 120 }, 75, 30)).toBeCloseTo(89 / 30)
    expect(waveformSourceSeconds(item, 90, 30, 15)).toBe(3.5)
    expect(waveformSourceSeconds(item, 90, 30, 0, { from: 75, durationInFrames: 30, sourceStart: 60 })).toBe(3)
  })
  it('writes real silence flat, voice peaks tall, seek deterministic with no geometry allocation', () => {
    const item = voiceShape(), payload = item.waveformVisualizer!, bins = getEnvelopeBins(payload)!
    const output = new Float32Array(payload.bars * 3)
    writeWaveformGeometry(bins, payload, 0.5, 128, 64, output)
    for (let i = 0; i < payload.bars; i++) expect(output[i * 3 + 1]).toBe(output[i * 3 + 2])
    writeWaveformGeometry(bins, payload, 1.5, 128, 64, output)
    expect(output[1]).toBe(0); expect(output[2]).toBe(64)
    const peak = [...output]
    writeWaveformGeometry(bins, payload, 3, 128, 64, output)
    expect([...output]).not.toEqual(peak)
    writeWaveformGeometry(bins, payload, 1.5, 128, 64, output)
    expect([...output]).toEqual(peak)
  })
  it('rejects missing/stale envelopes including nested references without affecting ordinary shapes', () => {
    const item = voiceShape(), ordinary = { ...item, waveformVisualizer: undefined }
    expect(waveformReadiness(ordinary)).toBeNull()
    expect(waveformReadiness(item, `sha256:${'b'.repeat(64)}`)).toMatch(/đã cũ/)
    const missing = { ...item, waveformVisualizer: { ...item.waveformVisualizer!, envelopeBase64: undefined } }
    expect(collectWaveformIssues([missing])).toHaveLength(1)
    const wrapper = { id: 'nested', type: 'composition', compositionId: 'comp' } as Parameters<typeof collectWaveformIssues>[0][number]
    expect(collectWaveformIssues([wrapper], [{ id: 'comp', items: [missing, wrapper] }])).toHaveLength(1)
  })
})
