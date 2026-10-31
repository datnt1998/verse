/**
 * Stateful windowed DSP for one audio segment.
 *
 * Pipeline order mirrors the whole-buffer export path
 * (canvas-audio.ts processAudio):
 *   source PCM (windowed pushes; reversed order is applied by the caller's
 *   source mapper for isReversed)
 *     → TimeStretchFilter (vendored SoundTouch, kept alive across windows)
 *     → StreamingAudioEq (biquad/one-pole state carried across windows)
 *
 * The SoundTouch SimpleFilter is driven exactly like the whole-buffer
 * applySpeedAndPitch (pull-based extract of 4096-frame chunks), so its
 * internal buffering — and therefore its deterministic output stream — is
 * identical. The filter's source is a FIFO fed by pushInput(); output is
 * capped at floor(sourceFrames / speed), matching the reference.
 */

import type { ResolvedAudioEqSettings } from '@editor/types/audio'
import { StreamingAudioEq } from '@editor/shared/utils/audio-eq'
import {
  getAudioPitchRatioFromSemitones,
  isAudioPitchShiftActive,
} from '@editor/shared/utils/audio-pitch'
// The vendored SoundTouch module is @ts-nocheck; declare the used surface.
import {
  TimeStretchFilter,
  TimeStretchProcessor,
} from '@editor/infrastructure/audio/time-stretch'

/** Per-clip or bus effect state, constructed once at its processing rate. */
export interface StatefulAudioStage {
  processChunk(input: Float32Array, channel: number): Float32Array
  readonly tailFrames: number
}

export interface StreamingSegmentParams {
  /** Output sample rate of the DECODED source (DSP runs at this rate). */
  sampleRate: number
  /** Channel count of the decoded source (1 or 2 handled natively). */
  channels: number
  /** Total source frames this segment consumes (post trim, pre speed). */
  sourceFrames: number
  speed: number
  isReversed: boolean
  pitchShiftSemitones: number
  audioEqStages?: ReadonlyArray<ResolvedAudioEqSettings>
  audioFxStages?: StatefulAudioStage[] | null
}

/**
 * Per-segment DSP state carried across output windows (contract:
 * plan Phase-2 step 5 / reports/bakeoff/dsp-streaming.md).
 */
export interface SegmentDspState {
  /** Vendored SoundTouch processor + FIFO source; null when speed=1∧pitch=0. */
  stretchActive: boolean
  /** Per-channel EQ filter chains (carried DF-I state). */
  eq: StreamingAudioEq
  /** Source frames fed so far (after reverse mapping). */
  sourceFramesConsumed: number
  /** Output frames emitted so far. */
  outputFramesEmitted: number
  /** floor(sourceFrames / speed); output is capped here. */
  expectedOutputFrames: number
}

/** Pull source for interleaved stereo frames; returns frames written. */
export type InterleavedStereoReader = (
  target: Float32Array,
  numFrames: number,
  sourcePosition: number,
) => number

interface StretchSource {
  extract: InterleavedStereoReader
}

interface StretchFilterInstance {
  extract: (target: Float32Array, numFrames: number) => number
}

interface StretchProcessorInstance {
  tempo: number
  pitch: number
  rate: number
}

const EXTRACT_CHUNK_FRAMES = 4096 // same chunk size the whole-buffer path uses

/** Bounded FIFO of interleaved stereo frames fed by pushInput(). Fixed-size
 * slabs keep pushes allocation-free and bound retention to a few slabs. */
const FIFO_SLAB_FRAMES = 32768

class InterleavedSourceFifo {
  private slabs: Float32Array[] = []
  private pool: Float32Array[] = []
  private readHead = 0 // frame offset into slabs[0]
  private bufferedFrames = 0

  push(interleaved: Float32Array): void {
    const frames = Math.floor(interleaved.length / 2)
    if (frames <= 0) return
    let srcOffset = 0
    while (srcOffset < frames) {
      let tail = this.slabs[this.slabs.length - 1]
      if (!tail || this.tailFilled === FIFO_SLAB_FRAMES) {
        tail = this.pool.pop() ?? new Float32Array(FIFO_SLAB_FRAMES * 2)
        this.slabs.push(tail)
        this.tailFilled = 0
      }
      const take = Math.min(frames - srcOffset, FIFO_SLAB_FRAMES - this.tailFilled)
      tail.set(interleaved.subarray(srcOffset * 2, (srcOffset + take) * 2), this.tailFilled * 2)
      this.tailFilled += take
      srcOffset += take
      this.bufferedFrames += take
    }
  }

  private tailFilled = 0

  get framesAvailable(): number {
    return this.bufferedFrames
  }

  extract: InterleavedStereoReader = (target, numFrames) => {
    let filled = 0
    while (filled < numFrames && this.bufferedFrames > 0) {
      const slab = this.slabs[0]!
      const available = this.tailAvailable(slab)
      const take = Math.min(numFrames - filled, available - this.readHead)
      target.set(slab.subarray(this.readHead * 2, (this.readHead + take) * 2), filled * 2)
      filled += take
      this.readHead += take
      this.bufferedFrames -= take
      if (this.readHead >= available) {
        this.pool.push(this.slabs.shift()!)
        this.readHead = 0
      }
    }
    return filled
  }

  /** Frames written into a slab: full for drained middle slabs, the fill
   *  counter for the current tail slab. */
  private tailAvailable(slab: Float32Array): number {
    return slab === this.slabs[this.slabs.length - 1] ? this.tailFilled : FIFO_SLAB_FRAMES
  }
}

export interface WindowedDspStats {
  /** Frames the stretch stage emitted in total (before the reference cap). */
  stretchEmittedFrames: number
  /** Frames delivered to the caller (capped at expectedOutputFrames). */
  deliveredFrames: number
  /** Peak source-side FIFO depth in frames (bounded by the driver). */
  peakSourceFifoFrames: number
}

/**
 * Implements the Phase-1 contract: SoundTouch is fed through a FIFO,
 * reversed input arrives already reversed, output is capped at
 * floor(sourceFrames/speed), and EQ state persists across windows.
 */
export class StreamingSegmentProcessor implements SegmentDspState {
  readonly expectedOutputFrames: number
  readonly stretchActive: boolean
  readonly eq: StreamingAudioEq
  readonly tailOutputFrames: number
  private tailFramesEmitted = 0

  get tailFramesRemaining(): number {
    return this.tailOutputFrames - this.tailFramesEmitted
  }
  sourceFramesConsumed = 0
  get outputFramesEmitted(): number {
    return this.framesEmitted
  }

  private readonly fifo = new InterleavedSourceFifo()
  private readonly filter: StretchFilterInstance | null
  private readonly extractScratch = new Float32Array(EXTRACT_CHUNK_FRAMES * 2)
  private framesEmitted = 0
  private inputFinishedFlag = false
  private stretchLeftScratch = new Float32Array(0)
  private stretchRightScratch = new Float32Array(0)
  private stats: WindowedDspStats = {
    stretchEmittedFrames: 0,
    deliveredFrames: 0,
    peakSourceFifoFrames: 0,
  }

  constructor(private readonly params: StreamingSegmentParams) {
    this.expectedOutputFrames = Math.floor(params.sourceFrames / params.speed)
    this.stretchActive =
      Math.abs(params.speed - 1) > 0.0001 || isAudioPitchShiftActive(params.pitchShiftSemitones)
    this.eq = new StreamingAudioEq(params.channels, params.sampleRate, params.audioEqStages)
    this.tailOutputFrames = (params.audioFxStages ?? []).reduce(
      (frames, stage) => frames + stage.tailFrames,
      0,
    )

    if (this.stretchActive) {
      const st = new TimeStretchProcessor() as StretchProcessorInstance
      st.tempo = params.speed
      st.pitch = getAudioPitchRatioFromSemitones(params.pitchShiftSemitones)
      st.rate = 1.0
      const source: StretchSource = { extract: this.fifo.extract }
      this.filter = new TimeStretchFilter(
        source,
        st,
      ) as unknown as StretchFilterInstance
    } else {
      this.filter = null
    }
  }

  /**
   * Push one decoded source window (planar channels at params.sampleRate,
   * any length). Reversed clips: the caller's source mapper must deliver
   * frames already reversed so the stream is identical to reversing the
   * whole decoded range up-front. Interleaved once into the stretch FIFO.
   */
  pushInput(channels: Float32Array[], frames: number): void {
    if (!this.stretchActive) {
      throw new Error('pushInput is only for the stretch path; use processPassthroughWindow')
    }
    if (this.inputFinishedFlag) throw new Error('pushInput after finishInput')
    const clamped = Math.min(frames, channels[0]?.length ?? 0)
    if (clamped <= 0) return
    const interleaved = new Float32Array(clamped * 2)
    const left = channels[0]!
    const right = channels.length >= 2 ? channels[1]! : left
    for (let i = 0; i < clamped; i++) {
      interleaved[i * 2] = left[i]!
      interleaved[i * 2 + 1] = right[i]!
    }
    this.fifo.push(interleaved)
    this.sourceFramesConsumed += clamped
    this.stats.peakSourceFifoFrames = Math.max(
      this.stats.peakSourceFifoFrames,
      this.fifo.framesAvailable,
    )
  }

  /** Call after the last pushInput: the filter drains its internal tail. */
  finishInput(): void {
    this.inputFinishedFlag = true
  }

  get sourceExhausted(): boolean {
    return this.sourceFramesConsumed >= this.params.sourceFrames
  }

  get inputFinished(): boolean {
    return this.inputFinishedFlag
  }

  get bufferedSourceFrames(): number {
    return this.fifo.framesAvailable
  }

  /**
   * Pull the next output chunk through stretch (if active) then EQ.
   * Returns planar channels, or null when nothing can be produced right now
   * (feed more input, or the stream is done once inputFinished and a pull
   * returned null). Output stops at expectedOutputFrames — identical to the
   * whole-buffer cap (the WSOLA tail is dropped the same way).
   */
  pullOutput(maxFrames: number): Float32Array[] | null {
    if (!this.stretchActive) return null
    const channelCount = this.params.channels

    const remaining = this.expectedOutputFrames - this.framesEmitted
    if (remaining <= 0) return null
    const want = Math.min(maxFrames, remaining)

    if (this.stretchLeftScratch.length < want) {
      this.stretchLeftScratch = new Float32Array(want)
      this.stretchRightScratch = new Float32Array(want)
    }
    const stretchLeft = this.stretchLeftScratch
    const stretchRight = this.stretchRightScratch
    let outFrames = 0
    while (outFrames < want) {
      const extracted = this.filter!.extract(
        this.extractScratch,
        Math.min(EXTRACT_CHUNK_FRAMES, want - outFrames),
      )
      if (extracted === 0) break
      for (let i = 0; i < extracted; i++) {
        stretchLeft[outFrames + i] = this.extractScratch[i * 2]!
        stretchRight[outFrames + i] = this.extractScratch[i * 2 + 1]!
      }
      outFrames += extracted
    }

    this.stats.stretchEmittedFrames += outFrames
    if (outFrames === 0) return null
    this.framesEmitted += outFrames

    const out: Float32Array[] = [
      this.eqChunk(stretchLeft.subarray(0, outFrames) as Float32Array, 0),
    ]
    if (channelCount >= 2) {
      out.push(this.eqChunk(stretchRight.subarray(0, outFrames) as Float32Array, 1))
      // Extra decoded channels mirror the whole-buffer path: it feeds only
      // L/R into SoundTouch and duplicates left for channels beyond stereo.
      for (let c = 2; c < channelCount; c++) {
        out.push(this.eqChunk(stretchLeft.subarray(0, outFrames) as Float32Array, c))
      }
    }
    this.stats.deliveredFrames += outFrames
    return out
  }

  /**
   * Passthrough+EQ path for segments without stretch (speed 1, no pitch);
   * reversed input arrives already reversed. Returns processed copies.
   */
  processPassthroughWindow(channels: Float32Array[], frames: number): Float32Array[] {
    const out: Float32Array[] = []
    for (let c = 0; c < channels.length; c++) {
      const chunk = channels[c]!.subarray(0, frames) as Float32Array
      out.push(this.eqChunk(chunk, c))
    }
    this.framesEmitted += frames
    this.stats.deliveredFrames += frames
    return out
  }

  private eqChunk(chunk: Float32Array, channel: number): Float32Array {
    let processed = this.eq.processChunk(chunk, channel).subarray(0, chunk.length)
    for (const stage of this.params.audioFxStages ?? []) {
      processed = stage.processChunk(processed, channel)
      if (processed.length !== chunk.length) {
        throw new Error('Audio effect stage changed the frame count')
      }
    }
    return processed.slice(0, chunk.length)
  }

  /** Drain effect tails with zero input, after the source stream has ended. */
  pullTail(maxFrames: number): Float32Array[] | null {
    const frames = Math.min(maxFrames, this.tailOutputFrames - this.tailFramesEmitted)
    if (frames <= 0) return null
    const zero = new Float32Array(frames)
    const output = Array.from({ length: this.params.channels }, (_, channel) => {
      let processed: Float32Array = zero
      for (const stage of this.params.audioFxStages ?? []) {
        processed = stage.processChunk(processed, channel)
        if (processed.length !== frames) throw new Error('Audio effect stage changed the frame count')
      }
      return processed.slice()
    })
    this.tailFramesEmitted += frames
    return output
  }

  getStats(): WindowedDspStats {
    return { ...this.stats }
  }
}
