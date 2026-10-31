import { ENVELOPE_RATE, MAX_ENVELOPE_SECONDS, type WaveformEnvelope, type WaveformVisualizer } from './types'

export function encodeEnvelope(bytes: Uint8Array): string {
  let binary = ''
  for (let start = 0; start < bytes.length; start += 8192) binary += String.fromCharCode(...bytes.subarray(start, start + 8192))
  return btoa(binary)
}
export function decodeEnvelope(value: string): Uint8Array {
  const binary = atob(value)
  const bytes = new Uint8Array(binary.length)
  for (let i = 0; i < binary.length; i++) bytes[i] = binary.charCodeAt(i)
  return bytes
}

/** Retains only 50 small energy bins/second, never the source PCM. Stereo
 * energy is averaged across channels; peaks use max(abs(channel)), so inverted
 * stereo cannot cancel the voice. Every bin is timestamped by sample index. */
export class EnvelopeAccumulator {
  private energies: number[] = []
  private sumSquares = 0
  private peak = 0
  private framesInBin = 0
  private frames = 0
  constructor(readonly sampleRate: number, readonly channels: number) {
    if (!Number.isInteger(sampleRate) || sampleRate < ENVELOPE_RATE || !Number.isInteger(channels) || channels < 1) throw new Error('Invalid PCM format')
  }
  push(samples: Float32Array): void {
    if (samples.length % this.channels) throw new Error('Incomplete PCM frame')
    for (let i = 0; i < samples.length; i += this.channels) {
      if (this.frames >= MAX_ENVELOPE_SECONDS * this.sampleRate) throw new Error('Voice source exceeds 24 hours')
      let energy = 0
      for (let channel = 0; channel < this.channels; channel++) {
        const value = samples[i + channel]!
        if (!Number.isFinite(value)) throw new Error('Non-finite PCM sample')
        energy += value * value
        this.peak = Math.max(this.peak, Math.abs(value))
      }
      this.sumSquares += energy / this.channels
      this.framesInBin++
      this.frames++
      if (this.frames >= Math.ceil((this.energies.length + 1) * this.sampleRate / ENVELOPE_RATE)) this.flush()
    }
  }
  private flush(): void {
    if (!this.framesInBin) return
    this.energies.push(0.7 * Math.sqrt(this.sumSquares / this.framesInBin) + 0.3 * this.peak)
    this.sumSquares = this.peak = this.framesInBin = 0
  }
  finish(sourceHash: string): WaveformEnvelope {
    this.flush()
    if (!this.frames) throw new Error('Voice source contains no PCM samples')
    let normalization = 0
    for (const value of this.energies) normalization = Math.max(normalization, value)
    const bytes = new Uint8Array(this.energies.length)
    if (normalization > 0) for (let i = 0; i < bytes.length; i++) bytes[i] = Math.round(this.energies[i]! / normalization * 255)
    return { sourceHash, samplesPerSecond: ENVELOPE_RATE, durationSeconds: this.frames / this.sampleRate, normalization, envelopeBase64: encodeEnvelope(bytes) }
  }
}

/** Weak ownership follows the immutable item payload: replacing/opening a
 * project releases decoded bins without a process-global active-item cache. */
const decoded = new WeakMap<WaveformVisualizer, { encoded: string; hash: string; bins: Uint8Array }>()
export function getEnvelopeBins(payload: WaveformVisualizer): Uint8Array | null {
  if (!payload.envelopeBase64 || !payload.durationSeconds || payload.normalization === undefined) return null
  const cached = decoded.get(payload)
  if (cached?.encoded === payload.envelopeBase64 && cached.hash === payload.sourceHash) return cached.bins
  try {
    const bins = decodeEnvelope(payload.envelopeBase64)
    if (bins.length !== Math.ceil(payload.durationSeconds * payload.samplesPerSecond - 1e-8)) return null
    decoded.set(payload, { encoded: payload.envelopeBase64, hash: payload.sourceHash, bins })
    return bins
  } catch { return null }
}

const geometryBuffers = new WeakMap<WaveformVisualizer, Float32Array>()
export function getWaveformGeometryBuffer(payload: WaveformVisualizer): Float32Array {
  let buffer = geometryBuffers.get(payload)
  if (!buffer || buffer.length !== payload.bars * 3) {
    buffer = new Float32Array(payload.bars * 3)
    geometryBuffers.set(payload, buffer)
  }
  return buffer
}
