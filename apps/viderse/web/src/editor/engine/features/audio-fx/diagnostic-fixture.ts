import { DEFAULT_COMPRESSOR, DEFAULT_DELAY, DEFAULT_NOISE_REDUCTION, DEFAULT_REVERB, type AudioFxEffect } from '@editor/donor/openreel/audio/contracts'
import { learnNoiseProfile } from './noise-profile'

export function createAudioFxFixture(sampleRate = 48000, seconds = 30): { channels: Float32Array[]; effects: AudioFxEffect[] } {
  const frames = Math.round(sampleRate * seconds)
  const channels = Array.from({ length: 2 }, () => new Float32Array(frames))
  let seed = 123456789
  for (let i = 0; i < frames; i++) {
    seed ^= seed << 13
    seed ^= seed >>> 17
    seed ^= seed << 5
    const noise = ((seed >>> 0) / 0xffffffff * 2 - 1) * 0.025
    const t = i / sampleRate
    const tone = t < 0.3 || t > seconds - 3 ? 0 : Math.sin(t * Math.PI * 2 * 220) * 0.38 * (0.5 + 0.5 * Math.sin(t * Math.PI))
    channels[0]![i] = tone + noise
    channels[1]![i] = tone * 0.9 + noise
  }
  const profile = learnNoiseProfile(channels, sampleRate, 0, Math.max(0.25, 2048 / sampleRate))
  return { channels, effects: [
    { id: 'compressor', type: 'compressor', enabled: true, params: { ...DEFAULT_COMPRESSOR, makeupGain: 3 } },
    { id: 'noise-reduction', type: 'noiseReduction', enabled: true, params: { ...DEFAULT_NOISE_REDUCTION, profile, reduction: 0.7 } },
    { id: 'reverb', type: 'reverb', enabled: true, params: { ...DEFAULT_REVERB, roomSize: 0.2, wetLevel: 0.25 } },
    { id: 'delay', type: 'delay', enabled: true, params: { ...DEFAULT_DELAY, time: 0.125, wetLevel: 0.25 } },
  ] }
}

/** IEEE-float WAV avoids quantization when comparing realtime vs export DSP. */
export function encodeFloatWav(channels: readonly Float32Array[], sampleRate: number): Blob {
  const frames = channels[0]?.length ?? 0
  const count = channels.length
  const dataSize = frames * count * 4
  const bytes = new ArrayBuffer(44 + dataSize)
  const view = new DataView(bytes)
  function ascii(offset: number, text: string) {
    for (let i = 0; i < text.length; i++) view.setUint8(offset + i, text.charCodeAt(i))
  }
  ascii(0, 'RIFF'); view.setUint32(4, 36 + dataSize, true); ascii(8, 'WAVE')
  ascii(12, 'fmt '); view.setUint32(16, 16, true); view.setUint16(20, 3, true)
  view.setUint16(22, count, true); view.setUint32(24, sampleRate, true)
  view.setUint32(28, sampleRate * count * 4, true); view.setUint16(32, count * 4, true)
  view.setUint16(34, 32, true); ascii(36, 'data'); view.setUint32(40, dataSize, true)
  for (let frame = 0; frame < frames; frame++) {
    for (let channel = 0; channel < count; channel++) view.setFloat32(44 + (frame * count + channel) * 4, channels[channel]![frame]!, true)
  }
  return new Blob([bytes], { type: 'audio/wav' })
}

export function audioStats(channels: readonly Float32Array[]): { rms: number; peak: number } {
  let square = 0
  let peak = 0
  let count = 0
  for (const channel of channels) {
    for (const sample of channel) { square += sample * sample; peak = Math.max(peak, Math.abs(sample)); count++ }
  }
  return { rms: Math.sqrt(square / Math.max(1, count)), peak }
}
