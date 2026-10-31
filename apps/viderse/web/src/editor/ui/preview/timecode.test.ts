import test from 'node:test';
import assert from 'node:assert/strict';
import { frameTimecode } from './timecode';

test('timecode preserves the frame field across seconds and hours', () => {
  assert.equal(frameTimecode(0, 30), '00:00:00:00');
  assert.equal(frameTimecode(29, 30), '00:00:00:29');
  assert.equal(frameTimecode(30, 30), '00:00:01:00');
  assert.equal(frameTimecode(108001, 30), '01:00:00:01');
});
test('timecode clamps negative frames and rounds the display rate', () => {
  assert.equal(frameTimecode(-5, 30), '00:00:00:00');
  assert.equal(frameTimecode(31.9, 29.97), '00:00:01:01');
});
