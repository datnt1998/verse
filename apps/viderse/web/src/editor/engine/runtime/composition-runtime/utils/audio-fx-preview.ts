import type { AudioFxEffect } from '@editor/types/audio'
import { useItemsStore } from '@editor/features/timeline/stores/items-store'
import { useCompositionsStore } from '@editor/features/timeline/stores/compositions-store'
import { usePlaybackStore } from '@editor/shared/state/playback'
import { getAudioFxTailFrames } from '@editor/features/audio-fx/timing'

const loads = new WeakMap<AudioContext, Promise<void>>()
export function ensureAudioFxWorkletLoaded(context: AudioContext): Promise<void> {
  let pending = loads.get(context)
  if (!pending) {
    pending = context.audioWorklet.addModule('/editor-engine/audio-fx-processor.worklet.js')
    loads.set(context, pending)
  }
  return pending
}

function findOwner(itemId: string) {
  const current = useItemsStore.getState()
  const item = current.items.find(candidate => candidate.id === itemId)
  if (item) return { item, track: current.tracks.find(track => track.id === item.trackId) }
  for (const composition of useCompositionsStore.getState().compositions) {
    const nested = composition.items.find(candidate => candidate.id === itemId)
    if (nested) return { item: nested, track: composition.tracks.find(track => track.id === nested.trackId) }
  }
  return undefined
}

interface EffectNodes {
  input: GainNode
  output: GainNode
  ready: Promise<void>
  configure(effects: readonly AudioFxEffect[] | undefined): void
  reset(): void
  dispose(): void
}
function createEffectNodes(context: AudioContext, initial?: readonly AudioFxEffect[]): EffectNodes {
  const input = context.createGain()
  const output = context.createGain()
  let worklet: AudioWorkletNode | undefined
  let effects = initial ?? []
  let signature = ''
  let disposed = false
  let generation = 0
  const reconnect = () => {
    input.disconnect()
    worklet?.disconnect()
    if (worklet && effects.some(effect => effect.enabled)) {
      input.connect(worklet)
      worklet.connect(output)
    } else input.connect(output)
    input.gain.value = 1
  }
  let pending: Promise<void> | undefined
  const load = (): Promise<void> => {
    if (!pending) pending = ensureAudioFxWorkletLoaded(context).then(() => {
      if (disposed) return
      worklet = new AudioWorkletNode(context, 'ytf-audio-fx', {
        numberOfInputs: 1, numberOfOutputs: 1, outputChannelCount: [2], channelCount: 2,
        channelCountMode: 'explicit',
      })
      worklet.port.postMessage({ type: 'configure', effects })
      reconnect()
    })
    return pending
  }
  const ready = effects.some(effect => effect.enabled) ? load() : Promise.resolve()
  // Do not play an unprocessed opening while the worklet module loads.
  input.gain.value = effects.some(effect => effect.enabled) ? 0 : 1
  input.connect(output)
  return {
    input, output, ready,
    configure(next) {
      effects = next ?? []
      const nextSignature = JSON.stringify(effects)
      if (nextSignature === signature) return
      signature = nextSignature
      generation++
      worklet?.port.postMessage({ type: 'configure', effects })
      if (worklet) reconnect()
      else if (effects.some(effect => effect.enabled)) {
        input.gain.value = 0
        void load()
      }
    },
    reset() { worklet?.port.postMessage({ type: 'reset', generation }) },
    dispose() {
      disposed = true
      input.disconnect()
      worklet?.disconnect()
      worklet?.port.close()
      output.disconnect()
    },
  }
}

interface MasterBus { node: GainNode; users: number; unsubscribe: () => void }
const masters = new WeakMap<AudioContext, MasterBus>()
function acquireMaster(context: AudioContext): { node: GainNode; release: () => void } {
  let master = masters.get(context)
  if (!master) {
    const node = context.createGain()
    const update = () => {
      const state = usePlaybackStore.getState()
      node.gain.value = state.muted ? 0 : Math.pow(10, state.masterBusDb / 20) * state.volume
    }
    update()
    const unsubscribe = usePlaybackStore.subscribe((next, previous) => {
      if (next.muted !== previous.muted || next.masterBusDb !== previous.masterBusDb || next.volume !== previous.volume) update()
    })
    node.connect(context.destination)
    master = { node, users: 0, unsubscribe }
    masters.set(context, master)
  }
  master.users++
  const retained = master
  return { node: master.node, release() {
    if (--retained.users === 0) {
      retained.unsubscribe()
      retained.node.disconnect()
      masters.delete(context)
    }
  } }
}
interface TrackBus {
  nodes: EffectNodes
  users: number
  tailFrames: number
  disposal?: number
  unsubscribe: () => void
  releaseMaster: () => void
}
const buses = new WeakMap<AudioContext, Map<string, TrackBus>>()
function trackBus(context: AudioContext, id: string, effects?: readonly AudioFxEffect[]): TrackBus {
  let registry = buses.get(context)
  if (!registry) { registry = new Map(); buses.set(context, registry) }
  let bus = registry.get(id)
  if (!bus) {
    const nodes = createEffectNodes(context, effects)
    const unsubscribe = usePlaybackStore.subscribe((next, previous) => {
      if (next.isPlaying !== previous.isPlaying || next.playbackRate !== previous.playbackRate ||
          (!next.isPlaying && next.currentFrame !== previous.currentFrame) ||
          Math.abs(next.currentFrame - previous.currentFrame) > 8) nodes.reset()
    })
    const master = acquireMaster(context)
    bus = { nodes, users: 0, tailFrames: 0, unsubscribe, releaseMaster: master.release }
    bus.nodes.output.connect(master.node)
    registry.set(id, bus)
  }
  bus.nodes.configure(effects)
  if (bus.disposal) { clearTimeout(bus.disposal); bus.disposal = undefined }
  bus.tailFrames = getAudioFxTailFrames(effects, context.sampleRate)
  return bus
}

function releaseTrackBus(context: AudioContext, id: string, bus: TrackBus): void {
  if (--bus.users > 0) return
  const close = () => {
    if (bus.users > 0) return
    bus.unsubscribe()
    bus.nodes.dispose()
    bus.releaseMaster()
    if (buses.get(context)?.get(id) === bus) buses.get(context)?.delete(id)
  }
  if (usePlaybackStore.getState().isPlaying && bus.tailFrames > 0) {
    bus.disposal = window.setTimeout(close, bus.tailFrames / context.sampleRate * 1000)
  } else close()
}

export interface PreviewFxRouting {
  input: GainNode
  output: GainNode
  ready: Promise<void>
  readonly tailFrames: number
  connectOutput(source: AudioNode): void
  dispose(): void
}

/** One clip chain plus a shared nonlinear bus AFTER clips have been summed.
 * Sources without a project item id keep the ordinary direct destination.
 */
export function createPreviewFxRouting(context: AudioContext, itemId?: string): PreviewFxRouting {
  const owner = itemId ? findOwner(itemId) : undefined
  const clip = createEffectNodes(context, owner?.item.audioFx)
  let bus = owner?.track ? trackBus(context, owner.track.id, owner.track.audioFx) : undefined
  let trackId = owner?.track?.id
  if (bus) bus.users++
  let master = bus ? undefined : acquireMaster(context)
  let sourceOutput: AudioNode | undefined
  let clipTailFrames = getAudioFxTailFrames(owner?.item.audioFx, context.sampleRate)
  const connectOutput = (source: AudioNode) => {
    sourceOutput = source
    source.connect(bus?.nodes.input ?? master!.node)
  }
  const refresh = () => {
    const next = itemId ? findOwner(itemId) : undefined
    clip.configure(next?.item.audioFx)
    clipTailFrames = getAudioFxTailFrames(next?.item.audioFx, context.sampleRate)
    if (next?.track?.id !== trackId) {
      sourceOutput?.disconnect()
      if (bus && trackId) releaseTrackBus(context, trackId, bus)
      master?.release()
      trackId = next?.track?.id
      bus = next?.track ? trackBus(context, next.track.id, next.track.audioFx) : undefined
      if (bus) bus.users++
      master = bus ? undefined : acquireMaster(context)
      sourceOutput?.connect(bus?.nodes.input ?? master!.node)
    }
    if (next?.track && bus) {
      bus.nodes.configure(next.track.audioFx)
      bus.tailFrames = getAudioFxTailFrames(next.track.audioFx, context.sampleRate)
    }
  }
  const unsubscribeItems = useItemsStore.subscribe(refresh)
  const unsubscribeCompositions = useCompositionsStore.subscribe(refresh)
  const unsubscribePlayback = usePlaybackStore.subscribe((next, previous) => {
    if (next.isPlaying !== previous.isPlaying || next.playbackRate !== previous.playbackRate ||
        (!next.isPlaying && next.currentFrame !== previous.currentFrame) ||
        Math.abs(next.currentFrame - previous.currentFrame) > 8) {
      clip.reset()
    }
  })
  return {
    input: clip.input, output: clip.output,
    ready: Promise.all([clip.ready, bus?.nodes.ready]).then(() => undefined),
    get tailFrames() { return clipTailFrames },
    connectOutput,
    dispose() {
      unsubscribeItems()
      unsubscribeCompositions()
      unsubscribePlayback()
      sourceOutput?.disconnect()
      clip.dispose()
      master?.release()
      if (bus && trackId) releaseTrackBus(context, trackId, bus)
      bus = undefined
    },
  }
}
