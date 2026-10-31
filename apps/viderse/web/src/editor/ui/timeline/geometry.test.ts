import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import { clampFrame, fitZoomLevel, indexItems, isDenseWindow, marqueeItems, moveGroup, reorderTrackOrder, resizedTrackHeight, rulerKeyFrame, rulerStep, snapFrame, snapGroupDelta, timecode, visibleItems } from './geometry';
const clips = [
  { id: 'long', trackId: 'v', from: 0, durationInFrames: 900 },
  { id: 'short', trackId: 'v', from: 300, durationInFrames: 20 },
  { id: 'later', trackId: 'v', from: 1200, durationInFrames: 30 },
  { id: 'audio', trackId: 'a', from: 300, durationInFrames: 600 },
];
describe('timeline geometry', () => {
  it('clamps and formats exact frame timecodes', () => {
    assert.equal(clampFrame(-2), 0); assert.equal(clampFrame(33.8, 30), 30);
    assert.equal(timecode(109815, 30), '01:01:00:15');
  });
  it('snaps in screen pixels and preserves unsnapped frame rounding', () => {
    assert.deepEqual(snapFrame(298, [0, 300, 900], 2), { frame: 300, guide: 300 });
    assert.deepEqual(snapFrame(288.3, [300], 2), { frame: 288, guide: null });
    assert.equal(snapFrame(200, [300], 0.05).frame, 300);
  });
  it('snaps every group edge without collapsing offsets at zero', () => {
    const group = [{ id: 'a', trackId: 'v', from: 10, durationInFrames: 20 }, { id: 'b', trackId: 'a', from: 100, durationInFrames: 40 }];
    assert.deepEqual(snapGroupDelta(group, 6, [145], 1), { delta: 5, guide: 145 });
    assert.deepEqual(snapGroupDelta(group, -100, [], 1), { delta: -10, guide: null });
    assert.deepEqual(snapGroupDelta([], 20, [30], 1), { delta: 0, guide: null });
  });
  it('keeps overlapping long clips and pinned focus outside viewport', () => {
    const index = indexItems(clips).get('v');
    assert.deepEqual(visibleItems(index, 500, 600).map((item) => item.id), ['long']);
    assert.deepEqual(visibleItems(index, 500, 600, 'later').map((item) => item.id), ['long', 'later']);
    assert.deepEqual(visibleItems(index, 901, 1100), []);
  });
  it('culls a thousand clips in a fifteen-minute timeline', () => {
    const items = Array.from({ length: 1000 }, (_, n) => ({ id: String(n), trackId: 'v', from: n * 27, durationInFrames: 27 }));
    assert.equal(visibleItems(indexItems(items).get('v'), 13500, 13800).length, 13);
  });
  it('moves groups without losing offsets at frame zero', () => {
    assert.deepEqual(moveGroup(clips.slice(1, 3), -600).map((item) => item.from), [0, 900]);
  });
  it('selects marquee intersections only on chosen tracks', () => {
    assert.deepEqual(marqueeItems(clips, 600, 400, new Set(['a'])), ['audio']);
  });
  it('spaces ruler ticks for readable labels', () => { assert.equal(rulerStep(30, 1), 150); });
  it('fits fifteen minutes even below one percent zoom', () => {
    const zoom = fitZoomLevel(756, 27000, 30);
    assert.equal(zoom, 0.0084); assert.equal(zoom * 100 * 900, 756);
    assert.equal(fitZoomLevel(1200, 1, 30), 1.2);
  });
  it('rasterizes sub-target clips while leaving wide windows interactive', () => {
    const items = Array.from({ length: 1000 }, (_, n) => ({ id: String(n), trackId: 'v', from: n * 27, durationInFrames: 27 }));
    assert.equal(isDenseWindow(items, 0.03), true);
    assert.equal(isDenseWindow(items.slice(0, 20), 0.03), true);
    assert.equal(isDenseWindow(items, 2), false);
  });
  it('gives the native ruler slider complete keyboard seeking', () => {
    assert.equal(rulerKeyFrame('Home', 90, 30, 360), 0);
    assert.equal(rulerKeyFrame('End', 0, 30, 360), 359);
    assert.equal(rulerKeyFrame('ArrowLeft', 0, 30, 360), 0);
    assert.equal(rulerKeyFrame('ArrowRight', 359, 30, 360), 359);
    assert.equal(rulerKeyFrame('ArrowRight', 90, 30, 360, true), 120);
    assert.equal(rulerKeyFrame('PageUp', 90, 30, 360), 60);
    assert.equal(rulerKeyFrame('PageDown', 90, 30, 360), 120);
    assert.equal(rulerKeyFrame('KeyM', 90, 30, 360), null);
  });
  it('resizes local track height within pointer-specific limits', () => {
    assert.equal(resizedTrackHeight(100, 8), 108);
    assert.equal(resizedTrackHeight(100, -32), 72);
    assert.equal(resizedTrackHeight(64, 8, 104), 112);
    assert.equal(resizedTrackHeight(150, 32), 160);
  });
  it('rewrites persisted order fields without mutating the source tracks', () => {
    const tracks = [{ id: 'v', order: 0, kind: 'video' }, { id: 'a', order: 1, kind: 'audio' }, { id: 'b', order: 2, kind: 'audio' }];
    const reordered = reorderTrackOrder(tracks, 'a', 1);
    assert.deepEqual(reordered?.map(({ id, order }) => [id, order]), [['v', 0], ['b', 1], ['a', 2]]);
    assert.deepEqual(tracks.map(({ id, order }) => [id, order]), [['v', 0], ['a', 1], ['b', 2]]);
    assert.deepEqual(reorderTrackOrder(reordered ?? [], 'a', -1), tracks);
  });
  it('uses canonical order rather than array position for reordering', () => {
    assert.deepEqual(reorderTrackOrder([{ id: 'b', order: 1 }, { id: 'a', order: 0 }], 'a', 1), [{ id: 'b', order: 0 }, { id: 'a', order: 1 }]);
    assert.equal(reorderTrackOrder([{ id: 'a', order: 0 }], 'a', 1), null);
    assert.equal(reorderTrackOrder([{ id: 'a', order: 0 }], 'missing', 1), null);
  });
});
