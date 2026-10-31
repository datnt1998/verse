import { describe, expect, it } from 'vitest'
import { EnvelopeAccumulator, decodeEnvelope, encodeEnvelope, getEnvelopeBins } from './envelope'
import { DEFAULT_WAVEFORM_STYLE, waveformVisualizerSchema } from './types'
import { VOICE_HASH } from '@editor/test/waveform-fixture'

describe('quantized voice envelope', () => {
  it('uses sample-exact bins and stable whole-source normalization across chunks', () => {
    const samples = Float32Array.from({ length: 2001 }, (_, i) => i < 1000 ? 0 : 0.25)
    const whole = new EnvelopeAccumulator(1000, 1), chunks = new EnvelopeAccumulator(1000, 1)
    whole.push(samples)
    for (let i = 0; i < samples.length; i += 37) chunks.push(samples.subarray(i, i + 37))
    const envelope = whole.finish(VOICE_HASH)
    expect(chunks.finish(VOICE_HASH)).toEqual(envelope)
    expect(envelope.durationSeconds).toBe(2.001)
    expect(decodeEnvelope(envelope.envelopeBase64)).toHaveLength(101)
    expect([...decodeEnvelope(envelope.envelopeBase64).subarray(0, 50)]).toEqual(Array(50).fill(0))
    expect(envelope.normalization).toBeCloseTo(0.25)
  })
  it('preserves inverted stereo energy and silence has zero normalization', () => {
    const stereo = new EnvelopeAccumulator(1000, 2)
    stereo.push(Float32Array.from({ length: 40 }, (_, i) => i % 2 ? -0.5 : 0.5))
    expect([...decodeEnvelope(stereo.finish(VOICE_HASH).envelopeBase64)]).toEqual([255])
    const silence = new EnvelopeAccumulator(1000, 1); silence.push(new Float32Array(20))
    expect(silence.finish(VOICE_HASH).normalization).toBe(0)
  })
  it('decodes once for an owned immutable payload and rejects malformed/bounded payloads', () => {
    const accumulator = new EnvelopeAccumulator(1000, 1); accumulator.push(new Float32Array(20))
    const payload = { version: 1 as const, ...DEFAULT_WAVEFORM_STYLE, ...accumulator.finish(VOICE_HASH) }
    expect(getEnvelopeBins(payload)).toBe(getEnvelopeBins(payload))
    expect(waveformVisualizerSchema.safeParse(payload).success).toBe(true)
    expect(waveformVisualizerSchema.safeParse({ ...payload, bars: 257 }).success).toBe(false)
    expect(waveformVisualizerSchema.safeParse({ ...payload, envelopeBase64: 'AAAA' }).success).toBe(false)
    expect(waveformVisualizerSchema.safeParse({ ...payload, sourceHash: 'unknown' }).success).toBe(false)
  })
  it('validates and round-trips hour-long compact envelopes without recursive regex overflow', () => {
    const bins = new Uint8Array(3600 * 50)
    const payload = { version: 1 as const, ...DEFAULT_WAVEFORM_STYLE, sourceHash: VOICE_HASH, samplesPerSecond: 50 as const, durationSeconds: 3600, normalization: 0, envelopeBase64: encodeEnvelope(bins) }
    expect(waveformVisualizerSchema.safeParse(payload).success).toBe(true)
    expect(getEnvelopeBins(payload)).toEqual(bins)
  })
})
