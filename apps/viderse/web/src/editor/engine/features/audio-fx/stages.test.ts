import { describe, expect, it } from 'vitest'
import { FFT } from '@editor/donor/openreel/audio/fft'
import {
  DEFAULT_COMPRESSOR, DEFAULT_DELAY, DEFAULT_NOISE_REDUCTION, DEFAULT_REVERB,
  createReverbImpulse, type AudioFxEffect,
} from '@editor/donor/openreel/audio/contracts'
import { learnNoiseProfile } from './noise-profile'
import { CompressorStage, DelayStage, NoiseReductionStage, ReverbStage, createAudioFxChain, type StatefulAudioStage } from './stages'

const rate = 8000
function render(stage: StatefulAudioStage, input: Float32Array, chunks: readonly number[]): Float32Array {
  const output = new Float32Array(input.length)
  let offset = 0
  let index = 0
  while (offset < input.length) {
    const end = Math.min(input.length, offset + chunks[index++ % chunks.length]!)
    output.set(stage.processChunk(input.subarray(offset, end), 0), offset)
    offset = end
  }
  return output
}
function rms(samples: Float32Array): number {
  return Math.sqrt(samples.reduce((sum, sample) => sum + sample * sample, 0) / samples.length)
}
const noise = Float32Array.from({ length: rate }, (_, i) => 0.04 * Math.sin(2 * Math.PI * 500 * i / rate))
const profile = learnNoiseProfile([noise], rate, 0, 1)
const effects: AudioFxEffect[] = [
  { id: 'compressor', type: 'compressor', enabled: true, params: DEFAULT_COMPRESSOR },
  { id: 'noise', type: 'noiseReduction', enabled: true, params: { ...DEFAULT_NOISE_REDUCTION, profile } },
  { id: 'reverb', type: 'reverb', enabled: true, params: { ...DEFAULT_REVERB, roomSize: 0 } },
  { id: 'delay', type: 'delay', enabled: true, params: { ...DEFAULT_DELAY, time: 0.031, feedback: 0.4 } },
]

describe('stateful donor audio effects', () => {
  it('FFT impulse and round trip match known vectors', () => {
    const fft = new FFT(8)
    const real = Float32Array.from([1, 0, 0, 0, 0, 0, 0, 0])
    const imag = new Float32Array(8)
    fft.transform(real, imag)
    expect(Array.from(real)).toEqual(Array(8).fill(1))
    expect(Array.from(imag)).toEqual(Array(8).fill(0))
    fft.transform(real, imag, true)
    expect(Array.from(real)).toEqual([1, 0, 0, 0, 0, 0, 0, 0])
  })
  it('compressor settles to the threshold/ratio transfer and independent stereo envelopes', () => {
    const stage = new CompressorStage({ ...DEFAULT_COMPRESSOR, threshold: -20, ratio: 4, knee: 0 }, 2, rate)
    const loud = stage.processChunk(new Float32Array(rate).fill(1), 0)
    expect(loud[rate - 1]).toBeCloseTo(Math.pow(10, -15 / 20), 6)
    expect(stage.processChunk(Float32Array.from([0.01]), 1)[0]).toBeCloseTo(0.01, 7)
  })
  it('delay impulse repeats and tail duration are exact', () => {
    const stage = new DelayStage({ time: 0.001, feedback: 0.5, wetLevel: 1 }, 1, rate)
    const impulse = new Float32Array(8 + stage.tailFrames)
    impulse[0] = 1
    const output = render(stage, impulse, [3, 1, 11])
    expect(output[0]).toBe(0)
    expect(output[8]).toBe(1)
    expect(output[16]).toBe(0.5)
    expect(output[24]).toBe(0.25)
    expect(stage.tailFrames).toBe(8 * Math.ceil(Math.log(1e-4) / Math.log(0.5)))
  })
  it('partitioned reverb impulse equals seeded donor IR including pre-delay and final tail', () => {
    const params = { ...DEFAULT_REVERB, roomSize: 0, preDelay: 3, wetLevel: 1, dryLevel: 0 }
    const stage = new ReverbStage(params, 1, rate)
    const expected = createReverbImpulse(rate, params, 0)
    const impulse = new Float32Array(1 + stage.tailFrames)
    impulse[0] = 1
    const output = render(stage, impulse, [1, 127, 513])
    expect(stage.tailFrames).toBe(expected.length - 1 + stage.latencyFrames)
    let error = 0
    for (let i = 0; i < expected.length; i++) error = Math.max(error, Math.abs(output[i + stage.latencyFrames]! - expected[i]!))
    expect(error).toBeLessThan(1e-7)
  })
  it('learns only the selected noise range and attenuates its spectral line', () => {
    expect(profile.frequencyBins[128]).toBe(500)
    expect(profile.magnitudes[128]).toBeCloseTo(0.04 * (2048 - 1) / 4, 2)
    const stage = new NoiseReductionStage({ threshold: -40, reduction: 1, smoothing: 0, profile }, 1, rate)
    const output = render(stage, noise, [128])
    expect(rms(output.subarray(4096))).toBeLessThan(rms(noise) * 0.02)
    expect(() => learnNoiseProfile([noise], rate, 0, 0.01)).toThrow()
  })
  it('noise reduction has complete Hann overlap at the clip boundary without peak amplification', () => {
    const stage = new NoiseReductionStage({ threshold: -40, reduction: 0.7, smoothing: 0.15, profile }, 1, rate)
    const output = render(stage, noise, [128])
    expect(Math.max(...output.map(Math.abs))).toBeLessThan(0.04)
    expect(output.slice(0, stage.latencyFrames)).toEqual(new Float32Array(stage.latencyFrames))
  })
  it.each(effects)('$type is invariant across arbitrary window boundaries', effect => {
    const input = Float32Array.from({ length: rate * 2 }, (_, i) => Math.sin(i * 0.123) * (i < 6000 ? 0.8 : 0.03))
    const a = createAudioFxChain([effect], 1, rate)
    const b = createAudioFxChain([effect], 1, rate)
    expect(render(a, input, [input.length])).toEqual(render(b, input, [1, 127, 509, 1024, 13]))
  })
  it('whole chain is identical to preview 128-frame quanta and bounded for longer streams', () => {
    const a = createAudioFxChain(effects, 1, rate)
    const b = createAudioFxChain(effects, 1, rate)
    const input = Float32Array.from({ length: rate * 3 }, (_, i) => Math.sin(i * 0.02) * 0.2)
    expect(render(a, input, [input.length])).toEqual(render(b, input, [128]))
    const retained = b.retainedBytes
    const chunk = new Float32Array(128)
    for (let i = 0; i < 1000; i++) b.processChunk(chunk, 0)
    expect(b.retainedBytes).toBe(retained)
    expect(b.tailFrames).toBe(b.stages.reduce((sum, stage) => sum + stage.tailFrames, 0))
    expect(b.latencyFrames).toBe(2048 + 256)
  })
})
