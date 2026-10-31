import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { audioFxEffectSchema } from "@editor/features/audio-fx/schema";
import { createAudioFx } from "./create-audio-fx";

describe("audio effect initial state", () => {
  it("allows adding noise reduction before a profile has been learned", () => {
    const effect = createAudioFx("noiseReduction", "noise");
    assert.equal(effect.enabled, false);
    assert.equal(audioFxEffectSchema.safeParse(effect).success, true);
    assert.equal(audioFxEffectSchema.safeParse({ ...effect, enabled: true }).success, false);
  });
  it("starts the other effects enabled with independent parameter objects", () => {
    for (const type of ["compressor", "reverb", "delay"] as const) {
      const first = createAudioFx(type, "first");
      const second = createAudioFx(type, "second");
      assert.equal(first.enabled, true);
      assert.equal(audioFxEffectSchema.safeParse(first).success, true);
      assert.notEqual(first.params, second.params);
    }
  });
});
