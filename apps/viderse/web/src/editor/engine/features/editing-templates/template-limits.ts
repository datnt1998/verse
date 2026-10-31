/*
 * Every numeric limit of a template's recipe and backdrop settings, in one dependency-free leaf:
 * the zod schemas, the recipe clamp, the Template pages and the backdrop pipeline all read these
 * numbers, and so do the sentences that quote them. Keep this file free of imports so any of
 * those modules can use it without forming a cycle.
 */
export const TEMPLATE_LIMITS = {
  /** Subject images a recipe may ask for: the fixed count, and the clamp on the count derived from minutes. */
  fixedImages: { min: 1, max: 60 },
  /** One image per this many minutes of source video. */
  perMinutes: { min: 1, max: 60 },
  motionIntensity: { min: 0.25, max: 2 },
  motionSpeed: { min: 0.25, max: 3 },
  /** Characters of the template's image prompt applied to every generated image. */
  imagePrompt: { max: 2000 },
  /** Shared opacity of the scene-image layer on built clones: 0 hidden, 1 opaque. */
  sceneOpacity: { min: 0, max: 1 },
  backgroundAppearance: {
    opacity: { min: 0, max: 1 },
    blurRadius: { min: 0, max: 50 },
    brightness: { min: -1, max: 1 },
    contrast: { min: 0, max: 3 },
    saturation: { min: 0, max: 3 },
    hueShift: { min: 0, max: 1 },
    speed: { min: 0.25, max: 3 },
    zoom: { min: 1, max: 3 },
  },
  /** Backdrop channels linked to one template. */
  backdropChannels: { min: 1, max: 20 },
  kindsPerChannel: { min: 1, max: 3 },
  /** Pinned videos per backdrop channel. */
  pinnedPerChannel: { max: 200 },
  /** Backdrop clips one job may hold. */
  backdropClips: { min: 1, max: 40 },
  /** Seconds per backdrop clip. */
  backdropClipSec: { min: 10, max: 600 },
} as const
