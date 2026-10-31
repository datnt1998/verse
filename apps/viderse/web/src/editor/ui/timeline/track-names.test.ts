import assert from 'node:assert/strict';
import test from 'node:test';
import { nextTrackName } from './track-names';

const named = (...names: string[]) => names.map((name) => ({ name }));

test('numbers video and audio tracks independently', () => {
  assert.equal(nextTrackName(named('V1', 'A1'), 'video'), 'V2');
  assert.equal(nextTrackName(named('V1', 'V2', 'A1'), 'audio'), 'A2');
  assert.equal(nextTrackName([], 'audio'), 'A1');
});

test('reuses the lowest free number after a track was removed', () => {
  assert.equal(nextTrackName(named('V1', 'V3'), 'video'), 'V2');
});

test('ignores custom names when picking the number', () => {
  assert.equal(nextTrackName(named('Video', 'Giọng đọc', 'V1'), 'video'), 'V2');
});
