import type { AudioFxEffect } from '@editor/types/audio'

/** Timing metadata without allocating the processor's delay/FFT state. */
export function getAudioFxTailFrames(effects: readonly AudioFxEffect[] | undefined, sampleRate: number): number {
  let frames = 0
  for (const effect of effects ?? []) {
    if (!effect.enabled) continue
    switch (effect.type) {
      case 'compressor': break
      case 'noiseReduction': frames += effect.params.profile?.fftSize ?? 2048; break
      case 'reverb': frames += 256 + (effect.params.wetLevel > 0
        ? Math.floor(sampleRate * (0.5 + effect.params.roomSize * 3.5)) + Math.round(sampleRate * effect.params.preDelay / 1000) - 1 : 0); break
      case 'delay': {
        const { time, feedback, wetLevel } = effect.params
        if (wetLevel === 0) break
        const repeats = feedback > 0 ? Math.max(1, Math.ceil(Math.log(1e-4 / Math.max(wetLevel, 1e-4)) / Math.log(feedback))) : 1
        frames += Math.max(1, Math.round(sampleRate * time)) * repeats
        break
      }
    }
  }
  return frames
}
