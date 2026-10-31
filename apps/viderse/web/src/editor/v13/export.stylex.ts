import * as stylex from '@stylexjs/stylex'
import { color, font, radius } from '@noverse/ui/tokens.stylex'
import { metrics } from './editor.stylex'
import { v13 } from './editor-tokens.stylex'

export const tokens = stylex.defineVars({ readyFill: 'rgba(157,236,201,.05)', readyBorder: 'rgba(157,236,201,.2)', playFill: 'rgba(11,11,13,.55)', playBorder: 'rgba(255,255,255,.25)', durationFill: 'rgba(0,0,0,.75)', durationRadius: '4px' })
export const exportStyles = stylex.create({
  row: { display: 'flex', alignItems: 'center', gap: metrics['--ed-space-10'] },
  muted: { color: v13.muted },
  meta: { color: v13.muted, fontSize: 12.5 },
  dot: { width: 16, height: 16, flexShrink: 0, display: 'flex', alignItems: 'center', justifyContent: 'center' },
  videoFrame: { position: 'relative', width: '100%', aspectRatio: '16/9', borderRadius: radius['--radius-tile'], overflow: 'hidden', backgroundColor: color['--color-bg'] },
  video: { display: 'block', width: '100%', height: '100%' },
  play: { position: 'absolute', left: '50%', top: '50%', transform: 'translate(-50%,-50%)', width: 52, height: 52, borderRadius: '50%', borderWidth: 1, borderStyle: 'solid', borderColor: tokens.playBorder, backgroundColor: tokens.playFill, color: color['--color-fg'], display: 'flex', alignItems: 'center', justifyContent: 'center', cursor: 'pointer' },
  duration: { position: 'absolute', right: metrics['--ed-space-8'], bottom: metrics['--ed-space-7'], paddingInline: metrics['--ed-space-5'], borderRadius: tokens.durationRadius, backgroundColor: tokens.durationFill, color: color['--color-fg'], fontSize: 11, fontFamily: font['--font-mono'], fontVariantNumeric: 'tabular-nums' },
  notice: { display: 'flex', alignItems: 'flex-start', gap: metrics['--ed-space-10'], paddingBlock: metrics['--ed-space-10'], paddingInline: metrics['--ed-space-12'], borderRadius: radius['--radius-row'], borderWidth: 1, borderStyle: 'solid', borderColor: tokens.readyBorder, backgroundColor: tokens.readyFill, fontSize: 12.5, color: color['--color-fg-2'], lineHeight: 1.55 },
})
