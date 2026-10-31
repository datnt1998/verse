import assert from "node:assert/strict";
import test from "node:test";
import { effectPreview } from "./effect-preview";

test("every effect category gets a recognisable preview, and known effects get their own", () => {
  for (const category of ["color", "blur", "distort", "stylize", "keying"] as const) {
    const preview = effectPreview("gpu-not-listed", category);
    assert.ok(preview.filter || preview.overlay, category);
  }
  assert.equal(effectPreview("gpu-grayscale", "color").filter, "grayscale(1)");
  assert.ok(effectPreview("gpu-vignette", "stylize").overlay);
});
