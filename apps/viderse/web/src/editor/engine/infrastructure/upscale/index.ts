/**
 * Stub for the upstream `infrastructure/upscale` barrel.
 * Anime4K ONNX upscaling is ML and was dropped; the variant constants remain
 * because media-library constants import the type.
 */

export const UPSCALE_VARIANTS = ['liveAction', 'animation', 'threeD'] as const

export type UpscaleVariant = (typeof UPSCALE_VARIANTS)[number]

/** The only factor the upstream network implements (2x pixel shuffle). */
export const UPSCALE_FACTOR = 2
