import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import type { TimelineItem } from '@editor/types/timeline';
import { previewTimelineItems } from './preview';
const clips: TimelineItem[] = [
  { id: 'v', type: 'video', trackId: 'video', label: 'Cảnh', src: '/real.mp4', from: 0, durationInFrames: 60, sourceStart: 100, sourceEnd: 220, sourceFps: 60, linkedGroupId: 'av' },
  { id: 'a', type: 'audio', trackId: 'audio', label: 'Tiếng', src: '/real.mp4', from: 0, durationInFrames: 60, sourceStart: 0, sourceEnd: 96000, sourceFps: 48000, linkedGroupId: 'av' },
  { id: 'next', type: 'text', text: 'Tiếp', color: '#ffffff', trackId: 'video', label: 'Tiếp', from: 60, durationInFrames: 40 },
];
describe('timeline render preview', () => {
  it('leaves canonical objects unchanged during a track move', () => {
    const preview = previewTimelineItems(clips, { kind: 'move', updates: [{ id: 'v', from: 30, trackId: 'other' }] }, 30);
    assert.equal(preview[0].from, 30); assert.equal(preview[0].trackId, 'other');
    assert.equal(clips[0].from, 0); assert.equal(preview[1], clips[1]);
  });
  it('previews linked trims at each source frame rate', () => {
    const preview = previewTimelineItems(clips, { kind: 'trim', itemId: 'v', side: 'start', delta: 10, mode: 'normal', linkedSelection: true }, 30);
    assert.equal(preview[0].sourceStart, 120); assert.equal(preview[1].sourceStart, 16000);
    assert.equal(preview[0].from, 10); assert.equal(preview[1].durationInFrames, 50);
    assert.equal(preview[2], clips[2]);
  });
  it('holds ripple heads in place and closes downstream space', () => {
    const preview = previewTimelineItems(clips, { kind: 'trim', itemId: 'v', side: 'start', delta: 10, mode: 'ripple', linkedSelection: true }, 30);
    assert.equal(preview[0].from, 0); assert.equal(preview[0].durationInFrames, 50);
    assert.equal(preview[1].from, 0); assert.equal(preview[2].from, 50);
  });
  it('rolls adjoining clips without changing the total duration', () => {
    const preview = previewTimelineItems(clips, { kind: 'trim', itemId: 'v', side: 'end', delta: 10, mode: 'rolling', linkedSelection: false }, 30);
    assert.equal(preview[0].durationInFrames, 70); assert.equal(preview[2].from, 70); assert.equal(preview[2].durationInFrames, 30);
    assert.equal(preview[2].from + preview[2].durationInFrames, 100);
    assert.equal(preview[1], clips[1]);
  });
  it('returns the original array when there is no active edit', () => { assert.equal(previewTimelineItems(clips, null, 30), clips); });
});
