/** Default decimals shown by number fields: 1 for degrees, 2 otherwise. */
export function defaultPrecision(unit?: string): number {
  return unit === '°' ? 1 : 2;
}

/** Rounds for display only and trims trailing zeros (38.37109 → 38.37, 2.50 → 2.5). */
export function roundForDisplay(value: number, precision: number): number {
  return Number(value.toFixed(precision));
}

/**
 * Whether committing `next` from a typed draft is a real edit. Re-entering the
 * rounded value that is on screen must not overwrite the full-precision value.
 */
export function isNumberEdit(next: number, value: number, precision: number): boolean {
  return next !== value && next !== roundForDisplay(value, precision);
}
