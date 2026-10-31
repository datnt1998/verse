// @vitest-environment node
import { describe, expect, it } from 'vitest'
import { compositionFrameForOutput, outputFrameCount } from './output-frame-clock'

describe('output frame clock', () => {
  it.each([[60, 30, 600, 300], [30, 60, 300, 600], [25, 30, 250, 300], [29.97, 30, 2997, 3000]])('maps %s fps to %s fps without changing elapsed time', (source, target, duration, count) => {
    expect(outputFrameCount(duration, source, target)).toBe(count)
    expect(compositionFrameForOutput(0, source, target, duration)).toBe(0)
    for (let i = 0; i < count; i++) {
      expect(compositionFrameForOutput(i, source, target, duration)).toBe(Math.min(duration - 1, Math.floor(i * source / target + 1e-9)))
    }
  })
  it('duplicates or drops frames and clamps the fractional final frame', () => {
    expect([0, 1, 2, 3].map(i => compositionFrameForOutput(i, 60, 30, 600))).toEqual([0, 2, 4, 6])
    expect([0, 1, 2, 3].map(i => compositionFrameForOutput(i, 30, 60, 300))).toEqual([0, 0, 1, 1])
    expect(compositionFrameForOutput(3000, 29.97, 30, 2997)).toBe(2996)
  })
})
