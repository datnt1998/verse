export function outputFrameCount(durationInFrames: number, compositionFps: number, outputFps: number): number {
  return Math.round(durationInFrames / compositionFps * outputFps)
}

export function compositionFrameForOutput(frame: number, compositionFps: number, outputFps: number, durationInFrames: number): number {
  return Math.min(durationInFrames - 1, Math.max(0, Math.floor(frame * compositionFps / outputFps + 1e-9)))
}
