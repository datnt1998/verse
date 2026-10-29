// Ngăn kéo Não truyện (Gói U): khung 1180/84vw; nội dung dùng chung màn Não truyện.
import * as stylex from '@stylexjs/stylex'
import { brain, color, line } from '@noverse/ui/tokens.stylex'

const NO_MOTION = '@media (prefers-reduced-motion: reduce)'

const slideIn = stylex.keyframes({
  from: { transform: 'translateX(28px)', opacity: 0 },
  to: { transform: 'none', opacity: 1 },
})

const fadeIn = stylex.keyframes({
  from: { opacity: 0 },
  to: { opacity: 1 },
})

export const s = stylex.create({
  overlay: { position: 'fixed', inset: 0, zIndex: 40, display: 'flex', justifyContent: 'flex-end' },
  scrim: {
    position: 'absolute', inset: 0, backgroundColor: brain['--brain-scrim'],
    animationName: { default: fadeIn, [NO_MOTION]: 'none' },
    animationDuration: '.18s', animationTimingFunction: 'ease', animationFillMode: 'both',
  },
  panel: {
    position: 'relative', width: 'min(1180px, 84vw)', maxWidth: '100%', height: '100%',
    display: 'flex', flexDirection: 'column', minHeight: 0, overflow: 'hidden',
    backgroundColor: brain['--brain-drawer-bg'], borderLeftWidth: 1, borderLeftStyle: 'solid',
    borderLeftColor: line['--line-3'], boxShadow: brain['--brain-drawer-shadow'],
    animationName: { default: slideIn, [NO_MOTION]: 'none' },
    animationDuration: '.24s', animationTimingFunction: 'cubic-bezier(.2,.9,.3,1)', animationFillMode: 'both',
    outline: 'none',
  },
  header: {
    flexShrink: 0, display: 'flex', alignItems: 'center', gap: 10, paddingBlock: 14, paddingInline: 18,
    borderBottomWidth: 1, borderBottomStyle: 'solid', borderBottomColor: line['--line-1'],
  },
  headCopy: { flexShrink: 1, flexBasis: 'auto', minWidth: 0, display: 'flex', flexDirection: 'column' },
  title: { margin: 0, fontSize: 15, fontWeight: 600 },
  sub: {
    fontSize: 12, color: color['--color-fg-4'], whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis',
  },
  lockChip: {
    display: 'inline-flex', alignItems: 'center', gap: 6, height: 26, paddingInline: 9,
    marginLeft: 6, borderRadius: 7, backgroundColor: color['--color-scope-bg'],
    borderWidth: 1, borderStyle: 'solid', borderColor: line['--line-2'],
    fontSize: 12, color: color['--color-fg-2'], whiteSpace: 'nowrap',
  },
  headActions: { flexGrow: 1, display: 'flex', alignItems: 'center', justifyContent: 'flex-end', gap: 10 },
  // Nút ghost có viền (Main.dc.html › “Mở toàn màn ›” / “Đóng Esc”): a{color} ở base.css → màu ở JSX.
  openFull: {
    height: 30, paddingInline: 10, display: 'flex', alignItems: 'center', gap: 6,
    borderRadius: 8, borderWidth: 1, borderStyle: 'solid', borderColor: line['--line-4'],
    color: color['--color-fg'], fontSize: 13,
  },
  close: {
    height: 30, paddingLeft: 10, paddingRight: 8, borderRadius: 8, borderWidth: 1, borderStyle: 'solid',
    borderColor: line['--line-4'], backgroundColor: 'transparent', color: color['--color-fg'],
    display: 'flex', alignItems: 'center', gap: 8, fontSize: 13, cursor: 'pointer',
  },
  body: {
    flexGrow: 1, flexShrink: 1, flexBasis: 0, minHeight: 0,
    display: 'grid', gridTemplateColumns: 'minmax(0, 1fr) 320px',
  },
  graphCol: {
    minWidth: 0, minHeight: 0, display: 'flex', flexDirection: 'column',
    borderRightWidth: 1, borderRightStyle: 'solid', borderRightColor: line['--line-1'],
    background: brain['--brain-glow-radial'],
  },
  detailCol: { minWidth: 0, minHeight: 0, display: 'flex', flexDirection: 'column', overflow: 'hidden' },
  detailScroll: {
    flexGrow: 1, flexShrink: 1, flexBasis: 0, minHeight: 0, display: 'flex', flexDirection: 'column',
    overflow: 'hidden',
  },
  lockNote: {
    flexShrink: 0, marginTop: 'auto', paddingBlock: 10, paddingInline: 12, borderRadius: 10,
    backgroundColor: 'var(--kit-acc-6)', borderWidth: 1, borderStyle: 'solid', borderColor: brain['--brain-lock-line'], fontSize: 12, color: brain['--brain-chg'], lineHeight: 1.5,
  },
})
