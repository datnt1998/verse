/** Debounced real metadata preview; URL order is retained for the manual front block. */
export const PLAN_DEBOUNCE_MS = 300

export function parseUrls(text: string): string[] {
  return text.split(/\s+/).map((url) => url.trim()).filter(Boolean)
}


