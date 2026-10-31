import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { formatTimecode, parseTimecode } from "./timecode";

describe("caption timecode", () => {
  it("formats frames as mm:ss:ff and adds hours from one hour", () => {
    assert.equal(formatTimecode(0, 30), "00:00:00");
    assert.equal(formatTimecode(95, 30), "00:03:05");
    assert.equal(formatTimecode(108000 + 1830 + 15, 30), "01:01:01:15");
  });

  it("round-trips ss:ff, mm:ss:ff and hh:mm:ss:ff", () => {
    assert.equal(parseTimecode("03:05", 30), 95);
    assert.equal(parseTimecode("00:03:05", 30), 95);
    assert.equal(parseTimecode("01:01:01:15", 30), 108000 + 1830 + 15);
    for (const frame of [0, 29, 30, 1799, 1800, 109815]) {
      assert.equal(parseTimecode(formatTimecode(frame, 30), 30), frame);
    }
  });

  it("accepts semicolons and surrounding whitespace", () => {
    assert.equal(parseTimecode(" 00;03;05 ", 30), 95);
  });

  it("rejects a frame part at or above the frame rate", () => {
    assert.equal(parseTimecode("00:00:30", 30), null);
    assert.equal(parseTimecode("00:00:29", 30), 29);
  });

  it("rejects non-numeric, empty and wrong-length input", () => {
    for (const text of ["", "12", "aa:bb", "00:-1:05", "1.5:03", "00:00:00:00:00", "00::05"]) {
      assert.equal(parseTimecode(text, 30), null, text);
    }
  });

  it("rounds fractional frame rates and floors them at 1", () => {
    assert.equal(parseTimecode("00:01:00", 29.97), 30);
    assert.equal(formatTimecode(30, 29.97), "00:01:00");
    assert.equal(formatTimecode(5, 0), "00:05:00");
  });
});
