// @vitest-environment node

/**
 * Quality contract for the pure-JS polyphase resampler (worker export path).
 *
 * Thresholds (measured on this implementation, stated here so regressions
 * are caught):
 *  - SNR of a mid-band sine vs its analytic reference: >= 90 dB for both
 *    44.1 -> 48 kHz (up) and 48 -> 44.1 kHz (down). Measured ~101 dB.
 *  - Stopband attenuation (tone at 48.8% of input rate, inside the stopband
 *    for both directions): >= 85 dB. Measured ~92 dB.
 *  - Group delay: 0 frames (kernel is centered); verified via the lag-0
 *    optimum of the SNR fit.
 *  - Determinism: push/finish streaming in arbitrary chunks is bit-identical
 *    to resamplePcm on the whole buffer.
 *  - Output length: ceil(inputFrames * outRate / inRate), matching the
 *    OfflineAudioContext length convention of the replaced code path.
 *  - In-browser parity vs OfflineAudioContext (headed Chrome) is measured
 *    once via the diagnostic harness; numbers live in the phase report.
 */
import { describe, expect, it } from 'vitest'
import {
  PolyphaseResampler,
  resamplePcm,
  resampledFrameCount,
} from './polyphase-resampler'

function sine(freq: number, rate: number, frames: number, amp = 0.8): Float32Array {
  const out = new Float32Array(frames)
  for (let i = 0; i < frames; i++) out[i] = amp * Math.sin((2 * Math.PI * freq * i) / rate)
  return out
}

/** SNR (dB) of `signal` vs a sine of `freq` at `rate`, best small integer lag. */
function sineSnrDb(
  signal: Float32Array,
  freq: number,
  rate: number,
  amp = 0.8,
  edge = 512,
): { snrDb: number; lag: number } {
  let bestLag = 0
  let bestErr = Infinity
  for (let lag = -4; lag <= 4; lag++) {
    let err = 0
    for (let i = edge; i < signal.length - edge; i++) {
      const ref = amp * Math.sin((2 * Math.PI * freq * (i + lag)) / rate)
      err += (signal[i]! - ref) ** 2
    }
    if (err < bestErr) {
      bestErr = err
      bestLag = lag
    }
  }
  let errPow = 0
  let sigPow = 0
  for (let i = edge; i < signal.length - edge; i++) {
    const ref = amp * Math.sin((2 * Math.PI * freq * (i + bestLag)) / rate)
    errPow += (signal[i]! - ref) ** 2
    sigPow += ref * ref
  }
  return { snrDb: 10 * Math.log10(sigPow / errPow), lag: bestLag }
}

function rmsDb(signal: Float32Array, edge = 512): number {
  let power = 0
  for (let i = edge; i < signal.length - edge; i++) power += signal[i]! * signal[i]!
  return 10 * Math.log10(power / (signal.length - 2 * edge))
}

describe('PolyphaseResampler quality', () => {
  it.each([[44_100, 48_000], [48_000, 44_100]])(
    'reconstructs a 100–15000 Hz analytic sweep above 90 dB SNR: %i -> %i',
    (inputRate, outputRate) => {
      const input = new Float32Array(inputRate)
      for (let frame = 0; frame < input.length; frame++) {
        const seconds = frame / inputRate
        input[frame] = 0.6 * Math.sin(2 * Math.PI * (100 * seconds + 14900 * seconds ** 2 / 2))
      }
      const output = resamplePcm(input, inputRate, outputRate)
      let signalPower = 0
      let errorPower = 0
      for (let frame = 1024; frame < output.length - 1024; frame++) {
        const seconds = frame / outputRate
        const reference = 0.6 * Math.sin(2 * Math.PI * (100 * seconds + 14900 * seconds ** 2 / 2))
        signalPower += reference ** 2
        errorPower += (output[frame]! - reference) ** 2
      }
      expect(10 * Math.log10(signalPower / errorPower)).toBeGreaterThanOrEqual(90)
    },
  )
  it('upsamples 44.1 -> 48 kHz at >= 90 dB SNR with zero group delay', () => {
    const input = sine(5000, 44_100, 44_100)
    const output = resamplePcm(input, 44_100, 48_000)
    expect(output.length).toBe(48_000)
    const { snrDb, lag } = sineSnrDb(output, 5000, 48_000)
    expect(snrDb).toBeGreaterThanOrEqual(90)
    expect(lag).toBe(0)
  })

  it('downsamples 48 -> 44.1 kHz at >= 90 dB SNR with zero group delay', () => {
    const input = sine(5000, 48_000, 48_000)
    const output = resamplePcm(input, 48_000, 44_100)
    expect(output.length).toBe(44_100)
    const { snrDb, lag } = sineSnrDb(output, 5000, 44_100)
    expect(snrDb).toBeGreaterThanOrEqual(90)
    expect(lag).toBe(0)
  })

  it('attenuates stopband content >= 85 dB in both directions', () => {
    // 21.5 kHz at 44.1 kHz: above the 0.45*inRate passband edge for 44.1->48.
    const stopUp = resamplePcm(sine(21_500, 44_100, 44_100), 44_100, 48_000)
    // 23 kHz at 48 kHz: above the 0.45*outRate anti-alias edge for 48->44.1.
    const stopDown = resamplePcm(sine(23_000, 48_000, 48_000), 48_000, 44_100)
    // 0.8-amp sine RMS = -1.94 dBFS; attenuation = ref - measured.
    const refDb = -1.94
    expect(refDb - rmsDb(stopUp)).toBeGreaterThanOrEqual(85)
    expect(refDb - rmsDb(stopDown)).toBeGreaterThanOrEqual(85)
  })

  it('keeps passband flat: 0.1x and 0.4x Nyquist within 0.2 dB', () => {
    const lo = resamplePcm(sine(2_000, 44_100, 44_100), 44_100, 48_000)
    const hi = resamplePcm(sine(17_600, 44_100, 44_100), 44_100, 48_000)
    expect(Math.abs(rmsDb(lo) - rmsDb(hi))).toBeLessThanOrEqual(0.2)
  })

  it('is bit-identical between whole-buffer and chunked streaming', () => {
    const input = sine(5000, 44_100, 44_100)
    const whole = resamplePcm(input, 44_100, 48_000)
    for (const chunkSize of [1, 997, 4096, 44_100]) {
      const resampler = new PolyphaseResampler(44_100, 48_000)
      const parts: Float32Array[] = []
      for (let o = 0; o < input.length; o += chunkSize) {
        parts.push(resampler.push(input.subarray(o, o + chunkSize) as Float32Array))
      }
      parts.push(resampler.finish())
      const total = parts.reduce((sum, part) => sum + part.length, 0)
      const streamed = new Float32Array(total)
      let offset = 0
      for (const part of parts) {
        streamed.set(part, offset)
        offset += part.length
      }
      expect(streamed.length).toBe(whole.length)
      let maxDiff = 0
      for (let i = 0; i < whole.length; i++) {
        maxDiff = Math.max(maxDiff, Math.abs(streamed[i]! - whole[i]!))
      }
      expect(maxDiff).toBe(0)
    }
  })

  it('reports output length via resampledFrameCount matching the OAC convention', () => {
    expect(resampledFrameCount(44_100, 44_100, 48_000)).toBe(48_000)
    expect(resampledFrameCount(48_000, 48_000, 44_100)).toBe(44_100)
    expect(resampledFrameCount(1, 44_100, 48_000)).toBe(2)
    expect(resampledFrameCount(0, 44_100, 48_000)).toBe(0)
  })

  it('passes through identical rates and handles silence/impulse', () => {
    const input = sine(440, 48_000, 1024)
    expect(resamplePcm(input, 48_000, 48_000)).toBe(input)
    const silence = new Float32Array(4096)
    const out = resamplePcm(silence, 44_100, 48_000)
    expect(Math.max(...out.map(Math.abs))).toBe(0)
    const impulse = new Float32Array(4096)
    impulse[2048] = 1
    const ir = resamplePcm(impulse, 44_100, 48_000)
    expect(ir.length).toBe(resampledFrameCount(4096, 44_100, 48_000))
    expect(Math.max(...ir.map(Math.abs))).toBeGreaterThan(0.5)
  })
})
