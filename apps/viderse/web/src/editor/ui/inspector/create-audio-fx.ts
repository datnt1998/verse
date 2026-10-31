import type { AudioFxEffect } from "@editor/types/audio";
import { DEFAULT_COMPRESSOR, DEFAULT_DELAY, DEFAULT_NOISE_REDUCTION, DEFAULT_REVERB } from "@editor/donor/openreel/audio/contracts";

export function createAudioFx(type: AudioFxEffect["type"], id: string): AudioFxEffect {
  switch (type) {
    case "compressor": return { id, enabled: true, type, params: { ...DEFAULT_COMPRESSOR } };
    // Learning must be available before enabling a noise gate with a real profile.
    case "noiseReduction": return { id, enabled: false, type, params: { ...DEFAULT_NOISE_REDUCTION } };
    case "reverb": return { id, enabled: true, type, params: { ...DEFAULT_REVERB } };
    case "delay": return { id, enabled: true, type, params: { ...DEFAULT_DELAY } };
  }
}
