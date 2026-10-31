import * as stylex from '@stylexjs/stylex'
import { color } from '@noverse/ui/tokens.stylex'

const spacing = stylex.defineVars({ gap: '12px', padding: '32px' })

export const styles = stylex.create({
  page: { minHeight: '100dvh', display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', gap: spacing.gap, padding: spacing.padding, backgroundColor: color['--color-bg'], color: color['--color-fg'], fontSize: 14, lineHeight: 1.55, textAlign: 'center' },
  heading: { margin: 0, fontSize: 16, fontWeight: 600 },
  message: { margin: 0, maxWidth: 640, overflowWrap: 'anywhere' },
  error: { color: color['--color-err'] },
})
