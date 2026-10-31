/**
 * Engine audio-measurement helpers shared by the measurement worker and its
 * driver: interleave the windowed mixer output into little-endian float32
 * stereo PCM for the server's ebur128 pass. PCM never gets re-mixed — the
 * bytes are the exact `processAudioWindows` output.
 */

/** Interleave planar f32 channels into interleaved little-endian f32le bytes. */
export function interleavePlanarToF32le(samples: Float32Array[]): Uint8Array {
  if (samples.length === 0) return new Uint8Array(0)
  const channels = samples.length
  const frames = samples[0]!.length
  const out = new Float32Array(frames * channels)
  for (let c = 0; c < channels; c++) {
    const channel = samples[c]!
    for (let i = 0; i < frames; i++) out[i * channels + c] = channel[i]!
  }
  return new Uint8Array(out.buffer, out.byteOffset, out.byteLength)
}

/** Bytes per second of interleaved f32le audio. */
export function f32leBytesPerSecond(sampleRate: number, channels: number): number {
  return sampleRate * channels * 4
}

/**
 * Duration in seconds represented by a PCM byte count; null when the count
 * isn't frame-aligned (a corrupt stream must never be reported as longer).
 */
export function pcmDurationSec(bytes: number, sampleRate: number, channels: number): number | null {
  const frameBytes = channels * 4
  if (frameBytes <= 0 || bytes % frameBytes !== 0) return null
  return bytes / frameBytes / sampleRate
}

export interface PcmChunkEnvelope {
  /** Monotonic chunk index starting at 0 — the server's dedup/order key. */
  seq: number
  sampleRate: number
  channels: number
}

/**
 * Split a large PCM window into upload-sized chunks. The windowed mixer
 * already emits ~30 s windows (≈11.5 MB f32 stereo); uploads go in smaller
 * slices so a retry never re-sends more than `maxChunkBytes`.
 */
export function planPcmChunks(
  windowBytes: number,
  frameBytes: number,
  maxChunkBytes: number,
): { offset: number; length: number }[] {
  const aligned = Math.max(frameBytes, maxChunkBytes - (maxChunkBytes % frameBytes))
  const chunks: { offset: number; length: number }[] = []
  for (let offset = 0; offset < windowBytes; offset += aligned) {
    chunks.push({ offset, length: Math.min(aligned, windowBytes - offset) })
  }
  return chunks
}
