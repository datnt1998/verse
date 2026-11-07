// @vitest-environment node

// Real coverage of the extracted export frame loop (replaces the old
// canvas-render-orchestrator.test.ts mock-echo, which re-implemented the loop
// shape on mocks without importing production code). End-to-end protection of
// the full orchestrator remains the headless chrome e2e (headless/test.mjs).

import { describe, it, expect, vi } from 'vitest'
import { runPipelinedFrameLoop, type PipelinedFrameTimings } from './pipelined-frame-loop'

interface Deferred {
  promise: Promise<void>
  resolve: () => void
  reject: (error: unknown) => void
}

function deferred(): Deferred {
  let resolve!: () => void
  let reject!: (error: unknown) => void
  const promise = new Promise<void>((res, rej) => {
    resolve = res
    reject = rej
  })
  return { promise, resolve, reject }
}

function tick(): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, 0))
}

interface FakeSample {
  frame: number
  closed: boolean
  close(): void
}

interface HarnessOptions {
  signal?: AbortSignal
  getPendingError?: () => unknown
  timings?: PipelinedFrameTimings
  captureImpl?: () => void
  renderImpl?: (frame: number) => void | Promise<void>
  encodeImpl?: (sample: FakeSample, keyFrame: boolean) => Promise<void>
}

function createHarness(totalFrames: number, opts: HarnessOptions = {}) {
  const events: string[] = []
  const samples: FakeSample[] = []

  const run = () =>
    runPipelinedFrameLoop<FakeSample>({
      totalFrames,
      signal: opts.signal,
      getPendingError: opts.getPendingError,
      timings: opts.timings,
      renderFrame: async (frame) => {
        events.push(`render-${frame}`)
        await opts.renderImpl?.(frame)
      },
      captureSample: (frame) => {
        opts.captureImpl?.()
        const sample: FakeSample = {
          frame,
          closed: false,
          close() {
            this.closed = true
            events.push(`close-${frame}`)
          },
        }
        samples.push(sample)
        events.push(`capture-${frame}`)
        return sample
      },
      encodeSample: (sample, keyFrame) => {
        events.push(`encode-start-${sample.frame}${keyFrame ? '-key' : ''}`)
        return (opts.encodeImpl?.(sample, keyFrame) ?? Promise.resolve()).then(() => {
          events.push(`encode-end-${sample.frame}`)
        })
      },
      onAbort: async () => {
        events.push('abort-cancel')
      },
      onFrameProgress: (frame) => {
        events.push(`progress-${frame}`)
      },
    })

  return { events, samples, run }
}

const indexOf = (events: string[], event: string) => {
  const index = events.indexOf(event)
  expect(index, `expected event "${event}" in trace ${JSON.stringify(events)}`).toBeGreaterThan(-1)
  return index
}

describe('runPipelinedFrameLoop', () => {
  it('counts render, capture and only residual encode waits including the final drain', async () => {
    let now = 0
    const clock = vi.spyOn(performance, 'now').mockImplementation(() => now)
    const timings: PipelinedFrameTimings = { renderMs: 0, captureMs: 0, encoderWaitMs: 0, frames: 0 }
    const encodes: Deferred[] = []
    try {
      const { run } = createHarness(2, {
        timings,
        renderImpl: () => { now += 10 },
        captureImpl: () => { now += 3 },
        encodeImpl: () => {
          const pending = deferred()
          encodes.push(pending)
          return pending.promise
        },
      })
      const running = run()
      await tick()
      // The second render has overlapped encode 0; none of that encode's
      // elapsed time belongs to the residual wait counter.
      expect(timings).toEqual({ renderMs: 20, captureMs: 3, encoderWaitMs: 0, frames: 1 })
      now += 7
      encodes[0]!.resolve()
      await tick()
      expect(timings).toEqual({ renderMs: 20, captureMs: 6, encoderWaitMs: 7, frames: 2 })
      now += 11
      encodes[1]!.resolve()
      await running
      expect(timings).toEqual({ renderMs: 20, captureMs: 6, encoderWaitMs: 18, frames: 2 })
    } finally {
      clock.mockRestore()
    }
  })

  it('retains elapsed render time without counting an uncaptured frame on failure', async () => {
    let now = 0
    const clock = vi.spyOn(performance, 'now').mockImplementation(() => now)
    const timings: PipelinedFrameTimings = { renderMs: 0, captureMs: 0, encoderWaitMs: 0, frames: 0 }
    const failure = new Error('render failed')
    try {
      const { run } = createHarness(1, {
        timings,
        renderImpl: () => { now += 9; throw failure },
      })
      await expect(run()).rejects.toBe(failure)
      expect(timings).toEqual({ renderMs: 9, captureMs: 0, encoderWaitMs: 0, frames: 0 })
    } finally {
      clock.mockRestore()
    }
  })

  it('encodes all frames in order and closes every sample', async () => {
    const { events, samples, run } = createHarness(5)
    await run()

    const starts = events.filter((e) => e.startsWith('encode-start-'))
    expect(starts).toEqual([
      'encode-start-0-key',
      'encode-start-1',
      'encode-start-2',
      'encode-start-3',
      'encode-start-4',
    ])
    expect(samples).toHaveLength(5)
    expect(samples.every((sample) => sample.closed)).toBe(true)
  })

  it('marks only frame 0 as a keyframe', async () => {
    const { events, run } = createHarness(3)
    await run()
    expect(events.filter((e) => e.includes('-key'))).toEqual(['encode-start-0-key'])
  })

  it('overlaps the next render with the in-flight encode', async () => {
    const encodes: Deferred[] = []
    const { events, run } = createHarness(3, {
      encodeImpl: () => {
        const d = deferred()
        encodes.push(d)
        return d.promise
      },
    })

    const running = run()
    await tick()
    // Encode 0 has not finished, yet render 1 already started — the overlap.
    expect(events).toContain('render-1')
    expect(events).not.toContain('encode-end-0')

    // But capture 1 must wait for encode 0 to drain.
    expect(events).not.toContain('capture-1')

    for (let i = 0; i < 3; i++) {
      encodes[i]?.resolve()
      await tick()
    }
    await running
    expect(events.filter((e) => e.startsWith('encode-end-'))).toHaveLength(3)
  })

  it('keeps at most one encode in flight: capture N waits for encode N-1', async () => {
    const { events, run } = createHarness(4)
    await run()

    for (let frame = 1; frame < 4; frame++) {
      const previousEncodeEnd = indexOf(events, `encode-end-${frame - 1}`)
      const capture = indexOf(events, `capture-${frame}`)
      const encodeStart = indexOf(events, `encode-start-${frame}`)
      expect(previousEncodeEnd).toBeLessThan(capture)
      expect(capture).toBeLessThan(encodeStart)
    }
  })

  it('reports progress once per frame, synchronously after the encode is kicked off', async () => {
    const { events, run } = createHarness(3)
    await run()

    for (let frame = 0; frame < 3; frame++) {
      expect(events.filter((e) => e === `progress-${frame}`)).toHaveLength(1)
      const encodeStart = events.findIndex((e) => e.startsWith(`encode-start-${frame}`))
      const progress = indexOf(events, `progress-${frame}`)
      expect(encodeStart).toBeLessThan(progress)
      // Progress fires before the encode settles — the encode is unawaited.
      expect(progress).toBeLessThan(indexOf(events, `encode-end-${frame}`))
    }
  })

  it('closes the sample and propagates the error when the encoder throws', async () => {
    const encoderError = new Error('encoder exploded')
    const { events, samples, run } = createHarness(4, {
      encodeImpl: (sample) =>
        sample.frame === 1 ? Promise.reject(encoderError) : Promise.resolve(),
    })

    await expect(run()).rejects.toBe(encoderError)
    // The failing encode was kicked off unawaited, so the next render still
    // overlapped it; the error surfaced at the drain point, before capture.
    expect(events).toContain('render-2')
    expect(events).not.toContain('capture-2')
    expect(samples[1]?.closed).toBe(true)
  })

  it('honours an abort signalled before the loop starts', async () => {
    const controller = new AbortController()
    controller.abort()
    const { events, run } = createHarness(5, { signal: controller.signal })

    const error = await run().then(
      () => null,
      (e: unknown) => e,
    )
    expect(error).toBeInstanceOf(DOMException)
    expect((error as DOMException).name).toBe('AbortError')
    expect((error as DOMException).message).toBe('Render cancelled')
    expect(events).toEqual(['abort-cancel'])
  })

  it('drains the in-flight encode on abort, swallowing its rejection', async () => {
    const controller = new AbortController()
    const encodes: Deferred[] = []
    const { events, samples, run } = createHarness(5, {
      signal: controller.signal,
      renderImpl: (frame) => {
        if (frame === 1) controller.abort()
      },
      encodeImpl: () => {
        const d = deferred()
        encodes.push(d)
        return d.promise
      },
    })

    const running = run()
    const outcome = running.then(
      () => null,
      (e: unknown) => e,
    )
    await tick()
    encodes[0]?.resolve()
    await tick()
    // Frame 1 was captured before the abort could be observed (abort is only
    // checked at the top of each iteration), and its encode is now in flight.
    expect(events).toContain('capture-1')

    // The in-flight encode fails during the abort drain — the loop must still
    // surface AbortError, not the encoder error.
    encodes[1]?.reject(new Error('encoder died mid-abort'))
    const error = await outcome
    expect((error as DOMException).name).toBe('AbortError')
    expect(events).toContain('abort-cancel')
    expect(indexOf(events, 'abort-cancel')).toBeGreaterThan(indexOf(events, 'capture-1'))
    expect(samples[1]?.closed).toBe(true)
    expect(events).not.toContain('render-2')
    expect(events).not.toContain('capture-2')
  })

  it('throws a pending error at the top of the next iteration', async () => {
    const pendingError = new Error('audio task failed')
    let raised: unknown
    const { events, run } = createHarness(5, {
      getPendingError: () => raised,
      renderImpl: (frame) => {
        if (frame === 1) raised = pendingError
      },
    })

    await expect(run()).rejects.toBe(pendingError)
    expect(events).toContain('progress-1')
    expect(events).not.toContain('render-2')
  })

  it('ignores falsy pending errors (truthiness semantics)', async () => {
    for (const falsy of [undefined, '', 0, null]) {
      const { samples, run } = createHarness(2, { getPendingError: () => falsy })
      await run()
      expect(samples).toHaveLength(2)
    }
  })

  it('resolves immediately for zero frames without touching any callback', async () => {
    const controller = new AbortController()
    controller.abort()
    const { events, run } = createHarness(0, {
      signal: controller.signal,
      getPendingError: () => new Error('never checked'),
    })
    await run()
    // Pre-loop abort/error checks are the caller's responsibility.
    expect(events).toEqual([])
  })

  it('drains the final in-flight encode before resolving', async () => {
    const lastEncode = deferred()
    const { samples, run } = createHarness(1, { encodeImpl: () => lastEncode.promise })

    let settled = false
    const running = run().then(() => {
      settled = true
    })
    await tick()
    expect(settled).toBe(false)

    lastEncode.resolve()
    await running
    expect(settled).toBe(true)
    expect(samples[0]?.closed).toBe(true)
  })
})
