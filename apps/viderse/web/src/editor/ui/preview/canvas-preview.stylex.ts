import * as stylex from '@stylexjs/stylex'
import { color } from '@noverse/ui/tokens.stylex'

export const canvasPreviewStyles = stylex.create({
  canvas: { pointerEvents: 'none', position: 'absolute', inset: 0, width: '100%', height: '100%' },
  error: { pointerEvents: 'none', position: 'absolute', inset: 0, display: 'flex', alignItems: 'center', justifyContent: 'center', backgroundColor: color['--color-bg'], padding: 16, textAlign: 'center', fontSize: 12, color: color['--color-fg'] },
})
