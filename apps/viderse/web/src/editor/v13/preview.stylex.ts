import * as stylex from '@stylexjs/stylex'
import { color, font, line, radius } from '@noverse/ui/tokens.stylex'
import { corners, space, typography } from '../../styles/viderse.stylex'
import { metrics } from './editor.stylex'
import { v13 } from './editor-tokens.stylex'
import { previewTokens as t } from './preview-tokens.stylex'

export const previewStyles = stylex.create({
  root: { flexGrow: 1, minWidth: 0, minHeight: 0, display: 'flex', flexDirection: 'column', paddingTop: metrics['--ed-space-12'], paddingInline: metrics['--ed-space-14'], paddingBottom: metrics['--ed-space-8'], gap: metrics['--ed-space-8'], backgroundColor: t.background, color: color['--color-fg'] },
  stage: { flexGrow: 1, minHeight: 0, display: 'flex', alignItems: 'center', justifyContent: 'center', containerType: 'size', overflow: 'hidden' },
  frame: (ratio: number) => ({ position: 'relative', flexShrink: 0, width: `min(100cqw, calc(100cqh * ${ratio}))`, aspectRatio: ratio, borderRadius: radius['--radius-chip'], overflow: 'hidden', backgroundColor: color['--color-accent-fg'], boxShadow: `0 0 0 1px ${line['--line-1']}` }),
  player: { position: 'absolute', inset: 0, width: '100%', height: '100%' },
  outerSafe: { position: 'absolute', inset: '5%', borderWidth: 1, borderStyle: 'dashed', borderColor: t.safeOuter, pointerEvents: 'none' },
  innerSafe: { position: 'absolute', inset: '10%', borderWidth: 1, borderStyle: 'dashed', borderColor: t.safeInner, pointerEvents: 'none' },
  transport: { flexShrink: 0, display: 'flex', alignItems: 'center', gap: metrics['--ed-space-4'], minWidth: 0 },
  button: { display: 'inline-flex', alignItems: 'center', justifyContent: 'center', boxSizing: 'border-box', flexShrink: 0, width: space['--vv-s-28'], height: space['--vv-s-28'], padding: 0, borderWidth: 0, borderRadius: corners['--vv-r-7'], backgroundColor: { default: 'transparent', ':hover': t.active }, color: color['--color-fg-2'], cursor: 'pointer', ':focus-visible': { outlineWidth: 2, outlineStyle: 'solid', outlineColor: v13.accent, outlineOffset: 2 } },
  play: { width: space['--vv-s-34'], height: space['--vv-s-34'], borderRadius: '50%', backgroundColor: { default: color['--color-fg'], ':hover': color['--color-fg-2'] }, color: color['--color-bg'] },
  timer: { marginInlineStart: metrics['--ed-space-8'], fontFamily: font['--font-sans'], fontSize: typography['--vv-fs-detail'], fontVariantNumeric: 'tabular-nums', whiteSpace: 'nowrap' },
  muted: { color: t.muted },
  mutedInk: { color: t.muted },
  spacer: { flexGrow: 1 },
  safeButton: { width: 'auto', gap: metrics['--ed-space-6'], paddingInline: space['--vv-s-9'], borderWidth: 1, borderStyle: 'solid', borderColor: line['--line-1'], fontSize: typography['--vv-fs-meta'], fontFamily: 'inherit', whiteSpace: 'nowrap' },
  safeActive: { borderColor: t.safeInner },
  field: { display: 'inline-flex', alignItems: 'center', gap: metrics['--ed-space-6'], width: 'auto', flexShrink: 0, height: space['--vv-s-28'], paddingInline: metrics['--ed-space-8'], borderRadius: corners['--vv-r-7'], borderWidth: 1, borderStyle: 'solid', borderColor: line['--line-4'], backgroundColor: color['--color-surface-1'], fontSize: typography['--vv-fs-detail'], color: color['--color-fg'], whiteSpace: 'nowrap' },
  quality: { display: { default: 'inline-flex', '@media (max-width: 1360px)': 'none' } },
  zoom: { display: { default: 'inline-flex', '@media (max-width: 1279px)': 'none' } },
  safeLabel: { display: { default: 'inline', '@media (max-width: 1180px)': 'none' } },
  volume: { position: 'relative', display: 'flex', alignItems: 'center' },
  volumePanel: { position: 'absolute', bottom: 36, right: 0, zIndex: 10, display: 'flex', alignItems: 'center', gap: metrics['--ed-space-8'], paddingBlock: metrics['--ed-space-10'], paddingInline: metrics['--ed-space-12'], borderRadius: corners['--vv-r-9'], borderWidth: 1, borderStyle: 'solid', borderColor: line['--line-3'], backgroundColor: color['--color-surface-0'], color: color['--color-fg-2'], fontSize: typography['--vv-fs-meta'] },
  volumeRange: { width: 90, accentColor: t.red },
  volumeValue: { minWidth: space['--vv-s-34'], fontVariantNumeric: 'tabular-nums' },
  empty: { position: 'absolute', inset: 0, display: 'flex', alignItems: 'center', justifyContent: 'center', color: t.muted, fontSize: typography['--vv-fs-ui'], padding: metrics['--ed-space-16'], textAlign: 'center', pointerEvents: 'none' },
  interaction: { position: 'absolute', inset: 0, touchAction: 'none', outline: 'none' },
  selectionBounds: { position: 'absolute', boxSizing: 'border-box', borderWidth: 2, borderStyle: 'solid', borderColor: v13.accent, pointerEvents: 'none' },
  transformHandle: { position: 'absolute', display: 'flex', alignItems: 'center', justifyContent: 'center', width: 26, height: 26, padding: 0, borderWidth: 1, borderStyle: 'solid', borderColor: v13.accent, borderRadius: 4, backgroundColor: color['--color-surface-1'], color: color['--color-fg'], pointerEvents: 'auto', touchAction: 'none', ':focus-visible': { outlineWidth: 2, outlineStyle: 'solid', outlineColor: v13.accent, outlineOffset: 2 } },
  moveHandle: { left: '50%', top: '50%', transform: 'translate(-50%, -50%)', cursor: 'move' },
  scaleHandle: { right: 0, bottom: 0, cursor: 'nwse-resize' },
  rotateHandle: { left: '50%', top: 0, transform: 'translateX(-50%)', borderRadius: '50%', cursor: 'grab' },
})
