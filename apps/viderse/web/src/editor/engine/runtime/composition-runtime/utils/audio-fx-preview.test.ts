import { afterEach, describe, expect, it, vi } from 'vitest'
import { createPreviewFxRouting } from './audio-fx-preview'
import { useItemsStore } from '@editor/features/timeline/stores/items-store'
import { usePlaybackStore } from '@editor/shared/state/playback'
import { DEFAULT_COMPRESSOR } from '@editor/donor/openreel/audio/contracts'
import type { TimelineItem, TimelineTrack } from '@editor/types/timeline'

class Gain {
  gain = { value: 1 }
  targets: unknown[] = []
  connect(target: unknown) { this.targets.push(target) }
  disconnect() { this.targets = [] }
}
class Worklet extends Gain {
  port = { postMessage: vi.fn(), close: vi.fn() }
}
function context() {
  const gains: Gain[] = []
  const destination = {}
  const addModule = vi.fn(async () => {})
  return { gains, destination, addModule, audio: {
    sampleRate: 48000, audioWorklet: { addModule }, destination,
    createGain() { const node = new Gain(); gains.push(node); return node },
  } as unknown as AudioContext }
}
afterEach(() => vi.unstubAllGlobals())
describe('preview clip and track FX routing', () => {
  it('empty FX never loads a worklet module', async () => {
    const ctx = context()
    const route = createPreviewFxRouting(ctx.audio)
    try {
      await route.ready
      expect(ctx.addModule).not.toHaveBeenCalled()
    } finally { route.dispose() }
  })
  it('sums two clips into one nonlinear track bus and applies master gain afterwards', async () => {
    const beforeItems = useItemsStore.getState()
    const beforePlayback = usePlaybackStore.getState()
    const effect = { id: 'compressor', type: 'compressor' as const, enabled: true, params: DEFAULT_COMPRESSOR }
    const items: TimelineItem[] = ['a', 'b'].map(id => ({ id, type: 'audio', src: '/tone.wav', trackId: 'track', from: 0, durationInFrames: 300, label: id, audioFx: [effect] }))
    const track: TimelineTrack = { id: 'track', name: 'Âm thanh', height: 64, order: 0, locked: false, visible: true, muted: false, solo: false, items, audioFx: [effect] }
    useItemsStore.setState({ items, tracks: [track] })
    usePlaybackStore.setState({ isPlaying: false, volume: 0.5, muted: false, masterBusDb: 6 })
    const worklets: Worklet[] = []
    vi.stubGlobal('AudioWorkletNode', class extends Worklet { constructor() { super(); worklets.push(this) } })
    const ctx = context()
    const a = createPreviewFxRouting(ctx.audio, 'a')
    const b = createPreviewFxRouting(ctx.audio, 'b')
    try {
      await Promise.all([a.ready, b.ready])
      const sourceA = new Gain()
      const sourceB = new Gain()
      a.connectOutput(sourceA as unknown as AudioNode)
      b.connectOutput(sourceB as unknown as AudioNode)
      expect(sourceA.targets[0]).toBe(sourceB.targets[0])
      expect(worklets).toHaveLength(3)
      const master = ctx.gains.find(node => node.targets.includes(ctx.destination))!
      expect(master.gain.value).toBeCloseTo(Math.pow(10, 6 / 20) * 0.5)
      const trackInput = sourceA.targets[0] as Gain
      expect(trackInput.targets[0]).toBeInstanceOf(Worklet)
      usePlaybackStore.setState({ muted: true })
      expect(master.gain.value).toBe(0)
      expect(trackInput.gain.value).toBe(1)
    } finally {
      a.dispose(); b.dispose()
      useItemsStore.setState(beforeItems)
      usePlaybackStore.setState(beforePlayback)
    }
  })
})
