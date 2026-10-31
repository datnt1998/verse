/**
 * Stub for the upstream `infrastructure/interpolation` barrel.
 * RIFE frame interpolation is ML and was dropped; the factor constants remain
 * because media-library constants import the type.
 */

export const SUPPORTED_INTERPOLATION_FACTORS = [2, 3, 4, 5, 6, 7, 8] as const

export type InterpolationFactor = (typeof SUPPORTED_INTERPOLATION_FACTORS)[number]

export function isSupportedInterpolationFactor(value: number): value is InterpolationFactor {
  return (SUPPORTED_INTERPOLATION_FACTORS as readonly number[]).includes(value)
}
