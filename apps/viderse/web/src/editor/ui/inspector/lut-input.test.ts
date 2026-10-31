import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { assertSupportedLutSize } from "./lut-input";

describe("LUT input size", () => {
  it("accepts standard 33 and maximum 65 cubes", () => {
    assert.doesNotThrow(() => assertSupportedLutSize("TITLE \"Test\"\nLUT_3D_SIZE 33\n"));
    assert.doesNotThrow(() => assertSupportedLutSize("  LUT_3D_SIZE 65 # maximum\r\n"));
  });
  it("rejects larger cubes before parsing their samples", () => {
    assert.throws(() => assertSupportedLutSize("LUT_3D_SIZE 66\n"), /không vượt quá 65³/);
    assert.throws(() => assertSupportedLutSize("LUT_3D_SIZE 129\n"), /không vượt quá 65³/);
  });
  it("leaves missing or malformed sizes to the engine parser", () => {
    assert.doesNotThrow(() => assertSupportedLutSize("# LUT_3D_SIZE 129\nLUT_3D_SIZE 33\n"));
    assert.doesNotThrow(() => assertSupportedLutSize("LUT_3D_SIZE invalid\n"));
    assert.doesNotThrow(() => assertSupportedLutSize("LUT_1D_SIZE 128\n"));
  });
});
