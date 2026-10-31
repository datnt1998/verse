import { describe, expect, it } from 'vitest'
import { f32leBytesPerSecond, interleavePlanarToF32le, pcmDurationSec, planPcmChunks } from './audio-measurement'

describe('interleavePlanarToF32le', () => {
  it('interleaves stereo samples in channel order', () => {
    const left = new Float32Array([0.5, -0.5, 0.25])
    const right = new Float32Array([0.1, -0.1, 0.9])
    const bytes = interleavePlanarToF32le([left, right])
    const view = new Float32Array(bytes.buffer, bytes.byteOffset, bytes.byteLength / 4)
    expect([...view]).toEqual([left[0], right[0], left[1], right[1], left[2], right[2]])
    expect(bytes.byteLength).toBe(3 * 2 * 4)
  })

  it('returns empty for no channels', () => {
    expect(interleavePlanarToF32le([]).byteLength).toBe(0)
  })
})

describe('pcmDurationSec', () => {
  it('computes exact seconds and rejects misaligned streams', () => {
    expect(pcmDurationSec(f32leBytesPerSecond(48_000, 2) * 3, 48_000, 2)).toBe(3)
    expect(pcmDurationSec(10, 48_000, 2)).toBeNull()
    expect(pcmDurationSec(8, 48_000, 0)).toBeNull()
  })
})

describe('planPcmChunks', () => {
  it('splits windows into frame-aligned slices covering every byte', () => {
    const chunks = planPcmChunks(1_000_000, 8, 300_000)
    expect(chunks[0]).toEqual({ offset: 0, length: 300_000 })
    const covered = chunks.reduce((sum, c) => sum + c.length, 0)
    expect(covered).toBe(1_000_000)
    for (const chunk of chunks) {
      expect(chunk.offset % 8).toBe(0)
      expect(chunk.length).toBeGreaterThan(0)
    }
    expect(chunks[chunks.length - 1]!.length % 8).toBe(1_000_000 % 8 === 0 ? 0 : 1_000_000 % 8)
  })

  it('handles windows smaller than the chunk size', () => {
    const chunks = planPcmChunks(80, 8, 300_000)
    expect(chunks).toEqual([{ offset: 0, length: 80 }])
  })
})
