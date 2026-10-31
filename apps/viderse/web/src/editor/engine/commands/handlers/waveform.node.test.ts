import assert from 'node:assert/strict'
import { test } from 'node:test'
import { createNodeSession, createSeededIdFactory, getCapabilityManifest } from '../index'
import { waveformStateFixture } from '@editor/test/waveform-state-fixture'
import { voiceShape, VOICE_HASH } from '@editor/test/waveform-fixture'
import { waveformEnvelopeSchema } from '@editor/features/waveform-visualizer/types'
import { waveformSourceSeconds } from '@editor/features/waveform-visualizer/geometry'

const payload = voiceShape().waveformVisualizer!
const envelope = waveformEnvelopeSchema.parse({ sourceHash: payload.sourceHash, samplesPerSecond: payload.samplesPerSecond, durationSeconds: payload.durationSeconds, normalization: payload.normalization, envelopeBase64: payload.envelopeBase64 })
test('waveform commands add a media-backed shape only, serialize and undo/redo style/envelope', () => {
  const session = createNodeSession(waveformStateFixture(), createSeededIdFactory('voice'))
  try {
    const initial = session.getState()
    const commands = [{ type: 'waveform.add', params: { id: 'voice', trackId: 'visual', from: 0, durationInFrames: 120, mediaId: 'voice-audio', envelope } }, { type: 'waveform.setStyle', params: { itemId: 'voice', style: { color: '#aabbcc', bars: 32 } } }, { type: 'waveform.setEnvelope', params: { itemId: 'voice', envelope } }]
    const result = session.executor.executeCommands(JSON.parse(JSON.stringify(commands)))
    assert.equal(result.ok, true, result.ok ? '' : JSON.stringify(result.issues))
    const changed = session.getState()
    assert.equal(changed.items.length, 1)
    const item = changed.items[0]; assert.equal(item.type, 'shape')
    if (item.type !== 'shape') throw new Error('Expected shape')
    assert.equal(item.mediaId, 'voice-audio'); assert.equal(item.waveformVisualizer?.envelopeBase64, envelope.envelopeBase64)
    assert.equal(item.waveformVisualizer?.color, '#aabbcc'); assert.equal(item.waveformVisualizer?.bars, 32)
    assert.equal(session.executor.undo().ok, true); assert.deepEqual(session.getState(), initial)
    assert.equal(session.executor.redo().ok, true); assert.deepEqual(session.getState(), changed)
    for (const type of ['waveform.add', 'waveform.setStyle', 'waveform.setEnvelope']) assert.ok(getCapabilityManifest().commands.some(command => command.type === type))
  } finally { session.dispose() }
})
test('raw item add/update cannot bypass waveform bounds, shape/source constraints or authoritative hash', () => {
  const session = createNodeSession(waveformStateFixture())
  try {
    for (const item of [voiceShape({ waveformVisualizer: { ...payload, bars: 0 } }), voiceShape({ mediaId: undefined }), voiceShape({ waveformVisualizer: { ...payload, sourceHash: `sha256:${'b'.repeat(64)}` } })]) assert.equal(session.executor.executeCommands([{ type: 'item.add', params: { item } }]).ok, false)
    assert.equal(session.executor.executeCommands([{ type: 'item.add', params: { item: voiceShape() } }]).ok, true)
    for (const updates of [{ waveformVisualizer: { ...payload, envelopeBase64: 'garbage' } }, { type: 'text' }, { isMask: true }, { mediaId: 'missing' }]) assert.equal(session.executor.executeCommands([{ type: 'item.update', params: { id: 'voice', updates } }]).ok, false)
    assert.equal(session.executor.executeCommands([{ type: 'waveform.setEnvelope', params: { itemId: 'voice', envelope: { ...envelope, sourceHash: `sha256:${'c'.repeat(64)}` } } }]).ok, false)
    assert.equal(session.getState().items[0].mediaId, 'voice-audio')
    assert.equal(payload.sourceHash, VOICE_HASH)
  } finally { session.dispose() }
})
test('split and trim preserve voice source samples in both playback directions and frame rates', () => {
  for (const fps of [30, 60]) for (const reversed of [false, true]) {
    const state = waveformStateFixture(); state.fps = fps; state.items = [voiceShape({ durationInFrames: fps * 4, sourceEnd: 120, isReversed: reversed })]
    const session = createNodeSession(state, createSeededIdFactory(`split-${fps}-${reversed}`))
    try {
      const original = voiceShape({ durationInFrames: fps * 4, sourceEnd: 120, isReversed: reversed })
      const result = session.executor.executeCommands([{ type: 'edit.splitItem', params: { id: 'voice', frame: fps, linkedSelection: false } }])
      assert.equal(result.ok, true, result.ok ? '' : JSON.stringify(result.issues))
      const right = session.getState().items.find(item => item.from === fps)
      assert.ok(right && right.type === 'shape')
      assert.equal(waveformSourceSeconds(right, fps * 2, fps), waveformSourceSeconds(original, fps * 2, fps))
      const trim = session.executor.executeCommands([{ type: 'edit.trimItemStart', params: { id: right.id, amount: fps / 2, handle: 'start', linkedSelection: false } }])
      assert.equal(trim.ok, true, trim.ok ? '' : JSON.stringify(trim.issues))
      const trimmed = session.getState().items.find(item => item.id === right.id)
      assert.ok(trimmed && trimmed.type === 'shape')
      assert.equal(waveformSourceSeconds(trimmed, fps * 2, fps), waveformSourceSeconds(original, fps * 2, fps))
    } finally { session.dispose() }
  }
})
