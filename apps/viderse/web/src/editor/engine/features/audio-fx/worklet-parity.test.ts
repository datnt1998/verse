import { readFileSync } from 'node:fs'
import { runInNewContext } from 'node:vm'
import { describe, expect, it } from 'vitest'
import { createAudioFxFixture, audioStats } from './diagnostic-fixture'
import { createAudioFxChain } from './stages'

interface WorkletInstance {
  port: { onmessage: (event: { data: unknown }) => void }
  process(inputs: Float32Array[][], outputs: Float32Array[][]): boolean
}
type WorkletConstructor = new () => WorkletInstance

describe('bundled realtime DSP', () => {
  it('actual built AudioWorklet matches export processors sample-for-sample', () => {
    const rate = 8000
    const fixture = createAudioFxFixture(rate, 4)
    let ctor: WorkletConstructor | undefined
    const sandbox = {
      sampleRate: rate, currentFrame: 0,
      AudioWorkletProcessor: class { port: { onmessage: (event: { data: unknown }) => void; postMessage: () => void } = { onmessage: () => {}, postMessage: () => {} } },
      registerProcessor(_name: string, registered: WorkletConstructor) { ctor = registered },
      Float32Array, Float64Array, Uint32Array, Math,
    }
    runInNewContext(readFileSync('public/editor-engine/audio-fx-processor.worklet.js', 'utf8'), sandbox)
    expect(ctor).toBeDefined()
    const worklet = new ctor!()
    worklet.port.onmessage({ data: { type: 'configure', effects: fixture.effects } })
    const exportChain = createAudioFxChain(fixture.effects, 2, rate)
    const expected = fixture.channels.map((channel, index) => Float32Array.from(exportChain.processChunk(channel, index)))
    const actual = fixture.channels.map(channel => new Float32Array(channel.length))
    for (let frame = 0; frame < fixture.channels[0]!.length; frame += 128) {
      const input = fixture.channels.map(channel => channel.subarray(frame, frame + 128))
      const output = input.map(channel => new Float32Array(channel.length))
      sandbox.currentFrame = frame
      worklet.process([input], [output])
      output.forEach((channel, index) => actual[index]!.set(channel, frame))
    }
    expect(actual).toEqual(expected)
    expect(audioStats(actual)).toEqual(audioStats(expected))
  })
  it('retains fixed typed-array storage while duration grows', () => {
    let allocatedBytes = 0
    class MeteredFloat32Array extends Float32Array {
      constructor(value: number | ArrayBuffer, offset?: number, length?: number) {
        if (typeof value === 'number') {
          super(value)
          allocatedBytes += value * 4
        } else super(value, offset, length)
      }
    }
    const rate = 8000
    const fixture = createAudioFxFixture(rate, 4)
    let ctor: WorkletConstructor | undefined
    runInNewContext(readFileSync('public/editor-engine/audio-fx-processor.worklet.js', 'utf8'), {
      sampleRate: rate, currentFrame: 0,
      AudioWorkletProcessor: class { port: { onmessage: (event: { data: unknown }) => void; postMessage: () => void } = { onmessage: () => {}, postMessage: () => {} } },
      registerProcessor(_name: string, registered: WorkletConstructor) { ctor = registered },
      Float32Array: MeteredFloat32Array, Float64Array, Uint32Array, Math,
    })
    const worklet = new ctor!()
    worklet.port.onmessage({ data: { type: 'configure', effects: fixture.effects } })
    const channels = [new Float32Array(128), new Float32Array(128)]
    worklet.process([channels], [channels])
    const afterWarmup = allocatedBytes
    for (let quantum = 0; quantum < 5000; quantum++) worklet.process([channels], [channels])
    expect(allocatedBytes).toBe(afterWarmup)
  })
})
