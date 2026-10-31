/**
 * RSS measurement driver for the windowed DSP tests.
 *
 * Runs the StreamingSegmentProcessor over a synthesized (not stored) source:
 * windows are generated on demand so the source itself never counts toward
 * the measured footprint — matching the phase-1 spike methodology, where the
 * PCM reader streamed from disk. Peak RSS is sampled after every processed
 * output window.
 *
 * Invoked as a child process by the vitest case so the measurement is not
 * polluted by the test runner's own heap.
 *
 *   tsx windowed-dsp-memory-run.ts <sourceSeconds> [mode]
 *
 * Prints a single JSON line: { sourceSeconds, windows, peakRssBytes, emittedFrames }
 */
import { StreamingSegmentProcessor } from './streaming-segment-dsp'
import type { ResolvedAudioEqSettings } from '@editor/types/audio'

const SAMPLE_RATE = 48_000
const WINDOW_FRAMES = 30 * SAMPLE_RATE

const EQ_STAGE: ResolvedAudioEqSettings = {
  outputGainDb: 0,
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
  lowMidEnabled: false,
  lowMidType: 'peaking',
  lowMidGainDb: 0,
  lowMidFrequencyHz: 400,
  lowMidQ: 1.4,
  midGainDb: -2,
  highMidEnabled: false,
  highMidType: 'peaking',
  highMidGainDb: 0,
  highMidFrequencyHz: 1600,
  highMidQ: 1.1,
  highEnabled: true,
  highType: 'high-shelf',
  highGainDb: -2,
  highFrequencyHz: 2800,
  highQ: 2.3,
  band6Enabled: false,
  band6Type: 'low-shelf',
  band6FrequencyHz: 14000,
  band6GainDb: 0,
  band6Q: 1.1,
  band6SlopeDbPerOct: 12,
  highCutEnabled: false,
  highCutFrequencyHz: 22000,
  highCutSlopeDbPerOct: 12,
}

/** Stateless signal — formula only, so the source never lives in memory. */
function fillWindow(left: Float32Array, right: Float32Array, startFrame: number): void {
  for (let i = 0; i < left.length; i++) {
    const t = (startFrame + i) / SAMPLE_RATE
    left[i] = 0.4 * Math.sin(2 * Math.PI * 220 * t) + 0.15 * Math.sin(2 * Math.PI * 1800 * t)
    right[i] =
      0.4 * Math.sin(2 * Math.PI * 220 * t + 0.3) + 0.15 * Math.sin(2 * Math.PI * 2500 * t + 1)
  }
}
interface RunResult {
  sourceSeconds: number
  windows: number
  peakRssBytes: number
  settledRssBytes: number
  emittedFrames: number
}

export function measureStreamingPeakRss(sourceSeconds: number): RunResult {
  const sourceFrames = sourceSeconds * SAMPLE_RATE
  // speed 0.75 + EQ: the heaviest case — more output than input, stateful EQ,
  // SoundTouch FIFOs.
  const dsp = new StreamingSegmentProcessor({
    sampleRate: SAMPLE_RATE,
    channels: 2,
    sourceFrames,
    speed: 0.75,
    isReversed: false,
    pitchShiftSemitones: 0,
    audioEqStages: [EQ_STAGE],
  })

  const pushLeft = new Float32Array(WINDOW_FRAMES)
  const pushRight = new Float32Array(WINDOW_FRAMES)
  let served = 0
  let emitted = 0
  let windows = 0
  let peakRss = process.memoryUsage().rss

  for (;;) {
    const want = Math.min(WINDOW_FRAMES, dsp.expectedOutputFrames - emitted)
    if (want <= 0) break
    const pulled = dsp.pullOutput(want)
    if (pulled && pulled[0]!.length > 0) {
      emitted += pulled[0]!.length
      windows += 1
      // Collect unreachable previous-window buffers before sampling. This
      // isolates duration-independent pipeline state from V8 GC scheduling.
      // The current output window remains live and is included in RSS.
      globalThis.gc?.()
      peakRss = Math.max(peakRss, process.memoryUsage().rss)
      continue
    }
    if (dsp.inputFinished) break
    if (dsp.sourceExhausted) {
      dsp.finishInput()
      continue
    }
    const frames = Math.min(WINDOW_FRAMES, sourceFrames - served)
    if (frames <= 0) {
      dsp.finishInput()
      continue
    }
    fillWindow(
      pushLeft.subarray(0, frames) as Float32Array,
      pushRight.subarray(0, frames) as Float32Array,
      served,
    )
    dsp.pushInput(
      [
        pushLeft.subarray(0, frames) as Float32Array,
        pushRight.subarray(0, frames) as Float32Array,
      ],
      frames,
    )
    served += frames
  }

  const settledPeak = process.memoryUsage().rss
  globalThis.gc?.()
  globalThis.gc?.()
  return {
    sourceSeconds,
    windows,
    peakRssBytes: Math.max(peakRss, settledPeak),
    settledRssBytes: process.memoryUsage().rss,
    emittedFrames: emitted,
  }
}

const isMain = process.argv[1]?.endsWith('windowed-dsp-memory-run.ts')
if (isMain) {
  const seconds = Number(process.argv[2] ?? 120)
  const result = measureStreamingPeakRss(seconds)
  console.log(JSON.stringify(result))
}
