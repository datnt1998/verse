import test from 'node:test';
import assert from 'node:assert/strict';
import type { TimelineItem } from '@editor/types/timeline';
import type { ItemEffect } from '@editor/types/effects';
import type { SubComposition } from '@editor/features/timeline/stores/compositions-store';
import { needsCanvasPreview } from './canvas-preview-routing';

const image: TimelineItem = { id: 'image', type: 'image', label: 'Ảnh', trackId: 'v1', from: 0, durationInFrames: 90, src: '/image.png' };
const brightness: ItemEffect = { id: 'brightness', enabled: true, effect: { type: 'gpu-effect', gpuEffectType: 'gpu-brightness', params: { brightness: .5 } } };
const lut: ItemEffect = { id: 'lut', enabled: true, effect: { type: 'gpu-effect', gpuEffectType: 'gpu-lut', params: { preset: 'warm' } } };
const compound: TimelineItem = { id: 'compound', type: 'composition', label: 'Nhóm', trackId: 'v1', from: 0, durationInFrames: 90, compositionId: 'nested', compositionWidth: 1920, compositionHeight: 1080 };
const nested: SubComposition = { id: 'nested', name: 'Nhóm', items: [], tracks: [], keyframes: [], transitions: [], fps: 30, width: 1920, height: 1080, durationInFrames: 90 };

test('plain clips and disabled effects keep the DOM fast path', () => {
  assert.equal(needsCanvasPreview([image], {}), false);
  assert.equal(needsCanvasPreview([{ ...image, effects: [{ ...brightness, enabled: false }] }], {}), false);
});
test('brightness and LUT effects use the existing canvas renderer', () => {
  assert.equal(needsCanvasPreview([{ ...image, effects: [brightness] }], {}), true);
  assert.equal(needsCanvasPreview([{ ...image, effects: [lut] }], {}), true);
});
test('live effect previews route before canonical effects commit', () => {
  assert.equal(needsCanvasPreview([image], {}, { image: { effects: [brightness] } }), true);
  assert.equal(needsCanvasPreview([{ ...image, effects: [brightness] }], {}, { image: { effects: [] } }), false);
});
test('non-normal blending and nested effects route without looping on cycles', () => {
  assert.equal(needsCanvasPreview([{ ...image, blendMode: 'multiply' }], {}), true);
  assert.equal(needsCanvasPreview([compound], { nested: { ...nested, items: [{ ...image, effects: [lut] }] } }), true);
  assert.equal(needsCanvasPreview([compound], { nested: { ...nested, items: [compound] } }), false);
});
