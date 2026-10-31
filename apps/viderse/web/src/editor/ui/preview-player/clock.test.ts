import assert from "node:assert/strict";
import test from "node:test";

import { clockLabel, spokenClock } from "./clock";

test("the clock shows whole seconds, with hours only when needed", () => {
  assert.equal(clockLabel(0, 30), "0:00");
  assert.equal(clockLabel(29, 30), "0:00");
  assert.equal(clockLabel(210, 30), "0:07");
  assert.equal(clockLabel(30 * 125, 30), "2:05");
  assert.equal(clockLabel(30 * 3723, 30), "1:02:03");
  assert.equal(clockLabel(-5, 30), "0:00");
});

test("the spoken clock names minutes and seconds", () => {
  assert.equal(spokenClock(210, 30), "7 giây");
  assert.equal(spokenClock(30 * 125, 30), "2 phút 5 giây");
});
