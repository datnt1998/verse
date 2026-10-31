import assert from 'node:assert/strict'
import { test } from 'node:test'
import { DEFAULT_WAVEFORM_STYLE, waveformStyleSchema } from '@/editor/engine/features/waveform-visualizer/types'
import { WAVEFORM_SLIDERS, waveformStyleCommand } from './waveform-settings'

test('waveform controls use bounded canonical style commands without regenerating envelopes', () => {
  for (const setting of WAVEFORM_SLIDERS) {
    assert.equal(waveformStyleSchema.safeParse({ ...DEFAULT_WAVEFORM_STYLE, [setting.key]: setting.min }).success, true)
    assert.equal(waveformStyleSchema.safeParse({ ...DEFAULT_WAVEFORM_STYLE, [setting.key]: setting.max }).success, true)
    assert.equal(waveformStyleSchema.safeParse({ ...DEFAULT_WAVEFORM_STYLE, [setting.key]: setting.max + setting.step }).success, false)
    assert.ok(setting.label)
  }
  assert.deepEqual(waveformStyleCommand('voice', { color: '#aabbcc', bars: 32 }), { type: 'waveform.setStyle', params: { itemId: 'voice', style: { color: '#aabbcc', bars: 32 } } })
})
