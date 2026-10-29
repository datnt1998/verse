import type { ConfigModelInfo } from '../api/types'

const contextNumber = new Intl.NumberFormat('vi', { maximumFractionDigits: 2 })
/** Compact context capacity; K and M always use decimal catalog units. */
export function formatContextK(tokens: number): string {
  if (!Number.isFinite(tokens) || tokens <= 0) return ''
  return tokens >= 1_000_000 ? `${contextNumber.format(tokens / 1_000_000)}M` : `${contextNumber.format(tokens / 1000)}K`
}
export function contextThousands(value: string): number | null {
  if (!value.trim()) return null
  const n = Number(value)
  return Number.isFinite(n) && Number.isInteger(n * 1000) && n >= 8 && n <= 2000 ? n * 1000 : null
}
export const CONTEXT_PRESETS = [32000, 64000, 128000, 200000, 1000000] as const
export type ContextDraft = { mode: 'auto' | 'custom'; value: string }
export function contextDraft(model: ConfigModelInfo): ContextDraft {
  return { mode: model.contextSource === 'model_config' ? 'custom' : 'auto', value: String(model.contextWindow / 1000) }
}
export function contextThreshold(model: ConfigModelInfo, window: number): number {
  return window === model.contextWindow ? model.summaryThreshold : Math.max(0, window - Math.max(8000, window - Math.floor(window * .85)))
}
