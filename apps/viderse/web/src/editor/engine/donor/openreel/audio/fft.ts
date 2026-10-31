/** Adapted from OpenReel packages/core/src/audio/fft.ts (c9340465).
 * Copyright (c) 2024-2026 Augustus Otu and Contributors. MIT; see src/editor/LICENSE-openreel.
 * Caller-owned spectra avoid allocations inside realtime/streaming transforms.
 */
export class FFT {
  private readonly cosine: Float64Array
  private readonly sine: Float64Array
  private readonly reverse: Uint32Array

  constructor(readonly size: number) {
    if (!Number.isInteger(size) || size < 2 || (size & (size - 1)) !== 0) {
      throw new Error('FFT size must be a power of two')
    }
    this.cosine = new Float64Array(size / 2)
    this.sine = new Float64Array(size / 2)
    this.reverse = new Uint32Array(size)
    for (let i = 0; i < size / 2; i++) {
      this.cosine[i] = Math.cos(-2 * Math.PI * i / size)
      this.sine[i] = Math.sin(-2 * Math.PI * i / size)
    }
    const bits = Math.log2(size)
    for (let i = 0; i < size; i++) {
      let reversed = 0
      for (let bit = 0; bit < bits; bit++) reversed = (reversed << 1) | ((i >> bit) & 1)
      this.reverse[i] = reversed
    }
  }

  transform(real: Float32Array, imag: Float32Array, inverse = false): void {
    const n = this.size
    for (let i = 0; i < n; i++) {
      const j = this.reverse[i]!
      if (j <= i) continue
      const r = real[i]!
      const im = imag[i]!
      real[i] = real[j]!
      imag[i] = imag[j]!
      real[j] = r
      imag[j] = im
    }
    for (let length = 2; length <= n; length *= 2) {
      const half = length / 2
      const step = n / length
      for (let i = 0; i < n; i += length) {
        for (let j = 0; j < half; j++) {
          const cos = this.cosine[j * step]!
          const sin = this.sine[j * step]! * (inverse ? -1 : 1)
          const even = i + j
          const odd = even + half
          const r = real[odd]! * cos - imag[odd]! * sin
          const im = real[odd]! * sin + imag[odd]! * cos
          real[odd] = real[even]! - r
          imag[odd] = imag[even]! - im
          real[even] = real[even]! + r
          imag[even] = imag[even]! + im
        }
      }
    }
    if (inverse) {
      for (let i = 0; i < n; i++) {
        real[i] = real[i]! / n
        imag[i] = imag[i]! / n
      }
    }
  }
}
