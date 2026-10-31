import * as stylex from '@stylexjs/stylex'
import { color, line } from '@noverse/ui/tokens.stylex'
import { previewTokens as t } from './preview-tokens.stylex'

export const previewStyles = stylex.create({
  root: { flexGrow: 1, minWidth: 0, minHeight: 0, display: 'flex', flexDirection: 'column', padding: '12px 14px 8px', gap: 8, backgroundColor: t.background, color: color['--color-fg'] },
  stage: { flexGrow: 1, minHeight: 0, display: 'flex', alignItems: 'center', justifyContent: 'center', containerType: 'size', overflow: 'hidden' },
  frame: (ratio: number) => ({ position: 'relative', flexShrink: 0, width: `min(100cqw, calc(100cqh * ${ratio}))`, aspectRatio: ratio, borderRadius: 6, overflow: 'hidden', backgroundColor: color['--color-accent-fg'], boxShadow: `0 0 0 1px ${line['--line-1']}` }),
  player: { position: 'absolute', inset: 0, width: '100%', height: '100%' },
  outerSafe: { position: 'absolute', inset: '5%', border: `1px dashed ${t.safeOuter}`, pointerEvents: 'none' },
  innerSafe: { position: 'absolute', inset: '10%', border: `1px dashed ${t.safeInner}`, pointerEvents: 'none' },
  transport: { flexShrink: 0, display: 'flex', alignItems: 'center', gap: 4, minWidth: 0 },
  button: { display: 'inline-flex', alignItems: 'center', justifyContent: 'center', boxSizing: 'border-box', flexShrink: 0, width: 28, height: 28, padding: 0, borderWidth: 0, borderRadius: 7, backgroundColor: { default: 'transparent', ':hover': t.active }, color: color['--color-fg-2'], cursor: 'pointer', outlineColor: t.red },
  play: { width: 34, height: 34, borderRadius: '50%', backgroundColor: { default: color['--color-fg'], ':hover': color['--color-fg-2'] }, color: color['--color-bg'] },
  timer: { marginInlineStart: 8, fontFamily: 'var(--font-mono, monospace)', fontSize: 12.5, fontVariantNumeric: 'tabular-nums', whiteSpace: 'nowrap' },
  muted: { color: t.muted },
  spacer: { flexGrow: 1 },
  safeButton: { width: 'auto', gap: 6, paddingInline: 9, border: `1px solid ${line['--line-1']}`, fontSize: 12, fontFamily: 'inherit', whiteSpace: 'nowrap' },
  safeActive: { borderColor: t.safeInner },
  field: { display: 'inline-flex', alignItems: 'center', gap: 6, width: 'auto', flexShrink: 0, height: 28, paddingInline: 8, borderRadius: 7, border: `1px solid ${line['--line-4']}`, backgroundColor: color['--color-surface-1'], fontSize: 12.5, color: color['--color-fg'], whiteSpace: 'nowrap' },
  quality: { display: { default: 'inline-flex', '@media (max-width: 1360px)': 'none' } },
  zoom: { display: { default: 'inline-flex', '@media (max-width: 1279px)': 'none' } },
  safeLabel: { display: { default: 'inline', '@media (max-width: 1180px)': 'none' } },
  volume: { position: 'relative', display: 'flex', alignItems: 'center' },
  volumePanel: { position: 'absolute', bottom: 36, right: 0, zIndex: 10, display: 'flex', alignItems: 'center', gap: 8, padding: '10px 12px', borderRadius: 9, border: `1px solid ${line['--line-3']}`, backgroundColor: color['--color-surface-0'], color: color['--color-fg-2'], fontSize: 12 },
  volumeRange: { width: 90, accentColor: t.red },
  volumeValue: { minWidth: 34, fontVariantNumeric: 'tabular-nums' },
  empty: { position: 'absolute', inset: 0, display: 'flex', alignItems: 'center', justifyContent: 'center', color: t.muted, fontSize: 13, padding: 16, textAlign: 'center', pointerEvents: 'none' },
})
