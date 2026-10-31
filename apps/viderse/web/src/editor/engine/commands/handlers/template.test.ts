import { describe, expect, it } from 'vitest'
import { createNodeSession } from '../node-session'
import { captureProjectState } from '../project-state'
import { createSeededIdFactory } from '../id-factory'
import { registerAllCommands } from './index'
import { getBuiltInEditingTemplates } from '@editor/features/editing-templates/registry'

registerAllCommands()

describe('faceless editing templates', () => {
  for (const template of getBuiltInEditingTemplates()) {
    it(`applies ${template.id} as valid canonical commands and one undo entry`, () => {
      const initial = { ...captureProjectState(), items: [], tracks: [], transitions: [], keyframes: [], markers: [], compositions: [], topLevelSequenceIds: [] }
      const session = createNodeSession(initial, createSeededIdFactory('template'))
      try {
        const result = session.executor.executeCommands([{ type: 'template.apply', params: { templateId: template.id, context: { idPrefix: 'story', fps: 30, width: 1920, height: 1080, durationInFrames: 900, narration: { mediaId: 'voice', src: '/voice.m4a' }, sections: [{ title: 'Mở đầu', from: 0, durationInFrames: 450, image: { mediaId: 'still', src: '/still.png' } }, { title: 'Kết thúc', from: 450, durationInFrames: 450 }], words: [{ text: 'Xin', startFrame: 0, endFrame: 30 }, { text: 'chào', startFrame: 30, endFrame: 60 }] } } }])
        expect(result).toMatchObject({ ok: true })
        const state = session.getState()
        expect(state.items.length).toBeGreaterThanOrEqual(3)
        expect(state.keyframes).toHaveLength(1)
        expect(state.items.every((item) => state.tracks.some((track) => track.id === item.trackId))).toBe(true)
        expect(state.items.every((item) => item.from >= 0 && item.durationInFrames > 0 && item.from + item.durationInFrames <= 900)).toBe(true)
        expect(result.ok === true && result.commands.every((command) => command.type === 'template.apply')).toBe(true)
        const replayed = createNodeSession(initial, createSeededIdFactory('replayed'))
        try {
          if (result.ok === true) {
            expect(replayed.executor.executeCommands(JSON.parse(JSON.stringify(result.commands))).ok).toBe(true)
            expect(replayed.getState().items).toEqual(state.items)
          }
        } finally { replayed.dispose() }
        session.executor.undo()
        expect(session.getState().items).toEqual([])
        session.executor.redo()
        expect(session.getState().items).toEqual(state.items)
        session.executor.undo()
        expect(session.getState().items).toEqual([])
        expect(session.getState().tracks).toEqual([])
      } finally { session.dispose() }
    })
  }
})

describe('template.subscribe preset', () => {
  it('expands to shape/text items plus keyframes under the requested tracks', () => {
    const tracks = ['sub-back', 'sub-icon', 'sub-text'].map((id, order) => ({ id, name: id, kind: 'video' as const, height: 64, locked: false, visible: true, muted: false, solo: false, order, items: [] }))
    const initial = { ...captureProjectState(), items: [], tracks, transitions: [], keyframes: [], markers: [], compositions: [], topLevelSequenceIds: [], mediaItems: [] }
    const session = createNodeSession(initial, createSeededIdFactory('sub'))
    try {
      const result = session.executor.executeCommands([{ type: 'template.subscribe', params: { idPrefix: 'clone', trackIds: tracks.map((track) => track.id), from: 0, durationInFrames: 111, fps: 30, width: 1920, height: 1080 } }])
      expect(result.ok).toBe(true)
      const state = session.getState()
      expect(state.items.map((item) => item.type)).toEqual(['shape', 'shape', 'text'])
      expect(new Set(state.items.map((item) => item.linkedGroupId)).size).toBe(1)
      expect(state.keyframes.length).toBeGreaterThan(0)
      session.executor.undo()
      expect(session.getState().items).toEqual([])
    } finally { session.dispose() }
  })
})
