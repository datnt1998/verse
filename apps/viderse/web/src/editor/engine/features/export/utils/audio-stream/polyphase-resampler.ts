/**
 * Pure-JS windowed-sinc polyphase resampler for the export audio path.
 *
 * Replaces OfflineAudioContext resampling so export audio works inside
 * workers (where OfflineAudioContext does not exist). Both the whole-buffer
 * helper and the streaming class produce the same deterministic output.
 *
 * Design: rational rate factor P/Q (continued-fraction approximation),
 * prototype low-pass = Kaiser-windowed sinc with cutoff 0.45/max(P,Q)
 * cycles per upsampled sample, each polyphase branch normalized to unity
 * DC gain. Output uses integer phase/index stepping — no accumulated
 * floating-point drift — and the kernel is centered on the output position,
 * so group delay is zero.
 */

export interface ResampleRatio {
  /** Upsampling factor (output frames per Q input frames). */
  up: number
  /** Downsampling factor. */
  down: number
}

const DEFAULT_TAPS_PER_PHASE = 96
const MAX_DENOMINATOR = 1024
const PASSBAND_FRACTION_OF_NYQUIST = 0.9
const KAISER_BETA = 9.0

/**
 * Approximate `value` as an integer ratio with denominator <= maxDen using
 * continued-fraction convergents plus a final semiconvergent step.
 */
export function approximateRatio(
  value: number,
  maxDen = MAX_DENOMINATOR,
): ResampleRatio {
  if (!(value > 0) || !Number.isFinite(value)) {
    throw new Error(`Invalid resample ratio ${value}`)
  }
  let p0 = 0
  let q0 = 1
  let p1 = 1
  let q1 = 0
  let x = value
  while (x > 0) {
    const a = Math.floor(x)
    const p2 = a * p1 + p0
    const q2 = a * q1 + q0
    if (q2 > maxDen) break
    p0 = p1
    q0 = q1
    p1 = p2
    q1 = q2
    if (Math.abs(p1 / q1 - value) < 1e-12) return { up: p1, down: q1 }
    x = 1 / (x - a)
  }
  // Best approximation within the denominator bound lies between the last
  // convergent and the previous one: take the largest valid semiconvergent.
  const k = Math.floor((maxDen - q0) / q1)
  if (k > 0) return { up: p0 + k * p1, down: q0 + k * q1 }
  return { up: p0, down: q0 }
}

/** Frames emitted for `inputFrames` at the given rates. */
export function resampledFrameCount(
  inputFrames: number,
  inputSampleRate: number,
  outputSampleRate: number,
): number {
  // Matches the OfflineAudioContext length convention used by the previous
  // export path: ceil(duration * outputRate).
  return Math.max(0, Math.ceil((inputFrames * outputSampleRate) / inputSampleRate))
}

function sinc(x: number): number {
  if (x === 0) return 1
  const px = Math.PI * x
  return Math.sin(px) / px
}

/** Modified Bessel function I0 — power series, converges fast for |x| <= ~20. */
function besselI0(x: number): number {
  let sum = 1
  let term = 1
  const x2 = (x * x) / 4
  for (let k = 1; k <= 30; k++) {
    term *= x2 / (k * k)
    sum += term
    if (term < sum * 1e-16) break
  }
  return sum
}

interface PolyphaseBank {
  /** Taps for each phase p in [0, up): g[p][s], s in [0, tapsPerPhase). */
  phases: Float64Array[]
  tapsPerPhase: number
  /** floor(tapsPerPhase / 2); the kernel covers input indices [i-M+1, i+M]. */
  halfTaps: number
}

/**
 * Build the polyphase filter bank for rate factor up/down.
 *
 * Prototype: h[k] = 2*fc*sinc(2*fc*k)*kaiser(k) for k in (-M*up, M*up],
 * fc = passbandFraction/2 / max(up, down) cycles per upsampled sample.
 * Phase p holds taps k = p + t*up for t in [-M, M); each phase is normalized
 * to sum 1 (unity DC gain per branch).
 */
function buildPolyphaseBank(
  up: number,
  down: number,
  tapsPerPhase: number,
  passbandFraction: number,
  beta: number,
): PolyphaseBank {
  const halfTaps = Math.floor(tapsPerPhase / 2)
  const spanK = halfTaps * up
  const fc = passbandFraction / 2 / Math.max(up, down)
  const i0Beta = besselI0(beta)

  const phases: Float64Array[] = []
  for (let p = 0; p < up; p++) {
    const g = new Float64Array(tapsPerPhase)
    let sum = 0
    for (let s = 0; s < tapsPerPhase; s++) {
      const k = p + (s - halfTaps) * up
      const r = spanK === 0 ? 0 : k / spanK
      const inner = Math.max(0, 1 - r * r)
      const window = besselI0(beta * Math.sqrt(inner)) / i0Beta
      const tap = 2 * fc * sinc(2 * fc * k) * window
      g[s] = tap
      sum += tap
    }
    if (sum !== 0) {
      for (let s = 0; s < tapsPerPhase; s++) g[s] /= sum
    }
    phases.push(g)
  }
  return { phases, tapsPerPhase, halfTaps }
}

/**
 * Streaming resampler for one channel. Push input blocks of any size; output
 * for the covered range is returned immediately. Call finish() once at
 * end-of-input to flush the filter tail (input beyond the last sample is
 * zero-padded). push*+finish on a chunked input equals resamplePcm on the
 * whole input.
 */
export class PolyphaseResampler {
  readonly inputSampleRate: number
  readonly outputSampleRate: number
  readonly ratio: ResampleRatio
  private readonly bank: PolyphaseBank
  /** Buffered input; bufStart = absolute index of buf[0]. */
  private buf = new Float32Array(0)
  private bufStart = 0
  private inputEnd = 0 // absolute index one past the last pushed input frame
  private nextOut = 0 // absolute output frame index to emit next
  private finished = false

  constructor(
    inputSampleRate: number,
    outputSampleRate: number,
    tapsPerPhase = DEFAULT_TAPS_PER_PHASE,
  ) {
    if (inputSampleRate <= 0 || outputSampleRate <= 0) {
      throw new Error(`Invalid sample rates ${inputSampleRate} -> ${outputSampleRate}`)
    }
    this.inputSampleRate = inputSampleRate
    this.outputSampleRate = outputSampleRate
    this.ratio = approximateRatio(outputSampleRate / inputSampleRate)
    this.bank = buildPolyphaseBank(
      this.ratio.up,
      this.ratio.down,
      tapsPerPhase,
      PASSBAND_FRACTION_OF_NYQUIST,
      KAISER_BETA,
    )
  }

  /** Output frame index the next emitted frame corresponds to. */
  get outputFramesEmitted(): number {
    return this.nextOut
  }

  get isFinished(): boolean {
    return this.finished
  }

  /** Input frames of lookahead needed before one more output can emit. */
  get lookaheadFrames(): number {
    return this.bank.halfTaps + 1
  }

  private emit(outLimit: number, inputLimit: number, out: Float32Array): number {
    const { up, down } = this.ratio
    const { phases, tapsPerPhase, halfTaps } = this.bank
    const buf = this.buf
    const bufStart = this.bufStart
    let emitted = 0
    while (this.nextOut < outLimit) {
      const m = this.nextOut
      const pq = m * down
      const i = Math.floor(pq / up)
      const p = pq % up
      // Output m needs input [i - M + 1, i + M]; emit only while the
      // lookahead is real input (finishing passes an unbounded limit).
      if (i + halfTaps >= inputLimit) break
      const g = phases[p]!
      let acc = 0
      for (let s = 0; s < tapsPerPhase; s++) {
        const rel = i + halfTaps - s - bufStart
        if (rel >= 0 && rel < buf.length) acc += g[s]! * buf[rel]!
      }
      out[emitted] = acc
      this.nextOut += 1
      emitted += 1
    }
    return emitted
  }

  /** Drop buffered input frames that no future output can reference. */
  private compact(): void {
    const { up, down } = this.ratio
    const { halfTaps } = this.bank
    const minIndex = Math.floor((this.nextOut * down) / up) - halfTaps + 1
    const drop = Math.min(Math.max(0, minIndex - this.bufStart), this.buf.length)
    if (drop > 0) {
      this.buf = this.buf.slice(drop)
      this.bufStart += drop
    }
  }

  /**
   * Push input frames; returns the resampled frames that became available
   * (freshly allocated).
   */
  push(input: Float32Array): Float32Array {
    if (this.finished) throw new Error('push() after finish()')
    if (input.length === 0) return new Float32Array(0)

    this.compact()
    const merged = new Float32Array(this.buf.length + input.length)
    merged.set(this.buf, 0)
    merged.set(input, this.buf.length)
    this.buf = merged
    this.inputEnd += input.length

    const total = resampledFrameCount(this.inputEnd, this.inputSampleRate, this.outputSampleRate)
    const out = new Float32Array(total - this.nextOut)
    const emitted = this.emit(total, this.inputEnd, out)
    this.compact()
    return out.subarray(0, emitted) as Float32Array
  }

  /**
   * Flush: emit all remaining output frames, treating input beyond the last
   * pushed sample as zeros. Call exactly once after the final push().
   */
  finish(): Float32Array {
    if (this.finished) return new Float32Array(0)
    this.finished = true
    const total = resampledFrameCount(this.inputEnd, this.inputSampleRate, this.outputSampleRate)
    const out = new Float32Array(total - this.nextOut)
    const emitted = this.emit(total, Number.MAX_SAFE_INTEGER / 4, out)
    this.buf = new Float32Array(0)
    return out.subarray(0, emitted) as Float32Array
  }
}

/**
 * Whole-buffer resample — drop-in replacement for the OfflineAudioContext
 * resampler. Output length is ceil(input.length * outRate / inRate).
 */
export function resamplePcm(
  samples: Float32Array,
  inputSampleRate: number,
  outputSampleRate: number,
): Float32Array {
  if (inputSampleRate === outputSampleRate) return samples
  if (samples.length === 0) return new Float32Array(0)
  const resampler = new PolyphaseResampler(inputSampleRate, outputSampleRate)
  const head = resampler.push(samples)
  const tail = resampler.finish()
  const out = new Float32Array(head.length + tail.length)
  out.set(head, 0)
  out.set(tail, head.length)
  return out
}
