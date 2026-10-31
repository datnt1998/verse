// @vitest-environment node
import { describe, expect, it } from 'vitest'
import { PlanarFrameQueue } from './planar-queue'

describe('planar frame queue', () => {
  it('reads from the undrained head across multiple chunks', () => {
    const queue = new PlanarFrameQueue()
    queue.push(Float32Array.from([1, 2, 3, 4]))
    queue.push(Float32Array.from([5, 6, 7]))
    queue.drop(2)
    expect(queue.at(0)).toBe(3)
    const output = new Float32Array(5)
    expect(queue.readInto(output, 0, 0, 5)).toBe(5)
    expect(Array.from(output)).toEqual([3, 4, 5, 6, 7])
    queue.drop(3)
    expect(queue.at(0)).toBe(6)
    const tail = new Float32Array(4)
    expect(queue.readInto(tail, 1, 0, 4)).toBe(2)
    expect(Array.from(tail)).toEqual([0, 6, 7, 0])
  })
})
