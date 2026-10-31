/** Adapted from OpenReel audio-effects-engine.ts, noise-reduction.ts,
 * audio-effect-routing.ts and types/effects.ts at c9340465.
 * Copyright (c) 2024-2026 Augustus Otu and Contributors. MIT; see src/editor/LICENSE-openreel.
 */
export interface SerializedNoiseProfile {
  frequencyBins: number[]
  magnitudes: number[]
  standardDeviations: number[]
  sampleRate: number
  fftSize: number
}

export interface CompressorParams {
  threshold: number
  ratio: number
  attack: number
  release: number
  knee: number
  makeupGain: number
}
export interface ReverbParams {
  roomSize: number
  damping: number
  wetLevel: number
  dryLevel: number
  preDelay: number
}
export interface DelayParams {
  time: number
  feedback: number
  wetLevel: number
}
export interface NoiseReductionParams {
  threshold: number
  reduction: number
  smoothing: number
  profile?: SerializedNoiseProfile
}
export type AudioFxEffect = { id: string; enabled: boolean } & (
  | { type: 'compressor'; params: CompressorParams }
  | { type: 'noiseReduction'; params: NoiseReductionParams }
  | { type: 'reverb'; params: ReverbParams }
  | { type: 'delay'; params: DelayParams }
)

export const AUDIO_EFFECT_TYPES = ['compressor', 'noiseReduction', 'reverb', 'delay'] as const
export const DEFAULT_COMPRESSOR: CompressorParams = {
  threshold: -24, ratio: 4, attack: 0.003, release: 0.25, knee: 30, makeupGain: 0,
}
export const DEFAULT_REVERB: ReverbParams = {
  roomSize: 0.5, damping: 0.5, wetLevel: 0.5, dryLevel: 0.7, preDelay: 0,
}
export const DEFAULT_DELAY: DelayParams = { time: 0.5, feedback: 0.3, wetLevel: 0.5 }
export const DEFAULT_NOISE_REDUCTION: NoiseReductionParams = {
  threshold: -40, reduction: 0.5, smoothing: 0.15,
}

/** Native ConvolverNode normalizes random IRs; explicit energy normalization
 * and a fixed PRNG seed make the donor's decay reproducible on every renderer.
 */
export function createReverbImpulse(sampleRate: number, params: ReverbParams, channel: number): Float32Array {
  const duration = 0.5 + params.roomSize * 3.5
  const length = Math.floor(sampleRate * duration)
  const preDelay = Math.round(sampleRate * params.preDelay / 1000)
  const impulse = new Float32Array(length + preDelay)
  let seed = (0x6d2b79f5 + channel * 0x9e3779b9) >>> 0
  let energy = 0
  for (let i = 0; i < length; i++) {
    seed ^= seed << 13
    seed ^= seed >>> 17
    seed ^= seed << 5
    const noise = (seed >>> 0) / 0xffffffff * 2 - 1
    const t = i / sampleRate
    const value = noise * Math.exp(-3 * t / duration) * (1 - params.damping * t / duration)
    impulse[i + preDelay] = value
    energy += value * value
  }
  const gain = 1 / Math.sqrt(energy || 1)
  for (let i = preDelay; i < impulse.length; i++) impulse[i] = impulse[i]! * gain
  return impulse
}
