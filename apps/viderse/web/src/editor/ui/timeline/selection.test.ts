import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import type { TimelineItem } from '@editor/types/timeline';
import { computeItemSelection, hasPointerDrag } from './selection';
const items: TimelineItem[] = [
  { id: 'a1', type: 'audio', label: 'Tiếng 1', src: '/one.mp4', from: 0, durationInFrames: 30, trackId: 'audio', linkedGroupId: 'pair1' },
  { id: 'v1', type: 'video', label: 'Cảnh 1', src: '/one.mp4', from: 0, durationInFrames: 30, trackId: 'video', linkedGroupId: 'pair1' },
  { id: 'v2', type: 'video', label: 'Cảnh 2', src: '/two.mp4', from: 30, durationInFrames: 30, trackId: 'video', linkedGroupId: 'pair2' },
  { id: 'a2', type: 'audio', label: 'Tiếng 2', src: '/two.mp4', from: 30, durationInFrames: 30, trackId: 'audio', linkedGroupId: 'pair2' },
];
const base = { items, currentIds: [], itemId: 'v1', anchorId: null, append: false, range: false, linkedSelection: true };
describe('timeline pointer selection', () => {
  it('selects a video and linked audio with the clicked video primary', () => {
    assert.deepEqual(computeItemSelection(base), ['v1', 'a1']);
    assert.deepEqual(computeItemSelection({ ...base, itemId: 'a1' }), ['a1', 'v1']);
  });
  it('does not include linked companions when linked selection is disabled', () => {
    assert.deepEqual(computeItemSelection({ ...base, linkedSelection: false }), ['v1']);
  });
  it('collapses a plain click on an existing multi-selection to its linked pair', () => {
    assert.deepEqual(computeItemSelection({ ...base, currentIds: items.map((item) => item.id) }), ['v1', 'a1']);
  });
  it('toggles the linked pair off without mutating the press snapshot', () => {
    const currentIds = ['v1', 'a1', 'v2', 'a2'];
    const planned = computeItemSelection({ ...base, currentIds, append: true });
    assert.deepEqual(planned, ['v2', 'a2']);
    assert.deepEqual(currentIds, ['v1', 'a1', 'v2', 'a2']);
  });
  it('adds a linked pair without dropping an existing selection', () => {
    assert.deepEqual(computeItemSelection({ ...base, currentIds: ['v2', 'a2'], append: true }), ['v1', 'v2', 'a2', 'a1']);
  });
  it('selects a same-track range and its companions with the clicked item primary', () => {
    const selection = computeItemSelection({ ...base, itemId: 'v2', anchorId: 'v1', range: true });
    assert.equal(selection[0], 'v2');
    assert.deepEqual(new Set(selection), new Set(['v1', 'a1', 'v2', 'a2']));
  });
  it('does not create a cross-track range or select removed items', () => {
    assert.deepEqual(computeItemSelection({ ...base, itemId: 'v2', anchorId: 'a1', range: true }), ['v2', 'a2']);
    assert.deepEqual(computeItemSelection({ ...base, itemId: 'removed' }), []);
  });
  it('keeps click jitter separate from horizontal and vertical drags', () => {
    assert.equal(hasPointerDrag(100, 50, 103, 53), false);
    assert.equal(hasPointerDrag(100, 50, 104, 50), true);
    assert.equal(hasPointerDrag(100, 50, 100, 46), true);
  });
});
