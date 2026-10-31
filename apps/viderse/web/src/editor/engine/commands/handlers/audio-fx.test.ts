import { describe, expect, it } from 'vitest'
import { captureProjectState, commitProjectState, type ProjectState } from '../project-state'
import { createNodeSession } from '../node-session'
import { createSeededIdFactory } from '../id-factory'
import { registerAudioFxCommands } from './audio-fx'
import { DEFAULT_COMPRESSOR, DEFAULT_DELAY, DEFAULT_NOISE_REDUCTION, type AudioFxEffect } from '@editor/donor/openreel/audio/contracts'
import { learnNoiseProfile } from '@editor/features/audio-fx/noise-profile'
import { createCommandFixture } from '../test-fixture'
import { serializeCommand, deserializeCommand } from '../serializer'
import type { SerializedCommand } from '../types'
import { captureSnapshot, restoreSnapshot } from '@editor/test/freecut-snapshot'

registerAudioFxCommands()
function session() {
  return createNodeSession({
    ...captureProjectState(), items: [{ id: 'clip', type: 'audio', trackId: 'track', src: '/tone.wav', from: 0, durationInFrames: 900, label: 'Âm thanh' }],
    tracks: [{ id: 'track', name: 'Âm thanh', height: 60, locked: false, visible: true, muted: false, solo: false, order: 0, items: [] }],
    transitions: [], keyframes: [], markers: [], compositions: [], topLevelSequenceIds: [],
  }, createSeededIdFactory('audio'))
}
describe('audio FX commands', () => {
  it('adds updates reorders removes and undoes clip and track chains', () => {
    const current = session()
    try {
      const delay = { id: 'delay', type: 'delay', enabled: true, params: DEFAULT_DELAY }
      expect(current.executor.executeCommands([
        { type: 'audioFx.add', params: { target: { itemId: 'clip' }, effect: delay } },
        { type: 'audioFx.add', params: { target: { trackId: 'track' }, effect: delay } },
      ]).ok).toBe(true)
      expect(current.getState().items[0]!.audioFx?.[0]?.type).toBe('delay')
      expect(current.getState().tracks[0]!.audioFx).toHaveLength(1)
      expect(current.executor.executeCommands([{ type: 'audioFx.update', params: { target: { itemId: 'clip' }, effectId: 'delay', params: { time: 0.1 } } }]).ok).toBe(true)
      expect(current.getState().items[0]!.audioFx?.[0]?.params).toMatchObject({ time: 0.1 })
      current.executor.undo()
      expect(current.getState().items[0]!.audioFx?.[0]?.params).toMatchObject({ time: 0.5 })
      expect(current.executor.executeCommands([{ type: 'audioFx.reorder', params: { target: { itemId: 'clip' }, effectIds: ['delay'] } }]).ok).toBe(true)
      expect(current.executor.executeCommands([{ type: 'audioFx.remove', params: { target: { trackId: 'track' }, effectId: 'delay' } }]).ok).toBe(true)
      expect(current.getState().tracks[0]!.audioFx).toEqual([])
    } finally { current.dispose() }
  })
  it('rejects invalid parameters duplicate identifiers and incomplete permutations atomically', () => {
    const current = session()
    try {
      current.executor.executeCommands([{ type: 'audioFx.add', params: { target: { itemId: 'clip' }, effect: { id: 'd', type: 'delay', enabled: true, params: DEFAULT_DELAY } } }])
      const before = current.getState()
      expect(current.executor.executeCommands([{ type: 'audioFx.update', params: { target: { itemId: 'clip' }, effectId: 'd', params: { feedback: 1 } } }]).ok).toBe(false)
      expect(current.executor.executeCommands([{ type: 'audioFx.reorder', params: { target: { itemId: 'clip' }, effectIds: [] } }]).ok).toBe(false)
      expect(current.executor.executeCommands([{ type: 'audioFx.add', params: { target: { itemId: 'clip' }, effect: { id: 'd', type: 'delay', enabled: true, params: DEFAULT_DELAY } } }]).ok).toBe(false)
      expect(current.getState().items).toEqual(before.items)
    } finally { current.dispose() }
  })
  it('stores learned spectra as JSON numbers and requires a profile before enabling', () => {
    const current = session()
    try {
      expect(current.executor.executeCommands([{ type: 'audioFx.add', params: { target: { itemId: 'clip' }, effect: { id: 'n', type: 'noiseReduction', enabled: false, params: DEFAULT_NOISE_REDUCTION } } }]).ok).toBe(true)
      expect(current.executor.executeCommands([{ type: 'audioFx.update', params: { target: { itemId: 'clip' }, effectId: 'n', enabled: true } }]).ok).toBe(false)
      const profile = learnNoiseProfile([new Float32Array(8000).fill(0.01)], 8000, 0, 1)
      expect(current.executor.executeCommands([{ type: 'audioFx.learnNoiseProfile', params: { target: { itemId: 'clip' }, effectId: 'n', profile, range: { startSeconds: 0, endSeconds: 1 } } }]).ok).toBe(true)
      expect(current.executor.executeCommands([{ type: 'audioFx.update', params: { target: { itemId: 'clip' }, effectId: 'n', enabled: true } }]).ok).toBe(true)
      expect(JSON.parse(JSON.stringify(current.getState().items[0]!.audioFx))).toMatchObject([{ params: { profile: { magnitudes: profile.magnitudes } } }])
    } finally { current.dispose() }
  })
})

type AudioTarget = { itemId: string } | { trackId: string }
type AudioParityCase = 'add' | 'remove' | 'update' | 'reorder' | 'learnNoiseProfile'

function parityFixture(seed: number): ProjectState {
  const fixture = createCommandFixture(seed)
  const audioFx: AudioFxEffect[] = [
    { id: 'delay', type: 'delay', enabled: true, params: { ...DEFAULT_DELAY, time: 0.1 + seed % 10 * 0.01 } },
    { id: 'noise', type: 'noiseReduction', enabled: false, params: DEFAULT_NOISE_REDUCTION },
  ]
  return {
    ...fixture,
    items: fixture.items.map(item => item.id === 'audio-1' ? { ...item, audioFx: structuredClone(audioFx) } : item),
    tracks: fixture.tracks.map(track => track.id === 'track-audio' ? { ...track, audioFx: structuredClone(audioFx) } : track),
  }
}

function parityCommand(kind: AudioParityCase, target: AudioTarget, seed: number): SerializedCommand {
  switch (kind) {
    case 'add': return { type: 'audioFx.add', params: {
      target, effect: { id: `compressor-${seed}`, type: 'compressor', enabled: true, params: { ...DEFAULT_COMPRESSOR, threshold: -20 - seed % 10 } },
    } }
    case 'remove': return { type: 'audioFx.remove', params: { target, effectId: 'delay' } }
    case 'update': return { type: 'audioFx.update', params: { target, effectId: 'delay', enabled: false, params: { time: 0.6 + seed % 10 * 0.01, feedback: 0.7 } } }
    case 'reorder': return { type: 'audioFx.reorder', params: { target, effectIds: ['noise', 'delay'] } }
    case 'learnNoiseProfile': {
      const samples = Float32Array.from({ length: 1024 }, (_, i) => Math.sin(i * 0.17) * (0.01 + seed % 10 * 0.001))
      const profile = learnNoiseProfile([samples], 8000, 0, samples.length / 8000, 256)
      return { type: 'audioFx.learnNoiseProfile', params: { target, effectId: 'noise', profile, range: { startSeconds: 0, endSeconds: samples.length / 8000 } } }
    }
  }
}

function checkAudioFxParity(command: SerializedCommand, initial: ProjectState, seed: number): void {
  const idsSeed = `audio-fx-parity-${seed}`
  const current = createNodeSession(initial, createSeededIdFactory(idsSeed))
  try {
    const before = structuredClone(captureProjectState())
    const snapshot = captureSnapshot()
    const result = current.executor.executeCommands([command])
    expect(result, JSON.stringify(result)).toMatchObject({ ok: true })
    const after = structuredClone(captureProjectState())
    expect(after).not.toEqual(before)

    expect(current.executor.undo()).toMatchObject({ ok: true })
    const inverted = structuredClone(captureProjectState())
    expect(inverted).toEqual(before)
    expect(current.executor.redo()).toMatchObject({ ok: true })
    expect(captureProjectState()).toEqual(after)

    restoreSnapshot(snapshot)
    expect(captureProjectState()).toEqual(inverted)
    commitProjectState(before)
    const replay = createNodeSession(before, createSeededIdFactory(idsSeed))
    try {
      const restoredCommand = deserializeCommand(serializeCommand(command))
      expect(restoredCommand).toEqual(command)
      expect(replay.executor.executeCommands([restoredCommand])).toMatchObject({ ok: true })
      expect(captureProjectState()).toEqual(after)
    } finally { replay.dispose() }
  } finally { current.dispose() }
}

describe('audio FX snapshot undo redo and JSON replay parity', () => {
  const commands: AudioParityCase[] = ['add', 'remove', 'update', 'reorder', 'learnNoiseProfile']
  const targets: AudioTarget[] = [{ itemId: 'audio-1' }, { trackId: 'track-audio' }]
  for (const kind of commands) {
    for (const target of targets) {
      const targetKind = 'itemId' in target ? 'clip' : 'track'
      it(`audioFx.${kind} restores snapshots, redoes and replays on ${targetKind}`, () => {
        for (const seed of [1, 17, 911]) {
          checkAudioFxParity(parityCommand(kind, target, seed), parityFixture(seed), seed)
        }
      })
    }
  }
})
