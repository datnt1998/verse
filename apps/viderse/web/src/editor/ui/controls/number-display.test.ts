import assert from 'node:assert/strict';
import test from 'node:test';
import { defaultPrecision, isNumberEdit, roundForDisplay } from './number-display';

test('shows at most two decimals, one for degrees, without trailing zeros', () => {
  assert.equal(roundForDisplay(-27.25573, defaultPrecision('px')), -27.26);
  assert.equal(roundForDisplay(38.37109, defaultPrecision('%')), 38.37);
  assert.equal(roundForDisplay(12.345, defaultPrecision('°')), 12.3);
  assert.equal(roundForDisplay(2.5, 2), 2.5);
  assert.equal(roundForDisplay(100, 2), 100);
});

test('re-entering the displayed rounded value keeps the stored value', () => {
  assert.equal(isNumberEdit(38.37, 38.37109, 2), false);
  assert.equal(isNumberEdit(38.37109, 38.37109, 2), false);
});

test('a different typed value is an edit', () => {
  assert.equal(isNumberEdit(38.4, 38.37109, 2), true);
  assert.equal(isNumberEdit(-0.01, -0.004, 2), true);
  assert.equal(isNumberEdit(0, -0.004, 2), false);
});
