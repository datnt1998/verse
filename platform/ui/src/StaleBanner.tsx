import * as stylex from '@stylexjs/stylex'
import { color } from './tokens.stylex'
import { Button } from './Button'
import { getKitLabels } from './labels'

const sx = stylex.create({
  root: {
    paddingTop: 6, paddingBottom: 6, paddingInline: 12,
    display: 'flex', alignItems: 'center', gap: 10,
    color: color['--color-warn'], backgroundColor: color['--color-surface-2'], fontSize: 12,
  },
})

export function StaleBanner({ error, onRetry }: { error?: unknown; onRetry: () => unknown }) {
  if (!error) return null
  return <div role="status" aria-live="polite" {...stylex.props(sx.root)}>
    <span title={error instanceof Error ? error.message : undefined}>{getKitLabels().stale.loadFailed}</span><span aria-hidden="true">·</span><Button size="sm" variant="ghost" onClick={() => { void onRetry() }}>{getKitLabels().stale.retry}</Button>
  </div>
}
