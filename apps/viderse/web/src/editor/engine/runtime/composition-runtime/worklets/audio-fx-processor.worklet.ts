import { createAudioFxChain, type AudioFxChain } from '@editor/features/audio-fx/stages'
import type { AudioFxEffect } from '@editor/types/audio'

declare const sampleRate: number
declare const currentFrame: number
declare abstract class AudioWorkletProcessor {
  readonly port: MessagePort
  constructor(options?: unknown)
  abstract process(inputs: Float32Array[][], outputs: Float32Array[][]): boolean
}
declare function registerProcessor(name: string, ctor: new (options?: unknown) => AudioWorkletProcessor): void

class AudioFxProcessor extends AudioWorkletProcessor {
  private effects: AudioFxEffect[] = []
  private chain: AudioFxChain = createAudioFxChain([], 2, sampleRate)
  private zero = new Float32Array(128)
  private channels = 2
  private recording: { startFrame: number; frames: number; sent: number } | null = null
  constructor() {
    super()
    this.port.onmessage = (event: MessageEvent<
      { type: 'configure' | 'reset'; effects?: AudioFxEffect[] } |
      { type: 'record'; startFrame: number; frames: number }
    >) => {
      if (event.data.type === 'record') {
        this.recording = { startFrame: event.data.startFrame, frames: event.data.frames, sent: 0 }
        return
      }
      if (event.data.type === 'configure') this.effects = event.data.effects ?? []
      this.chain = createAudioFxChain(this.effects, this.channels, sampleRate)
    }
  }
  process(inputs: Float32Array[][], outputs: Float32Array[][]): boolean {
    const output = outputs[0]
    if (!output?.length) return true
    if (this.recording && currentFrame === this.recording.startFrame) {
      this.chain = createAudioFxChain(this.effects, this.channels, sampleRate)
    }
    if (this.channels !== output.length) {
      this.channels = output.length
      this.chain = createAudioFxChain(this.effects, this.channels, sampleRate)
    }
    const input = inputs[0]
    for (let channel = 0; channel < output.length; channel++) {
      const target = output[channel]!
      if (this.zero.length !== target.length) this.zero = new Float32Array(target.length)
      const source = input?.[channel] ?? input?.[0] ?? this.zero
      target.set(this.chain.processChunk(source, channel))
    }
    const recording = this.recording
    if (recording && currentFrame >= recording.startFrame && recording.sent < recording.frames) {
      const length = Math.min(output[0]!.length, recording.frames - recording.sent)
      const channels = output.map(channel => channel.slice(0, length).buffer)
      recording.sent += length
      this.port.postMessage({ type: 'record-chunk', channels, done: recording.sent === recording.frames }, channels)
      if (recording.sent === recording.frames) this.recording = null
    }
    return true
  }
}
registerProcessor('ytf-audio-fx', AudioFxProcessor)
