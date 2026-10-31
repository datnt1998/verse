// @vitest-environment node

/**
 * Memory bound for windowed DSP: peak RSS while streaming a 6-minute source
 * through the heaviest case (0.75× speed + EQ) must exceed the 2-minute peak
 * by no more than 16 MB — the windowed pipeline's state must not grow with
 * clip duration.
 *
 * Method (documented per the phase contract): the measured run executes in a
 * fresh `tsx` child process so the vitest runner's heap cannot pollute the
 * number; the source signal is generated per-window from a closed-form sine
 * sum (never stored), and peak RSS is sampled with
 * `process.memoryUsage().rss` after every 30 s output window. Explicit GC
 * before sampling discards unreachable previous windows, while the current
 * output and DSP buffers remain live; V8 scheduling is not part of the bound.
 */
import { describe, expect, it } from 'vitest'
import { execFileSync } from 'node:child_process'
import { fileURLToPath } from 'node:url'

const RUN_SCRIPT = fileURLToPath(
  new URL('./windowed-dsp-memory-run.ts', import.meta.url),
)

function measurePeakRss(sourceSeconds: number): { peakRssBytes: number; settledRssBytes: number } {
  const out = execFileSync(
    process.execPath,
    ['--expose-gc', '--import', 'tsx', RUN_SCRIPT, String(sourceSeconds)],
    { encoding: 'utf8', maxBuffer: 4 * 1024 * 1024 },
  ).trim()
  const result = JSON.parse(out) as {
    peakRssBytes: number
    settledRssBytes: number
    windows: number
  }
  expect(result.windows).toBeGreaterThan(0)
  return result
}

describe('windowed DSP memory bound', () => {
  it(
    'peak RSS for 6 min of audio stays within 16 MB of the 2 min run',
    () => {
      const short = measurePeakRss(120)
      const long = measurePeakRss(360)
      // Peak sampled at window boundaries with current output still live.
      const deltaBytes = long.peakRssBytes - short.peakRssBytes
      expect(deltaBytes).toBeLessThanOrEqual(16 * 1024 * 1024)
      // Sanity: the windowed peak must not be wild — under 512 MB total.
      expect(long.peakRssBytes).toBeLessThan(512 * 1024 * 1024)
    },
    300_000,
  )
})
