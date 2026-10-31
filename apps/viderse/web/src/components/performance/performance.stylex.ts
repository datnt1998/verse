import * as stylex from '@stylexjs/stylex'
import { color, line, surface } from '@noverse/ui/tokens.stylex'
import { space, corners } from '../../styles/viderse.stylex'

// Trợ lý nhận ra (artboard Channel › filter=posted): section .card r14, padding
// 2px 16px 4px; đầu 10/8; mỗi InsightRow lưới 1fr|auto gap 6/14, padding 12 0,
// kẻ trên hairline; thanh 150px | 1fr | 60px, cao 6.
export const performanceStyles = stylex.create({
  card: {
    borderRadius: corners['--vv-r-14'], padding: '2px var(--vv-s-16) var(--vv-s-4)',
    backgroundImage: surface['--card-bg'], backgroundColor: 'transparent',
    borderWidth: space['--vv-s-1'], borderStyle: 'solid', borderColor: line['--line-2'], boxShadow: surface['--card-shadow'],
  },
  head: { display: 'flex', alignItems: 'center', gap: space['--vv-s-10'], padding: 'var(--vv-s-10) 0 var(--vv-s-8)', flexWrap: 'wrap' },
  title: { fontSize: 14, fontWeight: 600 },
  tag: {
    display: 'inline-flex', alignItems: 'center', height: 22, padding: '0 var(--vv-s-8)', gap: space['--vv-s-6'],
    borderRadius: corners['--vv-r-6'], fontSize: 11.5, backgroundColor: 'rgba(246,203,140,.12)', color: color['--color-warn'],
  },
  grow: { flex: '1 1 0', minWidth: 0 },
  help: { fontSize: 11.5, color: 'var(--vv-fg-note)' },
  row: {
    display: 'grid', gridTemplateColumns: 'minmax(0, 1fr) auto', alignItems: 'center', gap: 'var(--vv-s-6) var(--vv-s-14)',
    padding: 'var(--vv-s-12) 0', borderTopWidth: space['--vv-s-1'], borderTopStyle: 'solid', borderTopColor: line['--line-1'],
  },
  text: { fontSize: 13.5, lineHeight: 1.45, minWidth: 0 },
  bars: { gridColumn: 1, display: 'flex', flexDirection: 'column', gap: space['--vv-s-4'] },
  bar: {
    display: 'grid', gridTemplateColumns: '150px minmax(0, 1fr) 60px', alignItems: 'center', gap: space['--vv-s-8'],
    fontSize: 12, color: color['--color-fg-3'],
  },
  barLabel: { minWidth: 0, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' },
  track: { position: 'relative', height: 6, borderRadius: corners['--vv-r-3'], backgroundColor: 'rgba(255,255,255,.08)', overflow: 'hidden' },
  fill: { position: 'absolute', top: 0, bottom: 0, left: 0, borderRadius: corners['--vv-r-3'], backgroundColor: 'rgba(255,255,255,.3)' },
  fillWidth: (percent: number) => ({ width: `${percent}%` }),
  skelWidth: (percent: number) => ({ width: `${percent}%` }),
  fillTop: { backgroundColor: 'var(--vv-scout-ink)' },
  value: { textAlign: 'right', fontWeight: 500, color: color['--color-fg-2'], fontVariantNumeric: 'tabular-nums' },
  applied: { fontSize: 12, color: color['--color-ok'], whiteSpace: 'nowrap' },
  note: { display: 'flex', flexDirection: 'column', gap: space['--vv-s-4'], padding: 'var(--vv-s-12) 0', borderTopWidth: space['--vv-s-1'], borderTopStyle: 'solid', borderTopColor: line['--line-1'] },
  noteTitle: { fontSize: 13.5, color: color['--color-fg'] },
  noteSub: { fontSize: 12, color: 'var(--vv-fg-note)', lineHeight: 1.45 },
  noteErr: { fontSize: 13, color: color['--color-err'] },
  noteRow: { display: 'flex', alignItems: 'center', gap: space['--vv-s-10'], flexWrap: 'wrap' },
  skel: { height: 12, borderRadius: corners['--vv-r-4'], backgroundColor: 'rgba(255,255,255,.08)' },
  // Thẻ mời xem (filter=all): .vv-suggest dạng nút 3 cột.
  teaser: {
    display: 'grid', gridTemplateColumns: 'auto minmax(0, 1fr) auto', alignItems: 'center', gap: 'var(--vv-s-4) var(--vv-s-10)',
    width: '100%', padding: space['--vv-s-12'], textAlign: 'left', cursor: 'pointer', color: 'inherit', font: 'inherit',
    borderRadius: corners['--vv-r-12'], backgroundColor: color['--color-surface-1'],
    borderWidth: space['--vv-s-1'], borderStyle: 'solid', borderColor: line['--line-2'],
  },
  teaserBody: { display: 'flex', flexDirection: 'column', gap: space['--vv-s-1'], minWidth: 0 },
  teaserTitle: { fontSize: 13.5 },
  teaserEv: { fontSize: 12, lineHeight: 1.45, color: 'var(--vv-fg-note)', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' },
  teaserGo: { fontSize: 12.5, color: color['--color-fg-3'], whiteSpace: 'nowrap' },
  dialogField: { display: 'flex', flexDirection: 'column', gap: space['--vv-s-6'], marginTop: space['--vv-s-12'] },
  dialogLabel: { fontSize: 12, color: color['--color-fg-3'] },
})
