// @vitest-environment node
import { describe, expect, it } from 'vitest'
import type { LottieItem } from '@editor/types/timeline'
import { makeTimelineAudioItem, makeTimelineTrack, makeTimelineVideoItem } from '../../features/timeline/test-helpers'
import { captureProjectState } from '../project-state'
import { createNodeSession } from '../node-session'
import { registerItemCommands } from './item'

registerItemCommands()

describe('object property loop commands', () => {
  it('enables Lottie looping after disabling it, with reversible playback and editable content', () => {
    const item: LottieItem = { id: 'animation', type: 'lottie', trackId: 'visual', label: 'Hoạt ảnh', src: '/animation.json', from: 0, durationInFrames: 120, frameRate: 30, totalFrames: 60 }
    const session = createNodeSession({ ...captureProjectState(), tracks: [makeTimelineTrack({ id: 'visual', kind: 'video' })], items: [item], compositions: [] })
    try {
      expect(session.executor.executeCommands([{ type: 'item.update', params: { id: item.id, updates: { loop: false } } }]).ok).toBe(true)
      expect(session.getState().items[0]).toMatchObject({ loop: false })
      const updates = { loop: true, speed: 2, reversed: true, loopMode: 'pingpong', segmentStart: 5, segmentEnd: 40, textOverrides: { '1': 'Xin chào' }, colorOverrides: { c0: '#ff0000' }, slotOverrides: { amount: 25, position: [10, 20] } }
      expect(session.executor.executeCommands([{ type: 'item.update', params: { id: item.id, updates } }]).ok).toBe(true)
      expect(session.getState().items[0]).toMatchObject({ ...updates, durationInFrames: 120 })
      expect(session.executor.undo().ok).toBe(true)
      expect(session.getState().items[0]).toMatchObject({ loop: false, durationInFrames: 120 })
      expect(session.executor.redo().ok).toBe(true)
      expect(session.getState().items[0]).toMatchObject(updates)
    } finally { session.dispose() }
  })

  it('keeps rejecting audible, linked and unknown-window videos and audio-only clips', () => {
    const session = createNodeSession({
      ...captureProjectState(),
      tracks: [makeTimelineTrack({ id: 'visual', kind: 'video' }), makeTimelineTrack({ id: 'sound', kind: 'audio' })],
      items: [
        makeTimelineVideoItem({ id: 'audible', trackId: 'visual', from: 0, embeddedAudioMuted: false, sourceStart: 0, sourceEnd: 60 }),
        makeTimelineVideoItem({ id: 'linked', trackId: 'visual', from: 100, embeddedAudioMuted: true, linkedGroupId: 'pair', sourceStart: 0, sourceEnd: 60 }),
        makeTimelineAudioItem({ id: 'companion', trackId: 'sound', linkedGroupId: 'pair' }),
        makeTimelineVideoItem({ id: 'unknown', trackId: 'visual', from: 200, embeddedAudioMuted: true, sourceEnd: undefined, sourceDuration: undefined }),
      ],
      compositions: [],
    })
    try {
      for (const id of ['audible', 'linked', 'unknown', 'companion']) {
        expect(session.executor.executeCommands([{ type: 'item.update', params: { id, updates: { loop: true } } }]).ok).toBe(false)
      }
    } finally { session.dispose() }
  })
})
