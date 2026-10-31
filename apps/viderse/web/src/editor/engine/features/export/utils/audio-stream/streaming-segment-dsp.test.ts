// @vitest-environment node

/**
 * Windowed DSP parity tests: a 2-minute stereo signal processed in 30 s
 * output windows must be bit-identical to the whole-buffer reference
 * (reverse → SoundTouch → applyAudioEqStages), for every DSP clip shape:
 * EQ only, pitch, speed up/down, and reverse.
 *
 * The reference replicates canvas-audio's processAudio segment pipeline
 * exactly (same vendored SoundTouch, same 4096-frame extract cadence, same
 * floor(input/speed) cap) — the parity contract is bit-exactness, so the
 * reference must share the implementation, not re-derive it.
 */
import { describe, expect, it } from 'vitest'
import {
  StreamingSegmentProcessor,
  type StreamingSegmentParams,
} from './streaming-segment-dsp'
import {
  TimeStretchFilter,
  TimeStretchProcessor,
} from '@editor/infrastructure/audio/time-stretch'
import { applyAudioEqStages } from '@editor/shared/utils/audio-eq'
import {
  getAudioPitchRatioFromSemitones,
  isAudioPitchShiftActive,
} from '@editor/shared/utils/audio-pitch'
import type { ResolvedAudioEqSettings } from '@editor/types/audio'

const SAMPLE_RATE = 48_000
const CHANNELS = 2
const WINDOW_SECONDS = 30
const WINDOW_FRAMES = WINDOW_SECONDS * SAMPLE_RATE
const INPUT_WINDOW_FRAMES = WINDOW_FRAMES // 30 s source pushes
const SIGNAL_SECONDS = 120
const SIGNAL_FRAMES = SIGNAL_SECONDS * SAMPLE_RATE

const EQ_STAGE: ResolvedAudioEqSettings = {
  outputGainDb: 1.5,
  band1Enabled: true,
  band1Type: 'high-pass',
  band1FrequencyHz: 80,
  band1GainDb: 0,
  band1Q: 1.1,
  band1SlopeDbPerOct: 24,
  lowCutEnabled: false,
  lowCutFrequencyHz: 30,
  lowCutSlopeDbPerOct: 12,
  lowEnabled: true,
  lowType: 'low-shelf',
  lowGainDb: 3,
  lowFrequencyHz: 120,
  lowQ: 2.3,
  lowMidEnabled: true,
  lowMidType: 'peaking',
  lowMidGainDb: -4,
  lowMidFrequencyHz: 400,
  lowMidQ: 1.4,
  midGainDb: 0,
  highMidEnabled: true,
  highMidType: 'peaking',
  highMidGainDb: 2,
  highMidFrequencyHz: 1600,
  highMidQ: 1.1,
  highEnabled: true,
  highType: 'high-shelf',
  highGainDb: -2,
  highFrequencyHz: 2800,
  highQ: 2.3,
  band6Enabled: true,
  band6Type: 'low-pass',
  band6FrequencyHz: 14000,
  band6GainDb: 0,
  band6Q: 1.1,
  band6SlopeDbPerOct: 12,
  highCutEnabled: false,
  highCutFrequencyHz: 22000,
  highCutSlopeDbPerOct: 12,
}

interface DspCase {
  name: string
  speed: number
  isReversed: boolean
  pitchSemitones: number
  eq?: ResolvedAudioEqSettings[]
}

const CASES: DspCase[] = [
  { name: 'eq-only', speed: 1, isReversed: false, pitchSemitones: 0, eq: [EQ_STAGE] },
  { name: 'pitch+3', speed: 1, isReversed: false, pitchSemitones: 3 },
  { name: 'speed-1.5x', speed: 1.5, isReversed: false, pitchSemitones: 0 },
  { name: 'speed-0.75x+eq', speed: 0.75, isReversed: false, pitchSemitones: 0, eq: [EQ_STAGE] },
  { name: 'reverse+eq', speed: 1, isReversed: true, pitchSemitones: 0, eq: [EQ_STAGE] },
]

/** Deterministic test signal: detuned sines + pseudo-noise, stereo. */
function makeSignal(frames: number): Float32Array[] {
  const left = new Float32Array(frames)
  const right = new Float32Array(frames)
  let seed = 0x12345
  const noise = () => {
    seed = (seed * 1103515245 + 12345) & 0x7fffffff
    return seed / 0x7fffffff - 0.5
  }
  for (let i = 0; i < frames; i++) {
    const t = i / SAMPLE_RATE
    const wobble = 1 + 0.001 * Math.sin(2 * Math.PI * 0.07 * t)
    left[i] =
      0.4 * Math.sin(2 * Math.PI * 220 * wobble * t) +
      0.2 * Math.sin(2 * Math.PI * 1750 * t + 0.5) +
      0.05 * noise()
    right[i] =
      0.4 * Math.sin(2 * Math.PI * 220 * wobble * t + 0.3) +
      0.18 * Math.sin(2 * Math.PI * 3300 * t) +
      0.05 * noise()
  }
  return [left, right]
}

function reverseChannels(channels: Float32Array[]): Float32Array[] {
  return channels.map((samples) => {
    const reversed = new Float32Array(samples.length)
    for (let i = 0; i < samples.length; i++) {
      reversed[i] = samples[samples.length - 1 - i] ?? 0
    }
    return reversed
  })
}

interface StretchSourceLike {
  extract: (target: Float32Array, numFrames: number, position: number) => number
}
interface StretchFilterLike {
  extract: (target: Float32Array, numFrames: number) => number
}

/** Whole-buffer speed/pitch — canvas-audio's applySpeedAndPitch, verbatim. */
function applySpeedAndPitchReference(
  channels: Float32Array[],
  speed: number,
  pitchShiftSemitones: number,
): Float32Array[] {
  const requiresTimeStretch =
    Math.abs(speed - 1) > 0.0001 || isAudioPitchShiftActive(pitchShiftSemitones)
  if (!requiresTimeStretch) return channels
  if (channels.length === 0 || channels[0]!.length === 0) return channels

  const numChannels = channels.length
  const samplesPerChannel = channels[0]!.length

  const st = new TimeStretchProcessor() as {
    tempo: number
    pitch: number
    rate: number
  }
  st.tempo = speed
  st.pitch = getAudioPitchRatioFromSemitones(pitchShiftSemitones)
  st.rate = 1.0

  const stereoInput = new Float32Array(samplesPerChannel * 2)
  const left = channels[0]!
  const right = numChannels >= 2 ? channels[1]! : left
  for (let i = 0; i < samplesPerChannel; i++) {
    stereoInput[i * 2] = left[i]!
    stereoInput[i * 2 + 1] = right[i]!
  }

  let inputOffset = 0
  const source: StretchSourceLike = {
    extract: (target, numFrames) => {
      const samplesToRead = Math.min(numFrames * 2, stereoInput.length - inputOffset)
      if (samplesToRead <= 0) return 0
      for (let i = 0; i < samplesToRead; i++) {
        target[i] = stereoInput[inputOffset + i]!
      }
      inputOffset += samplesToRead
      return samplesToRead / 2
    },
  }

  const filter = new TimeStretchFilter(source, st) as unknown as StretchFilterLike
  const expectedOutputLength = Math.floor(samplesPerChannel / speed)
  const stereoOutput = new Float32Array(expectedOutputLength * 2)

  let outputOffset = 0
  const chunkSize = 4096
  const chunk = new Float32Array(chunkSize * 2)
  while (outputOffset < stereoOutput.length) {
    const framesExtracted = filter.extract(chunk, chunkSize)
    if (framesExtracted === 0) break
    const samplesToWrite = Math.min(framesExtracted * 2, stereoOutput.length - outputOffset)
    for (let i = 0; i < samplesToWrite; i++) {
      stereoOutput[outputOffset + i] = chunk[i]!
    }
    outputOffset += framesExtracted * 2
  }

  const actualOutputLength = Math.floor(outputOffset / 2)
  const outLeft = new Float32Array(actualOutputLength)
  const outRight = new Float32Array(actualOutputLength)
  for (let i = 0; i < actualOutputLength; i++) {
    outLeft[i] = stereoOutput[i * 2]!
    outRight[i] = stereoOutput[i * 2 + 1]!
  }
  return numChannels >= 2 ? [outLeft, outRight] : [outLeft]
}

/** The whole-buffer path exactly as processAudio runs it per segment. */
function processSegmentReference(
  channels: Float32Array[],
  params: Pick<
    StreamingSegmentParams,
    'speed' | 'isReversed' | 'pitchShiftSemitones' | 'audioEqStages' | 'sampleRate'
  >,
): Float32Array[] {
  let processed = channels
  if (params.isReversed) processed = reverseChannels(processed)
  processed = applySpeedAndPitchReference(
    processed,
    params.speed,
    params.pitchShiftSemitones,
  )
  return applyAudioEqStages(processed, params.sampleRate, params.audioEqStages)
}

/**
 * Serves the source in 30 s windows in PLAYBACK order — for reversed
 * segments, last window first with each window channel-reversed (identical
 * to reversing the whole decoded range up-front).
 */
class WindowedSourceFeeder {
  private served = 0
  constructor(
    private readonly channels: Float32Array[],
    readonly totalFrames: number,
    private readonly windowFrames: number,
    private readonly reversed: boolean,
  ) {}

  next(): { channels: Float32Array[]; frames: number } | null {
    if (this.served >= this.totalFrames) return null
    const frames = Math.min(this.windowFrames, this.totalFrames - this.served)
    const start = this.reversed ? this.totalFrames - this.served - frames : this.served
    const slices = this.channels.map((channel) => {
      const slice = channel.slice(start, start + frames)
      if (!this.reversed) return slice
      const reversed = new Float32Array(frames)
      for (let i = 0; i < frames; i++) reversed[i] = slice[frames - 1 - i] ?? 0
      return reversed
    })
    this.served += frames
    return { channels: slices, frames }
  }
}

interface StreamedResult {
  channels: Float32Array[]
  outputBoundaries: number[]
  peakSourceFifoFrames: number
}

/** Drive the processor in 30 s output windows, pushing source on demand. */
function runStreaming(
  source: Float32Array[],
  params: StreamingSegmentParams,
  sourceFrames: number,
): StreamedResult {
  const dsp = new StreamingSegmentProcessor(params)
  const feeder = new WindowedSourceFeeder(
    source,
    sourceFrames,
    INPUT_WINDOW_FRAMES,
    params.isReversed,
  )
  const outL: number[] = []
  const outR: number[] = []
  const boundaries: number[] = []

  const append = (window: Float32Array[]) => {
    if (outL.length > 0) boundaries.push(outL.length)
    for (let i = 0; i < window[0]!.length; i++) outL.push(window[0]![i]!)
    const ch2 = window[1] ?? window[0]!
    for (let i = 0; i < ch2.length; i++) outR.push(ch2[i]!)
  }

  if (!dsp.stretchActive) {
    for (;;) {
      const window = feeder.next()
      if (!window) break
      append(dsp.processPassthroughWindow(window.channels, window.frames))
    }
  } else {
    for (;;) {
      const want = Math.min(WINDOW_FRAMES, dsp.expectedOutputFrames - outL.length)
      if (want <= 0) break
      const pulled = dsp.pullOutput(want)
      if (pulled && pulled[0]!.length > 0) {
        append(pulled)
        continue
      }
      if (dsp.inputFinished) break
      if (dsp.sourceExhausted) {
        dsp.finishInput()
        continue
      }
      const window = feeder.next()
      if (!window) {
        dsp.finishInput()
        continue
      }
      dsp.pushInput(window.channels, window.frames)
    }
  }

  return {
    channels: [Float32Array.from(outL), Float32Array.from(outR)],
    outputBoundaries: boundaries,
    peakSourceFifoFrames: dsp.getStats().peakSourceFifoFrames,
  }
}

function maxJumpWithin(samples: Float32Array, center: number, radius: number): number {
  let max = 0
  const lo = Math.max(1, center - radius)
  const hi = Math.min(samples.length - 1, center + radius)
  for (let i = lo; i <= hi; i++) {
    max = Math.max(max, Math.abs(samples[i]! - samples[i - 1]!))
  }
  return max
}

describe('StreamingSegmentProcessor parity', () => {
  const source = makeSignal(SIGNAL_FRAMES)

  for (const dspCase of CASES) {
    it(
      `matches the whole-buffer reference bit-exactly: ${dspCase.name}`,
      () => {
        const reference = processSegmentReference(source, {
          speed: dspCase.speed,
          isReversed: dspCase.isReversed,
          pitchShiftSemitones: dspCase.pitchSemitones,
          audioEqStages: dspCase.eq,
          sampleRate: SAMPLE_RATE,
        })

        const streamed = runStreaming(
          source,
          {
            sampleRate: SAMPLE_RATE,
            channels: CHANNELS,
            sourceFrames: SIGNAL_FRAMES,
            speed: dspCase.speed,
            isReversed: dspCase.isReversed,
            pitchShiftSemitones: dspCase.pitchSemitones,
            audioEqStages: dspCase.eq,
          },
          SIGNAL_FRAMES,
        )

        // Length equals the reference exactly (the reference itself emits
        // fewer than floor(input/speed) for stretch cases).
        expect(streamed.channels[0]!.length).toBe(reference[0]!.length)
        expect(streamed.channels[1]!.length).toBe(reference[1]!.length)

        // Bit-exact per channel.
        for (let c = 0; c < 2; c++) {
          const ref = reference[c]!
          const got = streamed.channels[c]!
          let maxErr = 0
          let maxErrAt = -1
          for (let i = 0; i < ref.length; i++) {
            const err = Math.abs(got[i]! - ref[i]!)
            if (err > maxErr) {
              maxErr = err
              maxErrAt = i
            }
          }
          expect(maxErr, `channel ${c} diverged at frame ${maxErrAt}`).toBe(0)
        }

        // Boundary discontinuity: within ±5 ms (240 frames) of each window
        // edge, the streamed jump must equal the reference jump at the same
        // positions.
        for (const boundary of streamed.outputBoundaries) {
          for (let c = 0; c < 2; c++) {
            const jumpStream = maxJumpWithin(streamed.channels[c]!, boundary, 240)
            const jumpRef = maxJumpWithin(reference[c]!, boundary, 240)
            expect(jumpStream, `boundary ${boundary} ch ${c}`).toBe(jumpRef)
          }
        }

        // Source FIFO stays within 2 windows.
        expect(streamed.peakSourceFifoFrames).toBeLessThanOrEqual(2 * INPUT_WINDOW_FRAMES)
      },
      120_000,
    )
  }
})
