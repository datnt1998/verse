/** Adapted from OpenReel audio/effects-worklet-processor.ts and noise-reduction.ts (c9340465).
 * Copyright (c) 2024-2026 Augustus Otu and Contributors. MIT; see src/editor/LICENSE-openreel.
 */
import { FFT } from '@editor/donor/openreel/audio/fft'
import {
  createReverbImpulse,
  type AudioFxEffect, type CompressorParams, type DelayParams,
  type NoiseReductionParams, type ReverbParams,
} from '@editor/donor/openreel/audio/contracts'

export interface StatefulAudioStage {
  processChunk(input: Float32Array, channel: number): Float32Array
  readonly tailFrames: number
  readonly latencyFrames: number
  readonly retainedBytes: number
}

abstract class AudioStage implements StatefulAudioStage {
  abstract readonly tailFrames: number
  abstract readonly latencyFrames: number
  abstract readonly retainedBytes: number
  private readonly scratch: Float32Array[] = []
  protected output(length: number, channel: number): Float32Array {
    let buffer = this.scratch[channel]
    if (!buffer || buffer.length < length) this.scratch[channel] = buffer = new Float32Array(length)
    return buffer.subarray(0, length)
  }
  abstract processChunk(input: Float32Array, channel: number): Float32Array
}

/** OpenReel worklet's smoothed dB gain reduction, extended with the documented
 * soft knee and makeup gain. Each channel has an independent envelope.
 */
export class CompressorStage extends AudioStage {
  readonly tailFrames = 0
  readonly latencyFrames = 0
  readonly retainedBytes: number
  private readonly envelopes: Float64Array
  private readonly attack: number
  private readonly release: number
  private readonly makeup: number
  constructor(private readonly params: CompressorParams, channels: number, sampleRate: number) {
    super()
    this.envelopes = new Float64Array(channels)
    this.retainedBytes = this.envelopes.byteLength
    this.attack = Math.exp(-1 / (sampleRate * params.attack))
    this.release = Math.exp(-1 / (sampleRate * params.release))
    this.makeup = Math.pow(10, params.makeupGain / 20)
  }
  processChunk(input: Float32Array, channel: number): Float32Array {
    const output = this.output(input.length, channel)
    let envelope = this.envelopes[channel]!
    const { threshold, ratio, knee } = this.params
    const slope = 1 - 1 / ratio
    for (let i = 0; i < input.length; i++) {
      const sample = input[i]!
      const over = 20 * Math.log10(Math.max(1e-12, Math.abs(sample))) - threshold
      const reduction = knee > 0 && over > -knee / 2 && over < knee / 2
        ? slope * (over + knee / 2) ** 2 / (2 * knee)
        : over >= knee / 2 ? slope * over : 0
      const coefficient = reduction > envelope ? this.attack : this.release
      envelope = coefficient * envelope + (1 - coefficient) * reduction
      output[i] = sample * Math.pow(10, -envelope / 20) * this.makeup
    }
    this.envelopes[channel] = envelope
    return output
  }
}

/** OpenReel delay feedback topology: delay(input + feedback * delayed).
 * Tail ends when a unit impulse's next repeat is below -80 dB amplitude.
 */
export class DelayStage extends AudioStage {
  readonly latencyFrames = 0
  readonly tailFrames: number
  readonly retainedBytes: number
  private readonly lines: Float32Array[]
  private readonly positions: Uint32Array
  constructor(private readonly params: DelayParams, channels: number, sampleRate: number) {
    super()
    const length = Math.max(1, Math.round(sampleRate * params.time))
    this.lines = Array.from({ length: channels }, () => new Float32Array(length))
    this.positions = new Uint32Array(channels)
    const repeats = params.feedback > 0
      ? Math.max(1, Math.ceil(Math.log(1e-4 / Math.max(params.wetLevel, 1e-4)) / Math.log(params.feedback)))
      : 1
    this.tailFrames = params.wetLevel === 0 ? 0 : length * repeats
    this.retainedBytes = length * channels * 4 + this.positions.byteLength
  }
  processChunk(input: Float32Array, channel: number): Float32Array {
    const output = this.output(input.length, channel)
    const line = this.lines[channel]!
    let position = this.positions[channel]!
    for (let i = 0; i < input.length; i++) {
      const delayed = line[position]!
      const sample = input[i]!
      line[position] = sample + delayed * this.params.feedback
      output[i] = sample * (1 - this.params.wetLevel) + delayed * this.params.wetLevel
      if (++position === line.length) position = 0
    }
    this.positions[channel] = position
    return output
  }
}

interface SpectralChannel {
  input: Float32Array
  overlap: Float32Array
  normalization: Float32Array
  real: Float32Array
  imag: Float32Array
  consumed: number
}

/** OpenReel spectral subtraction with bounded overlap-add rings.
 * The causal output is delayed exactly fftSize samples. Zero input flushes
 * that latency; there is no duration-sized reconstruction buffer.
 */
export class NoiseReductionStage extends AudioStage {
  readonly tailFrames: number
  readonly latencyFrames: number
  readonly retainedBytes: number
  private readonly fft: FFT
  private readonly window: Float32Array
  private readonly states: SpectralChannel[]
  private readonly noise: Float32Array
  private readonly deviation: Float32Array
  constructor(private readonly params: NoiseReductionParams, channels: number, sampleRate: number) {
    super()
    if (!params.profile) throw new Error('Cần lấy mẫu nhiễu trước khi bật giảm nhiễu')
    const profile = params.profile
    const n = profile.fftSize
    this.fft = new FFT(n)
    this.latencyFrames = n
    this.tailFrames = n
    this.window = Float32Array.from({ length: n }, (_, i) => 0.5 * (1 - Math.cos(2 * Math.PI * i / (n - 1))))
    const bins = n / 2 + 1
    // Spectral magnitude is proportional to window size, not sample rate.
    // Interpolate frequency bins when preview's context rate differs from media.
    this.noise = new Float32Array(bins)
    this.deviation = new Float32Array(bins)
    for (let i = 0; i < bins; i++) {
      const sourceBin = i * sampleRate / profile.sampleRate
      const low = Math.floor(sourceBin)
      const fraction = sourceBin - low
      this.noise[i] = (profile.magnitudes[low] ?? 0) * (1 - fraction) + (profile.magnitudes[low + 1] ?? 0) * fraction
      this.deviation[i] = (profile.standardDeviations[low] ?? 0) * (1 - fraction) + (profile.standardDeviations[low + 1] ?? 0) * fraction
    }
    this.states = Array.from({ length: channels }, () => ({
      input: new Float32Array(n), overlap: new Float32Array(n * 2),
      normalization: new Float32Array(n * 2), real: new Float32Array(n),
      imag: new Float32Array(n), consumed: 0,
    }))
    this.retainedBytes = (n + bins * 2 + channels * n * 7) * 4
  }
  private frame(state: SpectralChannel, start: number): void {
    const n = this.fft.size
    for (let i = 0; i < n; i++) {
      const index = start + i
      state.real[i] = index < 0 ? 0 : state.input[index % n]! * this.window[i]!
    }
    state.imag.fill(0)
    this.fft.transform(state.real, state.imag)
    const threshold = Math.pow(10, this.params.threshold / 20)
    for (let i = 0; i <= n / 2; i++) {
      const magnitude = Math.hypot(state.real[i]!, state.imag[i]!)
      const subtracted = magnitude - (1 + this.params.reduction) * (this.noise[i]! + this.deviation[i]!)
      const floor = this.noise[i]! * (1 - this.params.reduction) * threshold
      const result = Math.max(subtracted, floor) * (1 - this.params.smoothing) + magnitude * this.params.smoothing
      const gain = magnitude > 1e-20 ? result / magnitude : 0
      state.real[i] = state.real[i]! * gain
      state.imag[i] = state.imag[i]! * gain
      if (i > 0 && i < n / 2) {
        state.real[n - i] = state.real[i]!
        state.imag[n - i] = -state.imag[i]!
      }
    }
    this.fft.transform(state.real, state.imag, true)
    for (let i = 0; i < n; i++) {
      if (start + i < 0) continue
      const index = (start + i) % (n * 2)
      const window = this.window[i]!
      state.overlap[index] = state.overlap[index]! + state.real[i]! * window
      state.normalization[index] = state.normalization[index]! + window * window
    }
  }
  processChunk(input: Float32Array, channel: number): Float32Array {
    const output = this.output(input.length, channel)
    const state = this.states[channel]!
    const n = this.fft.size
    const hop = n / 4
    for (let i = 0; i < input.length; i++) {
      const t = state.consumed++
      state.input[t % n] = input[i]!
      // Zero-padded negative frames give the clip boundary the same complete
      // Hann overlap as interior samples, avoiding edge normalization spikes.
      if (state.consumed % hop === 0) this.frame(state, state.consumed - n)
      if (t < n) { output[i] = 0; continue }
      const index = (t - n) % (n * 2)
      const weight = state.normalization[index]!
      output[i] = weight > 1e-8 ? state.overlap[index]! / weight : 0
      state.overlap[index] = 0
      state.normalization[index] = 0
    }
    return output
  }
}

const CONVOLUTION_BLOCK = 256
interface ConvolutionChannel {
  input: Float32Array
  output: Float32Array
  overlap: Float32Array
  dry: Float32Array
  historyReal: Float32Array[]
  historyImag: Float32Array[]
  sumReal: Float32Array
  sumImag: Float32Array
  position: number
  block: number
}

/** Deterministic OpenReel decaying-noise impulse, uniform partitioned
 * convolution. Fixed 256-frame latency also delays the dry branch.
 */
export class ReverbStage extends AudioStage {
  readonly latencyFrames = CONVOLUTION_BLOCK
  readonly tailFrames: number
  readonly retainedBytes: number
  private readonly fft = new FFT(CONVOLUTION_BLOCK * 2)
  private readonly kernelsReal: Float32Array[][] = []
  private readonly kernelsImag: Float32Array[][] = []
  private readonly states: ConvolutionChannel[]
  constructor(private readonly params: ReverbParams, channels: number, sampleRate: number) {
    super()
    const impulses = Array.from({ length: channels }, (_, channel) => createReverbImpulse(sampleRate, params, channel))
    const partitions = Math.ceil(impulses[0]!.length / CONVOLUTION_BLOCK)
    for (const impulse of impulses) {
      const reals: Float32Array[] = []
      const imags: Float32Array[] = []
      for (let p = 0; p < partitions; p++) {
        const real = new Float32Array(this.fft.size)
        const imag = new Float32Array(this.fft.size)
        real.set(impulse.subarray(p * CONVOLUTION_BLOCK, (p + 1) * CONVOLUTION_BLOCK))
        this.fft.transform(real, imag)
        reals.push(real)
        imags.push(imag)
      }
      this.kernelsReal.push(reals)
      this.kernelsImag.push(imags)
    }
    this.states = Array.from({ length: channels }, () => ({
      input: new Float32Array(CONVOLUTION_BLOCK), output: new Float32Array(CONVOLUTION_BLOCK),
      overlap: new Float32Array(CONVOLUTION_BLOCK), dry: new Float32Array(CONVOLUTION_BLOCK),
      historyReal: Array.from({ length: partitions }, () => new Float32Array(this.fft.size)),
      historyImag: Array.from({ length: partitions }, () => new Float32Array(this.fft.size)),
      sumReal: new Float32Array(this.fft.size), sumImag: new Float32Array(this.fft.size), position: 0, block: 0,
    }))
    this.tailFrames = this.latencyFrames + (params.wetLevel > 0 ? impulses[0]!.length - 1 : 0)
    this.retainedBytes = channels * (partitions * this.fft.size * 4 + CONVOLUTION_BLOCK * 8) * 4
  }
  private block(state: ConvolutionChannel, channel: number): void {
    const kernelsReal = this.kernelsReal[channel]!
    const kernelsImag = this.kernelsImag[channel]!
    const count = kernelsReal.length
    const head = state.block % count
    const real = state.historyReal[head]!
    const imag = state.historyImag[head]!
    real.fill(0)
    real.set(state.input)
    imag.fill(0)
    this.fft.transform(real, imag)
    state.sumReal.fill(0)
    state.sumImag.fill(0)
    for (let p = 0; p < Math.min(count, state.block + 1); p++) {
      const index = (head - p + count) % count
      const xr = state.historyReal[index]!
      const xi = state.historyImag[index]!
      const hr = kernelsReal[p]!
      const hi = kernelsImag[p]!
      for (let i = 0; i < this.fft.size; i++) {
        state.sumReal[i] = state.sumReal[i]! + xr[i]! * hr[i]! - xi[i]! * hi[i]!
        state.sumImag[i] = state.sumImag[i]! + xr[i]! * hi[i]! + xi[i]! * hr[i]!
      }
    }
    this.fft.transform(state.sumReal, state.sumImag, true)
    for (let i = 0; i < CONVOLUTION_BLOCK; i++) {
      state.output[i] = state.sumReal[i]! + state.overlap[i]!
      state.overlap[i] = state.sumReal[i + CONVOLUTION_BLOCK]!
    }
    state.block++
  }
  processChunk(input: Float32Array, channel: number): Float32Array {
    const output = this.output(input.length, channel)
    const state = this.states[channel]!
    for (let i = 0; i < input.length; i++) {
      const p = state.position
      output[i] = state.dry[p]! * this.params.dryLevel + state.output[p]! * this.params.wetLevel
      state.dry[p] = input[i]!
      state.input[p] = input[i]!
      if (++state.position === CONVOLUTION_BLOCK) {
        this.block(state, channel)
        state.position = 0
      }
    }
    return output
  }
}

export function createAudioFxStages(
  effects: readonly AudioFxEffect[] | undefined, channels: number, sampleRate: number,
): StatefulAudioStage[] {
  return (effects ?? []).filter(effect => effect.enabled).map(effect => {
    switch (effect.type) {
      case 'compressor': return new CompressorStage(effect.params, channels, sampleRate)
      case 'noiseReduction': return new NoiseReductionStage(effect.params, channels, sampleRate)
      case 'reverb': return new ReverbStage(effect.params, channels, sampleRate)
      case 'delay': return new DelayStage(effect.params, channels, sampleRate)
    }
  })
}

export class AudioFxChain implements StatefulAudioStage {
  readonly tailFrames: number
  readonly latencyFrames: number
  readonly retainedBytes: number
  constructor(readonly stages: readonly StatefulAudioStage[]) {
    this.tailFrames = stages.reduce((sum, stage) => sum + stage.tailFrames, 0)
    this.latencyFrames = stages.reduce((sum, stage) => sum + stage.latencyFrames, 0)
    this.retainedBytes = stages.reduce((sum, stage) => sum + stage.retainedBytes, 0)
  }
  processChunk(input: Float32Array, channel: number): Float32Array {
    let output = input
    for (const stage of this.stages) output = stage.processChunk(output, channel)
    return output
  }
}
export function createAudioFxChain(effects: readonly AudioFxEffect[] | undefined, channels: number, sampleRate: number): AudioFxChain {
  return new AudioFxChain(createAudioFxStages(effects, channels, sampleRate))
}
