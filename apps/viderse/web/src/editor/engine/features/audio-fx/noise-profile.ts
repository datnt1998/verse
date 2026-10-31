/** Adapted from OpenReel audio/noise-reduction.ts (c9340465).
 * Copyright (c) 2024-2026 Augustus Otu and Contributors. MIT; see src/editor/LICENSE-openreel.
 */
import { FFT } from '@editor/donor/openreel/audio/fft'
import type { SerializedNoiseProfile } from '@editor/donor/openreel/audio/contracts'

/** Selected source range in seconds; averages all channels without retaining PCM.
 * Adapted from OpenReel SpectralNoiseReducer.learnNoiseProfile (MIT).
 */
export function learnNoiseProfile(
  channels: readonly Float32Array[], sampleRate: number,
  startSeconds: number, endSeconds: number, fftSize = 2048,
): SerializedNoiseProfile {
  if (!Number.isFinite(sampleRate) || sampleRate <= 0 || !Number.isFinite(startSeconds) ||
      !Number.isFinite(endSeconds) || startSeconds < 0 || endSeconds <= startSeconds || !channels.length) {
    throw new Error('Khoảng lấy mẫu nhiễu không hợp lệ')
  }
  const fft = new FFT(fftSize)
  const start = Math.round(startSeconds * sampleRate)
  const end = Math.round(endSeconds * sampleRate)
  if (channels.some(channel => end > channel.length) || end - start < fftSize) {
    throw new Error('Khoảng nhiễu phải chứa ít nhất một khung FFT')
  }
  const bins = fftSize / 2 + 1
  const sum = new Float64Array(bins)
  const square = new Float64Array(bins)
  const real = new Float32Array(fftSize)
  const imag = new Float32Array(fftSize)
  let count = 0
  for (let frame = start; frame + fftSize <= end; frame += fftSize / 4) {
    for (const channel of channels) {
      for (let i = 0; i < fftSize; i++) {
        real[i] = channel[frame + i]! * 0.5 * (1 - Math.cos(2 * Math.PI * i / (fftSize - 1)))
      }
      imag.fill(0)
      fft.transform(real, imag)
      for (let i = 0; i < bins; i++) {
        const magnitude = Math.hypot(real[i]!, imag[i]!)
        sum[i] = sum[i]! + magnitude
        square[i] = square[i]! + magnitude * magnitude
      }
      count++
    }
  }
  const magnitudes = Array.from(sum, value => value / count)
  return {
    frequencyBins: Array.from({ length: bins }, (_, i) => i * sampleRate / fftSize),
    magnitudes,
    standardDeviations: Array.from(square, (value, i) => Math.sqrt(Math.max(0, value / count - magnitudes[i]! ** 2))),
    sampleRate, fftSize,
  }
}
