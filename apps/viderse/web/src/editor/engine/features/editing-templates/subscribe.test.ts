import { describe, expect, it } from 'vitest'
import { createNodeSession } from '@editor/commands/node-session'
import { captureProjectState } from '@editor/commands/project-state'
import { createSeededIdFactory } from '@editor/commands/id-factory'
import { registerAllCommands } from '@editor/commands/handlers'
import type { TimelineItem } from '@editor/types/timeline'
import { buildSubscribeCommands, subscribeGeometry, subscribeTiming } from './subscribe'

registerAllCommands()

const baseContext = {
  idPrefix: 'clone',
  trackIds: ['t-back', 't-icon', 't-text'] as [string, string, string],
  from: 45,
  durationInFrames: 111,
  fps: 30,
  width: 1920,
  height: 1080,
}

/** The preset emits full item payloads in `item.add` params. */
type ItemAddParams = { item: TimelineItem & Record<string, unknown> }
/** keyframe.add params as emitted by the preset builder. */
type KeyParams = { itemId: string; property: string; frame: number; value: number }

function emptyState() {
  return {
    ...captureProjectState(),
    items: [],
    tracks: [
      { id: 't-back', name: 'Nền', kind: 'video' as const, height: 64, locked: false, visible: true, muted: false, solo: false, order: 0, items: [] },
      { id: 't-icon', name: 'Icon', kind: 'video' as const, height: 64, locked: false, visible: true, muted: false, solo: false, order: 1, items: [] },
      { id: 't-text', name: 'Chữ', kind: 'video' as const, height: 64, locked: false, visible: true, muted: false, solo: false, order: 2, items: [] },
    ],
    transitions: [],
    keyframes: [],
    markers: [],
    compositions: [],
    topLevelSequenceIds: [],
    mediaItems: [],
  }
}

describe('subscribe preset timing', () => {
  it('splits ~0.4s in / ~3s hold / ~0.3s out at 30fps', () => {
    const timing = subscribeTiming(111, 30)
    expect(timing).toEqual({ introFrames: 12, holdFrames: 90, outroFrames: 9 })
    expect(timing.introFrames + timing.holdFrames + timing.outroFrames).toBe(111)
  })

  it('clamps inside short clips instead of exceeding their duration', () => {
    for (const duration of [1, 2, 15, 40]) {
      const timing = subscribeTiming(duration, 30)
      expect(timing.introFrames + timing.holdFrames + timing.outroFrames).toBe(duration)
      expect(timing.introFrames).toBeLessThanOrEqual(Math.max(1, duration - 1))
    }
  })
})

describe('buildSubscribeCommands', () => {
  it('emits deterministic ids and a single linked group', () => {
    const result = buildSubscribeCommands(baseContext)
    expect(result.itemIds).toEqual(['clone-subscribe-backdrop', 'clone-subscribe-play', 'clone-subscribe-text'])
    expect(result.groupId).toBe('clone-subscribe-group')
    const adds = result.commands.filter((command) => command.type === 'item.add')
    expect(adds).toHaveLength(3)
    const items = adds.map((command) => {
      const params = command.params as ItemAddParams
      return params.item
    })
    expect(items.map((item) => item.type)).toEqual(['shape', 'shape', 'text'])
    expect(items.every((item) => item.linkedGroupId === 'clone-subscribe-group')).toBe(true)
    expect(items[0]).toMatchObject({ shapeType: 'rectangle', cornerRadius: expect.any(Number) })
    expect(items[1]).toMatchObject({ shapeType: 'triangle', direction: 'right' })
    expect(items[2]).toMatchObject({ text: 'Subscribe' })
    // No third-party asset: nothing references lottie/media/src.
    for (const item of items) {
      expect(item.mediaId).toBeUndefined()
      expect(item.src).toBeUndefined()
    }
  })

  it('keyframes fade/slide in, hold and fade out inside the item duration', () => {
    const result = buildSubscribeCommands(baseContext)
    const keys = result.commands.filter((command) => command.type === 'keyframe.add')
    const perItem = new Map<string, KeyParams[]>()
    for (const command of keys) {
      const params = command.params as KeyParams
      expect(params.frame).toBeLessThanOrEqual(110)
      const list = perItem.get(params.itemId) ?? []
      list.push(params)
      perItem.set(params.itemId, list)
    }
    for (const itemId of result.itemIds) {
      const list = perItem.get(itemId)!
      const opacity = list.filter((k) => k.property === 'opacity')
      expect(opacity.at(0)).toMatchObject({ frame: 0, value: 0 })
      expect(opacity.some((k) => k.frame === 12 && k.value === 1)).toBe(true)
      expect(opacity.at(-1)).toMatchObject({ frame: 110, value: 0 })
      const xs = list.filter((k) => k.property === 'x')
      expect(xs[0]!.value).toBeGreaterThan(xs[1]!.value) // slides in from the right
    }
  })
  it('places the pill in the safe top-right corner', () => {
    const geo = subscribeGeometry({ width: 1920, height: 1080 })
    const result = buildSubscribeCommands(baseContext)
    const add = result.commands[0]!.params as ItemAddParams
    const backdrop = add.item
    // Transform x/y are center-based: right edge lands one margin inside the canvas.
    const rightEdge = geo.x + geo.buttonWidth / 2
    const topEdge = geo.y - geo.buttonHeight / 2
    expect(rightEdge).toBeCloseTo(1920 / 2 - geo.margin, 0)
    expect(topEdge).toBeCloseTo(-1080 / 2 + geo.margin, 0)
    expect(backdrop.transform!.height).toBe(geo.buttonHeight)
  })

  it('applies through template.subscribe as canonical commands in one session', () => {
    const session = createNodeSession(emptyState(), createSeededIdFactory('subscribe'))
    try {
      const outcome = session.executor.executeCommands([{ type: 'template.subscribe', params: baseContext }])
      expect(outcome.ok).toBe(true)
      const state = session.getState()
      expect(state.items).toHaveLength(3)
      const byId = Object.fromEntries(state.items.map((item) => [item.id, item]))
      expect(byId['clone-subscribe-backdrop']).toMatchObject({ type: 'shape', trackId: 't-back', from: 45, durationInFrames: 111 })
      expect(byId['clone-subscribe-text']).toMatchObject({ type: 'text', trackId: 't-text', text: 'Subscribe' })
      expect(state.items.every((item) => item.linkedGroupId === 'clone-subscribe-group')).toBe(true)
      expect(state.keyframes.length).toBeGreaterThanOrEqual(3)
      // Replay the serialized result on a fresh session: deterministic state.
      const replayed = createNodeSession(emptyState(), createSeededIdFactory('replay'))
      try {
        expect(replayed.executor.executeCommands(JSON.parse(JSON.stringify(buildSubscribeCommands(baseContext).commands))).ok).toBe(true)
        expect(replayed.getState().items).toEqual(state.items)
      } finally { replayed.dispose() }
    } finally { session.dispose() }
  })

  it('rejects when a target track is missing or audio', () => {
    const session = createNodeSession(emptyState(), createSeededIdFactory('subscribe'))
    try {
      const missing = session.executor.executeCommands([{ type: 'template.subscribe', params: { ...baseContext, trackIds: ['nope', 't-icon', 't-text'] } }])
      expect(missing.ok).toBe(false)
      const audio = createNodeSession({ ...emptyState(), tracks: [{ id: 'a1', name: 'A', kind: 'audio' as const, height: 64, locked: false, visible: true, muted: false, solo: false, order: 0, items: [] }, ...emptyState().tracks.slice(1)] }, createSeededIdFactory('audio'))
      try {
        const outcome = audio.executor.executeCommands([{ type: 'template.subscribe', params: { ...baseContext, trackIds: ['a1', 't-icon', 't-text'] } }])
        expect(outcome.ok).toBe(false)
      } finally { audio.dispose() }
    } finally { session.dispose() }
  })
})
