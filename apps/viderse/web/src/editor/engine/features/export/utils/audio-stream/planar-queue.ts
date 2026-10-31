/**
 * Bounded per-channel frame queue carrying produced audio (post-DSP,
 * post-resample) across 30 s output windows. Produced chunk sizes do not
 * line up with output windows for speed/pitch/resampled segments, so the
 * driver appends produced chunks here and drains exactly the span each
 * window needs. Memory stays bounded because the driver only produces
 * enough to cover the current window.
 */
export class PlanarFrameQueue {
  private chunks: Float32Array[] = []
  private head = 0
  private frames = 0

  get length(): number {
    return this.frames
  }

  push(samples: Float32Array): void {
    if (samples.length === 0) return
    this.chunks.push(samples)
    this.frames += samples.length
  }

  /** Frames at queue position index (0 = oldest). */
  at(index: number): number {
    index += this.head
    let base = 0
    for (const chunk of this.chunks) {
      const chunkFrames = chunk.length
      if (index < base + chunkFrames) return chunk[index - base] ?? 0
      base += chunkFrames
    }
    return 0
  }

  /**
   * Copy `count` frames starting at queue position `offset` into `target`
   * starting at `targetOffset`. Copies fewer frames at the queue end.
   */
  readInto(target: Float32Array, targetOffset: number, offset: number, count: number): number {
    let copied = 0
    let base = 0
    let remaining = Math.min(count, Math.max(0, this.frames - offset))
    let pos = offset + this.head
    for (const chunk of this.chunks) {
      if (remaining <= 0) break
      const chunkStart = base
      const chunkEnd = base + chunk.length
      if (pos >= chunkEnd) {
        base = chunkEnd
        continue
      }
      const inChunk = Math.max(0, pos - chunkStart)
      const take = Math.min(remaining, chunk.length - inChunk)
      target.set(chunk.subarray(inChunk, inChunk + take), targetOffset + copied)
      copied += take
      remaining -= take
      pos += take
      base = chunkEnd
    }
    return copied
  }

  /** Drop `count` frames from the front. */
  drop(count: number): void {
    let remaining = Math.min(count, this.frames)
    this.frames -= remaining
    while (remaining > 0 && this.chunks.length > 0) {
      const chunk = this.chunks[0]!
      const available = chunk.length - this.head
      if (available > remaining) {
        this.head += remaining
        remaining = 0
      } else {
        remaining -= available
        this.chunks.shift()
        this.head = 0
      }
    }
  }

  clear(): void {
    this.chunks = []
    this.head = 0
    this.frames = 0
  }
}
