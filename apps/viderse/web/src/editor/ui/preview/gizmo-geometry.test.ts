import test from 'node:test';
import assert from 'node:assert/strict';
import { gizmoScaleFactor } from './gizmo-geometry';

test('uniform resize grows and shrinks around the center', () => {
  assert.equal(gizmoScaleFactor(200, 100, 0, 100, 50), 2);
  assert.equal(gizmoScaleFactor(200, 100, 0, -50, -25), .5);
  assert.equal(gizmoScaleFactor(200, 100, 0, 0, 0), 1);
});
test('resize respects rotated local axes and cannot invert the item', () => {
  assert.ok(Math.abs(gizmoScaleFactor(200, 100, 90, -50, 100) - 2) < 1e-10);
  assert.equal(gizmoScaleFactor(200, 100, 0, -1000, -500), .02);
  assert.equal(gizmoScaleFactor(0, 0, 0, 10, 10), 1);
});
