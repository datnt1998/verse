// Điểm Kiểm định trên Review (artboard Review › score=true: .vv-imgscore,
// dòng tóm tắt .vv-banner, cột phải "Điểm của Kiểm định" + .vv-why).
import * as stylex from '@stylexjs/stylex'
import { color, line } from '@noverse/ui/tokens.stylex'
import { space, corners } from '../../styles/viderse.stylex'

export const scoreStyles = stylex.create({
  badge: { height: space['--vv-s-20'], paddingInline: space['--vv-s-6'], display: 'inline-flex', alignItems: 'center', gap: space['--vv-s-3'], borderRadius: corners['--vv-r-6'], backgroundColor: 'rgba(0,0,0,.72)', color: 'var(--vv-qa-ink)', fontSize: 11.5, fontWeight: 600, fontVariantNumeric: 'tabular-nums' },
  badgeLow: { backgroundColor: color['--color-err'], color: 'var(--vv-warn-btn-ink)' },
  badgeCorner: { position: 'absolute', left: space['--vv-s-4'], bottom: space['--vv-s-4'], height: 17, fontSize: 10.5 },
  banner: { flex: '0 0 auto', display: 'flex', alignItems: 'center', gap: space['--vv-s-12'], paddingBlock: space['--vv-s-8'], paddingInline: space['--vv-s-12'], borderRadius: corners['--vv-r-12'], borderWidth: space['--vv-s-1'], borderStyle: 'solid', borderColor: line['--line-2'], backgroundColor: color['--color-surface-1'], fontSize: 12, color: color['--color-fg-3'], lineHeight: 1.5 },
  bannerText: { flex: '1 1 0', minWidth: 0 },
  strong: { color: color['--color-fg'], fontWeight: 600 },
  strongLow: { color: 'var(--vv-warn-ink)' },
  jump: { flex: '0 0 auto', height: space['--vv-s-30'], paddingInline: space['--vv-s-10'], borderRadius: corners['--vv-r-8'], borderWidth: space['--vv-s-1'], borderStyle: 'solid', borderColor: line['--line-4'], backgroundColor: 'transparent', color: color['--color-fg'], fontFamily: 'inherit', fontSize: 12, cursor: 'pointer', whiteSpace: 'nowrap' },
  tileLow: { borderColor: color['--color-err'] },
  panel: { display: 'flex', flexDirection: 'column', gap: space['--vv-s-6'] },
  panelHead: { display: 'flex', alignItems: 'center', gap: space['--vv-s-8'] },
  panelTitle: { margin: 0, fontSize: 13, fontWeight: 500, color: color['--color-fg-3'], whiteSpace: 'nowrap' },
  spacer: { flex: '1 1 0' },
  why: { display: 'inline-flex', alignItems: 'center', gap: space['--vv-s-4'], height: space['--vv-s-22'], paddingInline: space['--vv-s-2'], borderWidth: 0, borderStyle: 'none', borderColor: 'currentColor', backgroundColor: 'transparent', color: { default: color['--color-fg-3'], ':hover': color['--color-fg'] }, fontFamily: 'inherit', fontSize: 12, whiteSpace: 'nowrap', textDecorationLine: 'underline', textDecorationStyle: 'dotted', textUnderlineOffset: space['--vv-s-3'], cursor: 'pointer' },
  whyOpen: { color: color['--color-fg'] },
  status: { margin: 0, fontSize: 12, color: 'var(--vv-fg-note)' },
})
